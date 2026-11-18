package inputhandlers

import (
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/auththrottle"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"golang.org/x/crypto/bcrypt"
)

// gameLoginThrottle counts failed game logins per client IP: telnet, the AI
// port and the web client alike. The /admin login keeps its own tracker.
var gameLoginThrottle = auththrottle.New("GAME LOGIN THROTTLE")

// invalidLoginText is the one answer to a wrong password AND an unknown
// username, so the reply never tells which usernames exist.
const invalidLoginText = `Invalid login.`

// throttledLoginText answers a login from a source that is locked out.
const throttledLoginText = `Too many failed logins. Try again in a minute.`

// loginThrottleKey is the client IP of a connection: the socket peer for
// telnet, the proxy-resolved address for a websocket. Empty when the
// connection is gone.
func loginThrottleKey(id connections.ConnectionId) string {
	cd := connections.Get(id)
	if cd == nil {
		return ``
	}
	return cd.ClientIP()
}

// loginThrottled reports whether a connection's source is locked out.
func loginThrottled(id connections.ConnectionId) bool {
	key := loginThrottleKey(id)
	if key == `` {
		return false
	}
	locked, _ := gameLoginThrottle.Throttled(key, time.Now())
	return locked
}

// noteLoginFailure counts a failed login against a connection's source.
func noteLoginFailure(id connections.ConnectionId) {
	if key := loginThrottleKey(id); key != `` {
		gameLoginThrottle.NoteFailure(key, time.Now())
	}
}

// clearLoginFailures forgets a source's failures after a good login.
func clearLoginFailures(id connections.ConnectionId) {
	if key := loginThrottleKey(id); key != `` {
		gameLoginThrottle.Clear(key)
	}
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// spendPasswordCheck runs one bcrypt comparison against a fixed hash, so a
// login for an unknown username takes as long as a wrong password for a
// real one and the timing does not tell which usernames exist.
func spendPasswordCheck(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = bcrypt.GenerateFromPassword([]byte(`dogmud-unknown-user`), bcrypt.DefaultCost)
	})
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}
