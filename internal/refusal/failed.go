package refusal

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// FailedText is what a player reads when the server failed them. action is
// a noun for what failed ("salvage"), or empty.
func FailedText(action string) string {
	if action == `` {
		return `That didn't work. The error has been logged.`
	}
	return `That ` + action + ` didn't work. The error has been logged.`
}

// Failed logs err with its cause and logArgs (pass "userId", id), then tells
// the player plainly on CategoryError (spec section 4). It is the one helper
// that does not send on CategoryRefusal: this is the server's fault, not a
// refusal of what the player asked.
func Failed(to Recipient, action string, err error, logArgs ...any) {
	_, file, line, _ := runtime.Caller(1)
	mudlog.Error(`refusal.Failed`, append([]any{`action`, action, `error`, err, `at`, fmt.Sprintf(`%s:%d`, filepath.Base(file), line)}, logArgs...)...)
	to.SendText(messaging.CategoryError, FailedText(action))
}
