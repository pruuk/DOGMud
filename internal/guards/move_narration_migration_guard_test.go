package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
)

// ---------------------------------------------------------------------------
// Task 12 (messaging M4e-1): the guards that keep the special-move migration
// from silently rotting.
//
// Fourteen internal/mobcommands special-move files (bash, charge, drain,
// gore, grapple, hamstring, kick, maul, pounce, rake, shoot, smother,
// throttle, trip)
// now read every line they narrate from
// _datafiles/world/dogmud/narration/special-moves/*.yaml through
// internal/movenarration, via the sendMoveEvent/renderMoveEvent helpers in
// internal/mobcommands/move_narration.go and, for grapple's two
// internal/combat-owned events (crit_failure, disarm), renderGrappleEvent in
// internal/combat/grapple_narration.go.
//
// Three things can rot without anyone noticing:
//   - a literal creeping back into a "migrated" file (TestMigratedFilesHold-
//     NoNarrationLiterals);
//   - a Go call site and the YAML drifting apart on which (verb, event)
//     pairs exist, in EITHER direction (TestMoveEventKeysAgree);
//   - an event authoring a role set the reader doesn't expect, since
//     narration.ValidateVariants SKIPS an empty pool rather than failing on
//     it, so an event that quietly stops authoring an entire role validates
//     clean (TestMoveEventRoleSetsAgree).
// ---------------------------------------------------------------------------

// migratedNarrationLiteralFiles is the fourteen mob special-move files whose
// wording moved to the movenarration store. This is deliberately the same
// file list that came OUT of m2RoutingFiles and m2FrozenFiles as each was
// migrated (see those vars' comments in m2_routing_guard_test.go and
// messaging_surface_guard_test.go) -- this guard picks up exactly where the
// frozen fingerprint left off, on the same files, with a stronger claim.
var migratedNarrationLiteralFiles = []string{
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
	"internal/mobcommands/smother.go",
	"internal/mobcommands/throttle.go",
	"internal/mobcommands/trip.go",
	// The twelve player twins, migrated by M4e PR 1b. With these added the
	// list is the complete 25-file special-move surface, which is what let
	// TestM2LiteralsAreFrozen be deleted: a fingerprint saying "these
	// literals have not changed" is strictly weaker than this guard saying
	// "there are no literals".
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
}

// migratedNarrationLiteralAllowlist names a "path|literal text" pair this
// guard may not flag despite the literal looking like prose.
//
// LEFT EMPTY ON PURPOSE, AND MUST STAY THAT WAY ABSENT A GENUINE STRUCTURAL
// EXCEPTION. A guard whose allowlist is populated the day it is written is a
// guard that has already stopped doing its job: the entire point of
// graduating from a frozen fingerprint (TestM2LiteralsAreFrozen) to this
// guard is that the thirteen files carry no narration literal at all, not
// "no narration literal except the ones nobody got around to fixing".
var migratedNarrationLiteralAllowlist = map[string]bool{}

// sentencePunct and pureWordToken back isProseLiteral, which decides whether
// a backtick raw-string literal found in a migrated file is narration
// wording rather than a structural fragment the file is allowed to keep: an
// ansi tag fragment, an event key, a verb name, or a struct tag.
//
// THE RULE: a literal counts as prose if it carries sentence-ending
// punctuation (., !, or ?) ANYWHERE, OR if it contains two or more
// whitespace-separated tokens that are themselves nothing but letters or
// apostrophes -- i.e., at least two real words sitting next to each other.
//
// WHAT IT CATCHES: `You feel kicked.` (a period); `a figure` (two bare-word
// tokens with nothing else on them).
//
// WHAT IT DELIBERATELY PERMITS, and why the rule lets each one through:
//   - `<ansi fg="mobname">%s</ansi>` -- splits on its one space into
//     "<ansi" and `fg="mobname">%s</ansi>`; neither token is PURELY letters
//     (one starts with <, the other carries =, ", %, >), so the word-count
//     branch never reaches 2, and there is no ., ! or ? anywhere in it.
//   - a bare verb or event-key literal such as `kick` or `standard_hit` --
//     one token, no punctuation, word-count never reaches 2.
//   - a struct tag such as `yaml:"actor"` -- one token (no space at all),
//     and even split it carries non-letter characters throughout.
//
// WHAT IT CANNOT CATCH: a single bare word with no punctuation and no
// companion word on the same literal. shoot.go assigns the backtick literal
// Someone to its sneaking-shooter stand-in (actor = the word Someone,
// anonymizing a sneaking shooter's display name) and this rule does not flag
// it, because one lone word is indistinguishable from a verb literal like
// kick without also flagging every legitimate one-word fragment in the file.
// This is reported as a finding in the task report rather than papered over
// with a rule that also catches kick.
var sentencePunct = regexp.MustCompile(`[.!?]`)
var pureWordToken = regexp.MustCompile(`^[A-Za-z']+$`)

func isProseLiteral(lit string) bool {
	if sentencePunct.MatchString(lit) {
		return true
	}
	words := 0
	for _, tok := range strings.Fields(lit) {
		if pureWordToken.MatchString(tok) {
			words++
			if words >= 2 {
				return true
			}
		}
	}
	return false
}

// refusalLiteralPositions returns the position of every string literal that
// sits inside a `.SendText(...)` call.
//
// 🔑 WHY THIS EXISTS. The guard's claim is "no NARRATION literal remains",
// but isProseLiteral can only see "this looks like prose". For the thirteen
// MOB files those coincide, because a mob is never refused anything. The
// twelve PLAYER files are different: they legitimately carry refusals and
// mechanical notes ("You don't have a ranged weapon equipped.", "Could not
// find your target.", "You can't see well enough to aim."), which reach a
// player through a direct user.SendText and are GROUP A, owned by M7, not by
// this arc stage. Flagging them would be demanding a migration this slice was
// never scoped to do.
//
// The discriminator is the DELIVERY PATH, which is exactly the distinction
// that matters: narration goes out through SendTrio and the store, a refusal
// goes out through SendText. Scoping by path rather than by wording is what
// keeps this from becoming a guard with an allowlist full of excuses.
func refusalLiteralPositions(file *ast.File) map[token.Pos]bool {
	marked := map[token.Pos]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fn.Sel != nil {
				name = fn.Sel.Name
			}
		case *ast.Ident:
			name = fn.Name
		}
		// SendText is the refusal/mechanical-note delivery path. A `refuse...`
		// helper is the same thing behind a wrapper: throw.go's
		// refuseWhileBusy(user, `throw anything`) takes a sentence FRAGMENT
		// that it completes into a refusal, and the fragment is not narration.
		if name != "SendText" && !strings.HasPrefix(name, "refuse") {
			return true
		}
		// Mark every literal in this call's subtree, so a literal wrapped in
		// fmt.Sprintf inside the SendText is marked too.
		ast.Inspect(call, func(inner ast.Node) bool {
			if lit, ok := inner.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				marked[lit.Pos()] = true
			}
			return true
		})
		return true
	})
	return marked
}

// TestMigratedFilesHoldNoNarrationLiterals asserts each migrated file holds no
// backtick-quoted prose literal on a NARRATION path, per isProseLiteral's rule
// above and refusalLiteralPositions' scoping. See
// migratedNarrationLiteralAllowlist for why that map must stay empty.
func TestMigratedFilesHoldNoNarrationLiterals(t *testing.T) {
	for _, path := range migratedNarrationLiteralFiles {
		path := path
		t.Run(path, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			refusals := refusalLiteralPositions(file)
			// Walk FUNCTION BODIES only. A package-level const declaration is
			// not a send site, and the ones in these files
			// (shoot.go's recoveryArrowsText, surpriseShotRevealedText,
			// aimedWhileEngagedText and friends) are one-audience mechanical
			// notes -- Group A, owned by M7. Narration reaches a player from
			// inside a function, through the store.
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					lit, ok := n.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING || !strings.HasPrefix(lit.Value, "`") {
						return true
					}
					text := strings.Trim(lit.Value, "`")
					if !isProseLiteral(text) {
						return true
					}
					// A refusal or mechanical note delivered by SendText is Group
					// A, owned by M7. Not this stage's to move.
					if refusals[lit.Pos()] {
						return true
					}
					if migratedNarrationLiteralAllowlist[path+"|"+text] {
						return true
					}
					pos := fset.Position(lit.Pos())
					t.Errorf("%s:%d: narration-looking backtick literal remains in a migrated file: %q", pos.Filename, pos.Line, text)
					return true
				})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Guard 2: keys AND roles agree, in both directions.
// ---------------------------------------------------------------------------

// moveEventPair is one (verb, event) combination named either by a Go call
// site or by the shipped YAML store.
type moveEventPair struct {
	Verb  string
	Event string
}

// moveEventScanDirs is where sendMoveEvent/renderMoveEvent/renderGrappleEvent
// call sites live: the mob special-move commands themselves, and
// internal/combat, which owns grapple's crit_failure and disarm events (see
// internal/combat/grapple_narration.go's own comment on why renderGrappleEvent
// is not just a call into mobcommands.renderMoveEvent -- mobcommands already
// imports combat, so the reverse import would cycle).
//
// internal/usercommands joined M4e-1b: the player-side special-move files
// carry their own sendMoveEvent/renderMoveEvent pair
// (internal/usercommands/move_narration.go), naming event keys in the SAME
// store files this scan already covers. Since M6 slice 2 (ledger row 56) the
// mob and player call sites name the same keys and draw from one pool.
var moveEventScanDirs = []string{
	"internal/mobcommands",
	"internal/combat",
	"internal/usercommands",
}

// collectStringAssignments returns every string literal ever assigned to each
// local identifier in fn, keyed by identifier name.
//
// This exists because two of the thirteen files do not spell their event key
// as a literal at the call site: kick.go assigns a `var partialEvent
// movenarration.EventKey` across a switch statement's three cases, and
// trip.go assigns a `prefix` string across an if/else and then concatenates
// it with a literal suffix at the call site. Both are simple, one variable
// getting different literal values in different branches, so collecting
// every value ever assigned to a name (order does not matter; this is not a
// flow analysis, just "what values could this name ever hold") and letting
// resolveStringExpr combine them at the use site recovers every real event
// pair those two call sites can produce.
func collectStringAssignments(fn *ast.FuncDecl) map[string][]string {
	out := map[string][]string{}
	if fn.Body == nil {
		return out
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			if i >= len(as.Rhs) {
				continue
			}
			id, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			lit, ok := as.Rhs[i].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				continue
			}
			out[id.Name] = append(out[id.Name], s)
		}
		return true
	})
	return out
}

// resolveStringExpr returns every string value expr could evaluate to, given
// assigns (see collectStringAssignments). It understands a direct string
// literal, a local identifier, a single-argument type-conversion call such as
// movenarration.EventKey(x), and a string concatenation (x + y), recursing
// into both sides and returning the cross product -- which is exactly what
// trip.go's `movenarration.EventKey(prefix+"knockdown")` needs: prefix
// resolves to ["trip_", "tailsweep_"], the suffix is the literal
// "knockdown", and the product is ["trip_knockdown", "tailsweep_knockdown"].
//
// Anything else (a function call whose result isn't one of the above, a
// struct field, ...) resolves to nothing, which is the conservative
// direction: a pair this cannot resolve is simply not added to the
// referenced set rather than guessed at.
func resolveStringExpr(e ast.Expr, assigns map[string][]string) []string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			if s, err := strconv.Unquote(v.Value); err == nil {
				return []string{s}
			}
		}
	case *ast.Ident:
		return assigns[v.Name]
	case *ast.CallExpr:
		if len(v.Args) == 1 {
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "EventKey" {
				return resolveStringExpr(v.Args[0], assigns)
			}
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			lefts := resolveStringExpr(v.X, assigns)
			rights := resolveStringExpr(v.Y, assigns)
			var out []string
			for _, l := range lefts {
				for _, r := range rights {
					out = append(out, l+r)
				}
			}
			return out
		}
	}
	return nil
}

// collectReferencedMoveEvents scans every non-test .go file directly inside
// each of dirs for sendMoveEvent/renderMoveEvent/renderGrappleEvent call
// sites and returns every (verb, event) pair they can name.
//
// renderGrappleEvent takes no verb argument (it only ever renders grapple's
// own two combat-owned events), so its calls are recorded under the literal
// verb "grapple".
func collectReferencedMoveEvents(t *testing.T, dirs []string) map[moveEventPair]bool {
	t.Helper()
	out := map[moveEventPair]bool{}
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatalf("globbing %s: %v", dir, err)
		}
		if len(matches) == 0 {
			t.Fatalf("no .go files found in %s; the scan cannot succeed", dir)
		}
		for _, path := range matches {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				assigns := collectStringAssignments(fn)
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					ident, ok := call.Fun.(*ast.Ident)
					if !ok {
						return true
					}
					switch ident.Name {
					// sendPartialMove (#631) takes verb and event in the
					// same two leading positions and sends through
					// sendMoveEvent.
					case "sendMoveEvent", "renderMoveEvent", "sendPartialMove":
						if len(call.Args) < 2 {
							return true
						}
						for _, verb := range resolveStringExpr(call.Args[0], assigns) {
							for _, event := range resolveStringExpr(call.Args[1], assigns) {
								out[moveEventPair{verb, event}] = true
							}
						}
					case "renderGrappleEvent":
						if len(call.Args) < 1 {
							return true
						}
						for _, event := range resolveStringExpr(call.Args[0], assigns) {
							out[moveEventPair{"grapple", event}] = true
						}
					}
					return true
				})
			}
		}
	}
	return out
}

// loadMoveNarrationGroups loads the shipped special-move store directly
// (never through movenarration.LoadMoveNarrationFiles / configs, since a test
// binary never reads config.yaml -- see shipped_narration_data_guard_test.go's
// own note on shippedWorldRoot for why that matters).
func loadMoveNarrationGroups(t *testing.T) map[string]*movenarration.MoveNarrationGroup {
	t.Helper()
	dir := filepath.Join(shippedWorldRoot, "narration", "special-moves")
	groups, err := fileloader.LoadAllFlatFiles[string, *movenarration.MoveNarrationGroup](dir)
	if err != nil {
		t.Fatalf("loading %s: %v", dir, err)
	}
	if len(groups) == 0 {
		t.Fatalf("no special-move files loaded from %s", dir)
	}
	return groups
}

// TestMoveEventKeysAgree proves the Go call sites and the shipped YAML name
// exactly the same set of (verb, event) pairs, in both directions.
//
// Direction 1 (referenced but not authored): a Go call site names an event
// the YAML does not declare. That move narrates NOTHING in play -- GetMove
// or Variants fails, renderMoveEvent logs and returns ok=false, and the
// player sees silence where a line should have been.
//
// Direction 2 (authored but not referenced): the YAML declares an event no
// Go call site ever names. That is dead wording nobody can reach.
func TestMoveEventKeysAgree(t *testing.T) {
	referenced := collectReferencedMoveEvents(t, moveEventScanDirs)
	if len(referenced) == 0 {
		t.Fatal("no call sites found; the regex/AST scan cannot succeed, so this test proves nothing")
	}

	groups := loadMoveNarrationGroups(t)
	authored := map[moveEventPair]bool{}
	for verb, g := range groups {
		for key := range g.Events {
			authored[moveEventPair{verb, string(key)}] = true
		}
	}
	if len(authored) == 0 {
		t.Fatal("no authored events found in the shipped store; the loader cannot succeed, so this test proves nothing")
	}

	var missingFromYAML, deadInYAML []string
	for pair := range referenced {
		if !authored[pair] {
			missingFromYAML = append(missingFromYAML, pair.Verb+"/"+pair.Event)
		}
	}
	for pair := range authored {
		if !referenced[pair] {
			deadInYAML = append(deadInYAML, pair.Verb+"/"+pair.Event)
		}
	}
	sort.Strings(missingFromYAML)
	sort.Strings(deadInYAML)

	for _, name := range missingFromYAML {
		t.Errorf("%s is referenced by a Go call site but the shipped store declares no such event; that move narrates nothing in play", name)
	}
	for _, name := range deadInYAML {
		t.Errorf("%s is authored in the shipped store but no Go call site ever names it; that wording is unreachable dead content", name)
	}
}

// ---------------------------------------------------------------------------
// Guard 3: role-set agreement -- the hole that matters most.
//
// narration.ValidateVariants SKIPS an empty pool (`if len(role.pool) == 0 {
// continue }`) rather than failing on it, so an event that omits an ENTIRE
// role validates clean and then tells that audience nothing. Deleting a
// whole observer: block was proven, earlier in this arc, to pass every
// existing guard.
//
// An absent role is not automatically a bug -- several are legitimate (a
// detail line that is private knowledge; throw has no single actee; shoot
// splits its narration across events that each own exactly one audience) --
// but the absence must be DECLARED, with a reason, rather than
// merely happening to be true today.
// ---------------------------------------------------------------------------

// intentionallySilentRoles is every "verb/event" this guard's default
// expectation (both actee and observer authored) does not hold for, with the
// reason read directly off the call site that sends it, not guessed from the
// YAML alone.
var intentionallySilentRoles = map[string]string{
	"shoot/surprise_hit": "actor only: the ambush shot's EXTRA line, read by the shooter alone. " +
		"The room and the target learn about the shot through fire_announce and " +
		"hit/partial/miss exactly as they do for an ordinary shot; this line " +
		"is what makes firing from concealment feel different to the person doing it, and giving " +
		"it a room line would tell everyone the shot came from cover.",
	"shoot/surprise_miss": "actor only: see shoot/surprise_hit.",
	"shoot/hit":           "actor and actee, no observer: the room's line for a same-room shot always comes from fire_announce's observer role, sent unconditionally before the outcome is known; hit/partial/miss carry only the shooter's own line (actor, read by a player shooter; a mob shooter has no client) and the target's own line (actee) (shoot.go's same-room branch, mob and player alike).",
	"shoot/partial":       "actor and actee, no observer: same as shoot/hit -- the room line is fire_announce's, not this event's.",
	"shoot/miss":          "actor and actee, no observer: same as shoot/hit -- the room line is fire_announce's, not this event's.",

	"shoot/fire_announce": "observer only: this is the room's line for a same-room shot, sent once regardless of outcome; the outcome itself is authored separately on hit/partial/miss's actor and actee roles (shoot.go's same-room branch). No actor role either -- the shooter's own line for this shot is the hit/partial/miss actor role, not a duplicate announce.",
	"shoot/fire_depart":   "observer only: the SHOOTER's own room sees the shot leave; the outcome is narrated to the TARGET's room by the arrival_* events instead, and the target got their own hit/partial/miss line already (shoot.go's cross-room branch).",

	"shoot/arrival_unknown_hit":     "remote_observer only: these six arrival_* events exist solely for the defender's-room audience on a cross-room shot. The shooter's room already got fire_depart and the target already got hit/partial/miss (shoot.go's cross-room arrival send).",
	"shoot/arrival_unknown_partial": "remote_observer only: see shoot/arrival_unknown_hit.",
	"shoot/arrival_unknown_miss":    "remote_observer only: see shoot/arrival_unknown_hit.",
	"shoot/arrival_known_hit":       "remote_observer only: see shoot/arrival_unknown_hit.",
	"shoot/arrival_known_partial":   "remote_observer only: see shoot/arrival_unknown_hit.",
	"shoot/arrival_known_miss":      "remote_observer only: see shoot/arrival_unknown_hit.",

	// Detail lines that are private knowledge, carried over from the M4e-1b
	// player-side shapes and renamed when the mob and player pools merged
	// (M6 slice 2, ledger row 56).
	"grapple/prone_penalty":   "actor only: private knowledge about the actor's own roll against an already-prone target (grapple.go's own comment) -- a room line here would invent an observation nobody in the room made, about a grapple the success event already narrated to them.",
	"grapple/defense_exposed": "actor only: private knowledge, same ruling as prone_penalty -- the actor alone learns their failed grapple left them exposed.",

	"throw/hurl":           "no actee: throw is an untargeted room AoE against every hostile present, so it has no single actee at all (throw.go's own comment: \"17 actor sends and zero actee sends... the one genuinely actee-less member of the special-move family\").",
	"throw/fumble":         "no actee: same as throw/hurl -- a fumble hits the thrower, not a chosen target.",
	"throw/cast_interrupt": "no actee: the interrupted party is a mob with no client; its name rides the actor and observer text as plain prose instead.",
	"throw/partial_hit":    "actor only: the room's line for a defended throw always comes from the channel defence triad (combat.RenderChannelDefenceMessages, sourced outside this store), and there is no actee -- see throw/hurl.",

	// M4e-1b Task 4b Step 2: drain's lifesteal detail lines. Private
	// knowledge under the detail-line ruling -- this reports the drainer's
	// own returning vigour to the drainer, not a world event -- so neither
	// carries an actee or an observer role, same shape as
	// grapple/prone_penalty above.
	"drain/hit_heal":     "actor only: private knowledge about the drainer's own returning vigour (drain.go's own comment) -- a room or target line here would invent an observation nobody made about a lifesteal the hit event already narrated as damage.",
	"drain/partial_heal": "actor only: private knowledge, same ruling as drain/hit_heal -- a partial drain that still lifesteals reports the trickle to the drainer alone.",
}

// TestMoveEventRoleSetsAgree asserts every authored event carries both an
// actee and an observer role UNLESS intentionallySilentRoles declares why
// not, and fails on a stale entry whose event authors the role after all (so
// the map cannot rot into a blanket exemption nobody re-checks).
//
// actor is deliberately not part of the default expectation: mob-only events
// (charge, hamstring, trip's whiff_on_prone) author no actor line, since a
// mob has no client, while every event a player can fire does author one.
func TestMoveEventRoleSetsAgree(t *testing.T) {
	groups := loadMoveNarrationGroups(t)

	seenExceptions := map[string]bool{}
	for verb, g := range groups {
		for key, ev := range g.Events {
			name := verb + "/" + string(key)
			hasActee := len(ev.Actee) > 0
			hasObserver := len(ev.Observer) > 0

			reason, excepted := intentionallySilentRoles[name]
			if excepted {
				seenExceptions[name] = true
			}

			if hasActee && hasObserver {
				if excepted {
					t.Errorf("%s: intentionallySilentRoles carries a stale entry (%q) -- this event now authors BOTH actee and observer, so the exception is no longer true; remove it", name, reason)
				}
				continue
			}

			if !excepted {
				missing := []string{}
				if !hasActee {
					missing = append(missing, "actee")
				}
				if !hasObserver {
					missing = append(missing, "observer")
				}
				t.Errorf("%s: authors no %s role and carries no intentionallySilentRoles entry explaining why; an undeclared silent role validates clean today (narration.ValidateVariants skips empty pools) and then tells that audience nothing", name, strings.Join(missing, "/"))
			}
		}
	}

	var stale []string
	for name := range intentionallySilentRoles {
		if !seenExceptions[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	for _, name := range stale {
		t.Errorf("intentionallySilentRoles names %q, which is not an event in the shipped store at all; remove the stale entry", name)
	}
}
