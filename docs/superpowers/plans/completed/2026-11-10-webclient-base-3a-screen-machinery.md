# Web Client Base, Plan 3a: Screen Machinery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let any screen the server draws fit a reader down to 30 columns. Templates get narrow forms picked by width and a `screenwidth` function, tables get two narrow forms, splash art falls back to its wrapped words, the MOTD gets a narrow form, and a guard knows every template and why it is safe.

**Architecture:** `templates.Process` reads the reader's `WrapWidth` on every call. When a `.template`'s output is wider than the reader, it renders the forms beside it (`name.w42.template`, `.w40`, `.w30`) widest first and uses the first that fits. Which forms a template has is found once and remembered, and only a template that has forms is ever measured. A base that fits is always used, so a desktop changes only where a table was already wider than 80 columns. No web client reports a narrow width until plan 5, so web players see nothing yet.

**Tech Stack:** Go 1.25 `text/template`, `testify`, guards in `internal/guards`.

**Spec:** `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md`, Design section 2 and rulings 3, 22 to 25. The owner split plan 3 into 3a (this) and 3b (the hand-laid screens) on 2026-11-10. Part of #783.

**Needs:** plan 1 (`UserRecord.WrapWidth`, merged in #835). Independent of plan 2 (#841).

---

## Facts verified against source

Read 2026-11-10 from master at `2644b3731`. Everything in the patch at the end of this plan was applied in a detached worktree of that commit and run there: the full suite (`go test . ./internal/... ./modules/...`) had no failing package, every `tools/jstest` and `tools/webclient-tests` file passed, and `golangci-lint --new-from-merge-base=origin/master` reported 0 issues. The patch extracted from this file applied cleanly to a second fresh worktree, and each task's red and green steps were run there with the results written below. The sabotage steps in Task 5 were each run and went red. A subagent surveyed the 85 templates outside `help/` and `admincommands/` (callers, data types, message categories), with a documented negative search for each template it found no caller for. A blind adversarial review found eight defects, all fixed in this version.

| # | Fact | Where |
|---|---|---|
| 1 | `Process` re-reads and re-parses the file on every call. It tries `fname.screenreader.template` first for a screen-reader reader, then `fname.md`, then `fname.template`. The parse-ANSI branches are dead (`parseAnsiTags` is always false) | `internal/templates/templates.go:160-294` |
| 2 | `templateConfig` is cached per user and cleared only when the `screenreader` setting changes, so a width must not be cached there | `templates.go:108-142`; `internal/hooks/UserSettingChanged_ClearSettingCaches.go:24-26` |
| 3 | `func (u *UserRecord) WrapWidth() int`: a websocket session's reported columns capped at 80 and by a saved `LineWidth`, floor 30; otherwise `GetLineWidth()`, which for telnet can be 40 to 240 (`set.go:521`). `MinWrapWidth = 30`, `MaxAutoWrapWidth = 80` | `internal/users/wrapwidth.go:8`, `:11`, `:23-32` |
| 4 | A function added with `Funcs` after `Parse` is the one used at `Execute` (run: an override to 30 printed 30) | scratch program, 2026-11-10 |
| 5 | Template paths are built with `util.FilePath`, which uses the OS separator, so a test filesystem must key files the same way | `internal/util/util.go:877-882` |
| 6 | With no filesystem registered, `readFile` returns `(nil, nil)` and the first attempt "succeeds" empty; production registers module filesystems | `templates.go:68-77`; the `SetFSForTest` comment |
| 7 | `GetTable` has 27 Go callers (25 generic, 1 shoplist, 1 lockpicking). Widths are data-driven: shop titles are `Items available by <seller>` (`list.go:482`), the player `history` Log column and the `online` player view are unbounded, and the secondhand table puts the item name first (`list.go:194`) | survey; `templates.go:382` |
| 8 | `Splash_Deliver` renders the art once and sends screen-reader users the caption; every splash sender sets a caption; `CategorySplash` is not wrapped by the pipeline; the mutation caption can be over 100 columns | `internal/hooks/Splash_Deliver.go:16-40`; `internal/messaging/pipeline.go:124`; `MutationGained_Reveal.go:31-32` |
| 9 | The splash art is at most 76 columns | survey (`generic/splash/*`) |
| 10 | `motdBox(text, lw)` draws a box exactly `lw` wide; the spaced-out title and its leading space take 41 of the `lw-4` content columns, so the title fits from 45; a word wider than `lw-5` bursts the right border; the shipped MOTD's longest word is 72 | `internal/usercommands/motd.go:40-89`; `_datafiles/config.yaml:86` |
| 11 | `questup` (on `CategorySystem`, wrapped), `skillup` and `statup` (on `CategorySkillProgress`, not wrapped) draw 79-column rows of `*` | `templates/character/{questup,skillup,statup}.template`; survey |
| 12 | `descriptions/room.template` frames room alerts in a 73-column border; four alerts are hand-indented Go strings and the tracking line is never wrapped, so the room needs real narrow work (3b) | `templates/descriptions/room.template:4`, `:6`; `internal/rooms/roomdetails.go:99-111` |
| 13 | `mail/message.template` wraps the message at a literal 71 after the 9-column `Message: ` label; at 30 its `Sent:` line is 39 columns (the date), so mail needs a form (3b) | `templates/mail/message.template:8`; `_datafiles/world/dogmud/localize/en.yaml:14-16` |
| 14 | The function map has `repeat`, `sub`, `splitstring` (`util.SplitStringNL(input, width, prefix...)`, which treats a colour tag as one token) and no `min` | `templatesfunctions.go:44-131`; `util.go:79`, `:321` |
| 15 | Guards render real templates by overlaying `FilePaths.DataFiles` and registering the live world's folder; `language.T` needs `InitTranslation`, and `mudlog` needs `SetupLogger` or an error log panics | `internal/guards/wrap_live_render_golden_test.go:89-105`; `internal/usercommands/usercommands_test.go:70-76` |
| 16 | 608 template files in the world folder, 23 more in `modules/*/files/datafiles/templates`. The auctions module's four `auctions/auction-*` templates draw 79-column rules and are broadcast to every player | `find`; `modules/auctions/auctions.go:253,356,513,668,715` |
| 17 | 15 world templates have no production caller: the six `generic/moon_*`, `generic/sunrise`, `generic/sunset`, `character/experience`, `jobs`, `permadeath`, `descriptions/train`, `login/character-name.prompt`, `login/newuser.prompt`, `maps/secret` (eight more screen-reader copies belong to them) | survey |
| 18 | Measured on this desktop with one CPU (`-cpu 1`), a 20-row table: 0.60 ms at 80 (one render), 1.70 ms at 30 (base, 40 and 30 forms). Not measured on the droplet, which is slower | `go test -bench BenchmarkTableAt -cpu 1 -count 3` |

## Where this plan departs from the spec, and why

1. **The width is read per render, not cached in `templateConfig`.** The spec put it in `templateConfig` and cleared that on a width report. Fact 2: that cache is cleared only for `screenreader`, so a resize or a new `set linewidth` would show a stale form. Reading `WrapWidth` costs one map lookup.
2. **A `screenwidth` template function** is new: a template fits a rule or a wrapped paragraph to its reader without a form (the three banners' `*` rows, mail's paragraph). It is capped at 80, so layouts are never drawn wider than they are today; at 80 and above each draws exactly its old width.
3. **Forms apply at any width where the base does not fit (owner, 2026-11-10).** The spec said forms are tried only under 80. Ruling 22 says the widest form that fits: a table wider than a desktop's 80 columns (the `history` Log, the `online` list, and the admin combat stats, timer stats and locate tables) wraps mid-row in the 80-column game window today, and now takes a narrower form. A base that fits is always used, so nothing else changes on a desktop. Plan 3b redesigns each table that is over 80 so it fits (owner, 2026-11-10). The spec's Design section 2 and Plans table say so.
4. **Plan 3 is two plans** (owner, 2026-11-10). 3b takes the room view (fact 12), mail (fact 13), the `skillup` and `statup` banners (sentences with colour tags mid-sentence on a category that is not wrapped), the four auction broadcasts (fact 16), the Go-built skill banner (`internal/banner/banner.go`, 64 columns), the Go-built companion, craft and guild screens, and every table now wider than 80 columns.
5. **The guard renders the templates that must fit now, not every template.** Each other template is named in a class with its reason: splash art (narrow readers get the words), maps and wide tables, raw login text, text the caller or pipeline wraps, plan 3b, help (plan 4), admin (ruling 25), and no caller. 3b and 4 move templates into the rendered class.
6. **The cost was measured on this desktop with one CPU, not on the droplet.** A narrow table costs about 1 ms more (fact 18), only on a screen a player asked for; even several times slower on the droplet, that is well under a game round. No parsed-template cache. Which forms a template has is found once and remembered, and the width is measured only for a template that has forms, so a template with none (most of them, including the auction broadcasts) costs one map lookup.
7. **Raw login text stays as it is.** Login prompts and the connect splash go out before a web client has reported a width (plan 5), outside the pipeline; the terminal soft-wraps them.

What players see: web players, nothing until plan 5 sends a width. Telnet players with a saved `set linewidth` under 80: tables in a narrower form when the bordered one does not fit, banner rules sized to their line, a splash's words instead of its art below 76, and the MOTD's plain form below 45. Any player: a table wider than 80 takes a narrower form instead of overflowing.

## Table forms

The spec named the forms, not their look. As built, with the title and the first cell wrapped to the screen:

```
40 columns, plain              30 columns, two lines per item
.: Spells                      .: Lost and found
SpellId  Name         Cost     the Ancient Lantern of the
aurora   Aurora of... 12       Drowned Lighthouse
                                 Where: the Thornwall Temple
                                 District · Since: two days
```

The 30-column form puts the first cell on its own line and the rest as `Header: value` pairs wrapped two spaces in. Every form shows every cell (ruling 24). A header can end one line with its value on the next; accepted.

## Template classes in the guard

Every template, in the world folder and every module folder, has exactly one class. The classes are the state of the work.

| Class | Rule | Templates |
|---|---|---|
| fits | rendered at 30, 42 and 80 with realistic data; no line wider than the width (wrapped first when its category is wrapped) | `tables/generic`, `tables/shoplist`, `character/questup` |
| caption | narrow readers get the scene's words, wrapped (`Splash_Deliver`) | `generic/splash/*` |
| wide | maps and wide tables may scroll sideways (ruling 3) | `maps/map`, `tables/lockpicking` |
| raw | sent outside the pipeline before a width is known, or on leaving | `goodbye`, `login/connect-splash`, the login prompts and mask, `generic/prompt.yn` |
| wrapped | the caller or the pipeline wraps it to the reader | `descriptions/who`, `descriptions/ontheground`, `player-despawn` |
| plan 3b | narrow forms come in 3b | 14 character templates, 10 description templates, `mail/message`, the 4 auction templates |
| no caller | no production code renders it | fact 17 |
| help, admin | plan 4; out of scope (ruling 25) | `help/`, `admincommands/` |

## Files

The whole change is one patch, at the end of this plan, made from the dry run. Each task applies the files it owns with `git apply --include`.

| File | Change |
|---|---|
| Create `internal/templates/forms.go` | `FullWidth`, `FormWidths`, `FormPath`, `formsOf` and its cache, `readerWidth`, `widthFuncs`, `WidestLine`, `narrowestFit` |
| Modify `internal/templates/templates.go` | `Process` split into `process` and `renderFile`; forms on the `.template` attempt; `TemplateTable.RowDetail` |
| Modify `internal/templates/templatesfunctions.go` | `screenwidth` (80 with no reader) and `min` |
| Modify `internal/templates/testing_support.go` | `SetFSForTest` clears the form cache; `ProcessAtWidth` |
| Create `internal/templates/forms_test.go`, `forms_bench_test.go` | Form picking, the cache, `screenwidth`, `WidestLine`; the benchmark |
| Create `tables/generic.w40.template`, `generic.w30.template`, `shoplist.w40.template`, `shoplist.w30.template` | Table forms (under `_datafiles/world/dogmud/templates/`) |
| Modify `character/questup`, `skillup`, `statup`, `mail/message` (`.template`) | `screenwidth` in place of 79 and 71 |
| Modify `internal/hooks/Splash_Deliver.go`, `_test.go` | `splashFor`: art only where it fits, the caption wrapped |
| Modify `internal/usercommands/motd.go`, `motd_test.go` | `motdPlain`, `motdLongestWord`, `motdBoxMinWidth` |
| Create `internal/guards/template_width_guard_test.go` | The guard |
| Modify `internal/templates/context.md`, the spec, `docs/README.md` | Docs, and the 3a/3b split in the spec's Plans table |

Edits land only by `git apply` or the Edit and Write tools; never `sed -i`, a `>` into a repo file, or Python writing a repo file (a hook enforces this). `$S` is the full path of the session scratchpad; set it at the start of every Bash call that uses it.

---

### Task 0: Branch and the patch

- [ ] **Step 1: Worktree**

```bash
cd "/c/Users/Calabe Davis/workspace/DOGMud"
git fetch origin
git rev-parse --short origin/master
git worktree add C:/tmp/dogmud-3a -b feature/webclient-base-3a-screen-machinery origin/master
```

If `origin/master` is past `2644b3731`, Step 2's `git apply --check` decides: if it passes, go on; if it fails, stop and report. If #841 has merged, its `docs/README.md` rows sit in another part of the file.

- [ ] **Step 2: Extract the patch and check it**

The patch is the only fenced `diff` block in this plan, which sits in the main checkout until Task 4 copies it in. Write this script to `$S/extract_patch.py` with the Write tool:

```python
import re, sys
text = open(sys.argv[1], encoding="utf-8").read().replace("\r\n", "\n")
blocks = re.findall(r"^````diff\n(.*?)^````$", text, re.S | re.M)
assert len(blocks) == 1, len(blocks)
open(sys.argv[2], "w", encoding="utf-8", newline="\n").write(blocks[0])
```

Then:

```bash
S="<the session scratchpad, full path>"
python -I "$S/extract_patch.py" "/c/Users/Calabe Davis/workspace/DOGMud/docs/superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md" "$S/3a.patch"
cd C:/tmp/dogmud-3a && git apply --check "$S/3a.patch" && echo CHECK-OK
```

Expected: `CHECK-OK`. Nothing is applied yet.

---

### Task 1: Form picking in `Process`

- [ ] **Step 1: The tests, red**

```bash
cd C:/tmp/dogmud-3a; S="<scratchpad>"
git apply --include=internal/templates/forms_test.go "$S/3a.patch"
go test ./internal/templates/ -count=1 2>&1 | tail -5
```

Expected: a build failure, `undefined: process`.

- [ ] **Step 2: The mechanism**

```bash
git apply --include=internal/templates/forms.go --include=internal/templates/templates.go --include=internal/templates/templatesfunctions.go --include=internal/templates/testing_support.go --include=internal/templates/forms_bench_test.go --include=internal/templates/context.md "$S/3a.patch"
gofmt -l internal/templates
go test ./internal/templates/ -count=1 2>&1 | tail -3
```

Expected: `gofmt` prints nothing; `ok`. The existing template tests pass unchanged, which is the evidence that splitting `Process` kept its behaviour.

- [ ] **Step 3: Commit**

```bash
git add internal/templates/forms.go internal/templates/forms_test.go internal/templates/forms_bench_test.go internal/templates/templates.go internal/templates/templatesfunctions.go internal/templates/testing_support.go internal/templates/context.md
git commit -F - <<'EOF'
feat(templates): narrow forms picked by the reader's width

Process renders for the reader's WrapWidth. A template wider than its reader
is replaced by the widest of its .wNN forms that fits. A screenwidth function
lets a template fit a rule or a paragraph to its reader.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git show --stat HEAD | tail -9
```

---

### Task 2: The guard, the table forms and the width-aware templates

- [ ] **Step 1: The guard, red**

```bash
R='TestEveryTemplateHasOneWidthClass|TestTemplateFormsAreNamed|TestFittingTemplates'
git apply --include=internal/guards/template_width_guard_test.go "$S/3a.patch"
go test ./internal/guards/ -run "$R" -count=1 -v 2>&1 | grep -E '^--- |_test.go:[0-9]+: \S+ at' | cut -c1-120
```

Expected: `TestFittingTemplatesFitAtEveryWidth` fails, naming `character/questup` (79 columns), `tables/generic` and `tables/shoplist` at 30 and 42, and `tables/generic` at 80; the other three tests pass.

- [ ] **Step 2: The templates**

```bash
git apply --include='_datafiles/world/dogmud/templates/*' "$S/3a.patch"
go test ./internal/guards/ -run "$R" -count=1 2>&1 | tail -1
go test ./internal/templates/ ./internal/usercommands/ -count=1 2>&1 | tail -2
```

Expected: `ok` three times.

- [ ] **Step 3: Commit**

```bash
T=_datafiles/world/dogmud/templates
git add internal/guards/template_width_guard_test.go $T/tables/generic.w40.template $T/tables/generic.w30.template $T/tables/shoplist.w40.template $T/tables/shoplist.w30.template $T/character/questup.template $T/character/skillup.template $T/character/statup.template $T/mail/message.template
git commit -F - <<'EOF'
feat(templates): table forms, width-aware banners, and the template guard

Tables get a 40-column plain form and a 30-column two-line form, with the
title and first cell wrapped. Banner rules and mail's paragraph follow the
reader's width, at most 80. A guard knows every template's width class.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git show --stat HEAD | tail -11
```

---

### Task 3: Splash and MOTD

- [ ] **Step 1: The tests, red**

```bash
git apply --include=internal/hooks/Splash_Deliver_test.go --include=internal/usercommands/motd_test.go "$S/3a.patch"
go test ./internal/hooks/ -run Splash -count=1 2>&1 | tail -2
go test ./internal/usercommands/ -run Motd -count=1 2>&1 | tail -4
```

Expected: hooks fails to build (`undefined: splashFor`); `TestMotdNarrowFormFitsTheScreen` fails with "still boxed" at 30, 42 and 44.

- [ ] **Step 2: The code**

```bash
git apply --include=internal/hooks/Splash_Deliver.go --include=internal/usercommands/motd.go "$S/3a.patch"
go test ./internal/hooks/ ./internal/usercommands/ -count=1 2>&1 | tail -2
```

Expected: `ok` twice.

- [ ] **Step 3: Commit**

```bash
git add internal/hooks/Splash_Deliver.go internal/hooks/Splash_Deliver_test.go internal/usercommands/motd.go internal/usercommands/motd_test.go
git commit -F - <<'EOF'
feat(screens): splash words on narrow screens, and the MOTD's narrow form

A reader whose screen is narrower than the art gets the scene's caption,
wrapped to the screen. The MOTD is a plain title and wrapped text below 45
columns, or when a word is wider than its box.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
```

---

### Task 4: Docs

- [ ] **Step 1:**

```bash
git apply --include=docs/README.md --include=docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md "$S/3a.patch"
cp "/c/Users/Calabe Davis/workspace/DOGMud/docs/superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md" docs/superpowers/plans/
git add docs/README.md docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md docs/superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md
git diff --cached --stat
git commit -F - <<'EOF'
docs(webclient): plan 3a, the 3a/3b split, and the docs index

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
git status --short
```

Expected: README 3 lines added; the spec 9 added and 8 removed; `git status` empty. If #841 has merged first and the README part fails to apply, add the three rows by hand with the Edit tool after the `no_entrance_guard_test.go` row.

---

### Task 5: Verify and ship

- [ ] **Step 1: Prove the guard and the mechanism can fail.** Undo each break exactly; `git status --short` must be empty at the end.

1. Write `_datafiles/world/dogmud/templates/generic/zz-unclassified.template` holding `hello`. Expected: `TestEveryTemplateHasOneWidthClass` names it. Delete it.
2. Write `tables/generic.w33.template` holding `x`. Expected: `TestTemplateFormsAreNamedForAKnownWidth` says 33 is not in `FormWidths`. Delete it.
3. Move `tables/generic.w30.template` to the scratchpad. Expected: `TestFittingTemplatesFitAtEveryWidth` reports `tables/generic` too wide at 30, 42 and 80. Move it back.
4. In `forms.go`, change `if WidestLine(out) <= width {` to `if false {`. Expected: `TestFormsPickTheWidestThatFits` fails. Undo.

- [ ] **Step 2: Gates**, in a worktree with no server running in it (a running server's saves make a lighting guard fail):

```bash
gofmt -l internal/ modules/
go build ./...
go test . ./internal/... ./modules/... -count=1 2>&1 | grep -vE '^ok|no test files' | tail -20
for t in tools/jstest/*.test.js tools/webclient-tests/*.js; do node "$t" > /dev/null 2>&1 || echo "RED $t"; done; echo done
golangci-lint run --new-from-merge-base=origin/master
go test ./internal/templates/ -run '^$' -bench BenchmarkTableAt -cpu 1 -benchmem -count 3
```

Expected: nothing from `gofmt` or the test filter, only `done`, `0 issues.`, and benchmark numbers near fact 18.

- [ ] **Step 3: Boot check** on private ports, as in the `dogmud-shipping` skill (`timeout 180`, exit 124, 0 panics, `Server Ready` 1). Offer the owner a telnet spot check: `set linewidth 40`, then `spells`, `list` in a shop, and `motd`.

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feature/webclient-base-3a-screen-machinery
gh pr create --repo pruuk/DOGMud --base master --head feature/webclient-base-3a-screen-machinery --title "Web client base 3a: screen machinery for narrow screens" --body-file "$S/pr3a-body.md"
```

Write the body first with the Write tool: what players see (the paragraph above the table forms), the seven departures, what was tested, "Part of #783", no closing keyword with any issue number, ending with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Read the URL `gh` prints and confirm it says `pruuk/DOGMud`. Before pushing, add this to the top of `docs/PATCH_NOTES.md` with the Edit tool, under the title, dated the day it ships, and commit it on its own: it is the one change any player can see.

```markdown
## <ship date, YYYY-MM-DD>: Tables that fit

- A table too wide for your screen, such as a long history or a crowded
  list of who is online, now shows in a narrower layout instead of running
  off the edge.
```

Merge only when the owner says.

---

## Self-review against the spec

| Spec item (section 2, 3a's part) | Where |
|---|---|
| Widest form that fits; forms beside the base named for their width; tried like the screen-reader file | Task 1 (`forms.go`, `process`) |
| Forms only for a template that has them, widest first, at most three renders for a screen; never on a per-round path | Task 1; `formsOf` remembers which exist; `FormWidths` has three widths |
| Measure with `util.VisibleWidth`; player text stays data | `WidestLine` |
| Measure the cost before merging; a cache only if it shows | Fact 18, departure 6 |
| Generated tables: bordered, plain (40), two lines per item (30) | Task 2 |
| Splash: full art or words only, by the caption path | Task 3 |
| `motdBox` narrow form | Task 3 |
| Template guard at 30, 42, 80 with an exempt list | Task 2; the classes; departure 5 |
| Rulings 24 and 25: nothing hidden behind a command; admin screens exempt | table forms show every cell; guard `admin` class |

## The patch

Applied in the tasks above, never by hand.

````diff
diff --git a/_datafiles/world/dogmud/templates/character/questup.template b/_datafiles/world/dogmud/templates/character/questup.template
index ceb81cf09..94c45ecfe 100644
--- a/_datafiles/world/dogmud/templates/character/questup.template
+++ b/_datafiles/world/dogmud/templates/character/questup.template
@@ -1,7 +1,7 @@
 
-<ansi fg="magenta-bold">*******************************************************************************</ansi>
+<ansi fg="magenta-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
 
 <ansi fg="yellow"> {{ . }}</ansi>
 <ansi fg="yellow"> type <ansi fg="command">quests</ansi> to track your progress on quests.</ansi>
 
-<ansi fg="magenta-bold">*******************************************************************************</ansi>
+<ansi fg="magenta-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
diff --git a/_datafiles/world/dogmud/templates/character/skillup.template b/_datafiles/world/dogmud/templates/character/skillup.template
index 911617a87..0f1a46994 100644
--- a/_datafiles/world/dogmud/templates/character/skillup.template
+++ b/_datafiles/world/dogmud/templates/character/skillup.template
@@ -1,9 +1,9 @@
 
-<ansi fg="yellow-bold">*******************************************************************************</ansi>
-<ansi fg="yellow-bold">*******************************************************************************</ansi>
+<ansi fg="yellow-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
+<ansi fg="yellow-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
 
 <ansi fg="yellow"> Your mastery of <ansi fg="skill">{{ .SkillName }}</ansi> has grown — you are now <ansi fg="yellow-bold">{{ skillRank .SkillLevel }}</ansi>!
 <ansi fg="yellow"> type <ansi fg="command">help {{ .SkillName }}</ansi> to learn more about using this skill.</ansi>
 
-<ansi fg="yellow-bold">*******************************************************************************</ansi>
-<ansi fg="yellow-bold">*******************************************************************************</ansi>
+<ansi fg="yellow-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
+<ansi fg="yellow-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
diff --git a/_datafiles/world/dogmud/templates/character/statup.template b/_datafiles/world/dogmud/templates/character/statup.template
index 83003486e..330e4860b 100644
--- a/_datafiles/world/dogmud/templates/character/statup.template
+++ b/_datafiles/world/dogmud/templates/character/statup.template
@@ -1,8 +1,8 @@
 
-<ansi fg="yellow-bold">*******************************************************************************</ansi>
-<ansi fg="yellow-bold">*******************************************************************************</ansi>
+<ansi fg="yellow-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
+<ansi fg="yellow-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
 
 <ansi fg="yellow"> {{ . }}</ansi>
 
-<ansi fg="yellow-bold">*******************************************************************************</ansi>
-<ansi fg="yellow-bold">*******************************************************************************</ansi>
+<ansi fg="yellow-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
+<ansi fg="yellow-bold">{{ repeat "*" (sub screenwidth 1) }}</ansi>
diff --git a/_datafiles/world/dogmud/templates/mail/message.template b/_datafiles/world/dogmud/templates/mail/message.template
index 1297d8d7b..1c898546c 100644
--- a/_datafiles/world/dogmud/templates/mail/message.template
+++ b/_datafiles/world/dogmud/templates/mail/message.template
@@ -5,7 +5,7 @@
 <ansi fg="mail-title{{ $readMarker }}">{{ t "Inbox.Sent" }}</ansi><ansi fg="mail-date{{ $readMarker }}">{{ .DateString }}</ansi>
 <ansi fg="mail-title{{ $readMarker }}">{{ t "Inbox.From" }}</ansi><ansi fg="username">{{ .FromName }}</ansi>
 
-<ansi fg="mail-title{{ $readMarker }}">{{ t "Inbox.Message" }}</ansi><ansi fg="mail-message{{ $readMarker }}">{{ splitstring .Message 71 "         " }}</ansi>
+<ansi fg="mail-title{{ $readMarker }}">{{ t "Inbox.Message" }}</ansi><ansi fg="mail-message{{ $readMarker }}">{{ splitstring .Message (sub screenwidth 9) "         " }}</ansi>
 {{ if gt .Gold 0 }}
 {{ $mapNoteGold := map "Gold" .Gold -}}
 <ansi fg="mail-note{{ $readMarker }}"><ansi fg="alert-4">{{ t "Inbox.Note" }}</ansi>{{ t "Inbox.NoteGold" $mapNoteGold }}</ansi>
diff --git a/_datafiles/world/dogmud/templates/tables/generic.w30.template b/_datafiles/world/dogmud/templates/tables/generic.w30.template
new file mode 100644
index 000000000..aea0ede0c
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/tables/generic.w30.template
@@ -0,0 +1,6 @@
+{{- $tableData := . -}}
+{{- $headers := .Header -}}
+<ansi fg="black-bold">.:</ansi> <ansi fg="table-title">{{ splitstring .Title (sub screenwidth 3) "   " }}</ansi>
+{{ range $rowIndex, $row := .Rows }}<ansi fg="{{ lowercase (index $headers 0) }}">{{ splitstring (index $row 0) screenwidth }}</ansi>
+{{ with $tableData.RowDetail $rowIndex }}  {{ splitstring . (sub screenwidth 2) "  " }}
+{{ end }}{{ end }}
diff --git a/_datafiles/world/dogmud/templates/tables/generic.w40.template b/_datafiles/world/dogmud/templates/tables/generic.w40.template
new file mode 100644
index 000000000..358d2dc10
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/tables/generic.w40.template
@@ -0,0 +1,7 @@
+{{- $tableData := . -}}
+{{- $headers := .Header -}}
+<ansi fg="black-bold">.:</ansi> <ansi fg="table-title">{{ splitstring .Title (sub screenwidth 3) "   " }}</ansi>
+<ansi fg="white">{{ range $i, $col := .Header }}{{ if $i }}  {{ end }}{{ $tableData.GetHeaderCell $i }}{{ end }}</ansi>
+{{ range $rowIndex, $row := .Rows }}
+{{- range $i, $col := $row }}{{ if $i }}  {{ end }}<ansi fg="{{ lowercase (index $headers $i) }}">{{ $tableData.GetCell $rowIndex $i }}</ansi>{{ end }}
+{{ end }}
diff --git a/_datafiles/world/dogmud/templates/tables/shoplist.w30.template b/_datafiles/world/dogmud/templates/tables/shoplist.w30.template
new file mode 100644
index 000000000..50d8b252c
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/tables/shoplist.w30.template
@@ -0,0 +1,6 @@
+{{- $tableData := . -}}
+{{- $headers := .Header -}}
+<ansi fg="black-bold">.:</ansi> <ansi fg="table-title">{{ splitstring .Title (sub screenwidth 3) "   " }}</ansi>
+{{ range $rowIndex, $row := .Rows }}<ansi fg="shop-{{ lowercase (index $headers 0) }}">{{ splitstring (index $row 0) screenwidth }}</ansi>
+{{ with $tableData.RowDetail $rowIndex }}  {{ splitstring . (sub screenwidth 2) "  " }}
+{{ end }}{{ end }}
diff --git a/_datafiles/world/dogmud/templates/tables/shoplist.w40.template b/_datafiles/world/dogmud/templates/tables/shoplist.w40.template
new file mode 100644
index 000000000..7cc95718d
--- /dev/null
+++ b/_datafiles/world/dogmud/templates/tables/shoplist.w40.template
@@ -0,0 +1,7 @@
+{{- $tableData := . -}}
+{{- $headers := .Header -}}
+<ansi fg="black-bold">.:</ansi> <ansi fg="table-title">{{ splitstring .Title (sub screenwidth 3) "   " }}</ansi>
+<ansi fg="white">{{ range $i, $col := .Header }}{{ if $i }}  {{ end }}{{ $tableData.GetHeaderCell $i }}{{ end }}</ansi>
+{{ range $rowIndex, $row := .Rows }}
+{{- range $i, $col := $row }}{{ if $i }}  {{ end }}<ansi fg="shop-{{ lowercase (index $headers $i) }}">{{ $tableData.GetCell $rowIndex $i }}</ansi>{{ end }}
+{{ end }}
diff --git a/docs/README.md b/docs/README.md
index de8a98990..9b426bac5 100644
--- a/docs/README.md
+++ b/docs/README.md
@@ -248,6 +248,9 @@ than reading an old result:
 | [`../internal/hooks/gossip_common_test.go`](../internal/hooks/gossip_common_test.go) | Pins common talk (#375): a fact tagged `common` reaches every gossiper's ordinary gossip as one slot among its own facts, and never the hint timer |
 | [`../internal/guards/closing_step_guard_test.go`](../internal/guards/closing_step_guard_test.go) | #375 closing step guards: `TestLoopBacklogStaysRetired` (no `Ratchet`, `ParseBacklog`, `FormatBacklog`, `LoadBacklog`, `BacklogPath` or `loop-backlog.txt` in the Go source) and `TestRoomInteractStaysRetired` (no `room_interact` event or quest `verb` in shipped content or code), each with a fixture proving it can fail |
 | [`../internal/guards/command_parity_guard_test.go`](../internal/guards/command_parity_guard_test.go) | Owner ruling 2026-11-07: the boot's command parity audit, run on the real user and mob registries with modules loaded, finds only `throw` (the standing reminder that a mob throw is owed); every other one-sided command is allowlisted with its reason in `internal/actions/divergences.go`. Includes a fixture proving the audit reports an unlisted gap |
+| [`superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md`](superpowers/plans/2026-11-10-webclient-base-3a-screen-machinery.md) | Plan 3a of the web client base: narrow template forms picked by width, the `screenwidth` template function, table forms, splash to words only on narrow screens, the MOTD's narrow form, and the template width guard |
+| [`../internal/templates/forms.go`](../internal/templates/forms.go) | Narrow forms: `FormWidths`, `WidestLine`, the reader's width and the widest-form-that-fits rule behind `templates.Process` |
+| [`../internal/guards/template_width_guard_test.go`](../internal/guards/template_width_guard_test.go) | Ruling 3 of the web client base: every template has one width class (fits at 30, 42 and 80, words-only splash, map or wide table, raw login text, wrapped, plan 3b, help, admin, or no caller), form files are named for a known width, and the fitting ones are rendered and measured |
 | [`../internal/guards/no_entrance_guard_test.go`](../internal/guards/no_entrance_guard_test.go) | Owner ruling 2026-11-07: loading the shipped world, no room is left that no exit leads into unless `rooms.noEntranceByDesign` lists how players reach it (tutorial room, instance template, ferries) |
 | [`../internal/rooms/move_missing_room_test.go`](../internal/rooms/move_missing_room_test.go) | Entering the world into a room that no longer exists lands at the start room; an ordinary move to a missing room still refuses |
 | [`../internal/web/not_found_test.go`](../internal/web/not_found_test.go) | A refused `_` file that is not a page gets one plain 404 (no second status), an ordinary image is still served, and the mutation fallback picture is `generic.png`, which the `_` rule would refuse under its old name |
diff --git a/docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md b/docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md
index 06e9fef52..e17cfe802 100644
--- a/docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md
+++ b/docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md
@@ -131,9 +131,9 @@ Text already on screen keeps its line breaks. New text uses the new width.
 
 ### 2. Templates to 30 columns
 
-A desktop does not change: at 80 columns the widest form is used. Narrower forms appear only on screens that cannot fit it. A template may have several forms, and **the game uses the widest form whose longest line fits the session's width.** No per-device rules or width thresholds are written down, so a shop list with short item names stays bordered on a narrower screen than one with long names.
+A desktop does not change: at 80 columns the widest form is used. Narrower forms appear only on screens that cannot fit it, and on a desktop only for a table already wider than 80 columns, which wraps mid-row today (owner, 2026-11-10). A template may have several forms, and **the game uses the widest form whose longest line fits the session's width.** No per-device rules or width thresholds are written down, so a shop list with short item names stays bordered on a narrower screen than one with long names.
 
-Variants sit beside the base file and are named for the width they are drawn to, such as `character/status.w42.template` and `.w30.template`, tried the way `name.screenreader.template` is. The width joins `templateConfig`, and a width report clears the cache with `ClearTemplateConfigCache`. Forms are only rendered and compared when the session is under 80 columns.
+Variants sit beside the base file and are named for the width they are drawn to, such as `character/status.w42.template` and `.w30.template`, tried the way `name.screenreader.template` is. The width joins `templateConfig`, and a width report clears the cache with `ClearTemplateConfigCache`. Forms are only rendered and compared when the base is wider than the session, and only for a template that has forms (owner, 2026-11-10).
 
 Forms by use case, as chosen against real devices (the columns are the usual ones for those devices):
 
@@ -146,16 +146,16 @@ Forms by use case, as chosen against real devices (the columns are the usual one
 
 Every form shows all of the information: no screen moves content behind a separate command.
 
-Measuring limits. `Process` reads and parses a template on every call (`internal/templates/templates.go:212-214`), so each extra form tried costs one more read, parse and execute. Forms are tried only for a session under 80 columns and only for a template that has variants, widest first, stopping at the first that fits, so a phone pays at most three renders for a player-requested screen. Measuring never runs on a per-round or per-broadcast path. Plan 3 measures the cost on the production-sized machine limit before merging, and adds a parsed-template cache if it shows up. Measuring counts columns with `util.VisibleWidth` and executes nothing new: player text is passed as template data, as it is today, and `text/template` does not evaluate data as a template.
+Measuring limits. `Process` reads and parses a template on every call (`internal/templates/templates.go:212-214`), so each extra form tried costs one more read, parse and execute. Forms are tried only when the base is wider than the session and only for a template that has variants, widest first, stopping at the first that fits, so a phone pays at most three renders for a player-requested screen. Measuring never runs on a per-round or per-broadcast path. Plan 3a measures the cost on the production-sized machine limit before merging, and adds a parsed-template cache if it shows up. Measuring counts columns with `util.VisibleWidth` and executes nothing new: player text is passed as template data, as it is today, and `text/template` does not evaluate data as a template.
 
 Rendered at 30, 42 and 80 columns, no line may exceed the width and no column may break, apart from the exemption in ruling 3.
 
 Three kinds of wrapping sit inside templates, outside plan 1's sweep, and belong to plans 3 and 4:
-- **Fixed widths in templates.** 11 lines in 10 template files call `splitstring` with a literal 59 to 73 (plan 3), and `setdesc.go` echoes at 72 to match them.
+- **Fixed widths in templates.** 11 lines in 10 template files call `splitstring` with a literal 59 to 73 (plan 3b; `screenwidth` from plan 3a replaces the literal), and `setdesc.go` echoes at 72 to match them.
 - **Hard line breaks in authored text.** Of 468 help files, 434 have prose broken by hand into lines of 45 columns or more, and help rides `CategoryScreen`, which the pipeline does not wrap. So plan 4 covers every help page's prose as well as the 170 aligned ones: on a session under 80 columns, the lines of a paragraph are joined and wrapped to the reader's width.
 - **Rendering without a reader.** `help.go:222` calls `Process` with user id 0, so plan 4 passes the reader through.
 
-`motdBox` needs a narrow form (plan 3): seen at 42 columns on 2026-11-09, its spaced-out title is cut off, and URLs too long for the box burst through its sides. `motdrestyle.js` matches the box by its shape and did not remove it at that width, so plan 5 checks it at narrow widths.
+`motdBox` needs a narrow form (plan 3a): seen at 42 columns on 2026-11-09, its spaced-out title is cut off, and URLs too long for the box burst through its sides. `motdrestyle.js` matches the box by its shape and did not remove it at that width, so plan 5 checks it at narrow widths.
 
 A 320px screen at 16px fits only 29 columns because of the game window's padding, so that padding is trimmed at that width to reach 30.
 
@@ -215,9 +215,10 @@ One spec, one plan per slice. Each ships on its own and leaves the game working.
 |---|---|---|---|
 | 1 | Server wrap width: `WrapWidth`, the `Client.Display` handler, the sweep, telnet golden test | Almost nothing: a few line breaks at the end of lines move, and a saved `set linewidth` now also applies to speech, whispers, room descriptions, menus and `set` | None |
 | 2 | Type system: JetBrains Mono, size tokens, no italics, button weights, top and bottom bars, the site pages, brand standard | The client and site look like one design. Game text still changes size with the window | None |
-| 3 | Screens and tables to 30 columns: the form mechanism, generated tables, the 58 hard layouts, the template guard | Nothing yet, since no client reports a narrow width | 1 |
-| 4 | Help pages to 30 columns, in batches: the aligned details, and paragraph rewrapping for hand-broken prose | Help details change form at every width | 3 |
-| 5 | Game window goes live: fixed text size, columns follow the window, the client reports its width | Readable phone text, and 200% zoom works | 1, 2, 3, 4 |
+| 3a | Screen machinery: the form mechanism, `screenwidth`, generated tables, splash to words only, the MOTD box, the template guard (split from plan 3, owner 2026-11-10) | Nothing yet, since no client reports a narrow width | 1 |
+| 3b | The hand-laid screens: narrow forms for status, inventory, skills and the other character and description screens, mail, the skill and stat banners, the auction broadcasts, the Go-built screens, and every table now wider than 80 columns redesigned to fit (owner, 2026-11-10), shown as a mock first | Nothing yet | 3a |
+| 4 | Help pages to 30 columns, in batches: the aligned details, and paragraph rewrapping for hand-broken prose | Help details change form at every width | 3a |
+| 5 | Game window goes live: fixed text size, columns follow the window, the client reports its width | Readable phone text, and 200% zoom works | 1, 2, 3a, 3b, 4 |
 | 6 | Layout and phones: large-screen layout, sideways tablets, retiring rail and column dragging, 30% tabs, keyboard rules, X, toast | The new arrangement | 2, 5 |
 | 7 | Panels: Map title and bar, Quests tabs, Comms rename, compressed Equipped, item menus by tap and right-click | The panel changes | 2, 6 |
 
diff --git a/internal/guards/template_width_guard_test.go b/internal/guards/template_width_guard_test.go
new file mode 100644
index 000000000..11782d31b
--- /dev/null
+++ b/internal/guards/template_width_guard_test.go
@@ -0,0 +1,301 @@
+package guards
+
+import (
+	"io/fs"
+	"os"
+	"path/filepath"
+	"regexp"
+	"sort"
+	"strconv"
+	"strings"
+	"testing"
+
+	"github.com/GoMudEngine/GoMud/internal/configs"
+	"github.com/GoMudEngine/GoMud/internal/language"
+	"github.com/GoMudEngine/GoMud/internal/messaging"
+	"github.com/GoMudEngine/GoMud/internal/mudlog"
+	"github.com/GoMudEngine/GoMud/internal/templates"
+	textlang "golang.org/x/text/language"
+)
+
+// Every template works down to 30 columns (web client base spec, ruling 3),
+// apart from maps and wide tables. This guard knows every template, in the
+// live world and in every module, and why it is safe at 30. It renders the
+// ones that must fit at 30, 42 and 80.
+//
+// The classes are the state of the work, not a permanent exemption list:
+// plan 3b empties "plan 3b", plan 4 does "help/". When both are done, only
+// "wide" and the classes that never reach a narrow screen unwrapped remain.
+
+// templateFolders are every place a template is read from: the live world,
+// and each module's own files (modules/<name>/files/datafiles/templates).
+func templateFolders(t *testing.T) []string {
+	t.Helper()
+	root := repoRoot(t)
+	folders := []string{filepath.Join(root, "_datafiles", "world", "dogmud", "templates")}
+	modules, err := filepath.Glob(filepath.Join(root, "modules", "*", "files", "datafiles", "templates"))
+	if err != nil {
+		t.Fatal(err)
+	}
+	if len(modules) == 0 {
+		t.Fatal("found no module template folders; the glob is not reaching them")
+	}
+	return append(folders, modules...)
+}
+
+// templateWidths are the widths every fitting template is rendered at: the
+// narrowest screen, an upright phone, and a desktop.
+var templateWidths = []int{30, 42, 80}
+
+// templateClassNames lists the templates in each exact-name class.
+var templateClassNames = map[string][]string{
+	// Maps and wide tables may scroll sideways (ruling 3).
+	"wide": {"maps/map", "tables/lockpicking"},
+	// Sent outside the pipeline, before a reader has a width or as they
+	// leave; the terminal soft-wraps them.
+	"raw": {"goodbye", "login/connect-splash", "login/email-new.prompt",
+		"login/password-new-verify.prompt", "login/password-new.prompt",
+		"login/password.mask", "login/password.prompt", "login/username-new.prompt",
+		"login/username.prompt", "generic/prompt.yn"},
+	// Wrapped to the reader by the caller (actions/roster.go) or the
+	// pipeline (CategoryLogout).
+	"wrapped": {"descriptions/who", "descriptions/ontheground", "player-despawn"},
+	// Narrow forms come in plan 3b.
+	"plan 3b": {"character/conditions", "character/cooldowns",
+		"character/description", "character/description-corpse",
+		"character/inventory", "character/inventory-look", "character/pet",
+		"character/quests", "character/skills", "character/skillup",
+		"character/statup", "character/status", "character/storage",
+		"character/title", "descriptions/biome", "descriptions/identify",
+		"descriptions/insidecontainer", "descriptions/track",
+		"descriptions/exits", "descriptions/fixtures", "descriptions/room",
+		"descriptions/room-title", "descriptions/rune", "descriptions/sign",
+		"mail/message", "auctions/auction-bid", "auctions/auction-end",
+		"auctions/auction-start", "auctions/auction-update"},
+	// No production code renders these (survey 2026-11-10).
+	"no caller": {"character/experience", "character/jobs",
+		"character/permadeath", "descriptions/train", "login/character-name.prompt",
+		"login/newuser.prompt", "maps/secret",
+		"generic/moon_eye_full", "generic/moon_eye_new", "generic/moon_swiftmoon_full",
+		"generic/moon_swiftmoon_new", "generic/moon_wanderer_full",
+		"generic/moon_wanderer_new", "generic/sunrise", "generic/sunset"},
+}
+
+// templateClassPrefixes are the classes held by a folder.
+var templateClassPrefixes = map[string]string{
+	"generic/splash/": "caption", // narrow readers get the scene's words (Splash_Deliver)
+	"help/":           "help",    // plan 4
+	"admincommands/":  "admin",   // out of scope (ruling 25)
+}
+
+// templateFixture is data that makes a template as wide as it gets in play.
+// wrapped says the template goes out on a category the pipeline wraps to the
+// reader, so the guard wraps it the same way before measuring.
+type templateFixture struct {
+	data    func() any
+	wrapped bool
+}
+
+// templateFits are the templates that must fit at every width.
+var templateFits = map[string]templateFixture{
+	"tables/generic": {data: func() any {
+		return templates.GetTable("Lost and found",
+			[]string{"Item", "Where", "Since"},
+			[][]string{
+				{"the Ancient Lantern of the Drowned Lighthouse", "the Thornwall Temple District", "two days"},
+				{"a rope", "Main Street", "today"},
+			})
+	}},
+	"tables/shoplist": {data: func() any {
+		return templates.GetTable("Items available by Grumbleguts the Provisioner",
+			[]string{"Qty", "Name", "Type", "Price"},
+			[][]string{
+				{"3", "a flask of clear river water", "drinkable", "12 gold"},
+				{"1", "a lantern", "light", "40 gold"},
+			})
+	}},
+	"character/questup": {data: func() any { return "You have found a new quest: The Long Road Home." }, wrapped: true},
+}
+
+// templateClasses names every class that holds name.
+func templateClasses(name string) []string {
+	var hits []string
+	if _, ok := templateFits[name]; ok {
+		hits = append(hits, "fits")
+	}
+	for class, names := range templateClassNames {
+		for _, n := range names {
+			if n == name {
+				hits = append(hits, class)
+			}
+		}
+	}
+	for prefix, class := range templateClassPrefixes {
+		if strings.HasPrefix(name, prefix) {
+			hits = append(hits, class)
+		}
+	}
+	return hits
+}
+
+var templateFormFile = regexp.MustCompile(`^(.+)\.w(\d+)\.template$`)
+
+// templateBases walks every template folder and returns every base name (a
+// form or a screen-reader file belongs to its base) and the widths of every
+// form file, per base.
+func templateBases(t *testing.T) (bases map[string]bool, forms map[string][]int) {
+	t.Helper()
+	bases, forms = map[string]bool{}, map[string][]int{}
+	for _, root := range templateFolders(t) {
+		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
+			if err != nil || d.IsDir() {
+				return err
+			}
+			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(os.PathSeparator)))
+			if m := templateFormFile.FindStringSubmatch(rel); m != nil {
+				n, _ := strconv.Atoi(m[2])
+				forms[m[1]] = append(forms[m[1]], n)
+				return nil
+			}
+			name := strings.TrimSuffix(strings.TrimSuffix(rel, ".template"), ".md")
+			bases[strings.TrimSuffix(name, ".screenreader")] = true
+			return nil
+		})
+		if err != nil {
+			t.Fatal(err)
+		}
+	}
+	if len(bases) < 500 {
+		t.Fatalf("found only %d templates; the walk is not reaching the folders", len(bases))
+	}
+	return bases, forms
+}
+
+func TestEveryTemplateHasOneWidthClass(t *testing.T) {
+	bases, _ := templateBases(t)
+	for name := range bases {
+		switch hits := templateClasses(name); len(hits) {
+		case 1:
+		case 0:
+			t.Errorf("%s: no width class. Make it fit at 30 columns and add a fixture to templateFits, or name why it is safe", name)
+		default:
+			t.Errorf("%s: in more than one class: %v", name, hits)
+		}
+	}
+	listed := []string{}
+	for name := range templateFits {
+		listed = append(listed, name)
+	}
+	for _, names := range templateClassNames {
+		listed = append(listed, names...)
+	}
+	for _, name := range listed {
+		if !bases[name] {
+			t.Errorf("%s is classified but no such template exists", name)
+		}
+	}
+}
+
+func TestTemplateFormsAreNamedForAKnownWidth(t *testing.T) {
+	bases, forms := templateBases(t)
+	known := map[int]bool{}
+	for _, w := range templates.FormWidths {
+		known[w] = true
+	}
+	for base, widths := range forms {
+		for _, w := range widths {
+			if !known[w] {
+				t.Errorf("%s.w%d.template: %d is not in templates.FormWidths %v, so it is never read", base, w, w, templates.FormWidths)
+			}
+			if !bases[base] {
+				t.Errorf("%s.w%d.template has no base template", base, w)
+			}
+		}
+	}
+}
+
+// templateRenderSetup renders the live world's templates with its English
+// labels, as the server does.
+func templateRenderSetup(t *testing.T) {
+	t.Helper()
+	mudlog.SetupLogger(nil, `LOW`, ``, false)
+	language.InitTranslation(language.BundleCfg{
+		DefaultLanguage: textlang.English,
+		Language:        textlang.English,
+		LanguagePaths:   []string{"_datafiles/localize", "_datafiles/world/dogmud/localize"},
+	})
+	prev := configs.GetFilePathsConfig().DataFiles.String()
+	if err := configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": dogmudDataFilesRoot}); err != nil {
+		t.Fatal(err)
+	}
+	t.Cleanup(func() { _ = configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": prev}) })
+	templates.SetFSForTest(t, os.DirFS(dogmudDataFilesRoot).(fs.ReadFileFS))
+}
+
+func TestFittingTemplatesFitAtEveryWidth(t *testing.T) {
+	templateRenderSetup(t)
+	names := make([]string, 0, len(templateFits))
+	for name := range templateFits {
+		names = append(names, name)
+	}
+	sort.Strings(names)
+	for _, name := range names {
+		for _, width := range templateWidths {
+			fixture := templateFits[name]
+			out, err := templates.ProcessAtWidth(name, fixture.data(), width)
+			if err != nil {
+				t.Errorf("%s at %d: %v", name, width, err)
+				continue
+			}
+			if fixture.wrapped {
+				out = messaging.WrapAnsi(out, width)
+			}
+			if strings.TrimSpace(out) == "" {
+				t.Errorf("%s at %d rendered nothing; the fixture proves nothing", name, width)
+				continue
+			}
+			if got := templates.WidestLine(out); got > width {
+				t.Errorf("%s at %d columns has a line %d wide:\n%s", name, width, got, out)
+			}
+		}
+	}
+}
+
+// A desktop sees what it always did: the width-aware banners draw their old
+// 79-column rules at 80 and above, and a table that fits keeps its borders.
+func TestFittingTemplatesKeepTheirFullWidthLook(t *testing.T) {
+	templateRenderSetup(t)
+	looks := map[string]any{
+		"character/questup": "You have found a new quest.",
+		"character/skillup": struct {
+			SkillName  string
+			SkillLevel int
+		}{"brawling", 2},
+		"character/statup": "Your strength has grown.",
+		"tables/generic": templates.GetTable("Spells",
+			[]string{"SpellId", "Name", "Target", "Cost"},
+			[][]string{{"aurora", "Aurora of the Slumbering Deep", "group", "12"}}),
+		"tables/shoplist": templateFits["tables/shoplist"].data(),
+	}
+	render := func(name string, width int) string {
+		out, err := templates.ProcessAtWidth(name, looks[name], width)
+		if err != nil {
+			t.Fatalf("%s: %v", name, err)
+		}
+		return out
+	}
+	stars := strings.Repeat("*", 79)
+	for _, name := range []string{"character/questup", "character/skillup", "character/statup"} {
+		for _, width := range []int{80, 120} {
+			out := render(name, width)
+			if !strings.Contains(out, stars) || strings.Contains(out, stars+"*") {
+				t.Errorf("%s at %d does not draw its 79-column rule", name, width)
+			}
+		}
+	}
+	for _, name := range []string{"tables/generic", "tables/shoplist"} {
+		if !strings.Contains(render(name, 80), "╒") {
+			t.Errorf("%s at 80 lost its border", name)
+		}
+	}
+}
diff --git a/internal/hooks/Splash_Deliver.go b/internal/hooks/Splash_Deliver.go
index bdb1ec358..5515f5b59 100644
--- a/internal/hooks/Splash_Deliver.go
+++ b/internal/hooks/Splash_Deliver.go
@@ -26,16 +26,30 @@ func Splash_Deliver(e events.Event) events.ListenerReturn {
 	// (per-recipient line wrapping happens later in SendText).
 	text, terr := templates.Process(splashTemplatePath(evt.SceneId), evt.Data)
 	artOK := terr == nil && text != ""
+	artWidth := templates.WidestLine(text)
 
 	for _, u := range splash.Recipients(evt) {
-		// Screen-reader users get the caption only — never the art.
-		if u.ScreenReader || !artOK {
-			if evt.Caption != "" {
-				u.SendText(messaging.CategorySplash, evt.Caption)
-			}
-			continue
+		if out, ok := splashFor(text, artWidth, artOK, u.ScreenReader, u.WrapWidth(), evt.Caption); ok {
+			u.SendText(messaging.CategorySplash, out)
 		}
-		u.SendText(messaging.CategorySplash, text)
 	}
 	return events.Continue
 }
+
+// splashFor is what one reader is sent: the art when it fits their screen,
+// otherwise the caption, words only (web client base spec, section 2: there
+// is no small art). CategorySplash is not wrapped by the pipeline, so the
+// caption is wrapped here; screen-reader users get it as it is. ok is false
+// when there is nothing to send.
+func splashFor(art string, artWidth int, artOK, screenReader bool, width int, caption string) (string, bool) {
+	if artOK && !screenReader && artWidth <= width {
+		return art, true
+	}
+	if caption == "" {
+		return "", false
+	}
+	if screenReader {
+		return caption, true
+	}
+	return messaging.WrapAnsi(caption, width), true
+}
diff --git a/internal/hooks/Splash_Deliver_test.go b/internal/hooks/Splash_Deliver_test.go
index b3201ee09..0d36d2a23 100644
--- a/internal/hooks/Splash_Deliver_test.go
+++ b/internal/hooks/Splash_Deliver_test.go
@@ -1,9 +1,11 @@
 package hooks
 
 import (
+	"strings"
 	"testing"
 
 	"github.com/GoMudEngine/GoMud/internal/events"
+	"github.com/GoMudEngine/GoMud/internal/templates"
 	"github.com/stretchr/testify/assert"
 )
 
@@ -16,6 +18,38 @@ type notASplashEvent struct{}
 
 func (notASplashEvent) Type() string { return "dummy" }
 
+func TestSplashArtOnlyWhereItFits(t *testing.T) {
+	const art, caption = "art", "The sun rises."
+	for _, tc := range []struct {
+		name          string
+		artOK, sr     bool
+		width         int
+		want          string
+		wantSomething bool
+	}{
+		{"a full-width screen gets the art", true, false, 80, art, true},
+		{"art exactly as wide as the screen", true, false, 76, art, true},
+		{"a narrow screen gets the words", true, false, 75, caption, true},
+		{"a screen reader gets the words", true, true, 80, caption, true},
+		{"failed art falls back to the words", false, false, 80, caption, true},
+	} {
+		got, ok := splashFor(art, 76, tc.artOK, tc.sr, tc.width, caption)
+		assert.Equal(t, tc.want, got, tc.name)
+		assert.Equal(t, tc.wantSomething, ok, tc.name)
+	}
+	_, ok := splashFor(art, 76, true, false, 40, "")
+	assert.False(t, ok, "a narrow screen with no caption is sent nothing")
+
+	long := "A shimmer passes over your skin as the change settles in. You feel the mutation take hold."
+	got, ok := splashFor(art, 76, true, false, 30, long)
+	assert.True(t, ok)
+	for _, line := range strings.Split(got, "\n") {
+		assert.LessOrEqual(t, templates.WidestLine(line), 30, "the caption is wrapped to the screen: %q", line)
+	}
+	got, _ = splashFor(art, 76, true, true, 30, long)
+	assert.Equal(t, long, got, "a screen reader gets the caption as it is")
+}
+
 func TestSplashDeliverWrongEventType(t *testing.T) {
 	// A non-Splash event is ignored (Continue), no panic.
 	assert.Equal(t, events.Continue, Splash_Deliver(notASplashEvent{}))
diff --git a/internal/templates/context.md b/internal/templates/context.md
index bc3422a71..156f140f9 100644
--- a/internal/templates/context.md
+++ b/internal/templates/context.md
@@ -139,6 +139,31 @@ Cached template with metadata for efficient reuse and cache invalidation.
 - **Alternative Layouts**: Screen reader optimized template variants
 - **Configurable Output**: User-controllable output formatting
 
+### Narrow forms and the reader's width (`forms.go`)
+- `Process` renders for the reader's width, `users.UserRecord.WrapWidth()`
+  (`FullWidth`, 80, with no reader). The width is read on every call, never
+  cached, so a resized screen or a new `set linewidth` takes effect at once.
+- The `screenwidth` template function returns that width, capped at
+  `FullWidth`, so a template can fit a rule or a wrapped paragraph to its
+  reader: `{{ repeat "*" (sub screenwidth 1) }}`,
+  `{{ splitstring .X (sub screenwidth 9) "  " }}`. `min` is there for caps:
+  `(min 67 (sub screenwidth 12))`.
+- A `.template` may have narrow forms beside it, named for the width they are
+  drawn to: `tables/generic.w40.template`, `tables/generic.w30.template`. Only
+  widths in `FormWidths` (42, 40, 30) are read. When the base output's widest
+  line (`WidestLine`) is wider than the reader, the forms are rendered widest
+  first and the first that fits is used, or the narrowest one found. A base
+  that fits is always used. Which forms a template has is found once and
+  remembered (`formsOf`); a form file added while the server runs is seen
+  after a restart.
+- A screen-reader file still comes first and has no forms. A `.md` file has
+  no forms.
+- `TemplateTable.RowDetail` gives a row's other cells as `Header: value`
+  pairs for the two-line table form.
+- `ProcessAtWidth` (testing_support.go) renders at a given width for guards.
+- `internal/guards/template_width_guard_test.go` knows every template and why
+  it is safe at 30 columns, and renders the ones that must fit at 30, 42, 80.
+
 ### Multi-Source Loading
 - **Plugin Templates**: Templates from plugin file systems
 - **Core Templates**: Built-in game templates
diff --git a/internal/templates/forms.go b/internal/templates/forms.go
new file mode 100644
index 000000000..d40e1806c
--- /dev/null
+++ b/internal/templates/forms.go
@@ -0,0 +1,115 @@
+package templates
+
+import (
+	"os"
+	"strconv"
+	"strings"
+	"sync"
+	"text/template"
+
+	"github.com/GoMudEngine/GoMud/internal/configs"
+	"github.com/GoMudEngine/GoMud/internal/users"
+	"github.com/GoMudEngine/GoMud/internal/util"
+)
+
+// Narrow forms (web client base spec, section 2). A template may have forms
+// drawn for narrower screens beside it, named for the width they are drawn
+// to: character/status.w42.template, character/status.w30.template. When the
+// base output is wider than the reader's screen, Process renders the forms
+// widest first and uses the first whose longest line fits, or the narrowest
+// if none does (ruling 22: the widest form that fits). A base that fits is
+// always used, so a desktop sees a form only where the base was already
+// wider than its 80 columns.
+
+// FullWidth is the widest a screen is laid out for.
+const FullWidth = users.MaxAutoWrapWidth
+
+// FormWidths are the widths a form may be drawn to, widest first. A file
+// named for any other width is never read; internal/guards holds that.
+var FormWidths = []int{42, 40, 30}
+
+// formCache remembers which forms each template has, so a template with none
+// costs nothing after its first narrow render. A form file added while the
+// server runs is seen after a restart. SetFSForTest clears it.
+var formCache sync.Map // fname -> []int, widest first
+
+// FormPath is the file name of fname's form drawn to width.
+func FormPath(fname string, width int) string {
+	return util.FilePath(`templates/`, fname+`.w`+strconv.Itoa(width)+`.template`)
+}
+
+// formsOf lists the widths fname has a form drawn to, widest first.
+func formsOf(fname string) []int {
+	if have, ok := formCache.Load(fname); ok {
+		return have.([]int)
+	}
+	have := []int{}
+	for _, width := range FormWidths {
+		if formExists(FormPath(fname, width)) {
+			have = append(have, width)
+		}
+	}
+	formCache.Store(fname, have)
+	return have
+}
+
+// formExists looks for path where Process would read it: a registered
+// filesystem first, then the data files on disk.
+func formExists(path string) bool {
+	for _, f := range fileSystems {
+		if file, err := f.Open(path); err == nil {
+			_ = file.Close()
+			return true
+		}
+	}
+	_, err := os.Stat(util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/`, path))
+	return err == nil
+}
+
+// readerWidth is the width a reader's screen has room for: the user's
+// WrapWidth, or FullWidth when there is no reader.
+func readerWidth(userId int) int {
+	if userId > 0 {
+		if u := users.GetByUserId(userId); u != nil {
+			return u.WrapWidth()
+		}
+	}
+	return FullWidth
+}
+
+// widthFuncs replaces the screenwidth template function for one render, so a
+// template can fit a rule or a wrapped paragraph to its reader. Layouts are
+// drawn for at most FullWidth, so a telnet reader with a wider saved line
+// width sees them as before.
+func widthFuncs(width int) template.FuncMap {
+	width = min(width, FullWidth)
+	return template.FuncMap{"screenwidth": func() int { return width }}
+}
+
+// WidestLine is the display width of the longest line in s, colour tags
+// not counted.
+func WidestLine(s string) int {
+	widest := 0
+	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
+		widest = max(widest, util.VisibleWidth(line))
+	}
+	return widest
+}
+
+// narrowestFit renders fname's forms widest first and returns the first that
+// fits width, or the narrowest one. base is the base form's output, returned
+// when the template has no forms.
+func narrowestFit(fname string, data any, tplConfig templateConfig, width int, base string, forms []int) string {
+	best := base
+	for _, formWidth := range forms {
+		out, found, err := renderFile(templateDetails{name: fname, path: FormPath(fname, formWidth)}, data, tplConfig, width)
+		if !found || err != nil {
+			continue
+		}
+		best = out
+		if WidestLine(out) <= width {
+			return out
+		}
+	}
+	return best
+}
diff --git a/internal/templates/forms_bench_test.go b/internal/templates/forms_bench_test.go
new file mode 100644
index 000000000..a0117dfd3
--- /dev/null
+++ b/internal/templates/forms_bench_test.go
@@ -0,0 +1,40 @@
+package templates
+
+import (
+	"strconv"
+	"testing"
+)
+
+// The spec asks plan 3a to measure what trying narrow forms costs before it
+// merges (section 2, "Measuring limits"). Run on one CPU, as production has:
+//
+//	go test ./internal/templates/ -run '^$' -bench BenchmarkTableAt -cpu 1 -benchmem
+//
+// At 80 the base table renders once. At 30 the base, the 40-column form and
+// the 30-column form each render: the worst case a player-requested screen
+// pays.
+func benchTable() TemplateTable {
+	rows := make([][]string, 0, 20)
+	for i := 0; i < 20; i++ {
+		rows = append(rows, []string{"spell" + strconv.Itoa(i), "Aurora of the Slumbering Deep", "group", "12"})
+	}
+	return GetTable("Spells", []string{"SpellId", "Name", "Target", "Cost"}, rows)
+}
+
+func BenchmarkTableAt80(b *testing.B) {
+	data := benchTable()
+	for i := 0; i < b.N; i++ {
+		if _, err := process("tables/generic", data, templateConfig{}, 80); err != nil {
+			b.Fatal(err)
+		}
+	}
+}
+
+func BenchmarkTableAt30(b *testing.B) {
+	data := benchTable()
+	for i := 0; i < b.N; i++ {
+		if _, err := process("tables/generic", data, templateConfig{}, 30); err != nil {
+			b.Fatal(err)
+		}
+	}
+}
diff --git a/internal/templates/forms_test.go b/internal/templates/forms_test.go
new file mode 100644
index 000000000..e03e6cd1d
--- /dev/null
+++ b/internal/templates/forms_test.go
@@ -0,0 +1,108 @@
+package templates
+
+import (
+	"strconv"
+	"strings"
+	"testing"
+	"testing/fstest"
+
+	"github.com/GoMudEngine/GoMud/internal/util"
+	"github.com/stretchr/testify/assert"
+	"github.com/stretchr/testify/require"
+)
+
+// formsFS holds a template "t" 50 columns wide with forms drawn to 42 and 30,
+// and "plain" with no forms.
+func formsFS() fstest.MapFS {
+	files := map[string]string{
+		"templates/t.template":        strings.Repeat("b", 50),
+		"templates/t.w42.template":    strings.Repeat("f", 40),
+		"templates/t.w30.template":    strings.Repeat("n", 30),
+		"templates/plain.template":    strings.Repeat("p", 50),
+		"templates/wide.template":     strings.Repeat("w", 50),
+		"templates/wide.w30.template": strings.Repeat("x", 35),
+		"templates/sw.template":       `{{ screenwidth }}`,
+	}
+	// Template paths are built with the OS separator (util.FilePath).
+	fsys := fstest.MapFS{}
+	for name, data := range files {
+		fsys[util.FilePath(name)] = &fstest.MapFile{Data: []byte(data)}
+	}
+	return fsys
+}
+
+func TestFormsPickTheWidestThatFits(t *testing.T) {
+	SetFSForTest(t, formsFS())
+	for _, tc := range []struct {
+		width int
+		want  string
+	}{
+		{80, strings.Repeat("b", 50)}, // a base that fits is always used
+		{60, strings.Repeat("b", 50)}, // the base fits
+		{45, strings.Repeat("f", 40)}, // a form drawn to 42 can fit 45
+		{42, strings.Repeat("f", 40)},
+		{35, strings.Repeat("n", 30)},
+		{30, strings.Repeat("n", 30)},
+	} {
+		got, err := process("t", nil, templateConfig{}, tc.width)
+		require.NoError(t, err)
+		assert.Equal(t, tc.want, got, "width %d", tc.width)
+	}
+}
+
+func TestFormsFallBackToTheNarrowest(t *testing.T) {
+	SetFSForTest(t, formsFS())
+	got, err := process("wide", nil, templateConfig{}, 30)
+	require.NoError(t, err)
+	assert.Equal(t, strings.Repeat("x", 35), got, "no form fits, so the narrowest one found")
+
+	got, err = process("plain", nil, templateConfig{}, 30)
+	require.NoError(t, err)
+	assert.Equal(t, strings.Repeat("p", 50), got, "a template with no forms is unchanged")
+}
+
+func TestScreenReaderFormComesFirst(t *testing.T) {
+	fsys := formsFS()
+	fsys[util.FilePath("templates/t.screenreader.template")] = &fstest.MapFile{Data: []byte("spoken")}
+	SetFSForTest(t, fsys)
+	got, err := process("t", nil, templateConfig{ScreenReader: true}, 30)
+	require.NoError(t, err)
+	assert.Equal(t, "spoken", got)
+}
+
+func TestScreenwidthIsTheReaderWidth(t *testing.T) {
+	SetFSForTest(t, formsFS())
+	for _, w := range []int{30, 42, 80} {
+		got, err := process("sw", nil, templateConfig{}, w)
+		require.NoError(t, err)
+		assert.Equal(t, strconv.Itoa(w), got)
+	}
+	got, err := process("sw", nil, templateConfig{}, 120)
+	require.NoError(t, err)
+	assert.Equal(t, "80", got, "layouts are drawn for at most 80, so a wider telnet line sees them as before")
+
+	got, err = ProcessText(`{{ screenwidth }}`, nil)
+	require.NoError(t, err)
+	assert.Equal(t, "80", got, "text rendered with no reader is laid out full width")
+}
+
+func TestFormsAreFoundOnceAndRemembered(t *testing.T) {
+	SetFSForTest(t, formsFS())
+	assert.Equal(t, []int{42, 30}, formsOf("t"))
+	assert.Equal(t, []int{}, formsOf("plain"))
+	if have, ok := formCache.Load("t"); !ok || len(have.([]int)) != 2 {
+		t.Fatalf("formsOf did not remember t's forms: %v %v", have, ok)
+	}
+}
+
+func TestWidestLineIgnoresColourTags(t *testing.T) {
+	assert.Equal(t, 5, WidestLine("<ansi fg=\"red\">abc</ansi>\r\nabcde\nab"))
+	assert.Equal(t, 0, WidestLine(""))
+}
+
+func TestFormWidthsAreWidestFirst(t *testing.T) {
+	for i := 1; i < len(FormWidths); i++ {
+		assert.Greater(t, FormWidths[i-1], FormWidths[i])
+	}
+	assert.Less(t, FormWidths[0], FullWidth)
+}
diff --git a/internal/templates/templates.go b/internal/templates/templates.go
index 2e3715ae4..6978ece7f 100644
--- a/internal/templates/templates.go
+++ b/internal/templates/templates.go
@@ -114,6 +114,7 @@ type templateDetails struct {
 	name       string
 	path       string
 	preProcess func(string) string
+	hasForms   bool // the .template file, which may have narrow forms
 }
 
 func ClearTemplateConfigCache(userId int) {
@@ -157,129 +158,61 @@ func processMarkdown(in string) string {
 	return "\n" + divider + "\n" + p.Parse().String(0) + "\n"
 }
 
+// Process renders a template for a reader. receivingUserId picks the
+// screen-reader form and the width the reader's screen has room for.
 func Process(fname string, data any, receivingUserId ...int) (string, error) {
-	ansiLock.RLock()
-	defer ansiLock.RUnlock()
-
-	var parseAnsiTags bool = false
-
-	var ansitagsParseBehavior []ansitags.ParseBehavior = make([]ansitags.ParseBehavior, 0, 2)
-
-	if forceAnsiFlags != AnsiTagsDefault {
-		//	ansiFlags = append(ansiFlags, forceAnsiFlags)
-	}
-
 	userId := 0
 	if len(receivingUserId) > 0 {
 		userId = receivingUserId[0]
 	}
+	return process(fname, data, getTemplateConfig(userId), readerWidth(userId))
+}
 
-	tplConfig := getTemplateConfig(userId)
-
-	var buf bytes.Buffer
+// process renders fname at width. A screen-reader form is tried first, then
+// a markdown file, then the .template file. Only the .template file has
+// narrow forms: when its output is wider than the
+// reader, the widest form that fits is used (see forms.go).
+func process(fname string, data any, tplConfig templateConfig, width int) (string, error) {
+	ansiLock.RLock()
+	defer ansiLock.RUnlock()
 
-	// Contains each template to attempt to load, in order.
-	// This allows us to attempt optional adjusted template paths first.
+	// Each template to attempt to load, in order.
 	filesToAttempt := []templateDetails{}
 
-	// Try a screen-reader friendly template first?
 	if tplConfig.ScreenReader {
-		filesToAttempt = append(filesToAttempt,
-			templateDetails{
-				name: fname,
-				path: util.FilePath(`templates/`, fname+`.screenreader.template`), // All templates must end with .template
-			},
-		)
-	}
-
-	filesToAttempt = append(filesToAttempt,
-		templateDetails{
-			name:       fname,
-			path:       util.FilePath(`templates/`, fname+`.md`), // All templates must end with .template
-			preProcess: processMarkdown,
-		},
-	)
-
-	filesToAttempt = append(filesToAttempt,
-		templateDetails{
+		filesToAttempt = append(filesToAttempt, templateDetails{
 			name: fname,
-			path: util.FilePath(`templates/`, fname+`.template`), // All templates must end with .template
-		},
-	)
-
-	for _, tplInfo := range filesToAttempt {
-
-		if fileBytes, err := readFile(tplInfo.path); err == nil {
-
-			tpl, err := template.New(tplInfo.name).Funcs(funcMap).Parse(string(fileBytes))
-			if err != nil {
-				mudlog.Error("template parse error", "module", true, "filepath", tplInfo.path, "error", err)
-				return string(fileBytes), err
-			}
-
-			err = tpl.Execute(&buf, data)
-			if err != nil {
-				mudlog.Error("could not parse template file", "module", true, "filepath", tplInfo.path, "error", err)
-				return "[TEMPLATE ERROR]", err
-			}
-
-			// return the final data as a string, parse ansi tags if needed (No need to parse if it was preparsed)
-			if parseAnsiTags {
-				return ansitags.Parse(buf.String(), ansitagsParseBehavior...), nil
-			}
-
-			if tplInfo.preProcess != nil {
-				return tplInfo.preProcess(buf.String()), nil
-			}
-			return buf.String(), nil
-		}
+			path: util.FilePath(`templates/`, fname+`.screenreader.template`),
+		})
+	}
 
-		//
-		// nothing able to load from the plugin files, lets try the normal filesystem.
-		//
+	filesToAttempt = append(filesToAttempt, templateDetails{
+		name:       fname,
+		path:       util.FilePath(`templates/`, fname+`.md`),
+		preProcess: processMarkdown,
+	})
 
-		fullPath := util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/`, tplInfo.path)
+	filesToAttempt = append(filesToAttempt, templateDetails{
+		name:     fname,
+		path:     util.FilePath(`templates/`, fname+`.template`),
+		hasForms: true,
+	})
 
-		// Get the file contents
-		fileContents, err := os.ReadFile(fullPath)
-		if err != nil {
+	for _, tplInfo := range filesToAttempt {
+		out, found, err := renderFile(tplInfo, data, tplConfig, width)
+		if !found {
 			continue
 		}
-
-		// parse the file contents as a template
-		tpl, err := template.New(tplInfo.name).Funcs(funcMap).Parse(string(fileContents))
-		if err != nil {
-			mudlog.Error("template parse error", "module", false, "filepath", fullPath, "error", err)
-			return string(fileContents), err
+		if err != nil || !tplInfo.hasForms {
+			return out, err
 		}
-
-		// execute the template and store the results into a buffer
-
-		err = tpl.Execute(&buf, data)
-		if err != nil {
-			mudlog.Error("could not parse template file", "module", false, "filepath", fullPath, "error", err)
-			return "[TEMPLATE ERROR]", err
+		// Only a template that has forms is measured, so a broadcast of a
+		// template without them costs nothing extra.
+		forms := formsOf(fname)
+		if len(forms) == 0 || WidestLine(out) <= width {
+			return out, nil
 		}
-
-		if tplConfig.ScreenReader {
-			strippedOut := util.StripCharsForScreenReaders(buf.String())
-
-			// return the final data as a string, parse ansi tags if needed (No need to parse if it was preparsed)
-			if parseAnsiTags {
-				return ansitags.Parse(strippedOut, ansitagsParseBehavior...), nil
-			}
-			return strippedOut, nil
-		}
-
-		// return the final data as a string, parse ansi tags if needed (No need to parse if it was preparsed)
-		if parseAnsiTags {
-			return ansitags.Parse(buf.String(), ansitagsParseBehavior...), nil
-		}
-
-		if tplInfo.preProcess != nil {
-			return tplInfo.preProcess(buf.String()), nil
-		}
-		return buf.String(), nil
+		return narrowestFit(fname, data, tplConfig, width, out, forms), nil
 	}
 
 	//
@@ -293,6 +226,58 @@ func Process(fname string, data any, receivingUserId ...int) (string, error) {
 	return fmt.Sprintf(`[TEMPLATE READ ERROR: FNF (%s) `, strings.Join(allFiles, `, `)), fmt.Errorf(`Files not found: %s`, strings.Join(allFiles, `, `))
 }
 
+// renderFile reads and executes one template file. found is false when the
+// file exists nowhere. A registered filesystem (a module) is read first, then
+// the data files on disk; a screen-reader reader gets the on-disk output
+// stripped of decorative characters, as before.
+func renderFile(tplInfo templateDetails, data any, tplConfig templateConfig, width int) (out string, found bool, err error) {
+	if fileBytes, rerr := readFile(tplInfo.path); rerr == nil {
+		tpl, perr := template.New(tplInfo.name).Funcs(funcMap).Parse(string(fileBytes))
+		if perr != nil {
+			mudlog.Error("template parse error", "module", true, "filepath", tplInfo.path, "error", perr)
+			return string(fileBytes), true, perr
+		}
+		var buf bytes.Buffer
+		if xerr := tpl.Funcs(widthFuncs(width)).Execute(&buf, data); xerr != nil {
+			mudlog.Error("could not parse template file", "module", true, "filepath", tplInfo.path, "error", xerr)
+			return "[TEMPLATE ERROR]", true, xerr
+		}
+		if tplInfo.preProcess != nil {
+			return tplInfo.preProcess(buf.String()), true, nil
+		}
+		return buf.String(), true, nil
+	}
+
+	//
+	// nothing able to load from the plugin files, lets try the normal filesystem.
+	//
+	fullPath := util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/`, tplInfo.path)
+	fileContents, rerr := os.ReadFile(fullPath)
+	if rerr != nil {
+		return "", false, nil
+	}
+
+	tpl, perr := template.New(tplInfo.name).Funcs(funcMap).Parse(string(fileContents))
+	if perr != nil {
+		mudlog.Error("template parse error", "module", false, "filepath", fullPath, "error", perr)
+		return string(fileContents), true, perr
+	}
+
+	var buf bytes.Buffer
+	if xerr := tpl.Funcs(widthFuncs(width)).Execute(&buf, data); xerr != nil {
+		mudlog.Error("could not parse template file", "module", false, "filepath", fullPath, "error", xerr)
+		return "[TEMPLATE ERROR]", true, xerr
+	}
+
+	if tplConfig.ScreenReader {
+		return util.StripCharsForScreenReaders(buf.String()), true, nil
+	}
+	if tplInfo.preProcess != nil {
+		return tplInfo.preProcess(buf.String()), true, nil
+	}
+	return buf.String(), true, nil
+}
+
 func ProcessText(text string, data any, ansiFlags ...AnsiFlag) (string, error) {
 
 	var parseAnsiTags bool = false
@@ -379,6 +364,24 @@ func (t TemplateTable) GetCell(row int, column int) string {
 	return cellStr
 }
 
+// RowDetail is a row without its first cell, as "Header: value" pairs joined
+// by " · ", for the two-line table form (tables/*.w30.template). A blank cell
+// is left out, and a cell under a blank header shows its value alone.
+func (t TemplateTable) RowDetail(row int) string {
+	parts := []string{}
+	for c := 1; c < len(t.Rows[row]); c++ {
+		value := strings.TrimSpace(t.Rows[row][c])
+		if value == `` {
+			continue
+		}
+		if c < len(t.Header) && strings.TrimSpace(t.Header[c]) != `` {
+			value = t.Header[c] + `: ` + value
+		}
+		parts = append(parts, value)
+	}
+	return strings.Join(parts, ` · `)
+}
+
 func GetTable(title string, headers []string, rows [][]string, formatting ...[]string) TemplateTable {
 
 	var table TemplateTable = TemplateTable{
diff --git a/internal/templates/templatesfunctions.go b/internal/templates/templatesfunctions.go
index 38c5083f5..1b773cea9 100644
--- a/internal/templates/templatesfunctions.go
+++ b/internal/templates/templatesfunctions.go
@@ -129,7 +129,11 @@ var (
 		"divFloat":     func(a int, b int) float64 { return float64(a) / float64(b) },
 		"stringor":     stringOr,
 		"splitstring":  util.SplitStringNL,
-		"ansiparse":    TplAnsiParse,
+		// screenwidth is the reader's width; Process replaces it per render
+		// (forms.go). Rendered with no reader, it is FullWidth.
+		"screenwidth": func() int { return FullWidth },
+		"min":         func(a, b int) int { return min(a, b) },
+		"ansiparse":   TplAnsiParse,
 		"conditionname": func(conditionId int) string {
 			conditionSpec := conditions.GetConditionSpec(conditionId)
 			if conditionSpec == nil {
diff --git a/internal/templates/testing_support.go b/internal/templates/testing_support.go
index f48e24e3d..e62b622f4 100644
--- a/internal/templates/testing_support.go
+++ b/internal/templates/testing_support.go
@@ -26,12 +26,22 @@ import (
 // Use this in tests instead of RegisterFS, which appends and leaks. This is
 // not safe with parallel tests (t.Parallel()): fileSystems is an unlocked
 // package-level var, so two tests racing SetFSForTest step on each other's
-// registrations and cleanups.
+// registrations and cleanups. It also forgets which narrow forms exist, on
+// the way in and out, since they were found on the old filesystems.
 func SetFSForTest(t *testing.T, filesystems ...fs.ReadFileFS) {
 	t.Helper()
 	original := fileSystems
 	fileSystems = filesystems
+	formCache.Clear()
 	t.Cleanup(func() {
 		fileSystems = original
+		formCache.Clear()
 	})
 }
+
+// ProcessAtWidth renders fname as Process does for a reader whose screen is
+// width columns wide, without a screen reader. For guards that hold every
+// template to the width it is shown at (internal/guards).
+func ProcessAtWidth(fname string, data any, width int) (string, error) {
+	return process(fname, data, templateConfig{}, width)
+}
diff --git a/internal/usercommands/motd.go b/internal/usercommands/motd.go
index e446aff69..0e3f2b4ac 100644
--- a/internal/usercommands/motd.go
+++ b/internal/usercommands/motd.go
@@ -36,8 +36,18 @@ func Motd(rest string, user *users.UserRecord, room *rooms.Room, flags events.Ev
 	return true, nil
 }
 
+// motdBoxMinWidth is the narrowest box the spaced-out title fits in: the
+// title and its leading space take 41 of the lw-4 content columns.
+const motdBoxMinWidth = 45
+
 // motdBox draws the MOTD text in a double-line box exactly lw columns wide.
+// A screen narrower than the title, or text holding a word wider than the
+// box (a long link), gets the narrow form instead: a plain title and the text
+// wrapped to the screen, because a box cannot hold either (spec section 2).
 func motdBox(text string, lw int) string {
+	if lw < motdBoxMinWidth || motdLongestWord(text) > lw-5 {
+		return motdPlain(text, lw)
+	}
 	// Box outer width scales with the player's configured line width.
 	// Layout: " " + "║" + " " + <content padded to padWidth> + "║"
 	//          1     1     1          ...                       1   = lw
@@ -87,3 +97,19 @@ func motdBox(text string, lw int) string {
 
 	return output
 }
+
+// motdPlain is the MOTD's narrow form: the title on its own line and the
+// text wrapped to lw. Only a single word wider than lw can run past it.
+func motdPlain(text string, lw int) string {
+	return `<ansi fg="cyan-bold">Message of the Day</ansi>` + "\n" +
+		`<ansi fg="yellow">` + messaging.WrapAnsi(text, lw) + `</ansi>` + "\n"
+}
+
+// motdLongestWord is the display width of the longest word in text.
+func motdLongestWord(text string) int {
+	longest := 0
+	for _, word := range strings.Fields(motdAnsiTagRE.ReplaceAllString(text, "")) {
+		longest = max(longest, motdVisibleWidth(word))
+	}
+	return longest
+}
diff --git a/internal/usercommands/motd_test.go b/internal/usercommands/motd_test.go
index 3bad0cc1e..11e5ff407 100644
--- a/internal/usercommands/motd_test.go
+++ b/internal/usercommands/motd_test.go
@@ -29,3 +29,28 @@ func TestMotdBoxIsExactlyTheLineWidth(t *testing.T) {
 		}
 	}
 }
+
+// The narrow form: below the width the title fits, or with a word wider than
+// the box, the MOTD is a plain title and wrapped text, with no box to burst.
+func TestMotdNarrowFormFitsTheScreen(t *testing.T) {
+	text := "Welcome to Delusions of Grandeur. Read the rules, mind the " +
+		"newbie zone hazards, and ask a mod if anything looks broken."
+	for _, lw := range []int{30, 42, 44} {
+		out := motdBox(text, lw)
+		if strings.Contains(out, "╔") {
+			t.Errorf("lw %d: still boxed:\n%s", lw, out)
+		}
+		for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
+			if w := motdVisibleWidth(motdAnsiTagRE.ReplaceAllString(line, "")); w > lw {
+				t.Errorf("lw %d line %d is %d columns: %q", lw, i, w, line)
+			}
+		}
+	}
+	if out := motdBox(text, 45); !strings.Contains(out, "╔") {
+		t.Errorf("lw 45 holds the title, so it keeps the box:\n%s", out)
+	}
+	link := "Join us: https://example.org/a/very/long/path/that/does/not/fit/inside/the/box/at/all/x"
+	if out := motdBox(link, 80); strings.Contains(out, "╔") {
+		t.Errorf("a word wider than the box gets the plain form:\n%s", out)
+	}
+}
````
