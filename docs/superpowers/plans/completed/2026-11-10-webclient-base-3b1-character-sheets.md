# Web Client Base, Plan 3b-1: Character Sheets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the character sheets their approved 42- and 30-column forms: status, inventory, skills, conditions, cooldowns, quests, storage, title and companion. The 80-column output stays byte for byte as it is.

**Architecture:** Each template gets `name.w42.template` and `name.w30.template` beside it, which plan 3a's form picking uses when the reader is narrower than the base. Two template functions help: `visiblewidth` measures a value without its colour tags, and `wraptext` wraps with `messaging.WrapAnsi`, the game's one wrapper. The companion screen is built in Go and picks its form the same way. Golden files recorded from master pin every 80-column screen.

**Tech Stack:** Go 1.25 `text/template`, `testify`, guards in `internal/guards`.

**Spec:** `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md`, Design section 2, rulings 3 and 22 to 25. The approved mock, with the owner's rulings of 2026-11-10: https://claude.ai/artifact/4vUBgswyRd3umYPrBmyHvZ (source `_archive/notes/reports/narrow-screens-3b-mock/` in the main checkout). Plan 3b is three plans; this is the first (owner, 2026-11-10). Part of #783.

**Needs:** plan 3a (merged in #843).

---

## Facts verified against source

Read 2026-11-10 from master at `af14f392b`. The patch at the end of this plan was built in a worktree of that commit by a subagent and checked again by the controller. The full suite (`go test . ./internal/... ./modules/...`) had no failing package, every `tools/jstest` and `tools/webclient-tests` file passed, `gofmt` was clean and `golangci-lint --new-from-merge-base=origin/master` reported 0 issues. The patch applied to a fresh worktree of `af14f392b`, and each task's red and green steps below were run there with the results written as they printed. Every 80-column golden was recorded from master's own code, in a worktree carrying only the tests and two type moves, and matched the branch byte for byte. A deliberately broken form failed the guard in both packages.

| # | Fact | Where |
|---|---|---|
| 1 | Plan 3a picks the widest form that fits, and only reads forms for a template that has them; `screenwidth` gives the reader's width, capped at 80 | `internal/templates/forms.go`; `templates.go` `process` |
| 2 | The approved drawings: `forms/<screen>.w42.txt` and `.w30.txt`, beside today's `final/<screen>.txt` | the mock folder |
| 3 | Rendered with each capture's own data, the new forms equal the drawings for inventory, skills, conditions, cooldowns, quests, storage, title and companion at both widths; status differs only where the owner ruled after the mock | subagent diff, 2026-11-10 |
| 4 | `util.SplitString` leaves an opening bracket at a line's end ("Missing Person (" then "repeatable)"); changing it would move line breaks at 80 elsewhere, so the forms wrap with `messaging.WrapAnsi` through `wraptext` | `internal/util/util.go:235`; `internal/messaging/wrap.go` |
| 5 | The quests and title screens passed types local to their functions; they become package types (`questsScreen`, `questRecord`, `titleScreen`, `statBar`) so a test can build them. The 20-cell quest bar needs the stored completion, because rescaling the 25-cell bar rounds differently (25% is 6 of 25 cells, 5 of 20) | `internal/usercommands/quests.go:33-60`; `title.go:18` |
| 6 | Conditions, quests and title pass types private to `usercommands`, so their fit tests live in that package (`narrow_forms_test.go`) and the guard names them in a "fits in usercommands" class | `internal/guards/template_width_guard_test.go` |
| 7 | The live-render guard's character has no species, so its status shows the 14-letter fallback species and the 80 line runs past 80; under plan 3a's ruling a screen wider than 80 takes a narrower form, so that golden is re-recorded showing the 42 form | `internal/guards/testdata/wrap_live_render.golden` |
| 8 | The configured name limit is 32 letters and a name is never split, so a name of 27 to 32 letters is wider than a 30-column screen in the status title, as a long link is | `_datafiles/config.yaml` validation |

## Choices made while building, for your review

1. **A screen already wider than 80 takes a form on a desktop too** (plan 3a's ruling, owner 2026-11-10). With the game's shipped data, the only desktop screen this changes in 3b-1 is the companion detail view with a long mutation list (86 columns today). Status with the fallback species and a long repeatable quest would also take a form, but no player can reach either with shipped data: human is the only selectable species, and no repeatable quest has a long enough name.
2. **Forms are drawn to fixed widths (42 and 30).** A 60-column reader sees the 42 form.
3. **Status at 42 is two columns** when every left value is 6 letters or fewer; otherwise that section goes one pair per line.
4. **Status Vitals pairs each pool with its reserve at 42** (owner). At 30 it keeps the drawn order, pools then reserves, since the ruling named 42.
5. **The companion screens were not in the mock.** They are designed by analogy with status. A heading is one rule line at the full width when the name fits beside it; when it does not, the name sits on its own line or lines with a full-width rule under it, so no text runs past a rule. The detail view keeps today's three-across rows at 80. At 42 it goes one pair per line, with the mutation list hanging under its first entry as today; at 30 the list moves under its label:

```
42                                          30
━━━ Steppe Spirit Wolf [summoned] ━━━━━━━━  Steppe Spirit Wolf [summoned]
  Health:      pristine                     ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  ...                                         Health:      pristine
  Mutations:   Commanding Presence (L3),      ...
               Rending Claws,                 Mutations:
               Sustaining Presence (L4),        Commanding Presence (L3),
               Tail                             Rending Claws,
  Auto-Assist: on                               Sustaining Presence (L4),
                                                Tail
Fenwickerly the Ashen Hound [bonded]          Auto-Assist: on
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

6. **The companion list at 80 keeps today's output exactly,** including the pipeline wrapping a long Mutations line without its indent; fixing that would change the 80 output.
7. **A name of 27 to 32 letters runs past a 30-column status screen,** on the name line only. A name is never split and the name limit is 32, so a 32-letter name makes that line 36 wide, with the title moved to the next line. The test allows exactly that one line. This is the same exception you accepted for long links in the MOTD. The alternative is lowering the name limit to 26, which touches existing characters.
8. **Status Surrender policy and the broken-limb line continue two spaces in at 42 and 30,** per your prose ruling. The approved drawing hung "health" under the value; this one line differs from it. (Approved; then Task 4b hides the Surrender line at every width until #848 is resolved, owner 2026-11-10.)

## Files

| File | Change |
|---|---|
| Create `_datafiles/world/dogmud/templates/character/{status,inventory,skills,conditions,cooldowns,quests,storage,title}.{w42,w30}.template` | 16 forms |
| Modify `internal/templates/templatesfunctions.go`, `internal/templates/context.md` | `visiblewidth`, `wraptext` |
| Modify `internal/usercommands/quests.go`, `title.go` | Package types; the quest bar at any cell count |
| Modify `internal/usercommands/companion.go` | List and detail forms at 42 and 30 |
| Create `internal/usercommands/narrow_forms_test.go`, `testdata/narrow_forms_80/*.golden` | Fit and 80-column tests: conditions, quests, title, companion |
| Create `internal/guards/character_screens_test.go`, `testdata/character_screens_80/*.golden` | Fixtures and 80-column goldens: status, inventory, skills, cooldowns, storage |
| Modify `internal/guards/template_width_guard_test.go`, `testdata/wrap_live_render.golden` | Fixtures that seed registries and render several modes; the screens leave "plan 3b" |
| Modify `internal/usercommands/context.md`, `docs/README.md` | Docs |

Edits land only by `git apply` or the Edit and Write tools; never `sed -i`, a `>` into a repo file, or Python writing a repo file (a hook enforces this). `$S` is the full path of the session scratchpad; set it at the start of every Bash call that uses it. `git apply` prints whitespace warnings for the goldens' trailing spaces; those are the recorded bytes and are expected.

---

### Task 0: Branch and the patch

- [ ] **Step 1:**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git worktree add C:/tmp/dogmud-3b1 -b feature/webclient-base-3b1-character-sheets origin/master
```

- [ ] **Step 2:** Write this script to `$S/extract_patch.py` with the Write tool:

```python
import re, sys
text = open(sys.argv[1], encoding="utf-8").read().replace("\r\n", "\n")
blocks = re.findall(r"^````diff\n(.*?)^````$", text, re.S | re.M)
assert len(blocks) == 1, len(blocks)
open(sys.argv[2], "w", encoding="utf-8", newline="\n").write(blocks[0])
```

```bash
S="<the session scratchpad, full path>"
python -I "$S/extract_patch.py" "/c/Users/Calabe Davis/workspace/DOGMud/docs/superpowers/plans/2026-11-10-webclient-base-3b1-character-sheets.md" "$S/3b1.patch"
cd C:/tmp/dogmud-3b1 && git apply --check "$S/3b1.patch" && echo CHECK-OK
```

Expected: `CHECK-OK`. If `origin/master` has moved past `af14f392b` and the check fails, stop and report.

---

### Task 1: The tests, red

```bash
cd C:/tmp/dogmud-3b1; S="<scratchpad>"; P="$S/3b1.patch"
git apply --include='internal/guards/character_screens_test.go' --include='internal/usercommands/narrow_forms_test.go' --include='internal/guards/testdata/character_screens_80/*' --include='internal/usercommands/testdata/narrow_forms_80/*' "$P"
go vet ./internal/usercommands/ 2>&1 | grep undefined | head -2
```

Expected: `undefined: questsScreen`. No commit yet.

---

### Task 2: The Go support

```bash
git apply --include=internal/templates/templatesfunctions.go --include=internal/usercommands/quests.go --include=internal/usercommands/title.go --include=internal/usercommands/companion.go "$P"
go build ./...
go test ./internal/usercommands/ -run 'KeepTheir80|KeepsIts80' -count=1 2>&1 | tail -1
go test ./internal/usercommands/ -run 'FitEveryWidth|FitsEveryWidth' -count=1 2>&1 | grep -E '^--- FAIL' | head -3
```

Expected: the build passes; the 80-column goldens pass (`ok`), which proves the type moves changed no output; `TestNarrowTemplateScreensFitEveryWidth` fails because the forms are not there yet. `./internal/guards/` does not build until Task 3 (`unknown field setup in struct literal of type templateFixture`).

---

### Task 3: The forms and the guard

```bash
git apply --include='_datafiles/world/dogmud/templates/*' --include=internal/guards/template_width_guard_test.go --include=internal/guards/testdata/wrap_live_render.golden "$P"
gofmt -l internal/
go test ./internal/usercommands/ ./internal/templates/ ./internal/guards/ -count=1 2>&1 | tail -3
```

Expected: `gofmt` prints nothing; `ok` three times.

Commit Tasks 1 to 3 together, with named paths:

```bash
T=_datafiles/world/dogmud/templates/character
git add $T/status.w42.template $T/status.w30.template $T/inventory.w42.template $T/inventory.w30.template $T/skills.w42.template $T/skills.w30.template $T/conditions.w42.template $T/conditions.w30.template $T/cooldowns.w42.template $T/cooldowns.w30.template $T/quests.w42.template $T/quests.w30.template $T/storage.w42.template $T/storage.w30.template $T/title.w42.template $T/title.w30.template internal/templates/templatesfunctions.go internal/usercommands/quests.go internal/usercommands/title.go internal/usercommands/companion.go internal/usercommands/narrow_forms_test.go internal/usercommands/testdata/narrow_forms_80 internal/guards/character_screens_test.go internal/guards/testdata/character_screens_80 internal/guards/template_width_guard_test.go internal/guards/testdata/wrap_live_render.golden
git commit -F - <<'EOF'
feat(screens): narrow forms for the character sheets

Status, inventory, skills, conditions, cooldowns, quests, storage, title and
companion get their approved 42- and 30-column forms. Goldens recorded from
master pin every 80-column screen, and the guard renders each form.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git status --short
```

Expected: `git status --short` prints nothing.

---

### Task 4: Docs

```bash
git apply --include=docs/README.md --include=internal/templates/context.md --include=internal/usercommands/context.md "$P"
cp "/c/Users/Calabe Davis/workspace/DOGMud/docs/superpowers/plans/2026-11-10-webclient-base-3b1-character-sheets.md" docs/superpowers/plans/
git add docs/README.md internal/templates/context.md internal/usercommands/context.md docs/superpowers/plans/2026-11-10-webclient-base-3b1-character-sheets.md
git diff --cached --stat
git commit -F - <<'EOF'
docs(screens): plan 3b-1, and the docs for the character sheet forms

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git apply --check -R "$P" && echo REVERSE-OK
```

Expected: README 4 added, templates context 13, usercommands context 19, the plan added; `REVERSE-OK`.

---

### Task 4b: Hide the surrender policy (#848, owner 2026-11-10)

The surrender policy is saved and shown but nothing in combat reads it (#848). The owner ruled it is not shown to players until #848 is resolved. The data, `ParseSurrenderPolicy`, mob policies and the `set surrender` command stay, so nothing is lost and resolving #848 only has to put the displays back; the command is left working but undocumented. This changes the 80-column status screen, so its goldens are re-recorded and the change gets a patch note.

**Files:**
- Modify `_datafiles/world/dogmud/templates/character/status.template:18`: the line becomes ` <ansi fg="yellow">Submission policy:</ansi> {{ .Character.SubmissionPolicy }}` (the four spaces and the Surrender label and value go).
- Modify `character/status.w42.template` and `status.w30.template`: delete the ` {{ wraptext (print "<ansi fg=\"yellow\">Surrender policy:</ansi> " ...` line. In the `.w42` header comment, "Prose after a label (the surrender policy, the broken limb line)" becomes "Prose after a label (the broken limb line)".
- Modify `internal/usercommands/set.go:167`: delete the `row(\`surrender\`, ...)` line of the `set` overview.
- Modify help pages: `help/set.template` deletes the four-line `set surrender` entry (from `set surrender <never|always|auto-tap-below N>` through `See help surrender.`, and the blank line after it) and drops `help surrender` from its See also (the line before it then ends without a trailing comma); `help/status.template` deletes the three-line `Surrender` entry and the blank line before it, and the `help surrender` line under Related; `help/combat.template:129`, `help/grapple.template:151` and `help/submission.template:53` drop `help surrender` from their lists, keeping each list's commas and line breaks tidy.
- Delete `_datafiles/world/dogmud/templates/help/surrender.template`, and in `_datafiles/world/dogmud/keywords.yaml` delete `- surrender` from the help topic list (line 104) and the `surrender: [submit, tap out, tapout]` alias line (263).
- Modify `internal/usercommands/template_freeze_test.go:161`: `assert.Contains(t, plain, "Surrender policy: tap out below 15% health")` becomes `assert.NotContains(t, plain, "Surrender policy", "hidden until #848 is resolved")`.
- Modify `docs/PATCH_NOTES.md`: under today's date, "The status screen and the settings list no longer show a surrender policy. The setting never took effect in a fight, so it is hidden until it does."

- [ ] **Step 1: Red.** Change the freeze test first, then run `go test ./internal/usercommands/ -run TestTemplateFreeze_StatusReadsInPlainWords -count=1`. Expected: FAIL, the status still prints "Surrender policy".
- [ ] **Step 2:** Make the template, `set.go`, help and keyword edits above with the Edit tool; `git rm` the help page.
- [ ] **Step 3:** Grep for anything still showing it: `grep -rn "help surrender\|Surrender policy\|row(\`surrender" _datafiles/world/dogmud/templates internal/usercommands --include=*.template --include=*.go`. Expected: no hits outside tests and comments. Any other guard that lists help topics or keywords must still pass; fix the list rather than the guard.
- [ ] **Step 4: Re-record goldens.** `go test ./internal/guards/ -run TestCharacterScreensKeepTheir80ColumnOutput -update-character-screens -count=1`, and re-record `internal/guards/testdata/wrap_live_render.golden` with that test's own update flag (read the test for its name). `git diff` on both goldens must show only the Surrender text removed.
- [ ] **Step 5: Green.** `gofmt -l internal/`, `go build ./...`, `go test ./internal/usercommands/ ./internal/templates/ ./internal/guards/ ./internal/help/... -count=1` (drop the help path if it does not exist). Expected: `ok` for each.
- [ ] **Step 6: Commit** with named paths only (every file above, the two goldens, `git rm`'s staged deletion), message `feat(status): hide the surrender policy until it works (#848)` with the Co-Authored-By line. Use `refs #848`; never a closing keyword.

---

### Task 5: Verify and ship

- [ ] **Step 1: Prove the guard can fail.** Make the last rule in `character/status.w30.template` one character longer. Expected: the guard reports `character/status (data 0) at 30 columns has a line 31 wide`. Do the same in `conditions.w30.template`. Expected: `TestNarrowTemplateScreensFitEveryWidth` fails for `character/conditions at 30 columns`. Undo both exactly; `git status --short` is empty.

- [ ] **Step 2: Gates**, in a worktree with no server running in it:

```bash
gofmt -l internal/ modules/
go build ./...
go test . ./internal/... ./modules/... -count=1 2>&1 | grep -vE '^ok|no test files' | tail -20
for t in tools/jstest/*.test.js tools/webclient-tests/*.js; do node "$t" > /dev/null 2>&1 || echo "RED $t"; done; echo done
golangci-lint run --new-from-merge-base=origin/master
```

- [ ] **Step 3: Boot check** on private ports (the `dogmud-shipping` skill), building to `C:/tmp/dogmud-3b1/boot-check.exe`. Offer the owner a telnet test server: `set linewidth 40` (telnet's minimum; it shows the 30-column forms, since 40 is under 42) and `set linewidth 42`, then `status`, `inventory`, `skills`, `conditions`, `quests`, `storage` at a bank, `title`, `companion`. A true 30-column screen is reachable only on the web, from plan 5.

- [ ] **Step 4: Push and open the PR** with `gh pr create --repo pruuk/DOGMud --base master --head feature/webclient-base-3b1-character-sheets --body-file "$S/pr3b1-body.md"`: what players see (nothing on the web until plan 5; telnet with `set linewidth` from 40 to 79 sees the forms, the 30-column ones below 42), the choices above, what was tested, "Part of #783", no closing keyword, ending with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. The forms get no `PATCH_NOTES` entry, since no web player sees them until plan 5; Task 4b's hidden surrender line has its own. The PR says "refs #848" for Task 4b. Merge only when the owner says.

---

## Self-review against the mock and rulings

| Ruling or drawing | Where |
|---|---|
| Horizontal rules only on narrow forms, full width, no text past a rule | the 16 forms; guard fit tests |
| Status: no closing rule between sections; Vitals pairs pools with reserves at 42 | `status.w42`, `status.w30` |
| Quest bar 20 cells at 30, never disagreeing with the percentage | `quests.w30`; `withProgress` is the one place that sets them, held by `TestQuestProgressHasOneSource` |
| Long prose under a label two spaces in | `status.w42` and `status.w30` (Surrender policy, broken limb) |
| Companion headings: rules at full width, name above the rule when it does not fit beside it | `companion.go`; `TestCompanionListFitsEveryWidth`, `TestCompanionDetailFitsEveryWidth` |
| A template in the "fits in usercommands" class has both forms and is rendered by the fit test | `TestFitsInUsercommandsClassIsHeld` |
| A 32-letter name overflows on its own line only | `TestStatusWithTheLongestNameRunsPastOnlyOnItsName` |
| Every form shows all the information; 80 unchanged | drawings diffed; 80 goldens |

## The patch

Applied in the tasks above, never by hand.

````diff
diff --git a/_datafiles/world/dogmud/templates/character/conditions.w30.template b/_datafiles/world/dogmud/templates/character/conditions.w30.template
new file mode 100644
index 000000000..8804b4a09
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/conditions.w30.template
@@ -0,0 +1,8 @@
+{{- /* The conditions at 30 columns: per condition its name 1 in, then its
+description wrapped 3 in, then how long it lasts. */ -}}
+{{ $w := 30 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Conditions</ansi> {{ repeat "─" (sub $w 16) }}
+{{ if eq (len .) 0 }} None
+{{ else }}{{ range $key, $val := . }} <ansi fg="yellow-bold">{{ wraptext $val.Name (sub $w 1) " " }}</ansi>
+   {{ wraptext $val.Description (sub $w 3) "   " }}{{ if not .Permanent }}
+   {{ durationQuality $val.RoundsLeft }}{{ end }}
+{{ end }}{{ end }}{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/conditions.w42.template b/_datafiles/world/dogmud/templates/character/conditions.w42.template
new file mode 100644
index 000000000..f1c2fca5a
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/conditions.w42.template
@@ -0,0 +1,9 @@
+{{- /* The conditions at 42 columns: per condition its name, then its
+description wrapped 6 in, then how long it lasts. Drawn to 42 whatever the
+reader's width. */ -}}
+{{ $w := 42 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Conditions</ansi> {{ repeat "─" (sub $w 16) }}
+{{ if eq (len .) 0 }}   None
+{{ else }}{{ range $key, $val := . }}   <ansi fg="yellow-bold">{{ wraptext $val.Name (sub $w 3) "   " }}</ansi>
+      {{ wraptext $val.Description (sub $w 6) "      " }}{{ if not .Permanent }}
+      {{ durationQuality $val.RoundsLeft }}{{ end }}
+{{ end }}{{ end }}{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/cooldowns.w30.template b/_datafiles/world/dogmud/templates/character/cooldowns.w30.template
new file mode 100644
index 000000000..87b6a50e1
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/cooldowns.w30.template
@@ -0,0 +1,7 @@
+{{- /* The cooldowns at 30 columns: each name 1 in, how long it lasts under
+it, 3 in. */ -}}
+{{ $w := 30 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Cooldowns</ansi> {{ repeat "─" (sub $w 15) }}
+{{ range $name, $cooldown := . }} <ansi fg="yellow-bold">{{ wraptext $name (sub $w 1) " " }}</ansi>
+   {{ durationQuality $cooldown }}
+{{ end }}{{ repeat "─" $w }}
+
diff --git a/_datafiles/world/dogmud/templates/character/cooldowns.w42.template b/_datafiles/world/dogmud/templates/character/cooldowns.w42.template
new file mode 100644
index 000000000..4b10dda26
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/cooldowns.w42.template
@@ -0,0 +1,8 @@
+{{- /* The cooldowns at 42 columns: the name padded to 24, then how long it
+lasts, or the duration under the name when the two do not fit on a line.
+Drawn to 42 whatever the reader's width. */ -}}
+{{ $w := 42 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Cooldowns</ansi> {{ repeat "─" (sub $w 15) }}
+{{ range $name, $cooldown := . }}{{ $d := durationQuality $cooldown }}{{ if and (le (len $name) 24) (le (add 28 (visiblewidth $d)) $w) }}   <ansi fg="yellow-bold">{{ padRight 24 $name }}</ansi> {{ $d }}{{ else }}   <ansi fg="yellow-bold">{{ wraptext $name (sub $w 3) "   " }}</ansi>
+      {{ $d }}{{ end }}
+{{ end }}{{ repeat "─" $w }}
+
diff --git a/_datafiles/world/dogmud/templates/character/inventory.w30.template b/_datafiles/world/dogmud/templates/character/inventory.w30.template
new file mode 100644
index 000000000..352b6bf1f
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/inventory.w30.template
@@ -0,0 +1,75 @@
+{{- /* The inventory at 30 columns: the equipment between two full-width
+rules, then the carried lists. An item name that does not fit beside its
+slot label goes on the next line, 2 past the label. A list wraps between
+names, hanging under the first name when every name fits there, else 2 past
+the label; a name longer than a whole line is wrapped on its own. Slots
+sit 1 in from the edge. */ -}}
+{{- define "slot" }}{{ if le (add (len .ind) 12 (visiblewidth .v)) .w }}{{ .ind }}<ansi fg="yellow">{{ padRight 12 .label }}</ansi><ansi fg="itemname">{{ .v }}</ansi>{{ else }}{{ .ind }}<ansi fg="yellow">{{ .label }}</ansi>
+{{ .ind }}  <ansi fg="itemname">{{ wraptext .v (sub .w (len .ind) 2) (print .ind "  ") }}</ansi>{{ end }}
+{{ end -}}
+{{- define "list" -}}
+{{- $w := .w -}}{{- $col := add 1 (len .label) -}}{{- $hang := add $col 1 -}}
+{{- if lt (sub $w $hang) 20 -}}{{- $hang = 3 -}}{{- end -}}
+{{- range .names -}}{{- if gt (add (visiblewidth .) 1) (sub $w $hang) -}}{{- $hang = 3 -}}{{- end -}}{{- end -}}
+{{- $pad := repeat " " $hang -}}{{- $cur := $col -}}{{- $last := sub (len .names) 1 -}}
+{{ " " }}{{ .label }}{{ if not .names }} {{ .empty }}{{ end }}
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
+{{ end -}}
+{{- define "bag" -}}
+{{- $w := .w -}}{{- $hang := 20 -}}
+{{- if lt (sub $w $hang) 20 -}}{{- $hang = 3 -}}{{- end -}}
+{{- range .names -}}{{- if gt (visiblewidth .) (sub $w $hang) -}}{{- $hang = 3 -}}{{- end -}}{{- end -}}
+{{- $pad := repeat " " $hang -}}
+{{ " " }}Found in your bag:
+{{- range $i, $f := .names -}}
+{{- if and (eq $i 0) (eq $hang 20) -}}
+{{ " " }}{{ $f }}
+{{- else -}}
+{{ "\n" }}{{ $pad }}{{ wraptext $f (sub $w $hang) $pad }}
+{{- end -}}
+{{- end }}
+{{ end -}}
+{{ $w := 30 }}{{ $ind := " " }}{{ if not .Searching }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Equipment</ansi> {{ repeat "─" (sub $w 15) }}
+{{ if not .Equipment.Weapon.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Weapon:" "v" .Equipment.Weapon.NameComplex) }}{{ end -}}
+{{- if and (not .Equipment.Offhand.IsDisabled) (not (.Character.IsBlockedBy2H "offhand")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Offhand:" "v" .Equipment.Offhand.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraArm1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 3:" "v" .Equipment.ExtraArm1.NameComplex) }}{{ end -}}
+{{- if and (not .Equipment.ExtraArm2.IsDisabled) (not (.Character.IsBlockedBy2H "extra arm 2")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 4:" "v" .Equipment.ExtraArm2.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraArm3.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 5:" "v" .Equipment.ExtraArm3.NameComplex) }}{{ end -}}
+{{- if and (not .Equipment.ExtraArm4.IsDisabled) (not (.Character.IsBlockedBy2H "extra arm 4")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 6:" "v" .Equipment.ExtraArm4.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Head.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Head:" "v" .Equipment.Head.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Neck.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Neck:" "v" .Equipment.Neck.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Shoulders.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Shoulders:" "v" .Equipment.Shoulders.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Body.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Body:" "v" .Equipment.Body.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Back.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Back:" "v" .Equipment.Back.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Belt.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Belt:" "v" .Equipment.Belt.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Wrist1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 1:" "v" .Equipment.Wrist1.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Wrist2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 2:" "v" .Equipment.Wrist2.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 3:" "v" .Equipment.ExtraWrist1.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 4:" "v" .Equipment.ExtraWrist2.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist3.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 5:" "v" .Equipment.ExtraWrist3.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist4.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 6:" "v" .Equipment.ExtraWrist4.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Gloves.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Gloves:" "v" .Equipment.Gloves.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Ring.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Ring:" "v" .Equipment.Ring.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Ring2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Ring:" "v" .Equipment.Ring2.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Legs.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Legs:" "v" .Equipment.Legs.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Feet.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Feet:" "v" .Equipment.Feet.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Tail.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Tail:" "v" .Equipment.Tail.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Light.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Light:" "v" .Equipment.Light.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ComponentBag.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Components:" "v" .Equipment.ComponentBag.NameComplex) }}{{ end -}}
+{{ repeat "─" $w }}
+{{ template "list" (map "w" $w "label" "Carrying:" "names" .ItemNamesFormatted "empty" "nothing") -}}
+{{ if .ComponentNames }}{{ template "list" (map "w" $w "label" "Components:" "names" .ComponentNamesFormatted "empty" "") }}{{ end -}}
+{{ if .PotionNames }}{{ template "list" (map "w" $w "label" "Bandolier:" "names" .PotionNamesFormatted "empty" "") }}{{ end -}}
+{{ " " }}<ansi fg="{{ .EncumbranceColor }}">Encumbrance: [{{ .EncumbranceLabel }}]</ansi>
+{{ else }}
+{{ template "bag" (map "w" $w "names" .ItemNamesFormatted) }}{{ end }}
diff --git a/_datafiles/world/dogmud/templates/character/inventory.w42.template b/_datafiles/world/dogmud/templates/character/inventory.w42.template
new file mode 100644
index 000000000..70de52af1
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/inventory.w42.template
@@ -0,0 +1,76 @@
+{{- /* The inventory at 42 columns: the equipment between two full-width
+rules, then the carried lists. An item name that does not fit beside its
+slot label goes on the next line, 2 past the label. A list wraps between
+names, hanging under the first name when every name fits there, else 2 past
+the label; a name longer than a whole line is wrapped on its own. The form
+is drawn to 42 whatever the reader's width, so a reader narrower than that
+gets the 30-column form. */ -}}
+{{- define "slot" }}{{ if le (add (len .ind) 12 (visiblewidth .v)) .w }}{{ .ind }}<ansi fg="yellow">{{ padRight 12 .label }}</ansi><ansi fg="itemname">{{ .v }}</ansi>{{ else }}{{ .ind }}<ansi fg="yellow">{{ .label }}</ansi>
+{{ .ind }}  <ansi fg="itemname">{{ wraptext .v (sub .w (len .ind) 2) (print .ind "  ") }}</ansi>{{ end }}
+{{ end -}}
+{{- define "list" -}}
+{{- $w := .w -}}{{- $col := add 1 (len .label) -}}{{- $hang := add $col 1 -}}
+{{- if lt (sub $w $hang) 20 -}}{{- $hang = 3 -}}{{- end -}}
+{{- range .names -}}{{- if gt (add (visiblewidth .) 1) (sub $w $hang) -}}{{- $hang = 3 -}}{{- end -}}{{- end -}}
+{{- $pad := repeat " " $hang -}}{{- $cur := $col -}}{{- $last := sub (len .names) 1 -}}
+{{ " " }}{{ .label }}{{ if not .names }} {{ .empty }}{{ end }}
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
+{{ end -}}
+{{- define "bag" -}}
+{{- $w := .w -}}{{- $hang := 20 -}}
+{{- if lt (sub $w $hang) 20 -}}{{- $hang = 3 -}}{{- end -}}
+{{- range .names -}}{{- if gt (visiblewidth .) (sub $w $hang) -}}{{- $hang = 3 -}}{{- end -}}{{- end -}}
+{{- $pad := repeat " " $hang -}}
+{{ " " }}Found in your bag:
+{{- range $i, $f := .names -}}
+{{- if and (eq $i 0) (eq $hang 20) -}}
+{{ " " }}{{ $f }}
+{{- else -}}
+{{ "\n" }}{{ $pad }}{{ wraptext $f (sub $w $hang) $pad }}
+{{- end -}}
+{{- end }}
+{{ end -}}
+{{ $w := 42 }}{{ $ind := "   " }}{{ if not .Searching }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Equipment</ansi> {{ repeat "─" (sub $w 15) }}
+{{ if not .Equipment.Weapon.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Weapon:" "v" .Equipment.Weapon.NameComplex) }}{{ end -}}
+{{- if and (not .Equipment.Offhand.IsDisabled) (not (.Character.IsBlockedBy2H "offhand")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Offhand:" "v" .Equipment.Offhand.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraArm1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 3:" "v" .Equipment.ExtraArm1.NameComplex) }}{{ end -}}
+{{- if and (not .Equipment.ExtraArm2.IsDisabled) (not (.Character.IsBlockedBy2H "extra arm 2")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 4:" "v" .Equipment.ExtraArm2.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraArm3.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 5:" "v" .Equipment.ExtraArm3.NameComplex) }}{{ end -}}
+{{- if and (not .Equipment.ExtraArm4.IsDisabled) (not (.Character.IsBlockedBy2H "extra arm 4")) }}{{ template "slot" (map "w" $w "ind" $ind "label" "Arm 6:" "v" .Equipment.ExtraArm4.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Head.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Head:" "v" .Equipment.Head.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Neck.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Neck:" "v" .Equipment.Neck.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Shoulders.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Shoulders:" "v" .Equipment.Shoulders.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Body.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Body:" "v" .Equipment.Body.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Back.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Back:" "v" .Equipment.Back.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Belt.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Belt:" "v" .Equipment.Belt.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Wrist1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 1:" "v" .Equipment.Wrist1.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Wrist2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 2:" "v" .Equipment.Wrist2.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist1.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 3:" "v" .Equipment.ExtraWrist1.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 4:" "v" .Equipment.ExtraWrist2.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist3.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 5:" "v" .Equipment.ExtraWrist3.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ExtraWrist4.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Wrist 6:" "v" .Equipment.ExtraWrist4.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Gloves.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Gloves:" "v" .Equipment.Gloves.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Ring.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Ring:" "v" .Equipment.Ring.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Ring2.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Ring:" "v" .Equipment.Ring2.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Legs.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Legs:" "v" .Equipment.Legs.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Feet.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Feet:" "v" .Equipment.Feet.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Tail.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Tail:" "v" .Equipment.Tail.NameComplex) }}{{ end -}}
+{{- if not .Equipment.Light.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Light:" "v" .Equipment.Light.NameComplex) }}{{ end -}}
+{{- if not .Equipment.ComponentBag.IsDisabled }}{{ template "slot" (map "w" $w "ind" $ind "label" "Components:" "v" .Equipment.ComponentBag.NameComplex) }}{{ end -}}
+{{ repeat "─" $w }}
+{{ template "list" (map "w" $w "label" "Carrying:" "names" .ItemNamesFormatted "empty" "nothing") -}}
+{{ if .ComponentNames }}{{ template "list" (map "w" $w "label" "Components:" "names" .ComponentNamesFormatted "empty" "") }}{{ end -}}
+{{ if .PotionNames }}{{ template "list" (map "w" $w "label" "Bandolier:" "names" .PotionNamesFormatted "empty" "") }}{{ end -}}
+{{ " " }}<ansi fg="{{ .EncumbranceColor }}">Encumbrance: [{{ .EncumbranceLabel }}]</ansi>
+{{ else }}
+{{ template "bag" (map "w" $w "names" .ItemNamesFormatted) }}{{ end }}
diff --git a/_datafiles/world/dogmud/templates/character/quests.w30.template b/_datafiles/world/dogmud/templates/character/quests.w30.template
new file mode 100644
index 000000000..69168f617
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/quests.w30.template
@@ -0,0 +1,15 @@
+{{- /* The quest list at 30 columns: the count under the title rule, then
+per quest its name, a 20-cell bar and percentage on a line of their own,
+then the step, progress and guide lines, all wrapped 1 in. */ -}}
+{{ $w := 30 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Quests</ansi> {{ repeat "─" (sub $w 12) }}
+ <ansi fg="20">{{ printf "( %d out of %d shown )" .QuestsFound .QuestsTotal }}</ansi>
+{{ $nlLen := sub (len .Records) 1 }}{{ range $idx, $qInfo := .Records }} <ansi fg="questname">{{ wraptext (print $qInfo.Name (or (and $qInfo.Repeatable " (repeatable)") "") (or (and $qInfo.Following " (following)") "")) (sub $w 1) " " }}</ansi>
+{{ if or $qInfo.BarFull $qInfo.BarEmpty }} <ansi fg="green">{{ $qInfo.BarFullAt 20 }}</ansi><ansi fg="black-bold">{{ $qInfo.BarEmptyAt 20 }}</ansi> <ansi fg="cyan-bold">{{ $qInfo.Completion }}</ansi>
+{{ end }} <ansi fg="white-bold">{{ wraptext $qInfo.Description (sub $w 1) " " }}</ansi>{{ if $qInfo.Progress }}
+ <ansi fg="white">{{ wraptext $qInfo.Progress (sub $w 1) " " }}</ansi>{{ end }}{{ if $qInfo.Guide }}
+ <ansi fg="240">{{ wraptext $qInfo.Guide (sub $w 1) " " }}</ansi>{{ end }}
+{{ if lt $idx $nlLen }}
+{{ end }}{{ end }}{{ repeat "─" $w }}
+{{ if ne .QuestsFound .QuestsTotal }} <ansi fg="240">To see all quests (including
+ completed), use <ansi fg="command">quests all</ansi>.</ansi>
+{{ end }}
diff --git a/_datafiles/world/dogmud/templates/character/quests.w42.template b/_datafiles/world/dogmud/templates/character/quests.w42.template
new file mode 100644
index 000000000..a130beeef
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/quests.w42.template
@@ -0,0 +1,14 @@
+{{- /* The quest list at 42 columns: per quest its name, its 25-cell bar
+and percentage on a line of their own, then the step, progress and guide
+lines, all wrapped 3 in. Drawn to 42 whatever the reader's width. */ -}}
+{{ $w := 42 }}{{ $count := printf "Quests ( %d out of %d shown )" .QuestsFound .QuestsTotal }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">{{ $count }}</ansi> {{ repeat "─" (sub $w 6 (len $count)) }}
+{{ $nlLen := sub (len .Records) 1 }}{{ range $idx, $qInfo := .Records }}   <ansi fg="questname">{{ wraptext (print $qInfo.Name (or (and $qInfo.Repeatable " (repeatable)") "") (or (and $qInfo.Following " (following)") "")) (sub $w 3) "   " }}</ansi>
+{{ if or $qInfo.BarFull $qInfo.BarEmpty }}   <ansi fg="green">{{ $qInfo.BarFull }}</ansi><ansi fg="black-bold">{{ $qInfo.BarEmpty }}</ansi> <ansi fg="cyan-bold">{{ $qInfo.Completion }}</ansi>
+{{ end }}   <ansi fg="white-bold">{{ wraptext $qInfo.Description (sub $w 3) "   " }}</ansi>{{ if $qInfo.Progress }}
+   <ansi fg="white">{{ wraptext $qInfo.Progress (sub $w 3) "   " }}</ansi>{{ end }}{{ if $qInfo.Guide }}
+   <ansi fg="240">{{ wraptext $qInfo.Guide (sub $w 3) "   " }}</ansi>{{ end }}
+{{ if lt $idx $nlLen }}
+{{ end }}{{ end }}{{ repeat "─" $w }}
+{{ if ne .QuestsFound .QuestsTotal }} <ansi fg="240">To see all quests (including completed),
+ use <ansi fg="command">quests all</ansi>.</ansi>
+{{ end }}
diff --git a/_datafiles/world/dogmud/templates/character/skills.w30.template b/_datafiles/world/dogmud/templates/character/skills.w30.template
new file mode 100644
index 000000000..6da637298
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/skills.w30.template
@@ -0,0 +1,11 @@
+{{- /* The skills at 30 columns: per skill its name and rank 1 in, then its
+description wrapped 3 in. "Recovering..." follows the rank when it fits,
+else it takes a line of its own at the description's indent. */ -}}
+{{ $w := 30 }}{{ $cooldowns := .SkillCooldowns }}{{ $blurbs := .SkillBlurbs }}{{ $nw := or .NameWidth 12 }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Skills</ansi> {{ repeat "─" (sub $w 12) }}
+{{ range $skillName, $skillLevel := .SkillList }}{{ $rank := "[Unknown]" }}{{ if ne $skillLevel 0 }}{{ $rank = print "<ansi fg=\"white\">[" (skillRank $skillLevel) "]</ansi>" }}{{ end -}}
+{{ " " }}<ansi fg="yellow-bold">{{ padRight $nw $skillName }}</ansi> {{ $rank }}{{ if ne (index $cooldowns $skillName) 0 }}{{ if le (add 16 $nw (visiblewidth $rank)) $w }} <ansi fg="red">Recovering...</ansi>{{ else }}
+   <ansi fg="red">Recovering...</ansi>{{ end }}{{ end }}
+{{ if index $blurbs $skillName }}   <ansi fg="black-bold">{{ wraptext (index $blurbs $skillName) (sub $w 3) "   " }}</ansi>
+{{ end }}{{ end }}{{ if eq (len .SkillList) 0 }} {{ wraptext "No Skills! Visit a guild or training center to train." (sub $w 1) " " }}
+{{ end }}{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/skills.w42.template b/_datafiles/world/dogmud/templates/character/skills.w42.template
new file mode 100644
index 000000000..4dff90a68
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/skills.w42.template
@@ -0,0 +1,12 @@
+{{- /* The skills at 42 columns: per skill its name and rank, then its
+description wrapped 6 in. "Recovering..." follows the rank when it fits,
+else it takes a line of its own at the description's indent. Drawn to 42
+whatever the reader's width. */ -}}
+{{ $w := 42 }}{{ $cooldowns := .SkillCooldowns }}{{ $blurbs := .SkillBlurbs }}{{ $nw := or .NameWidth 12 }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Skills</ansi> {{ repeat "─" (sub $w 12) }}
+{{ range $skillName, $skillLevel := .SkillList }}{{ $rank := "[Unknown]" }}{{ if ne $skillLevel 0 }}{{ $rank = print "<ansi fg=\"white\">[" (skillRank $skillLevel) "]</ansi>" }}{{ end -}}
+{{ "   " }}<ansi fg="yellow-bold">{{ padRight $nw $skillName }}</ansi> {{ $rank }}{{ if ne (index $cooldowns $skillName) 0 }}{{ if le (add 18 $nw (visiblewidth $rank)) $w }} <ansi fg="red">Recovering...</ansi>{{ else }}
+      <ansi fg="red">Recovering...</ansi>{{ end }}{{ end }}
+{{ if index $blurbs $skillName }}      <ansi fg="black-bold">{{ wraptext (index $blurbs $skillName) (sub $w 6) "      " }}</ansi>
+{{ end }}{{ end }}{{ if eq (len .SkillList) 0 }}   {{ wraptext "No Skills! Visit a guild or training center to train." (sub $w 3) "   " }}
+{{ end }}{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/status.w30.template b/_datafiles/world/dogmud/templates/character/status.w30.template
new file mode 100644
index 000000000..098e111c1
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/status.w30.template
@@ -0,0 +1,46 @@
+{{- /* The status sheet at 30 columns: each section a titled rule and one
+label: value pair per line, everything the 80-column sheet shows. One
+closing rule follows the last section. A name too long to keep "the"
+beside it at 30 columns puts "the" on the title's line. */ -}}
+{{- define "one" }} <ansi fg="yellow">{{ padRight 13 .a }}</ansi>{{ .va }}{{ end -}}
+{{ $w := 30 }}{{ $c := .Character }}{{ $title := title .Character }}
+{{ if le (add 9 (len $c.Name) (len $title)) $w }} <ansi fg="black-bold">.:</ansi> <ansi fg="username">{{ $c.Name }}</ansi> the {{ $title }}{{ else if le (add 8 (len $c.Name)) $w }} <ansi fg="black-bold">.:</ansi> <ansi fg="username">{{ $c.Name }}</ansi> the
+    {{ wraptext $title (sub $w 4) "    " }}{{ else }} <ansi fg="black-bold">.:</ansi> <ansi fg="username">{{ $c.Name }}</ansi>
+    {{ wraptext (print "the " $title) (sub $w 4) "    " }}{{ end }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Attributes</ansi> {{ repeat "─" (sub $w 16) }}
+{{ template "one" (map "a" "Strength:" "va" (statQuality .Character.Stats.Strength.Value)) }}
+{{ template "one" (map "a" "Dexterity:" "va" (statQuality .Character.Stats.Dexterity.Value)) }}
+{{ template "one" (map "a" "Perception:" "va" (statQuality .Character.Stats.Perception.Value)) }}
+{{ template "one" (map "a" "Vitality:" "va" (statQuality .Character.Stats.Vitality.Value)) }}
+{{ template "one" (map "a" "Willpower:" "va" (statQuality .Character.Stats.Willpower.Value)) }}
+{{ template "one" (map "a" "Charisma:" "va" (statQuality .Character.Stats.Charisma.Value)) }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Vitals</ansi> {{ repeat "─" (sub $w 12) }}
+{{ template "one" (map "a" "Health:" "va" (vitalQuality $c.Health ($c.EffectivePoolMaxNamed "health"))) }}
+{{ template "one" (map "a" "Stamina:" "va" (vitalQuality $c.Stamina ($c.EffectivePoolMaxNamed "stamina"))) }}
+{{ template "one" (map "a" "Conviction:" "va" (vitalQuality $c.Conviction ($c.EffectivePoolMaxNamed "conviction"))) }}
+{{ template "one" (map "a" "HP reserve:" "va" (reservationQuality ($c.ReservationBandName "health"))) }}
+{{ template "one" (map "a" "SP reserve:" "va" (reservationQuality ($c.ReservationBandName "stamina"))) }}
+{{ template "one" (map "a" "CP reserve:" "va" (reservationQuality ($c.ReservationBandName "conviction"))) }}
+{{ template "one" (map "a" "Toxicity:" "va" (toxicityQuality $c.ToxicityBandName)) }}
+{{ template "one" (map "a" "Encumbrance:" "va" (encumbranceQuality $c.GetCarriedWeight $c.CarryCapacity)) }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Defenses</ansi> {{ repeat "─" (sub $w 14) }}
+{{ template "one" (map "a" "Physical:" "va" (defenseQuality (effectiveMitigation $c "physical"))) }}
+{{ template "one" (map "a" "Magical:" "va" (defenseQuality (effectiveMitigation $c "magical"))) }}
+{{ template "one" (map "a" "Conviction:" "va" (defenseQuality (effectiveMitigation $c "conviction"))) }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Info</ansi> {{ repeat "─" (sub $w 10) }}
+{{ template "one" (map "a" "Species:" "va" $c.SpeciesDisplay) }}
+{{ template "one" (map "a" "Gold:" "va" (numberFormat $c.Gold)) }}
+{{ template "one" (map "a" "Bank:" "va" (numberFormat $c.Bank)) }}
+{{ repeat "─" $w }}
+ <ansi fg="yellow">Submission policy:</ansi> {{ $c.SubmissionPolicy }}
+ {{ wraptext (print "<ansi fg=\"yellow\">Surrender policy:</ansi> " $c.SurrenderPolicy.Describe) (sub $w 3) "   " }}
+{{- if $c.Conditions.HasCondition 83 }}
+ <ansi fg="red-bold">{{ wraptext (printf "Broken limb: %d rounds remaining" ($c.Conditions.TriggersLeft 83)) (sub $w 3) "   " }}</ansi>
+{{- end }}
+{{- if $c.Mutations }}
+ <ansi fg="black-bold">.:</ansi><ansi fg="20">Mutations</ansi><ansi fg="black-bold">:.</ansi>
+{{- range $id, $level := $c.Mutations }}
+ <ansi fg="yellow">{{ wraptext $id (sub $w 1) " " }}</ansi>
+   <ansi fg="magenta">{{ mutationLevel $level }}</ansi>
+{{- end }}
+{{- end }}
diff --git a/_datafiles/world/dogmud/templates/character/status.w42.template b/_datafiles/world/dogmud/templates/character/status.w42.template
new file mode 100644
index 000000000..59a8b579a
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/status.w42.template
@@ -0,0 +1,85 @@
+{{- /* The status sheet at 42 columns: each section a titled rule and its
+label: value pairs, two to a line with the second at column 22. A section
+with a value too long for its column lists one pair per line. One closing
+rule follows the last section. Prose after a label (the surrender policy,
+the broken limb line) continues 2 past the label (owner, 2026-11-10). The
+form is drawn to 42 whatever the reader's width, so a reader narrower than
+that gets the 30-column form. */ -}}
+{{- define "pair" }} <ansi fg="yellow">{{ padRight 13 .a }}</ansi>{{ .va }}{{ if .b }}{{ repeat " " (sub 8 (visiblewidth .va)) }}<ansi fg="yellow">{{ padRight 13 .b }}</ansi>{{ .vb }}{{ end }}{{ end -}}
+{{- define "one" }} <ansi fg="yellow">{{ padRight 13 .a }}</ansi>{{ .va }}{{ end -}}
+{{ $w := 42 }}{{ $c := .Character }}{{ $title := title .Character }}
+{{ if le (add 9 (len $c.Name) (len $title)) $w }} <ansi fg="black-bold">.:</ansi> <ansi fg="username">{{ $c.Name }}</ansi> the {{ $title }}{{ else }} <ansi fg="black-bold">.:</ansi> <ansi fg="username">{{ $c.Name }}</ansi> the
+    {{ wraptext $title (sub $w 4) "    " }}{{ end }}
+{{- $a1 := statQuality $c.Stats.Strength.Value }}{{ $a2 := statQuality $c.Stats.Dexterity.Value }}
+{{- $a3 := statQuality $c.Stats.Perception.Value }}{{ $a4 := statQuality $c.Stats.Vitality.Value }}
+{{- $a5 := statQuality $c.Stats.Willpower.Value }}{{ $a6 := statQuality $c.Stats.Charisma.Value }}
+{{- $right := sub $w 35 }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Attributes</ansi> {{ repeat "─" (sub $w 16) }}
+{{ if and (le (len $a1) 6) (le (len $a3) 6) (le (len $a5) 6) (le (len $a2) $right) (le (len $a4) $right) (le (len $a6) $right) -}}
+{{ template "pair" (map "a" "Strength:" "va" $a1 "b" "Dexterity:" "vb" $a2) }}
+{{ template "pair" (map "a" "Perception:" "va" $a3 "b" "Vitality:" "vb" $a4) }}
+{{ template "pair" (map "a" "Willpower:" "va" $a5 "b" "Charisma:" "vb" $a6) }}
+{{ else -}}
+{{ template "one" (map "a" "Strength:" "va" $a1) }}
+{{ template "one" (map "a" "Dexterity:" "va" $a2) }}
+{{ template "one" (map "a" "Perception:" "va" $a3) }}
+{{ template "one" (map "a" "Vitality:" "va" $a4) }}
+{{ template "one" (map "a" "Willpower:" "va" $a5) }}
+{{ template "one" (map "a" "Charisma:" "va" $a6) }}
+{{ end -}}
+{{- $v1 := vitalQuality $c.Health ($c.EffectivePoolMaxNamed "health") }}{{ $v2 := reservationQuality ($c.ReservationBandName "health") }}
+{{- $v3 := vitalQuality $c.Stamina ($c.EffectivePoolMaxNamed "stamina") }}{{ $v4 := reservationQuality ($c.ReservationBandName "stamina") }}
+{{- $v5 := vitalQuality $c.Conviction ($c.EffectivePoolMaxNamed "conviction") }}{{ $v6 := reservationQuality ($c.ReservationBandName "conviction") }}
+{{- $v7 := toxicityQuality $c.ToxicityBandName }}{{ $v8 := encumbranceQuality $c.GetCarriedWeight $c.CarryCapacity -}}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Vitals</ansi> {{ repeat "─" (sub $w 12) }}
+{{ if and (le (visiblewidth $v1) 6) (le (visiblewidth $v3) 6) (le (visiblewidth $v5) 6) (le (visiblewidth $v7) 6) (le (visiblewidth $v2) $right) (le (visiblewidth $v4) $right) (le (visiblewidth $v6) $right) (le (visiblewidth $v8) $right) -}}
+{{ template "pair" (map "a" "Health:" "va" $v1 "b" "HP reserve:" "vb" $v2) }}
+{{ template "pair" (map "a" "Stamina:" "va" $v3 "b" "SP reserve:" "vb" $v4) }}
+{{ template "pair" (map "a" "Conviction:" "va" $v5 "b" "CP reserve:" "vb" $v6) }}
+{{ template "pair" (map "a" "Toxicity:" "va" $v7 "b" "Encumbrance:" "vb" $v8) }}
+{{ else -}}
+{{ template "one" (map "a" "Health:" "va" $v1) }}
+{{ template "one" (map "a" "HP reserve:" "va" $v2) }}
+{{ template "one" (map "a" "Stamina:" "va" $v3) }}
+{{ template "one" (map "a" "SP reserve:" "va" $v4) }}
+{{ template "one" (map "a" "Conviction:" "va" $v5) }}
+{{ template "one" (map "a" "CP reserve:" "va" $v6) }}
+{{ template "one" (map "a" "Toxicity:" "va" $v7) }}
+{{ template "one" (map "a" "Encumbrance:" "va" $v8) }}
+{{ end -}}
+{{- $d1 := defenseQuality (effectiveMitigation $c "physical") }}{{ $d2 := defenseQuality (effectiveMitigation $c "magical") }}
+{{- $d3 := defenseQuality (effectiveMitigation $c "conviction") -}}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Defenses</ansi> {{ repeat "─" (sub $w 14) }}
+{{ if and (le (len $d1) 6) (le (len $d2) $right) -}}
+{{ template "pair" (map "a" "Physical:" "va" $d1 "b" "Magical:" "vb" $d2) }}
+{{ else -}}
+{{ template "one" (map "a" "Physical:" "va" $d1) }}
+{{ template "one" (map "a" "Magical:" "va" $d2) }}
+{{ end -}}
+{{ template "one" (map "a" "Conviction:" "va" $d3) }}
+{{- $i1 := $c.SpeciesDisplay }}{{ $i2 := numberFormat $c.Gold }}{{ $i3 := numberFormat $c.Bank }}
+── <ansi fg="black-bold">.:</ansi><ansi fg="20">Info</ansi> {{ repeat "─" (sub $w 10) }}
+{{ if and (le (len $i1) 6) (le (len $i2) $right) -}}
+{{ template "pair" (map "a" "Species:" "va" $i1 "b" "Gold:" "vb" $i2) }}
+{{ else -}}
+{{ template "one" (map "a" "Species:" "va" $i1) }}
+{{ template "one" (map "a" "Gold:" "va" $i2) }}
+{{ end -}}
+{{ template "one" (map "a" "Bank:" "va" $i3) }}
+{{ repeat "─" $w }}
+ <ansi fg="yellow">Submission policy:</ansi> {{ $c.SubmissionPolicy }}
+ {{ wraptext (print "<ansi fg=\"yellow\">Surrender policy:</ansi> " $c.SurrenderPolicy.Describe) (sub $w 3) "   " }}
+{{- if $c.Conditions.HasCondition 83 }}
+ <ansi fg="red-bold">{{ wraptext (printf "Broken limb: %d rounds remaining" ($c.Conditions.TriggersLeft 83)) (sub $w 3) "   " }}</ansi>
+{{- end }}
+{{- if $c.Mutations }}
+ <ansi fg="black-bold">.:</ansi><ansi fg="20">Mutations</ansi><ansi fg="black-bold">:.</ansi>
+{{- range $id, $level := $c.Mutations }}
+{{- if and (le (len $id) 25) (le (add 29 (len (mutationLevel $level))) $w) }}
+   <ansi fg="yellow">{{ printf "%-25s" $id }}</ansi> <ansi fg="magenta">{{ mutationLevel $level }}</ansi>
+{{- else }}
+   <ansi fg="yellow">{{ wraptext $id (sub $w 3) "   " }}</ansi>
+      <ansi fg="magenta">{{ mutationLevel $level }}</ansi>
+{{- end }}
+{{- end }}
+{{- end }}
diff --git a/_datafiles/world/dogmud/templates/character/storage.w30.template b/_datafiles/world/dogmud/templates/character/storage.w30.template
new file mode 100644
index 000000000..4dfd7c9e4
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/storage.w30.template
@@ -0,0 +1,13 @@
+{{- /* Storage at 30 columns: the numbered list 1 in between two full-width
+rules, a long name wrapped under itself, then the commands, each kept whole. */ -}}
+{{ $w := 30 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">In Storage</ansi> {{ repeat "─" (sub $w 16) }}
+{{ range $idx, $itemName := . }}{{ $num := printf " %d. " (add $idx 1) }}{{ $num }}<ansi fg="itemname">{{ wraptext $itemName (sub $w (len $num)) (repeat " " (len $num)) }}</ansi>
+{{ end }}{{ if eq (len .) 0 }} Nothing
+{{ end }}{{ repeat "─" $w }}
+ <ansi fg="command">storage add [item/all]</ansi> to
+ store items. <ansi fg="command">unstore [item/#]</ansi>
+ to retrieve.
+ <ansi fg="command">stow</ansi> deposits all your
+ crafting components here at
+ once.
+
diff --git a/_datafiles/world/dogmud/templates/character/storage.w42.template b/_datafiles/world/dogmud/templates/character/storage.w42.template
new file mode 100644
index 000000000..08fdb5e71
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/storage.w42.template
@@ -0,0 +1,12 @@
+{{- /* Storage at 42 columns: the numbered list between two full-width
+rules, a long name wrapped under itself, then the commands, each kept whole.
+Drawn to 42 whatever the reader's width. */ -}}
+{{ $w := 42 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">In Storage</ansi> {{ repeat "─" (sub $w 16) }}
+{{ range $idx, $itemName := . }}{{ $num := printf "   %d. " (add $idx 1) }}{{ $num }}<ansi fg="itemname">{{ wraptext $itemName (sub $w (len $num)) (repeat " " (len $num)) }}</ansi>
+{{ end }}{{ if eq (len .) 0 }}   Nothing
+{{ end }}{{ repeat "─" $w }}
+ <ansi fg="command">storage add [item/all]</ansi> to store items.
+ <ansi fg="command">unstore [item/#]</ansi> to retrieve.
+ <ansi fg="command">stow</ansi> deposits all your crafting
+ components here at once.
+
diff --git a/_datafiles/world/dogmud/templates/character/title.w30.template b/_datafiles/world/dogmud/templates/character/title.w30.template
new file mode 100644
index 000000000..9318ba076
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/title.w30.template
@@ -0,0 +1,17 @@
+{{- /* The title screen at 30 columns: labels 1 in with one space before
+the value; what does not fit beside a label goes under it, 2 past it. The
+stat bars move under their names and keep their 20 cells. */ -}}
+{{- define "tier" }} <ansi fg="yellow">{{ .label }}</ansi> {{ .value }}{{ if le (add 3 (len .label) (len .value) (len .paren)) .w }} {{ else }}
+   {{ end }}<ansi fg="black-bold">{{ .paren }}</ansi>{{ end -}}
+{{ $w := 30 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Title</ansi> {{ repeat "─" (sub $w 11) }}
+{{ if le (add 16 (len .FullTitle)) $w }} <ansi fg="yellow-bold">Current Title:</ansi> <ansi fg="white-bold">{{ .FullTitle }}</ansi>{{ else }} <ansi fg="yellow-bold">Current Title:</ansi>
+   <ansi fg="white-bold">{{ wraptext .FullTitle (sub $w 3) "   " }}</ansi>{{ end }}
+
+{{ template "tier" (map "w" $w "label" "Mutation Tier:" "value" .MutTier "paren" (printf "(%s presence)" .MutLoad)) }}
+{{ template "tier" (map "w" $w "label" "Skill Tier:" "value" .SkillTier "paren" (printf "(%s)" .SkillPct)) }}
+ <ansi fg="yellow">Archetype:</ansi> {{ .Archetype }}
+
+ <ansi fg="yellow">Stat Profile:</ansi>
+{{ range $idx, $stat := .TopStats }}  {{ $stat.Name }}
+    {{ $stat.Bar }}
+{{ end }}{{ repeat "─" $w }}
diff --git a/_datafiles/world/dogmud/templates/character/title.w42.template b/_datafiles/world/dogmud/templates/character/title.w42.template
new file mode 100644
index 000000000..c82179141
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/character/title.w42.template
@@ -0,0 +1,17 @@
+{{- /* The title screen at 42 columns: label: value lines, a value's
+parenthesis hanging under the value when the two do not fit on a line, and
+a title too long for its line moved under the label, 2 past it. The stat
+bars keep their 20 cells. Drawn to 42 whatever the reader's width. */ -}}
+{{- define "tier" }}   <ansi fg="yellow">{{ .label }}</ansi>{{ repeat " " (sub 16 (len .label)) }}{{ .value }}{{ if le (add 19 (len .value) 1 (len .paren)) .w }} {{ else }}
+                   {{ end }}<ansi fg="black-bold">{{ .paren }}</ansi>{{ end -}}
+{{ $w := 42 }}── <ansi fg="black-bold">.:</ansi><ansi fg="20">Title</ansi> {{ repeat "─" (sub $w 11) }}
+{{ if le (add 18 (len .FullTitle)) $w }}   <ansi fg="yellow-bold">Current Title:</ansi> <ansi fg="white-bold">{{ .FullTitle }}</ansi>{{ else }}   <ansi fg="yellow-bold">Current Title:</ansi>
+     <ansi fg="white-bold">{{ wraptext .FullTitle (sub $w 5) "     " }}</ansi>{{ end }}
+
+{{ template "tier" (map "w" $w "label" "Mutation Tier:" "value" .MutTier "paren" (printf "(%s presence)" .MutLoad)) }}
+{{ template "tier" (map "w" $w "label" "Skill Tier:" "value" .SkillTier "paren" (printf "(%s)" .SkillPct)) }}
+   <ansi fg="yellow">Archetype:</ansi>      {{ .Archetype }}
+
+   <ansi fg="yellow">Stat Profile:</ansi>
+{{ range $idx, $stat := .TopStats }}    {{ padRight 12 $stat.Name }} {{ $stat.Bar }}
+{{ end }}{{ repeat "─" $w }}
diff --git a/docs/README.md b/docs/README.md
index 5c5684ec0..2510ac31b 100644
--- a/docs/README.md
+++ b/docs/README.md
@@ -249,8 +249,12 @@ than reading an old result:
 | [`../internal/guards/closing_step_guard_test.go`](../internal/guards/closing_step_guard_test.go) | #375 closing step guards: `TestLoopBacklogStaysRetired` (no `Ratchet`, `ParseBacklog`, `FormatBacklog`, `LoadBacklog`, `BacklogPath` or `loop-backlog.txt` in the Go source) and `TestRoomInteractStaysRetired` (no `room_interact` event or quest `verb` in shipped content or code), each with a fixture proving it can fail |
 | [`../internal/guards/command_parity_guard_test.go`](../internal/guards/command_parity_guard_test.go) | Owner ruling 2026-11-07: the boot's command parity audit, run on the real user and mob registries with modules loaded, finds only `throw` (the standing reminder that a mob throw is owed); every other one-sided command is allowlisted with its reason in `internal/actions/divergences.go`. Includes a fixture proving the audit reports an unlisted gap |
 | [`superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md`](superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md) | Plan 3a of the web client base: narrow template forms picked by width, the `screenwidth` template function, table forms, splash to words only on narrow screens, the MOTD's narrow form, and the template width guard |
+| [`superpowers/plans/2026-11-10-webclient-base-3b1-character-sheets.md`](superpowers/plans/2026-11-10-webclient-base-3b1-character-sheets.md) | Plan 3b-1 of the web client base: 42- and 30-column forms of the character sheets (status, inventory, skills, conditions, cooldowns, quests, storage, title) and the companion screen, with their fit tests and 80-column goldens recorded from master |
 | [`../internal/templates/forms.go`](../internal/templates/forms.go) | Narrow forms: `FormWidths`, `WidestLine`, the reader's width and the widest-form-that-fits rule behind `templates.Process` |
 | [`../internal/guards/template_width_guard_test.go`](../internal/guards/template_width_guard_test.go) | Ruling 3 of the web client base: every template has one width class (fits at 30, 42 and 80, words-only splash, map or wide table, raw login text, wrapped, plan 3b, help, admin, or no caller), form files are named for a known width, and the fitting ones are rendered and measured |
+| [`../internal/guards/character_screens_test.go`](../internal/guards/character_screens_test.go) | Plan 3b: the widest-in-play fixtures of the status, inventory, skills, cooldowns and storage screens for the template width guard, and their 80-column goldens (`testdata/character_screens_80/`, recorded from master) proving a desktop sees what it saw |
+| [`../internal/usercommands/narrow_forms_test.go`](../internal/usercommands/narrow_forms_test.go) | Plan 3b: the conditions, quests, title and companion screens fit at 30, 42 and 80 columns with their widest data, and keep their 80-column output (`testdata/narrow_forms_80/`, recorded from master) |
+| `../_datafiles/world/dogmud/templates/character/{status,inventory,skills,conditions,cooldowns,quests,storage,title}.w42.template` and `.w30.template` | Plan 3b: the narrow forms of the character screens, as drawn in `_archive/notes/reports/narrow-screens-3b-mock/forms/` with the owner's 2026-11-10 rulings |
 | [`../internal/guards/no_entrance_guard_test.go`](../internal/guards/no_entrance_guard_test.go) | Owner ruling 2026-11-07: loading the shipped world, no room is left that no exit leads into unless `rooms.noEntranceByDesign` lists how players reach it (tutorial room, instance template, ferries) |
 | [`../internal/rooms/move_missing_room_test.go`](../internal/rooms/move_missing_room_test.go) | Entering the world into a room that no longer exists lands at the start room; an ordinary move to a missing room still refuses |
 | [`../internal/web/not_found_test.go`](../internal/web/not_found_test.go) | A refused `_` file that is not a page gets one plain 404 (no second status), an ordinary image is still served, and the mutation fallback picture is `generic.png`, which the `_` rule would refuse under its old name |
diff --git a/internal/guards/character_screens_test.go b/internal/guards/character_screens_test.go
new file mode 100644
index 000000000..2505316dc
--- /dev/null
+++ b/internal/guards/character_screens_test.go
@@ -0,0 +1,275 @@
+package guards
+
+import (
+	"flag"
+	"fmt"
+	"os"
+	"path/filepath"
+	"strings"
+	"testing"
+
+	"github.com/GoMudEngine/GoMud/internal/characters"
+	"github.com/GoMudEngine/GoMud/internal/conditions"
+	"github.com/GoMudEngine/GoMud/internal/items"
+	"github.com/GoMudEngine/GoMud/internal/skills"
+	"github.com/GoMudEngine/GoMud/internal/species"
+	"github.com/GoMudEngine/GoMud/internal/templates"
+	"github.com/GoMudEngine/GoMud/internal/usercommands"
+	"github.com/GoMudEngine/GoMud/internal/users"
+)
+
+// The character screens with narrow forms (plan 3b): fixtures that make each
+// as wide as it gets in play, built from the data types their commands pass,
+// and the goldens that prove a desktop sees at 80 columns what it saw before
+// the forms. conditions, quests and title pass types private to
+// internal/usercommands, so they are held there (narrow_forms_test.go).
+
+var updateCharacterScreens = flag.Bool("update-character-screens", false,
+	"rewrite internal/guards/testdata/character_screens_80/*.golden from the current templates")
+
+// seedCharacterScreenRegistries gives the fixtures the species and the
+// broken-limb condition they read, restored when the test ends.
+func seedCharacterScreenRegistries(t *testing.T) {
+	t.Helper()
+	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
+		1: {SpeciesId: 1, Name: "human", Selectable: true},
+	}))
+	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
+		83: {ConditionId: 83, Name: "Broken Limb", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 12},
+	}))
+}
+
+// screenItem is an item whose spec is its own, so no item registry is read.
+func screenItem(id int, name string, typ items.ItemType, bonus int, enchanted bool) items.Item {
+	itm := items.Item{ItemId: id, Spec: &items.ItemSpec{
+		ItemId: id, Name: name, Type: typ, Hands: 1,
+		Damage: items.Damage{BonusDamage: bonus},
+	}}
+	if enchanted {
+		itm.Enchantments = 1
+	}
+	return itm
+}
+
+// statusScreenFixture is the status sheet of a character whose every value
+// runs long: a long name and title, a stat at its longest word, a pool at a
+// long word, gold and bank in the millions, a broken limb, two mutations with
+// the longest ids, and the longest surrender policy.
+func statusScreenFixture() any {
+	return statusScreenFixtureNamed("Maximilianovich")
+}
+
+// statusScreenLongName is the status sheet of a character whose 26-letter
+// name, the longest that fits beside " .: " at 30 columns, pushes "the" onto
+// the title's line there.
+func statusScreenLongName() any {
+	return statusScreenFixtureNamed("Bartholomewsworthingtonian")
+}
+
+func statusScreenFixtureNamed(name string) any {
+	u := users.NewUserRecord(88150, 0)
+	c := u.Character
+	c.Name = name
+	c.SpeciesId = 1
+	c.Stats.Strength.Base = 250
+	c.Stats.Dexterity.Base = 140
+	c.Stats.Perception.Base = 70
+	c.Stats.Vitality.Base = 240
+	c.Stats.Willpower.Base = 100
+	c.Stats.Charisma.Base = 50
+	c.RecalculateStats()
+	c.Health = 1
+	c.Gold = 1234567
+	c.Bank = 987654321
+	c.Mutations = map[string]int{"reflect-skin-frostbite": 4, "sustaining-presence": 2}
+	c.SurrenderPolicy = characters.SurrenderPolicy{Mode: characters.SurrenderAutoTap, HpPctThreshold: 100}
+	c.Conditions.Validate(true)
+	c.Conditions.AddCondition(83, false)
+	return u
+}
+
+// inventoryScreenFixture is the inventory data internal/usercommands'
+// Inventory builds, for a character in long-named, enchanted gear carrying a
+// long pack, components and a full bandolier.
+func inventoryScreenFixture(searching bool) func() any {
+	return func() any {
+		u := users.NewUserRecord(88151, 0)
+		c := u.Character
+		c.Equipment.Weapon = screenItem(99201, "Pit Fighter's Serrated Basalt-Iron Blade", items.Weapon, 3, true)
+		c.Equipment.Back = screenItem(99202, "Wayfarer's Bottomless Pack", items.Back, 0, false)
+		c.Equipment.Neck = screenItem(99203, "A Salt-Stiff Page of a Sailor's Telling", items.Neck, 0, true)
+		c.Equipment.Light = screenItem(99204, "Hooded Lantern", items.Light, 0, false)
+		names := []string{"A Salt-Stiff Page of a Sailor's Telling (x12)", "Leather Backpack",
+			"Reinforced Travel Pack", "Strongbox Key", "Pit Fighter's Sword", "Basalt-Iron Blade",
+			"Spirit Fetish (turned)", "Healing Draught (3)", "A Stained Page with a Deathbed Word"}
+		formatted := make([]string, len(names))
+		for i, n := range names {
+			formatted[i] = `<ansi fg="itemname">` + n + `</ansi>`
+		}
+		comps := []string{"Iron Ore (x24)", "Rough-Cut Moonstone Shard (x3)", "Tanned Hide"}
+		compsFormatted := []string{`<ansi fg="itemname">Iron Ore</ansi> <ansi fg="uses-left">(x24)</ansi>`,
+			`<ansi fg="itemname">Rough-Cut Moonstone Shard</ansi> <ansi fg="uses-left">(x3)</ansi>`,
+			`<ansi fg="itemname">Tanned Hide</ansi>`}
+		pots := []string{"Minor Potion of Healing (x4)", "Conviction Draught", "Stamina Draught (turned)"}
+		potsFormatted := []string{`<ansi fg="itemname">Minor Potion of Healing</ansi> <ansi fg="uses-left">(x4)</ansi>`,
+			`<ansi fg="itemname">Conviction Draught</ansi>`, `<ansi fg="8">Stamina Draught (turned)</ansi>`}
+		return map[string]any{
+			`Character`:               c,
+			`Equipment`:               &c.Equipment,
+			`ItemNames`:               names,
+			`ItemNamesFormatted`:      formatted,
+			`ComponentNames`:          comps,
+			`ComponentNamesFormatted`: compsFormatted,
+			`PotionNames`:             pots,
+			`PotionNamesFormatted`:    potsFormatted,
+			`Searching`:               searching,
+			`EncumbranceLabel`:        "overburdened",
+			`EncumbranceColor`:        "red-bold",
+		}
+	}
+}
+
+// skillsScreenFixture is every skill at its longest rank word, two of them
+// recovering, as usercommands.Skills builds it.
+func skillsScreenFixture() any {
+	list, cooldowns, blurbs := map[string]int{}, map[string]int{}, map[string]string{}
+	width := 12
+	for _, tag := range skills.GetAllSkillNames() {
+		name := string(tag)
+		list[name] = 100
+		cooldowns[name] = 0
+		blurbs[name] = skills.SkillBlurb(name)
+		width = max(width, len(name))
+	}
+	list["alchemy"] = 0
+	cooldowns["unarmed-combat"] = 3
+	cooldowns["skullduggery"] = 2
+	return usercommands.SkillsOptions{SkillList: list, SkillCooldowns: cooldowns, SkillBlurbs: blurbs, NameWidth: width}
+}
+
+// cooldownsScreenFixture is the cooldown map at its longest duration word.
+func cooldownsScreenFixture() any {
+	return map[string]int{"special moves": 50, "unarmed-combat": 5, "skullduggery": 2}
+}
+
+// storageScreenFixture is the slot list Storage builds: NameComplex, with a
+// stack count, for long, enchanted, bonus-damage items, past ten slots.
+func storageScreenFixture() any {
+	slots := []string{}
+	for i := 0; i < 11; i++ {
+		itm := screenItem(99300+i, "Basalt-Iron Blade", items.Weapon, 0, false)
+		if i == 4 {
+			itm = screenItem(99300+i, "A Salt-Stiff Page of a Sailor's Telling", items.Weapon, 12, true)
+		}
+		name := itm.NameComplex()
+		if i == 10 {
+			name += ` <ansi fg="uses-left">(x14)</ansi>`
+		}
+		slots = append(slots, name)
+	}
+	return slots
+}
+
+// statusLongestName is a name at the configured maximum (NameSizeMax, 32).
+const statusLongestName = "Bartholomewsworthingtonianfellow"
+
+// A name is never split (owner, 2026-11-10: a single unbreakable word is an
+// accepted exception, like a long URL). With a 32-letter name the status
+// sheet still fits at 42 and 80; at 30 the one line that runs past is the
+// name's own, " .: " and the name, 36 wide.
+func TestStatusWithTheLongestNameRunsPastOnlyOnItsName(t *testing.T) {
+	templateRenderSetup(t)
+	seedCharacterScreenRegistries(t)
+	if len(statusLongestName) != 32 {
+		t.Fatalf("the name is %d letters, not the 32 maximum", len(statusLongestName))
+	}
+	for _, width := range templateWidths {
+		out, err := templates.ProcessAtWidth("character/status", statusScreenFixtureNamed(statusLongestName), width)
+		if err != nil {
+			t.Fatal(err)
+		}
+		t.Logf("at %d the widest line is %d:\n%s", width, templates.WidestLine(out), out)
+		for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
+			got := templates.WidestLine(line)
+			if got <= width {
+				continue
+			}
+			if width != 30 || !strings.Contains(line, statusLongestName) || got != 4+len(statusLongestName) {
+				t.Errorf("at %d a line other than the bare name line is %d wide: %q", width, got, line)
+			}
+		}
+	}
+}
+
+// characterScreenGoldens are the screens whose 80-column output is pinned:
+// each fixture's base fits 80, so the golden is the base as master drew it.
+var characterScreenGoldens = map[string]templateFixture{
+	"character/status":    {setup: seedCharacterScreenRegistries, data: statusScreenFixture},
+	"character/inventory": {data: inventoryScreenFixture(false)},
+	"character/skills":    {data: skillsScreenFixture},
+	"character/cooldowns": {data: cooldownsScreenFixture},
+	"character/storage":   {data: storageScreenFixture},
+}
+
+func characterScreenGoldenPath(name string) string {
+	return testdataPath(filepath.Join("character_screens_80", strings.ReplaceAll(name, "/", "_")+".golden"))
+}
+
+// A desktop sees what it always did: at 80 columns each character screen
+// renders exactly the bytes master drew, line ends aside (a Windows checkout
+// reads the templates with CRLF).
+func TestCharacterScreensKeepTheir80ColumnOutput(t *testing.T) {
+	templateRenderSetup(t)
+	for name, fixture := range characterScreenGoldens {
+		t.Run(name, func(t *testing.T) {
+			if fixture.setup != nil {
+				fixture.setup(t)
+			}
+			out, err := templates.ProcessAtWidth(name, fixture.data(), 80)
+			if err != nil {
+				t.Fatal(err)
+			}
+			got := strings.ReplaceAll(out, "\r\n", "\n")
+			path := characterScreenGoldenPath(name)
+			if *updateCharacterScreens {
+				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
+					t.Fatal(err)
+				}
+				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
+					t.Fatal(err)
+				}
+			}
+			want, err := os.ReadFile(path)
+			if err != nil {
+				t.Fatalf("read golden: %v (record it from master with -update-character-screens)", err)
+			}
+			if w := templates.WidestLine(string(want)); w > 80 {
+				t.Fatalf("the golden is %d wide: its fixture no longer fits 80, so it pins a form, not the base", w)
+			}
+			if got != string(want) {
+				t.Errorf("%s at 80 changed:\n%s", name, lineDiff(string(want), got))
+			}
+		})
+	}
+}
+
+// lineDiff names the first lines that differ.
+func lineDiff(want, got string) string {
+	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
+	var b strings.Builder
+	shown := 0
+	for i := 0; i < max(len(w), len(g)) && shown < 6; i++ {
+		var wl, gl string
+		if i < len(w) {
+			wl = w[i]
+		}
+		if i < len(g) {
+			gl = g[i]
+		}
+		if wl != gl {
+			fmt.Fprintf(&b, "line %d\n want %q\n got  %q\n", i+1, wl, gl)
+			shown++
+		}
+	}
+	return b.String()
+}
diff --git a/internal/guards/template_width_guard_test.go b/internal/guards/template_width_guard_test.go
index 11782d31b..2b4b79b8c 100644
--- a/internal/guards/template_width_guard_test.go
+++ b/internal/guards/template_width_guard_test.go
@@ -1,6 +1,9 @@
 package guards
 
 import (
+	"go/ast"
+	"go/parser"
+	"go/token"
 	"io/fs"
 	"os"
 	"path/filepath"
@@ -60,13 +63,15 @@ var templateClassNames = map[string][]string{
 	// Wrapped to the reader by the caller (actions/roster.go) or the
 	// pipeline (CategoryLogout).
 	"wrapped": {"descriptions/who", "descriptions/ontheground", "player-despawn"},
+	// Fit at every width, held by internal/usercommands' own test
+	// (TestNarrowTemplateScreensFitEveryWidth): the data its commands pass
+	// is of types private to that package.
+	"fits in usercommands": {"character/conditions", "character/quests", "character/title"},
 	// Narrow forms come in plan 3b.
-	"plan 3b": {"character/conditions", "character/cooldowns",
-		"character/description", "character/description-corpse",
-		"character/inventory", "character/inventory-look", "character/pet",
-		"character/quests", "character/skills", "character/skillup",
-		"character/statup", "character/status", "character/storage",
-		"character/title", "descriptions/biome", "descriptions/identify",
+	"plan 3b": {"character/description", "character/description-corpse",
+		"character/inventory-look", "character/pet",
+		"character/skillup", "character/statup",
+		"descriptions/biome", "descriptions/identify",
 		"descriptions/insidecontainer", "descriptions/track",
 		"descriptions/exits", "descriptions/fixtures", "descriptions/room",
 		"descriptions/room-title", "descriptions/rune", "descriptions/sign",
@@ -89,10 +94,15 @@ var templateClassPrefixes = map[string]string{
 }
 
 // templateFixture is data that makes a template as wide as it gets in play.
-// wrapped says the template goes out on a category the pipeline wraps to the
-// reader, so the guard wraps it the same way before measuring.
+// setup, when set, seeds the registries the data reads, restored when the
+// test ends. more are further data the template is rendered with too, such
+// as a second mode of the same screen. wrapped says the template goes out on
+// a category the pipeline wraps to the reader, so the guard wraps it the same
+// way before measuring.
 type templateFixture struct {
+	setup   func(t *testing.T)
 	data    func() any
+	more    []func() any
 	wrapped bool
 }
 
@@ -115,6 +125,12 @@ var templateFits = map[string]templateFixture{
 			})
 	}},
 	"character/questup": {data: func() any { return "You have found a new quest: The Long Road Home." }, wrapped: true},
+	// The character screens (character_screens_test.go).
+	"character/status":    {setup: seedCharacterScreenRegistries, data: statusScreenFixture, more: []func() any{statusScreenLongName}},
+	"character/inventory": {data: inventoryScreenFixture(false), more: []func() any{inventoryScreenFixture(true)}},
+	"character/skills":    {data: skillsScreenFixture},
+	"character/cooldowns": {data: cooldownsScreenFixture},
+	"character/storage":   {data: storageScreenFixture},
 }
 
 // templateClasses names every class that holds name.
@@ -214,6 +230,127 @@ func TestTemplateFormsAreNamedForAKnownWidth(t *testing.T) {
 	}
 }
 
+// Every narrow form closes each colour tag it opens, so no colour runs on
+// into the lines after it. A form is checked as written, tags in template
+// strings included, so a tag opened in one action and closed in another
+// still balances.
+func TestTemplateFormsBalanceTheirColourTags(t *testing.T) {
+	opens, closes := regexp.MustCompile(`<ansi[\s>]`), regexp.MustCompile(`</ansi>`)
+	checked := 0
+	for _, root := range templateFolders(t) {
+		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
+			if err != nil || d.IsDir() || !templateFormFile.MatchString(filepath.ToSlash(path)) {
+				return err
+			}
+			raw, err := os.ReadFile(path)
+			if err != nil {
+				return err
+			}
+			checked++
+			o, c := len(opens.FindAllIndex(raw, -1)), len(closes.FindAllIndex(raw, -1))
+			if o != c {
+				t.Errorf("%s opens %d <ansi> tags and closes %d", path, o, c)
+			}
+			return nil
+		})
+		if err != nil {
+			t.Fatal(err)
+		}
+	}
+	if checked < 20 {
+		t.Fatalf("checked only %d form files; the walk is not reaching them", checked)
+	}
+}
+
+// The "fits in usercommands" class is held, not just named: each template in
+// it has a 42 and a 30 form, and internal/usercommands' narrow_forms_test.go
+// lists it in narrowTemplateScreens, which TestNarrowTemplateScreensFitEveryWidth
+// renders at every width in narrowWidths, which must hold 30 and 42. The second part is read from that file's source,
+// since a test in another package cannot be called from here.
+func TestFitsInUsercommandsClassIsHeld(t *testing.T) {
+	_, forms := templateBases(t)
+	held := templateClassNames["fits in usercommands"]
+	if len(held) == 0 {
+		t.Fatal(`the "fits in usercommands" class is empty; the check proves nothing`)
+	}
+	for _, name := range held {
+		have := map[int]bool{}
+		for _, w := range forms[name] {
+			have[w] = true
+		}
+		for _, w := range []int{42, 30} {
+			if !have[w] {
+				t.Errorf("%s is in \"fits in usercommands\" but has no .w%d.template", name, w)
+			}
+		}
+	}
+
+	path := filepath.Join(repoRoot(t), "internal", "usercommands", "narrow_forms_test.go")
+	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
+	if err != nil {
+		t.Fatal(err)
+	}
+	listed := map[string]bool{}
+	widths := map[string]bool{}
+	renders, atWidths := false, false
+	ast.Inspect(f, func(n ast.Node) bool {
+		switch x := n.(type) {
+		case *ast.ValueSpec:
+			if len(x.Names) == 1 && x.Names[0].Name == "narrowWidths" && len(x.Values) == 1 {
+				if lit, ok := x.Values[0].(*ast.CompositeLit); ok {
+					for _, el := range lit.Elts {
+						if b, ok := el.(*ast.BasicLit); ok {
+							widths[b.Value] = true
+						}
+					}
+				}
+			}
+			if len(x.Names) == 1 && x.Names[0].Name == "narrowTemplateScreens" && len(x.Values) == 1 {
+				if lit, ok := x.Values[0].(*ast.CompositeLit); ok {
+					for _, el := range lit.Elts {
+						if kv, ok := el.(*ast.KeyValueExpr); ok {
+							if key, ok := kv.Key.(*ast.BasicLit); ok {
+								listed[strings.Trim(key.Value, "`\"")] = true
+							}
+						}
+					}
+				}
+			}
+		case *ast.FuncDecl:
+			if x.Name.Name == "TestNarrowTemplateScreensFitEveryWidth" {
+				ast.Inspect(x.Body, func(m ast.Node) bool {
+					if r, ok := m.(*ast.RangeStmt); ok {
+						if id, ok := r.X.(*ast.Ident); ok && id.Name == "narrowTemplateScreens" {
+							renders = true
+						}
+						if id, ok := r.X.(*ast.Ident); ok && id.Name == "narrowWidths" {
+							atWidths = true
+						}
+					}
+					return true
+				})
+			}
+		}
+		return true
+	})
+	if !renders {
+		t.Errorf("%s: TestNarrowTemplateScreensFitEveryWidth no longer ranges over narrowTemplateScreens", path)
+	}
+	if !atWidths {
+		t.Errorf("%s: TestNarrowTemplateScreensFitEveryWidth no longer ranges over narrowWidths", path)
+	}
+	for _, w := range []string{"30", "42"} {
+		if !widths[w] {
+			t.Errorf("%s: narrowWidths does not hold %s, so the fit test never renders the forms there", path, w)
+		}
+	}
+	for _, name := range held {
+		if !listed[name] {
+			t.Errorf("%s is in \"fits in usercommands\" but %s's narrowTemplateScreens does not render it", name, path)
+		}
+	}
+}
+
 // templateRenderSetup renders the live world's templates with its English
 // labels, as the server does.
 func templateRenderSetup(t *testing.T) {
@@ -240,24 +377,31 @@ func TestFittingTemplatesFitAtEveryWidth(t *testing.T) {
 	}
 	sort.Strings(names)
 	for _, name := range names {
-		for _, width := range templateWidths {
-			fixture := templateFits[name]
-			out, err := templates.ProcessAtWidth(name, fixture.data(), width)
-			if err != nil {
-				t.Errorf("%s at %d: %v", name, width, err)
-				continue
-			}
-			if fixture.wrapped {
-				out = messaging.WrapAnsi(out, width)
-			}
-			if strings.TrimSpace(out) == "" {
-				t.Errorf("%s at %d rendered nothing; the fixture proves nothing", name, width)
-				continue
+		fixture := templateFits[name]
+		t.Run(name, func(t *testing.T) {
+			if fixture.setup != nil {
+				fixture.setup(t)
 			}
-			if got := templates.WidestLine(out); got > width {
-				t.Errorf("%s at %d columns has a line %d wide:\n%s", name, width, got, out)
+			for i, data := range append([]func() any{fixture.data}, fixture.more...) {
+				for _, width := range templateWidths {
+					out, err := templates.ProcessAtWidth(name, data(), width)
+					if err != nil {
+						t.Errorf("%s (data %d) at %d: %v", name, i, width, err)
+						continue
+					}
+					if fixture.wrapped {
+						out = messaging.WrapAnsi(out, width)
+					}
+					if strings.TrimSpace(out) == "" {
+						t.Errorf("%s (data %d) at %d rendered nothing; the fixture proves nothing", name, i, width)
+						continue
+					}
+					if got := templates.WidestLine(out); got > width {
+						t.Errorf("%s (data %d) at %d columns has a line %d wide:\n%s", name, i, width, got, out)
+					}
+				}
 			}
-		}
+		})
 	}
 }
 
diff --git a/internal/guards/testdata/character_screens_80/character_cooldowns.golden b/internal/guards/testdata/character_screens_80/character_cooldowns.golden
new file mode 100644
index 000000000..b8151adcd
--- /dev/null
+++ b/internal/guards/testdata/character_screens_80/character_cooldowns.golden
@@ -0,0 +1,6 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Cooldowns</ansi> ──────────────────────────────────────────────────────────────┐
+   <ansi fg="yellow-bold">skullduggery            </ansi> <ansi fg="red">fading fast</ansi>
+   <ansi fg="yellow-bold">special moves           </ansi> <ansi fg="green">well established</ansi>
+   <ansi fg="yellow-bold">unarmed-combat          </ansi> <ansi fg="yellow">briefly active</ansi>
+ └────────────────────────────────────────────────────────────────────────────┘
+
diff --git a/internal/guards/testdata/character_screens_80/character_inventory.golden b/internal/guards/testdata/character_screens_80/character_inventory.golden
new file mode 100644
index 000000000..c2e1eed85
--- /dev/null
+++ b/internal/guards/testdata/character_screens_80/character_inventory.golden
@@ -0,0 +1,28 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Equipment</ansi> ──────────────────────────────────────────────────────────────┐
+   <ansi fg="yellow">Weapon:     </ansi><ansi fg="itemname"><ansi fg="item-flags">[<ansi fg="item-enchanted">e</ansi>]</ansi> Pit Fighter's Serrated Basalt-Iron Blade <ansi fg="item-bonus-damage">+3</ansi></ansi>
+   <ansi fg="yellow">Offhand:    </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Head:       </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Neck:       </ansi><ansi fg="itemname"><ansi fg="item-flags">[<ansi fg="item-enchanted">e</ansi>]</ansi> A Salt-Stiff Page of a Sailor's Telling</ansi>
+   <ansi fg="yellow">Shoulders:  </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Body:       </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Back:       </ansi><ansi fg="itemname">Wayfarer's Bottomless Pack</ansi>
+   <ansi fg="yellow">Belt:       </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Wrist 1:    </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Wrist 2:    </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Gloves:     </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Ring:       </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Ring:       </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Legs:       </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Feet:       </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+   <ansi fg="yellow">Light:      </ansi><ansi fg="itemname">Hooded Lantern</ansi>
+   <ansi fg="yellow">Components: </ansi><ansi fg="itemname"><ansi fg="item-nothing">-nothing-</ansi></ansi>
+ └────────────────────────────────────────────────────────────────────────────┘
+ Carrying: <ansi fg="itemname">A Salt-Stiff Page of a Sailor's Telling (x12)</ansi>, <ansi fg="itemname">Leather Backpack</ansi>, 
+           <ansi fg="itemname">Reinforced Travel Pack</ansi>, <ansi fg="itemname">Strongbox Key</ansi>, <ansi fg="itemname">Pit Fighter's Sword</ansi>, 
+           <ansi fg="itemname">Basalt-Iron Blade</ansi>, <ansi fg="itemname">Spirit Fetish (turned)</ansi>, <ansi fg="itemname">Healing Draught (3)</ansi>, 
+           <ansi fg="itemname">A Stained Page with a Deathbed Word</ansi>
+ Components: <ansi fg="itemname">Iron Ore</ansi> <ansi fg="uses-left">(x24)</ansi>, <ansi fg="itemname">Rough-Cut Moonstone Shard</ansi> <ansi fg="uses-left">(x3)</ansi>, <ansi fg="itemname">Tanned Hide</ansi>
+ Bandolier: <ansi fg="itemname">Minor Potion of Healing</ansi> <ansi fg="uses-left">(x4)</ansi>, <ansi fg="itemname">Conviction Draught</ansi>, 
+            <ansi fg="8">Stamina Draught (turned)</ansi>
+ <ansi fg="red-bold">Encumbrance: [overburdened]</ansi>
+
diff --git a/internal/guards/testdata/character_screens_80/character_skills.golden b/internal/guards/testdata/character_screens_80/character_skills.golden
new file mode 100644
index 000000000..41bdd89c0
--- /dev/null
+++ b/internal/guards/testdata/character_screens_80/character_skills.golden
@@ -0,0 +1,36 @@
+
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Skills</ansi> ─────────────────────────────────────────────────────────────────┐
+   <ansi fg="yellow-bold">alchemy       </ansi> [Unknown]    
+      <ansi fg="black-bold">Brewing potions, salves, and medicines.</ansi>
+   <ansi fg="yellow-bold">bartering     </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Better prices and appraisal when you trade.</ansi>
+   <ansi fg="yellow-bold">blacksmithing </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Forging metal weapons, armor, and tools.</ansi>
+   <ansi fg="yellow-bold">cooking       </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Preparing food and the benefits good meals give.</ansi>
+   <ansi fg="yellow-bold">enchanting    </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Imbuing items with magic.</ansi>
+   <ansi fg="yellow-bold">jewelcrafting </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Rings, pendants, and gemwork.</ansi>
+   <ansi fg="yellow-bold">manifestation </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Summoning companions, charming, and necromancy.</ansi>
+   <ansi fg="yellow-bold">ranged-combat </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Bows, crossbows, and firearms (aimed with Perception).</ansi>
+   <ansi fg="yellow-bold">rhetoric      </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Conviction attacks: taunt and demoralize.</ansi>
+   <ansi fg="yellow-bold">salvage       </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Breaking items down into materials.</ansi>
+   <ansi fg="yellow-bold">search        </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Finding hidden things, and foraging the wild for resources.</ansi>
+   <ansi fg="yellow-bold">skullduggery  </ansi> <ansi fg="white">[grandmaster]</ansi>    <ansi fg="red">Recovering...</ansi>
+      <ansi fg="black-bold">Sneaking, stealing, lockpicking, and ambush.</ansi>
+   <ansi fg="yellow-bold">spellcasting  </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">All magic, offense and defense.</ansi>
+   <ansi fg="yellow-bold">tailoring     </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Crafting cloth and leather goods.</ansi>
+   <ansi fg="yellow-bold">unarmed-combat</ansi> <ansi fg="white">[grandmaster]</ansi>    <ansi fg="red">Recovering...</ansi>
+      <ansi fg="black-bold">Fists, grappling, and unarmed defense.</ansi>
+   <ansi fg="yellow-bold">weapon-combat </ansi> <ansi fg="white">[grandmaster]</ansi>    
+      <ansi fg="black-bold">Melee with weapons: attack and defense.</ansi>
+ └────────────────────────────────────────────────────────────────────────────┘
+
diff --git a/internal/guards/testdata/character_screens_80/character_status.golden b/internal/guards/testdata/character_screens_80/character_status.golden
new file mode 100644
index 000000000..741214f12
--- /dev/null
+++ b/internal/guards/testdata/character_screens_80/character_status.golden
@@ -0,0 +1,22 @@
+
+ <ansi fg="black-bold">.:</ansi> <ansi fg="username">Maximilianovich</ansi> the Novice Juggernaut
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Attributes</ansi> ──────────────────────────────────────────────────────────────┐
+ │ <ansi fg="yellow">Strength:   </ansi>transcendent <ansi fg="yellow">Dexterity:  </ansi>exceptional  <ansi fg="yellow">Perception: </ansi>poor          │
+ │ <ansi fg="yellow">Vitality:   </ansi>transcendent <ansi fg="yellow">Willpower:  </ansi>average      <ansi fg="yellow">Charisma:   </ansi>feeble        │
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Vitals</ansi> ──────────────────────────────────────────────────────────────────┐
+ │ <ansi fg="yellow">Health:     </ansi><ansi fg="green">full</ansi>         <ansi fg="yellow">Stamina:    </ansi><ansi fg="green">full</ansi>         <ansi fg="yellow">Conviction: </ansi><ansi fg="green">full</ansi>          │
+ │ <ansi fg="yellow">HP reserve: </ansi><ansi fg="green">none</ansi>         <ansi fg="yellow">SP reserve: </ansi><ansi fg="green">none</ansi>         <ansi fg="yellow">CP reserve: </ansi><ansi fg="green">none</ansi>          │
+ │ <ansi fg="yellow">Toxicity:   </ansi><ansi fg="green">clear</ansi>        <ansi fg="yellow">Encumbrance: </ansi><ansi fg="green">light</ansi>                                 │
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Defenses</ansi> ────────────────────────────────────────────────────────────────┐
+ │ <ansi fg="yellow">Physical:   </ansi>none         <ansi fg="yellow">Magical:    </ansi>none         <ansi fg="yellow">Conviction: </ansi>none          │
+ └─────────────────────────────────────────────────────────────────────────────┘
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Info</ansi> ────────────────────────────────────────────────────────────────────┐
+ │ <ansi fg="yellow">Species:    </ansi>Human        <ansi fg="yellow">Gold:       </ansi>1,234,567    <ansi fg="yellow">Bank:       </ansi>987,654,321   │
+ └─────────────────────────────────────────────────────────────────────────────┘
+ <ansi fg="yellow">Submission policy:</ansi> subdue    <ansi fg="yellow">Surrender policy:</ansi> tap out below 100% health
+ <ansi fg="red-bold">Broken limb: 12 rounds remaining</ansi>
+ <ansi fg="black-bold">.:</ansi><ansi fg="20">Mutations</ansi><ansi fg="black-bold">:.</ansi>
+   <ansi fg="yellow">reflect-skin-frostbite   </ansi> <ansi fg="magenta">extreme</ansi>
+   <ansi fg="yellow">sustaining-presence      </ansi> <ansi fg="magenta">moderate</ansi>
diff --git a/internal/guards/testdata/character_screens_80/character_storage.golden b/internal/guards/testdata/character_screens_80/character_storage.golden
new file mode 100644
index 000000000..558a0ff55
--- /dev/null
+++ b/internal/guards/testdata/character_screens_80/character_storage.golden
@@ -0,0 +1,16 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">In Storage</ansi> ───────────────────────────────────────────────────────────┐
+   1. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   2. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   3. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   4. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   5. <ansi fg="itemname"><ansi fg="item-flags">[<ansi fg="item-enchanted">e</ansi>]</ansi> A Salt-Stiff Page of a Sailor's Telling <ansi fg="item-bonus-damage">+12</ansi></ansi>
+   6. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   7. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   8. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   9. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   10. <ansi fg="itemname">Basalt-Iron Blade</ansi>
+   11. <ansi fg="itemname">Basalt-Iron Blade <ansi fg="uses-left">(x14)</ansi></ansi>
+ └──────────────────────────────────────────────────────────────────────────┘
+ <ansi fg="command">storage add [item/all]</ansi> to store items. <ansi fg="command">unstore [item/#]</ansi> to retrieve.
+ <ansi fg="command">stow</ansi> deposits all your crafting components here at once.
+
diff --git a/internal/guards/testdata/wrap_live_render.golden b/internal/guards/testdata/wrap_live_render.golden
index 91aea5615..1beff459b 100644
--- a/internal/guards/testdata/wrap_live_render.golden
+++ b/internal/guards/testdata/wrap_live_render.golden
@@ -1,27 +1,26 @@
 === status (CategorySystem) ===
 <ansi fg="system"></ansi>
 <ansi fg="system"> <ansi fg="black-bold">.:</ansi> <ansi fg="username">Fixturewick</ansi> the Novice Warrior</ansi>
-<ansi fg="system">┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Attributes</ansi></ansi>
-<ansi fg="system">──────────────────────────────────────────────────────────────┐</ansi>
-<ansi fg="system">│ <ansi fg="yellow">Strength: </ansi>modest <ansi fg="yellow">Dexterity: </ansi>poor <ansi fg="yellow">Perception: </ansi>feeble │</ansi>
-<ansi fg="system">│ <ansi fg="yellow">Vitality: </ansi>feeble <ansi fg="yellow">Willpower: </ansi>feeble <ansi fg="yellow">Charisma: </ansi>feeble │</ansi>
-<ansi fg="system">└─────────────────────────────────────────────────────────────────────────────┘</ansi>
-<ansi fg="system">┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Vitals</ansi></ansi>
-<ansi fg="system">──────────────────────────────────────────────────────────────────┐</ansi>
-<ansi fg="system">│ <ansi fg="yellow">Health: </ansi><ansi fg="green">full</ansi> <ansi fg="yellow">Stamina: </ansi><ansi fg="green">full</ansi> <ansi fg="yellow">Conviction: </ansi><ansi fg="green">full</ansi> │</ansi>
-<ansi fg="system">│ <ansi fg="yellow">HP reserve: </ansi><ansi fg="green">none</ansi> <ansi fg="yellow">SP reserve: </ansi><ansi fg="green">none</ansi> <ansi fg="yellow">CP reserve: </ansi><ansi fg="green">none</ansi> │</ansi>
-<ansi fg="system">│ <ansi fg="yellow">Toxicity: </ansi><ansi fg="green">clear</ansi> <ansi fg="yellow">Encumbrance: </ansi><ansi fg="green">light</ansi> │</ansi>
-<ansi fg="system">└─────────────────────────────────────────────────────────────────────────────┘</ansi>
-<ansi fg="system">┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Defenses</ansi></ansi>
-<ansi fg="system">────────────────────────────────────────────────────────────────┐</ansi>
-<ansi fg="system">│ <ansi fg="yellow">Physical: </ansi>none <ansi fg="yellow">Magical: </ansi>none <ansi fg="yellow">Conviction: </ansi>none │</ansi>
-<ansi fg="system">└─────────────────────────────────────────────────────────────────────────────┘</ansi>
-<ansi fg="system">┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Info</ansi></ansi>
-<ansi fg="system">────────────────────────────────────────────────────────────────────┐</ansi>
-<ansi fg="system">│ <ansi fg="yellow">Species: </ansi>Ghostly Spirit<ansi fg="yellow">Gold: </ansi>250 <ansi fg="yellow">Bank: </ansi>100 │</ansi>
-<ansi fg="system">└─────────────────────────────────────────────────────────────────────────────┘</ansi>
-<ansi fg="system"><ansi fg="yellow">Submission policy:</ansi> subdue <ansi fg="yellow">Surrender policy:</ansi> tap out</ansi>
-<ansi fg="system">below 15% health.</ansi>
+<ansi fg="system">── <ansi fg="black-bold">.:</ansi><ansi fg="20">Attributes</ansi> ──────────────────────────</ansi>
+<ansi fg="system"> <ansi fg="yellow">Strength:    </ansi>modest  <ansi fg="yellow">Dexterity:   </ansi>poor</ansi>
+<ansi fg="system"> <ansi fg="yellow">Perception:  </ansi>feeble  <ansi fg="yellow">Vitality:    </ansi>feeble</ansi>
+<ansi fg="system"> <ansi fg="yellow">Willpower:   </ansi>feeble  <ansi fg="yellow">Charisma:    </ansi>feeble</ansi>
+<ansi fg="system">── <ansi fg="black-bold">.:</ansi><ansi fg="20">Vitals</ansi> ──────────────────────────────</ansi>
+<ansi fg="system"> <ansi fg="yellow">Health:      </ansi><ansi fg="green">full</ansi>    <ansi fg="yellow">HP reserve:  </ansi><ansi fg="green">none</ansi></ansi>
+<ansi fg="system"> <ansi fg="yellow">Stamina:     </ansi><ansi fg="green">full</ansi>    <ansi fg="yellow">SP reserve:  </ansi><ansi fg="green">none</ansi></ansi>
+<ansi fg="system"> <ansi fg="yellow">Conviction:  </ansi><ansi fg="green">full</ansi>    <ansi fg="yellow">CP reserve:  </ansi><ansi fg="green">none</ansi></ansi>
+<ansi fg="system"> <ansi fg="yellow">Toxicity:    </ansi><ansi fg="green">clear</ansi>   <ansi fg="yellow">Encumbrance: </ansi><ansi fg="green">light</ansi></ansi>
+<ansi fg="system">── <ansi fg="black-bold">.:</ansi><ansi fg="20">Defenses</ansi> ────────────────────────────</ansi>
+<ansi fg="system"> <ansi fg="yellow">Physical:    </ansi>none    <ansi fg="yellow">Magical:     </ansi>none</ansi>
+<ansi fg="system"> <ansi fg="yellow">Conviction:  </ansi>none</ansi>
+<ansi fg="system">── <ansi fg="black-bold">.:</ansi><ansi fg="20">Info</ansi> ────────────────────────────────</ansi>
+<ansi fg="system"> <ansi fg="yellow">Species:     </ansi>Ghostly Spirit</ansi>
+<ansi fg="system"> <ansi fg="yellow">Gold:        </ansi>250</ansi>
+<ansi fg="system"> <ansi fg="yellow">Bank:        </ansi>100</ansi>
+<ansi fg="system">──────────────────────────────────────────</ansi>
+<ansi fg="system"> <ansi fg="yellow">Submission policy:</ansi> subdue</ansi>
+<ansi fg="system"> <ansi fg="yellow">Surrender policy:</ansi> tap out below 15%</ansi>
+<ansi fg="system">   health.</ansi>
 === item list (CategorySystem) ===
 <ansi fg="system"> <ansi fg="red-bold">1</ansi>. <ansi fg="yellow-bold">Iron Sword</ansi>       <ansi fg="red-bold">2</ansi>. <ansi fg="yellow-bold">Healing Draught</ansi> </ansi>
 <ansi fg="system"> <ansi fg="red-bold">3</ansi>. <ansi fg="yellow-bold">Leather Boots</ansi>    <ansi fg="red-bold">4</ansi>. <ansi fg="yellow-bold">Silver Ring</ansi>     </ansi>
diff --git a/internal/templates/context.md b/internal/templates/context.md
index 156f140f9..cd9786d39 100644
--- a/internal/templates/context.md
+++ b/internal/templates/context.md
@@ -148,6 +148,19 @@ Cached template with metadata for efficient reuse and cache invalidation.
   reader: `{{ repeat "*" (sub screenwidth 1) }}`,
   `{{ splitstring .X (sub screenwidth 9) "  " }}`. `min` is there for caps:
   `(min 67 (sub screenwidth 12))`.
+- `wraptext text width prefix` wraps with `messaging.WrapAnsi`, the game's
+  one wrapper, and starts each later line with prefix. It breaks only at
+  spaces and reopens a colour tag on each line, where `splitstring`
+  (`util.SplitString`) can leave an opening bracket at the end of a line
+  ("Missing Person (" / "following)"). Narrow forms use it.
+- `visiblewidth s` is the columns s takes, colour tags not counted, so a form
+  can test whether a coloured value (a `vitalQuality` word, a `NameComplex`)
+  fits beside its label.
+- The character screens' forms (`character/status`, `inventory`, `skills`,
+  `conditions`, `cooldowns`, `quests`, `storage`, `title`, each `.w42` and
+  `.w30`) are drawn to their own width, not to `screenwidth`: a 42 form drawn
+  to a 35-column reader would fit and hide the 30 form. Shared layout
+  pieces live in `{{ define }}` blocks inside each form file.
 - A `.template` may have narrow forms beside it, named for the width they are
   drawn to: `tables/generic.w40.template`, `tables/generic.w30.template`. Only
   widths in `FormWidths` (42, 40, 30) are read. When the base output's widest
diff --git a/internal/templates/templatesfunctions.go b/internal/templates/templatesfunctions.go
index 1b773cea9..d372322b7 100644
--- a/internal/templates/templatesfunctions.go
+++ b/internal/templates/templatesfunctions.go
@@ -16,6 +16,7 @@ import (
 	"github.com/GoMudEngine/GoMud/internal/gametime"
 	"github.com/GoMudEngine/GoMud/internal/language"
 	"github.com/GoMudEngine/GoMud/internal/mapper"
+	"github.com/GoMudEngine/GoMud/internal/messaging"
 	"github.com/GoMudEngine/GoMud/internal/mobs"
 	"github.com/GoMudEngine/GoMud/internal/mutations"
 	"github.com/GoMudEngine/GoMud/internal/skills"
@@ -133,7 +134,11 @@ var (
 		// (forms.go). Rendered with no reader, it is FullWidth.
 		"screenwidth": func() int { return FullWidth },
 		"min":         func(a, b int) int { return min(a, b) },
-		"ansiparse":   TplAnsiParse,
+		// visiblewidth is the columns s takes on screen, colour tags not
+		// counted, so a narrow form can ask whether a coloured value fits.
+		"visiblewidth": util.VisibleWidth,
+		"wraptext":     wrapText,
+		"ansiparse":    TplAnsiParse,
 		"conditionname": func(conditionId int) string {
 			conditionSpec := conditions.GetConditionSpec(conditionId)
 			if conditionSpec == nil {
@@ -458,6 +463,20 @@ var (
 	}
 )
 
+// wrapText wraps s to width columns with messaging.WrapAnsi, the game's one
+// wrapper: it breaks only at spaces, so "(following)" never loses its
+// bracket to the line above as splitstring's can, and it reopens a colour
+// tag on each line. Every line after the first starts with prefix.
+//
+//	{{ wraptext .Description 36 "      " }}
+func wrapText(s string, width int, prefix ...string) string {
+	pad := ""
+	if len(prefix) > 0 {
+		pad = prefix[0]
+	}
+	return strings.ReplaceAll(messaging.WrapAnsi(s, width), "\n", "\n"+pad)
+}
+
 // Usage:
 //
 //	{{ implode .items "," }}
diff --git a/internal/usercommands/companion.go b/internal/usercommands/companion.go
index f74bfbc38..815e1894a 100644
--- a/internal/usercommands/companion.go
+++ b/internal/usercommands/companion.go
@@ -13,6 +13,7 @@ import (
 	"github.com/GoMudEngine/GoMud/internal/refusal"
 	"github.com/GoMudEngine/GoMud/internal/rooms"
 	"github.com/GoMudEngine/GoMud/internal/users"
+	"github.com/GoMudEngine/GoMud/internal/util"
 )
 
 // Companion handles the `companion` command in four forms:
@@ -33,52 +34,8 @@ func Companion(rest string, user *users.UserRecord,
 			return true, nil
 		}
 
-		user.SendText(messaging.CategorySystem, `<ansi fg="yellow">━━━ Companions ━━━</ansi>`)
-		for _, comp := range companions {
-			mob := mobs.GetInstance(comp.InstanceId)
-
-			assistStr := "off"
-			if comp.AutoAssist {
-				assistStr = "on"
-			}
-
-			if mob == nil {
-				user.SendText(messaging.CategorySystem, fmt.Sprintf(
-					`  <ansi fg="mobname">%s</ansi> [%s] `+
-						`<ansi fg="239">[offline]</ansi> `+
-						`(assist: %s)`,
-					comp.Name, comp.SourceType, assistStr,
-				))
-				continue
-			}
-
-			hpBar := users.RenderVitalBar(
-				mob.Character.Health,
-				mob.Character.HealthMax.Value,
-				0,
-			)
-			spBar := users.RenderVitalBar(
-				mob.Character.Stamina,
-				mob.Character.StaminaMax.Value,
-				0,
-			)
-			cpBar := users.RenderVitalBar(
-				mob.Character.Conviction,
-				mob.Character.ConvictionMax.Value,
-				0,
-			)
-
-			user.SendText(messaging.CategorySystem, fmt.Sprintf(
-				`  <ansi fg="mobname">%s</ansi> [%s] (assist: %s)`,
-				comp.Name, comp.SourceType, assistStr,
-			))
-			user.SendText(messaging.CategorySystem, fmt.Sprintf(
-				`    HP: %s  SP: %s  CP: %s`,
-				hpBar, spBar, cpBar,
-			))
-			if mutList := formatCompanionMutations(mob.Character.Mutations); mutList != "" {
-				user.SendText(messaging.CategorySystem, fmt.Sprintf(`    Mutations: %s`, mutList))
-			}
+		for _, msg := range companionList(companions, user.WrapWidth()) {
+			user.SendText(messaging.CategorySystem, msg)
 		}
 		return true, nil
 	}
@@ -170,46 +127,287 @@ func Companion(rest string, user *users.UserRecord,
 		assistStr = "on"
 	}
 
-	user.SendText(messaging.CategorySystem, fmt.Sprintf(
-		`<ansi fg="yellow">━━━ %s [%s] ━━━</ansi>`,
-		comp.Name, comp.SourceType,
-	))
+	for _, line := range companionDetail(comp, mob, assistStr, user.WrapWidth()) {
+		user.SendText(line.cat, line.text)
+	}
 
-	if mob == nil {
-		user.SendText(messaging.CategorySystem, `  This companion is not currently present.`)
-		user.SendText(messaging.CategoryScreen, fmt.Sprintf(`  Auto-Assist: %s`, assistStr))
-		return true, nil
+	return true, nil
+}
+
+// Narrow forms of the companion screens (web client base spec, section 2),
+// picked as templates.Process picks a template's forms: the 80-column screen
+// goes to any reader it fits, as it always did. A reader it does not fit gets
+// the form drawn to 42 columns, or to 30 below that: the heading rule runs
+// the full width, the bars stack one per line, and words break only between
+// whole names. Every form shows everything the 80-column screen does.
+
+// companionNarrowWidth is the width to draw a companion screen to for a
+// reader width columns wide: 0 when the 80-column lines fit, else 42, or 30
+// (or the reader's width, if narrower still).
+func companionNarrowWidth(width int, lines ...string) int {
+	fits := true
+	for _, line := range lines {
+		if util.VisibleWidth(line) > width {
+			fits = false
+			break
+		}
 	}
+	switch {
+	case fits:
+		return 0
+	case width >= 42:
+		return 42
+	default:
+		return min(width, 30)
+	}
+}
 
-	// Vitals descriptions (no hard numbers)
-	user.SendText(messaging.CategoryScreen, fmt.Sprintf(
-		`  Health: %-10s Stamina: %-10s Conviction: %s`,
-		vitalDesc(mob.Character.Health, mob.Character.HealthMax.Value),
-		vitalDesc(mob.Character.Stamina, mob.Character.StaminaMax.Value),
-		vitalDesc(mob.Character.Conviction, mob.Character.ConvictionMax.Value),
-	))
+// packCompanionUnits lays units out in lines of at most width columns: the
+// first line indented by first, the rest by rest, breaking only between
+// units. A unit goes on the line in hand when it fits there (the first line
+// measured from its own indent), else on a new line. Only a unit too long
+// for a whole new line is wrapped between its words (messaging.WrapAnsi).
+func packCompanionUnits(units []string, first, rest, width int) []string {
+	lines := []string{}
+	line, used := strings.Repeat(" ", first), first
+	empty := true
+	newLine := func() {
+		lines = append(lines, line)
+		line, used, empty = strings.Repeat(" ", rest), rest, true
+	}
+	place := func(word string) {
+		w := util.VisibleWidth(word)
+		sep := 1
+		if empty {
+			sep = 0
+		}
+		if used+sep+w > width && !empty {
+			newLine()
+			sep = 0
+		}
+		line += strings.Repeat(" ", sep) + word
+		used += sep + w
+		empty = false
+	}
+	for _, unit := range units {
+		w := util.VisibleWidth(unit)
+		sep := 1
+		if empty {
+			sep = 0
+		}
+		if used+sep+w <= width || w <= width-rest {
+			place(unit)
+			continue
+		}
+		if !empty {
+			newLine()
+		}
+		for _, part := range strings.Split(messaging.WrapAnsi(unit, width-rest), "\n") {
+			place(part)
+		}
+	}
+	lines = append(lines, line)
+	for i := range lines {
+		lines[i] = strings.TrimRight(lines[i], " ")
+	}
+	return lines
+}
 
-	// Stat descriptions using the same tiers as the player status command
-	user.SendText(messaging.CategoryScreen, fmt.Sprintf(
-		`  <ansi fg="yellow">Strength:</ansi>   %-13s <ansi fg="yellow">Dexterity:</ansi>  %-13s <ansi fg="yellow">Perception:</ansi> %s`,
-		statQualityDesc(mob.Character.Stats.Strength.ValueAdj),
-		statQualityDesc(mob.Character.Stats.Dexterity.ValueAdj),
-		statQualityDesc(mob.Character.Stats.Perception.ValueAdj),
-	))
-	user.SendText(messaging.CategoryScreen, fmt.Sprintf(
-		`  <ansi fg="yellow">Vitality:</ansi>   %-13s <ansi fg="yellow">Willpower:</ansi>  %-13s <ansi fg="yellow">Charisma:</ansi>   %s`,
-		statQualityDesc(mob.Character.Stats.Vitality.ValueAdj),
-		statQualityDesc(mob.Character.Stats.Willpower.ValueAdj),
-		statQualityDesc(mob.Character.Stats.Charisma.ValueAdj),
-	))
+// companionRule is a heading on a rule that spans width: "━━━ title "
+// filled with "━" to width when at least three fit after it. A longer title
+// goes on its own line or lines, broken only between its parts (a name is
+// one part), and a full-width rule follows.
+func companionRule(width int, parts ...string) string {
+	title := strings.Join(parts, ` `)
+	head := `━━━ ` + title + ` `
+	if util.VisibleWidth(head)+3 <= width {
+		return `<ansi fg="yellow">` + head + strings.Repeat(`━`, width-util.VisibleWidth(head)) + `</ansi>`
+	}
+	lines := packCompanionUnits(parts, 0, 0, width)
+	return `<ansi fg="yellow">` + strings.Join(lines, "\n") + "\n" + strings.Repeat(`━`, width) + `</ansi>`
+}
 
-	if mutList := formatCompanionMutations(mob.Character.Mutations); mutList != "" {
-		user.SendText(messaging.CategoryScreen, fmt.Sprintf(`  <ansi fg="yellow">Mutations:</ansi>  %s`, mutList))
+// companionMutationLines is "Mutations: a, b, c" at indent, each entry (a
+// name with its level and comma) kept whole. The entries hang under the first
+// one when that leaves 20 columns and every entry fits there; else they sit
+// 2 past the label; else, when an entry is too long even for that, under the
+// label itself. Only an entry longer than a whole line at the label's indent
+// is broken.
+func companionMutationLines(label, mutList string, indent, width int) []string {
+	names := strings.Split(mutList, ", ")
+	for i := range names[:len(names)-1] {
+		names[i] += ","
 	}
+	longest := 0
+	for _, name := range names {
+		longest = max(longest, util.VisibleWidth(name))
+	}
+	hang := indent
+	for _, try := range []int{indent + util.VisibleWidth(label) + 1, indent + 2} {
+		if width-try >= longest && (try == indent+2 || width-try >= 20) {
+			hang = try
+			break
+		}
+	}
+	return packCompanionUnits(append([]string{label}, names...), indent, hang, width)
+}
 
-	user.SendText(messaging.CategoryScreen, fmt.Sprintf(`  Auto-Assist: %s`, assistStr))
+// companionList is the messages of the companion list for a reader width
+// columns wide.
+func companionList(companions []characters.CompanionInfo, width int) []string {
+	type row struct {
+		name, kind, offline, assist string
+		mob                         *mobs.Mob
+		bars                        [3]string
+		mutList                     string
+	}
+	rows := make([]row, 0, len(companions))
+	wide := []string{`━━━ Companions ━━━`}
+	for _, comp := range companions {
+		assistStr := "off"
+		if comp.AutoAssist {
+			assistStr = "on"
+		}
+		r := row{name: `<ansi fg="mobname">` + comp.Name + `</ansi>`, kind: `[` + string(comp.SourceType) + `]`,
+			assist: `(assist: ` + assistStr + `)`, mob: mobs.GetInstance(comp.InstanceId)}
+		if r.mob == nil {
+			r.offline = `<ansi fg="239">[offline]</ansi>`
+			wide = append(wide, fmt.Sprintf(`  %s %s %s %s`, r.name, r.kind, r.offline, r.assist))
+		} else {
+			c := r.mob.Character
+			r.bars = [3]string{
+				users.RenderVitalBar(c.Health, c.HealthMax.Value, 0),
+				users.RenderVitalBar(c.Stamina, c.StaminaMax.Value, 0),
+				users.RenderVitalBar(c.Conviction, c.ConvictionMax.Value, 0),
+			}
+			r.mutList = formatCompanionMutations(c.Mutations)
+			wide = append(wide, fmt.Sprintf(`  %s %s %s`, r.name, r.kind, r.assist),
+				fmt.Sprintf(`    HP: %s  SP: %s  CP: %s`, r.bars[0], r.bars[1], r.bars[2]))
+			if r.mutList != "" {
+				wide = append(wide, `    Mutations: `+r.mutList)
+			}
+		}
+		rows = append(rows, r)
+	}
 
-	return true, nil
+	// The pipeline has always wrapped a long mutations line at 80 columns,
+	// so only the laid-out lines decide there; below 80 every line does.
+	measured := wide
+	if width >= users.MaxAutoWrapWidth {
+		measured = []string{}
+		for _, line := range wide {
+			if !strings.HasPrefix(line, `    Mutations: `) {
+				measured = append(measured, line)
+			}
+		}
+	}
+	formWidth := companionNarrowWidth(width, measured...)
+	if formWidth == 0 {
+		// As it always was: the heading, then a name line, a bars line
+		// and a mutations line per companion.
+		msgs := []string{`<ansi fg="yellow">━━━ Companions ━━━</ansi>`}
+		return append(msgs, wide[1:]...)
+	}
+
+	msgs := []string{companionRule(formWidth, `Companions`)}
+	for _, r := range rows {
+		units := []string{r.name, r.kind}
+		if r.offline != "" {
+			units = append(units, r.offline)
+		}
+		msgs = append(msgs, strings.Join(packCompanionUnits(append(units, r.assist), 2, 4, formWidth), "\n"))
+		if r.mob == nil {
+			continue
+		}
+		msgs = append(msgs, `    HP: `+r.bars[0], `    SP: `+r.bars[1], `    CP: `+r.bars[2])
+		if r.mutList != "" {
+			msgs = append(msgs, strings.Join(companionMutationLines(`Mutations:`, r.mutList, 4, formWidth), "\n"))
+		}
+	}
+	return msgs
+}
+
+// companionLine is one message of the companion detail view.
+type companionLine struct {
+	cat  messaging.Category
+	text string
+}
+
+// companionDetail is the detail view of one companion for a reader width
+// columns wide. mob is nil when the companion is not present.
+func companionDetail(comp *characters.CompanionInfo, mob *mobs.Mob, assistStr string, width int) []companionLine {
+	heading := fmt.Sprintf(`<ansi fg="yellow">━━━ %s [%s] ━━━</ansi>`, comp.Name, comp.SourceType)
+	assist := fmt.Sprintf(`  Auto-Assist: %s`, assistStr)
+	if mob == nil {
+		absent := `  This companion is not currently present.`
+		formWidth := companionNarrowWidth(width, heading, absent, assist)
+		if formWidth == 0 {
+			return []companionLine{{messaging.CategorySystem, heading},
+				{messaging.CategorySystem, absent}, {messaging.CategoryScreen, assist}}
+		}
+		return []companionLine{{messaging.CategorySystem, companionDetailRule(comp, formWidth)},
+			{messaging.CategorySystem, strings.Join(packCompanionUnits(strings.Fields(absent), 2, 2, formWidth), "\n")},
+			{messaging.CategoryScreen, assist}}
+	}
+
+	c := mob.Character
+	vitals := [3]string{vitalDesc(c.Health, c.HealthMax.Value), vitalDesc(c.Stamina, c.StaminaMax.Value),
+		vitalDesc(c.Conviction, c.ConvictionMax.Value)}
+	stats := [6]string{statQualityDesc(c.Stats.Strength.ValueAdj), statQualityDesc(c.Stats.Dexterity.ValueAdj),
+		statQualityDesc(c.Stats.Perception.ValueAdj), statQualityDesc(c.Stats.Vitality.ValueAdj),
+		statQualityDesc(c.Stats.Willpower.ValueAdj), statQualityDesc(c.Stats.Charisma.ValueAdj)}
+	mutList := formatCompanionMutations(c.Mutations)
+
+	wide := []string{heading,
+		// Vitals descriptions (no hard numbers)
+		fmt.Sprintf(`  Health: %-10s Stamina: %-10s Conviction: %s`, vitals[0], vitals[1], vitals[2]),
+		// Stat descriptions using the same tiers as the player status command
+		fmt.Sprintf(`  <ansi fg="yellow">Strength:</ansi>   %-13s <ansi fg="yellow">Dexterity:</ansi>  %-13s <ansi fg="yellow">Perception:</ansi> %s`,
+			stats[0], stats[1], stats[2]),
+		fmt.Sprintf(`  <ansi fg="yellow">Vitality:</ansi>   %-13s <ansi fg="yellow">Willpower:</ansi>  %-13s <ansi fg="yellow">Charisma:</ansi>   %s`,
+			stats[3], stats[4], stats[5]),
+	}
+	if mutList != "" {
+		wide = append(wide, fmt.Sprintf(`  <ansi fg="yellow">Mutations:</ansi>  %s`, mutList))
+	}
+	wide = append(wide, assist)
+
+	formWidth := companionNarrowWidth(width, wide...)
+	if formWidth == 0 {
+		lines := []companionLine{{messaging.CategorySystem, wide[0]}}
+		for _, text := range wide[1:] {
+			lines = append(lines, companionLine{messaging.CategoryScreen, text})
+		}
+		return lines
+	}
+
+	// One label and value per line, the label column 13 wide, as the
+	// status sheet's 30-column form lists them.
+	pair := func(label, value string, yellow bool) string {
+		padded := fmt.Sprintf(`%-13s`, label)
+		if yellow {
+			padded = `<ansi fg="yellow">` + padded + `</ansi>`
+		}
+		return `  ` + padded + value
+	}
+	lines := []companionLine{{messaging.CategorySystem, companionDetailRule(comp, formWidth)}}
+	for i, label := range []string{`Health:`, `Stamina:`, `Conviction:`} {
+		lines = append(lines, companionLine{messaging.CategoryScreen, pair(label, vitals[i], false)})
+	}
+	for i, label := range []string{`Strength:`, `Dexterity:`, `Perception:`, `Vitality:`, `Willpower:`, `Charisma:`} {
+		lines = append(lines, companionLine{messaging.CategoryScreen, pair(label, stats[i], true)})
+	}
+	if mutList != "" {
+		lines = append(lines, companionLine{messaging.CategoryScreen,
+			strings.Join(companionMutationLines(`<ansi fg="yellow">Mutations:</ansi>  `, mutList, 2, formWidth), "\n")})
+	}
+	return append(lines, companionLine{messaging.CategoryScreen, pair(`Auto-Assist:`, assistStr, false)})
+}
+
+// companionDetailRule is the detail view's heading at formWidth: the name
+// and kind on a full-width rule, as companionRule draws it.
+func companionDetailRule(comp *characters.CompanionInfo, formWidth int) string {
+	return companionRule(formWidth, comp.Name, `[`+string(comp.SourceType)+`]`)
 }
 
 // formatCompanionMutations renders a companion's mutation list as a
diff --git a/internal/usercommands/context.md b/internal/usercommands/context.md
index 63dea8e7e..37d10144a 100644
--- a/internal/usercommands/context.md
+++ b/internal/usercommands/context.md
@@ -477,6 +477,25 @@ Type more of its name." (a numbered search such as `2.the` is taken as typed).
 "Set aside. Talk to <giver> to take it up again." and no bar; plain `quests`
 does not show them.
 
+### Narrow forms of the character screens (web client base plan 3b)
+
+`character/quests` gets a `questsScreen`. A `questRecord`'s percentage and
+bar are set only by `withProgress(done)`, which keeps the completion too, so
+the 30-column form draws the same bar 20 cells wide (`BarFullAt`,
+`BarEmptyAt`; a set-aside row has no bar at any width) and it never
+disagrees with the percentage. `TestQuestProgressHasOneSource` fails any
+other code in the package that sets those fields. `character/title` gets a `titleScreen` of `statBar`s. `companion.go`
+lays out its own narrow forms (`companionList`, `companionDetail`): the
+80-column screen goes to any reader it fits, else the form drawn to 42 or 30
+columns, chosen by `companionNarrowWidth` as `templates.Process` chooses a
+template's forms. Every heading in a companion form sits on a rule the
+full form width (`companionRule`); a heading too long for that goes on its
+own line or lines with a full-width rule under it. `narrow_forms_test.go` renders conditions, quests, title
+and companion at 30, 42 and 80 and pins their 80-column output to goldens
+recorded from master (`testdata/narrow_forms_80/`, `-update-narrow-forms`).
+The other character screens are held in `internal/guards`
+(`character_screens_test.go`).
+
 ## Message categories (messaging M7 slice 4, #371)
 
 `CategorySystem` keeps reports, tables, help and confirmations of the acting
diff --git a/internal/usercommands/narrow_forms_test.go b/internal/usercommands/narrow_forms_test.go
new file mode 100644
index 000000000..bf27a796f
--- /dev/null
+++ b/internal/usercommands/narrow_forms_test.go
@@ -0,0 +1,406 @@
+package usercommands
+
+import (
+	"flag"
+	"fmt"
+	"go/ast"
+	"go/parser"
+	"go/token"
+	"go/types"
+	"math"
+	"os"
+	"path/filepath"
+	"strings"
+	"testing"
+	"unicode/utf8"
+
+	"github.com/GoMudEngine/GoMud/internal/characters"
+	"github.com/GoMudEngine/GoMud/internal/conditions"
+	"github.com/GoMudEngine/GoMud/internal/events"
+	"github.com/GoMudEngine/GoMud/internal/mobs"
+	"github.com/GoMudEngine/GoMud/internal/mutations"
+	"github.com/GoMudEngine/GoMud/internal/templates"
+	"github.com/GoMudEngine/GoMud/internal/util"
+	"github.com/stretchr/testify/require"
+)
+
+// The narrow forms of the screens whose data types are private to this
+// package (plan 3b): character/conditions, character/quests and
+// character/title, and the companion list built here in Go. Each is rendered
+// at 30, 42 and 80 columns with data as wide as it gets in play, and its
+// 80-column output is pinned to what master drew. internal/guards holds the
+// other character screens the same way (character_screens_test.go).
+
+var updateNarrowForms = flag.Bool("update-narrow-forms", false,
+	"rewrite internal/usercommands/testdata/narrow_forms_80/*.golden from the current output")
+
+// narrowWidths are the widths every screen is rendered at: the narrowest
+// screen, an upright phone, and a desktop.
+var narrowWidths = []int{30, 42, 80}
+
+// conditionsWide is the conditions list at its widest: the longest shipped
+// name with a stack count, long descriptions, every duration word.
+func conditionsWide() []conditionEntry {
+	return []conditionEntry{
+		{Name: "Chrysalis Regeneration (99)", Description: "Your body knits itself back together, stronger in the places it broke.", RoundsLeft: 50},
+		{Name: "Minor Potion Healing", Description: "A small healing effect is working on you.", RoundsLeft: 20},
+		{Name: "Death's Shadow", Description: "A dark shadow clings to you, dulling your senses and weakening your body.", RoundsLeft: 2},
+		{Name: "Stoneskin", Description: "Skin like rock.", Permanent: true},
+	}
+}
+
+// questRow is a quest list row as activeQuestRecord builds it.
+func questRow(id int, name string, repeatable, following bool, desc, progress, guide string, done float64) questRecord {
+	return questRecord{Id: id, Name: name, Repeatable: repeatable, Following: following,
+		Description: desc, Progress: progress, Guide: guide}.withProgress(done)
+}
+
+// The quest bar and the percentage beside it come from one number: only
+// withProgress writes Completion, BarFull, BarEmpty and done (no other
+// code in this package's non-test files sets them), and the bar it draws at
+// any width agrees with the percentage.
+//
+// The package is type-checked from source (imports stand in as empty
+// packages, and the errors that causes are ignored), so a literal is known to
+// be a questRecord by its type, not its spelling: an elided element of a
+// []questRecord, a pointer literal, an alias and a package-level var are all
+// caught, as is a positional literal. An assignment or ++ to one of those
+// field names is flagged whatever its receiver, since no other type in the
+// package has fields so named.
+func TestQuestProgressHasOneSource(t *testing.T) {
+	for _, finding := range questProgressWriters(t, ".") {
+		t.Error(finding)
+	}
+
+	for _, done := range []float64{0, 1.0 / 8, 1.0 / 7, 0.25, 1.0 / 3, 0.5, 0.999, 1} {
+		r := questRecord{}.withProgress(done)
+		require.Equal(t, fmt.Sprintf(`%d%%`, int(math.Floor(done*100))), r.Completion)
+		require.Equal(t, r.BarFull, r.BarFullAt(questBarCells), "done %v", done)
+		require.Equal(t, r.BarEmpty, r.BarEmptyAt(questBarCells), "done %v", done)
+		require.Equal(t, int(math.Floor(20*done)), utf8.RuneCountInString(r.BarFullAt(20)), "done %v", done)
+	}
+	require.Empty(t, questRecord{Name: "set aside"}.BarFullAt(20), "a row with no bar has none at any width")
+}
+
+// emptyImporter answers every import with an empty, complete package, so
+// the package under test type-checks from its own source alone.
+type emptyImporter struct{}
+
+func (emptyImporter) Import(path string) (*types.Package, error) {
+	pkg := types.NewPackage(path, filepath.Base(path))
+	pkg.MarkComplete()
+	return pkg, nil
+}
+
+// questProgressWriters lists every place in dir's non-test Go files, outside
+// questRecord.withProgress, that sets a questRecord's progress fields.
+func questProgressWriters(t *testing.T, dir string) []string {
+	t.Helper()
+	fields := map[string]bool{"Completion": true, "BarFull": true, "BarEmpty": true, "done": true}
+	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
+	require.NoError(t, err)
+	fset := token.NewFileSet()
+	files := []*ast.File{}
+	for _, path := range paths {
+		if strings.HasSuffix(path, "_test.go") {
+			continue
+		}
+		f, err := parser.ParseFile(fset, path, nil, 0)
+		require.NoError(t, err)
+		files = append(files, f)
+	}
+	require.Greater(t, len(files), 100, "the scan must reach the package's files")
+
+	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
+	conf := types.Config{Importer: emptyImporter{}, Error: func(error) {}}
+	pkg, _ := conf.Check("usercommands", fset, files, info)
+	obj := pkg.Scope().Lookup("questRecord")
+	require.NotNil(t, obj, "questRecord must type-check")
+	isRecord := func(e ast.Expr) bool {
+		tv, ok := info.Types[e]
+		if !ok || tv.Type == nil {
+			return false
+		}
+		typ := tv.Type
+		if p, ok := typ.(*types.Pointer); ok {
+			typ = p.Elem()
+		}
+		return types.Identical(typ, obj.Type())
+	}
+
+	var found []string
+	for _, f := range files {
+		ast.Inspect(f, func(n ast.Node) bool {
+			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "withProgress" && fn.Recv != nil {
+				return false
+			}
+			flag := func(pos token.Pos, what string) {
+				found = append(found, fmt.Sprintf("%s: %s; use withProgress", fset.Position(pos), what))
+			}
+			switch x := n.(type) {
+			case *ast.CompositeLit:
+				if !isRecord(x) {
+					return true
+				}
+				for _, el := range x.Elts {
+					kv, ok := el.(*ast.KeyValueExpr)
+					if !ok {
+						flag(el.Pos(), "a positional questRecord literal sets every field")
+						break
+					}
+					if key, ok := kv.Key.(*ast.Ident); ok && fields[key.Name] {
+						flag(kv.Pos(), "a questRecord literal sets "+key.Name)
+					}
+				}
+			case *ast.AssignStmt:
+				for _, lhs := range x.Lhs {
+					if sel, ok := lhs.(*ast.SelectorExpr); ok && fields[sel.Sel.Name] {
+						flag(sel.Pos(), "an assignment sets ."+sel.Sel.Name)
+					}
+				}
+			case *ast.IncDecStmt:
+				if sel, ok := x.X.(*ast.SelectorExpr); ok && fields[sel.Sel.Name] {
+					flag(sel.Pos(), "an increment sets ."+sel.Sel.Name)
+				}
+			}
+			return true
+		})
+	}
+	return found
+}
+
+// questsWide is a quest list at its widest: the longest shipped quest name,
+// repeatable and followed, a long step description, progress and guide
+// lines, a finished repeatable, a quest set aside, and three-digit counts.
+func questsWide() questsScreen {
+	rows := []questRecord{
+		questRow(1, "The City Watch's Missing Person", true, true,
+			"Orrin has set you against the Unfolded. Climb out of the grove through the Veil Tear, cross the Folded Distance and the Unraveling, and defeat the Unfolded in the Thinnest Place.",
+			"Unfolded defeated in the Thinnest Place: 2 of 5", "Usually found around here: Drillmaster Vorn the Unyielding.", 1.0/3),
+		questRow(2, "First Blood", false, false,
+			"Crier Toke hands you a cudgel when you take this on and sends you to Drillmaster Vorn in the Drill Yard.",
+			"", "Take the portal.", 1.0/8),
+	}
+	rows = append(rows, questRecord{Id: 3, Name: "The Peddler's Overdue Freight", Repeatable: true,
+		Description: repeatReadyLine}.withProgress(1))
+	rows = append(rows, questRecord{Id: 4, Name: "The Innkeeper's Complaint",
+		Description: "Set aside. Talk to Innkeeper Bramblewick of the Sleeping Stag to take it up again."})
+	return questsScreen{QuestsTotal: 112, QuestsFound: len(rows), Records: rows}
+}
+
+// questsAsCaptured is the quest list as the 2026-11-10 capture showed it,
+// which fits 80 columns, for the 80-column golden.
+func questsAsCaptured() questsScreen {
+	return questsScreen{QuestsTotal: 7, QuestsFound: 2, Records: []questRecord{
+		questRow(1, "Find Your Footing", false, true,
+			"Crier Toke wants you to learn the town before the town has to learn you. Start at the Chrysalis School Hall. If you lose your way, the map command shows where you have been.",
+			"", "Take the portal.", 1.0/7),
+		questRow(2, "The Unfolded", false, false,
+			"Orrin has set you against the Unfolded. Climb out of the grove through the Veil Tear, cross the Folded Distance and the Unraveling, and defeat the Unfolded in the Thinnest Place.",
+			"Unfolded defeated: 0 of 1", "Take the portal.", 1.0/3),
+	}}
+}
+
+// titleWide is the title screen at its widest words.
+func titleWide() titleScreen {
+	bar := func(done float64) string {
+		full, empty := util.ProgressBar(done, 20)
+		return `<ansi fg="green">` + full + `</ansi><ansi fg="black-bold">` + empty + `</ansi>`
+	}
+	return titleScreen{FullTitle: "Ascendant Grandmaster Juggernaut", MutTier: "Ascendant",
+		MutLoad: "overwhelming", SkillTier: "Grandmaster", SkillPct: "accomplished", Archetype: "Juggernaut",
+		TopStats: []statBar{{"Perception", 300, bar(1)}, {"Strength", 290, bar(0.96)}, {"Vitality", 120, bar(0.4)}}}
+}
+
+// narrowTemplateScreens are the template screens held here: data for the
+// width checks, and data whose base fits 80 for the golden.
+var narrowTemplateScreens = map[string]struct{ wide, golden func() any }{
+	"character/conditions": {func() any { return conditionsWide() }, func() any { return conditionsWide() }},
+	"character/quests":     {func() any { return questsWide() }, func() any { return questsAsCaptured() }},
+	"character/title":      {func() any { return titleWide() }, func() any { return titleWide() }},
+}
+
+func narrowGoldenPath(name string) string {
+	return filepath.Join("testdata", "narrow_forms_80", strings.ReplaceAll(name, "/", "_")+".golden")
+}
+
+// checkGolden compares got, line ends aside, with the named golden, or
+// records it with -update-narrow-forms.
+func checkGolden(t *testing.T, name, got string) {
+	t.Helper()
+	got = strings.ReplaceAll(got, "\r\n", "\n")
+	path := narrowGoldenPath(name)
+	if *updateNarrowForms {
+		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
+		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
+	}
+	want, err := os.ReadFile(path)
+	require.NoError(t, err, "record the golden from master with -update-narrow-forms")
+	require.LessOrEqual(t, templates.WidestLine(string(want)), 80, "the golden no longer fits 80, so it pins a form")
+	require.Equal(t, string(want), got, "%s changed at 80 columns", name)
+}
+
+func TestNarrowTemplateScreensFitEveryWidth(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	useDogmudTemplates(t)
+	for name, screen := range narrowTemplateScreens {
+		for _, width := range narrowWidths {
+			out, err := templates.ProcessAtWidth(name, screen.wide(), width)
+			require.NoError(t, err, "%s at %d", name, width)
+			require.NotEmpty(t, strings.TrimSpace(out), "%s at %d rendered nothing", name, width)
+			require.LessOrEqual(t, templates.WidestLine(out), width, "%s at %d columns:\n%s", name, width, out)
+		}
+	}
+}
+
+func TestNarrowTemplateScreensKeepTheir80ColumnOutput(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	useDogmudTemplates(t)
+	for name, screen := range narrowTemplateScreens {
+		out, err := templates.ProcessAtWidth(name, screen.golden(), 80)
+		require.NoError(t, err)
+		checkGolden(t, name, out)
+	}
+}
+
+// companionScreen runs `companion` for user 1 with a summoned wolf whose
+// name and mutation list run long and a bonded hound that is away, at a
+// reader width, and returns what was sent.
+func companionScreen(t *testing.T, width int) string {
+	t.Helper()
+	return companionCommand(t, width, "", longCompanionMutations)
+}
+
+// longCompanionMutations are the two longest shipped mutation names, at
+// levels that add "(Ln)", and two short ones; capturedCompanionMutations are
+// the two the 2026-11-10 capture showed.
+var (
+	longCompanionMutations     = map[string]int{"rending-claws": 1, "tail": 1, "sustaining-presence": 4, "commanding-presence": 3}
+	capturedCompanionMutations = map[string]int{"rending-claws": 1, "tail": 1}
+)
+
+// companionCommand runs `companion <rest>` as companionScreen does, the wolf
+// holding muts.
+func companionCommand(t *testing.T, width int, rest string, muts map[string]int) string {
+	t.Helper()
+	user, room := getTestUserAndRoom(t)
+	prevWidth, prevComps := user.LineWidth, user.Character.Companions
+	t.Cleanup(func() { user.LineWidth, user.Character.Companions = prevWidth, prevComps })
+	user.LineWidth = width
+
+	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
+		"rending-claws":       {MutationId: "rending-claws", Name: "Rending Claws"},
+		"tail":                {MutationId: "tail", Name: "Tail"},
+		"sustaining-presence": {MutationId: "sustaining-presence", Name: "Sustaining Presence"},
+		"commanding-presence": {MutationId: "commanding-presence", Name: "Commanding Presence"},
+	}))
+	wolf := &mobs.Mob{MobId: 1, InstanceId: 301, Character: characters.Character{
+		Name: "Steppe Spirit Wolf", Health: 40, Stamina: 30, Conviction: 10,
+		Conditions: conditions.New(), Cooldowns: map[string]int{},
+		Mutations: muts,
+	}}
+	wolf.Character.HealthMax.Value = 40
+	wolf.Character.StaminaMax.Value = 60
+	wolf.Character.ConvictionMax.Value = 50
+	mobs.SetInstanceForTest(301, wolf)
+	t.Cleanup(func() { mobs.SetInstanceForTest(301, nil) })
+
+	user.Character.Companions = []characters.CompanionInfo{
+		{MobId: 1, InstanceId: 301, SourceType: characters.CompanionSummoned, Name: "Steppe Spirit Wolf", AutoAssist: true},
+		{MobId: 1, InstanceId: 999, SourceType: characters.CompanionBonded, Name: "Fenwickerly the Ashen Hound", AutoAssist: false},
+	}
+
+	events.DrainQueuedMessagesForTest(user.UserId)
+	_, err := Companion(rest, user, room, 0)
+	require.NoError(t, err)
+	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
+}
+
+// The detail view of one companion, present and away, fits too. Its
+// mutations line is never wrapped (CategoryScreen), so the long list runs
+// past 80 columns there and gets a form even at 80, as a template wider than
+// its reader does. With the captured list it fits 80 and is pinned.
+func TestCompanionDetailFitsEveryWidth(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	for _, name := range []string{"steppe", "fenwickerly"} {
+		for _, width := range narrowWidths {
+			out := companionCommand(t, width, name, longCompanionMutations)
+			require.Contains(t, out, "Auto-Assist", "%s at %d", name, width)
+			require.LessOrEqual(t, templates.WidestLine(out), width, "companion %s at %d columns:\n%s", name, width, out)
+			if width < 80 {
+				requireFullWidthRules(t, out, width)
+			}
+		}
+	}
+	// The long mutations line does not fit 80, so the wolf's detail view
+	// is the 42 form there too.
+	requireFullWidthRules(t, companionCommand(t, 80, "steppe", longCompanionMutations), 42)
+}
+
+// A mutation entry (its name, its level and comma) is never split while it
+// fits on a line by itself, in the list or the detail view, with the two
+// longest shipped names at a level.
+func TestCompanionMutationEntriesStayWhole(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	entries := []string{"Commanding Presence (L3),", "Rending Claws,", "Sustaining Presence (L4),", "Tail"}
+	for _, width := range []int{30, 42} {
+		for _, rest := range []string{"", "steppe"} {
+			out := companionCommand(t, width, rest, longCompanionMutations)
+			for _, entry := range entries {
+				if rest == "steppe" {
+					entry = strings.TrimSuffix(entry, ",")
+				}
+				require.Contains(t, out, entry, "companion %q at %d splits %q:\n%s", rest, width, entry, out)
+			}
+		}
+		// The first line is measured from its own indent: the 27-letter
+		// name fits after 2 spaces at 30.
+		require.Contains(t, companionScreen(t, width), "Fenwickerly the Ashen Hound", "at %d", width)
+	}
+}
+
+// requireFullWidthRules holds a companion form to the ruling that every
+// rule spans the form's width: each line with a "━" in it is exactly
+// formWidth wide, and there is at least one.
+func requireFullWidthRules(t *testing.T, out string, formWidth int) {
+	t.Helper()
+	rules := 0
+	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
+		if !strings.Contains(line, "━") {
+			continue
+		}
+		rules++
+		require.Equal(t, formWidth, templates.WidestLine(line), "rule line %q in:\n%s", line, out)
+	}
+	require.Positive(t, rules, "no rule line in:\n%s", out)
+}
+
+func TestCompanionDetailKeepsIts80ColumnOutput(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	checkGolden(t, "companion_detail", companionCommand(t, 80, "steppe", capturedCompanionMutations)+
+		companionCommand(t, 80, "fenwickerly", capturedCompanionMutations))
+}
+
+func TestCompanionListFitsEveryWidth(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	for _, width := range narrowWidths {
+		out := companionScreen(t, width)
+		require.Contains(t, out, "Steppe Spirit Wolf", "at %d", width)
+		require.LessOrEqual(t, templates.WidestLine(out), width, "companion at %d columns:\n%s", width, out)
+		if width < 80 {
+			requireFullWidthRules(t, out, width)
+		}
+	}
+}
+
+func TestCompanionListKeepsIts80ColumnOutput(t *testing.T) {
+	cleanup := seedAllRegistries()
+	defer cleanup()
+	checkGolden(t, "companion", companionScreen(t, 80))
+}
diff --git a/internal/usercommands/quests.go b/internal/usercommands/quests.go
index 64479121d..270be944f 100644
--- a/internal/usercommands/quests.go
+++ b/internal/usercommands/quests.go
@@ -26,9 +26,54 @@ type questRecord struct {
 	Description string
 	Progress    string // "Roots gathered: 2 of 5" for a count or explore step, else ""
 	Guide       string // the destination line (questengine.GuideLine), else ""
-	Completion  string
-	BarFull     string
-	BarEmpty    string
+	// Completion, BarFull, BarEmpty and done are set together, only by
+	// withProgress, so the percentage and the bar at every width come from
+	// one number (TestQuestProgressHasOneSource).
+	Completion string
+	BarFull    string
+	BarEmpty   string
+	// done is the completion, 0 to 1, so a narrow form can draw the same bar
+	// shorter (BarFullAt).
+	done float64
+}
+
+// questBarCells is the bar's width in the 80- and 42-column quest lists.
+const questBarCells = 25
+
+// withProgress is r showing completion done: its percentage and its bar.
+func (r questRecord) withProgress(done float64) questRecord {
+	r.done = done
+	r.Completion = fmt.Sprintf(`%d%%`, int(math.Floor(done*100)))
+	r.BarFull, r.BarEmpty = util.ProgressBar(done, questBarCells)
+	return r
+}
+
+// questsScreen is the data of the character/quests template.
+type questsScreen struct {
+	QuestsTotal int
+	QuestsFound int
+	Records     []questRecord
+}
+
+// BarFullAt and BarEmptyAt are the row's progress bar drawn cells wide, for
+// a narrow form (the 30-column quest list draws 20 cells). A row with no bar,
+// a quest set aside, has none at any width.
+func (r questRecord) BarFullAt(cells int) string {
+	full, _ := r.barAt(cells)
+	return full
+}
+
+// BarEmptyAt is the empty part of the bar BarFullAt draws.
+func (r questRecord) BarEmptyAt(cells int) string {
+	_, empty := r.barAt(cells)
+	return empty
+}
+
+func (r questRecord) barAt(cells int) (string, string) {
+	if r.BarFull == `` && r.BarEmpty == `` {
+		return ``, ``
+	}
+	return util.ProgressBar(r.done, cells)
 }
 
 // The finished-repeatable status lines in "quests all" (#375 P6).
@@ -65,9 +110,8 @@ func finishedRepeatRows(c *characters.Character, progress map[int]string, showHi
 		case c.QuestCooldownActive(q.QuestId):
 			line = repeatWaitLine
 		}
-		full, empty := util.ProgressBar(1, 25)
 		rows = append(rows, questRecord{Id: q.QuestId, Name: q.Name, Repeatable: true,
-			Description: line, Completion: "100%", BarFull: full, BarEmpty: empty})
+			Description: line}.withProgress(1))
 	}
 	return rows
 }
@@ -96,7 +140,6 @@ func activeQuestRecord(user *users.UserRecord, questInfo *quests.Quest, questSte
 	}
 
 	completion := float64(completedSteps) / float64(totalSteps)
-	barFull, barEmpty := util.ProgressBar(completion, 25)
 
 	return questRecord{
 		Id:          questInfo.QuestId,
@@ -106,10 +149,7 @@ func activeQuestRecord(user *users.UserRecord, questInfo *quests.Quest, questSte
 		Description: description,
 		Progress:    progress,
 		Guide:       guide,
-		Completion:  fmt.Sprintf(`%d%%`, int(math.Floor(completion*100))),
-		BarFull:     barFull,
-		BarEmpty:    barEmpty,
-	}, completion
+	}.withProgress(completion), completion
 }
 
 // questsSubcommand splits `quests abandon <name>` / `quests focus <name>`;
@@ -178,16 +218,10 @@ func Quests(rest string, user *users.UserRecord, room *rooms.Room, flags events.
 		return true, nil
 	}
 
-	type QuestInfo struct {
-		QuestsTotal int
-		QuestsFound int
-		Records     []questRecord
-	}
-
 	showHidden := questsShowHidden(rest, user)
 	showComplete := rest == `all` || rest == `all+`
 
-	qInfo := QuestInfo{}
+	qInfo := questsScreen{}
 	allQuests := []questRecord{}
 	var completion float64
 
diff --git a/internal/usercommands/testdata/narrow_forms_80/character_conditions.golden b/internal/usercommands/testdata/narrow_forms_80/character_conditions.golden
new file mode 100644
index 000000000..3a330ba14
--- /dev/null
+++ b/internal/usercommands/testdata/narrow_forms_80/character_conditions.golden
@@ -0,0 +1,11 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Conditions</ansi> ─────────────────────────────────────────────────────────────┐
+   <ansi fg="yellow-bold">Chrysalis Regeneration (99)</ansi> Your body knits itself back together, stronger in
+                            the places it broke.
+                            <ansi fg="green">well established</ansi>
+   <ansi fg="yellow-bold">Minor Potion Healing    </ansi> A small healing effect is working on you.
+                            <ansi fg="green">active</ansi>
+   <ansi fg="yellow-bold">Death's Shadow          </ansi> A dark shadow clings to you, dulling your senses
+                            and weakening your body.
+                            <ansi fg="red">fading fast</ansi>
+   <ansi fg="yellow-bold">Stoneskin               </ansi> Skin like rock.
+ └────────────────────────────────────────────────────────────────────────────┘
diff --git a/internal/usercommands/testdata/narrow_forms_80/character_quests.golden b/internal/usercommands/testdata/narrow_forms_80/character_quests.golden
new file mode 100644
index 000000000..00ad9a9b0
--- /dev/null
+++ b/internal/usercommands/testdata/narrow_forms_80/character_quests.golden
@@ -0,0 +1,16 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Quests ( 2 out of 7 shown )</ansi> ──────────────────────────────────────────┐
+   <ansi fg="questname">Find Your Footing (following)            </ansi> <ansi fg="green">███</ansi><ansi fg="black-bold">░░░░░░░░░░░░░░░░░░░░░░</ansi> <ansi fg="cyan-bold">14% </ansi>
+   <ansi fg="white-bold">Crier Toke wants you to learn the town before the town has to learn
+   you. Start at the Chrysalis School Hall. If you lose your way, the map
+   command shows where you have been.</ansi>
+   <ansi fg="240">Take the portal.</ansi>
+
+   <ansi fg="questname">The Unfolded                             </ansi> <ansi fg="green">████████</ansi><ansi fg="black-bold">░░░░░░░░░░░░░░░░░</ansi> <ansi fg="cyan-bold">33% </ansi>
+   <ansi fg="white-bold">Orrin has set you against the Unfolded. Climb out of the grove through
+   the Veil Tear, cross the Folded Distance and the Unraveling, and defeat
+   the Unfolded in the Thinnest Place.</ansi>
+   <ansi fg="white">Unfolded defeated: 0 of 1</ansi>
+   <ansi fg="240">Take the portal.</ansi>
+ └──────────────────────────────────────────────────────────────────────────┘
+ <ansi fg="240">To see all quests (including completed), use <ansi fg="command">quests all</ansi>.</ansi>
+ 
diff --git a/internal/usercommands/testdata/narrow_forms_80/character_title.golden b/internal/usercommands/testdata/narrow_forms_80/character_title.golden
new file mode 100644
index 000000000..7cda6fe59
--- /dev/null
+++ b/internal/usercommands/testdata/narrow_forms_80/character_title.golden
@@ -0,0 +1,12 @@
+ ┌─ <ansi fg="black-bold">.:</ansi><ansi fg="20">Title</ansi> ────────────────────────────────────────────────────────────────┐
+   <ansi fg="yellow-bold">Current Title:</ansi> <ansi fg="white-bold">Ascendant Grandmaster Juggernaut</ansi>
+
+   <ansi fg="yellow">Mutation Tier:</ansi>  Ascendant <ansi fg="black-bold">(overwhelming presence)</ansi>
+   <ansi fg="yellow">Skill Tier:</ansi>     Grandmaster <ansi fg="black-bold">(accomplished)</ansi>
+   <ansi fg="yellow">Archetype:</ansi>      Juggernaut
+
+   <ansi fg="yellow">Stat Profile:</ansi>
+    Perception   <ansi fg="green">████████████████████</ansi><ansi fg="black-bold"></ansi>
+    Strength     <ansi fg="green">███████████████████</ansi><ansi fg="black-bold">░</ansi>
+    Vitality     <ansi fg="green">████████</ansi><ansi fg="black-bold">░░░░░░░░░░░░</ansi>
+ └──────────────────────────────────────────────────────────────────────────┘
diff --git a/internal/usercommands/testdata/narrow_forms_80/companion.golden b/internal/usercommands/testdata/narrow_forms_80/companion.golden
new file mode 100644
index 000000000..d99302c2e
--- /dev/null
+++ b/internal/usercommands/testdata/narrow_forms_80/companion.golden
@@ -0,0 +1,6 @@
+<ansi fg="system"><ansi fg="yellow">━━━ Companions ━━━</ansi></ansi>
+<ansi fg="system">  <ansi fg="mobname">Steppe Spirit Wolf</ansi> [summoned] (assist: on)</ansi>
+<ansi fg="system">    HP: <ansi fg="71">██████████</ansi>  SP: <ansi fg="221">█████</ansi><ansi fg="238">░░░░░</ansi>  CP: <ansi fg="203">██</ansi><ansi fg="238">░░░░░░░░</ansi></ansi>
+<ansi fg="system"> Mutations: Commanding Presence (L3), Rending Claws, Sustaining Presence (L4),</ansi>
+<ansi fg="system">Tail.</ansi>
+<ansi fg="system">  <ansi fg="mobname">Fenwickerly the Ashen Hound</ansi> [bonded] <ansi fg="239">[offline]</ansi> (assist: off)</ansi>
diff --git a/internal/usercommands/testdata/narrow_forms_80/companion_detail.golden b/internal/usercommands/testdata/narrow_forms_80/companion_detail.golden
new file mode 100644
index 000000000..0bdae4896
--- /dev/null
+++ b/internal/usercommands/testdata/narrow_forms_80/companion_detail.golden
@@ -0,0 +1,9 @@
+<ansi fg="system"><ansi fg="yellow">━━━ Steppe Spirit Wolf [summoned] ━━━</ansi></ansi>
+<ansi fg="screen">  Health: pristine   Stamina: steady     Conviction: critical</ansi>
+<ansi fg="screen">  <ansi fg="yellow">Strength:</ansi>   feeble        <ansi fg="yellow">Dexterity:</ansi>  feeble        <ansi fg="yellow">Perception:</ansi> feeble</ansi>
+<ansi fg="screen">  <ansi fg="yellow">Vitality:</ansi>   feeble        <ansi fg="yellow">Willpower:</ansi>  feeble        <ansi fg="yellow">Charisma:</ansi>   feeble</ansi>
+<ansi fg="screen">  <ansi fg="yellow">Mutations:</ansi>  Rending Claws, Tail</ansi>
+<ansi fg="screen">  Auto-Assist: on</ansi>
+<ansi fg="system"><ansi fg="yellow">━━━ Fenwickerly the Ashen Hound [bonded] ━━━</ansi></ansi>
+<ansi fg="system">  This companion is not currently present.</ansi>
+<ansi fg="screen">  Auto-Assist: off</ansi>
diff --git a/internal/usercommands/title.go b/internal/usercommands/title.go
index 0ac504d9d..ca496be49 100644
--- a/internal/usercommands/title.go
+++ b/internal/usercommands/title.go
@@ -14,6 +14,24 @@ import (
 	"github.com/GoMudEngine/GoMud/internal/util"
 )
 
+// statBar is one stat of the title screen's stat profile.
+type statBar struct {
+	Name  string
+	Value int
+	Bar   string
+}
+
+// titleScreen is the data of the character/title template.
+type titleScreen struct {
+	FullTitle string
+	MutTier   string
+	MutLoad   string
+	SkillTier string
+	SkillPct  string
+	Archetype string
+	TopStats  []statBar
+}
+
 func Title(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
 
 	char := user.Character
@@ -71,12 +89,6 @@ func Title(rest string, user *users.UserRecord, room *rooms.Room, flags events.E
 	}
 
 	// Stat lean - show top 3 stats
-	type statBar struct {
-		Name  string
-		Value int
-		Bar   string
-	}
-
 	statVals := []statBar{
 		{"Strength", char.Stats.Strength.Value, ""},
 		{"Dexterity", char.Stats.Dexterity.Value, ""},
@@ -106,17 +118,7 @@ func Title(rest string, user *users.UserRecord, room *rooms.Room, flags events.E
 		statVals[i].Bar = fmt.Sprintf(`<ansi fg="green">%s</ansi><ansi fg="black-bold">%s</ansi>`, barFull, barEmpty)
 	}
 
-	type TitleDisplay struct {
-		FullTitle string
-		MutTier   string
-		MutLoad   string
-		SkillTier string
-		SkillPct  string
-		Archetype string
-		TopStats  []statBar
-	}
-
-	data := TitleDisplay{
+	data := titleScreen{
 		FullTitle: fullTitle,
 		MutTier:   mutTierDisplay,
 		MutLoad:   mutLoadDesc,
````
