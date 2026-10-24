package connections

import (
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/gorilla/websocket"
)

const ReadBufferSize = 1024

type ConnectionId = uint64

var (

	//
	// Mutex
	//
	lock sync.RWMutex = sync.RWMutex{}
	//
	// Counters
	//
	connectCounter    uint64 = 0 // a counter for each time a connection is accepted
	disconnectCounter uint64 = 0 // a counter for each tim ea connection is dropped
	//
	// Track connections
	//
	netConnections map[ConnectionId]*ConnectionDetails = map[ConnectionId]*ConnectionDetails{} // a mapping of unique id's to connections
	//
	// Channel to send a shutdown signal to
	//
	shutdownChannel chan os.Signal // channel to receive shutdown signals
)

func SignalShutdown(s os.Signal) {
	if shutdownChannel != nil {
		shutdownChannel <- s
	}
}

func Add(conn net.Conn, wsConn *websocket.Conn, cType ...ConnType) *ConnectionDetails {

	lock.Lock()
	defer lock.Unlock()

	connectCounter++

	connDetails := NewConnectionDetails(
		connectCounter,
		conn,
		wsConn,
		nil, // use default settings for now TODO: add into overall config pattern?
	)

	if len(cType) > 0 {
		connDetails.connType = cType[0]
	}

	netConnections[connDetails.ConnectionId()] = connDetails

	// return the unique ID to find this connection later
	return connDetails
}

// Returns the total number of connections
func Get(id ConnectionId) *ConnectionDetails {
	lock.Lock()
	defer lock.Unlock()

	return netConnections[id]
}

func IsWebsocket(id ConnectionId) bool {
	lock.Lock()
	defer lock.Unlock()

	if cd, ok := netConnections[id]; ok {
		return cd.IsWebSocket()
	}

	return false
}

func GetAllConnectionIds() []ConnectionId {

	lock.Lock()
	defer lock.Unlock()

	ids := make([]ConnectionId, 0, len(netConnections))

	for id := range netConnections {
		ids = append(ids, id)
	}

	return ids
}

// cleanupWait bounds how long Cleanup waits for writers in total; see
// WriterDrainTimeout.
const cleanupWait = WriterDrainTimeout

// Cleanup removes every connection (server shutdown) and waits, bounded by
// cleanupWait in total, for each writer goroutine to deliver what was queued
// and exit. Runs on the shutdown path, not the game loop.
func Cleanup() {
	lock.RLock()
	cds := make([]*ConnectionDetails, 0, len(netConnections))
	for _, cd := range netConnections {
		cds = append(cds, cd)
	}
	lock.RUnlock()

	for _, cd := range cds {
		_ = Remove(cd.ConnectionId())
	}

	deadline := time.NewTimer(cleanupWait)
	defer deadline.Stop()
	for _, cd := range cds {
		if cd.wsConn == nil && cd.conn == nil {
			continue // socketless; never had a writer
		}
		select {
		case <-cd.sendQueue().done:
		case <-deadline.C:
			mudlog.Warn("Cleanup", "error", "writers still running at shutdown", "waited", cleanupWait)
			return
		}
	}
}

func Kick(id ConnectionId, reason string) (err error) {

	lock.Lock()
	defer lock.Unlock()

	// Try to retrieve the value
	if cd, ok := netConnections[id]; ok {

		// close the connection, no longer useful.
		cd.Close()
		// keep track of the number of disconnects
		disconnectCounter++
		// remove the connection from the map (2026-08-03: the comment was
		// here but the delete was not — a kicked connection leaked its
		// registry entry until something else happened to call Remove).
		delete(netConnections, id)
		mudlog.Info("connection kicked", "connectionId", id, "remoteAddr", cd.RemoteAddr().String(), `reason`, reason)

		return nil

	}

	return errors.New("connection not found")
}

func Remove(id ConnectionId) (err error) {

	lock.Lock()
	defer lock.Unlock()

	// Try to retrieve the value
	if cd, ok := netConnections[id]; ok {

		// close the connection, no longer useful.
		cd.Close()
		// keep track of the number of disconnects
		disconnectCounter++
		// Remove the entry
		delete(netConnections, id)

		return nil

	}

	return errors.New("connection not found")
}

// writeTarget pairs a connection id with the connection it was resolved from,
// so a write can happen after the registry lock has been released.
type writeTarget struct {
	id ConnectionId
	cd *ConnectionDetails
}

// reportSendError logs a failed enqueue. ErrSendQueueFull was already logged
// (as SendQueueOverCap, with the queued size) where it was detected, and
// ErrConnectionClosing means whatever closed the connection already logged
// why; repeating either per queued message is noise.
func reportSendError(where string, t writeTarget, err error) {
	if errors.Is(err, ErrSendQueueFull) || errors.Is(err, ErrConnectionClosing) {
		return
	}
	mudlog.Warn(where, "connectionId", t.id, "remoteAddr", t.cd.remoteAddrString(), "error", err)
}

// Broadcast queues text for every logged-in connection and returns without
// waiting on any socket (see sendqueue.go).
//
// The registry lock is held ONLY long enough to snapshot the targets. Queueing
// happens with it released, and a connection over its queue cap is Removed
// here, the same path a failed write has always taken.
func Broadcast(colorizedText []byte, skipConnectionIds ...ConnectionId) []ConnectionId {

	lock.RLock()
	targets := make([]writeTarget, 0, len(netConnections))
	for id, cd := range netConnections {

		if cd.State() == Login {
			continue
		}

		if len(skipConnectionIds) > 0 {
			skip := false
			for _, cId := range skipConnectionIds {
				if cId == id {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}

		targets = append(targets, writeTarget{id: id, cd: cd})
	}
	lock.RUnlock()

	removeIds := []ConnectionId{}
	sentToIds := make([]ConnectionId, 0, len(targets))

	// Lock released. A target may be removed out from under us here; Write()
	// returns an error (never panics) for a closed or detached socket.
	for _, t := range targets {

		if _, err := t.cd.Write(colorizedText); err != nil {
			reportSendError("Broadcast()", t, err)
			// Remove from the connections
			removeIds = append(removeIds, t.id)
		}

		sentToIds = append(sentToIds, t.id)
	}

	for _, id := range removeIds {
		Remove(id)
	}

	return sentToIds
}

// SendTo queues b for the named connections and returns without waiting on
// any socket. As with Broadcast, the registry lock is released before queueing.
func SendTo(b []byte, ids ...ConnectionId) {

	lock.RLock()
	targets := make([]writeTarget, 0, len(ids))
	for _, id := range ids {
		if cd, ok := netConnections[id]; ok {
			targets = append(targets, writeTarget{id: id, cd: cd})
		}
	}
	lock.RUnlock()

	removeIds := []ConnectionId{}

	for _, t := range targets {
		if _, err := t.cd.Write(b); err != nil {
			reportSendError("SendTo()", t, err)
			// Remove from the connections
			removeIds = append(removeIds, t.id)
		}
	}

	for _, id := range removeIds {
		Remove(id)
	}
}

// make this more efficient later
func ActiveConnectionCount() int {
	lock.RLock()
	defer lock.RUnlock()

	return len(netConnections)
}

func ActiveHumanConnectionCount() int {
	lock.RLock()
	defer lock.RUnlock()

	ct := 0
	for _, cd := range netConnections {
		if cd.ConnType() == ConnHuman {
			ct++
		}
	}
	return ct
}

func ActiveAIConnectionCount() int {
	lock.RLock()
	defer lock.RUnlock()

	ct := 0
	for _, cd := range netConnections {
		if cd.ConnType() == ConnAI {
			ct++
		}
	}
	return ct
}

// make this more efficient later
func SetShutdownChan(osSignalChan chan os.Signal) {
	lock.Lock()
	defer lock.Unlock()

	if shutdownChannel != nil {
		panic("Can't set shutdown channel a second time!")
	}
	shutdownChannel = osSignalChan
}

func Stats() (connections uint64, disconnections uint64) {
	lock.RLock()
	defer lock.RUnlock()

	return connectCounter, disconnectCounter
}

func GetClientSettings(id ConnectionId) ClientSettings {
	lock.Lock()
	defer lock.Unlock()

	if cd, ok := netConnections[id]; ok {
		return cd.clientSettings
	}

	return ClientSettings{}
}

func OverwriteClientSettings(id ConnectionId, cs ClientSettings) {
	lock.Lock()
	defer lock.Unlock()

	if cd, ok := netConnections[id]; ok {
		cd.clientSettings = cs
	}
}

// RegisterTestConnection registers a mock connection for testing purposes
func RegisterTestConnection(id ConnectionId) {
	lock.Lock()
	defer lock.Unlock()

	if id == 0 {
		id = 1
	}

	netConnections[id] = &ConnectionDetails{
		connectionId:   id,
		clientSettings: ClientSettings{},
	}
}

// UnregisterTestConnection removes a test connection (for cleanup)
func UnregisterTestConnection(id ConnectionId) {
	lock.Lock()
	defer lock.Unlock()

	delete(netConnections, id)
}
