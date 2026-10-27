package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// commandEventFile is the one file allowed to raise the quest "command" event
// by hand: questengine.NotifyCommand builds every command event's target
// fields (#375 phase 3b slice 2). A hand-built Notify("command", ...) skips
// them, and a quest step that names a target would never fire there.
const commandEventFile = "internal/questengine/command_event.go"

// isCommandNotify reports whether call is X.Notify("command", ...) or
// X.NotifyAll("command", ...) with the literal event name.
func isCommandNotify(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Notify" && sel.Sel.Name != "NotifyAll") || len(call.Args) == 0 {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && lit.Value == `"command"`
}

func TestCommandEventsOnlyThroughNotifyCommand(t *testing.T) {
	calls, _ := walkGoCalls(t, map[string]bool{"Notify": true, "NotifyAll": true})
	seenHelper := false
	var violations []string
	for _, c := range calls {
		if !isCommandNotify(c.call) {
			continue
		}
		if c.file == commandEventFile {
			seenHelper = true
			continue
		}
		violations = append(violations, c.file+":"+strconv.Itoa(c.line))
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("quest command event raised by hand (use questengine.NotifyCommand with the command's target):\n  %s",
			strings.Join(violations, "\n  "))
	}
	if !seenHelper {
		t.Fatalf("the scan never saw %s raise the command event: the guard cannot find calls", commandEventFile)
	}
}

// TestCommandEventGuardCanFail proves the matcher flags a hand-built call and
// ignores other events.
func TestCommandEventGuardCanFail(t *testing.T) {
	for src, want := range map[string]bool{
		`questengine.GetEngine().Notify("command", d, b, b)`:        true,
		`e.NotifyAll("command", ds, b, b)`:                          true,
		`questengine.GetEngine().Notify("command_issued", d, b, b)`: false,
		`questengine.GetEngine().Notify("room_enter", d, b, b)`:     false,
	} {
		expr, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		if got := isCommandNotify(expr.(*ast.CallExpr)); got != want {
			t.Errorf("isCommandNotify(%s) = %v, want %v", src, got, want)
		}
	}
}
