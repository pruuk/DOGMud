# Web Client Base, Plan 3b-2: Looking at Things Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the "looking at things" screens their approved narrow forms: looking at yourself, another player, a corpse, a pet and inside a container; identify and appraise; track; biome; and the room view (title, prose, alerts, fixtures, exits, runes and signs). The 80-column output stays byte for byte as it is, except the two changes the owner approved in the mock: room alerts and the biome box become plain rules at every width.

**Architecture:** Screens rendered through `templates.Process` get `name.w42.template` and `name.w30.template` beside the base, picked by plan 3a's form machinery. The room view cannot use fixed forms, because `rooms.GetDetails` already wraps the room's prose to the reader's own width; its templates draw to `screenwidth` instead, and `GetDetails` now hands the alerts over as bare text for the template to centre between two full-width rules. Three new template functions: `centerlines` centres text between rules; `wrapspans` wraps like `wraptext` but keeps each colour span (a name, an exit title, with any text glued to it) on one line when that unit fits; `wrapbreak` wraps like `wraptext` but breaks a word longer than a whole line. Golden files recorded from master pin every unchanged 80-column screen.

**Tech Stack:** Go 1.25 `text/template`, `testify`, guards in `internal/guards`.

**Spec:** `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md`, Design section 2, rulings 3 and 22 to 25. The approved mock, with the owner's rulings of 2026-11-10: https://claude.ai/artifact/4vUBgswyRd3umYPrBmyHvZ (source `_archive/notes/reports/narrow-screens-3b-mock/` in the main checkout). Plan 3b is three plans; this is the second (owner, 2026-11-10). Part of #783.

**Needs:** plan 3b-1 (merged in #849).

---

## Facts verified against source

Read 2026-11-10 from master at `79a3d1be7`. The patch at the end was built in a worktree of that commit by a subagent; the controller applied it again to a fresh worktree of `79a3d1be7` and ran each task's red and green steps there, with the results written below as they printed. The full suite (`go test . ./internal/... ./modules/...`) had no failing package, every `tools/jstest` and `tools/webclient-tests` file passed, `gofmt` was clean and `golangci-lint --new-from-merge-base=origin/master` reported 0 issues. Making one rule in `identify.w30.template` a column too long failed the guard with `descriptions/identify (data 0) at 30 columns has a line 31 wide`.

| # | Fact | Where |
|---|---|---|
| 1 | Forms are picked widest-that-fits from `FormWidths {42, 40, 30}`; `screenwidth` is the reader's width capped at 80 | `internal/templates/forms.go` |
| 2 | Callers in `look.go`: `character/description` :119 and :149, `inventory-look` :133 and :164, `pet` :485, `description-corpse` :531, `insidecontainer` :325 and :560; the room view's `room-title` :773, `room` :776, `fixtures` :783, `rune` :817, `sign` :825, `exits` :921 (and `exits.go:15`) | `internal/usercommands/look.go` |
| 3 | `descriptions/identify` has two callers: `appraise.go:106` (CategoryScreen) and `spell_resolution.go:935` (CategorySpellMental). `biome`: `biome.go:22`. `track`: `actions/track.go:494` | grep |
| 4 | CategoryRoomDescription is never wrapped by the message pipeline, so the room view must lay itself out | `internal/messaging/pipeline.go` `shouldWrap` |
| 5 | `GetDetails` wraps the room prose to the reader's width, so a form drawn to 42 would not fit a 60-column reader; the room templates draw to `screenwidth` | `internal/rooms/roomdetails.go:134-157` |
| 6 | Alerts had their box and padding built in Go (`roomdetails.go:98-112`), and mutator alerts were centred by `centerAlert` (:43-58, :195) | `internal/rooms/roomdetails.go` |
| 7 | `TestGetDetails_MinimapSitsAtTheViewersRightEdge` pins the telnet minimap beside the prose at 60 columns | `internal/rooms/roomdetails_wrap_width_test.go` |
| 8 | The web client never draws the minimap | `internal/usercommands/look.go:713-719` |
| 9 | Nothing in Go or JS calls `Room.AddSign`, so runes and signs show only where saved room data already holds one | grep |
| 10 | `GetHealthAppearance` returns one pre-coloured string, so a template cannot keep "perfect health." whole without a Go change | `internal/characters` |

## Choices made while building, for your review

1. **The room view has no fixed forms** (fact 5). Its title, prose, alerts, fixtures, exits, runes and signs draw to the reader's own width, up to 80. Fixed 42 and 30 forms would first need the prose wrap in `GetDetails` to change.
2. **The telnet minimap moves above the prose at 42 columns and below.** From 43 to 79 it stays beside the prose, as an existing test pins (fact 7). The web never shows it (fact 8).
3. **Runes and signs** (not in the mock, and rare: fact 9) keep today's line when it fits. A line too wide drops its 11-space indent and continues 2 in; a single word longer than a whole line is broken (`wrapbreak`):

```
42: A sign reads: [Beware the marsh at        30: A rune reads: [Beware the
      night. The lights you see out there are       marsh at night. The lights
      not lanterns, and they do not want you        you see out there are not
      home.]                                        lanterns, and they do not
                                                    want you home.]
```

4. **Alert wrapping:** an alert sits on one line when it fits, keeping its author's spacing; otherwise it breaks at each sentence end and at each run of two or more spaces, when every piece fits; otherwise it collapses the space runs and word-wraps. This reproduces every drawn alert, and the shipped wildfire alert reads `!!!` / `A wildfire is burning here!` / `!!!` at 30 and 42 (tested from its own YAML and colour pattern). At 30:

```
──────────────────────────────
  This is a bank! Type bank
     to deposit/withdraw.
──────────────────────────────
```

5. **Units kept whole:** an exit with its tag (`northeast (locked),`) and the room title's `☠ PK Area ☠` never split unless longer than a whole line. At 30 a long title puts the zone and the PK tag on their own lines, 3 in.
6. **Looking at yourself at 30** breaks the coloured health phrase (`Screenly is in perfect` / `health.`); the drawing kept "perfect health." whole. Doing that needs a Go change to how the health phrase is built (fact 10), so it is left.
7. **Prose after a label** (identify's Description, biome's Lighting and Description) continues 2 past the label at 42 and 30, per your ruling. The mock drew it hanging under the value at 42, so those lines differ from the drawing. At 30 the first line of a wrapped value breaks one word earlier than the drawing, a limit of `wraptext` that 3b-1's status form shares.
8. **"Also here:"** wraps without a hanging indent. It comes from `descriptions/who`, which the message pipeline already wraps, and is outside this plan.
9. **Long names at 30:** the name line wraps, so shipped mobs such as "Riss the Cartographer-Scholar" fit. A single-word name of 27 to 32 letters cannot fit after `.: `, so at 30 it may run past the rule on a line where it stands alone (up to 35 wide for a 32-letter name), on looking at a person, a corpse or a pet: the same exception you accepted for status. A test allows only those lines.
10. **A pet or track screen already wider than 80 takes the 42 form on a desktop,** under plan 3a's ruling. With shipped data that needs a pet description or a trail line over 80 columns.
11. **The default world's room template is left as it is.** You ruled on 2026-09-17 that the vestigial default world is left alone, and it cannot boot (recorded at `internal/guards/shipped_narration_data_guard_test.go:278`). Its alert box loses the centring the Go strings used to carry. If you want it updated, it is a copy of the dogmud alert block.

## What changes at 80 columns (owner-approved in the mock)

- **Room alerts:** each alert's 67-wide box indented 4 becomes two 80-wide red rules with the text centred across 80 (`room_alerts.golden`, `room_view.golden`). Mutator alerts (a wildfire and the like) take the same form.
- **Biome:** the corner box becomes two 80-wide rules, and a `Lighting:` value that ran past 80 (96 columns for an interior, 86 to 97 for an outdoor biome with a lamp) wraps under its value:

```
── .: Biome Info ───────────────────────────────────────────────────────────────
  Name:        Interior
  Symbol:      ⌂
  Lighting:    Only a little sky reaches here, filtered and dim. A lamp
               keeps it lit after dark.
  Description: Inside a built structure, whether a house, a hall, a temple
               or the sealed corridors of something older. Lit by what its
               builders put there rather than by the sky.
────────────────────────────────────────────────────────────────────────────────
```

- **Room lines already over 80** (an exits line, the tracking line, title, rune, sign) now wrap at 80 instead of running past it, keeping exit names and the PK tag whole.
- **A pet or track screen already wider than 80** takes the 42 form on a desktop (choice 10).
- Every other 80-column screen is byte for byte what master renders.

These are visible to every player, so Task 4 adds a patch note.

## Files

| File | Change |
|---|---|
| Create `_datafiles/world/dogmud/templates/character/{description,description-corpse,inventory-look,pet}.{w42,w30}.template` | 8 forms |
| Create `_datafiles/world/dogmud/templates/descriptions/{biome,identify,insidecontainer,track}.{w42,w30}.template` | 8 forms |
| Modify `descriptions/{biome,exits,fixtures,room,room-title,rune,sign}.template` | Biome rules at 80; the room view drawn to `screenwidth` |
| Modify `internal/templates/templatesfunctions.go` | `centerlines`, `wrapspans`, `wrapbreak` |
| Modify `internal/rooms/roomdetails.go` | Alerts as bare text (`AlertBank` and friends); the minimap above the prose at 42 and below |
| Create `internal/guards/looking_screens_test.go`, `testdata/looking_screens_80/*.golden` (24) | Fixtures, fit and unit tests, 80-column goldens |
| Create `internal/usercommands/room_view_forms_test.go`, `testdata/narrow_forms_80/room_view.golden` | The real `lookRoom` at 30, 42 and 80 |
| Create `internal/templates/centerlines_test.go`; modify `internal/rooms/roomdetails_modifier_wrap_test.go` | `centerlines` and `wrapspans`; alerts as bare text and map placement |
| Modify `internal/usercommands/template_freeze_test.go` | The biome wording check ignores line breaks |
| Modify `internal/guards/template_width_guard_test.go` | The 14 templates leave "plan 3b"; the rest is renamed "plan 3b-3" |
| Modify `internal/{templates,rooms,usercommands,guards}/context.md`, `docs/README.md`, `docs/PATCH_NOTES.md` | Docs |

Edits land only by `git apply` or the Edit and Write tools; never `sed -i`, a `>` into a repo file, or Python writing a repo file (a hook enforces this). `$S` is the full path of the session scratchpad; set it at the start of every Bash call that uses it. `git apply` prints whitespace warnings for the goldens' trailing spaces; those are the recorded bytes and are expected.

---

### Task 0: Branch and the patch

- [ ] **Step 1:**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add C:/tmp/dogmud-3b2-build -b feature/webclient-base-3b2-looking-at-things origin/master
```

`C:/tmp/dogmud-3b2-build` is named here so the end-of-day sweep keeps it; remove it with `git worktree remove` once the PR merges.

- [ ] **Step 2:** Write this script to `$S/extract_patch.py` with the Write tool (skip if it exists with this content):

```python
import re, sys
text = open(sys.argv[1], encoding="utf-8").read().replace("\r\n", "\n")
blocks = re.findall(r"^````diff\n(.*?)^````$", text, re.S | re.M)
assert len(blocks) == 1, len(blocks)
open(sys.argv[2], "w", encoding="utf-8", newline="\n").write(blocks[0])
```

```bash
S="<the session scratchpad, full path>"
python -I "$S/extract_patch.py" "/c/Users/Calabe Davis/workspace/DOGMud/docs/superpowers/plans/2026-11-10-webclient-base-3b2-looking-at-things.md" "$S/3b2.patch"
cd C:/tmp/dogmud-3b2-build && git apply --check "$S/3b2.patch" && echo CHECK-OK
```

Expected: `CHECK-OK`. If `origin/master` has moved past `79a3d1be7` and the check fails, stop and report.

---

### Task 1: The tests, red

```bash
cd C:/tmp/dogmud-3b2-build; S="<scratchpad>"; P="$S/3b2.patch"
git apply --include='internal/guards/looking_screens_test.go' --include='internal/guards/testdata/looking_screens_80/*' --include='internal/usercommands/room_view_forms_test.go' --include='internal/usercommands/testdata/narrow_forms_80/room_view.golden' --include='internal/templates/centerlines_test.go' --include='internal/rooms/roomdetails_modifier_wrap_test.go' --include='internal/usercommands/template_freeze_test.go' "$P"
go vet ./internal/guards/ ./internal/templates/ ./internal/rooms/ 2>&1 | grep undefined
```

Expected, these three lines in any order (Windows may prefix some with `vet.exe: `):

```
internal\rooms\roomdetails_modifier_wrap_test.go:54:20: undefined: AlertBank
internal\templates\centerlines_test.go:28:11: undefined: centerLines
internal\guards\looking_screens_test.go:289:32: undefined: rooms.AlertBank
```

No commit yet.

---

### Task 2: The Go support

```bash
git apply --include=internal/templates/templatesfunctions.go --include=internal/rooms/roomdetails.go "$P"
go build ./... && echo BUILD-OK
go test ./internal/templates/ ./internal/rooms/ -count=1 2>&1 | tail -2
go test ./internal/guards/ -run TestLookingScreensKeepTheir80ColumnOutput -count=1 2>&1 | grep -- '--- FAIL'
go test ./internal/usercommands/ -run TestRoomViewKeepsIts80ColumnOutput -count=1 2>&1 | grep -- '^--- FAIL'
```

Expected: `BUILD-OK`; `ok` for templates and rooms. Only the four goldens recorded for the approved 80-column changes fail, which proves every other golden matches master's templates:

```
--- FAIL: TestLookingScreensKeepTheir80ColumnOutput
    --- FAIL: TestLookingScreensKeepTheir80ColumnOutput/room_alerts
    --- FAIL: TestLookingScreensKeepTheir80ColumnOutput/biome_outdoor
    --- FAIL: TestLookingScreensKeepTheir80ColumnOutput/biome_interior
    --- FAIL: TestLookingScreensKeepTheir80ColumnOutput/biome_street
--- FAIL: TestRoomViewKeepsIts80ColumnOutput
```

---

### Task 3: The forms and the guard

```bash
git apply --include='_datafiles/world/dogmud/templates/*' --include=internal/guards/template_width_guard_test.go "$P"
echo "gofmt:[$(gofmt -l internal/)]"
go test ./internal/usercommands/ ./internal/templates/ ./internal/guards/ ./internal/rooms/ -count=1 2>&1 | tail -4
```

Expected: `gofmt:[]`; `ok` four times.

Commit Tasks 1 to 3 together, with named paths:

```bash
T=_datafiles/world/dogmud/templates
git add $T/character/description.w42.template $T/character/description.w30.template $T/character/description-corpse.w42.template $T/character/description-corpse.w30.template $T/character/inventory-look.w42.template $T/character/inventory-look.w30.template $T/character/pet.w42.template $T/character/pet.w30.template $T/descriptions/biome.template $T/descriptions/biome.w42.template $T/descriptions/biome.w30.template $T/descriptions/identify.w42.template $T/descriptions/identify.w30.template $T/descriptions/insidecontainer.w42.template $T/descriptions/insidecontainer.w30.template $T/descriptions/track.w42.template $T/descriptions/track.w30.template $T/descriptions/exits.template $T/descriptions/fixtures.template $T/descriptions/room.template $T/descriptions/room-title.template $T/descriptions/rune.template $T/descriptions/sign.template internal/templates/templatesfunctions.go internal/templates/centerlines_test.go internal/rooms/roomdetails.go internal/rooms/roomdetails_modifier_wrap_test.go internal/guards/looking_screens_test.go internal/guards/testdata/looking_screens_80 internal/guards/template_width_guard_test.go internal/usercommands/room_view_forms_test.go internal/usercommands/testdata/narrow_forms_80/room_view.golden internal/usercommands/template_freeze_test.go
git commit -F - <<'EOF'
feat(screens): narrow forms for looking at things

Looking at people, pets, corpses and containers, identify and appraise,
track and biome get their approved 42- and 30-column forms. The room view
draws to the reader's width; its alerts and the biome box become plain
rules at every width, as approved in the mock. Goldens recorded from master
pin every other 80-column screen.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git status --short
```

Expected: `git status --short` prints nothing.

---

### Task 4: Docs and the patch note

- [ ] **Step 1:** Apply the docs:

```bash
git apply --include=docs/README.md --include=internal/templates/context.md --include=internal/rooms/context.md --include=internal/usercommands/context.md --include=internal/guards/context.md "$P"
git diff --numstat
```

Expected: README 6 added 1 removed; guards context 4 and 2; rooms 9 and 3; templates 31 and 1; usercommands 6 and 1.

- [ ] **Step 2:** With the Edit tool, add this section to `docs/PATCH_NOTES.md` directly under the `# DOGMud Patch Notes` heading (dated the day you run it):

```markdown
## 2026-11-10: Room alerts and biome info

- The bank and storage notices in a room, and the biome screen, now sit
  between plain lines instead of in a box, and long lines in them wrap to
  fit your screen.
```

- [ ] **Step 3:** Commit, with the plan itself:

```bash
cp "/c/Users/Calabe Davis/workspace/DOGMud/docs/superpowers/plans/2026-11-10-webclient-base-3b2-looking-at-things.md" docs/superpowers/plans/
git add docs/README.md docs/PATCH_NOTES.md internal/templates/context.md internal/rooms/context.md internal/usercommands/context.md internal/guards/context.md docs/superpowers/plans/2026-11-10-webclient-base-3b2-looking-at-things.md
git diff --cached --stat
git commit -F - <<'EOF'
docs(screens): plan 3b-2, the docs and the patch note for looking at things

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git apply --check -R "$P" && echo REVERSE-OK
```

Expected: the stat lists the seven files; `REVERSE-OK`.

---

### Task 5: Verify and ship

- [ ] **Step 1: Prove the guard can fail.** In `descriptions/identify.w30.template`, change the Basic Info rule's `(sub $w 16)` to `(sub $w 15)`. Run `go test ./internal/guards/ -run Fit -count=1`. Expected: `descriptions/identify (data 0) at 30 columns has a line 31 wide`. Undo that one edit exactly (the file also has a `(sub $w 15)` on its Modifiers rule; leave that); `git status --short` is empty.

- [ ] **Step 2: Full gates.**

```bash
gofmt -l internal/ modules/
go build ./...
go test . ./internal/... ./modules/... -count=1 2>&1 | grep -vE '^ok|no test files' | tail -20
for t in tools/jstest/*.test.js tools/webclient-tests/*.js; do node "$t" > /dev/null 2>&1 || echo "RED $t"; done; echo done
golangci-lint run --new-from-merge-base=origin/master
```

Expected: no gofmt output, the build passes, no test lines, `done` with no `RED`, `0 issues.`

- [ ] **Step 3: Boot check** on private ports (the `dogmud-shipping` skill), building to `C:/tmp/dogmud-3b2-build/boot-check.exe`.

- [ ] **Step 4: STOP for the owner's manual test (owner, 2026-11-11).** Leave the boot-check server running on its private ports and hand the owner a punchlist covering web and telnet. Telnet: `set linewidth 40` (the 30 forms), `set linewidth 42`, and a width from 43 to 79 (the minimap stays beside the prose); at each, `look` in Thornwall Bank (room 510), `look` at yourself, another player, a pet, a corpse and inside a container, `appraise` an item at a shop, `identify`, `track`, and `biome`. Web at 80: the bank's alerts, `biome`, and a room whose exits line ran past 80. Nothing is pushed until the owner reports back; fix what the owner finds, re-run Step 2, and stop again.

- [ ] **Step 5: Push and open the PR** with `gh pr create --repo pruuk/DOGMud --base master --head feature/webclient-base-3b2-looking-at-things --body-file "$S/pr3b2-body.md"`: what players see (on a desktop, the room alerts and the biome screen as plain rules; telnet with `set linewidth` from 40 to 79 sees the forms; the web's narrow forms wait for plan 5), the choices above, what was tested, "Part of #783", no closing keyword, ending with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Merge only when the owner says.

---

## Self-review against the mock and rulings

| Ruling or drawing | Where |
|---|---|
| Horizontal rules only, full width, no text past a rule | the 16 forms; guard fit tests; `TestRoomViewKeepsIts80ColumnOutput` and the room view fit test check every rule's width |
| Alerts as plain rules at every width, 80 included | `roomdetails.go`, `room.template`, `centerlines`; `room_alerts.golden` |
| No minimap at 42 and below on the web; telnet narrow gets it above the prose | `roomdetails.go`; `roomdetails_modifier_wrap_test.go` |
| Biome at 80 as the approved `biome_interior.w80` drawing | `biome.template`; `biome_*.golden` |
| A screen already wider than 80 takes a form | plan 3a machinery; choice 10 |
| Long prose after a label continues 2 past the label | `identify`, `biome` forms; choice 7 |
| Names and units never split | exits, PK tag: `TestRoomViewUnitsStayWhole`; item names in identify and container lists |
| 80 unchanged otherwise | 20 goldens recorded from master pass in Task 2, including the tracking line and a mix of exits (5 more record the approved changes) |
| Every room line fits, glued text counted | `TestRoomViewUnitsStayWhole` (shipped long mob names on the tracking line) |
| Long names fit or overflow only alone at 30 | `TestLookingWithTheLongestNameRunsPastOnlyOnItsName`; shipped mob names in the fit fixtures |
| The shipped wildfire alert reads whole | `TestShippedWildfireAlertReadsWhole` |

## The patch

Applied in the tasks above, never by hand.

````diff
diff --git a/_datafiles/world/dogmud/templates/character/description-corpse.w30.template b/_datafiles/world/dogmud/templates/character/description-corpse.w30.template
new file mode 100644
index 000000000..01be2c3c9
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/description-corpse.w30.template
@@ -0,0 +1,10 @@
+{{- /* Looking at a corpse, at 30 columns: the description between two
+full-width rules, framed by two rows of 15 skulls, its prose wrapped 1 in. */ -}}
+{{ $w := 30 }}{{ $ind := " " }}
+<ansi fg="black-bold">.:</ansi> <ansi fg="8">{{ wraptext (print .Name " corpse") (sub $w 3) "   " }}</ansi>
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> {{ repeat "─" (sub $w 17) }}
+{{ $ind }}<ansi fg="red-bold">{{ repeat "☠ " 14 }}☠</ansi>
+{{ $ind }}<ansi fg="8">{{ wraptext .GetDescription (sub $w (len $ind)) $ind }}</ansi>
+{{ $ind }}<ansi fg="red-bold">{{ repeat "☠ " 14 }}☠</ansi>
+{{ $ind }}<ansi fg="8">{{ wraptext "This is a corpse. They are dead." (sub $w (len $ind)) $ind }}</ansi>
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/description-corpse.w42.template b/_datafiles/world/dogmud/templates/character/description-corpse.w42.template
new file mode 100644
index 000000000..e5a0cd7e7
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/description-corpse.w42.template
@@ -0,0 +1,11 @@
+{{- /* Looking at a corpse, at 42 columns: the description between two
+full-width rules, framed by two rows of 20 skulls, its prose wrapped 3 in.
+Drawn to 42 whatever the reader's width. */ -}}
+{{ $w := 42 }}{{ $ind := "   " }}
+<ansi fg="black-bold">.:</ansi> <ansi fg="8">{{ wraptext (print .Name " corpse") (sub $w 3) "   " }}</ansi>
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> {{ repeat "─" (sub $w 17) }}
+{{ $ind }}<ansi fg="red-bold">{{ repeat "☠ " 19 }}☠</ansi>
+{{ $ind }}<ansi fg="8">{{ wraptext .GetDescription (sub $w (len $ind)) $ind }}</ansi>
+{{ $ind }}<ansi fg="red-bold">{{ repeat "☠ " 19 }}☠</ansi>
+{{ $ind }}<ansi fg="8">This is a corpse. They are dead.</ansi>
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/description.w30.template b/_datafiles/world/dogmud/templates/character/description.w30.template
new file mode 100644
index 000000000..41cbe6730
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/description.w30.template
@@ -0,0 +1,11 @@
+{{- /* Looking at a person or a creature, at 30 columns: the description
+between two full-width rules, its prose wrapped 1 in. */ -}}
+{{ $w := 30 }}{{ $ind := " " }}
+<ansi fg="black-bold">.:</ansi> <ansi fg="username">{{ wraptext .Name (sub $w 3) "   " }}</ansi>
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> {{ repeat "─" (sub $w 17) }}
+{{ $ind }}{{ wraptext .GetDescription (sub $w (len $ind)) $ind }}
+{{- if .GetMutationVisuals }}
+{{ $ind }}<ansi fg="magenta">{{ wraptext .GetMutationVisuals (sub $w (len $ind)) $ind }}</ansi>
+{{- end }}
+{{ $ind }}{{ wraptext .GetHealthAppearance (sub $w (len $ind)) $ind }}
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/description.w42.template b/_datafiles/world/dogmud/templates/character/description.w42.template
new file mode 100644
index 000000000..591082ff6
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/description.w42.template
@@ -0,0 +1,13 @@
+{{- /* Looking at a person or a creature, at 42 columns: the description
+between two full-width rules, its prose wrapped 3 in. The form is drawn to
+42 whatever the reader's width, so a reader narrower than that gets the
+30-column form. */ -}}
+{{ $w := 42 }}{{ $ind := "   " }}
+<ansi fg="black-bold">.:</ansi> <ansi fg="username">{{ wraptext .Name (sub $w 3) "   " }}</ansi>
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> {{ repeat "─" (sub $w 17) }}
+{{ $ind }}{{ wraptext .GetDescription (sub $w (len $ind)) $ind }}
+{{- if .GetMutationVisuals }}
+{{ $ind }}<ansi fg="magenta">{{ wraptext .GetMutationVisuals (sub $w (len $ind)) $ind }}</ansi>
+{{- end }}
+{{ $ind }}{{ wraptext .GetHealthAppearance (sub $w (len $ind)) $ind }}
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/inventory-look.w30.template b/_datafiles/world/dogmud/templates/character/inventory-look.w30.template
new file mode 100644
index 000000000..2df8ec563
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/inventory-look.w30.template
@@ -0,0 +1,37 @@
+{{- /* Looking at what a person or creature wears, at 30 columns: the
+equipment between two full-width rules, one slot to a line 1 in, an item
+that does not fit beside its slot label on the next line 2 past the label. */ -}}
+{{- define "slot" }}{{ if le (add (len .ind) 12 (visiblewidth .v)) .w }}{{ .ind }}<ansi fg="yellow">{{ padRight 12 .label }}</ansi><ansi fg="itemname">{{ .v }}</ansi>{{ else }}{{ .ind }}<ansi fg="yellow">{{ .label }}</ansi>
+{{ .ind }}  <ansi fg="itemname">{{ wraptext .v (sub .w (len .ind) 2) (print .ind "  ") }}</ansi>{{ end }}
+{{ end -}}
+{{- $w := 30 }}{{ $ind := " " }}{{ if not .HideEquipment -}}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Equipment</ansi> {{ repeat "─" (sub $w 15) }}
+{{ if not .Equipment.Weapon.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Weapon:" "v" .Equipment.Weapon.NameSimple) }}{{ end -}}
+{{- if and (not .Equipment.Offhand.IsDisabled) (not (.Character.IsBlockedBy2H "offhand")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Offhand:" "v" .Equipment.Offhand.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraArm1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 3:" "v" .Equipment.ExtraArm1.NameSimple) }}{{ end -}}
+{{- if and (not .Equipment.ExtraArm2.IsDisabled) (not (.Character.IsBlockedBy2H "extra arm 2")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 4:" "v" .Equipment.ExtraArm2.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraArm3.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 5:" "v" .Equipment.ExtraArm3.NameSimple) }}{{ end -}}
+{{- if and (not .Equipment.ExtraArm4.IsDisabled) (not (.Character.IsBlockedBy2H "extra arm 4")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 6:" "v" .Equipment.ExtraArm4.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Head.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Head:" "v" .Equipment.Head.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Neck.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Neck:" "v" .Equipment.Neck.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Shoulders.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Shoulders:" "v" .Equipment.Shoulders.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Body.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Body:" "v" .Equipment.Body.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Back.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Back:" "v" .Equipment.Back.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Belt.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Belt:" "v" .Equipment.Belt.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Wrist1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 1:" "v" .Equipment.Wrist1.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Wrist2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 2:" "v" .Equipment.Wrist2.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 3:" "v" .Equipment.ExtraWrist1.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 4:" "v" .Equipment.ExtraWrist2.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist3.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 5:" "v" .Equipment.ExtraWrist3.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist4.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 6:" "v" .Equipment.ExtraWrist4.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Gloves.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Gloves:" "v" .Equipment.Gloves.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Ring.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Ring:" "v" .Equipment.Ring.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Ring2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Ring:" "v" .Equipment.Ring2.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Legs.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Legs:" "v" .Equipment.Legs.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Feet.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Feet:" "v" .Equipment.Feet.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Tail.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Tail:" "v" .Equipment.Tail.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Light.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Light:" "v" .Equipment.Light.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ComponentBag.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Components:" "v" .Equipment.ComponentBag.NameSimple) }}{{ end -}}
+{{ repeat "─" $w }}
+{{- end }}
+ Carrying: {{ $itmCt := len .ItemNames }}{{ if eq $itmCt 0 }}no{{ else if lt $itmCt 4 }}a few{{ else if lt $itmCt 7 }}several{{ else }}lots of{{ end }} objects
diff --git a/_datafiles/world/dogmud/templates/character/inventory-look.w42.template b/_datafiles/world/dogmud/templates/character/inventory-look.w42.template
new file mode 100644
index 000000000..14b83035c
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/inventory-look.w42.template
@@ -0,0 +1,38 @@
+{{- /* Looking at what a person or creature wears, at 42 columns: the
+equipment between two full-width rules, one slot to a line, an item that
+does not fit beside its slot label on the next line 2 past the label. Drawn
+to 42 whatever the reader's width. */ -}}
+{{- define "slot" }}{{ if le (add (len .ind) 12 (visiblewidth .v)) .w }}{{ .ind }}<ansi fg="yellow">{{ padRight 12 .label }}</ansi><ansi fg="itemname">{{ .v }}</ansi>{{ else }}{{ .ind }}<ansi fg="yellow">{{ .label }}</ansi>
+{{ .ind }}  <ansi fg="itemname">{{ wraptext .v (sub .w (len .ind) 2) (print .ind "  ") }}</ansi>{{ end }}
+{{ end -}}
+{{- $w := 42 }}{{ $ind := "   " }}{{ if not .HideEquipment -}}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Equipment</ansi> {{ repeat "─" (sub $w 15) }}
+{{ if not .Equipment.Weapon.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Weapon:" "v" .Equipment.Weapon.NameSimple) }}{{ end -}}
+{{- if and (not .Equipment.Offhand.IsDisabled) (not (.Character.IsBlockedBy2H "offhand")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Offhand:" "v" .Equipment.Offhand.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraArm1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 3:" "v" .Equipment.ExtraArm1.NameSimple) }}{{ end -}}
+{{- if and (not .Equipment.ExtraArm2.IsDisabled) (not (.Character.IsBlockedBy2H "extra arm 2")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 4:" "v" .Equipment.ExtraArm2.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraArm3.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 5:" "v" .Equipment.ExtraArm3.NameSimple) }}{{ end -}}
+{{- if and (not .Equipment.ExtraArm4.IsDisabled) (not (.Character.IsBlockedBy2H "extra arm 4")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 6:" "v" .Equipment.ExtraArm4.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Head.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Head:" "v" .Equipment.Head.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Neck.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Neck:" "v" .Equipment.Neck.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Shoulders.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Shoulders:" "v" .Equipment.Shoulders.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Body.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Body:" "v" .Equipment.Body.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Back.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Back:" "v" .Equipment.Back.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Belt.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Belt:" "v" .Equipment.Belt.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Wrist1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 1:" "v" .Equipment.Wrist1.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Wrist2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 2:" "v" .Equipment.Wrist2.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 3:" "v" .Equipment.ExtraWrist1.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 4:" "v" .Equipment.ExtraWrist2.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist3.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 5:" "v" .Equipment.ExtraWrist3.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist4.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 6:" "v" .Equipment.ExtraWrist4.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Gloves.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Gloves:" "v" .Equipment.Gloves.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Ring.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Ring:" "v" .Equipment.Ring.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Ring2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Ring:" "v" .Equipment.Ring2.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Legs.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Legs:" "v" .Equipment.Legs.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Feet.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Feet:" "v" .Equipment.Feet.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Tail.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Tail:" "v" .Equipment.Tail.NameSimple) }}{{ end -}}
+{{- if not .Equipment.Light.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Light:" "v" .Equipment.Light.NameSimple) }}{{ end -}}
+{{- if not .Equipment.ComponentBag.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Components:" "v" .Equipment.ComponentBag.NameSimple) }}{{ end -}}
+{{ repeat "─" $w }}
+{{- end }}
+ Carrying: {{ $itmCt := len .ItemNames }}{{ if eq $itmCt 0 }}no{{ else if lt $itmCt 4 }}a few{{ else if lt $itmCt 7 }}several{{ else }}lots of{{ end }} objects
diff --git a/_datafiles/world/dogmud/templates/character/pet.w30.template b/_datafiles/world/dogmud/templates/character/pet.w30.template
new file mode 100644
index 000000000..e701baa6c
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/pet.w30.template
@@ -0,0 +1,28 @@
+{{- /* Looking at a pet, at 30 columns: the description between two
+full-width rules, its prose wrapped 1 in, then what it carries, wrapped as
+at 42. */ -}}
+{{- define "carrying" -}}
+{{- $w := .w -}}{{- $col := add (len .ind) (len .label) -}}{{- $hang := add $col 1 -}}
+{{- if lt (sub $w $hang) 20 -}}{{- $hang = add (len .ind) 2 -}}{{- end -}}
+{{- $short := add (len .ind) 2 -}}{{- range .items -}}{{- if gt (add (visiblewidth .DisplayName) 1) (sub $w $hang) -}}{{- $hang = $short -}}{{- end -}}{{- end -}}
+{{- $pad := repeat " " $hang -}}{{- $cur := $col -}}{{- $last := sub (len .items) 1 -}}
+{{ .ind }}{{ .label }}
+{{- range $i, $itm := .items -}}
+{{- $f := $itm.DisplayName -}}{{- if lt $i $last -}}{{- $f = print $f "," -}}{{- end -}}
+{{- $n := visiblewidth $f -}}
+{{- if le (add $cur 1 $n) $w -}}
+{{ " " }}{{ $f }}{{- $cur = add $cur 1 $n -}}
+{{- else if le (add $hang $n) $w -}}
+{{ "\n" }}{{ $pad }}{{ $f }}{{- $cur = add $hang $n -}}
+{{- else -}}
+{{ "\n" }}{{ $pad }}{{ wraptext $f (sub $w $hang) $pad }}{{- $cur = $w -}}
+{{- end -}}
+{{- end }}
+{{- end -}}
+{{ $w := 30 }}{{ $ind := " " }}{{ $p := .Character.Pet }}
+<ansi fg="black-bold">.:</ansi> {{ wraptext $p.DisplayName (sub $w 3) "   " }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> {{ repeat "─" (sub $w 17) }}
+{{ $ind }}{{ wraptext (print $p.DisplayName ` is a pet <ansi fg="petname">` $p.Type `</ansi> owned by <ansi fg="username">` .Character.Name `</ansi>.`) (sub $w (len $ind)) $ind }}
+{{ $ind }}{{ wraptext (print $p.DisplayName ` hunger is: <ansi fg="hunger-` $p.Food `">` $p.Food `</ansi>`) (sub $w (len $ind)) $ind }}
+{{ repeat "─" $w }}
+{{ template "carrying" (map "w" $w "ind" " " "label" "Carrying:" "items" $p.Items) }}
\ No newline at end of file
diff --git a/_datafiles/world/dogmud/templates/character/pet.w42.template b/_datafiles/world/dogmud/templates/character/pet.w42.template
new file mode 100644
index 000000000..bff9290e1
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/pet.w42.template
@@ -0,0 +1,30 @@
+{{- /* Looking at a pet, at 42 columns: the description between two
+full-width rules, its prose wrapped 3 in, then what it carries. The list
+wraps between names, hanging under the first name when every name fits
+there, else 2 past the label; a name longer than a whole line is wrapped on
+its own. Drawn to 42 whatever the reader's width. */ -}}
+{{- define "carrying" -}}
+{{- $w := .w -}}{{- $col := add (len .ind) (len .label) -}}{{- $hang := add $col 1 -}}
+{{- if lt (sub $w $hang) 20 -}}{{- $hang = add (len .ind) 2 -}}{{- end -}}
+{{- $short := add (len .ind) 2 -}}{{- range .items -}}{{- if gt (add (visiblewidth .DisplayName) 1) (sub $w $hang) -}}{{- $hang = $short -}}{{- end -}}{{- end -}}
+{{- $pad := repeat " " $hang -}}{{- $cur := $col -}}{{- $last := sub (len .items) 1 -}}
+{{ .ind }}{{ .label }}
+{{- range $i, $itm := .items -}}
+{{- $f := $itm.DisplayName -}}{{- if lt $i $last -}}{{- $f = print $f "," -}}{{- end -}}
+{{- $n := visiblewidth $f -}}
+{{- if le (add $cur 1 $n) $w -}}
+{{ " " }}{{ $f }}{{- $cur = add $cur 1 $n -}}
+{{- else if le (add $hang $n) $w -}}
+{{ "\n" }}{{ $pad }}{{ $f }}{{- $cur = add $hang $n -}}
+{{- else -}}
+{{ "\n" }}{{ $pad }}{{ wraptext $f (sub $w $hang) $pad }}{{- $cur = $w -}}
+{{- end -}}
+{{- end }}
+{{- end -}}
+{{ $w := 42 }}{{ $ind := "   " }}{{ $p := .Character.Pet }}
+<ansi fg="black-bold">.:</ansi> {{ wraptext $p.DisplayName (sub $w 3) "   " }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> {{ repeat "─" (sub $w 17) }}
+{{ $ind }}{{ wraptext (print $p.DisplayName ` is a pet <ansi fg="petname">` $p.Type `</ansi> owned by <ansi fg="username">` .Character.Name `</ansi>.`) (sub $w (len $ind)) $ind }}
+{{ $ind }}{{ wraptext (print $p.DisplayName ` hunger is: <ansi fg="hunger-` $p.Food `">` $p.Food `</ansi>`) (sub $w (len $ind)) $ind }}
+{{ repeat "─" $w }}
+{{ template "carrying" (map "w" $w "ind" " " "label" "Carrying:" "items" $p.Items) }}
\ No newline at end of file
diff --git a/_datafiles/world/dogmud/templates/descriptions/biome.template b/_datafiles/world/dogmud/templates/descriptions/biome.template
index 4c2603ed1..ec533b680 100644
--- a/_datafiles/world/dogmud/templates/descriptions/biome.template
+++ b/_datafiles/world/dogmud/templates/descriptions/biome.template
@@ -1,6 +1,9 @@
-┌─ <ansi fg="black-bold">.:</ansi> Biome Info ─────────────────────────────────────────────────────────┐
+{{- $light := "" -}}
+{{- if le .SkyLightFraction 0.0 }}{{ $light = "No sky reaches this place at all." }}{{ else if ge .SkyLightFraction 0.8 }}{{ $light = "The open sky reaches this place freely." }}{{ else }}{{ $light = "Only a little sky reaches here, filtered and dim." }}{{ end -}}
+{{- if .HasLamp }}{{ if .StreetLamp }}{{ $light = print $light " Street lamps keep it lit from dusk to dawn." }}{{ else }}{{ $light = print $light " A lamp keeps it lit after dark." }}{{ end }}{{ end -}}
+── <ansi fg="black-bold">.:</ansi> Biome Info {{ repeat "─" 63 }}
   <ansi fg="yellow">Name:</ansi>        {{ .Name }}
   <ansi fg="yellow">Symbol:</ansi>      {{ .SymbolString }}
-  <ansi fg="yellow">Lighting:</ansi>    {{ if le .SkyLightFraction 0.0 }}No sky reaches this place at all.{{ else if ge .SkyLightFraction 0.8 }}The open sky reaches this place freely.{{ else }}Only a little sky reaches here, filtered and dim.{{ end }}{{ if .HasLamp }}{{ if .StreetLamp }} Street lamps keep it lit from dusk to dawn.{{ else }} A lamp keeps it lit after dark.{{ end }}{{ end }}
+  <ansi fg="yellow">Lighting:</ansi>    {{ splitstring $light 59 "               " }}
   <ansi fg="yellow">Description:</ansi> {{ splitstring .Description 59 "               " }}
-└─────────────────────────────────────────────────────────────────────────┘
+{{ repeat "─" 80 }}
diff --git a/_datafiles/world/dogmud/templates/descriptions/biome.w30.template b/_datafiles/world/dogmud/templates/descriptions/biome.w30.template
new file mode 100644
index 000000000..1f1430761
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/descriptions/biome.w30.template
@@ -0,0 +1,13 @@
+{{- /* The biome at 30 columns: between two full-width rules, label: value
+lines 2 in with one space after each label; a value that does not fit
+continues 2 past its label. */ -}}
+{{- $w := 30 -}}
+{{- $light := "" -}}
+{{- if le .SkyLightFraction 0.0 }}{{ $light = "No sky reaches this place at all." }}{{ else if ge .SkyLightFraction 0.8 }}{{ $light = "The open sky reaches this place freely." }}{{ else }}{{ $light = "Only a little sky reaches here, filtered and dim." }}{{ end -}}
+{{- if .HasLamp }}{{ if .StreetLamp }}{{ $light = print $light " Street lamps keep it lit from dusk to dawn." }}{{ else }}{{ $light = print $light " A lamp keeps it lit after dark." }}{{ end }}{{ end -}}
+── <ansi fg="black-bold">.:</ansi> Biome Info {{ repeat "─" (sub $w 17) }}
+  {{ wraptext (print `<ansi fg="yellow">Name:</ansi> ` .Name) (sub $w 4) "    " }}
+  <ansi fg="yellow">Symbol:</ansi> {{ .SymbolString }}
+  {{ wraptext (print `<ansi fg="yellow">Lighting:</ansi> ` $light) (sub $w 4) "    " }}
+  {{ wraptext (print `<ansi fg="yellow">Description:</ansi> ` .Description) (sub $w 4) "    " }}
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/descriptions/biome.w42.template b/_datafiles/world/dogmud/templates/descriptions/biome.w42.template
new file mode 100644
index 000000000..38688869b
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/descriptions/biome.w42.template
@@ -0,0 +1,14 @@
+{{- /* The biome at 42 columns: between two full-width rules, label: value
+lines 2 in. The lighting and the description are prose after a label, so
+they continue 2 past the label (owner, 2026-11-10). Drawn to 42 whatever
+the reader's width. */ -}}
+{{- $w := 42 -}}
+{{- $light := "" -}}
+{{- if le .SkyLightFraction 0.0 }}{{ $light = "No sky reaches this place at all." }}{{ else if ge .SkyLightFraction 0.8 }}{{ $light = "The open sky reaches this place freely." }}{{ else }}{{ $light = "Only a little sky reaches here, filtered and dim." }}{{ end -}}
+{{- if .HasLamp }}{{ if .StreetLamp }}{{ $light = print $light " Street lamps keep it lit from dusk to dawn." }}{{ else }}{{ $light = print $light " A lamp keeps it lit after dark." }}{{ end }}{{ end -}}
+── <ansi fg="black-bold">.:</ansi> Biome Info {{ repeat "─" (sub $w 17) }}
+  <ansi fg="yellow">Name:</ansi>        {{ wraptext .Name (sub $w 15) (repeat " " 15) }}
+  <ansi fg="yellow">Symbol:</ansi>      {{ .SymbolString }}
+  {{ wraptext (print `<ansi fg="yellow">Lighting:</ansi>    ` $light) (sub $w 4) "    " }}
+  {{ wraptext (print `<ansi fg="yellow">Description:</ansi> ` .Description) (sub $w 4) "    " }}
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/descriptions/exits.template b/_datafiles/world/dogmud/templates/descriptions/exits.template
index 3003308a4..39b2e71c3 100644
--- a/_datafiles/world/dogmud/templates/descriptions/exits.template
+++ b/_datafiles/world/dogmud/templates/descriptions/exits.template
@@ -1,15 +1,33 @@
-<ansi fg="yellow">Exits: </ansi>
+{{- /* The exits line. A list wider than the screen wraps between exits,
+hanging under the first one. An exit and its tag ("northeast (locked)",
+"(up)") are one unit, split only when longer than a whole line. $list is the
+line as it always was; $wrapped is the same units laid out to the screen. */ -}}
+{{- $w := screenwidth -}}{{- $pad := "       " -}}
 {{- $exitCount := add (len .VisibleExits) (len .TemporaryExits) -}}
+{{- $list := "" }}{{ $wrapped := "" }}{{ $cur := 7 -}}
 {{- if eq $exitCount 0 -}}
-    <ansi fg="room-description{{ if or .IsNight .IsDark }}-dark{{ end }}">None</ansi>
+    {{- $dark := "" }}{{ if or .IsNight .IsDark }}{{ $dark = "-dark" }}{{ end -}}
+    {{- $list = print `<ansi fg="room-description` $dark `">None</ansi>` -}}
 {{- else -}}
     {{- $displayed := 0 -}}
     {{- range $exitStr, $exitInfo := .VisibleExits -}}
             {{- $displayed = add $displayed 1 -}}
-            <ansi fg="{{ if $exitInfo.Secret }}secret-{{ end }}exit">{{ if $exitInfo.Secret }}({{ end }}{{ $exitStr }}{{ if $exitInfo.Secret }}){{ end }}</ansi>{{ if $exitInfo.HasLock }}{{ if not $exitInfo.Lock.IsLocked }} (unlocked){{ else }} (locked){{ end }}{{ end }}{{- if ne $displayed $exitCount }}, {{ end -}}
+            {{- $u := "" }}{{ if $exitInfo.Secret }}{{ $u = print `<ansi fg="secret-exit">(` $exitStr `)</ansi>` }}{{ else }}{{ $u = print `<ansi fg="exit">` $exitStr `</ansi>` }}{{ end -}}
+            {{- if $exitInfo.HasLock }}{{ if not $exitInfo.Lock.IsLocked }}{{ $u = print $u ` (unlocked)` }}{{ else }}{{ $u = print $u ` (locked)` }}{{ end }}{{ end -}}
+            {{- if ne $displayed $exitCount }}{{ $u = print $u `,` }}{{ end -}}
+            {{- $n := visiblewidth $u }}{{ $sep := 0 }}{{ if gt $displayed 1 }}{{ $sep = 1 }}{{ $list = print $list " " }}{{ end }}{{ $list = print $list $u -}}
+            {{- if le (add $cur $sep $n) $w }}{{ if $sep }}{{ $wrapped = print $wrapped " " }}{{ end }}{{ $wrapped = print $wrapped $u }}{{ $cur = add $cur $sep $n -}}
+            {{- else if le (add 7 $n) $w }}{{ $wrapped = print $wrapped "\n" $pad $u }}{{ $cur = add 7 $n -}}
+            {{- else }}{{ if $sep }}{{ $wrapped = print $wrapped "\n" $pad }}{{ end }}{{ $wrapped = print $wrapped (wraptext $u (sub $w 7) $pad) }}{{ $cur = $w }}{{ end -}}
     {{- end -}}
     {{- range $exitStr, $tmpExitInfo := .TemporaryExits -}}
             {{- $displayed = add $displayed 1 -}}
-            <ansi fg="exit">{{ $tmpExitInfo.Title }}</ansi>{{- if ne $displayed $exitCount }}, {{ end -}}
+            {{- $u := print `<ansi fg="exit">` $tmpExitInfo.Title `</ansi>` -}}
+            {{- if ne $displayed $exitCount }}{{ $u = print $u `,` }}{{ end -}}
+            {{- $n := visiblewidth $u }}{{ $sep := 0 }}{{ if gt $displayed 1 }}{{ $sep = 1 }}{{ $list = print $list " " }}{{ end }}{{ $list = print $list $u -}}
+            {{- if le (add $cur $sep $n) $w }}{{ if $sep }}{{ $wrapped = print $wrapped " " }}{{ end }}{{ $wrapped = print $wrapped $u }}{{ $cur = add $cur $sep $n -}}
+            {{- else if le (add 7 $n) $w }}{{ $wrapped = print $wrapped "\n" $pad $u }}{{ $cur = add 7 $n -}}
+            {{- else }}{{ if $sep }}{{ $wrapped = print $wrapped "\n" $pad }}{{ end }}{{ $wrapped = print $wrapped (wraptext $u (sub $w 7) $pad) }}{{ $cur = $w }}{{ end -}}
     {{- end -}}
-{{- end }}
+{{- end -}}
+<ansi fg="yellow">Exits: </ansi>{{ if le (add 7 (visiblewidth $list)) $w }}{{ $list }}{{ else }}{{ $wrapped }}{{ end }}
diff --git a/_datafiles/world/dogmud/templates/descriptions/fixtures.template b/_datafiles/world/dogmud/templates/descriptions/fixtures.template
index 1cd56a0e9..79357efd3 100644
--- a/_datafiles/world/dogmud/templates/descriptions/fixtures.template
+++ b/_datafiles/world/dogmud/templates/descriptions/fixtures.template
@@ -1,2 +1,4 @@
-{{- range $index, $fixture := .Fixtures }}<ansi fg="room-description{{ if or $.IsNight $.IsDark }}-dark{{ end }}">The <ansi fg="item">{{ $fixture.Name }}</ansi> {{ if $fixture.Darkness }}{{ if $fixture.Lit }}swallows the light{{ else }}is still{{ end }}{{ else }}is {{ if $fixture.Lit }}lit{{ else }}unlit{{ end }}{{ end }}.</ansi>
+{{- range $index, $fixture := .Fixtures }}{{ $dark := "" }}{{ if or $.IsNight $.IsDark }}{{ $dark = "-dark" }}{{ end -}}
+{{- $state := "" }}{{ if $fixture.Darkness }}{{ if $fixture.Lit }}{{ $state = "swallows the light" }}{{ else }}{{ $state = "is still" }}{{ end }}{{ else }}{{ if $fixture.Lit }}{{ $state = "is lit" }}{{ else }}{{ $state = "is unlit" }}{{ end }}{{ end -}}
+{{ wraptext (print `<ansi fg="room-description` $dark `">The <ansi fg="item">` $fixture.Name `</ansi> ` $state `.</ansi>`) screenwidth }}
 {{ end -}}
\ No newline at end of file
diff --git a/_datafiles/world/dogmud/templates/descriptions/identify.w30.template b/_datafiles/world/dogmud/templates/descriptions/identify.w30.template
new file mode 100644
index 000000000..8cf984aca
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/descriptions/identify.w30.template
@@ -0,0 +1,46 @@
+{{- /* Identify and appraise at 30 columns: four sections, each a titled
+rule, its lines 1 in, and a closing rule. Each label has one space after
+it; a value that does not fit beside it goes on the next line 2 past the
+label, and prose after a label continues 2 past it. */ -}}
+{{- define "slot" }}{{ if le (add (len .ind) .pad (visiblewidth .v)) .w }}{{ .ind }}<ansi fg="yellow">{{ padRight .pad .label }}</ansi>{{ .v }}{{ else }}{{ .ind }}<ansi fg="yellow">{{ .label }}</ansi>
+{{ .ind }}  {{ wraptext .v (sub .w (len .ind) 2) (print .ind "  ") }}{{ end }}{{ end -}}
+{{- define "prose" }}{{ .ind }}{{ wraptext (print `<ansi fg="yellow">` (padRight .pad .label) `</ansi>` .v) (sub .w (len .ind) 2) (print .ind "  ") }}{{ end -}}
+{{- $item := .Item }}{{ $spec := .ItemSpec }}{{ $w := 30 }}{{ $ind := " " -}}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Basic Info</ansi> {{ repeat "─" (sub $w 16) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 6 "label" "Name:" "v" (uc $item.Name)) }}
+{{ template "prose" (map "w" $w "ind" $ind "pad" 13 "label" "Description:" "v" $spec.Description) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 6 "label" "Type:" "v" (print (uc $spec.Type.String) " (" (uc $spec.Subtype.String) ")")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 7 "label" "Value:" "v" (printf "%d gold" $spec.Value)) }}
+{{ repeat "─" $w }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Combat Properties</ansi> {{ repeat "─" (sub $w 23) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 16 "label" "Striking Power:" "v" (or (and (eq $spec.Type.String "weapon") (damageQuality $spec.DamageMultiplier)) "N/A")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 18 "label" "Arcane Resonance:" "v" (or (and (ne $spec.SpellDamageMultiplier 0.0) (spellDamageQuality $spec.SpellDamageMultiplier)) "N/A")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 10 "label" "Physical:" "v" (or (and (ne $spec.PhysicalMitigation 0) (mitigationQuality (divFloat $spec.PhysicalMitigation 100))) "N/A")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 9 "label" "Magical:" "v" (or (and (ne $spec.MagicalMitigation 0) (mitigationQuality (divFloat $spec.MagicalMitigation 100))) "N/A")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 12 "label" "Conviction:" "v" (or (and (ne $spec.ConvictionMitigation 0) (mitigationQuality (divFloat $spec.ConvictionMitigation 100))) "N/A")) }}
+{{ repeat "─" $w }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Modifiers</ansi> {{ repeat "─" (sub $w 15) }}
+{{- $hasMods := false }}
+{{- range $statName, $qty := $spec.StatMods }}{{ $hasMods = true }}
+{{ $ind }}{{ wraptext (statModDescription $statName $qty) (sub $w (len $ind) 2) (print $ind "  ") }}
+{{- end }}
+{{- range $idx, $conditionId := $spec.ConditionIds }}{{ $hasMods = true }}
+{{ template "prose" (map "w" $w "ind" $ind "pad" 9 "label" "Applies:" "v" (print `<ansi fg="spellname">` (conditionname $conditionId) `</ansi> - ` (conditionduration $conditionId))) }}
+{{- end }}
+{{- if not $hasMods }}
+{{ $ind }}None detected.
+{{- end }}
+{{ repeat "─" $w }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Magical Effects</ansi> {{ repeat "─" (sub $w 21) }}
+{{- $hasMagic := false }}
+{{- if $item.IsCursed }}{{ $hasMagic = true }}
+{{ $ind }}It's <ansi fg="red-bold">CURSED!</ansi>{{ end }}
+{{- if gt (len $spec.Element.String) 0 }}{{ $hasMagic = true }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 9 "label" "Element:" "v" (uc $spec.Element.String)) }}{{ end }}
+{{- range $idx, $conditionId := $spec.Damage.CritConditionIds }}{{ $hasMagic = true }}
+{{ template "prose" (map "w" $w "ind" $ind "pad" 13 "label" "Crits Apply:" "v" (print `<ansi fg="spellname">` (conditionname $conditionId) `</ansi> - ` (conditionduration $conditionId))) }}
+{{- end }}
+{{- if not $hasMagic }}
+{{ $ind }}None detected.
+{{- end }}
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/descriptions/identify.w42.template b/_datafiles/world/dogmud/templates/descriptions/identify.w42.template
new file mode 100644
index 000000000..813a31cbc
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/descriptions/identify.w42.template
@@ -0,0 +1,47 @@
+{{- /* Identify and appraise at 42 columns: four sections, each a titled
+rule, its lines 3 in, and a closing rule. A value that does not fit beside
+its padded label goes on the next line 2 past the label. The description and
+the condition lines are prose after a label, so they continue 2 past the
+label (owner, 2026-11-10). Drawn to 42 whatever the reader's width. */ -}}
+{{- define "slot" }}{{ if le (add (len .ind) .pad (visiblewidth .v)) .w }}{{ .ind }}<ansi fg="yellow">{{ padRight .pad .label }}</ansi>{{ .v }}{{ else }}{{ .ind }}<ansi fg="yellow">{{ .label }}</ansi>
+{{ .ind }}  {{ wraptext .v (sub .w (len .ind) 2) (print .ind "  ") }}{{ end }}{{ end -}}
+{{- define "prose" }}{{ .ind }}{{ wraptext (print `<ansi fg="yellow">` (padRight .pad .label) `</ansi>` .v) (sub .w (len .ind) 2) (print .ind "  ") }}{{ end -}}
+{{- $item := .Item }}{{ $spec := .ItemSpec }}{{ $w := 42 }}{{ $ind := "   " -}}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Basic Info</ansi> {{ repeat "─" (sub $w 16) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 13 "label" "Name:" "v" (uc $item.Name)) }}
+{{ template "prose" (map "w" $w "ind" $ind "pad" 13 "label" "Description:" "v" $spec.Description) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 13 "label" "Type:" "v" (print (uc $spec.Type.String) " (" (uc $spec.Subtype.String) ")")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 13 "label" "Value:" "v" (printf "%d gold" $spec.Value)) }}
+{{ repeat "─" $w }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Combat Properties</ansi> {{ repeat "─" (sub $w 23) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 17 "label" "Striking Power:" "v" (or (and (eq $spec.Type.String "weapon") (damageQuality $spec.DamageMultiplier)) "N/A")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 18 "label" "Arcane Resonance:" "v" (or (and (ne $spec.SpellDamageMultiplier 0.0) (spellDamageQuality $spec.SpellDamageMultiplier)) "N/A")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 13 "label" "Physical:" "v" (or (and (ne $spec.PhysicalMitigation 0) (mitigationQuality (divFloat $spec.PhysicalMitigation 100))) "N/A")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 13 "label" "Magical:" "v" (or (and (ne $spec.MagicalMitigation 0) (mitigationQuality (divFloat $spec.MagicalMitigation 100))) "N/A")) }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 13 "label" "Conviction:" "v" (or (and (ne $spec.ConvictionMitigation 0) (mitigationQuality (divFloat $spec.ConvictionMitigation 100))) "N/A")) }}
+{{ repeat "─" $w }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Modifiers</ansi> {{ repeat "─" (sub $w 15) }}
+{{- $hasMods := false }}
+{{- range $statName, $qty := $spec.StatMods }}{{ $hasMods = true }}
+{{ $ind }}{{ wraptext (statModDescription $statName $qty) (sub $w (len $ind) 2) (print $ind "  ") }}
+{{- end }}
+{{- range $idx, $conditionId := $spec.ConditionIds }}{{ $hasMods = true }}
+{{ template "prose" (map "w" $w "ind" $ind "pad" 13 "label" "Applies:" "v" (print `<ansi fg="spellname">` (conditionname $conditionId) `</ansi> - ` (conditionduration $conditionId))) }}
+{{- end }}
+{{- if not $hasMods }}
+{{ $ind }}None detected.
+{{- end }}
+{{ repeat "─" $w }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Magical Effects</ansi> {{ repeat "─" (sub $w 21) }}
+{{- $hasMagic := false }}
+{{- if $item.IsCursed }}{{ $hasMagic = true }}
+{{ $ind }}It's <ansi fg="red-bold">CURSED!</ansi>{{ end }}
+{{- if gt (len $spec.Element.String) 0 }}{{ $hasMagic = true }}
+{{ template "slot" (map "w" $w "ind" $ind "pad" 13 "label" "Element:" "v" (uc $spec.Element.String)) }}{{ end }}
+{{- range $idx, $conditionId := $spec.Damage.CritConditionIds }}{{ $hasMagic = true }}
+{{ template "prose" (map "w" $w "ind" $ind "pad" 13 "label" "Crits Apply:" "v" (print `<ansi fg="spellname">` (conditionname $conditionId) `</ansi> - ` (conditionduration $conditionId))) }}
+{{- end }}
+{{- if not $hasMagic }}
+{{ $ind }}None detected.
+{{- end }}
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/descriptions/insidecontainer.w30.template b/_datafiles/world/dogmud/templates/descriptions/insidecontainer.w30.template
new file mode 100644
index 000000000..d4e6c2fae
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/descriptions/insidecontainer.w30.template
@@ -0,0 +1,26 @@
+{{- /* Inside a container or a corpse, at 30 columns: the list between two
+full-width rules, wrapped as at 42. */ -}}
+{{- define "list" -}}
+{{- $w := .w -}}{{- $col := add (len .ind) (len .label) -}}{{- $hang := add $col 1 -}}
+{{- if lt (sub $w $hang) 20 -}}{{- $hang = add (len .ind) 2 -}}{{- end -}}
+{{- $short := add (len .ind) 2 -}}{{- range .names -}}{{- if gt (add (visiblewidth .) 1) (sub $w $hang) -}}{{- $hang = $short -}}{{- end -}}{{- end -}}
+{{- $pad := repeat " " $hang -}}{{- $cur := $col -}}{{- $last := sub (len .names) 1 -}}
+{{ .ind }}<ansi fg="white">{{ .label }}</ansi>{{ if not .names }} {{ .empty }}{{ end }}
+{{- range $i, $f := .names -}}
+{{- if lt $i $last -}}{{- $f = print $f "," -}}{{- end -}}
+{{- $n := visiblewidth $f -}}
+{{- if le (add $cur 1 $n) $w -}}
+{{ " " }}{{ $f }}{{- $cur = add $cur 1 $n -}}
+{{- else if le (add $hang $n) $w -}}
+{{ "\n" }}{{ $pad }}{{ $f }}{{- $cur = add $hang $n -}}
+{{- else -}}
+{{ "\n" }}{{ $pad }}{{ wraptext $f (sub $w $hang) $pad }}{{- $cur = $w -}}
+{{- end -}}
+{{- end }}
+{{- end -}}
+{{- $w := 30 -}}
+<ansi fg="white">{{ repeat "─" $w }}</ansi>
+
+{{ template "list" (map "w" $w "ind" "  " "label" "Inside:" "names" .ItemNamesFormatted "empty" "Nothing") }}
+
+<ansi fg="white">{{ repeat "─" $w }}</ansi>
\ No newline at end of file
diff --git a/_datafiles/world/dogmud/templates/descriptions/insidecontainer.w42.template b/_datafiles/world/dogmud/templates/descriptions/insidecontainer.w42.template
new file mode 100644
index 000000000..8c67e49a2
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/descriptions/insidecontainer.w42.template
@@ -0,0 +1,28 @@
+{{- /* Inside a container or a corpse, at 42 columns: the list between two
+full-width rules. It wraps between names, hanging under the first name when
+every name fits there, else 2 past the label; a name longer than a whole
+line is wrapped on its own. Drawn to 42 whatever the reader's width. */ -}}
+{{- define "list" -}}
+{{- $w := .w -}}{{- $col := add (len .ind) (len .label) -}}{{- $hang := add $col 1 -}}
+{{- if lt (sub $w $hang) 20 -}}{{- $hang = add (len .ind) 2 -}}{{- end -}}
+{{- $short := add (len .ind) 2 -}}{{- range .names -}}{{- if gt (add (visiblewidth .) 1) (sub $w $hang) -}}{{- $hang = $short -}}{{- end -}}{{- end -}}
+{{- $pad := repeat " " $hang -}}{{- $cur := $col -}}{{- $last := sub (len .names) 1 -}}
+{{ .ind }}<ansi fg="white">{{ .label }}</ansi>{{ if not .names }} {{ .empty }}{{ end }}
+{{- range $i, $f := .names -}}
+{{- if lt $i $last -}}{{- $f = print $f "," -}}{{- end -}}
+{{- $n := visiblewidth $f -}}
+{{- if le (add $cur 1 $n) $w -}}
+{{ " " }}{{ $f }}{{- $cur = add $cur 1 $n -}}
+{{- else if le (add $hang $n) $w -}}
+{{ "\n" }}{{ $pad }}{{ $f }}{{- $cur = add $hang $n -}}
+{{- else -}}
+{{ "\n" }}{{ $pad }}{{ wraptext $f (sub $w $hang) $pad }}{{- $cur = $w -}}
+{{- end -}}
+{{- end }}
+{{- end -}}
+{{- $w := 42 -}}
+<ansi fg="white">{{ repeat "─" $w }}</ansi>
+
+{{ template "list" (map "w" $w "ind" "  " "label" "Inside:" "names" .ItemNamesFormatted "empty" "Nothing") }}
+
+<ansi fg="white">{{ repeat "─" $w }}</ansi>
\ No newline at end of file
diff --git a/_datafiles/world/dogmud/templates/descriptions/room-title.template b/_datafiles/world/dogmud/templates/descriptions/room-title.template
index 2b6d3ba79..71fd29299 100644
--- a/_datafiles/world/dogmud/templates/descriptions/room-title.template
+++ b/_datafiles/world/dogmud/templates/descriptions/room-title.template
@@ -2,4 +2,10 @@
 {{ if ne .RoomSymbol "" -}}
    {{ $mapSymbol = printf `<ansi fg="black-bold">[</ansi><ansi fg="map-%s" bg="mapbg-%s">%s</ansi><ansi fg="black-bold">]</ansi> ` (mapslug .RoomLegend) (mapslug .RoomLegend) .RoomSymbol }}
 {{- end }}
-<ansi fg="black-bold">.:</ansi> {{ $mapSymbol }}<ansi fg="room-title">{{ .Title }}</ansi>{{ if ne .Zone ""}} <ansi fg="room-zone">[{{ .Zone }}]</ansi>{{ end }}{{ if .ShowPvp }} <ansi fg="11" bg="52"> ☠ PK Area ☠ </ansi>{{ end }}
\ No newline at end of file
+{{- /* A title wider than the screen wraps 3 in, under the title. The PvP
+tag is one unit: when the line does not fit, it goes on a line of its own. */ -}}
+{{- $rest := print $mapSymbol `<ansi fg="room-title">` .Title `</ansi>` -}}
+{{- if ne .Zone "" }}{{ $rest = print $rest ` <ansi fg="room-zone">[` .Zone `]</ansi>` }}{{ end -}}
+{{- $pk := "" }}{{ $full := $rest }}{{ if .ShowPvp }}{{ $pk = `<ansi fg="11" bg="52"> ☠ PK Area ☠ </ansi>` }}{{ $full = print $rest " " $pk }}{{ end }}
+<ansi fg="black-bold">.:</ansi> {{ if le (add 3 (visiblewidth $full)) screenwidth }}{{ $full }}{{ else }}{{ wraptext $rest (sub screenwidth 3) "   " }}{{ if $pk }}
+   {{ $pk }}{{ end }}{{ end }}
\ No newline at end of file
diff --git a/_datafiles/world/dogmud/templates/descriptions/room.template b/_datafiles/world/dogmud/templates/descriptions/room.template
index 924ad85a8..9151fb888 100644
--- a/_datafiles/world/dogmud/templates/descriptions/room.template
+++ b/_datafiles/world/dogmud/templates/descriptions/room.template
@@ -1,10 +1,10 @@
 <ansi fg="room-description{{ if or .IsNight .IsDark }}-dark{{ end }}">{{ .Description }}</ansi>
 {{- range $index, $alertStr := .RoomAlerts }}
 
-    <ansi fg="red">┌───────────────────────────────────────────────────────────────────┐</ansi>
-      {{ $alertStr }}
-    <ansi fg="red">└───────────────────────────────────────────────────────────────────┘</ansi>
+<ansi fg="red">{{ repeat "─" screenwidth }}</ansi>
+{{ centerlines $alertStr screenwidth }}
+<ansi fg="red">{{ repeat "─" screenwidth }}</ansi>
 {{- end }}
 {{- if ne (len .TrackingString) 0 }}
-<ansi fg="182">{{ .TrackingString }}</ansi>
+<ansi fg="182">{{ wrapspans .TrackingString screenwidth }}</ansi>
 {{ end }}
diff --git a/_datafiles/world/dogmud/templates/descriptions/rune.template b/_datafiles/world/dogmud/templates/descriptions/rune.template
index b407c8cc3..75417aecf 100644
--- a/_datafiles/world/dogmud/templates/descriptions/rune.template
+++ b/_datafiles/world/dogmud/templates/descriptions/rune.template
@@ -1 +1,4 @@
-           A rune reads: [<ansi fg="magenta-bold">{{ .DisplayText }}</ansi>]
\ No newline at end of file
+{{- /* A rune wider than the screen starts at the left edge, its text
+continuing 2 in; a word longer than a whole line is broken. */ -}}
+{{- $text := print `A rune reads: [<ansi fg="magenta-bold">` .DisplayText `</ansi>]` -}}
+{{- if le (add 11 (visiblewidth $text)) screenwidth }}           {{ $text }}{{ else }}{{ wrapbreak $text (sub screenwidth 2) "  " }}{{ end }}
\ No newline at end of file
diff --git a/_datafiles/world/dogmud/templates/descriptions/sign.template b/_datafiles/world/dogmud/templates/descriptions/sign.template
index 0f6aded49..533a990c5 100644
--- a/_datafiles/world/dogmud/templates/descriptions/sign.template
+++ b/_datafiles/world/dogmud/templates/descriptions/sign.template
@@ -1 +1,4 @@
-           A sign reads: [<ansi fg="green-bold">{{ .DisplayText }}</ansi>]
\ No newline at end of file
+{{- /* A sign wider than the screen starts at the left edge, its text
+continuing 2 in; a word longer than a whole line is broken. */ -}}
+{{- $text := print `A sign reads: [<ansi fg="green-bold">` .DisplayText `</ansi>]` -}}
+{{- if le (add 11 (visiblewidth $text)) screenwidth }}           {{ $text }}{{ else }}{{ wrapbreak $text (sub screenwidth 2) "  " }}{{ end }}
\ No newline at end of file
diff --git a/_datafiles/world/dogmud/templates/descriptions/track.w30.template b/_datafiles/world/dogmud/templates/descriptions/track.w30.template
new file mode 100644
index 000000000..e0a7e45ac
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/descriptions/track.w30.template
@@ -0,0 +1,32 @@
+{{- /* Track at 30 columns: the trails between two full-width rules, one to
+a line 1 in, laid out as at 42. */ -}}
+{{- define "trail" -}}
+{{- $w := .w }}{{ $ind := .ind }}{{ $in2 := print .ind "  " -}}
+{{- $str := print "[<ansi fg=\"trail-" (padRight 4 (lowercase .t.Strength)) "\">" .t.Strength "</ansi>]" -}}
+{{- $name := print "<ansi fg=\"username\">" (stringor .t.Name "None" 0) "</ansi>" -}}
+{{- $went := "" }}{{ if ne .t.ExitName "" }}{{ $went = print "- It seems like they went <ansi fg=\"exit\">" .t.ExitName "</ansi>" }}{{ end -}}
+{{- $head := print $str " " $name -}}
+{{- if and $went (le (add (len $ind) (visiblewidth $head) 1 (visiblewidth $went)) $w) -}}
+{{ $ind }}{{ $head }} {{ $went }}
+{{- else -}}
+{{- if le (add (len $ind) (visiblewidth $head)) $w -}}
+{{ $ind }}{{ $head }}
+{{- else -}}
+{{ $ind }}{{ $str }}
+{{ $in2 }}{{ wraptext $name (sub $w (len $in2)) $in2 }}
+{{- end -}}
+{{- if $went }}
+{{ $in2 }}{{ wraptext $went (sub $w (len $in2)) $in2 }}
+{{- end -}}
+{{- end -}}
+{{- end -}}
+{{- $w := 30 }}{{ $ind := " " -}}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Recent Visitors</ansi> {{ repeat "─" (sub $w 21) }}
+{{- if eq (len .) 0 }}
+{{ $ind }}None
+{{- else }}
+{{- range $key, $val := . }}
+{{ template "trail" (map "w" $w "ind" $ind "t" $val) }}
+{{- end }}
+{{- end }}
+{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/descriptions/track.w42.template b/_datafiles/world/dogmud/templates/descriptions/track.w42.template
new file mode 100644
index 000000000..ff95e52c1
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/descriptions/track.w42.template
@@ -0,0 +1,35 @@
+{{- /* Track at 42 columns: the trails between two full-width rules, one to
+a line 3 in. A trail that does not fit puts its name on the next line 2 in
+when the name does not fit beside the strength, and where it went on the
+next line 2 in. A name is never split unless longer than a line. Drawn to
+42 whatever the reader's width. */ -}}
+{{- define "trail" -}}
+{{- $w := .w }}{{ $ind := .ind }}{{ $in2 := print .ind "  " -}}
+{{- $str := print "[<ansi fg=\"trail-" (padRight 4 (lowercase .t.Strength)) "\">" .t.Strength "</ansi>]" -}}
+{{- $name := print "<ansi fg=\"username\">" (stringor .t.Name "None" 0) "</ansi>" -}}
+{{- $went := "" }}{{ if ne .t.ExitName "" }}{{ $went = print "- It seems like they went <ansi fg=\"exit\">" .t.ExitName "</ansi>" }}{{ end -}}
+{{- $head := print $str " " $name -}}
+{{- if and $went (le (add (len $ind) (visiblewidth $head) 1 (visiblewidth $went)) $w) -}}
+{{ $ind }}{{ $head }} {{ $went }}
+{{- else -}}
+{{- if le (add (len $ind) (visiblewidth $head)) $w -}}
+{{ $ind }}{{ $head }}
+{{- else -}}
+{{ $ind }}{{ $str }}
+{{ $in2 }}{{ wraptext $name (sub $w (len $in2)) $in2 }}
+{{- end -}}
+{{- if $went }}
+{{ $in2 }}{{ wraptext $went (sub $w (len $in2)) $in2 }}
+{{- end -}}
+{{- end -}}
+{{- end -}}
+{{- $w := 42 }}{{ $ind := "   " -}}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Recent Visitors</ansi> {{ repeat "─" (sub $w 21) }}
+{{- if eq (len .) 0 }}
+{{ $ind }}None
+{{- else }}
+{{- range $key, $val := . }}
+{{ template "trail" (map "w" $w "ind" $ind "t" $val) }}
+{{- end }}
+{{- end }}
+{{ repeat "─" $w }}
diff --git a/docs/README.md b/docs/README.md
index 2510ac31b..51b9065b5 100644
--- a/docs/README.md
+++ b/docs/README.md
@@ -250,10 +250,15 @@ than reading an old result:
 | [`../internal/guards/command_parity_guard_test.go`](../internal/guards/command_parity_guard_test.go) | Owner ruling 2026-11-07: the boot's command parity audit, run on the real user and mob registries with modules loaded, finds only `throw` (the standing reminder that a mob throw is owed); every other one-sided command is allowlisted with its reason in `internal/actions/divergences.go`. Includes a fixture proving the audit reports an unlisted gap |
 | [`superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md`](superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md) | Plan 3a of the web client base: narrow template forms picked by width, the `screenwidth` template function, table forms, splash to words only on narrow screens, the MOTD's narrow form, and the template width guard |
 | [`superpowers/plans/2026-11-10-webclient-base-3b1-character-sheets.md`](superpowers/plans/2026-11-10-webclient-base-3b1-character-sheets.md) | Plan 3b-1 of the web client base: 42- and 30-column forms of the character sheets (status, inventory, skills, conditions, cooldowns, quests, storage, title) and the companion screen, with their fit tests and 80-column goldens recorded from master |
+| [`superpowers/plans/2026-11-10-webclient-base-3b2-looking-at-things.md`](superpowers/plans/2026-11-10-webclient-base-3b2-looking-at-things.md) | Plan 3b-2 of the web client base: 42- and 30-column forms of the looking-at-things screens (look at a person, corpse, pet, equipment, container; identify and appraise; track; biome), the room view drawn to the reader's width (alerts between full-width rules at every width, the minimap above the prose at 42 and below), the biome box as 80-wide rules, and their fit tests and 80-column goldens |
 | [`../internal/templates/forms.go`](../internal/templates/forms.go) | Narrow forms: `FormWidths`, `WidestLine`, the reader's width and the widest-form-that-fits rule behind `templates.Process` |
-| [`../internal/guards/template_width_guard_test.go`](../internal/guards/template_width_guard_test.go) | Ruling 3 of the web client base: every template has one width class (fits at 30, 42 and 80, words-only splash, map or wide table, raw login text, wrapped, plan 3b, help, admin, or no caller), form files are named for a known width, and the fitting ones are rendered and measured |
+| [`../internal/guards/template_width_guard_test.go`](../internal/guards/template_width_guard_test.go) | Ruling 3 of the web client base: every template has one width class (fits at 30, 42 and 80, words-only splash, map or wide table, raw login text, wrapped, plan 3b-3, help, admin, or no caller), form files are named for a known width, and the fitting ones are rendered and measured |
 | [`../internal/guards/character_screens_test.go`](../internal/guards/character_screens_test.go) | Plan 3b: the widest-in-play fixtures of the status, inventory, skills, cooldowns and storage screens for the template width guard, and their 80-column goldens (`testdata/character_screens_80/`, recorded from master) proving a desktop sees what it saw |
 | [`../internal/usercommands/narrow_forms_test.go`](../internal/usercommands/narrow_forms_test.go) | Plan 3b: the conditions, quests, title and companion screens fit at 30, 42 and 80 columns with their widest data, and keep their 80-column output (`testdata/narrow_forms_80/`, recorded from master) |
+| [`../internal/guards/looking_screens_test.go`](../internal/guards/looking_screens_test.go) | Plan 3b-2: the widest-in-play and as-captured fixtures of the looking-at-things screens and the room view's templates (the two longest shipped mob names read from their YAML), their 80-column goldens (`testdata/looking_screens_80/`, most recorded from master, the owner-approved alert and biome changes recorded on the branch), a 32-letter name running past only on its own line at 30, the shipped wildfire alert read whole, and exits, the PvP tag and the tracking line's names kept whole |
+| [`../internal/usercommands/room_view_forms_test.go`](../internal/usercommands/room_view_forms_test.go) | Plan 3b-2: the room view through the real `look` for a telnet reader at 30, 42 and 80: it fits, every alert rule is the screen's width, the minimap goes above the prose at 42 and below, and the 80-column view is pinned (`testdata/narrow_forms_80/room_view.golden`) |
+| [`../internal/templates/centerlines_test.go`](../internal/templates/centerlines_test.go) | Plan 3b-2: `centerlines` centres an alert on one line, a piece to a line, or wrapped, padded on the left only and never past its margin |
+| [`../internal/rooms/roomdetails_modifier_wrap_test.go`](../internal/rooms/roomdetails_modifier_wrap_test.go) | A mutator's description modifier wraps with the description and beside the minimap (#455); since plan 3b-2, alerts reach the template as bare text and the minimap goes above the prose at 42 columns and below |
 | `../_datafiles/world/dogmud/templates/character/{status,inventory,skills,conditions,cooldowns,quests,storage,title}.w42.template` and `.w30.template` | Plan 3b: the narrow forms of the character screens, as drawn in `_archive/notes/reports/narrow-screens-3b-mock/forms/` with the owner's 2026-11-10 rulings |
 | [`../internal/guards/no_entrance_guard_test.go`](../internal/guards/no_entrance_guard_test.go) | Owner ruling 2026-11-07: loading the shipped world, no room is left that no exit leads into unless `rooms.noEntranceByDesign` lists how players reach it (tutorial room, instance template, ferries) |
 | [`../internal/rooms/move_missing_room_test.go`](../internal/rooms/move_missing_room_test.go) | Entering the world into a room that no longer exists lands at the start room; an ordinary move to a missing room still refuses |
diff --git a/internal/guards/context.md b/internal/guards/context.md
index 3b08e7ad6..52dfcdd97 100644
--- a/internal/guards/context.md
+++ b/internal/guards/context.md
@@ -33,8 +33,10 @@ package main.
   `coulee_npc_light_guard_test.go`, `topic_manifest_test.go`.
 - **Goldens.** Record a rendered or computed output and compare it byte for
   byte: the lighting goldens (`lighting_*_golden_test.go`),
-  `wrap_live_render_golden_test.go` and `m2_routing_guard_test.go`. Their
-  files live in `testdata/` here.
+  `wrap_live_render_golden_test.go`, `m2_routing_guard_test.go`, and the
+  80-column goldens of the narrow-form screens, `character_screens_test.go`
+  (`-update-character-screens`) and `looking_screens_test.go`
+  (`-update-looking-screens`). Their files live in `testdata/` here.
 - **Copy guards.** House style for player text: `prose_dash_guard_test.go`,
   `go_prose_dash_guard_test.go`, `copy_no_dash_test.go`,
   `prose_tag_spacing_guard_test.go`, `combat_idiom_guard_test.go`,
diff --git a/internal/guards/looking_screens_test.go b/internal/guards/looking_screens_test.go
new file mode 100644
index 000000000..f32bdc2bd
--- /dev/null
+++ b/internal/guards/looking_screens_test.go
@@ -0,0 +1,615 @@
+package guards
+
+import (
+	"flag"
+	"fmt"
+	"os"
+	"path/filepath"
+	"regexp"
+	"slices"
+	"strings"
+	"testing"
+
+	"github.com/GoMudEngine/GoMud/internal/actions"
+	"github.com/GoMudEngine/GoMud/internal/characters"
+	"github.com/GoMudEngine/GoMud/internal/colorpatterns"
+	"github.com/GoMudEngine/GoMud/internal/conditions"
+	"github.com/GoMudEngine/GoMud/internal/exit"
+	"github.com/GoMudEngine/GoMud/internal/gamelock"
+	"github.com/GoMudEngine/GoMud/internal/items"
+	"github.com/GoMudEngine/GoMud/internal/mutations"
+	"github.com/GoMudEngine/GoMud/internal/mutators"
+	"github.com/GoMudEngine/GoMud/internal/pets"
+	"github.com/GoMudEngine/GoMud/internal/rooms"
+	"github.com/GoMudEngine/GoMud/internal/statmods"
+	"github.com/GoMudEngine/GoMud/internal/templates"
+	"github.com/GoMudEngine/GoMud/internal/users"
+	"github.com/GoMudEngine/GoMud/internal/util"
+	"gopkg.in/yaml.v3"
+)
+
+// The "looking at things" screens with narrow forms (plan 3b-2): looking at
+// a person, a corpse, a pet and into a container, identify and appraise,
+// track, biome, and the room view's templates. Each fixture is as wide as the
+// screen gets in play, or is the screen as the 2026-11-10 capture showed it,
+// built from the data types its callers pass. The goldens prove a desktop
+// sees at 80 columns what master drew, apart from the changes the owner
+// ruled (the room alerts at every width, and an interior biome whose
+// lighting line ran past 80).
+
+var updateLookingScreens = flag.Bool("update-looking-screens", false,
+	"rewrite internal/guards/testdata/looking_screens_80/*.golden from the current templates")
+
+// seedLookingScreenRegistries gives the fixtures the conditions and
+// mutations they name, restored when the test ends.
+func seedLookingScreenRegistries(t *testing.T) {
+	t.Helper()
+	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
+		83:  {ConditionId: 83, Name: "Broken Limb", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 12},
+		940: {ConditionId: 940, Name: "Chrysalis Regeneration", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 50},
+		941: {ConditionId: 941, Name: "Death's Shadow", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
+	}))
+	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
+		"reflect-skin-frostbite": {MutationId: "reflect-skin-frostbite", Name: "Reflect Skin",
+			Visual: "Their skin has the faint blue sheen of old ice, and frost gathers in the creases of their knuckles."},
+		"tail": {MutationId: "tail", Name: "Tail", Visual: "A long, sinuous tail sways behind them."},
+	}))
+}
+
+// lookedAt is a character as another sees it: name, description, health.
+func lookedAt(name, desc string, health, healthMax int, muts map[string]int) *characters.Character {
+	c := users.NewUserRecord(88160, 0).Character
+	c.Name = name
+	c.Description = desc
+	c.Health = health
+	c.HealthMax.Value = healthMax
+	c.Mutations = muts
+	return c
+}
+
+// descriptionCaptured is `look Screenly` as captured.
+func descriptionCaptured() any {
+	return lookedAt("Screenly", "They seem thoroughly uninteresting.", 100, 100, nil)
+}
+
+// descriptionWide is a looked-at character with a 26-letter name (the
+// longest whose health line fits beside its indent at 30), a two-paragraph
+// description, mutation visuals, and the longest health words.
+func descriptionWide() any {
+	return lookedAt("Bartholomewsworthingtonian",
+		"A tall, broad-shouldered traveller in a salt-stained greatcoat, with a weathered face and a nose that has been broken more than once. Their boots are caked with the grey mud of the lower marsh.\n\nA brass compass hangs from a cord at their throat, its needle trembling.",
+		5, 100, map[string]int{"reflect-skin-frostbite": 2, "tail": 1})
+}
+
+// The two longest shipped mob names (29 letters each), read from their YAML.
+const (
+	rissMobFile  = "mobs/the_confluence/9443-riss_the_cartographer_scholar.yaml"
+	odellMobFile = "mobs/east_road_to_greenford/9492-mistress_odell_the_victualler.yaml"
+)
+
+// shippedMobLooked is the shipped mob in file (under the dogmud world) as
+// look shows it: its own name and description, at full health.
+func shippedMobLooked(file string) func() any {
+	return func() any {
+		raw, err := os.ReadFile(filepath.Join(repoRootPath, dogmudDataFilesRoot, file))
+		if err != nil {
+			panic(err)
+		}
+		var mob struct {
+			Character struct {
+				Name        string `yaml:"name"`
+				Description string `yaml:"description"`
+			} `yaml:"character"`
+		}
+		if err := yaml.Unmarshal(raw, &mob); err != nil || mob.Character.Name == "" {
+			panic(fmt.Sprintf("%s: no character name (%v)", file, err))
+		}
+		return lookedAt(mob.Character.Name, mob.Character.Description, 100, 100, nil)
+	}
+}
+
+// lookingLongestName is a name at the configured maximum (NameSizeMax, 32),
+// for players and pets alike.
+const lookingLongestName = "Bartholomewsworthingtonianfellow"
+
+// corpseCaptured is `look corpse` at the marsh rat, as captured.
+func corpseCaptured() any {
+	return lookedAt("Marsh Rat", "A small grey-brown marsh rat, wet-furred and quick, with bright black eyes and a pink scaly tail. It freezes at your approach and weighs whether you are a problem. ", 0, 10, nil)
+}
+
+// corpseWide is a corpse with a long name and a long description.
+func corpseWide() any {
+	return lookedAt("Drillmaster Vorn the Unyielding", "A broad, scarred veteran of the city watch, his grey hair cropped close and his arms roped with old muscle. He carries himself like a man who has never once lost an argument he cared about, and his eyes follow every recruit in the yard.", 0, 10, nil)
+}
+
+// inventoryLookFixture is the data look builds for character/inventory-look:
+// the looked-at character's equipment and carried item names.
+func inventoryLookFixture(wide, hide bool) func() any {
+	return func() any {
+		c := users.NewUserRecord(88161, 0).Character
+		names := []string{"Leather Backpack", "Strongbox Key", "Healing Draught", "Iron Ore", "Tanned Hide"}
+		if wide {
+			long := screenItem(99211, "Pit Fighter's Serrated Basalt-Iron Blade", items.Weapon, 3, true)
+			long.Spec.NameSimple = "serrated basalt-iron blade"
+			c.Equipment.Weapon = long
+			back := screenItem(99212, "Wayfarer's Bottomless Pack", items.Back, 0, false)
+			back.Spec.NameSimple = "wayfarer's bottomless pack"
+			c.Equipment.Back = back
+			names = append(names, "Spirit Fetish", "A Salt-Stiff Page")
+		} else {
+			sword := screenItem(99213, "Longsword", items.Weapon, 0, false)
+			sword.Spec.NameSimple = "longsword"
+			c.Equipment.Weapon = sword
+			pack := screenItem(99214, "Pack", items.Back, 0, false)
+			pack.Spec.NameSimple = "pack"
+			c.Equipment.Back = pack
+		}
+		return map[string]any{
+			`Character`:     c,
+			`Equipment`:     &c.Equipment,
+			`ItemNames`:     names,
+			`HideEquipment`: hide,
+		}
+	}
+}
+
+// petFixture is the owner look passes to character/pet, with a pet whose
+// name and load run long when wide.
+func petFixture(wide bool) func() any {
+	return petNamed(wide, "Screenly", "Whiskerfield")
+}
+
+// petNamed is petFixture with the owner's and the pet's names given.
+func petNamed(wide bool, owner, pet string) func() any {
+	return func() any {
+		u := users.NewUserRecord(88162, 0)
+		u.Character.Name = owner
+		u.Character.Pet = pets.Pet{Name: pet, Type: "cat"}
+		if wide {
+			u.Character.Name = "Bartholomewsworthingtonian"
+			u.Character.Pet = pets.Pet{Name: "Sir Whiskerfield Pounce", Type: "warhound", Food: 3,
+				Items: []items.Item{
+					screenItem(99221, "Pit Fighter's Serrated Basalt-Iron Blade", items.Weapon, 0, false),
+					screenItem(99222, "Reinforced Travel Pack", items.Back, 0, false),
+					screenItem(99223, "A Salt-Stiff Page of a Sailor's Telling", items.Neck, 0, false),
+				}}
+		}
+		return u
+	}
+}
+
+// petNamedAfterMob is a pet given the name of the shipped mob in file, so
+// the pet's name line runs as long as the longest shipped names.
+func petNamedAfterMob(file string) func() any {
+	return func() any {
+		name := shippedMobLooked(file)().(*characters.Character).Name
+		return petNamed(false, "Screenly", name)()
+	}
+}
+
+// biomeFixture is a biome as rooms.GetBiome returns it.
+func biomeFixture(name, symbol string, sky float64, lamp, street bool, desc string) func() any {
+	return func() any {
+		b := &rooms.BiomeInfo{Name: name, Symbol: symbol, Description: desc, StreetLamp: street}
+		b.SkyLight = &sky
+		if lamp {
+			l := 40
+			b.Lamp = &l
+		}
+		return b
+	}
+}
+
+var (
+	biomeOutdoorCaptured = biomeFixture("Deep Water", "≈", 1, false, false,
+		"Deep water is dangerous to traverse without the appropriate equipment. Very easy to lose your footing, or go under.")
+	biomeInteriorCaptured = biomeFixture("Interior", "⌂", 0.4, true, false,
+		"Inside a built structure, whether a house, a hall, a temple or the sealed corridors of something older. Lit by what its builders put there rather than by the sky.")
+	biomeStreetLamp = biomeFixture("City Streets", "#", 1, true, true,
+		"Cobbled streets between close-set buildings, busy by day and watched by night. Lamplighters make their rounds at dusk.")
+)
+
+// identifyFixture is the data identify and appraise pass: the item and its
+// spec.
+func identifyFixture(item items.Item, spec items.ItemSpec) func() any {
+	return func() any {
+		itm, sp := item, spec
+		itm.Spec = &sp
+		return struct {
+			Item     *items.Item
+			ItemSpec *items.ItemSpec
+		}{&itm, &sp}
+	}
+}
+
+// identifyCaptured is the hull-plate cuirass the capture identified
+// (items/armor-20000/body/20091-hull_plate_cuirass.yaml).
+var identifyCaptured = identifyFixture(items.Item{ItemId: 20091}, items.ItemSpec{
+	ItemId: 20091, Name: "Hull-Plate Cuirass", NameSimple: "cuirass", Type: items.Body, Subtype: items.Wearable,
+	Description:        "A breastplate cut and folded from a curved section of the grey material, shaped somehow to the wearer without a rivet or a seam in sight. It is lighter than plate that size has any right to be and colder against the body, and it turns a blow with a low ringing note that hangs in the air. Long scores mar its face where something once tested it and failed. It was lifted from the leavings the buried place kept of those who came before.",
+	PhysicalMitigation: 16, MagicalMitigation: 8, ConvictionMitigation: 5,
+	StatMods: statmods.StatMods{"vitality": 8, "strength": 4}, Value: 1200,
+})
+
+// identifyWide is a cursed caster weapon with every section full: a long
+// name, an element, applied and crit conditions, and stat mods.
+var identifyWide = identifyFixture(items.Item{ItemId: 99230}, func() items.ItemSpec {
+	spec := items.ItemSpec{
+		ItemId: 99230, Name: "Pit Fighter's Serrated Basalt-Iron Blade", Type: items.Weapon, Subtype: items.Slashing,
+		Description: "A heavy blade of dark basalt-iron, its edge cut into teeth.", Cursed: true,
+		DamageMultiplier: 1.4, SpellDamageMultiplier: 1.2, Element: items.Electricity,
+		ConditionIds: []int{940}, StatMods: statmods.StatMods{"strength": 25, "perception": -12, "vitality": 3},
+		Value: 1234567,
+	}
+	spec.Damage.CritConditionIds = []int{941}
+	return spec
+}())
+
+// containerFixture is the data look passes for what is inside a container.
+func containerFixture(names ...string) func() any {
+	return func() any {
+		formatted := make([]string, len(names))
+		for i, n := range names {
+			formatted[i] = `<ansi fg="itemname">` + n + `</ansi>`
+		}
+		return map[string]any{`ItemNames`: names, `ItemNamesFormatted`: formatted}
+	}
+}
+
+var (
+	containerCaptured = containerFixture("Pit Fighter's Sword", "Reinforced Travel Pack")
+	containerWide     = containerFixture("1,234,567 gold", "A Salt-Stiff Page of a Sailor's Telling",
+		"Pit Fighter's Serrated Basalt-Iron Blade", "Reinforced Travel Pack", "Strongbox Key", "Iron Ore")
+	containerEmpty = containerFixture()
+)
+
+// trackCaptured is `track` in Market Square as captured; trackWide has long
+// names and the trails' exits.
+func trackCaptured() any {
+	return []actions.TrackingInfo{{Name: "City Guard", Type: "mob", Strength: "Warm"}}
+}
+
+func trackWide() any {
+	return []actions.TrackingInfo{
+		{Name: "Drillmaster Vorn the Unyielding", Type: "mob", Strength: "Fresh", ExitName: "northwest"},
+		{Name: "Bartholomewsworthingtonian", Type: "user", Strength: "Faint", ExitName: "shimmering portal"},
+		{Name: "City Guard", Type: "mob", Strength: "Warm"},
+	}
+}
+
+// roomViewFixture is the room description block as look passes it: prose
+// already wrapped (rooms.GetDetails wraps it to the reader, so it is short
+// here), the alerts GetDetails lists, and a tracking line. Wide adds the
+// longest shipped alerts: the character room, the start room, and the
+// wildfire and blizzard mutators' texts.
+func roomViewFixture(wide bool) func() any {
+	return func() any {
+		d := rooms.RoomTemplateDetails{
+			Description: "A solid stone building\nwith iron-banded doors.",
+			RoomAlerts:  []string{rooms.AlertBank, rooms.AlertStorage},
+		}
+		if wide {
+			d.RoomAlerts = append(d.RoomAlerts, rooms.AlertCharacterRoom, rooms.AlertStart,
+				"!!!                A wildfire is burning here!                !!!",
+				"A blizzard howls through here.")
+			d.TrackingString = `Tracking <ansi fg="mobname">Drillmaster Vorn the Unyielding</ansi>... They went <ansi fg="exit">northwest</ansi>`
+			d.IsNight = true
+		}
+		return d
+	}
+}
+
+// roomTitleFixture is the title line of a room as look passes it.
+func roomTitleFixture(title, zone string, pvp bool) func() any {
+	return func() any {
+		return rooms.RoomTemplateDetails{Title: title, Zone: zone, RoomSymbol: "⌂", RoomLegend: "Bank", ShowPvp: pvp}
+	}
+}
+
+var (
+	roomTitleCaptured = roomTitleFixture("Thornwall Bank", "Thornwall City", false)
+	roomTitleWide     = roomTitleFixture("The Drowned Lighthouse's Lower Gallery", "Saltmarsh Coast Ruins", true)
+)
+
+// exitsFixture is the exits line of a room: plain, locked, unlocked and
+// secret exits, and a temporary one.
+func exitsFixture(wide bool) func() any {
+	return func() any {
+		d := rooms.RoomTemplateDetails{
+			VisibleExits:   map[string]exit.RoomExit{"north": {RoomId: 1}},
+			TemporaryExits: map[string]exit.TemporaryRoomExit{},
+		}
+		if wide {
+			d.VisibleExits = map[string]exit.RoomExit{
+				"north": {RoomId: 1}, "south": {RoomId: 2}, "east": {RoomId: 3}, "west": {RoomId: 4},
+				"northeast": {RoomId: 5, Lock: gamelock.Lock{Difficulty: 5}},
+				"southwest": {RoomId: 6, Lock: gamelock.Lock{Difficulty: 5, UnlockedRound: 1}},
+				"up":        {RoomId: 7, Secret: true}, "down": {RoomId: 8},
+			}
+			d.TemporaryExits = map[string]exit.TemporaryRoomExit{"portal": {RoomId: 9, Title: "a shimmering violet portal"}}
+		}
+		return d
+	}
+}
+
+// exitsMixed is an exits line that fits 80 with every kind of exit: plain,
+// locked, unlocked (opened this round), secret and temporary.
+func exitsMixed() any {
+	return rooms.RoomTemplateDetails{
+		VisibleExits: map[string]exit.RoomExit{
+			"down":  {RoomId: 1},
+			"north": {RoomId: 2, Lock: gamelock.Lock{Difficulty: 5}},
+			"east":  {RoomId: 3, Lock: gamelock.Lock{Difficulty: 5, UnlockedRound: util.GetRoundCount() + 1}},
+			"up":    {RoomId: 4, Secret: true},
+		},
+		TemporaryExits: map[string]exit.TemporaryRoomExit{"ladder": {RoomId: 5, Title: "a rope ladder"}},
+	}
+}
+
+// fixturesFixture is the room's fixture lines as look passes them.
+func fixturesFixture(names ...string) func() any {
+	return func() any {
+		type line struct {
+			Name     string
+			Lit      bool
+			Darkness bool
+		}
+		lines := []line{}
+		for i, n := range names {
+			lines = append(lines, line{Name: n, Lit: i%2 == 0, Darkness: i == 2})
+		}
+		return map[string]any{`Fixtures`: lines, `IsDark`: false, `IsNight`: true}
+	}
+}
+
+var (
+	fixturesShort = fixturesFixture("brazier", "lamp", "shadowstone")
+	fixturesWide  = fixturesFixture("great iron chandelier of the Thornwall Temple", "lamp", "obsidian shard of the deep dark")
+)
+
+// signFixture is a sign or a rune as the room holds it.
+func signFixture(text string) func() any {
+	return func() any { return rooms.Sign{DisplayText: text} }
+}
+
+var (
+	signShort = signFixture("Welcome")
+	signWide  = signFixture("Beware the marsh at night. The lights you see out there are not lanterns, and they do not want you home.")
+	// A player-written sign with one word longer than a whole line at 42.
+	signLongWord = signFixture("Keep out: " + strings.Repeat("AAAAAAAAAA", 5) + " by order of the Watch.")
+)
+
+// lookingScreenGoldens are the screens whose 80-column output is pinned.
+// Each is keyed by the template and a short name for its data.
+var lookingScreenGoldens = map[string]struct {
+	name    string
+	fixture templateFixture
+}{
+	"description_captured":        {"character/description", templateFixture{setup: seedLookingScreenRegistries, data: descriptionCaptured}},
+	"description_wide":            {"character/description", templateFixture{setup: seedLookingScreenRegistries, data: descriptionWide}},
+	"description-corpse_captured": {"character/description-corpse", templateFixture{data: corpseCaptured}},
+	"inventory-look_captured":     {"character/inventory-look", templateFixture{data: inventoryLookFixture(false, false)}},
+	"inventory-look_hidden":       {"character/inventory-look", templateFixture{data: inventoryLookFixture(false, true)}},
+	"pet_captured":                {"character/pet", templateFixture{data: petFixture(false)}},
+	"identify_captured":           {"descriptions/identify", templateFixture{setup: seedLookingScreenRegistries, data: identifyCaptured}},
+	"identify_wide":               {"descriptions/identify", templateFixture{setup: seedLookingScreenRegistries, data: identifyWide}},
+	"insidecontainer_captured":    {"descriptions/insidecontainer", templateFixture{data: containerCaptured}},
+	"insidecontainer_wide":        {"descriptions/insidecontainer", templateFixture{data: containerWide}},
+	"insidecontainer_empty":       {"descriptions/insidecontainer", templateFixture{data: containerEmpty}},
+	"track_captured":              {"descriptions/track", templateFixture{data: trackCaptured}},
+	"room-title_captured":         {"descriptions/room-title", templateFixture{data: roomTitleCaptured}},
+	"exits_captured":              {"descriptions/exits", templateFixture{data: exitsFixture(false)}},
+	"exits_mixed":                 {"descriptions/exits", templateFixture{data: exitsMixed}},
+	"room_tracking":               {"descriptions/room", templateFixture{data: trackingAfterMob(rissMobFile)}},
+	"fixtures_short":              {"descriptions/fixtures", templateFixture{data: fixturesShort}},
+	"fixtures_wide":               {"descriptions/fixtures", templateFixture{data: fixturesWide}},
+	"rune_short":                  {"descriptions/rune", templateFixture{data: signShort}},
+	"sign_short":                  {"descriptions/sign", templateFixture{data: signShort}},
+	// Recorded on the branch: owner-approved 80-column changes (plan 3b-2):
+	// the alerts between full-width rules, and the biome box drawn as two
+	// 80-wide rules with its lighting wrapped under its value.
+	"room_alerts":    {"descriptions/room", templateFixture{data: roomViewFixture(false)}},
+	"biome_outdoor":  {"descriptions/biome", templateFixture{data: biomeOutdoorCaptured}},
+	"biome_interior": {"descriptions/biome", templateFixture{data: biomeInteriorCaptured}},
+	"biome_street":   {"descriptions/biome", templateFixture{data: biomeStreetLamp}},
+}
+
+// A name is never split (owner, 2026-11-10: a single unbreakable word is an
+// accepted exception, as on the status sheet). With a 32-letter name, the
+// configured maximum for players and pets, looking at a person, a corpse or
+// a pet fits at 42 and 80; at 30 a line runs past only where the name is
+// alone on it, beside at most its ".: " heading or its indent and one stop.
+func TestLookingWithTheLongestNameRunsPastOnlyOnItsName(t *testing.T) {
+	templateRenderSetup(t)
+	seedLookingScreenRegistries(t)
+	if len(lookingLongestName) != 32 {
+		t.Fatalf("the name is %d letters, not the 32 maximum", len(lookingLongestName))
+	}
+	tags := regexp.MustCompile(`</?ansi[^>]*>`)
+	alone := regexp.MustCompile(`^(\.: | *)` + lookingLongestName + `[.,]?$`)
+	screens := map[string]any{
+		"character/description":        lookedAt(lookingLongestName, "They seem thoroughly uninteresting.", 100, 100, nil),
+		"character/description-corpse": lookedAt(lookingLongestName, "A body.", 0, 10, nil),
+		"character/pet":                petNamed(false, lookingLongestName, lookingLongestName)(),
+	}
+	for name, data := range screens {
+		for _, width := range templateWidths {
+			out, err := templates.ProcessAtWidth(name, data, width)
+			if err != nil {
+				t.Fatal(err)
+			}
+			for _, line := range strings.Split(tags.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), ""), "\n") {
+				got := templates.WidestLine(line)
+				if got <= width {
+					continue
+				}
+				if width != 30 || !alone.MatchString(line) {
+					t.Errorf("%s at %d: a line other than the name alone is %d wide: %q\n%s", name, width, got, line, out)
+				} else {
+					t.Logf("%s at 30: the name's own line is %d wide: %q", name, got, line)
+				}
+			}
+		}
+	}
+}
+
+// The shipped wildfire mutator's alert, with its per-letter flame colours,
+// reads as its author spaced it at every width: "!!!", the sentence, "!!!",
+// on one line when it fits, else one to a line, each centred.
+func TestShippedWildfireAlertReadsWhole(t *testing.T) {
+	templateRenderSetup(t)
+	root := filepath.Join(repoRootPath, dogmudDataFilesRoot)
+	var spec mutators.MutatorSpec
+	raw, err := os.ReadFile(filepath.Join(root, "mutators", "wildfire.yaml"))
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := yaml.Unmarshal(raw, &spec); err != nil || spec.AlertModifier == nil {
+		t.Fatalf("wildfire.yaml has no alert (%v)", err)
+	}
+	var patterns map[string][]int
+	raw, err = os.ReadFile(filepath.Join(root, "color-patterns.yaml"))
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := yaml.Unmarshal(raw, &patterns); err != nil || len(patterns[spec.AlertModifier.ColorPattern]) == 0 {
+		t.Fatalf("color-patterns.yaml has no %q pattern (%v)", spec.AlertModifier.ColorPattern, err)
+	}
+	t.Cleanup(colorpatterns.SeedPatternsForTest(patterns))
+	alert := colorpatterns.ApplyColorPattern(spec.AlertModifier.Text, spec.AlertModifier.ColorPattern)
+	if !strings.Contains(alert, `<ansi fg=`) {
+		t.Fatal("the flame pattern was not applied; the test proves nothing")
+	}
+
+	pad := func(width int, s string) string { return strings.Repeat(" ", (width-len(s))/2) + s }
+	want := map[int][]string{
+		80: {pad(80, spec.AlertModifier.Text)},
+		42: {pad(42, "!!!"), pad(42, "A wildfire is burning here!"), pad(42, "!!!")},
+		30: {pad(30, "!!!"), pad(30, "A wildfire is burning here!"), pad(30, "!!!")},
+	}
+	tags := regexp.MustCompile(`</?ansi[^>]*>`)
+	for width, lines := range want {
+		out, err := templates.ProcessAtWidth("descriptions/room", rooms.RoomTemplateDetails{RoomAlerts: []string{alert}}, width)
+		if err != nil {
+			t.Fatal(err)
+		}
+		plain := strings.Split(strings.Trim(tags.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), ""), "\n"), "\n")
+		// The rules are the first and last lines; the alert is between.
+		if len(plain) < 3 || strings.Join(plain[1:len(plain)-1], "|") != strings.Join(lines, "|") {
+			t.Errorf("at %d the wildfire alert reads\n%q\nwant\n%q", width, plain, lines)
+		}
+	}
+}
+
+// An exit and its tag, and the room title's PvP tag, are one unit each: never
+// split across lines while the unit fits on a line by itself (owner,
+// 2026-11-10), at every narrow width.
+//
+// Every line fits too, but for one choice-9 line: a 32-letter single-word
+// name, which cannot fit at 30, alone on its line with what is glued to it.
+func TestRoomViewUnitsStayWhole(t *testing.T) {
+	templateRenderSetup(t)
+	tags := regexp.MustCompile(`</?ansi[^>]*>`)
+	lone := regexp.MustCompile(`^ *` + lookingLongestName + `\S*$`)
+	riss := shippedMobLooked(rissMobFile)().(*characters.Character).Name
+	for _, tc := range []struct {
+		name   string
+		data   func() any
+		widths []int
+		units  []string
+	}{
+		{"descriptions/exits", exitsFixture(true), []int{30, 42}, []string{"northeast (locked)", "southwest (locked)", "(up)"}},
+		{"descriptions/exits", func() any {
+			return rooms.RoomTemplateDetails{
+				VisibleExits: map[string]exit.RoomExit{"north": {RoomId: 1},
+					"northeast": {RoomId: 5, Lock: gamelock.Lock{Difficulty: 5}}},
+				TemporaryExits: map[string]exit.TemporaryRoomExit{},
+			}
+		}, []int{30, 42}, []string{"northeast (locked)"}},
+		{"descriptions/room-title", roomTitleWide, []int{30, 42}, []string{"☠ PK Area ☠"}},
+		// The tracking line as rooms.GetDetails builds it. At 30 the
+		// 29-letter name with its "..." (32) is wider than a line, so it may
+		// wrap; at 42 it stays whole.
+		{"descriptions/room", trackingRoom(riss, "a shimmering violet portal"), []int{42}, []string{riss, "a shimmering violet portal"}},
+		{"descriptions/room", trackingRoom(riss, "a shimmering violet portal"), []int{30}, []string{"a shimmering violet portal"}},
+		{"descriptions/room", trackingRoom(lookingLongestName, "north"), []int{30, 42}, []string{lookingLongestName}},
+	} {
+		for _, width := range []int{30, 42} {
+			out, err := templates.ProcessAtWidth(tc.name, tc.data(), width)
+			if err != nil {
+				t.Fatal(err)
+			}
+			plain := tags.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), "")
+			for _, line := range strings.Split(plain, "\n") {
+				if w := templates.WidestLine(line); w > width && !(width == 30 && lone.MatchString(line)) {
+					t.Errorf("%s at %d has a line %d wide: %q\n%s", tc.name, width, w, line, plain)
+				}
+			}
+			if !slices.Contains(tc.widths, width) {
+				continue
+			}
+			for _, unit := range tc.units {
+				if !strings.Contains(plain, unit) {
+					t.Errorf("%s at %d splits %q:\n%s", tc.name, width, unit, plain)
+				}
+			}
+		}
+	}
+}
+
+// trackingRoom is a room whose tracking line follows name out by exitName,
+// as rooms.GetDetails builds it.
+func trackingRoom(name, exitName string) func() any {
+	return func() any {
+		return rooms.RoomTemplateDetails{Description: "A quiet room.",
+			TrackingString: `Tracking <ansi fg="mobname">` + name + `</ansi>... They went <ansi fg="exit">` + exitName + `</ansi>`}
+	}
+}
+
+// trackingAfterMob is trackingRoom for the shipped mob in file.
+func trackingAfterMob(file string) func() any {
+	return func() any {
+		return trackingRoom(shippedMobLooked(file)().(*characters.Character).Name, "a shimmering violet portal")()
+	}
+}
+
+func lookingScreenGoldenPath(key string) string {
+	return testdataPath(filepath.Join("looking_screens_80", key+".golden"))
+}
+
+// A desktop sees what it always did: at 80 columns each screen renders
+// exactly the bytes in its golden, line ends aside.
+func TestLookingScreensKeepTheir80ColumnOutput(t *testing.T) {
+	templateRenderSetup(t)
+	for key, g := range lookingScreenGoldens {
+		t.Run(key, func(t *testing.T) {
+			if g.fixture.setup != nil {
+				g.fixture.setup(t)
+			}
+			out, err := templates.ProcessAtWidth(g.name, g.fixture.data(), 80)
+			if err != nil {
+				t.Fatal(err)
+			}
+			got := strings.ReplaceAll(out, "\r\n", "\n")
+			path := lookingScreenGoldenPath(key)
+			if *updateLookingScreens {
+				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
+					t.Fatal(err)
+				}
+				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
+					t.Fatal(err)
+				}
+			}
+			want, err := os.ReadFile(path)
+			if err != nil {
+				t.Fatalf("read golden: %v (record it from master with -update-looking-screens)", err)
+			}
+			if w := templates.WidestLine(string(want)); w > 80 {
+				t.Fatalf("the golden is %d wide: its fixture no longer fits 80, so it pins a form, not the base", w)
+			}
+			if got != string(want) {
+				t.Errorf("%s at 80 changed:\n%s", key, lineDiff(string(want), got))
+			}
+		})
+	}
+}
diff --git a/internal/guards/template_width_guard_test.go b/internal/guards/template_width_guard_test.go
index 2b4b79b8c..809a4261e 100644
--- a/internal/guards/template_width_guard_test.go
+++ b/internal/guards/template_width_guard_test.go
@@ -27,7 +27,7 @@ import (
 // ones that must fit at 30, 42 and 80.
 //
 // The classes are the state of the work, not a permanent exemption list:
-// plan 3b empties "plan 3b", plan 4 does "help/". When both are done, only
+// plan 3b-3 empties "plan 3b-3", plan 4 does "help/". When both are done, only
 // "wide" and the classes that never reach a narrow screen unwrapped remain.
 
 // templateFolders are every place a template is read from: the live world,
@@ -67,14 +67,8 @@ var templateClassNames = map[string][]string{
 	// (TestNarrowTemplateScreensFitEveryWidth): the data its commands pass
 	// is of types private to that package.
 	"fits in usercommands": {"character/conditions", "character/quests", "character/title"},
-	// Narrow forms come in plan 3b.
-	"plan 3b": {"character/description", "character/description-corpse",
-		"character/inventory-look", "character/pet",
-		"character/skillup", "character/statup",
-		"descriptions/biome", "descriptions/identify",
-		"descriptions/insidecontainer", "descriptions/track",
-		"descriptions/exits", "descriptions/fixtures", "descriptions/room",
-		"descriptions/room-title", "descriptions/rune", "descriptions/sign",
+	// Narrow forms come in plan 3b-3.
+	"plan 3b-3": {"character/skillup", "character/statup",
 		"mail/message", "auctions/auction-bid", "auctions/auction-end",
 		"auctions/auction-start", "auctions/auction-update"},
 	// No production code renders these (survey 2026-11-10).
@@ -131,6 +125,21 @@ var templateFits = map[string]templateFixture{
 	"character/skills":    {data: skillsScreenFixture},
 	"character/cooldowns": {data: cooldownsScreenFixture},
 	"character/storage":   {data: storageScreenFixture},
+	// The looking-at-things screens (looking_screens_test.go).
+	"character/description":        {setup: seedLookingScreenRegistries, data: descriptionWide, more: []func() any{descriptionCaptured, shippedMobLooked(rissMobFile), shippedMobLooked(odellMobFile)}},
+	"character/description-corpse": {data: corpseWide, more: []func() any{corpseCaptured, shippedMobLooked(rissMobFile), shippedMobLooked(odellMobFile)}},
+	"character/inventory-look":     {data: inventoryLookFixture(true, false), more: []func() any{inventoryLookFixture(false, false), inventoryLookFixture(false, true)}},
+	"character/pet":                {data: petFixture(true), more: []func() any{petFixture(false), petNamedAfterMob(rissMobFile), petNamedAfterMob(odellMobFile)}},
+	"descriptions/biome":           {data: biomeStreetLamp, more: []func() any{biomeOutdoorCaptured, biomeInteriorCaptured}},
+	"descriptions/identify":        {setup: seedLookingScreenRegistries, data: identifyWide, more: []func() any{identifyCaptured}},
+	"descriptions/insidecontainer": {data: containerWide, more: []func() any{containerCaptured, containerEmpty}},
+	"descriptions/track":           {data: trackWide, more: []func() any{trackCaptured}},
+	"descriptions/room":            {data: roomViewFixture(true), more: []func() any{roomViewFixture(false), trackingAfterMob(rissMobFile), trackingAfterMob(odellMobFile)}},
+	"descriptions/room-title":      {data: roomTitleWide, more: []func() any{roomTitleCaptured}},
+	"descriptions/exits":           {data: exitsFixture(true), more: []func() any{exitsFixture(false), exitsMixed}},
+	"descriptions/fixtures":        {data: fixturesWide, more: []func() any{fixturesShort}},
+	"descriptions/rune":            {data: signWide, more: []func() any{signShort, signLongWord}},
+	"descriptions/sign":            {data: signWide, more: []func() any{signShort, signLongWord}},
 }
 
 // templateClasses names every class that holds name.
diff --git a/internal/guards/testdata/looking_screens_80/biome_interior.golden b/internal/guards/testdata/looking_screens_80/biome_interior.golden
new file mode 100644
index 000000000..0a7f81cc3
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/biome_interior.golden
@@ -0,0 +1,9 @@
+── <ansi fg="black-bold">.:</ansi> Biome Info ───────────────────────────────────────────────────────────────
+  <ansi fg="yellow">Name:</ansi>        Interior
+  <ansi fg="yellow">Symbol:</ansi>      ⌂
+  <ansi fg="yellow">Lighting:</ansi>    Only a little sky reaches here, filtered and dim. A lamp
+               keeps it lit after dark.
+  <ansi fg="yellow">Description:</ansi> Inside a built structure, whether a house, a hall, a temple
+               or the sealed corridors of something older. Lit by what its
+               builders put there rather than by the sky.
+────────────────────────────────────────────────────────────────────────────────
diff --git a/internal/guards/testdata/looking_screens_80/biome_outdoor.golden b/internal/guards/testdata/looking_screens_80/biome_outdoor.golden
new file mode 100644
index 000000000..07ad8e018
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/biome_outdoor.golden
@@ -0,0 +1,7 @@
+── <ansi fg="black-bold">.:</ansi> Biome Info ───────────────────────────────────────────────────────────────
+  <ansi fg="yellow">Name:</ansi>        Deep Water
+  <ansi fg="yellow">Symbol:</ansi>      ≈
+  <ansi fg="yellow">Lighting:</ansi>    The open sky reaches this place freely.
+  <ansi fg="yellow">Description:</ansi> Deep water is dangerous to traverse without the appropriate
+               equipment. Very easy to lose your footing, or go under.
+────────────────────────────────────────────────────────────────────────────────
diff --git a/internal/guards/testdata/looking_screens_80/biome_street.golden b/internal/guards/testdata/looking_screens_80/biome_street.golden
new file mode 100644
index 000000000..c1b21dc3c
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/biome_street.golden
@@ -0,0 +1,9 @@
+── <ansi fg="black-bold">.:</ansi> Biome Info ───────────────────────────────────────────────────────────────
+  <ansi fg="yellow">Name:</ansi>        City Streets
+  <ansi fg="yellow">Symbol:</ansi>      #
+  <ansi fg="yellow">Lighting:</ansi>    The open sky reaches this place freely. Street lamps keep
+               it lit from dusk to dawn.
+  <ansi fg="yellow">Description:</ansi> Cobbled streets between close-set buildings, busy by day
+               and watched by night. Lamplighters make their rounds at
+               dusk.
+────────────────────────────────────────────────────────────────────────────────
diff --git a/internal/guards/testdata/looking_screens_80/description-corpse_captured.golden b/internal/guards/testdata/looking_screens_80/description-corpse_captured.golden
new file mode 100644
index 000000000..00fdf9e52
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/description-corpse_captured.golden
@@ -0,0 +1,10 @@
+
+<ansi fg="black-bold">.:</ansi> <ansi fg="8">Marsh Rat corpse</ansi>
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> ────────────────────────────────────────────────────────────┐
+   <ansi fg="red-bold">☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠</ansi>
+   <ansi fg="8">A small grey-brown marsh rat, wet-furred and quick, with bright black
+   eyes and a pink scaly tail. It freezes at your approach and weighs
+   whether you are a problem. </ansi>
+   <ansi fg="red-bold">☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠ ☠</ansi>
+   <ansi fg="8">This is a corpse. They are dead.</ansi>
+ └────────────────────────────────────────────────────────────────────────────┘
diff --git a/internal/guards/testdata/looking_screens_80/description_captured.golden b/internal/guards/testdata/looking_screens_80/description_captured.golden
new file mode 100644
index 000000000..f53e5be00
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/description_captured.golden
@@ -0,0 +1,6 @@
+
+<ansi fg="black-bold">.:</ansi> <ansi fg="username">Screenly</ansi>
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> ────────────────────────────────────────────────────────────┐
+   They seem thoroughly uninteresting.
+   <ansi fg="username">Screenly</ansi> is in <ansi fg="health-100">perfect health.</ansi>
+ └────────────────────────────────────────────────────────────────────────────┘
diff --git a/internal/guards/testdata/looking_screens_80/description_wide.golden b/internal/guards/testdata/looking_screens_80/description_wide.golden
new file mode 100644
index 000000000..310a51b33
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/description_wide.golden
@@ -0,0 +1,11 @@
+
+<ansi fg="black-bold">.:</ansi> <ansi fg="username">Bartholomewsworthingtonian</ansi>
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> ────────────────────────────────────────────────────────────┐
+   A tall, broad-shouldered traveller in a salt-stained greatcoat, with a
+   weathered face and a nose that has been broken more than once. Their
+   boots are caked with the grey mud of the lower marsh.
+   A brass compass hangs from a cord at their throat, its needle trembling.
+   <ansi fg="magenta">Their skin has the faint blue sheen of old ice, and frost gathers in the
+   creases of their knuckles. A long, sinuous tail sways behind them.</ansi>
+   <ansi fg="username">Bartholomewsworthingtonian</ansi> looks like they're <ansi fg="health-0">about to die!</ansi>
+ └────────────────────────────────────────────────────────────────────────────┘
diff --git a/internal/guards/testdata/looking_screens_80/exits_captured.golden b/internal/guards/testdata/looking_screens_80/exits_captured.golden
new file mode 100644
index 000000000..2e7671b0d
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/exits_captured.golden
@@ -0,0 +1 @@
+<ansi fg="yellow">Exits: </ansi><ansi fg="exit">north</ansi>
diff --git a/internal/guards/testdata/looking_screens_80/exits_mixed.golden b/internal/guards/testdata/looking_screens_80/exits_mixed.golden
new file mode 100644
index 000000000..59666da57
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/exits_mixed.golden
@@ -0,0 +1 @@
+<ansi fg="yellow">Exits: </ansi><ansi fg="exit">down</ansi>, <ansi fg="exit">east</ansi> (unlocked), <ansi fg="exit">north</ansi> (locked), <ansi fg="secret-exit">(up)</ansi>, <ansi fg="exit">a rope ladder</ansi>
diff --git a/internal/guards/testdata/looking_screens_80/fixtures_short.golden b/internal/guards/testdata/looking_screens_80/fixtures_short.golden
new file mode 100644
index 000000000..6d5ef0957
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/fixtures_short.golden
@@ -0,0 +1,3 @@
+<ansi fg="room-description-dark">The <ansi fg="item">brazier</ansi> is lit.</ansi>
+<ansi fg="room-description-dark">The <ansi fg="item">lamp</ansi> is unlit.</ansi>
+<ansi fg="room-description-dark">The <ansi fg="item">shadowstone</ansi> swallows the light.</ansi>
diff --git a/internal/guards/testdata/looking_screens_80/fixtures_wide.golden b/internal/guards/testdata/looking_screens_80/fixtures_wide.golden
new file mode 100644
index 000000000..eeba9bad5
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/fixtures_wide.golden
@@ -0,0 +1,3 @@
+<ansi fg="room-description-dark">The <ansi fg="item">great iron chandelier of the Thornwall Temple</ansi> is lit.</ansi>
+<ansi fg="room-description-dark">The <ansi fg="item">lamp</ansi> is unlit.</ansi>
+<ansi fg="room-description-dark">The <ansi fg="item">obsidian shard of the deep dark</ansi> swallows the light.</ansi>
diff --git a/internal/guards/testdata/looking_screens_80/identify_captured.golden b/internal/guards/testdata/looking_screens_80/identify_captured.golden
new file mode 100644
index 000000000..867ac9c7a
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/identify_captured.golden
@@ -0,0 +1,28 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Basic Info</ansi> ──────────────────────────────────────────────────────────────┐
+   <ansi fg="yellow">Name:</ansi>        Hull-Plate Cuirass                                   
+   <ansi fg="yellow">Description:</ansi> A breastplate cut and folded from a curved section of the
+                grey material, shaped somehow to the wearer without a rivet
+                or a seam in sight. It is lighter than plate that size has
+                any right to be and colder against the body, and it turns a
+                blow with a low ringing note that hangs in the air. Long
+                scores mar its face where something once tested it and
+                failed. It was lifted from the leavings the buried place kept
+                of those who came before.
+   <ansi fg="yellow">Type:</ansi>        Body (Wearable)
+   <ansi fg="yellow">Value:</ansi>       1200 gold                                            
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Combat Properties</ansi> ──────────────────────────────────────────────────────┐
+   <ansi fg="yellow">Striking Power:</ansi>  N/A                                            
+   <ansi fg="yellow">Arcane Resonance:</ansi> N/A                                           
+   <ansi fg="yellow">Physical:</ansi>    light                                                
+   <ansi fg="yellow">Magical:</ansi>     thin                                                 
+   <ansi fg="yellow">Conviction:</ansi>  thin                                                 
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Modifiers</ansi> ───────────────────────────────────────────────────────────────┐
+   slightly bolsters your strength
+   slightly bolsters your vitality
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Magical Effects</ansi> ─────────────────────────────────────────────────────────┐
+   None detected.
+ └─────────────────────────────────────────────────────────────────────────────┘
+
diff --git a/internal/guards/testdata/looking_screens_80/identify_wide.golden b/internal/guards/testdata/looking_screens_80/identify_wide.golden
new file mode 100644
index 000000000..97b5d2218
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/identify_wide.golden
@@ -0,0 +1,25 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Basic Info</ansi> ──────────────────────────────────────────────────────────────┐
+   <ansi fg="yellow">Name:</ansi>        Pit Fighter'S Serrated Basalt-Iron Blade             
+   <ansi fg="yellow">Description:</ansi> A heavy blade of dark basalt-iron, its edge cut into teeth.
+   <ansi fg="yellow">Type:</ansi>        Weapon (Slashing)
+   <ansi fg="yellow">Value:</ansi>       1234567 gold                                         
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Combat Properties</ansi> ──────────────────────────────────────────────────────┐
+   <ansi fg="yellow">Striking Power:</ansi>  moderate striking power                        
+   <ansi fg="yellow">Arcane Resonance:</ansi> moderate arcane resonance                     
+   <ansi fg="yellow">Physical:</ansi>    N/A                                                  
+   <ansi fg="yellow">Magical:</ansi>     N/A                                                  
+   <ansi fg="yellow">Conviction:</ansi>  N/A                                                  
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Modifiers</ansi> ───────────────────────────────────────────────────────────────┐
+   saps your perception
+   greatly bolsters your strength
+   slightly bolsters your vitality
+   <ansi fg="yellow">Applies:</ansi>     <ansi fg="spellname">Chrysalis Regeneration</ansi> - Activates every round (50x total)
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Magical Effects</ansi> ─────────────────────────────────────────────────────────┐
+   It's <ansi fg="red-bold">CURSED!</ansi>
+   <ansi fg="yellow">Element:</ansi>     Electricity                                          
+   <ansi fg="yellow">Crits Apply:</ansi> <ansi fg="spellname">Death's Shadow</ansi> - Activates every round (5x total)
+ └─────────────────────────────────────────────────────────────────────────────┘
+
diff --git a/internal/guards/testdata/looking_screens_80/insidecontainer_captured.golden b/internal/guards/testdata/looking_screens_80/insidecontainer_captured.golden
new file mode 100644
index 000000000..11486b4e9
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/insidecontainer_captured.golden
@@ -0,0 +1,5 @@
+<ansi fg="white"> ┌─────────────────────────────────────────────────────────────────────────┐</ansi>
+
+  <ansi fg="white">Inside: </ansi><ansi fg="itemname">Pit Fighter's Sword</ansi>, <ansi fg="itemname">Reinforced Travel Pack</ansi>
+
+<ansi fg="white"> └─────────────────────────────────────────────────────────────────────────┘</ansi>
\ No newline at end of file
diff --git a/internal/guards/testdata/looking_screens_80/insidecontainer_empty.golden b/internal/guards/testdata/looking_screens_80/insidecontainer_empty.golden
new file mode 100644
index 000000000..f966e1bd2
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/insidecontainer_empty.golden
@@ -0,0 +1,5 @@
+<ansi fg="white"> ┌─────────────────────────────────────────────────────────────────────────┐</ansi>
+
+  <ansi fg="white">Inside: </ansi>Nothing
+
+<ansi fg="white"> └─────────────────────────────────────────────────────────────────────────┘</ansi>
\ No newline at end of file
diff --git a/internal/guards/testdata/looking_screens_80/insidecontainer_wide.golden b/internal/guards/testdata/looking_screens_80/insidecontainer_wide.golden
new file mode 100644
index 000000000..65dc26f61
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/insidecontainer_wide.golden
@@ -0,0 +1,7 @@
+<ansi fg="white"> ┌─────────────────────────────────────────────────────────────────────────┐</ansi>
+
+  <ansi fg="white">Inside: </ansi><ansi fg="itemname">1,234,567 gold</ansi>, <ansi fg="itemname">A Salt-Stiff Page of a Sailor's Telling</ansi>, 
+          <ansi fg="itemname">Pit Fighter's Serrated Basalt-Iron Blade</ansi>, <ansi fg="itemname">Reinforced Travel Pack</ansi>, 
+          <ansi fg="itemname">Strongbox Key</ansi>, <ansi fg="itemname">Iron Ore</ansi>
+
+<ansi fg="white"> └─────────────────────────────────────────────────────────────────────────┘</ansi>
\ No newline at end of file
diff --git a/internal/guards/testdata/looking_screens_80/inventory-look_captured.golden b/internal/guards/testdata/looking_screens_80/inventory-look_captured.golden
new file mode 100644
index 000000000..497edb4cc
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/inventory-look_captured.golden
@@ -0,0 +1,20 @@
+┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Equipment</ansi> ──────────────────────────────────────────────────────────────┐
+   <ansi fg="yellow">Weapon:     </ansi><ansi fg="itemname">longsword</ansi>
+   <ansi fg="yellow">Offhand:    </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Head:       </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Neck:       </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Shoulders:  </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Body:       </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Back:       </ansi><ansi fg="itemname">pack</ansi>
+   <ansi fg="yellow">Belt:       </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Wrist 1:    </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Wrist 2:    </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Gloves:     </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Ring:       </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Ring:       </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Legs:       </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Feet:       </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Light:      </ansi><ansi fg="itemname">-nothing-</ansi>
+   <ansi fg="yellow">Components: </ansi><ansi fg="itemname">-nothing-</ansi>
+ └────────────────────────────────────────────────────────────────────────────┘
+ Carrying: several objects
diff --git a/internal/guards/testdata/looking_screens_80/inventory-look_hidden.golden b/internal/guards/testdata/looking_screens_80/inventory-look_hidden.golden
new file mode 100644
index 000000000..4cd1b2469
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/inventory-look_hidden.golden
@@ -0,0 +1,2 @@
+
+ Carrying: several objects
diff --git a/internal/guards/testdata/looking_screens_80/pet_captured.golden b/internal/guards/testdata/looking_screens_80/pet_captured.golden
new file mode 100644
index 000000000..2d88d0073
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/pet_captured.golden
@@ -0,0 +1,7 @@
+
+<ansi fg="black-bold">.:</ansi> <ansi fg="petname">Whiskerfield</ansi>
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Description</ansi> ────────────────────────────────────────────────────────────┐
+   <ansi fg="petname">Whiskerfield</ansi> is a pet <ansi fg="petname">cat</ansi> owned by <ansi fg="username">Screenly</ansi>.
+   <ansi fg="petname">Whiskerfield</ansi> hunger is: <ansi fg="hunger-dying">dying</ansi>
+ └────────────────────────────────────────────────────────────────────────────┘
+ Carrying: 
\ No newline at end of file
diff --git a/internal/guards/testdata/looking_screens_80/room-title_captured.golden b/internal/guards/testdata/looking_screens_80/room-title_captured.golden
new file mode 100644
index 000000000..8c7bc87a3
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/room-title_captured.golden
@@ -0,0 +1,2 @@
+
+<ansi fg="black-bold">.:</ansi> <ansi fg="black-bold">[</ansi><ansi fg="map-bank" bg="mapbg-bank">⌂</ansi><ansi fg="black-bold">]</ansi> <ansi fg="room-title">Thornwall Bank</ansi> <ansi fg="room-zone">[Thornwall City]</ansi>
\ No newline at end of file
diff --git a/internal/guards/testdata/looking_screens_80/room_alerts.golden b/internal/guards/testdata/looking_screens_80/room_alerts.golden
new file mode 100644
index 000000000..2a0a1dca4
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/room_alerts.golden
@@ -0,0 +1,10 @@
+<ansi fg="room-description">A solid stone building
+with iron-banded doors.</ansi>
+
+<ansi fg="red">────────────────────────────────────────────────────────────────────────────────</ansi>
+                 <ansi fg="yellow-bold">This is a bank!</ansi> Type <ansi fg="command">bank</ansi> to deposit/withdraw.
+<ansi fg="red">────────────────────────────────────────────────────────────────────────────────</ansi>
+
+<ansi fg="red">────────────────────────────────────────────────────────────────────────────────</ansi>
+        <ansi fg="yellow-bold">This is an item storage location!</ansi> Type <ansi fg="command">storage</ansi> to store/unstore.
+<ansi fg="red">────────────────────────────────────────────────────────────────────────────────</ansi>
diff --git a/internal/guards/testdata/looking_screens_80/room_tracking.golden b/internal/guards/testdata/looking_screens_80/room_tracking.golden
new file mode 100644
index 000000000..d52643c08
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/room_tracking.golden
@@ -0,0 +1,3 @@
+<ansi fg="room-description">A quiet room.</ansi>
+<ansi fg="182">Tracking <ansi fg="mobname">Riss the Cartographer-Scholar</ansi>... They went <ansi fg="exit">a shimmering violet portal</ansi></ansi>
+
diff --git a/internal/guards/testdata/looking_screens_80/rune_short.golden b/internal/guards/testdata/looking_screens_80/rune_short.golden
new file mode 100644
index 000000000..8aef74fdc
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/rune_short.golden
@@ -0,0 +1 @@
+           A rune reads: [<ansi fg="magenta-bold">Welcome</ansi>]
\ No newline at end of file
diff --git a/internal/guards/testdata/looking_screens_80/sign_short.golden b/internal/guards/testdata/looking_screens_80/sign_short.golden
new file mode 100644
index 000000000..52d9f9efb
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/sign_short.golden
@@ -0,0 +1 @@
+           A sign reads: [<ansi fg="green-bold">Welcome</ansi>]
\ No newline at end of file
diff --git a/internal/guards/testdata/looking_screens_80/track_captured.golden b/internal/guards/testdata/looking_screens_80/track_captured.golden
new file mode 100644
index 000000000..39ee0170e
--- /dev/null
+++ b/internal/guards/testdata/looking_screens_80/track_captured.golden
@@ -0,0 +1,3 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Recent Visitors</ansi> ─────────────────────────────────────────────────────┐
+   [<ansi fg="trail-warm">Warm</ansi>] <ansi fg="username">City Guard</ansi> 
+ └─────────────────────────────────────────────────────────────────────────┘
diff --git a/internal/rooms/context.md b/internal/rooms/context.md
index 4dbc50901..e2c577ce6 100644
--- a/internal/rooms/context.md
+++ b/internal/rooms/context.md
@@ -175,7 +175,11 @@ The `internal/rooms` package is the core world management system for GoMud, hand
 - **Dynamic content**: Visible players, mobs, corpses, and exits
 - **Environmental context**: Day/night cycles, lighting, biome effects
 - **User-specific views**: Personalized room information based on character state
-- **Room alerts**: Special notifications for banks, training, storage, etc.
+- **Room alerts**: Special notifications for banks, storage, character rooms,
+  the start room (`AlertBank`, `AlertStorage`, `AlertCharacterRoom`,
+  `AlertStart`) and each active mutator's `AlertModifier` text. They are bare
+  text, never padded: `descriptions/room` centres each between two rules as
+  wide as the reader's screen (`centerlines`, owner 2026-11-10).
 - **NPC talk marker** (`npcmarker.go`): `rooms.NpcMarker func(viewerUserId int,
   mob *mobs.Mob) string` is a hook installed at boot (`wireConversationHooks`
   in the root package, and by `internal/conversation`'s init). `GetDetails`
@@ -857,8 +861,10 @@ Comprehensive test coverage in `*_test.go` files covering:
 
 Every sender passes `UserRecord.WrapWidth()` as the pipeline's width.
 `GetDetails` wraps the description to the viewer's `WrapWidth()`, and to
-that less the minimap's 7 columns beside a minimap. `GetDescriptionFormatted`
-uses the width it is given.
+that less the minimap's 7 columns beside a minimap. A viewer 42 columns wide
+or narrower (`minimapAboveWidth`) gets the minimap above the prose instead,
+and the prose takes the whole width. `GetDescriptionFormatted` uses the width
+it is given.
 
 ## Special Considerations
 - **Memory management**: Rooms automatically unload when empty to conserve memory
diff --git a/internal/rooms/roomdetails.go b/internal/rooms/roomdetails.go
index af46a9e72..36cadabab 100644
--- a/internal/rooms/roomdetails.go
+++ b/internal/rooms/roomdetails.go
@@ -40,24 +40,21 @@ type RoomTemplateDetails struct {
 	ShowPvp        bool     // Whether to display that the room is PVP
 }
 
+// minimapAboveWidth is the widest reader whose minimap goes above the room's
+// prose instead of beside it: the widest narrow form a screen is drawn to
+// (templates.FormWidths), where the prose beside the map would be 34 columns.
+const minimapAboveWidth = 42
+
+// The room alerts, as GetDetails lists them. Each is bare text: the room
+// template centres it between two rules as wide as the reader's screen
+// (owner, 2026-11-10).
 const (
-	// alertCenterColumns is the widest span an alert centers in.
-	alertCenterColumns = 65
-	// alertTemplateMargin is the indent room.template prints before an alert,
-	// left and right of the span it centers in.
-	alertTemplateMargin = 6
+	AlertBank          = `<ansi fg="yellow-bold">This is a bank!</ansi> Type <ansi fg="command">bank</ansi> to deposit/withdraw.`
+	AlertStorage       = `<ansi fg="yellow-bold">This is an item storage location!</ansi> Type <ansi fg="command">storage</ansi> to store/unstore.`
+	AlertCharacterRoom = `<ansi fg="yellow-bold">This is a character room!</ansi> Type <ansi fg="command">character</ansi> to interact.`
+	AlertStart         = `<ansi fg="yellow-bold">Type <ansi fg="command">start</ansi> to begin playing.</ansi>`
 )
 
-// centerAlert pads text to center it for a reader width columns wide. Text
-// with no room to pad comes back as it went in.
-func centerAlert(text string, width int) string {
-	span := min(alertCenterColumns, width-2*alertTemplateMargin)
-	if len(text) >= span {
-		return text
-	}
-	return strings.Repeat(` `, (span-len(text))>>1) + text
-}
-
 func GetDetails(r *Room, user *users.UserRecord, tinymap ...[]string) RoomTemplateDetails {
 
 	c := configs.GetGamePlayConfig()
@@ -96,19 +93,19 @@ func GetDetails(r *Room, user *users.UserRecord, tinymap ...[]string) RoomTempla
 	//
 
 	if r.IsBank {
-		details.RoomAlerts = append(details.RoomAlerts, `          <ansi fg="yellow-bold">This is a bank!</ansi> Type <ansi fg="command">bank</ansi> to deposit/withdraw.`)
+		details.RoomAlerts = append(details.RoomAlerts, AlertBank)
 	}
 
 	if r.IsStorage {
-		details.RoomAlerts = append(details.RoomAlerts, ` <ansi fg="yellow-bold">This is an item storage location!</ansi> Type <ansi fg="command">storage</ansi> to store/unstore.`)
+		details.RoomAlerts = append(details.RoomAlerts, AlertStorage)
 	}
 
 	if r.IsCharacterRoom {
-		details.RoomAlerts = append(details.RoomAlerts, `      <ansi fg="yellow-bold">This is a character room!</ansi> Type <ansi fg="command">character</ansi> to interact.`)
+		details.RoomAlerts = append(details.RoomAlerts, AlertCharacterRoom)
 	}
 
 	if r.RoomId == -1 {
-		details.RoomAlerts = append(details.RoomAlerts, `      <ansi fg="yellow-bold">Type <ansi fg="command">start</ansi> to begin playing.</ansi>`)
+		details.RoomAlerts = append(details.RoomAlerts, AlertStart)
 	}
 
 	//
@@ -131,7 +128,15 @@ func GetDetails(r *Room, user *users.UserRecord, tinymap ...[]string) RoomTempla
 	// wrap, so a long modifier wraps with the rest (#455).
 	descParts := descriptionWithModifiers(r, details.Description)
 
-	if len(tinymap) > 0 {
+	if len(tinymap) > 0 && user.WrapWidth() <= minimapAboveWidth {
+		// A phone-width screen has no room for the map beside the prose
+		// (owner, 2026-11-10): the map goes above it, at the left edge, and
+		// the prose takes the full width. Only telnet reaches here; the web
+		// client never draws the minimap (look.go).
+		lines := append([]string{}, tinymap[0]...)
+		lines = append(lines, wrapDescriptionParts(descParts, user.WrapWidth(), nouns)...)
+		details.Description = strings.Join(lines, "\n")
+	} else if len(tinymap) > 0 {
 		desclineWidth := user.WrapWidth() - 7 // 7 is the width of the tinymap
 		padding := 1
 		description := wrapDescriptionParts(descParts, desclineWidth-padding, nouns)
@@ -192,9 +197,8 @@ func GetDetails(r *Room, user *users.UserRecord, tinymap ...[]string) RoomTempla
 		// No current plans to allow them to overwrite existing alerts.
 		if mutSpec.AlertModifier != nil {
 
-			alertText := centerAlert(mutSpec.AlertModifier.Text, user.WrapWidth())
-
-			details.RoomAlerts = append(details.RoomAlerts, colorpatterns.ApplyColorPattern(alertText, mutSpec.AlertModifier.ColorPattern))
+			// Bare text; the room template centres it (see AlertBank).
+			details.RoomAlerts = append(details.RoomAlerts, colorpatterns.ApplyColorPattern(mutSpec.AlertModifier.Text, mutSpec.AlertModifier.ColorPattern))
 
 		}
 	}
diff --git a/internal/rooms/roomdetails_modifier_wrap_test.go b/internal/rooms/roomdetails_modifier_wrap_test.go
index d00e300b8..f68384f12 100644
--- a/internal/rooms/roomdetails_modifier_wrap_test.go
+++ b/internal/rooms/roomdetails_modifier_wrap_test.go
@@ -32,48 +32,62 @@ func modifierWrapRoom(t *testing.T) (*Room, *users.UserRecord) {
 	return roomManager.rooms[1], users.GetByUserId(7491)
 }
 
-// A mutator's alert is centered for the reader. The widths at the top of the
-// range keep the 65-column centering they always had; below that the alert
-// centers between the room template's 6-column margins.
-func TestGetDetails_AlertCentersToTheReadersWidth(t *testing.T) {
-	const short = "Quake now!"          // 10 columns
-	const fits = "The floors are dusty" // 20 columns
-	for _, tc := range []struct {
-		name    string
-		width   int
-		alert   string
-		padding int
-	}{
-		{"short at 80 is unchanged", 80, short, 27},
-		{"longer at 80 is unchanged", 80, fits, 22},
-		{"short at 77 is unchanged", 77, short, 27},
-		{"short at 60", 60, short, 19},
-		{"short at 30", 30, short, 4},
-		{"longer at 30 has no room to pad", 30, fits, 0},
-	} {
-		t.Run(tc.name, func(t *testing.T) {
-			t.Cleanup(seedRegistry())
-			t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
-				MutatorId:     "test-wrap-alert",
-				AlertModifier: &mutators.TextModifier{Text: tc.alert},
-			}))
-			viewer := users.NewTestUser(7492, "alerted", "Alerted", 97492)
-			viewer.LineWidth = tc.width
-			t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7492: viewer}))
-			GetZoneConfig("TestZone").Mutators.Add("test-wrap-alert")
+// Every alert reaches the room template as bare text at every reader width:
+// the template centres it between two rules as wide as the reader's screen
+// (owner, 2026-11-10), so no width is baked in here.
+func TestGetDetails_AlertsAreBareText(t *testing.T) {
+	const quake = "Quake now! The floors are dusty."
+	for _, width := range []int{30, 42, 80} {
+		t.Cleanup(seedRegistry())
+		t.Cleanup(mutators.SeedSpecsForTest(mutators.MutatorSpec{
+			MutatorId:     "test-wrap-alert",
+			AlertModifier: &mutators.TextModifier{Text: quake},
+		}))
+		viewer := users.NewTestUser(7492, "alerted", "Alerted", 97492)
+		viewer.LineWidth = width
+		t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{7492: viewer}))
+		GetZoneConfig("TestZone").Mutators.Add("test-wrap-alert")
+		r := roomManager.rooms[1]
+		r.IsBank, r.IsStorage = true, true
 
-			alerts := GetDetails(roomManager.rooms[1], viewer).RoomAlerts
-			if len(alerts) != 1 {
-				t.Fatalf("want the one alert, got %q", alerts)
-			}
-			want := strings.Repeat(" ", tc.padding) + tc.alert
-			if got := alerts[0]; got != want {
-				t.Fatalf("alert %q, want %q", got, want)
-			}
-			if w := 6 + util.VisibleWidth(alerts[0]); w > tc.width {
-				t.Errorf("the alert ends at column %d, past the reader's %d", w, tc.width)
-			}
-		})
+		alerts := GetDetails(r, viewer).RoomAlerts
+		want := []string{AlertBank, AlertStorage, quake}
+		if strings.Join(alerts, "|") != strings.Join(want, "|") {
+			t.Fatalf("at %d the alerts are %q, want %q", width, alerts, want)
+		}
+		r.IsBank, r.IsStorage = false, false
+	}
+}
+
+// On a screen 42 columns wide or narrower the minimap goes above the prose,
+// which then takes the whole width (owner, 2026-11-10); wider, it stays
+// beside the prose (TestGetDetails_MinimapSitsAtTheViewersRightEdge).
+func TestGetDetails_MinimapGoesAboveTheProseOnANarrowScreen(t *testing.T) {
+	r, viewer := modifierWrapRoom(t)
+	tinymap := []string{"╔═════╗", "║.....║", "║.....║", "║..@..║", "║.....║", "║.....║", "╚═════╝"}
+	for _, width := range []int{30, 42} {
+		viewer.LineWidth = width
+		lines := strings.Split(GetDetails(r, viewer, tinymap).Description, "\n")
+		if len(lines) <= len(tinymap) || strings.Join(lines[:len(tinymap)], "\n") != strings.Join(tinymap, "\n") {
+			t.Fatalf("at %d the map is not above the prose:\n%s", width, strings.Join(lines, "\n"))
+		}
+		if !strings.Contains(strings.Join(lines[len(tinymap):], " "), "A peace older") {
+			t.Errorf("at %d the prose is missing:\n%s", width, strings.Join(lines, "\n"))
+		}
+		widest := 0
+		for _, line := range lines {
+			widest = max(widest, util.VisibleWidth(line))
+		}
+		if widest > width || widest < width-12 {
+			t.Errorf("at %d the prose does not take the width: widest line %d", width, widest)
+		}
+	}
+	for _, width := range []int{43, 80} {
+		viewer.LineWidth = width
+		lines := strings.Split(GetDetails(r, viewer, tinymap).Description, "\n")
+		if !strings.HasSuffix(lines[0], tinymap[0]) || lines[0] == tinymap[0] {
+			t.Errorf("at %d the map is no longer beside the prose: %q", width, lines[0])
+		}
 	}
 }
 
diff --git a/internal/templates/centerlines_test.go b/internal/templates/centerlines_test.go
new file mode 100644
index 000000000..39b41a677
--- /dev/null
+++ b/internal/templates/centerlines_test.go
@@ -0,0 +1,43 @@
+package templates
+
+import (
+	"strings"
+	"testing"
+
+	"github.com/GoMudEngine/GoMud/internal/util"
+)
+
+// centerlines centres an alert between the room view's rules (plan 3b-2):
+// whole when it fits inside a 2-column margin, else a piece (a sentence, or
+// text set apart by a run of spaces) to a line inside a 1-column margin,
+// else wrapped; every line padded on the left only.
+func TestCenterLines(t *testing.T) {
+	const bank = `<ansi fg="yellow-bold">This is a bank!</ansi> Type <ansi fg="command">bank</ansi> to deposit/withdraw.`
+	for _, tc := range []struct {
+		name, in string
+		width    int
+		want     []string
+	}{
+		{"fits on one line", bank, 80, []string{"                 This is a bank! Type bank to deposit/withdraw."}},
+		{"a sentence to a line", bank, 42, []string{"             This is a bank!", "      Type bank to deposit/withdraw."}},
+		{"a sentence too long for the span wraps the whole", bank, 30, []string{"  This is a bank! Type bank", "     to deposit/withdraw."}},
+		{"runs of spaces after a sentence go with the break", "!!!      Fire here!      !!!", 20, []string{"        !!!", "     Fire here!", "        !!!"}},
+		{"leading padding is not counted", "      Quake now!", 30, []string{"          Quake now!"}},
+	} {
+		t.Run(tc.name, func(t *testing.T) {
+			out := centerLines(tc.in, tc.width)
+			plain := strings.Split(util.StripANSI(TplAnsiParse(out)), "\n")
+			if strings.Join(plain, "|") != strings.Join(tc.want, "|") {
+				t.Errorf("got\n%q\nwant\n%q", plain, tc.want)
+			}
+			for _, line := range strings.Split(out, "\n") {
+				if w := util.VisibleWidth(line); w > tc.width-2 {
+					t.Errorf("line %q is %d wide, past the 1-column margin of %d", line, w, tc.width)
+				}
+				if strings.Count(line, "<ansi") != strings.Count(line, "</ansi>") {
+					t.Errorf("line %q does not close the colours it opens", line)
+				}
+			}
+		})
+	}
+}
diff --git a/internal/templates/context.md b/internal/templates/context.md
index cd9786d39..d14735064 100644
--- a/internal/templates/context.md
+++ b/internal/templates/context.md
@@ -156,11 +156,41 @@ Cached template with metadata for efficient reuse and cache invalidation.
 - `visiblewidth s` is the columns s takes, colour tags not counted, so a form
   can test whether a coloured value (a `vitalQuality` word, a `NameComplex`)
   fits beside its label.
+- `centerlines text width` centres text on a screen width columns wide:
+  whole when it fits with 2 columns clear each side, else one piece to a
+  line when every piece fits with 1 column clear, else wrapped with
+  `messaging.WrapAnsi` with 2 clear. A piece ends at a sentence end or at a
+  run of two or more spaces (`respace`, which steps over colour tags, so a
+  per-letter colour pattern such as the wildfire's flame reads by its
+  letters); wrapped, a run of spaces is one space. Lines are padded on the
+  left only. The room view's alerts use it between two `screenwidth` rules.
+- `wrapspans text width prefix` wraps as `wraptext` does but keeps each
+  top-level colour span (a name, an exit), with the text glued to it
+  ("Name..."), on one line unless that unit is wider than width. The room
+  view's tracking line uses it.
+- `wrapbreak text width prefix` wraps as `wraptext` does but first breaks a
+  word longer than width into width-wide pieces. Signs and runes use it,
+  since players write them. `visibleRunes` and `rebuild` are the shared
+  walk over a string's visible runes behind `respace`, `wrapspans` and
+  `wrapbreak`.
 - The character screens' forms (`character/status`, `inventory`, `skills`,
   `conditions`, `cooldowns`, `quests`, `storage`, `title`, each `.w42` and
   `.w30`) are drawn to their own width, not to `screenwidth`: a 42 form drawn
   to a 35-column reader would fit and hide the 30 form. Shared layout
-  pieces live in `{{ define }}` blocks inside each form file.
+  pieces live in `{{ define }}` blocks inside each form file. So are the
+  looking-at-things forms (plan 3b-2): `character/description`,
+  `description-corpse`, `inventory-look`, `pet`, and `descriptions/biome`,
+  `identify`, `insidecontainer`, `track`.
+- The room view's own templates (`descriptions/room`, `room-title`, `exits`,
+  `fixtures`, `rune`, `sign`) have no forms: `rooms.GetDetails` wraps the
+  room's prose to the reader's own width, so a form drawn to 42 would not
+  fit a 60-column reader's prose. They draw to `screenwidth` instead, and
+  wrap a line only when it is wider than the screen, so a line that fits
+  keeps the bytes it always had. An exit and its tag (`northeast (locked)`)
+  and the title's PvP tag are units, never split while they fit a line.
+- `descriptions/biome` draws its box as two 80-wide rules at every width
+  (owner-approved mock, 2026-11-10), with its lighting wrapped under its
+  value.
 - A `.template` may have narrow forms beside it, named for the width they are
   drawn to: `tables/generic.w40.template`, `tables/generic.w30.template`. Only
   widths in `FormWidths` (42, 40, 30) are read. When the base output's widest
diff --git a/internal/templates/templatesfunctions.go b/internal/templates/templatesfunctions.go
index d372322b7..cb75e1396 100644
--- a/internal/templates/templatesfunctions.go
+++ b/internal/templates/templatesfunctions.go
@@ -3,10 +3,12 @@ package templates
 import (
 	"fmt"
 	"math"
+	"regexp"
 	"strconv"
 	"strings"
 	"text/template"
 	"time"
+	"unicode/utf8"
 
 	"github.com/GoMudEngine/GoMud/internal/characters"
 	"github.com/GoMudEngine/GoMud/internal/colorpatterns"
@@ -138,6 +140,9 @@ var (
 		// counted, so a narrow form can ask whether a coloured value fits.
 		"visiblewidth": util.VisibleWidth,
 		"wraptext":     wrapText,
+		"centerlines":  centerLines,
+		"wrapspans":    wrapSpans,
+		"wrapbreak":    wrapBreak,
 		"ansiparse":    TplAnsiParse,
 		"conditionname": func(conditionId int) string {
 			conditionSpec := conditions.GetConditionSpec(conditionId)
@@ -477,6 +482,200 @@ func wrapText(s string, width int, prefix ...string) string {
 	return strings.ReplaceAll(messaging.WrapAnsi(s, width), "\n", "\n"+pad)
 }
 
+// wrapSpans wraps as wrapText does, but keeps each top-level colour span (a
+// name, an exit) on one line, together with any text glued to it ("Name..."),
+// unless that unit is wider than width; then the span's words wrap as usual.
+// A name is never split while it fits a line by itself (owner, 2026-11-10).
+//
+//	{{ wrapspans .TrackingString screenwidth }}
+func wrapSpans(s string, width int, prefix ...string) string {
+	const keep = "\x1f" // one column to WrapAnsi, and never a break
+	runes := visibleRunes(s)
+	what := make([]string, len(runes))
+	for i, v := range runes {
+		what[i] = s[v.start:v.end]
+	}
+	depth, open := 0, 0
+	for _, t := range ansiTag.FindAllStringIndex(s, -1) {
+		if !strings.HasPrefix(s[t[0]:t[1]], "</") {
+			if depth == 0 {
+				open = t[1]
+			}
+			depth++
+			continue
+		}
+		if depth--; depth != 0 {
+			continue
+		}
+		// The span's visible runes, then the unit: the span widened over
+		// whatever is glued to it up to the nearest space each side.
+		first, last := -1, -1
+		for i, v := range runes {
+			if v.start >= open && v.end <= t[0] {
+				if first < 0 {
+					first = i
+				}
+				last = i
+			}
+		}
+		if first < 0 {
+			continue
+		}
+		from, to := first, last
+		for from > 0 && runes[from-1].r != ' ' {
+			from--
+		}
+		for to < len(runes)-1 && runes[to+1].r != ' ' {
+			to++
+		}
+		unit := 0
+		for _, v := range runes[from : to+1] {
+			unit += util.VisibleWidth(string(v.r))
+		}
+		if unit > width {
+			continue
+		}
+		for i := first; i <= last; i++ {
+			if runes[i].r == ' ' {
+				what[i] = keep
+			}
+		}
+	}
+	return strings.ReplaceAll(wrapText(rebuild(s, runes, what), width, prefix...), keep, " ")
+}
+
+// wrapBreak wraps as wrapText does, but first breaks any word longer than
+// width into pieces width wide, so no line runs past: a word is split only
+// when it is longer than a whole line. Signs and runes use it, since a
+// player writes them.
+//
+//	{{ wrapbreak $text (sub screenwidth 2) "  " }}
+func wrapBreak(s string, width int, prefix ...string) string {
+	runes := visibleRunes(s)
+	what := make([]string, len(runes))
+	cols := 0
+	for i, v := range runes {
+		what[i] = s[v.start:v.end]
+		if v.r == ' ' {
+			cols = 0
+			continue
+		}
+		w := util.VisibleWidth(what[i])
+		if cols+w > width && cols > 0 {
+			// Only inside a word wider than a line: such a word is the only
+			// one that ever fills a whole line by itself.
+			what[i] = " " + what[i]
+			cols = 0
+		}
+		cols += w
+	}
+	return wrapText(rebuild(s, runes, what), width, prefix...)
+}
+
+// visibleRune is one rune of s that shows on screen, and its bytes in s.
+type visibleRune struct {
+	r          rune
+	start, end int
+}
+
+// visibleRunes lists the runes of s outside its colour tags.
+func visibleRunes(s string) []visibleRune {
+	tags := ansiTag.FindAllStringIndex(s, -1)
+	runes := []visibleRune{}
+	for i, t := 0, 0; i < len(s); {
+		if t < len(tags) && i == tags[t][0] {
+			i = tags[t][1]
+			t++
+			continue
+		}
+		r, size := utf8.DecodeRuneInString(s[i:])
+		runes = append(runes, visibleRune{r, i, i + size})
+		i += size
+	}
+	return runes
+}
+
+// rebuild is s with each visible rune i replaced by what[i], its colour tags
+// kept where they were.
+func rebuild(s string, runes []visibleRune, what []string) string {
+	var b strings.Builder
+	at := 0
+	for i, v := range runes {
+		b.WriteString(s[at:v.start])
+		b.WriteString(what[i])
+		at = v.end
+	}
+	b.WriteString(s[at:])
+	return b.String()
+}
+
+// centerLines centres s on a screen width columns wide: on one line when it
+// fits with 2 columns clear each side; else one piece to a line when every
+// piece fits with 1 column clear each side; else wrapped with
+// messaging.WrapAnsi with 2 columns clear. A piece ends at a sentence end or
+// at a run of two or more spaces, the author's own break (the wildfire
+// alert spaces "!!!", its sentence and "!!!" apart); wrapped, a run is one
+// space. Each line is padded on the left only. The room view's alerts use it
+// (owner, 2026-11-10: an alert is its text centred between two full-width
+// rules).
+//
+//	{{ centerlines $alertStr screenwidth }}
+func centerLines(s string, width int) string {
+	s = strings.TrimSpace(s)
+	span := width - 4
+	text, wrapAt := s, span
+	if util.VisibleWidth(s) > span {
+		text, wrapAt = respace(s, true), width-2
+		for _, line := range strings.Split(text, "\n") {
+			if util.VisibleWidth(line) > width-2 {
+				text, wrapAt = respace(s, false), span
+				break
+			}
+		}
+	}
+	lines := []string{}
+	for _, line := range strings.Split(messaging.WrapAnsi(text, wrapAt), "\n") {
+		w := util.VisibleWidth(line)
+		if w == 0 {
+			continue
+		}
+		lines = append(lines, strings.Repeat(" ", max(0, (width-w)/2))+line)
+	}
+	return strings.Join(lines, "\n")
+}
+
+// ansiTag matches one colour tag, opening or closing.
+var ansiTag = regexp.MustCompile(`</?ansi[^>]*>`)
+
+// respace rewrites each run of spaces in s as one space, or, when breaks is
+// set, as a line break where the run ends a sentence (after ".", "!" or
+// "?") or is two or more spaces long. Colour tags are passed over and kept,
+// so a colour pattern that tags every letter and space (the wildfire's
+// flame) is read by its letters; WrapAnsi then closes and reopens a colour
+// that spans a break.
+func respace(s string, breaks bool) string {
+	runes := visibleRunes(s)
+	// what[i] is what visible rune i becomes: itself, "" (dropped), " " or "\n".
+	what := make([]string, len(runes))
+	for i := 0; i < len(runes); i++ {
+		if runes[i].r != ' ' {
+			what[i] = s[runes[i].start:runes[i].end]
+			continue
+		}
+		j := i
+		for j < len(runes) && runes[j].r == ' ' {
+			what[j] = ""
+			j++
+		}
+		what[i] = " "
+		if breaks && (j-i >= 2 || (i > 0 && strings.ContainsRune(".!?", runes[i-1].r))) {
+			what[i] = "\n"
+		}
+		i = j - 1
+	}
+	return rebuild(s, runes, what)
+}
+
 // Usage:
 //
 //	{{ implode .items "," }}
diff --git a/internal/usercommands/context.md b/internal/usercommands/context.md
index 37d10144a..879ccfe7f 100644
--- a/internal/usercommands/context.md
+++ b/internal/usercommands/context.md
@@ -494,7 +494,12 @@ own line or lines with a full-width rule under it. `narrow_forms_test.go` render
 and companion at 30, 42 and 80 and pins their 80-column output to goldens
 recorded from master (`testdata/narrow_forms_80/`, `-update-narrow-forms`).
 The other character screens are held in `internal/guards`
-(`character_screens_test.go`).
+(`character_screens_test.go`), as are the looking-at-things screens of plan
+3b-2 (`looking_screens_test.go`). `room_view_forms_test.go` runs `lookRoom`
+for a telnet reader at 30, 42 and 80 columns: the minimap goes above the
+prose at 42 and below, each alert sits between two rules the full screen
+width, and the 80-column view is pinned (`room_view.golden`, recorded on the
+branch because the owner ruled the alerts' change at 80).
 
 ## Message categories (messaging M7 slice 4, #371)
 
diff --git a/internal/usercommands/room_view_forms_test.go b/internal/usercommands/room_view_forms_test.go
new file mode 100644
index 000000000..828d1a3f2
--- /dev/null
+++ b/internal/usercommands/room_view_forms_test.go
@@ -0,0 +1,80 @@
+package usercommands
+
+import (
+	"strings"
+	"testing"
+
+	"github.com/GoMudEngine/GoMud/internal/events"
+	"github.com/GoMudEngine/GoMud/internal/templates"
+	"github.com/stretchr/testify/require"
+)
+
+// The room view as look draws it (plan 3b-2): the title, the prose and the
+// minimap rooms.GetDetails lays out, the alerts, the roster and the exits,
+// for a telnet reader at 30, 42 and 80 columns. On a screen 42 columns wide
+// or narrower the minimap goes above the prose (wider, it stays beside it:
+// rooms.minimapAboveWidth); at every width each alert sits between
+// two rules as wide as the screen (owner, 2026-11-10).
+
+// thornwallBankDescription is the bank's prose as the 2026-11-10 capture
+// showed it.
+const thornwallBankDescription = "A solid stone building with iron-banded doors and narrow windows. Inside, a long counter of dark wood separates the public lobby from the vault area behind. Heavy ledgers line the shelves, and the faint clink of counted coins drifts from a back room. The air smells of ink, old paper, and the metallic tang of gold. A large iron strongbox sits against the far wall, its lock the size of a fist."
+
+// roomView runs look on user 1's room, made the Thornwall Bank with both
+// alerts, at a reader width, and returns each send.
+func roomView(t *testing.T, width int) []string {
+	t.Helper()
+	user, room := getTestUserAndRoom(t)
+	prev := *room
+	prevWidth := user.LineWidth
+	t.Cleanup(func() {
+		room.Title, room.Zone, room.Description = prev.Title, prev.Zone, prev.Description
+		room.IsBank, room.IsStorage = prev.IsBank, prev.IsStorage
+		user.LineWidth = prevWidth
+	})
+	room.Title, room.Zone, room.Description = "Thornwall Bank", "Thornwall City", thornwallBankDescription
+	room.IsBank, room.IsStorage = true, true
+	user.LineWidth = width
+
+	events.DrainQueuedMessagesForTest(user.UserId)
+	lookRoom(user, room.RoomId, true)
+	return events.DrainQueuedMessagesForTest(user.UserId)
+}
+
+func TestRoomViewFitsEveryWidth(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	useDogmudTemplates(t)
+	for _, width := range narrowWidths {
+		out := strings.Join(roomView(t, width), "\n")
+		require.Contains(t, out, "This is a bank!", "at %d", width)
+		require.Contains(t, out, "╔═════╗", "a telnet reader keeps the minimap at %d", width)
+		require.LessOrEqual(t, templates.WidestLine(out), width, "room view at %d columns:\n%s", width, out)
+
+		lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
+		rules := 0
+		for i, line := range lines {
+			if strings.Contains(line, "─") {
+				rules++
+				require.Equal(t, width, templates.WidestLine(line), "alert rule %q at %d", line, width)
+			}
+			if strings.Contains(line, "╔═════╗") {
+				// Beside the prose at 80, alone above it at 42 and 30.
+				require.Equal(t, width > 42, templates.WidestLine(line) > 7, "minimap top at %d: %q", width, line)
+				if width <= 42 {
+					require.Contains(t, strings.Join(lines[i+7:], " "), "A solid stone building", "the prose follows the map at %d", width)
+				}
+			}
+		}
+		require.Equal(t, 4, rules, "two alerts, two rules each, at %d", width)
+	}
+}
+
+// The 80-column room view is pinned: unchanged from master but for the
+// alerts, which the owner ruled become full-width rules at every width.
+func TestRoomViewKeepsIts80ColumnOutput(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	useDogmudTemplates(t)
+	checkGolden(t, "room_view", strings.Join(roomView(t, 80), "\n"))
+}
diff --git a/internal/usercommands/template_freeze_test.go b/internal/usercommands/template_freeze_test.go
index 05eb52381..75158b72f 100644
--- a/internal/usercommands/template_freeze_test.go
+++ b/internal/usercommands/template_freeze_test.go
@@ -306,6 +306,9 @@ func TestTemplateFreeze_BiomeSaysWhenLampsBurn(t *testing.T) {
 	for _, c := range cases {
 		out, err := templates.Process("descriptions/biome", &c.biome, 0)
 		require.NoError(t, err, c.name)
+		// The lighting line wraps under its value (plan 3b-2), so the
+		// words are compared with the line breaks taken out.
+		out = strings.Join(strings.Fields(out), " ")
 		assert.Contains(t, out, c.want, c.name)
 		assert.NotContains(t, out, c.deny, c.name)
 	}
diff --git a/internal/usercommands/testdata/narrow_forms_80/room_view.golden b/internal/usercommands/testdata/narrow_forms_80/room_view.golden
new file mode 100644
index 000000000..db3cac1cb
--- /dev/null
+++ b/internal/usercommands/testdata/narrow_forms_80/room_view.golden
@@ -0,0 +1,22 @@
+<ansi fg="room-description">
+<ansi fg="black-bold">.:</ansi> <ansi fg="black-bold">[</ansi><ansi fg="map-city" bg="mapbg-city">#</ansi><ansi fg="black-bold">]</ansi> <ansi fg="room-title">Thornwall Bank</ansi> <ansi fg="room-zone">[Thornwall City]</ansi></ansi>
+
+<ansi fg="room-description"><ansi fg="room-description-dark">A solid stone building with iron-banded doors and narrow windows.        ╔═════╗
+Inside, a long counter of dark wood separates the public lobby from the  ║  <ansi fg="map-room"><ansi fg="map-default" bg="mapbg-default">.</ansi></ansi>  ║
+vault area behind. Heavy ledgers line the shelves, and the faint clink   ║  │  ║
+of counted coins drifts from a back room. The air smells of ink, old     ║  <ansi fg="map-room"><ansi fg="map-you" bg="mapbg-you">@</ansi></ansi>  ║
+paper, and the metallic tang of gold. A large iron strongbox sits        ║     ║
+against the far wall, its lock the size of a fist.                       ║     ║
+                                                                         ╚═════╝</ansi>
+
+<ansi fg="red">────────────────────────────────────────────────────────────────────────────────</ansi>
+                 <ansi fg="yellow-bold">This is a bank!</ansi> Type <ansi fg="command">bank</ansi> to deposit/withdraw.
+<ansi fg="red">────────────────────────────────────────────────────────────────────────────────</ansi>
+
+<ansi fg="red">────────────────────────────────────────────────────────────────────────────────</ansi>
+        <ansi fg="yellow-bold">This is an item storage location!</ansi> Type <ansi fg="command">storage</ansi> to store/unstore.
+<ansi fg="red">────────────────────────────────────────────────────────────────────────────────</ansi></ansi>
+
+<ansi fg="room-description"><ansi fg="room-description-dark">Also here: </ansi><ansi fg="username">Bobrick</ansi> <ansi fg="black-bold">(100%)</ansi> and <ansi fg="mobname"><ansi fg="mobname">Skeleton</ansi> <ansi fg="black-bold">(50%)</ansi></ansi></ansi>
+
+<ansi fg="room-description"><ansi fg="yellow">Exits: </ansi><ansi fg="exit">north</ansi></ansi>
````
