# Messaging M2 — Shared Narration Seam — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build one `SendTrio` seam in `internal/messaging`, migrate all 23
special-move verb files and both defence helpers onto it, and close seven
narration defects.

**Architecture:** A narrated event is a `Trio` of three `Line`s (actor, actee,
observer), each carrying its own `messaging.Category`, delivered to an
`Audience` of two `Recipient`s and one `Broadcaster`. The seam lives in
`internal/messaging` because the import graph rules out both `internal/narration`
(it is imported by `items`, and `messaging` reaches `items` through
`characters`) and `internal/actions` (it imports `questengine`, which must be
able to call the seam in M3). Rendering, banding and token substitution are not
touched; they are M3's.

**Tech Stack:** Go. `go/ast` for the guard. No new dependencies.

**Spec:** [`../specs/2026-09-08-messaging-m2-shared-narration-seam-design.md`](../specs/2026-09-08-messaging-m2-shared-narration-seam-design.md).
Read section 3a before Task 2 and section 4 before Task 7.

---

## Things that will bite you

Read these before starting. Each has already cost someone time.

1. **The three roles do not share a `Category`.** Ten of the twelve player-side
   verbs send personal lines as `CategorySystem` and the room line as something
   verb-specific. `shoot` uses four personal categories; `throw` uses three on
   each side. **Never hoist a category out to the call.** Categories feed the
   verbosity suppression allowlists in `internal/messaging/verbosity.go`, so
   changing one changes what a player on medium or light verbosity sees.

2. **`UserRecord.SendText` is the audio channel** (`internal/users/userrecord.go:438`),
   hardcoded. The pipeline runs the sight gate and the anonymizer **only** on
   `ChannelVisual` (`internal/messaging/pipeline.go:59`). So a line sent with
   `SendText` is never gated and never anonymized. This is why the mob-side
   defence helper anonymizes by hand, and it is not redundant.

3. **Refusals are not events.** "You need a shield equipped", "Your target is
   gone!", cost refusals and cooldown notices stay plain `SendText`. If the
   message only answers "why did my command not work", leave it alone. If it
   would still make sense to somebody who was not the actor, it is an event and
   goes through `SendTrio`.

4. **The test binary's working directory is not reliably the package
   directory.** The root guard test asserts its roots exist and tells you to run
   from the repo root. Run `go test .` from the repo root for it.

5. **Do not run `git add -A`.** Add the files each step names.

6. **`gofmt -l internal/ modules/` must print nothing** before any commit. It
   has its own CI gate and has broken a push before.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/messaging/trio.go` | **Create.** `Line`, `Say`, `NoLine`, `Trio`, `Recipient`, `Broadcaster`, `Audience`, `SendTrio`. Nothing else. |
| `internal/messaging/trio_test.go` | **Create.** Unit tests for the seam with fake recipients. |
| `messaging_surface_guard_test.go` | **Modify.** Add the literal freeze (Task 0) and the `Trio` completeness guard (Task 1). Repo root, `package main`. |
| `internal/usercommands/skill_move_defence.go` | **Modify.** Send half moves to `SendTrio`; identity resolution stays. |
| `internal/mobcommands/skill_move_defence.go` | **Modify.** Same, and stops calling `sendAudioRoomText`. |
| `internal/usercommands/{12 verbs}.go` | **Modify.** One commit each. |
| `internal/mobcommands/{11 verbs}.go` | **Modify.** One commit each. |
| `internal/messaging/context.md` | **Modify.** Remove four phantom symbols, document the seam. |
| `internal/narration/context.md` | **Create.** The package has none. |
| `internal/banner/banner.go` | **Modify.** Comment points at a file that does not exist. |
| `docs/PATCH_NOTES.md` | **Modify.** Dated entry, player-facing framing, no raw numbers, no em dashes. |

---

## Task 0: Freeze the literals before anything moves

**Why this is first.** M1's goldens snapshot the message *stores*. None of them
touches a special-move verb. Without this the 23-file migration has no
automated protection at all.

**Files:**
- Modify: `messaging_surface_guard_test.go` (repo root, `package main`)

- [ ] **Step 1: Add the file list and the freeze test**

Append to `messaging_surface_guard_test.go`:

```go
// ---------------------------------------------------------------------------
// M2 literal freeze
//
// The messaging arc's M2 slice rewrites the 23 files below onto
// messaging.SendTrio. Their player-facing text is hand-rolled fmt.Sprintf
// literals, NOT a data store, so M1's goldens under
// internal/narration/testdata/stores/ do not cover a single line of it.
//
// This freezes the multiset of string literals in each file. A refactor that
// drops a line, alters a string, or reorders a pool changes the multiset and
// fails here. It says nothing about rendered output -- that is the honest
// limit, and it is the same tradeoff M1 made for its Group C inventory.
//
// Import declarations are skipped, because the migration legitimately adds an
// import to files that did not already have one.
//
// WHEN A FINGERPRINT LEGITIMATELY CHANGES (the seven output changes in the M2
// spec's section 4), update it IN THE SAME COMMIT as the text change, so the
// diff shows both together.
// ---------------------------------------------------------------------------

var m2FrozenFiles = map[string]string{
	"internal/usercommands/bash.go":                "",
	"internal/usercommands/drain.go":               "",
	"internal/usercommands/gore.go":                "",
	"internal/usercommands/kick.go":                "",
	"internal/usercommands/maul.go":                "",
	"internal/usercommands/pounce.go":              "",
	"internal/usercommands/rake.go":                "",
	"internal/usercommands/throttle.go":            "",
	"internal/usercommands/trip.go":                "",
	"internal/usercommands/grapple.go":             "",
	"internal/usercommands/shoot.go":               "",
	"internal/usercommands/throw.go":               "",
	"internal/usercommands/skill_move_defence.go":  "",
	"internal/mobcommands/bash.go":                 "",
	"internal/mobcommands/drain.go":                "",
	"internal/mobcommands/gore.go":                 "",
	"internal/mobcommands/kick.go":                 "",
	"internal/mobcommands/maul.go":                 "",
	"internal/mobcommands/pounce.go":               "",
	"internal/mobcommands/rake.go":                 "",
	"internal/mobcommands/throttle.go":             "",
	"internal/mobcommands/trip.go":                 "",
	"internal/mobcommands/grapple.go":              "",
	"internal/mobcommands/shoot.go":                "",
	"internal/mobcommands/skill_move_defence.go":   "",
}

// m2LiteralFingerprint returns a stable hash of every string literal in the
// file outside its import declarations.
func m2LiteralFingerprint(path string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", err
	}
	var lits []string
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if ok && bl.Kind == token.STRING {
				lits = append(lits, bl.Value)
			}
			return true
		})
	}
	sort.Strings(lits)
	sum := sha256.Sum256([]byte(strings.Join(lits, "\x00")))
	return hex.EncodeToString(sum[:]), nil
}

func TestM2LiteralsAreFrozen(t *testing.T) {
	var drift []string
	for path, want := range m2FrozenFiles {
		got, err := m2LiteralFingerprint(path)
		if err != nil {
			t.Fatalf("fingerprint %s (test must run from the repo root): %v", path, err)
		}
		if want == "" {
			drift = append(drift, path+"  RECORD: "+got)
			continue
		}
		if got != want {
			drift = append(drift, path+"\n    want "+want+"\n    got  "+got)
		}
	}
	sort.Strings(drift)
	if len(drift) > 0 {
		t.Errorf("%d M2-frozen file(s) have a different set of string literals "+
			"than recorded:\n  %s\n\n"+
			"During the M2 migration this means text was lost or altered by a "+
			"refactor that was supposed to move it unchanged -- find the "+
			"dropped or edited literal rather than re-recording the hash. "+
			"Re-record ONLY when the commit deliberately changes player-facing "+
			"text (the seven output changes in the M2 spec's section 4), and "+
			"do it in that same commit.",
			len(drift), strings.Join(drift, "\n  "))
	}
}
```

- [ ] **Step 2: Add the imports the test needs**

`messaging_surface_guard_test.go` already imports `go/ast`, `go/parser`,
`go/token`, `sort` and `strings`. Add `crypto/sha256` and `encoding/hex` to its
import block.

- [ ] **Step 3: Run it to collect the fingerprints**

Run from the repo root: `go test . -run TestM2LiteralsAreFrozen -v`

Expected: FAIL, listing every path with `RECORD: <hash>`. This is the test
being capable of failing, which is the point of running it before it passes.

- [ ] **Step 4: Paste each hash into `m2FrozenFiles`**

Replace each `""` with the hash the failure printed for that path.

- [ ] **Step 5: Verify it now passes**

Run: `go test . -run TestM2LiteralsAreFrozen -v`
Expected: PASS.

- [ ] **Step 6: Prove the net can actually catch something**

This project has had three null probes that could not have failed. Do not skip
this step.

Edit `internal/usercommands/bash.go` and change one word inside any player-facing
string (for example `strikes` to `strikez`). Run:
`go test . -run TestM2LiteralsAreFrozen`

Expected: FAIL naming `internal/usercommands/bash.go` with a want/got hash pair.

Revert the edit with `git checkout -- internal/usercommands/bash.go` and
re-run. Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/ modules/
git add messaging_surface_guard_test.go
git commit -m "test(messaging): freeze the literals M2 is about to move"
```

---

## Task 1: The seam

**Files:**
- Create: `internal/messaging/trio.go`
- Create: `internal/messaging/trio_test.go`
- Modify: `messaging_surface_guard_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/messaging/trio_test.go`:

```go
package messaging

import "testing"

type fakeRecipient struct {
	cats  []Category
	texts []string
}

func (f *fakeRecipient) SendText(cat Category, text string) {
	f.cats = append(f.cats, cat)
	f.texts = append(f.texts, text)
}

type fakeBroadcaster struct {
	calls int
	cat   Category
	text  string
	excl  []int
}

func (f *fakeBroadcaster) SendTextVisual(cat Category, txt string, excludeUserIds ...int) {
	f.calls++
	f.cat = cat
	f.text = txt
	f.excl = append([]int(nil), excludeUserIds...)
}

func TestSendTrioDeliversAllThreeWithTheirOwnCategories(t *testing.T) {
	actor, actee := &fakeRecipient{}, &fakeRecipient{}
	room := &fakeBroadcaster{}

	SendTrio(Trio{
		Actor:    Say(CategorySystem, "you hit"),
		Actee:    Say(CategorySystem, "you are hit"),
		Observer: Say(CategoryBash, "a hits b"),
	}, Audience{Actor: actor, ActorId: 7, Actee: actee, ActeeId: 9, Room: room})

	if len(actor.texts) != 1 || actor.texts[0] != "you hit" {
		t.Fatalf("actor got %v", actor.texts)
	}
	if len(actee.texts) != 1 || actee.texts[0] != "you are hit" {
		t.Fatalf("actee got %v", actee.texts)
	}
	if room.calls != 1 || room.text != "a hits b" {
		t.Fatalf("room got %d calls, %q", room.calls, room.text)
	}
	if actor.cats[0] != CategorySystem || room.cat != CategoryBash {
		t.Fatalf("categories not preserved per role: actor %v room %v", actor.cats[0], room.cat)
	}
}

func TestSendTrioExcludesActorAndActeeFromTheRoom(t *testing.T) {
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "a"),
		Actee:    Say(CategorySystem, "b"),
		Observer: Say(CategoryBash, "c"),
	}, Audience{Actor: &fakeRecipient{}, ActorId: 7, Actee: &fakeRecipient{}, ActeeId: 9, Room: room})

	if len(room.excl) != 2 || room.excl[0] != 7 || room.excl[1] != 9 {
		t.Fatalf("exclusions = %v, want [7 9]", room.excl)
	}
}

func TestSendTrioOmitsZeroIdsFromExclusions(t *testing.T) {
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "a"),
		Actee:    NoLine,
		Observer: Say(CategoryBash, "c"),
	}, Audience{Actor: &fakeRecipient{}, ActorId: 7, Actee: nil, ActeeId: 0, Room: room})

	if len(room.excl) != 1 || room.excl[0] != 7 {
		t.Fatalf("exclusions = %v, want [7]", room.excl)
	}
}

func TestSendTrioSkipsActeeWhenTheActeeIsAMob(t *testing.T) {
	actor := &fakeRecipient{}
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "you hit"),
		Actee:    Say(CategorySystem, "never delivered"),
		Observer: Say(CategoryBash, "a hits b"),
	}, Audience{Actor: actor, ActorId: 7, Actee: nil, Room: room})

	if len(actor.texts) != 1 {
		t.Fatalf("actor got %v", actor.texts)
	}
	if room.calls != 1 {
		t.Fatalf("room calls = %d, want 1", room.calls)
	}
}

func TestSendTrioSkipsActorWhenTheActorIsAMob(t *testing.T) {
	actee := &fakeRecipient{}
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    NoLine,
		Actee:    Say(CategorySystem, "you are hit"),
		Observer: Say(CategoryBash, "a hits b"),
	}, Audience{Actor: nil, Actee: actee, ActeeId: 9, Room: room})

	if len(actee.texts) != 1 || actee.texts[0] != "you are hit" {
		t.Fatalf("actee got %v", actee.texts)
	}
	if room.calls != 1 {
		t.Fatalf("room calls = %d, want 1", room.calls)
	}
}

func TestSendTrioNoLineObserverSendsNoBroadcast(t *testing.T) {
	room := &fakeBroadcaster{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "private aside"),
		Actee:    NoLine,
		Observer: NoLine,
	}, Audience{Actor: &fakeRecipient{}, ActorId: 7, Room: room})

	if room.calls != 0 {
		t.Fatalf("room calls = %d, want 0", room.calls)
	}
}

func TestSendTrioNilRoomIsSafe(t *testing.T) {
	actor := &fakeRecipient{}
	SendTrio(Trio{
		Actor:    Say(CategorySystem, "a"),
		Actee:    NoLine,
		Observer: Say(CategoryBash, "c"),
	}, Audience{Actor: actor, ActorId: 7})

	if len(actor.texts) != 1 {
		t.Fatalf("actor got %v", actor.texts)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/messaging/ -run TestSendTrio -v`
Expected: FAIL to compile, `undefined: SendTrio`, `undefined: Trio`, `undefined: Say`, `undefined: NoLine`, `undefined: Audience`.

- [ ] **Step 3: Write the implementation**

Create `internal/messaging/trio.go`:

```go
package messaging

// Line is one audience's view of an event: what they are told, and under which
// category.
//
// THE CATEGORY RIDES ON THE LINE, NOT ON THE TRIO, because the three roles of
// one event do not share one. Counted across the twelve player-side special
// move verbs on 2026-09-08: ten send personal lines as CategorySystem and the
// room line as something verb-specific (CategoryBash, CategoryKick,
// CategoryTrip, CategoryGrappleFlow, CategoryHitNaturalSharp); shoot uses four
// categories on the personal side; throw uses three on each side.
//
// A single-Category seam would have silently recategorised two verbs. Category
// feeds the verbosity suppression allowlists in verbosity.go, so that reaches
// any player not on full verbosity.
type Line struct {
	Text string
	Cat  Category
}

// Say builds a Line. It exists so call sites read as prose rather than as
// struct literals.
func Say(cat Category, text string) Line { return Line{Text: text, Cat: cat} }

// NoLine marks a viewpoint that deliberately has nothing to say.
//
// It is the zero Line, spelled out so the guard can tell a considered absence
// from a forgotten one. Every one of the seven narration defects the M1 audit
// found was a duplicated code path that copied a mechanical effect and dropped
// the narration next to it, so "did the author mean this silence?" is the
// question this constant exists to answer.
var NoLine = Line{}

// Trio is one narrated event as its three audiences see it.
//
// Actor is the one acting. Actee is the one acted upon. Observer is everyone
// else in the room.
type Trio struct{ Actor, Actee, Observer Line }

// Recipient is anything that can be sent a categorized line. Satisfied by
// *users.UserRecord and by actions.Actor without either changing.
type Recipient interface {
	SendText(cat Category, text string)
}

// Broadcaster is anything that can broadcast to a room minus some user ids.
// Satisfied by *rooms.Room without change.
type Broadcaster interface {
	SendTextVisual(cat Category, txt string, excludeUserIds ...int)
}

// Audience is who is present for one narrated event.
//
// The ids are passed rather than derived from the Recipients because
// users.UserRecord carries UserId as a FIELD while actions.Actor exposes it as
// GetUserId(), so no single interface can reach both.
//
// A nil Actor is the mob side, where the actor has no client. A nil Actee is
// an actee that is a mob, or an event with no actee at all.
type Audience struct {
	Actor   Recipient
	ActorId int
	Actee   Recipient
	ActeeId int
	Room    Broadcaster
}

// SendTrio delivers one narrated event to everyone entitled to it.
//
// A line is delivered only if it has BOTH text and a recipient; either half
// being absent is a correct, silent skip. The room broadcast always excludes
// the actor and the actee, so a caller can no longer get the exclusion list
// wrong by hand.
//
// It does not render, choose, band or tokenise anything. Callers pass finished
// strings, which is also what lets a caller hand it text it has already
// anonymized itself (see mobcommands/skill_move_defence.go).
func SendTrio(t Trio, aud Audience) {
	if aud.Actor != nil && t.Actor.Text != "" {
		aud.Actor.SendText(t.Actor.Cat, t.Actor.Text)
	}
	if aud.Actee != nil && t.Actee.Text != "" {
		aud.Actee.SendText(t.Actee.Cat, t.Actee.Text)
	}
	if aud.Room != nil && t.Observer.Text != "" {
		aud.Room.SendTextVisual(t.Observer.Cat, t.Observer.Text, trioExclusions(aud)...)
	}
}

// trioExclusions builds the room broadcast's exclusion list. Zero ids are
// omitted: a mob has no user id, and passing 0 would be a no-op that reads
// like a bug.
func trioExclusions(aud Audience) []int {
	ids := make([]int, 0, 2)
	if aud.ActorId > 0 {
		ids = append(ids, aud.ActorId)
	}
	if aud.ActeeId > 0 {
		ids = append(ids, aud.ActeeId)
	}
	return ids
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/messaging/ -v`
Expected: PASS, all seven `TestSendTrio*` plus the package's existing tests.

- [ ] **Step 5: Add the Trio completeness guard**

Append to `messaging_surface_guard_test.go`:

```go
// TestEveryTrioLiteralNamesAllThreeRoles is the enforcement behind
// messaging.NoLine.
//
// A composite literal messaging.Trio{...} must name Actor, Actee AND Observer.
// A role left out is indistinguishable from a role forgotten, and forgetting a
// role is exactly how all seven defects in
// docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md happened.
// Write messaging.NoLine for a viewpoint that genuinely has nothing to say.
//
// There is deliberately no exceptions list.
func TestEveryTrioLiteralNamesAllThreeRoles(t *testing.T) {
	want := map[string]bool{"Actor": true, "Actee": true, "Observer": true}
	var bad []string

	for _, root := range messagingSurfaceGoRoots {
		fset := token.NewFileSet()
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
				if !ok || sel.Sel.Name != "Trio" {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "messaging" {
					return true
				}
				named := map[string]bool{}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if key, ok := kv.Key.(*ast.Ident); ok {
						named[key.Name] = true
					}
				}
				var missing []string
				for role := range want {
					if !named[role] {
						missing = append(missing, role)
					}
				}
				if len(missing) > 0 {
					sort.Strings(missing)
					bad = append(bad, filepath.ToSlash(path)+":"+
						strconv.Itoa(fset.Position(cl.Pos()).Line)+
						"  missing "+strings.Join(missing, ", "))
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
		t.Errorf("%d messaging.Trio literal(s) do not name all three roles:\n  %s\n\n"+
			"Name every role. If a viewpoint genuinely has nothing to say, write "+
			"messaging.NoLine for it, so a considered silence is visible as one. "+
			"An omitted field is indistinguishable from a forgotten one, and that "+
			"is how every defect in the M1 viewpoint audit happened.",
			len(bad), strings.Join(bad, "\n  "))
	}
}
```

Add `path/filepath`, `io/fs` and `strconv` to the file's imports if the
existing block does not already carry them. It does carry all three.

- [ ] **Step 6: Prove the guard can fail**

Temporarily add this to `internal/usercommands/bash.go` inside `Bash`:

```go
_ = messaging.Trio{Actor: messaging.NoLine, Actee: messaging.NoLine}
```

Run: `go test . -run TestEveryTrioLiteralNamesAllThreeRoles`
Expected: FAIL naming `internal/usercommands/bash.go` and `missing Observer`.

Remove the line, re-run, expect PASS. Also re-run `TestM2LiteralsAreFrozen`
and expect PASS, since no string literal changed.

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/messaging/ .
git add internal/messaging/trio.go internal/messaging/trio_test.go messaging_surface_guard_test.go
git commit -m "feat(messaging): one seam for narrating an event to its three audiences"
```

---

## Task 2: Unify the two defence helpers

**Read the spec's section 3a first.** This is not a pure refactor. It carries
one accepted, deliberate behavior change, and it must NOT carry the second one
(defect 7), which lands in Task 10.

**Files:**
- Modify: `internal/usercommands/skill_move_defence.go:32-70`
- Modify: `internal/mobcommands/skill_move_defence.go:30-71`

**What changes.** Both functions' send tails become one `SendTrio` call. The
mob copy stops calling `sendAudioRoomText`, so its room line moves from the
audio channel to the visual channel and picks up the pipeline's sight gate.
That is the accepted change: bystanders who are blind, asleep, or unaided in a
dark room stop receiving it.

**What must NOT change in this task.**

- The mob copy still anonymizes the defender's personal line by hand. Pass the
  already-anonymized string into the `Trio`.
- The player copy still sends its defender line raw. Fixing that is defect 7,
  and it lands in Task 10 as its own commit so the leak fix is reviewable on
  its own.
- Identity resolution stays in each package. A mob attacker names itself with
  `GetMobNameIndexed` and a player attacker with `GetPlayerName`; that
  divergence is correct and is not duplication.
- `roomOnly` semantics are unchanged.
- The shortage text still goes out as `CategorySystem` before the triad.

- [ ] **Step 1: Rewrite the player copy's send tail**

In `internal/usercommands/skill_move_defence.go`, replace the block from
`if !roomOnly {` through the `room.SendTextVisual(...)` call with:

```go
	actee := messaging.NoLine
	actor := messaging.NoLine
	if !roomOnly {
		actor = messaging.Say(category, string(triad.ToAttacker))
		if targetUser != nil {
			actee = messaging.Say(category, string(triad.ToDefender))
		}
	}

	var acteeRecipient messaging.Recipient
	if targetUser != nil {
		acteeRecipient = targetUser
	}

	messaging.SendTrio(messaging.Trio{
		Actor:    actor,
		Actee:    actee,
		Observer: messaging.Say(category, string(triad.ToRoom)),
	}, messaging.Audience{
		Actor:   user,
		ActorId: user.UserId,
		Actee:   acteeRecipient,
		ActeeId: target.UserId,
		Room:    room,
	})
	return true
```

Note `acteeRecipient` is declared as the interface type and left nil when there
is no target user. Assigning a typed nil `*users.UserRecord` into the interface
would make `aud.Actee != nil` true and panic on the call.

- [ ] **Step 2: Rewrite the mob copy's send tail**

In `internal/mobcommands/skill_move_defence.go`, replace the block from
`excluded := make([]int, 0, 1)` through the `sendAudioRoomText(...)` call with:

```go
	actee := messaging.NoLine
	var acteeRecipient messaging.Recipient
	if targetUser != nil {
		acteeRecipient = targetUser
		if !roomOnly {
			// The audio channel neither gates nor anonymizes (see
			// users.UserRecord.SendText, which is hardcoded to ChannelAudio),
			// so this line is anonymized here or not at all. M4's perception
			// verdict consolidation is where this moves into the pipeline.
			personal := string(triad.ToDefender)
			if !canSeeInDark(targetUser, room) {
				personal = messaging.Anonymize(personal)
			}
			actee = messaging.Say(category, personal)
		}
	}

	messaging.SendTrio(messaging.Trio{
		// A mob has no client, so the attacker's own line is categorically
		// absent rather than a per-site judgment.
		Actor:    messaging.NoLine,
		Actee:    actee,
		Observer: messaging.Say(category, string(triad.ToRoom)),
	}, messaging.Audience{
		Actee:   acteeRecipient,
		ActeeId: target.UserId,
		Room:    room,
	})
	return true
```

- [ ] **Step 3: Build and run the affected packages**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/combat/
```

Expected: PASS. If `sendAudioRoomText` is now unused in `mobcommands`, the
compiler will NOT tell you (it is used by 19 other files) — leave it in place.

- [ ] **Step 4: Confirm the freeze still passes**

Run: `go test . -run TestM2LiteralsAreFrozen`

Expected: PASS. No string literal changed in either file; only the delivery
call did. **If this fails, you altered text you were only supposed to move.**

- [ ] **Step 5: Commit**

```bash
git add internal/usercommands/skill_move_defence.go internal/mobcommands/skill_move_defence.go
git commit -m "refactor(messaging): both defence helpers narrate through one seam"
```

The commit body should record the accepted behavior change:

```
The mob copy's room line moves from sendAudioRoomText (audio channel,
hand-rolled NightVision check) to the seam's SendTextVisual (visual channel,
pipeline sight gate). Bystanders who are blind, asleep, or unaided in a dark
room stop receiving it. Nobody gains a message.

The defender's personal line keeps its call-site anonymization on the mob side
and stays raw on the player side. That inconsistency is defect 7 and is fixed
separately so the leak fix is reviewable on its own.
```

---

## Task 3: Migrate `internal/usercommands/`, twelve files

One commit per file. The conversion is identical every time, so Step 1 works
`bash.go` in full and the rest follow the same recipe with the per-file data in
the table.

### The recipe

1. **Every `room.SendTextVisual` call becomes exactly one `SendTrio`.** The
   `user.SendText` and `targetUser.SendText` (or `targetChar.SendText`, or
   `p.SendText`) calls in the same branch become that trio's `Actor` and
   `Actee`.
2. **An actor-only detail line riding on a parent event becomes its own
   `Trio`** with `Actee: messaging.NoLine, Observer: messaging.NoLine`.
3. **Refusals and mechanical feedback stay plain `SendText`.** See "Things that
   will bite you", item 3.
4. **Carry each line's existing category across verbatim.** Do not hoist.
5. Build the `Audience` once per branch, or once per function where the
   participants do not change.

### Step 1: `bash.go`, worked in full

**Files:** Modify `internal/usercommands/bash.go:59-104`

- [ ] Replace the `if result.Hit {` block's first branch:

```go
	var acteeRecipient messaging.Recipient
	if targetUser != nil {
		acteeRecipient = targetUser
	}
	aud := messaging.Audience{
		Actor: user, ActorId: user.UserId,
		Actee: acteeRecipient, ActeeId: target.UserId,
		Room:  room,
	}

	if result.Hit {
		if result.KnockedDown {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(messaging.CategorySystem,
					fmt.Sprintf(`Your <ansi fg="yellow-bold">shield bash</ansi> knocks <ansi fg="mobname">%s</ansi> to the ground! (<ansi fg="damage">%s</ansi>)`, target.Name, dmgDesc)),
				Actee: messaging.Say(messaging.CategorySystem,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>'s <ansi fg="yellow-bold">shield bash</ansi> knocks you to the ground! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, dmgDesc)),
				Observer: messaging.Say(messaging.CategoryBash,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>'s <ansi fg="yellow-bold">shield bash</ansi> knocks <ansi fg="mobname">%s</ansi> to the ground!`, user.Character.Name, target.Name)),
			}, aud)
		} else {
			messaging.SendTrio(messaging.Trio{
				Actor: messaging.Say(messaging.CategorySystem,
					fmt.Sprintf(`Your <ansi fg="yellow-bold">shield bash</ansi> strikes <ansi fg="mobname">%s</ansi>! (<ansi fg="damage">%s</ansi>)`, target.Name, dmgDesc)),
				Actee: messaging.Say(messaging.CategorySystem,
					fmt.Sprintf(`<ansi fg="username">%s</ansi>'s <ansi fg="yellow-bold">shield bash</ansi> strikes you! (<ansi fg="damage">%s</ansi>)`, user.Character.Name, dmgDesc)),
				Observer: messaging.Say(messaging.CategoryBash,
					fmt.Sprintf(`<ansi fg="username">%s</ansi> bashes <ansi fg="mobname">%s</ansi> with their shield!`, user.Character.Name, target.Name)),
			}, aud)
		}
	}
```

Apply the same shape to the `else if result.Damage > 0` branch and the final
`else` branch. The `NoShield` / `OnCooldown` / `NoTarget` / `Crafting` /
`CostRefused` sends at the top of the function are **refusals and stay
untouched.**

- [ ] Run: `gofmt -l internal/`, then
  `go build ./... && go test ./internal/usercommands/ .`
  Expected: PASS, including `TestM2LiteralsAreFrozen` and
  `TestEveryTrioLiteralNamesAllThreeRoles`.

- [ ] Commit: `git add internal/usercommands/bash.go && git commit -m "refactor(messaging): bash narrates through the seam"`

### Steps 2-12: the remaining eleven files

Same recipe. Per-file data:

| File | Personal category | Room category | Expected `SendTrio` calls |
|---|---|---|---|
| `drain.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 3 |
| `gore.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 4 |
| `kick.go` | `CategorySystem` | `CategoryKick` | 4 |
| `maul.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 3 |
| `pounce.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 4 |
| `rake.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 3 |
| `throttle.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 3, plus one actor+actee-only trio for the interrupted cast at `:77` |
| `trip.go` | `CategorySystem` | `CategoryTrip` | 8 |
| `grapple.go` | `CategorySystem` | `CategoryGrappleFlow` | 4, plus two actor-only trios (`:107` position penalty, `:131` defence penalty) |
| `shoot.go` | **mixed** — `CategorySystem`, `CategoryHitRanged`, `CategoryDodge`, `CategorySurpriseAttack`. Read each site. Its actee variable is named `p` | `CategoryHitRanged` | 6 |
| `throw.go` | **mixed** — `CategorySystem`, `CategoryDodge`, `CategorySpellDisruption` | **mixed** — `CategoryHitRanged`, `CategoryDodge`, `CategorySpellDisruption` | 4, every one with `Actee: messaging.NoLine` |

- [ ] For each file: apply the recipe, run
  `gofmt -l internal/ && go build ./... && go test ./internal/usercommands/ .`,
  expect PASS, then commit as
  `refactor(messaging): <verb> narrates through the seam`.

**`throw.go` is the one to be careful with.** It has 17 actor sends and zero
actee sends, and that is correct: it is an area effect against mobs. Every trio
in it takes `Actee: messaging.NoLine`. Do not invent an actee.

**`throttle.go` at `:77`** is the interrupted-cast detail line. In this task it
keeps today's behavior exactly: `Actor` and `Actee` set, `Observer:
messaging.NoLine`. Task 9 gives it a room line.

**`grapple.go` at `:107` and `:131`** are private asides under the spec's
detail-line ruling. Both become `Actee: messaging.NoLine, Observer:
messaging.NoLine` and stay actor-only. They do not gain lines in any later
task.

---

## Task 4: Migrate `internal/mobcommands/`, eleven files

Same recipe, with two differences that apply to every file:

- **`Actor: messaging.NoLine` and `Audience.Actor: nil`, always.** A mob has no
  client. This is categorical, not a judgment call.
- **The `canSeeInDark` call each file already makes stays exactly where it
  is.** It selects which text to render for the actee, which the seam does not
  care about because it takes finished strings. Do not remove it, do not move
  it into the seam.

| File | Personal category | Room category | Expected `SendTrio` calls |
|---|---|---|---|
| `bash.go` | `CategorySystem` | `CategoryBash` | 4 |
| `drain.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 3 |
| `gore.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 4 |
| `kick.go` | `CategorySystem` | `CategoryKick` | 10 |
| `maul.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 3 |
| `pounce.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 4 |
| `rake.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 3 |
| `throttle.go` | `CategorySystem` | `CategoryHitNaturalSharp` | 3 |
| `trip.go` | `CategorySystem` | `CategoryTrip` | 9 |
| `grapple.go` | `CategorySystem` | `CategoryGrappleFlow` | 4 |
| `shoot.go` | `CategorySystem` | `CategoryHitRanged` | 4. It also calls `messaging.Anonymize` once — leave that call at the call site, same reasoning as the defence helper |

- [ ] For each file: apply the recipe, run
  `gofmt -l internal/ && go build ./... && go test ./internal/mobcommands/ .`,
  expect PASS, then commit as
  `refactor(messaging): mob <verb> narrates through the seam`.

---

## Task 5: Documentation

**Files:**
- Modify: `internal/messaging/context.md`
- Create: `internal/narration/context.md`
- Modify: `internal/banner/banner.go:5-10`

- [ ] **Step 1: Re-derive `internal/messaging/context.md`'s Public API section**

It currently documents four symbols that do not exist: `UserSender`,
`ProgressionKind`, `FormatProgression`, `SendProgression`. There is no
`progression.go` in the package. Do not edit around them — re-derive the
section from the package:

```powershell
Select-String -Path internal\messaging\*.go -Pattern '^(func|type|const|var)\s' | Where-Object { $_.Line -notmatch '_test' }
```

Then add `Line`, `Say`, `NoLine`, `Trio`, `Recipient`, `Broadcaster`,
`Audience`, `SendTrio`, and a short section saying the package now owns both
the per-recipient pipeline and the fan-out of one event to three audiences,
with the import-graph reason it lives here.

- [ ] **Step 2: Write `internal/narration/context.md`**

The package has none, which the project convention requires. It holds
`picker.go` (`Picker`, `DefaultPicker`, `SequencePicker`), `picker_test.go`,
`snapshot_test.go` and `testdata/`. Cover: why `SequencePicker` is a closure
and not a seed (this repo runs a package's tests in one binary, so a seed makes
output order-dependent), and that the package must never import `messaging`
because `items` imports it and `messaging` reaches `items` through
`characters`.

- [ ] **Step 3: Fix the banner comment**

`internal/banner/banner.go:7` points at `internal/messaging/progression.go`,
which does not exist. Correct or remove the reference.

- [ ] **Step 4: Commit**

```bash
git add internal/messaging/context.md internal/narration/context.md internal/banner/banner.go
git commit -m "docs: correct messaging's phantom API and give narration a context.md"
```

**Already done, do not redo:** the spec's section 6 also lists `docs/README.md`
gaining a row for the M2 spec and its 18%-to-26% correction. Both landed with
the spec commits on this branch.

**No task for the quest bridge.** The spec rules its missing actee seam
correctly absent: quest YAML has `playermessage`, `roommessage` and `room_text`
and no actee key, so the slot would have nothing to fill it. M3's migration
table schedules it. Do not add an actee method to
`internal/questengine/bridge.go` in this slice.

---

## Tasks 6-12: The seven output changes

**These are the only tasks that change what a player reads.** One commit each.
Each one updates the affected file's fingerprint in `m2FrozenFiles` **in the
same commit**, so the text change and the hash change are one hunk.

> **Why these tasks name a sibling instead of pasting code.** Five of the seven
> defects are "this branch dropped the line its sibling branch sends". The
> sibling is the source of truth for the wording, the category and the ANSI
> tags, and it is a few lines away in the same file. Read it and mirror it.
> Pre-writing the text here would mean inventing a line that then has to be
> reconciled with the real one, which is how the wording drifts in the first
> place.

For every task below the shape is the same:

1. Make the change.
2. Run `go test . -run TestM2LiteralsAreFrozen`, expect FAIL with a new hash for
   that file (if the file is in the frozen list).
3. Record the new hash.
4. Run `gofmt -l internal/`, `go build ./...`, the package's tests, and
   `go test .`, expect PASS.
5. Commit the code change and the hash together.

- [ ] **Task 6 — `internal/actions/salvage.go:202` and `:207`.** The room
  broadcast sits on the `else if` mob branch, so a mob butchering a corpse is
  narrated and a player doing it is invisible. Move the broadcast onto the
  shared path so both narrate. Commit:
  `fix(messaging): a player butchering a corpse is visible to the room`

- [ ] **Task 7 — `internal/hooks/spell_resolution.go:1075`.** `case "buff":`
  never calls `sendVisualRoomText`; its sibling `case "heal":` at roughly
  `:1044` does. Add the broadcast, matching the heal case's shape and category.
  Commit: `fix(messaging): a buff spell is visible to the room like a heal is`

- [ ] **Task 8 — `internal/usercommands/rally.go:72` and
  `internal/usercommands/warcry.go:76`.** The Resonant Larynx fold loops apply
  `AddCondition` / `AddBuff` to every party member and never `SendText` them.
  Add a per-member line inside each loop. Both files, one commit, since the bug
  is mirrored. Commit:
  `fix(messaging): rally and warcry tell the party members they buff`

- [ ] **Task 9 — `internal/usercommands/equip.go:218`.** The arm-slot path drops
  the room line the shared path sends at `:274` and `:277`. Add it, matching the
  shared path's text and category. Commit:
  `fix(messaging): an arm-slot equip is visible to the room`

- [ ] **Task 10 — `internal/usercommands/admin.zap.go:82`.** The engaged-target
  path puts a player on 1 health with no message; the explicit-target path at
  `:46` does tell them. Add the actee line. Commit:
  `fix(messaging): zapping an engaged target tells them it happened`

- [ ] **Task 11 — `internal/usercommands/throttle.go:77`.** The interrupted-cast
  detail line is told to actor and actee only. Under the spec's detail-line
  ruling a collapsing spell is a world event, so it gains a room line. Replace
  its `Observer: messaging.NoLine` with a `Say(messaging.CategoryHitNaturalSharp, ...)`
  naming both parties. **This is a scope add, not a bug M1 found** — say so in
  the commit body. Commit:
  `feat(messaging): the room sees a spell collapse under a throttle`

- [ ] **Task 12 — `internal/usercommands/skill_move_defence.go`, defect 7.**
  The defender's personal line is sent raw here while the mob copy anonymizes
  it, so a player defending in the dark learns who hit them if the attacker was
  a player and does not if it was a mob. Add the same guard the mob copy uses:

```go
		if targetUser != nil {
			personal := string(triad.ToDefender)
			if !canSeeInDark(targetUser, room) {
				personal = messaging.Anonymize(personal)
			}
			actee = messaging.Say(category, personal)
		}
```

`canSeeInDark` is in `internal/mobcommands/darkness.go:44` and is not exported.
**First check whether `usercommands` already has an equivalent under another
name** — `grep -rn "NightVision\|GetVisibility" internal/usercommands/`. As of
2026-09-08 it does not. If it still does not, add an unexported twin in
`usercommands` and note in the commit body that the two should collapse when M4
consolidates the perception verdict. Do not build a shared helper package for
two call sites.

Commit: `fix(messaging): a defender in the dark is not told who hit them`

---

## Task 13: Verification and the playtest gate

- [ ] **Step 1: Full local gates**

```bash
gofmt -l internal/ modules/
go build ./...
go test ./...
```

Expected: all pass. Note that roughly 2.3% of attack rolls are fumbles that
always miss, so a combat test that fails once should be re-run before being
believed.

- [ ] **Step 2: Boot test in an isolated worktree**

```bash
git worktree add --detach C:/tmp/dogmud-boot-check HEAD
cp _datafiles/config.yaml C:/tmp/dogmud-boot-check/_datafiles/config.yaml
cd C:/tmp/dogmud-boot-check && go build -o boot-check.exe .
timeout 180 ./boot-check.exe > boot.log 2>&1
grep -cE "^panic:|goroutine [0-9]+ \[running\]|runtime error" boot.log   # want 0
grep -c "Server Ready" boot.log                                          # want 1
```

Exit code 124 is the success case. Do not grep for the bare word `panic`:
`GamePlay.MapConsistencyEnforce` legitimately has the value `panic`. Clean up
with `git worktree remove --force`.

- [ ] **Step 3: `docs/PATCH_NOTES.md`**

Dated entry, player-facing framing, no raw numbers, no em dashes. It must
mention the accepted delivery change from the spec's section 3a: fighting in
the dark now hides more than it used to.

- [ ] **Step 4: The adversarial playtest gate**

Required by the project content SOP, because the seven output changes author
new player-facing lines.

```text
/playtest local --checkout <abs> bug-finder 2026-09-08-messaging-m2.yaml
```

The goals file needs `ephemeral:`. Wipe instance saves first:

```bash
rm -rf _datafiles/world/dogmud/mobs.instances _datafiles/world/dogmud/rooms.instances
```

The session must exercise all seven scenarios from the spec's section 7,
including the two that are easy to mistake for bugs:

- A defended special move in a dark room watched by a bystander with no sight
  aid. **The bystander should now receive nothing.**
- A player defending a special move in the dark against a player attacker.
  **They should no longer be told the attacker's name.**

Fix what it finds, re-run if needed, and only then hand it over.

- [ ] **Step 5: Ship**

```bash
git push -u origin feature/messaging-m2-shared-narration-seam
gh pr create --repo pruuk/DOGMud --base master --head feature/messaging-m2-shared-narration-seam --fill
gh pr checks <n> --repo pruuk/DOGMud --watch
gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch
```

Always pass `--repo pruuk/DOGMud`. This repo is a fork and `gh` defaults to the
parent. A green check is not proof on its own; confirm with
`gh run view <id> --repo pruuk/DOGMud --log-failed`.
