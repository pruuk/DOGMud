# Messaging M7 slice 4: usage, admin and narration off System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Every subagent brief carries these rules:** edit files with the Edit and Write tools only (no `sed -i`, no Python `open(path, 'w')`, no `>` into a tracked file; `~/.claude/hooks/guard.py` blocks them, worktrees included); `git add` named paths only; never `--no-verify`; no em or en dashes in prose or player text; every `gh` command carries `--repo pruuk/DOGMud`; scratch output goes to the session scratchpad, never `$TMP`. Line numbers are as of master `ae94920b7`: once a file has been edited, find a site by its text.

**Goal:** `CategorySystem` keeps only reports, tables, help and confirmations of the player's own action: usage lines ride a new `CategoryUsage`, everything an admin-only command shows staff rides a new `CategoryAdmin`, narration and notices to other players ride their narration categories, and guards keep it that way.

**Architecture:** Two new categories (three if ruling R1 is approved), each treated exactly as `CategorySystem` (same alias colour 8, every normalize stage, never wraps), so a player sees no change until the owner recolours one. The site lists come from two ledgers: the slice 3 ledger's 47 USAGE and 63 NARR rows, and a new admin ledger (667 rows) built the same way. Two root guards: usage lines ride `CategoryUsage`, and no function reachable only from an admin-only command sends on `CategorySystem` (the admin set is derived from the command registry, not from file names).

**Tech Stack:** Go, testify, `go/ast` root guards (`package main` at the repo root).

**Spec:** `docs/superpowers/specs/2026-10-14-messaging-m7-refusals-design.md`, "Suggested slicing" item 4, sections 5 and 7. Slices 1 to 3 shipped as #547, #549, #559. The spec puts playtests at the end of slices 1 and 3 only; this slice changes no player-visible text beyond table C below, so it ends with a boot check, not a playtest.

---

## Facts verified against source (master `ae94920b7`, 2026-10-16)

| # | Fact | Source |
|---|---|---|
| 1 | The Category enum appends new values before `categoryMax`; `CategoryRefusal` is the last today, with `String()` "refusal" | `internal/messaging/messaging.go:127-140,275` |
| 2 | `CategoryRefusal` was added by `edee82c44` with alias `refusal: 8` (same as `system: 8`), and three tests in `internal/messaging/category_refusal_test.go`: alias matches system, same `skipStages`, not in `shouldWrap`, no verbosity suppresses it, and it renders as System | file, `git log` |
| 3 | `shouldWrap` (`pipeline.go:122`) lists wrapping categories; `skipStages` (`normalize.go:25`) returns 0 for System; `TestSystemNeverWraps` is `pipeline_test.go:268` | files |
| 4 | Admin-only commands are the `userCommands` entries whose fourth field (`AdminOnly`) is true: 48 of them; their handlers live in 45 `admin.*.go` files plus `ban.go`, `boot.go`, `unban.go`, `petitions.go` | `internal/usercommands/usercommands.go:34-38,55-230` |
| 5 | `admin.bounty.go` (`bounty`, `usercommands.go:132`) and `admin.reload.go` (`reload`) are NOT admin-only: players reach `bounty list/show` and the `reload` refusal; so "admin.*.go" is not "admin output" | registry, admin ledger notes |
| 6 | Module commands register through `plug.AddUserCommand(name, fn, allowWhenDowned, isAdminOnly)` (`internal/plugins/plugins.go:269`); admin-only ones: `aicompanion` (`modules/aicompanion/aicompanion.go:227`, method `cmdAICompanion`, `commands.go:38`), `ai-flag`, `ai-list` (`modules/playtest/commands.go:14-15`); `modules/aicompanion` also registers player commands with `usercommands.RegisterCommand(..., false)` (`aicompanion.go:585-591`) whose handlers share `commands.go` | files |
| 7 | Staff-only subcommands of player commands, gated inside by `HasRolePermission`: `party admin` (`party.go` ~845-980), `weather` admin subcommands (`modules/weather/weather_commands.go`), `bounty declare/withdraw/prune-expired` | admin ledger notes |
| 8 | Admin ledger (`docs/superpowers/audits/messaging-m7-admin-ledger.md`): 667 rows; ADMIN 338, REFUSAL 218, USAGE 88, NOT 11, NOTICE 10, ROOM 1, SHARED 1; final categories Admin 552, Usage 88, Refusal 4, System 11, other 12 | ledger header |
| 9 | Slice 3 ledger: 47 USAGE and 63 NARR rows outside the admin files | `docs/superpowers/audits/messaging-m7-refusal-ledger.md` |
| 10 | `modules/weather/weather.go:147` `sendLine(user, text)` is the module's single send, used by the player report and the admin subcommands alike | admin ledger SHARED row |
| 11 | Ten `*Usage(user *users.UserRecord)` functions in `internal/usercommands/admin.*.go` (`baubleUsage`, `bountyAdminUsage`, `bountyUserUsage`, `crimeShowUsage`, `factUsage`, `factionShowUsage`, `goalShowUsage`, `knowledgeUsage`, `opinionShowUsage`, `relationshipUsage`) | `grep -rn "func [a-zA-Z]*Usage(user" internal modules` |
| 12 | Channel chat rides `CategoryBroadcast` (`internal/hooks/ChannelMessage_SendToAll.go:30`), which skips every normalize stage; `CategoryOOC` exists and has no production sender | grep |
| 13 | The per-file category tests: `internal/usercommands/special_move_categories_test.go` (`systemNoticeMarkers` now `"text)"` for taunt's defence-shortage notice and `"crumbles apart"` for `throw.go:276`) and `internal/mobcommands/throttle_categories_test.go` (no System at all) | files |
| 14 | `combat.ChannelDefenceShortageText` ("You mount a desperate response, too spent to bring practiced technique to it.") rides `CategorySystem` on every path (`combat_helpers.go:204`, `taunt.go:160`, `skill_move_defence.go:81`, `shoot.go:491`, `spell_resolution.go:440`, two mobcommands sites) | grep |
| 15 | Root goldens that fingerprint narration sends: `testdata/m2-routing.golden` (`m2_routing_guard_test.go`, re-record with `M2_RECORD_ROUTING=1 go test . -run TestM2RoutingIsFrozen`) and the viewpoint audit in `messaging_surface_guard_test.go` (`TestNarrationSitesMatchViewpointAudit`) | files |
| 16 | `admin.teleport.go:134` sends "Moved to room %d." to non-staff party members (a raw room id); `admin.skillset.go:92` sends its "Skill not found" refusal to the target instead of the staff member; `admin.teleport.go:75,86` name `teleport.direction` in permission text where the checks are `teleport.playername` / `teleport.roomid`; `roommanager.go:293` says "enscribed" | admin ledger notes, files |
| 17 | `weather_commands.go:93` shows every player a front's internal id and intensity numbers | admin ledger note |

## Owner rulings (approve with the plan)

- **R1, party and guild lines.** The 13 party notices, 4 guild notices (joined, left, kicked, promoted, loot mode, invited) and 3 group chats (`party chat`, `guild` chat, `report` to the party) are notices and talk aimed at group members, not narration and not the player's own report. Recommended: a new `CategoryGroup` (alias `group: 8`, same treatment as System) for the notices, and the three chats on `CategoryBroadcast`, where channel chat already rides (it never rewrites player-typed text). Locks in: a client or the owner can recolour or filter group traffic later without touching code. Alternative: leave them on System (no new category; they stay in the mixed bucket).
- **R2, staff refusals.** A refusal an admin-only command shows staff ("No such room", "Unknown mob") rides `CategoryAdmin`, not `CategoryRefusal`: everything a staff command shows staff is one stream. The four refusals a non-staff player can reach in these files (`admin.bounty.go:43,164,169`, `admin.reload.go:36`) ride `CategoryRefusal`. Locks in: no per-line refusal-versus-report calls inside staff output (the ledger agents drew that line differently, which is the reason). Alternative: staff refusals ride `CategoryRefusal`, as slice 3 did for `admin.zap.go:60`.
- **R3, notices to the target of a staff action.** "You have been banned.", "You have been disconnected by staff.", "Your role has been set to ...", "Your skill level has been set ..." (when the target is someone else) and "You get the feeling someone is looking for you..." ride `CategoryWarning`. Staff actions the room sees (`zap`, `paz`, `teleport` arrival) ride `CategoryMobEmote`, the category their room lines already use.
- **R4, the per-file tests stay.** Spec section 7 folds `special_move_categories_test.go` and `throttle_categories_test.go` into the new guard. A general tag rule cannot tell narration from a confirmation, so folding would only move the same file list; this plan keeps both, tightens the special-move one to the shortage notice alone once `throw.go:276` moves, and records why in `internal/refusal/context.md`.

## Wording (owner review: approve with the plan)

### A. New categories

| Category | `String()` / alias | Carries |
|---|---|---|
| `CategoryUsage` | `usage: 8` | "Usage:" lines and usage blocks, staff and player alike |
| `CategoryAdmin` | `admin: 8` | everything an admin-only command or staff-only subcommand shows staff (R2) |
| `CategoryGroup` (R1) | `group: 8` | party and guild membership notices |

### B. Narration and notices leaving System (no words change)

| Site | Line | Category | Rule |
|---|---|---|---|
| `actions/move.go:257,351,405,408`, `actions/sneak.go:223`, `actions/search.go:498,501`, `actions/shadow.go:213` | someone notices you / you notice someone | `CategoryEmote` when the other party is a player, `CategoryMobEmote` when a mob | the actor's kind decides, as M5 did for emotes |
| `actions/move.go:452`, `actions/plant.go:333`, `actions/steal.go:517`, `actions/stolen_bauble.go:245,320`, `hooks/NewRound_DoCombat_helpers.go:1010` | a mob acts on you | `CategoryMobEmote` | |
| `behaviortree/actions_item_voice.go:227,237` | an item speaks | `CategoryNPCDialogue` | |
| `bountyhunter/bountyhunter.go:135` | a hunter took the contract | `CategoryWarning` | |
| `rooms/roommanager.go:293` | your rune faded | `CategoryRoomDescription` | (and table C) |
| `modules/follow/follow.go:211,222,468,494,535,615,637,659` | follow start and stop | `CategoryEmote` / `CategoryMobEmote` by the follower's kind | |
| `ban.go:90`, `boot.go:37`, `admin.modify.go:74`, `admin.skillset.go:69,86` (target not the typist), `admin.locate.go:41` | staff action notice | `CategoryWarning` | R3 |
| `admin.zap.go:46,90`, `admin.paz.go:45`, `admin.teleport.go:109,134` | staff action the target or room sees | `CategoryMobEmote` | R3 |
| `give.go:132,357` | someone gives you something | `CategoryLoot` (the room line's category) | |
| `go.go:352` | You follow the party leader. | `CategoryRoomExit` | |
| `go.go:461,463` | something notices you as you enter | `CategoryMobEmote` | |
| `inventory.go:68,77` | a grenade in your pack goes off | `CategoryHitRanged` (throw's grenade category) | |
| `inventory.go:94` | grenades dissolve | `CategoryEquipment` | |
| `throw.go:276` | a spoiled item crumbles as you throw | `CategoryHitRanged` | |
| `show.go:78,83` | someone shows you an item | `CategoryMobEmote` (the room line's category) | |
| `target.go:243` | someone shifts focus to you | `CategoryMobEmote` | |
| `usercommands.go:414` | You lose your concentration as you flee! | `CategorySpellDisruption` (the room line's category) | |
| `report.go:89` | a whispered report | `CategoryWhisper` | |
| `petition.go:77` | the staff alert for a new petition | `CategoryAdmin` | |
| `party.go:240,278,301,544,546,572,594,599,633,638,661,667,728`, `guild.go:86,294,353,394` | group membership notices | `CategoryGroup` | R1 |
| `party.go:814`, `guild.go:466`, `report.go:47` | group chat | `CategoryBroadcast` | R1 |

Where a site's own function already sends the same event's other roles on a category, that category wins over this table (one event, one category set); a site that differs is reported, not guessed.

### C. Words that change

| Site | Today | Becomes | Why |
|---|---|---|---|
| `admin.teleport.go:134` (to a non-staff party member) | Moved to room 123. | A flash of light carries you away. | raw room id to a player |
| `rooms/roommanager.go:293` | The rune you had enscribed here has faded away. | The rune you had inscribed here has faded away. | spelling |
| `admin.teleport.go:75,86` (staff only) | names `teleport.direction` | names the permission actually checked (`teleport.playername`, `teleport.roomid`) | wrong text |

`admin.skillset.go:92` keeps its words and goes to the staff member who typed the command instead of the target.

## File structure

| File | Responsibility |
|---|---|
| Modify `internal/messaging/messaging.go`, `_datafiles/world/dogmud/ansi-aliases.yaml` | the new categories and aliases |
| Create `internal/messaging/category_m7_tags_test.go` | their treatment tests |
| Create root `usage_category_guard_test.go` | usage lines ride `CategoryUsage` |
| Create root `admin_category_guard_test.go` | admin-only reachable code sends no `CategorySystem` |
| Modify the ledger sites | the moves |
| Modify `modules/weather/weather.go`, `weather_commands.go` | `sendLine` takes a category |
| Modify `internal/refusal/context.md`, `internal/messaging/context.md`, `internal/usercommands/context.md`, `docs/README.md`, both ledgers | docs |

---

### Task 1: The categories

**Files:** `internal/messaging/messaging.go`, `_datafiles/world/dogmud/ansi-aliases.yaml`; Create `internal/messaging/category_m7_tags_test.go`.

- [ ] **Step 1: Write the failing test.** `internal/messaging/category_m7_tags_test.go`:

```go
package messaging

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// M7 slice 4 (#371): the usage, admin and group categories are treated
// exactly as CategorySystem, where their lines rode, so moving them changes
// nothing a player sees until the owner recolours one.
var m7TagCategories = map[Category]string{
	CategoryUsage: "usage",
	CategoryAdmin: "admin",
	CategoryGroup: "group",
}

func TestM7TagCategoriesKeepSystemTreatment(t *testing.T) {
	for c, name := range m7TagCategories {
		require.Equal(t, name, c.String())
		require.Equal(t, skipStages(CategorySystem), skipStages(c), "%s must normalise as System", name)
		require.False(t, shouldWrap(c), "%s must not wrap: System never did", name)
		for _, v := range []Verbosity{VerbosityFull, VerbosityMedium, VerbosityLight} {
			require.False(t, v.Suppresses(c), "verbosity %v suppresses %s", v, name)
		}
	}
}

func TestM7TagAliasesMatchSystem(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "ansi-aliases.yaml"))
	require.NoError(t, err)
	var doc struct {
		Colors map[string]any `yaml:"colors"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))
	for _, name := range m7TagCategories {
		require.Contains(t, doc.Colors, name)
		require.Equal(t, doc.Colors["system"], doc.Colors[name], "%s must keep system's colour", name)
	}
}
```

If ruling R1 is not approved, drop `CategoryGroup` from the map and from Step 3.

- [ ] **Step 2: Run it.** `go test ./internal/messaging/ -run TestM7Tag` Expected: build failure, `undefined: CategoryUsage`.

- [ ] **Step 3: Implement.** After `CategoryRefusal` in the enum (before `categoryMax`), append:

```go
	// Usage lines and blocks (messaging M7 slice 4, #371): "Usage: x <y>"
	// and the *Usage(user) blocks, staff and player alike. Appended so no
	// existing value shifts; treated exactly as CategorySystem (alias usage
	// at system's value), so the move changes nothing a player sees.
	CategoryUsage

	// Staff output (M7 slice 4, #371): everything an admin-only command or a
	// staff-only subcommand shows the staff member who typed it, its
	// refusals included (owner ruling R2). Treated exactly as CategorySystem
	// (alias admin at system's value).
	CategoryAdmin

	// Party and guild membership notices (M7 slice 4, #371, owner ruling
	// R1): joined, left, kicked, promoted, invited, loot mode. Treated
	// exactly as CategorySystem (alias group at system's value). Group chat
	// rides CategoryBroadcast, where channel chat rides.
	CategoryGroup
```

`String()` cases `"usage"`, `"admin"`, `"group"`. In `ansi-aliases.yaml`, after `refusal: 8`, a comment and `usage: 8`, `admin: 8`, `group: 8`. Add none of them to `shouldWrap` or `skipStages`.

- [ ] **Step 4: Run.** `go test ./internal/messaging/` Expected: PASS (including `messaging_test.go:13` and `pipeline_test.go:131`, which walk to `categoryMax`).

- [ ] **Step 5: Commit.** `feat(messaging): CategoryUsage, CategoryAdmin and CategoryGroup, treated as System (#371)`.

### Task 2: Usage lines ride CategoryUsage

**Files:** Create root `usage_category_guard_test.go`; the USAGE rows of both ledgers.

- [ ] **Step 1: Write the guard.**

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Messaging M7 slice 4 (#371): usage rides messaging.CategoryUsage. Two
// rules: a call that passes a messaging category and then a literal holding
// "Usage:" uses CategoryUsage; and every SendText inside a function whose
// name ends in "Usage" and that takes a *users.UserRecord uses
// CategoryUsage. A hit is converted, never allow-listed by file.
func usageCategoryHits(t *testing.T, rel string, src []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	var hits []string
	at := func(n ast.Node) string { return rel + `:` + strconv.Itoa(fset.Position(n.Pos()).Line) }
	checkCalls := func(root ast.Node, inUsageFunc bool) {
		ast.Inspect(root, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for i, a := range call.Args {
				sel, ok := a.(*ast.SelectorExpr)
				if !ok || !strings.HasPrefix(sel.Sel.Name, `Category`) {
					continue
				}
				if x, ok := sel.X.(*ast.Ident); !ok || x.Name != `messaging` {
					continue
				}
				if sel.Sel.Name == `CategoryUsage` {
					continue
				}
				if inUsageFunc && calledName(call) == `SendText` {
					hits = append(hits, at(call)+`: usage function sends on `+sel.Sel.Name)
					continue
				}
				if i+1 < len(call.Args) {
					if s, ok := leadLiteral(call.Args[i+1]); ok && strings.Contains(openerTag.ReplaceAllString(s, ``), `Usage:`) {
						hits = append(hits, at(call)+`: "Usage:" line on `+sel.Sel.Name)
					}
				}
			}
			return true
		})
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		checkCalls(fd.Body, strings.HasSuffix(fd.Name.Name, `Usage`) && takesUserRecord(fd))
	}
	return hits
}

func takesUserRecord(fd *ast.FuncDecl) bool {
	for _, p := range fd.Type.Params.List {
		if star, ok := p.Type.(*ast.StarExpr); ok {
			if sel, ok := star.X.(*ast.SelectorExpr); ok && sel.Sel.Name == `UserRecord` {
				return true
			}
		}
	}
	return false
}

func TestUsageRidesCategoryUsage(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal(`runtime.Caller failed`)
	}
	root := filepath.Dir(here)
	scanned := 0
	for _, top := range []string{`internal`, `modules`} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, `.go`) || strings.HasSuffix(path, `_test.go`) {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			for _, h := range usageCategoryHits(t, filepath.ToSlash(rel), src) {
				t.Error(h)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 100 {
		t.Fatalf(`scanned only %d Go files; the walk is broken`, scanned)
	}
}

func TestUsageGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"func a(u U) { u.SendText(messaging.CategorySystem, `Usage: kick <name>`) }\n" +
		"func b(u U) { u.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg=\"yellow\">Usage: %s</ansi>`, x)) }\n" +
		"func fooUsage(user *users.UserRecord) { user.SendText(messaging.CategorySystem, txt) }\n" +
		"func c(u U) { u.SendText(messaging.CategoryUsage, `Usage: kick <name>`) }\n" +
		"func barUsage(user *users.UserRecord) { user.SendText(messaging.CategoryUsage, txt) }\n" +
		"func d(u U) { u.SendText(messaging.CategorySystem, `You kick it.`) }\n"
	hits := usageCategoryHits(t, `probe.go`, []byte(src))
	want := []string{`probe.go:2:`, `probe.go:3:`, `probe.go:4:`}
	if len(hits) != len(want) {
		t.Fatalf(`want %d hits, got %v`, len(want), hits)
	}
	for i, w := range want {
		if !strings.HasPrefix(hits[i], w) {
			t.Fatalf(`hit %d: want %s, got %s`, i, w, hits[i])
		}
	}
}
```

It reuses `calledName` (`aim_refusal_category_guard_test.go`), `leadLiteral` and `openerTag` (`refusal_system_opener_guard_test.go`), all `package main`.

- [ ] **Step 2: Run it (red).** `go test . -run 'TestUsage'` Expected: `CanFail` PASS, `RidesCategoryUsage` FAIL listing usage sends. Record the count.

- [ ] **Step 3: Convert.** Every USAGE row in both ledgers moves to `messaging.CategoryUsage` (category only). A usage line sent through a template (`templates.Process("help/command.x", ...)`, ruled USAGE in the admin ledger) moves too even if the guard cannot see it. A guard hit on a row ruled otherwise: read it; a usage line moves, anything else narrows the rule (report it).

- [ ] **Step 4: Run.** `go test . ./internal/... ./modules/...` Expected: PASS.

- [ ] **Step 5: Commit.** `refactor(messaging): usage lines ride CategoryUsage (#371)`.

### Task 3: The admin guard

**Files:** Create root `admin_category_guard_test.go`.

- [ ] **Step 1: Write the guard.**

```go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Messaging M7 slice 4 (#371): staff output rides messaging.CategoryAdmin.
// The admin set is DERIVED from the command registry, never from file names
// (admin.bounty.go is a player command): the handlers registered admin-only,
// and every function or method in the same package they reach, minus
// anything a player command also reaches. None of those may pass
// messaging.CategorySystem. Staff-only subcommands of player commands
// (party admin, weather's admin subcommands, bounty declare) are outside
// what a registry can see; the admin ledger covers them.

type pkgSrc struct {
	files map[string]*ast.File // rel path -> file
	fset  *token.FileSet
}

func parsePkg(t *testing.T, root, dir string) pkgSrc {
	t.Helper()
	p := pkgSrc{files: map[string]*ast.File{}, fset: token.NewFileSet()}
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, `.go`) || strings.HasSuffix(n, `_test.go`) {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(dir, n))
		f, err := parser.ParseFile(p.fset, rel, mustRead(t, filepath.Join(root, rel)), 0)
		if err != nil {
			t.Fatal(err)
		}
		p.files[rel] = f
	}
	return p
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// handlerName is the function or method a registration names.
func handlerName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	}
	return ``
}

// registrations returns the admin and player handler names a package
// registers: userCommands map entries {Func, _, _, adminOnly}, and
// AddUserCommand(name, fn, _, adminOnly) / RegisterCommand(name, fn, _, _, adminOnly) calls.
func registrations(p pkgSrc) (admin, player map[string]bool) {
	admin, player = map[string]bool{}, map[string]bool{}
	mark := func(fn ast.Expr, flag ast.Expr) {
		name := handlerName(fn)
		if name == `` {
			return
		}
		if id, ok := flag.(*ast.Ident); ok && id.Name == `true` {
			admin[name] = true
		} else {
			player[name] = true
		}
	}
	for _, f := range p.files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.CompositeLit:
				if id, ok := v.Type.(*ast.Ident); ok && id.Name == `CommandAccess` && len(v.Elts) == 4 {
					mark(v.Elts[0], v.Elts[3])
				}
				if mt, ok := v.Type.(*ast.MapType); ok {
					if id, ok := mt.Value.(*ast.Ident); ok && id.Name == `CommandAccess` {
						for _, el := range v.Elts {
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								if cl, ok := kv.Value.(*ast.CompositeLit); ok && len(cl.Elts) == 4 {
									mark(cl.Elts[0], cl.Elts[3])
								}
							}
						}
					}
				}
			case *ast.CallExpr:
				switch calledName(v) {
				case `AddUserCommand`:
					if len(v.Args) == 4 {
						mark(v.Args[1], v.Args[3])
					}
				case `RegisterCommand`:
					if len(v.Args) == 5 {
						mark(v.Args[1], v.Args[4])
					}
				}
			}
			return true
		})
	}
	return admin, player
}

// funcIndex maps a function or method name to its declarations.
func funcIndex(p pkgSrc) map[string][]*ast.FuncDecl {
	idx := map[string][]*ast.FuncDecl{}
	for _, f := range p.files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				idx[fd.Name.Name] = append(idx[fd.Name.Name], fd)
			}
		}
	}
	return idx
}

// reach is every declaration reachable from roots by name (calls, method
// values and function values alike), within the package.
func reach(idx map[string][]*ast.FuncDecl, roots map[string]bool) map[*ast.FuncDecl]bool {
	seen := map[*ast.FuncDecl]bool{}
	var queue []*ast.FuncDecl
	for r := range roots {
		queue = append(queue, idx[r]...)
	}
	for len(queue) > 0 {
		fd := queue[0]
		queue = queue[1:]
		if seen[fd] {
			continue
		}
		seen[fd] = true
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			var name string
			switch v := n.(type) {
			case *ast.Ident:
				name = v.Name
			case *ast.SelectorExpr:
				name = v.Sel.Name
			default:
				return true
			}
			queue = append(queue, idx[name]...)
			return true
		})
	}
	return seen
}

func adminSystemHits(p pkgSrc) (hits []string, adminFuncs int) {
	admin, player := registrations(p)
	idx := funcIndex(p)
	staff, open := reach(idx, admin), reach(idx, player)
	for fd := range staff {
		if open[fd] {
			continue
		}
		adminFuncs++
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == `CategorySystem` {
				hits = append(hits, p.fset.Position(sel.Pos()).String()+`: `+fd.Name.Name+` (reached only from admin-only commands) sends on CategorySystem`)
			}
			return true
		})
	}
	sort.Strings(hits)
	return hits, adminFuncs
}

func TestAdminOutputRidesCategoryAdmin(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal(`runtime.Caller failed`)
	}
	root := filepath.Dir(here)
	total := 0
	for _, dir := range []string{`internal/usercommands`, `modules/aicompanion`, `modules/playtest`} {
		hits, n := adminSystemHits(parsePkg(t, root, dir))
		total += n
		for _, h := range hits {
			t.Error(h)
		}
	}
	if total < 50 {
		t.Fatalf(`found only %d admin-only functions; the registry reader is broken`, total)
	}
}

func TestAdminGuardCanFail(t *testing.T) {
	src := "package p\n" +
		"var userCommands = map[string]CommandAccess{\n" +
		"  `zap`: {Zap, false, true, true},\n" +
		"  `look`: {Look, false, true, false},\n" +
		"}\n" +
		"func Zap(u U) { zapHelper(u); shared(u) }\n" +
		"func zapHelper(u U) { u.SendText(messaging.CategorySystem, `zapped`) }\n" +
		"func Look(u U) { shared(u) }\n" +
		"func shared(u U) { u.SendText(messaging.CategorySystem, `a report`) }\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, `probe.go`, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	hits, _ := adminSystemHits(pkgSrc{files: map[string]*ast.File{`probe.go`: f}, fset: fset})
	if len(hits) != 1 || !strings.Contains(hits[0], `probe.go:7`) || !strings.Contains(hits[0], `zapHelper`) {
		t.Fatalf(`want one hit in zapHelper, got %v`, hits)
	}
}
```

`strconv` is used only if you format positions by hand; drop the import if `go vet` reports it unused.

- [ ] **Step 2: Run it (red).** `go test . -run 'TestAdmin'` Expected: `CanFail` PASS, `RidesCategoryAdmin` FAIL with the admin ledger's ADMIN and staff REFUSAL sends in admin-only code. Compare the hit count against the ledger rows for files holding admin-only handlers; a hit the ledger rules NOT, or a ledger ADMIN row with no hit, is read and explained in the report (a helper shared with a player command is the expected cause).

- [ ] **Step 3: Commit the guard with Task 4** (it stays red until Task 4 converts the sites; do not commit a red guard alone). Keep the file uncommitted, or commit both together.

### Task 4: Staff output rides CategoryAdmin

**Files:** every admin-ledger row whose final category is `Admin`; `modules/weather/weather.go`, `weather_commands.go`.

- [ ] **Step 1: Convert.** For each admin-ledger row: final category `Admin` changes `messaging.CategorySystem` to `messaging.CategoryAdmin`; `Refusal` (the four player-reachable refusals) to `messaging.CategoryRefusal`; `System` stays. The `party admin` section of `party.go`, `bounty`'s staff subcommands and `actions/bauble_admin.go:38` are converted from the ledger though the guard cannot see them. `admin.zap.go:60` (slice 3 moved it to `CategoryRefusal`) moves to `CategoryAdmin` under R2. The NOTICE and ROOM rows that sit in admin-only code (`ban.go:90`, `boot.go:37`, `admin.modify.go:74`, `admin.skillset.go:69,86`, `admin.locate.go:41`, `admin.zap.go:46,90`, `admin.paz.go:45`, `admin.teleport.go:109,134`) take their table B category in this task, or the admin guard stays red; `admin.skillset.go:69,86` branch on whether the target is the typist (`CategoryAdmin`) or another player (`CategoryWarning`). Task 5 then skips them.

- [ ] **Step 2: Weather.** `sendLine(user, text)` becomes `sendLine(user, cat messaging.Category, text)`; the player report (`printLocalWeather`, `weather_commands.go` ~74-94) passes `CategorySystem`; the admin subcommands pass `CategoryAdmin`; `adminUsage` and the `weather spawn` usage line pass `CategoryUsage`. Read every caller (`grep -n sendLine modules/weather/*.go`).

- [ ] **Step 3: Run.** `go test . -run 'TestAdmin|TestUsage|TestRefusal'` then `go test . ./internal/... ./modules/...` Expected: PASS. Tests asserting `CategorySystem` on a converted staff line are updated; list each.

- [ ] **Step 4: Commit** the guard and the conversion: `refactor(messaging): staff output rides CategoryAdmin; admin guard (#371)`.

### Task 5: Narration and notices leave System

**Files:** the sites in Wording table B; `special_move_categories_test.go`; the goldens of Fact 15.

- [ ] **Step 1: Convert table B.** For each site, read the function: if the same event's other roles already ride a category, use it (report any disagreement with table B instead of guessing); else the table's category. Emote versus MobEmote is decided by the other party's kind (a mob actor or a player), with a branch where one function serves both.
- [ ] **Step 2: Tighten the special-move test (R4).** `systemNoticeMarkers` drops `"crumbles apart"` (throw.go:276 moved); its comment says the shortage notice (`combat.ChannelDefenceShortageText`) is the one notice left on System and why (it rides System on every path, Fact 14).
- [ ] **Step 3: Goldens.** Run `go test .`; if `TestM2RoutingIsFrozen` or `TestNarrationSitesMatchViewpointAudit` fails only because a category in a frozen site changed, re-record per the file's documented procedure in the same commit and quote the diff lines in the commit message. Any other change in the golden is a stop-and-report.
- [ ] **Step 4: Run.** `go test . ./internal/... ./modules/...` Expected: PASS.
- [ ] **Step 5: Commit.** `refactor(messaging): narration and group notices leave CategorySystem (#371)`.

### Task 6: Words and recipients (table C)

- [ ] **Step 1:** `admin.teleport.go:134`: the line a non-staff party member reads becomes `A flash of light carries you away.` on `CategoryMobEmote`; the staff member's own report keeps the room id.
- [ ] **Step 2:** `roommanager.go:293`: "enscribed" becomes "inscribed".
- [ ] **Step 3:** `admin.teleport.go:75,86`: the permission text names the permission each branch checks.
- [ ] **Step 4:** `admin.skillset.go:92`: send the refusal to the staff member (`user`), not `targetUser`, on `CategoryAdmin`. Add a test if `admin.skillset` has a test file (`grep -ln Skillset internal/usercommands/*_test.go`); else say so in the commit.
- [ ] **Step 5:** File an issue (search first) for `weather_commands.go:93` showing every player a front's internal id and intensity numbers (Fact 17), under the copy cleanup epic #383.
- [ ] **Step 6:** Run `go test ./internal/usercommands/ ./internal/rooms/`; commit `fix(admin): teleport and skillset lines reach the right reader (#371)`.

### Task 7: Re-inventory, docs, gate, PR

- [ ] **Step 1: Re-inventory.** Run `tools/refusal_classify.py --tsv` (non-admin files) into the session scratchpad: the rows left on `CategorySystem` must be the slice 3 ledger's NOT rows (695) give or take the Task 6 changes. Run the scratch variant with the admin skip removed: in admin files, the only `CategorySystem` rows left are the admin ledger's 11 NOT rows. Report both counts.
- [ ] **Step 2: Docs.**
  - `internal/messaging/context.md`: the three categories, what each carries, the rulings.
  - `internal/refusal/context.md`: the two new guards (`usage_category_guard_test.go`, `admin_category_guard_test.go`), what each can and cannot see (staff subcommands of player commands are ledger-only), R4 and the shortage notice; replace "Slice 4" with "M7 complete".
  - `internal/usercommands/context.md`: staff output rides `CategoryAdmin`, usage `CategoryUsage`.
  - Both ledgers: a line saying slice 4 converted the rows, with the PR number.
  - `docs/README.md`: the plan and the admin ledger rows. Run `python tools/context_md_audit.py <pkg>` one package per call for the three packages.
- [ ] **Step 3: Gate** per `dogmud-shipping`: `gofmt -l`, `go build ./...`, `golangci-lint run --new-from-merge-base=origin/master` (0 issues), the full `go test . ./internal/... ./modules/...`, the boot check in a detached worktree. `-race` needs a C compiler this machine lacks: say so.
- [ ] **Step 4: Patch notes.** Players see no change; add a one-line entry only if R1 or R3 changed anything visible (they do not: same colour). Otherwise no entry.
- [ ] **Step 5: PR** with `--repo pruuk/DOGMud`, body naming slice 4 and closing #371 only if the owner says M7 is done (otherwise no closing keyword), the rulings, the ledger links and the evidence.

## Self-review

- Spec section 5: `CategoryUsage` (Task 1, 2), `CategoryAdmin` (Task 1, 4), System keeps reports/tables/help/confirmations (Task 7 re-inventory), narration moved out (Task 5), `CategoryError`/`Warning`/`Tip` meanings recorded (Task 7 docs).
- Spec section 7 tags: admin output guard (Task 3, generalised from "admin.*.go" to the registry because `bounty` and `reload` live in admin files and are player commands), usage guard (Task 2), per-file tests (R4: kept, with the reason).
- Known limits: the admin guard sees registry handlers and same-package callees only; `party admin`, `weather` admin and `bounty` staff subcommands are converted from the ledger without a guard. The usage guard sees literal-led sends and `*Usage` functions; a usage block sent through a template is converted from the ledger.
