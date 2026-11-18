package inputhandlers

import (
	"time"

	"github.com/GoMudEngine/GoMud/internal/auththrottle"
	"github.com/GoMudEngine/GoMud/internal/connections"
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

// loginAttempt counts a login attempt against a connection's source, or
// refuses it (false) while the source is locked out. Called before any
// password work.
func loginAttempt(id connections.ConnectionId) bool {
	key := loginThrottleKey(id)
	if key == `` {
		return true
	}
	return gameLoginThrottle.Attempt(key, time.Now())
}

// loginSucceeded gives back the count of a login that succeeded. It never
// clears the source's other failures: anyone can make an account, so a
// clear on success would let a guesser reset the count between guesses.
func loginSucceeded(id connections.ConnectionId) {
	if key := loginThrottleKey(id); key != `` {
		gameLoginThrottle.Refund(key)
	}
}
