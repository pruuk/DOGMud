package ambient

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// MoveKind names one counted kind of mob move. The strings are telemetry
// keys tools/ambient_report.py reads.
type MoveKind string

const (
	MoveIdle  MoveKind = "move_idle"  // a wander, sneak or portal from the idle roll
	MoveTrack MoveKind = "move_track" // a step toward a tracked target (move_toward_tracked)
	MovePath  MoveKind = "move_path"  // one step along a path: schedule, patrol, goal planner, home
)

// MoveKinds lists every move kind, in report order.
var MoveKinds = []MoveKind{MoveIdle, MoveTrack, MovePath}

// TelemetryMessage is the log message every summary line carries.
const TelemetryMessage = "ambient.telemetry"

type lineCounts struct{ offered, delivered, suppressed int }

type roomCounts struct {
	zone           string
	listenerRounds int
	lines          map[Source]*lineCounts
	moves          map[MoveKind]int
}

var (
	telMu         sync.Mutex
	telRooms      = map[int]*roomCounts{}
	telNextReport uint64
	telWindow     int
	// telKept is set while telRooms or telNextReport may hold anything, so
	// a round with telemetry off costs one load and no lock.
	telKept atomic.Bool

	// emit writes one summary line. Tests replace it to read the lines.
	emit = func(args ...any) { mudlog.Info(TelemetryMessage, args...) }
)

// TelemetryEnabled reports Balance.AmbientTelemetry. Off, nothing is
// counted and nothing is kept.
func TelemetryEnabled() bool {
	return bool(configs.GetBalanceConfig().AmbientTelemetry)
}

func roomEntryLocked(roomId int, zone string) *roomCounts {
	rc := telRooms[roomId]
	if rc == nil {
		rc = &roomCounts{zone: zone, lines: map[Source]*lineCounts{}, moves: map[MoveKind]int{}}
		telRooms[roomId] = rc
		telKept.Store(true)
	}
	return rc
}

// countLine records one Plan: per listener, delivered and suppressed.
func countLine(room *rooms.Room, src Source, delivered, suppressed int) {
	if room == nil || delivered+suppressed == 0 || !TelemetryEnabled() {
		return
	}
	telMu.Lock()
	defer telMu.Unlock()
	rc := roomEntryLocked(room.RoomId, room.Zone)
	lc := rc.lines[src]
	if lc == nil {
		lc = &lineCounts{}
		rc.lines[src] = lc
	}
	lc.offered += delivered + suppressed
	lc.delivered += delivered
	lc.suppressed += suppressed
}

// CountMove counts one mob move that starts in roomId.
func CountMove(roomId int, kind MoveKind) {
	if !TelemetryEnabled() {
		return
	}
	room := rooms.LoadRoom(roomId)
	if room == nil {
		return
	}
	telMu.Lock()
	defer telMu.Unlock()
	roomEntryLocked(roomId, room.Zone).moves[kind]++
}

func roundSeconds() int {
	secs := int(configs.GetTimingConfig().RoundSeconds)
	if secs < 1 {
		secs = 4
	}
	return secs
}

func reportWindowRounds() uint64 {
	w := int(configs.GetBalanceConfig().AmbientTelemetryReportMinutes) * 60 / roundSeconds()
	if w < 1 {
		w = 1
	}
	return uint64(w)
}

// telemetryTick adds listener rounds for every room with players and, when
// the window is up, writes one line per active room and resets. Turned off,
// it drops whatever was kept once and then does nothing.
func telemetryTick(round uint64) {
	if !TelemetryEnabled() {
		if telKept.Load() {
			telMu.Lock()
			telRooms = map[int]*roomCounts{}
			telNextReport = 0
			telKept.Store(false)
			telMu.Unlock()
		}
		return
	}
	window := reportWindowRounds()

	type occupied struct {
		id, players int
		zone        string
	}
	var occ []occupied
	for _, roomId := range rooms.GetRoomsWithPlayers() {
		if room := rooms.LoadRoom(roomId); room != nil {
			if n := len(room.GetPlayers()); n > 0 {
				occ = append(occ, occupied{id: roomId, players: n, zone: room.Zone})
			}
		}
	}

	telMu.Lock()
	defer telMu.Unlock()
	if telNextReport == 0 {
		telNextReport = round + window
		telKept.Store(true)
	}
	for _, o := range occ {
		roomEntryLocked(o.id, o.zone).listenerRounds += o.players
	}
	if round < telNextReport {
		return
	}
	telWindow++
	flushLocked(window)
	telNextReport = round + window
}

func flushLocked(window uint64) {
	secs := roundSeconds()
	ids := make([]int, 0, len(telRooms))
	for id := range telRooms {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		rc := telRooms[id]
		args := []any{"window", telWindow, "room", id, "zone", rc.zone,
			"round_seconds", secs, "window_rounds", int(window), "listener_rounds", rc.listenerRounds}
		for _, src := range Sources {
			lc := rc.lines[src]
			if lc == nil {
				lc = &lineCounts{}
			}
			args = append(args,
				string(src)+"_offered", lc.offered,
				string(src)+"_delivered", lc.delivered,
				string(src)+"_suppressed", lc.suppressed)
		}
		for _, mk := range MoveKinds {
			args = append(args, string(mk), rc.moves[mk])
		}
		emit(args...)
	}
	telRooms = map[int]*roomCounts{}
}
