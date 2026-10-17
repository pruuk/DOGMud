package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The two doors this guard keeps apart, pinned so the compiler checks them.
var (
	_ func(messaging.Category, string) = (*users.UserRecord)(nil).SendText
	_ func(messaging.Category)         = (*users.UserRecord)(nil).SendBlankLine
)

// #610: since the categorized pipeline (T9, May 2026) an empty SendText
// delivers nothing, so a message that comes out empty at runtime stays
// silent. A deliberate blank line is a different thing and has its own door,
// UserRecord.SendBlankLine (and actions.Actor.SendBlankLine, which forwards to
// it). These guards keep the two apart:
//
//   - TestNoLiteralEmptySendText: no non-test Go passes a literal empty text
//     ("" or ``) to any SendText. Such a call can only mean a blank line, and
//     it delivers nothing.
//   - TestBlankLineSendsAreRecorded: every SendBlankLine call is counted per
//     file against blankLineSendAllowlist, and its category must be a literal
//     screen category (messaging.BlankLineAllowed). A new call, a removed
//     call or a moved one fails until the map is updated with a reason.

// blankLineSendAllowlist records every file that sends a deliberate blank
// line, how many calls it makes, and why. When a call is added or removed,
// update the count and keep the reason true. The calls came from the 123
// literal empty SendText calls #610 converted (113 moved here, the 10 that
// framed a one-line refusal were removed, as every sibling refusal is bare).
var blankLineSendAllowlist = map[string]struct {
	count  int
	reason string
}{
	"internal/actions/actor_user.go":                 {1, "UserActor forwards Actor.SendBlankLine to UserRecord.SendBlankLine; its category is the caller's"},
	"internal/actions/scan.go":                       {2, "scan screen: a blank line under the heading and after the last direction"},
	"internal/hooks/Looking_HandleLookTips.go":       {1, "look screen: the blank line under the merchant tip (it rode CategoryTip, now the look screen's System)"},
	"internal/usercommands/admin.item.go":            {6, "admin item create and list screens"},
	"internal/usercommands/admin.mob.go":             {2, "admin mob list screen"},
	"internal/usercommands/admin.mudmail.go":         {2, "admin mudmail screen"},
	"internal/usercommands/admin.room.dispatcher.go": {3, "admin room set screen"},
	"internal/usercommands/admin.room.exits.go":      {5, "admin room exit editor screen"},
	"internal/usercommands/admin.room.go":            {10, "admin room container, recipe and mutator editor screens"},
	"internal/usercommands/admin.server.go":          {8, "admin server and server config screens"},
	"internal/usercommands/admin.setmotd.go":         {1, "admin setmotd screen"},
	"internal/usercommands/admin.zone.go":            {4, "admin zone and zone edit screens"},
	"internal/usercommands/alias.go":                 {1, "alias list screen"},
	"internal/usercommands/bank.go":                  {2, "bank screen"},
	"internal/usercommands/character.go":             {4, "character menu, view, change and delete screens"},
	"internal/usercommands/craft.go":                 {10, "craft list and craftable-now screens"},
	"internal/usercommands/inbox.go":                 {2, "inbox screen"},
	"internal/usercommands/look.go":                  {17, "look screens: room, floor item, container contents and target"},
	"internal/usercommands/macros.go":                {2, "macros list screen"},
	"internal/usercommands/mutations.go":             {3, "mutations screen"},
	"internal/usercommands/picklock.go":              {9, "picklock screen: the lock display and its result"},
	"internal/usercommands/pvp.go":                   {2, "pvp status screen"},
	"internal/usercommands/set.go":                   {12, "set status and set fprompt screens"},
	"internal/usercommands/setdesc.go":               {1, "setdesc screen"},
	"internal/usercommands/sethome.go":               {1, "sethome screen"},
	"internal/usercommands/start.go":                 {1, "start screen"},
	"internal/usercommands/use.go":                   {2, "use screen: the blank lines around what a container produces"},
}

// blankLineForwarders are the files whose SendBlankLine call passes its own
// parameter through rather than a literal category.
var blankLineForwarders = map[string]bool{
	"internal/actions/actor_user.go": true,
}

// blankLineCategoryNames maps the literal a call site writes to the category
// it names, so the guard reads the same set messaging.BlankLineAllowed holds.
var blankLineCategoryNames = map[string]messaging.Category{
	"CategorySystem":          messaging.CategorySystem,
	"CategoryScreen":          messaging.CategoryScreen,
	"CategoryAdmin":           messaging.CategoryAdmin,
	"CategoryRoomDescription": messaging.CategoryRoomDescription,
}

// blankLineScanRoots are the trees scanned; the repo root's own non-test
// files are scanned too.
var blankLineScanRoots = []string{"internal", "modules", "cmd"}

type goCall struct {
	file string
	line int
	call *ast.CallExpr
	name string
}

// walkGoCalls parses every non-test Go file under the scan roots and the
// repo root, and returns each call to a method or function named in names.
func walkGoCalls(t *testing.T, names map[string]bool) (calls []goCall, files int) {
	t.Helper()
	fset := token.NewFileSet()
	visit := func(path string) {
		src, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files++
		rel := filepath.ToSlash(path)
		ast.Inspect(f, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := ce.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			}
			if names[name] {
				calls = append(calls, goCall{file: rel, line: fset.Position(ce.Pos()).Line, call: ce, name: name})
			}
			return true
		})
	}
	isSource := func(name string) bool {
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}
	for _, root := range blankLineScanRoots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !info.IsDir() && isSource(path) {
				visit(path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && isSource(e.Name()) {
			visit(e.Name())
		}
	}
	return calls, files
}

func isEmptyStringLit(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	s, err := strconv.Unquote(lit.Value)
	return err == nil && s == ""
}

// TestNoLiteralEmptySendText fails on any SendText whose text is a literal
// empty string. It delivers nothing; a blank line is SendBlankLine.
func TestNoLiteralEmptySendText(t *testing.T) {
	calls, files := walkGoCalls(t, map[string]bool{"SendText": true})
	if files < 100 || len(calls) < 100 {
		t.Fatalf("scanned %d files and found %d SendText calls; the walk is not reaching the tree, so a clean result proves nothing", files, len(calls))
	}
	var problems []string
	for _, c := range calls {
		if len(c.call.Args) >= 2 && isEmptyStringLit(c.call.Args[1]) {
			problems = append(problems, fmt.Sprintf("%s:%d", c.file, c.line))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		t.Errorf("SendText with a literal empty text delivers nothing (#610). For a deliberate blank line call SendBlankLine(<screen category>) and record it in blankLineSendAllowlist:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestBlankLineSendsAreRecorded holds every SendBlankLine call to the
// per-file allowlist and to a literal screen category.
func TestBlankLineSendsAreRecorded(t *testing.T) {
	for name, cat := range blankLineCategoryNames {
		if !messaging.BlankLineAllowed(cat) {
			t.Errorf("blankLineCategoryNames lists %s, which messaging.BlankLineAllowed refuses", name)
		}
	}

	calls, files := walkGoCalls(t, map[string]bool{"SendBlankLine": true})
	if files < 100 {
		t.Fatalf("scanned only %d files; the walk is not reaching the tree", files)
	}

	var problems []string
	counts := map[string]int{}
	for _, c := range calls {
		counts[c.file]++
		if len(c.call.Args) != 1 {
			problems = append(problems, fmt.Sprintf("%s:%d: SendBlankLine takes one category", c.file, c.line))
			continue
		}
		if blankLineForwarders[c.file] {
			continue
		}
		sel, ok := c.call.Args[0].(*ast.SelectorExpr)
		if ok {
			pkg, isIdent := sel.X.(*ast.Ident)
			ok = isIdent && pkg.Name == "messaging"
		}
		if !ok {
			problems = append(problems, fmt.Sprintf("%s:%d: SendBlankLine must name its category literally (messaging.CategoryX)", c.file, c.line))
			continue
		}
		if _, allowed := blankLineCategoryNames[sel.Sel.Name]; !allowed {
			problems = append(problems, fmt.Sprintf("%s:%d: messaging.%s is not a screen category; a blank line rides only %s", c.file, c.line, sel.Sel.Name, allowedBlankLineNames()))
		}
	}

	for file, n := range counts {
		want, ok := blankLineSendAllowlist[file]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: %d SendBlankLine call(s) not recorded in blankLineSendAllowlist; add the file with its count and the screen it spaces", file, n))
			continue
		}
		if want.count != n {
			problems = append(problems, fmt.Sprintf("%s: %d SendBlankLine call(s), blankLineSendAllowlist records %d; update the count and keep the reason true", file, n, want.count))
		}
	}
	for file := range blankLineSendAllowlist {
		if counts[file] == 0 {
			problems = append(problems, fmt.Sprintf("%s: recorded in blankLineSendAllowlist but sends no blank line; remove the entry", file))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Errorf("blank-line sends drifted from the record (#610):\n  %s", strings.Join(problems, "\n  "))
	}
}

func allowedBlankLineNames() string {
	var names []string
	for n := range blankLineCategoryNames {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
