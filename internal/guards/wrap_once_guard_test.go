package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// pipelineWrappedAfterM7 are the categories whose wrapping moved into the
// pipeline. A sender that wraps first leaves CRLF in the text and hands the
// pipeline a line it has already broken at some other width, so each one wraps
// once, there.
var pipelineWrappedAfterM7 = map[string]bool{
	"CategorySystem": true, "CategoryWarning": true, "CategoryError": true,
	"CategoryRefusal": true, "CategoryUsage": true, "CategoryGroup": true,
	"CategoryBroadcast": true,
}

// wrapOnceFuncs wrap prose. A layout that wraps its own inner column (motdBox)
// is deliberately absent: its lines fit the reader and pass the pipeline
// untouched.
var wrapOnceFuncs = map[string]bool{"SplitString": true, "SplitStringNL": true, "WrapAnsi": true}

// wrapOnceSends are the calls that take a category as their first argument.
var wrapOnceSends = map[string]bool{"SendText": true, "Say": true}

// wrapOnceFindings is every send in f that wraps its own text on one of the
// pipeline-wrapped categories. It reads the send's arguments, so a text wrapped
// on an earlier line and passed in a variable is not seen.
func wrapOnceFindings(fset *token.FileSet, f *ast.File) []string {
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		send, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !wrapOnceSends[send.Sel.Name] {
			return true
		}
		cat, ok := call.Args[0].(*ast.SelectorExpr)
		if !ok || !pipelineWrappedAfterM7[cat.Sel.Name] {
			return true
		}
		for _, arg := range call.Args[1:] {
			ast.Inspect(arg, func(a ast.Node) bool {
				inner, ok := a.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := inner.Fun.(*ast.SelectorExpr); ok && wrapOnceFuncs[sel.Sel.Name] {
					found = append(found, wrapExprString(fset, cat)+" "+sel.Sel.Name)
				}
				return true
			})
		}
		return true
	})
	return found
}

// TestPipelineWrappedCategoriesAreWrappedOnce fails a sender that wraps text on
// a category the pipeline already wraps.
func TestPipelineWrappedCategoriesAreWrappedOnce(t *testing.T) {
	fset, files, _ := wrapGuardParse(t)
	var bad []string
	for _, gf := range files {
		for _, finding := range wrapOnceFindings(fset, gf.file) {
			bad = append(bad, gf.rel+": "+finding)
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("these senders wrap text the pipeline wraps; drop the wrap:\n%s", strings.Join(bad, "\n"))
	}
}

// TestWrapOnceGuard_CanFail is the null probe.
func TestWrapOnceGuard_CanFail(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"SplitStringNL on System", `func f(u U) { u.SendText(messaging.CategorySystem, util.SplitStringNL(s, 80)) }`, 1},
		{"WrapAnsi on Warning", `func f(u U) { u.SendText(messaging.CategoryWarning, messaging.WrapAnsi(s, w)) }`, 1},
		{"inside a concatenation", `func f(u U) { u.SendText(messaging.CategorySystem, term.CRLFStr+messaging.WrapAnsi(s, w)) }`, 1},
		{"a Trio line", `func f() { _ = messaging.Say(messaging.CategorySystem, messaging.WrapAnsi(s, w)) }`, 1},
		{"a whisper keeps its wrap", `func f(u U) { u.SendText(messaging.CategoryWhisper, util.SplitStringNL(s, w)) }`, 0},
		{"a box lays out its own lines", `func f(u U) { u.SendText(messaging.CategoryBroadcast, motdBox(s, w)) }`, 0},
		{"plain text", `func f(u U) { u.SendText(messaging.CategorySystem, s) }`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "probe.go", "package p\n"+tc.body+"\n", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := wrapOnceFindings(fset, f); len(got) != tc.want {
				t.Fatalf("findings %q, want %d", got, tc.want)
			}
		})
	}
}
