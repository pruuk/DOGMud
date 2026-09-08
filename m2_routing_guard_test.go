package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// M2 routing freeze
//
// WHY THIS EXISTS, AND WHY THE LITERAL FREEZE IS NOT ENOUGH.
//
// A blind adversarial review of TestM2LiteralsAreFrozen on 2026-09-08 showed it
// catches text being EDITED and is blind to text being MISROUTED. Swapping the
// actor and actee lines of one branch in usercommands/bash.go is a two-line
// diff that compiles, vets, gofmts and passes every other guard in this repo:
// the multiset of literals is unchanged, so the fingerprint cannot move. The
// player who was knocked down then reads "Your shield bash knocks Grix to the
// ground!"
//
// That is not hypothetical. M2 moves each event's three lines from three
// separate SendText calls onto ADJACENT FIELDS OF ONE COMPOSITE LITERAL that
// differ only by role, so a transposition is materially easier to write after
// the refactor than it was before it.
//
// This freeze therefore records the PAIRING rather than the vocabulary: one
// record per narration send, carrying the recipient's ROLE, the
// messaging.Category, and a signature of the text expression. The baseline is
// taken from the PRE-M2 tree, so a migrated file must still produce an
// identical set. It also covers three things nothing else does:
//
//   - Category swaps. A Category is a Go identifier, not a string literal, so
//     the literal freeze cannot see one. A category decides the line's colour
//     (messaging/pipeline.go applyCategoryColor) and whether a light-verbosity
//     player sees the line at all (messaging/verbosity.go allowlists).
//   - A send deleted while its message pool is hoisted to a package-level var,
//     which leaves the literal multiset identical and still compiles.
//   - The MOB SIDE, which TestNarrationSitesMatchViewpointAudit structurally
//     cannot cover: narrationCandidateEvent requires an actor call, and mob
//     files have none, because a mob has no client.
//
// The golden is PLAIN TEXT on purpose. A re-recorded SHA-256 shows a reviewer
// nothing about which line moved, or whether a second line moved with it.
//
// TO RE-RECORD, only for a deliberate routing change and in the same commit:
//
//	M2_RECORD_ROUTING=1 go test . -run TestM2RoutingIsFrozen
//
// TO RECORD FROM ANOTHER TREE, which is how the pre-M2 baseline was taken:
//
//	M2_ROUTING_SRC=<dir> M2_RECORD_ROUTING=1 go test . -run TestM2RoutingIsFrozen
//
// ---------------------------------------------------------------------------

const m2RoutingGoldenPath = "testdata/m2-routing.golden"

// m2RoutingFiles is every file M2 migrates onto messaging.SendTrio.
//
// charge.go and hamstring.go are not in the M2 design's file list and were not
// in the literal freeze either. They belong: both are mob special-move verbs
// with the same actee-send plus room-broadcast shape, and both call the defence
// helper M2 rewrites, so M2's accepted room-line delivery change reaches them.
var m2RoutingFiles = []string{
	"internal/usercommands/bash.go",
	"internal/usercommands/drain.go",
	"internal/usercommands/gore.go",
	"internal/usercommands/grapple.go",
	"internal/usercommands/kick.go",
	"internal/usercommands/maul.go",
	"internal/usercommands/pounce.go",
	"internal/usercommands/rake.go",
	"internal/usercommands/shoot.go",
	"internal/usercommands/throttle.go",
	"internal/usercommands/throw.go",
	"internal/usercommands/trip.go",
	"internal/mobcommands/bash.go",
	"internal/mobcommands/charge.go",
	"internal/mobcommands/drain.go",
	"internal/mobcommands/gore.go",
	"internal/mobcommands/grapple.go",
	"internal/mobcommands/hamstring.go",
	"internal/mobcommands/kick.go",
	"internal/mobcommands/maul.go",
	"internal/mobcommands/pounce.go",
	"internal/mobcommands/rake.go",
	"internal/mobcommands/shoot.go",
	"internal/mobcommands/throttle.go",
	"internal/mobcommands/trip.go",
}

// m2SendRole maps a send's receiver identifier to the viewpoint it addresses.
//
// Derived by enumerating every receiver across the 25 files above on
// 2026-09-08, not guessed: user (133 sends), targetChar (77), targetUser (70),
// u (5, the target in mobcommands/shoot.go), p (5, the actee in
// usercommands/shoot.go, which is the variable name that hid that file from
// M1's scanner), room (101) and tr (4, the TARGET room of a cross-room shot,
// a genuinely different audience from the shooter's own room).
//
// An unrecognised receiver is recorded under its own name rather than dropped,
// so a new one surfaces as drift instead of silently leaving coverage.
func m2SendRole(name string) string {
	switch name {
	case "user":
		return "actor"
	case "targetUser", "targetChar", "u", "p":
		return "actee"
	case "room":
		return "observer"
	case "tr":
		return "observer_targetroom"
	}
	return "unknown_" + name
}

// m2ExprSig renders a text or category expression as a signature of its
// identifiers and string literals.
//
// ORDERED, NOT SORTED, so transposing two Sprintf arguments is visible.
// Identifiers are included so that a pooled send -- fmt.Sprintf(hitMsgs[...])
// -- is keyed on the POOL NAME: swapping hitMsgs and hitTargetMsgs between two
// roles changes the signature even though neither pool's contents moved, which
// is a case the literal freeze cannot see by construction.
func m2ExprSig(e ast.Expr) string {
	if e == nil {
		return ""
	}
	var parts []string
	ast.Inspect(e, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			parts = append(parts, v.Name)
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				parts = append(parts, v.Value)
			}
		}
		return true
	})
	return strings.Join(parts, "~")
}

// m2RoutingLines returns one record per narration send in the file.
//
// messaging.NoLine is deliberately NOT recorded. It is the absence of a line,
// and an absence was equally unrecorded in the pre-M2 tree, so recording it
// would make every migration look like drift. A real line turned into NoLine
// still surfaces, as a record that has GONE.
func m2RoutingLines(relPath string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, relPath, src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	add := func(role string, cat, text ast.Expr) {
		out = append(out, relPath+" | "+role+" | "+m2ExprSig(cat)+" | "+m2ExprSig(text))
	}

	// Pre-pass: resolve locals holding a messaging.Say(...).
	//
	// WHY. A verb whose room line is conditional cannot inline it -- it has to
	// build the line, branch, then compose once -- so it writes
	// `observer := messaging.Say(...)`, reassigns in the branch, and passes the
	// VARIABLE as the Trio field. A walk that only understood a direct Say call
	// dropped those records silently, which is the same blindness this guard
	// exists to remove, wearing a different hat. Every assignment is recorded,
	// so a conditional line contributes one record per branch and all of them
	// must survive.
	//
	// Name collisions across functions in one file only ever ADD records, which
	// is safe: this guard fails on absence, never on abundance.
	sayOf := map[string][][2]ast.Expr{}
	recordSay := func(lhs ast.Expr, rhs ast.Expr) {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			return
		}
		call, ok := rhs.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Say" {
			return
		}
		sayOf[id.Name] = append(sayOf[id.Name], [2]ast.Expr{call.Args[0], call.Args[1]})
	}
	ast.Inspect(file, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i := range as.Lhs {
			if i < len(as.Rhs) {
				recordSay(as.Lhs[i], as.Rhs[i])
			}
		}
		return true
	})

	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CompositeLit:
			sel, ok := v.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Trio" {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "messaging" {
				return true
			}
			for _, elt := range v.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				role := strings.ToLower(key.Name)

				// A local holding one or more messaging.Say values: emit a
				// record for each, so a conditional line is covered in every
				// branch it can take.
				if id, ok := kv.Value.(*ast.Ident); ok {
					for _, pair := range sayOf[id.Name] {
						add(role, pair[0], pair[1])
					}
					continue
				}

				call, ok := kv.Value.(*ast.CallExpr)
				if !ok {
					// messaging.NoLine: a considered silence, not a send.
					continue
				}
				csel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || csel.Sel.Name != "Say" || len(call.Args) < 2 {
					// Some other expression entirely. Record it rather than
					// drop it: an unrecognised shape must be visible as drift,
					// never as silence.
					add(role, nil, kv.Value)
					continue
				}
				add(role, call.Args[0], call.Args[1])
			}
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "sendAudioRoomText" && len(v.Args) >= 5 {
				// (room, mob, cat, anonMsg, fullMsg, excluded...)
				add("observer", v.Args[2], v.Args[4])
				return true
			}
			sel, ok := v.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name != "SendText" && sel.Sel.Name != "SendTextVisual" {
				return true
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok || len(v.Args) < 2 {
				return true
			}
			add(m2SendRole(recv.Name), v.Args[0], v.Args[1])
		}
		return true
	})
	sort.Strings(out)
	return out, nil
}

func TestM2RoutingIsFrozen(t *testing.T) {
	srcDir := os.Getenv("M2_ROUTING_SRC")

	var got []string
	for _, rel := range m2RoutingFiles {
		readFrom := rel
		if srcDir != "" {
			readFrom = filepath.Join(srcDir, rel)
		}
		src, err := os.ReadFile(readFrom)
		if err != nil {
			t.Fatalf("read %s (test must run from the repo root): %v", readFrom, err)
		}
		lines, err := m2RoutingLines(rel, src)
		if err != nil {
			t.Fatalf("parse %s: %v", readFrom, err)
		}
		got = append(got, lines...)
	}
	sort.Strings(got)

	if os.Getenv("M2_RECORD_ROUTING") == "1" {
		if err := os.MkdirAll(filepath.Dir(m2RoutingGoldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(m2RoutingGoldenPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Skipf("recorded %d routing records to %s", len(got), m2RoutingGoldenPath)
	}

	raw, err := os.ReadFile(m2RoutingGoldenPath)
	if err != nil {
		t.Fatalf("read %s: %v (record it with M2_RECORD_ROUTING=1)", m2RoutingGoldenPath, err)
	}
	want := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")

	wantSet, gotSet := map[string]int{}, map[string]int{}
	for _, l := range want {
		wantSet[l]++
	}
	for _, l := range got {
		gotSet[l]++
	}
	var gone, added []string
	for l, n := range wantSet {
		if gotSet[l] < n {
			gone = append(gone, l)
		}
	}
	for l, n := range gotSet {
		if wantSet[l] < n {
			added = append(added, l)
		}
	}
	sort.Strings(gone)
	sort.Strings(added)

	if len(gone) > 0 || len(added) > 0 {
		t.Errorf("M2 narration routing changed: %d record(s) gone, %d new.\n\nGONE:\n  %s\n\nNEW:\n  %s\n\n"+
			"Each record is `file | role | category | text-signature`.\n"+
			"  - A GONE and a NEW record carrying the SAME text signature under DIFFERENT\n"+
			"    roles is a viewpoint transposition: the line is being delivered to the\n"+
			"    wrong person. This is the defect this guard exists for.\n"+
			"  - A pair differing only in the category column is a recategorisation, which\n"+
			"    changes the line's colour and whether a light-verbosity player sees it.\n"+
			"  - A GONE record with no NEW partner is a send that was lost.\n\n"+
			"Re-record ONLY when the routing change is deliberate, and in the same commit\n"+
			"as the change so the diff shows both:\n"+
			"  M2_RECORD_ROUTING=1 go test . -run TestM2RoutingIsFrozen",
			len(gone), len(added),
			strings.Join(gone, "\n  "), strings.Join(added, "\n  "))
	}
}

// TestM2FileListsAgree anchors the two M2 freezes to each other.
//
// WHY. The literal freeze's failure message hands you the new fingerprint, so
// "make it green" has two equally easy paths: paste the hash, or delete the
// row. Deleting a row silently removes a file from coverage and nothing
// noticed. Its sibling TestNarrationSitesMatchViewpointAudit has a stale-entry
// check and a warning against exactly this; the M2 freezes had neither.
//
// Neither list is authoritative over the other on purpose: a file dropped from
// EITHER shows up here, so a reviewer sees one deletion as two failures.
func TestM2FileListsAgree(t *testing.T) {
	routing := map[string]bool{}
	for _, p := range m2RoutingFiles {
		routing[p] = true
	}
	var onlyLiteral, onlyRouting []string
	for p := range m2FrozenFiles {
		if !routing[p] {
			onlyLiteral = append(onlyLiteral, p)
		}
	}
	for p := range routing {
		if _, ok := m2FrozenFiles[p]; !ok {
			onlyRouting = append(onlyRouting, p)
		}
	}
	sort.Strings(onlyLiteral)
	sort.Strings(onlyRouting)
	if len(onlyLiteral) > 0 || len(onlyRouting) > 0 {
		t.Errorf("the two M2 freeze file lists disagree.\n\nIn m2FrozenFiles only:\n  %s\n\nIn m2RoutingFiles only:\n  %s\n\n"+
			"Every file M2 migrates needs BOTH guards: the literal freeze for its text "+
			"and the routing freeze for which viewpoint that text reaches. If a file "+
			"genuinely belongs in neither, remove it from both and say why in the commit.",
			strings.Join(onlyLiteral, "\n  "), strings.Join(onlyRouting, "\n  "))
	}
}

// TestEveryAudienceLiteralPairsIdsWithRecipients closes the room-broadcast
// exclusion hole.
//
// SendTrio derives the room broadcast's exclusion list from Audience.ActorId
// and Audience.ActeeId. Omitting one is a silent defect: the excluded party
// receives the third-person room line on top of their own personal line. That
// was previously an explicit trailing argument to room.SendTextVisual, and is
// now a struct field that is easier to leave out and equally invisible to the
// text guards.
//
// A missing RECIPIENT is legitimate and not checked: the mob side has no Actor
// at all, and an actee that is a mob has no client. What is never legitimate is
// naming a recipient and not naming its id.
func TestEveryAudienceLiteralPairsIdsWithRecipients(t *testing.T) {
	pairs := map[string]string{"Actor": "ActorId", "Actee": "ActeeId"}
	var bad []string

	for _, root := range []string{"internal", "modules"} {
		fset := token.NewFileSet()
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return nil
			}
			ast.Inspect(file, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				sel, ok := cl.Type.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Audience" {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "messaging" {
					return true
				}
				named := map[string]bool{}
				for _, elt := range cl.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok {
							named[key.Name] = true
						}
					}
				}
				for recipient, id := range pairs {
					if named[recipient] && !named[id] {
						bad = append(bad, filepath.ToSlash(path)+":"+
							strconv.Itoa(fset.Position(cl.Pos()).Line)+
							"  names "+recipient+" but not "+id)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d messaging.Audience literal(s) name a recipient without its id:\n  %s\n\n"+
			"SendTrio builds the room broadcast's exclusion list from ActorId and "+
			"ActeeId. Omit one and that person receives the third-person room line "+
			"on top of their own personal line.",
			len(bad), strings.Join(bad, "\n  "))
	}
}
