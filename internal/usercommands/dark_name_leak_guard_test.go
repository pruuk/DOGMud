package usercommands

// dark_name_leak_guard_test.go — guards the 2026-08-31 dark-room name leak.
//
// THE BUG THIS GUARDS. When a player entered a room and their Search beat a
// hidden mob's stealth, go.go announced the result with a bare
//
//	user.SendText(... `You notice <ansi fg="mobname">%s</ansi> lurking in the
//	shadows!` ...)
//
// which has no sight gate on it at all. In an unlit room with no nightvision
// the player learned the exact name of a creature they could not see. The
// hidden-PLAYER branch three lines above had the same defect, and the room
// broadcast went out on room.SendText — the AUDIO channel, which by its own
// docstring "bypasses sight gate + anonymize" — so every bystander read both
// names too, blind or not.
//
// Forty lines further down the SAME function the "notices you as you enter"
// line already got this right with an inline lit-or-nightvision check. That is
// what makes this a drift bug rather than an oversight: one site knew the rule
// and its neighbour did not.
//
// THE RULE. A player-facing line that names an actor must be reachable only
// when the reader can actually see that actor. There are exactly two ways to
// satisfy it, and this guard accepts both:
//
//  1. the line sits inside an `if` guarded by messaging.CanSeeClearly, or
//  2. the line is delivered by SendTextVisual, which runs the per-recipient
//     sight gate and the anonymizer for us.
//
// Anything else fails. The guard also fails if it finds FEWER than the known
// number of name-tagged lines, because a walk that silently matches nothing
// would otherwise pass forever — the lesson from the U6b site guards.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// nameTagged reports whether a literal names an actor via an ANSI identity tag.
func nameTagged(s string) bool {
	return strings.Contains(s, `fg="mobname"`) || strings.Contains(s, `fg="username"`)
}

// posRange is a half-open source span that makes a literal "protected".
type posRange struct {
	from, to token.Pos
	why      string
}

func TestHiddenDetectionNeverNamesWhatYouCannotSee(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "go.go", nil, parser.ParseComments)
	require.NoError(t, err, "go.go must parse")

	// Pass 1 — collect the spans inside which naming a creature is legitimate.
	var protected []posRange
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {

		case *ast.IfStmt:
			// An `if` whose condition consults the sight predicate protects
			// its body. We read the condition off the source text rather than
			// pattern-matching the AST so a rename of the receiver or an added
			// negation still has to be looked at by a human.
			var cond strings.Builder
			ast.Inspect(node.Cond, func(c ast.Node) bool {
				if id, ok := c.(*ast.Ident); ok {
					cond.WriteString(id.Name)
					cond.WriteString(".")
				}
				return true
			})
			if strings.Contains(cond.String(), "CanSeeClearly") {
				protected = append(protected, posRange{
					from: node.Body.Pos(), to: node.Body.End(),
					why: "inside a CanSeeClearly branch",
				})
			}

		case *ast.CallExpr:
			// SendTextVisual / SendTextVisualToUser run the sight gate and the
			// anonymizer per recipient, so their arguments are already safe.
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if strings.HasPrefix(sel.Sel.Name, "SendTextVisual") {
				protected = append(protected, posRange{
					from: node.Pos(), to: node.End(),
					why: "delivered by " + sel.Sel.Name,
				})
			}
		}
		return true
	})

	require.NotEmpty(t, protected,
		"the walk found no protected spans at all — it is broken, not clean")

	// Pass 2 — every name-tagged "shadows" line must fall inside one of them.
	found := 0
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if !strings.Contains(strings.ToLower(lit.Value), "shadows") {
			return true
		}
		if !nameTagged(lit.Value) {
			return true // an anonymous fallback line — that IS the fix
		}
		found++

		for _, p := range protected {
			if lit.Pos() >= p.from && lit.End() <= p.to {
				return true
			}
		}
		t.Errorf(
			"%s: this line names an actor but is not sight-gated.\n"+
				"  %s\n"+
				"Put it inside an `if messaging.CanSeeClearly(reader, room)` branch\n"+
				"with an unnamed fallback, or deliver it with SendTextVisual.",
			fset.Position(lit.Pos()), lit.Value)
		return true
	})

	// Both directions. Three such lines exist today: the hidden-player notice,
	// the hidden-mob notice, and the room broadcast. If a rename or a refactor
	// makes this walk match nothing, that is a broken guard, not a clean file.
	require.GreaterOrEqual(t, found, 3,
		"expected at least 3 name-tagged stealth lines in go.go, found %d — "+
			"the walk is no longer finding them", found)
}
