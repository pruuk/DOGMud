# Web Client Base, Plan 3b-3: Notices, Tables and Lists Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every remaining hand-laid screen its narrow form, so nothing a player sees runs past a 30, 42 or 80 column screen: mail, the quest, skill, stat and combat banners, auctions, both craft screens, the guild roster and list, the MOTD, tables over 80 (with a lead column each), `online` without truncation, achievements, scan, macros, channels, `skills extra` and container recipes; plus three copy fixes and a 28-letter name limit.

**Architecture:** Templates get `.w42`/`.w30` forms as in plans 3a to 3b-2. Screens whose base already fits (the star banners) change their base. Go-built screens lay themselves out in three bands, the 80 layout at 80 and over, the 42 form from 42 to 79, the 30 form below, packing coloured units with one shared packer (`messaging.PackUnits`). Broadcasts (auctions) and per-round paths (combat banners) never measure: they pick a form by width (`templates.ProcessForWidth`, `banner.Format(..., width)`), and an auction broadcast renders once per distinct (screen reader, width). Tables name the column that leads their two-line form (`TemplateTable.WithLead`).

**Tech Stack:** Go 1.25 `text/template`, `testify`, guards in `internal/guards`.

**Spec:** `docs/superpowers/specs/2026-11-09-webclient-base-layout-design.md`, section 2 and rulings 3, 22 to 25. Mocks: 3b (approved 2026-11-10) https://claude.ai/artifact/4vUBgswyRd3umYPrBmyHvZ, source `_archive/notes/reports/narrow-screens-3b-mock/`; 3b-3 (ruled 2026-11-11) https://claude.ai/artifact/Ap7hpZK45uwfWexJUwBjcA, source `_archive/notes/reports/narrow-screens-3b3-mock/`. Part of #783.

**How this plan differs from 3b-2.** The work is already built, test first, on branch `feature/webclient-base-3b3` in worktree `C:/tmp/dogmud-3b3` (20 commits on master `6b7902731`, below). It is too large to embed as a patch (141 files, 5,519 lines added). Executing this plan means: an independent review of each stage against the rulings, fixing what review finds (test first), the full gates, your manual test, then the PR. Nothing is pushed until you have tested.

---

## Owner rulings this plan follows

From 2026-11-10: horizontal rules only, full width, no text past a rule; long prose after a label continues 2 in; a screen already over 80 takes a narrower form on a desktop too; every form shows all information. From 2026-11-11:

1. A table's two-line form leads with a column the table names (history: Log; auction history: Item; who and online: Name).
2. `online` stops truncating names and zones with "...".
3. The eight Go screens the 3b mock never drew got their own mock first.
4. A name that fits the line but not its indent splits at a space and keeps the indent.
5. The character name limit drops from 32 to 28.
6. Craft rows keep the drawn indents: under the recipe name at 42 and 80, 2 in at 30.
7. Container recipes get their narrow form, though no DOGMud room uses one yet.

## Facts verified against source

Read 2026-11-11 from master `6b7902731` by grep and by the four stage agents (each re-read what it relied on; corrections are folded in).

| # | Fact | Where |
|---|---|---|
| 1 | Forms exist only at 42, 40 and 30; a file named for another width is never read, so the drawn 80 redesigns go into the base layout | `internal/templates/forms.go:29` |
| 2 | Forms are tried only when the base is wider than the reader, widest first | `forms.go:102-115`; `templates.go:209-215` |
| 3 | The star banners' rules were already `screenwidth - 1`, so their forms would never be tried; their base changes instead | `templates/character/{questup,skillup,statup}.template` |
| 4 | `auction-bid` was rendered once with the bidder's id and sent to everyone; start, update and end rendered once per online player, before the opt-out check | `modules/auctions/auctions.go:356-364`, :511-520, :666-675, :713-722 |
| 5 | Combat banners: a fixed 64-glyph rule, built in `characters` (which cannot import `users`), delivered by a hooks callback that holds the user | `internal/banner/banner.go:39,44-82`; `internal/hooks/progression_notify.go:16-19` |
| 6 | Mail is 99 columns at 80 today (NOTE lines); label padding lives in the locale strings | `inbox.go:45`; `localize/en.yaml:14-21` |
| 7 | The bare `craft` view is 203 columns at 80, `scan` 123, `macros` 109, `guild list` 103 (mock 3b-3 captures, widest data) | `_archive/.../narrow-screens-3b3-mock/final/` |
| 8 | `motdBox` drew corners at 45 and over; below that `motdPlain` had no rules | `internal/usercommands/motd.go:41,47-49,103-106` |
| 9 | All ten tables go through `GetTable`; five run past 80 with shipped data (history 97, auction history 94, keyring 104, spells 98, who 93 with long names, party 101) | survey arithmetic; `history.go:42`, `auctions.go:339`, `keyring.go:164`, `spells.go:137`, `online.go:203`, `party.go:458` |
| 10 | The 30-column table form used `splitstring`, which glued an unlearned lock sequence (`? ? ?`) into one 59-column word | `templates/tables/generic.w30.template` |
| 11 | `fitOnlineEntries` trimmed zone then name with "..." to stay inside 80 | `online.go:92`, called :184 |
| 12 | `NameSizeMax: 32`; existing-user login checks only the minimum, so a 29 to 32 letter username still logs in after the limit drops; `start` revalidated the username and would have renamed such a character "nameless-N" | `_datafiles/config.yaml:2714`; `inputhandlers/login_prompt_handler.go:85-97`; `usercommands/start.go:58-63` |
| 13 | The text normaliser appended a full stop after a line ending in `:`, printing `inside:.` | `internal/messaging/normalize.go:243-252` |
| 14 | No shipped DOGMud room has a container recipe; the only one is in GoMud's default world | `_datafiles/world/default/rooms/frostfang/611.yaml` |

## Decisions taken without asking

Each was made while building; what changes if it is wrong follows each.

**Worth your eye first:**

1. **Spaceless names pulled left.** A unit with no space that fits the line but not its indent (a 28-letter name under a 4-column indent at 30, in `guild list` and `macros`) starts further left so it stays whole. Ruling 4 covers names with spaces only. If wrong: that line's indent shrinks to fit the name instead (same look, different rule), or the name breaks at the edge.
2. **The colon fix is global.** The normaliser no longer adds a full stop after any line ending in `:`, not only the container intro. Nothing pinned the old behaviour; the whole suite passes. If wrong: revert it and reword the one sentence.
3. **MOTD from 43 to 79 columns uses the 42 rules-only form,** not today's cornered box (no corners in narrow forms). 80 is unchanged. If wrong: rules as wide as the screen between 43 and 79.
4. **Tables nobody touched change at 30 only:** the detail line now breaks between whole `Header: value` pairs (`Where: Main Street ·` / `Since: today`, where master split `Since:` from its value). This also fixes fact 10. If wrong: split mid-pair again and fix the lock overrun separately.
5. **One stat banner changes at 80:** the 79-letter perception message ran one column past its 79-star rule; it now wraps inside it. Every other banner at 80 is byte for byte.
6. **The container recipe line drops the "1":** `To receive a Fair Copy of the Compiled Survey:` (lower-casing alone gave "To receive 1 a Fair Copy"). Recipe and input order is now fixed (it was random on every look).

**The rest:**

7. Go screens use bands (80 at 80 and over, 42 from 42 to 79, 30 below) and never measure; the guild roster keeps master's line wherever it fits.
8. Mail's base wraps to a fixed 80 (with `screenwidth` the base would fit at 42 and its form would never be used). `Sent:` at 30 hangs under its value, as drawn; a sender too long goes under the label, 2 in. Labels drop padding at 30 through a new `trimspace` function, so no locale string changes.
9. `wrapspans` treats a span nested in another span as one unit (an item name inside a coloured NOTE).
10. The first line of prose after a label is 2 columns narrower than its continuation, the same `wraptext` limit 3b-2 shipped.
11. Auction name lines at 30 continue at the label's column so a 28-letter name fits the 29-wide word rule; Item and Description continue at column 3. The unsold `Winner: none (It will be returned to the owner)` goes under its label at 42.
12. Combat banners: `characters` hands the notifier a `render(width)` function; the hook calls it with `user.WrapWidth()`. Rule = min(width, 64).
13. Craft: ingredients split at a space keeping their indent when one does not fit (`1x Chrysalis` / `Filter-Membrane,` at 30); the reason ("need alchemy bench") is one unit; "Overall recipe knowledge:" moves its value to the next line, 2 in, as drawn.
14. Guild member line drops its 20-column name padding when it does not fit.
15. Lead columns beyond your three: spells Name, keyring Where (the exit or container the lock is on; Location is the room), shop list the item when the first column is Qty, party already led with Name. Killstats, alias, leaderboards and the alt list keep their first column. A wrapped lead continues at column 0; the detail is indented 2. Leads keep their header's colour alias, so Log, Name and Where print uncoloured, as Type did.
16. `skills extra` uses the cooldown display names ("special moves") and sorts them, and gives the time in words with the `cooldowns` helper (`templates.DurationQuality`, now exported).
17. A pre-3b-3 username of 29 to 32 letters is kept as the character name in `start` instead of failing validation (fact 12). The two guards that render 32-letter names keep them, commented as legacy names. A new test holds every playtest profile's generated username to 28 (the longest is 27).
18. Shared helpers moved: `packUnits` to `messaging.PackUnits`; the band helpers to `users.NarrowFormWidth` and `users.RedrawnFormWidth`, so `internal/actions` (scan) can use them.
19. The default world's templates are left as they are (your 2026-09-17 ruling).
20. The patch note gives no raw numbers ("a slightly shorter limit") and describes the colon fix generally, since no shipped room has a container recipe.

## What changes at 80 columns

- **Redesigned to fit** (they ran past 80): both craft screens (reason, station and speed on a second line; long ingredient lists wrap), `scan`, `macros`, `guild list`, mail NOTE lines.
- **Tables over 80** take the two-line form led by their named column. History, at 97 bordered today:

```
.: History
Purchased a lantern from Mistress Odell the Victualler for 40 gold
  Type: shop · Time: Nov 11 '26 3:04PM
```

- **`online` and `who`** show every name and zone whole; a list too wide takes the two-line form.
- Decision 5's stat banner and decision 2's colon.
- Every other 80-column screen is byte for byte master, pinned by goldens recorded from master before each change.

## The built branch

| # | Commit | What |
|---|---|---|
| 1 | `c002ff58b` | Mail forms at 42 and 30; base matches the drawn 80; `DateUnits`; `trimspace`; nested spans in `wrapspans`; `internal/guards/notice_screens_test.go` with 19 goldens from master |
| 2 | `2961b9cb1` | Quest, skill and stat banners fit every width (base change) |
| 3 | `735cc9e4e` | Auction forms; `templates.ProcessForWidth`; broadcast renders once per (screen reader, width), opt-out first, bid fixed |
| 4 | `b92f87f34` | Broadcast test reads the forms through the plugin registry (proves the module ships them) |
| 5 | `463adde29` | Combat banners drawn to the reader's width |
| 6 | `62f7b9a67` | Craft list and bare craft; `craft_rows.go`; `narrow_layout.go`; guild and MOTD goldens from master |
| 7 | `f04d643ae` | Guild roster and guild list |
| 8 | `946e1b3f1` | MOTD rules-only narrow form (`motdRuled` replaces `motdPlain`) |
| 9 | `635c0b581` | `TemplateTable.WithLead`, leads set, `messaging.PackUnits`, pair-whole detail lines |
| 10 | `0a2cc78c4` | `online` without `fitOnlineEntries` |
| 11 | `6d26e3343` | Goldens from master: achievements, macros, channels, container recipe |
| 12 | `5ede1fbd4` | Achievements |
| 13 | `649d88325` | Scan, 80 redrawn; band helpers to `users` |
| 14 | `17eac8cc7` | Macros, 80 redrawn |
| 15 | `fd407121e` | Channels heading at 30 |
| 16 | `6734cc287` | `skills extra` times in words |
| 17 | `dd55a56b0` | Container recipes; `inside:` and `To receive a` copy fixes |
| 18 | `97517b9d1` | `NameSizeMax` 28; `start` keeps legacy names; playtest username test |
| 19 | `46b88cf77` | The empty "plan 3b-3" guard class removed |
| 20 | `0178c209c` | Patch note, `docs/README.md` rows, `context.md` updates |

Every commit was written test first: its stage report records the red line seen before the code, the green after, and for every fit test a planted one-column-too-wide edit that made it fail and was undone. Full gates on the whole branch, run by the controller on 2026-11-11 at `0178c209c`: `gofmt -l` empty, `go build ./...` passes, `go test . ./internal/... ./modules/... -count=1` prints no failing line, every `tools/jstest` and `tools/webclient-tests` file passes, `golangci-lint --new-from-merge-base=origin/master` reports `0 issues.` No independent review has run yet; Tasks 1 to 4 are that review.

## Review round (Tasks 1 to 4, run 2026-11-11)

Four Opus reviewers, one per stage, each in its own detached worktree, found one medium and eight low defects plus an overclaiming patch-note heading. All ten were fixed test first in eight commits (`f1003e7fd` to `25c9747fd`):

1. A table detail line could start with a stray `·` (auction history under a 28-letter name; keyring under nine shipped room titles). The dot is now glued to its pair's last word.
2. A legacy name in a table detail was pulled to column 0, where it read as a new row. Tables never pull left of the detail indent (`messaging.PackUnitsKeepIndent`).
3. Spaceless text wider than the whole line (hyphenated guild names, long words in macros and auction descriptions) ran past the edge. It now breaks at the edge; a character name keeps the accepted exception (callers name them to `PackUnits`).
4. The guild roster at 40 and 41 columns took its 30 form where a template would take the 42 one; it now measures (`measuredFormLines`). Comments claiming the Go bands match `templates.Process` were corrected.
5. `PackUnits` could leave a trailing blank line.
6. An unsold auction's Winner prose at 30 now continues 2 past its label.
7. `start`'s legacy-name fallback set a name on any validation failure (banned, duplicate). It now skips only the length check (`ValidateActorOpts.SkipMaxLength`) with the user's own id excluded.
8. `ArticlePrefix` read the quest star's colour tag as the first word ("a ★An Old Key"); it, `MidSentenceName` and `NameWithoutArticle` now skip tags and the star, for every caller.
9. A no-lead table's plain-column form is now pinned by a golden recorded from master.
10. The patch note heading reads "More screens fit your window" (help waits for plan 4).

Decisions taken in that round without asking:

21. Tables keep every spaceless detail piece whole (they cannot tell a name from other text; today the only such pieces are names).
22. The table dot stays glued even when that carries the pair's last word to the next line (`Coachman's Rest, Common` / `Room · Bid: 40 gold`).
23. Companion names are not marked as names (at most 20 letters, with spaces).
24. `start` skips the length check outright; signup never allowed more than 32.
25. Three "You receive" lines (quest `GiveItem`, behaviour-tree give, reward summary) now get the right article but keep a name's own capital ("You receive A Damp Page"), which a test pins. A one-line change each if you want them lower-cased.
26. Other banded Go screens (companion, craft, MOTD, scan and the rest) keep their band at 40 and 41 (the 30 form); only the guild roster measures.

---

### Task 0: Bring the branch up to date

- [ ] **Step 1:**

```bash
cd C:/tmp/dogmud-3b3
git fetch origin
git log --oneline HEAD..origin/master
```

If master moved, `git merge origin/master` (no rebase: the commits above are cited by SHA), resolve, and re-run Task 5 Step 1 before going on. `C:/tmp/dogmud-3b3` is named here so the end-of-day sweep keeps it; remove it with `git worktree remove` once the PR merges.

---

### Task 1: Review stage 1 (commits 1 to 4: mail, star banners, auctions)

- [ ] **Step 1:** Dispatch an Opus reviewer, read-only, on `git diff 6b7902731 b92f87f34`, with the rulings above, the drawings (`mail_read`, `mail_compose_preview`, `banner_questup`, `banner_skillup`, `banner_statup`, `auction_update`, `auction_end` in the 3b mock's `forms/`) and decisions 5, 8 to 11. It reports correctness bugs with a concrete input and output each, and any line that breaks a ruling. It must not edit files.
- [ ] **Step 2:** For each finding it can demonstrate: a failing test first, then the fix, then the test green, committed with named paths. Findings it cannot demonstrate are dropped.

### Task 2: Review stage 2 (commits 5 to 8: combat banners, craft, guild, MOTD)

- [ ] **Step 1:** As Task 1 Step 1, on `git diff b92f87f34 946e1b3f1`, drawings `banner_combat_*`, `craft_list`, `guild_roster`, `login_motd_and_who` (3b mock) and `craft_ready`, `guild_list`, `guild_roster` (3b-3 mock), decisions 1, 3, 7, 12 to 14. Also check: no measuring on the combat banner path; the band helpers agree with `templates` form picking.
- [ ] **Step 2:** As Task 1 Step 2.

### Task 3: Review stage 3 (commits 9 and 10: tables, online)

- [ ] **Step 1:** As Task 1 Step 1, on `git diff 946e1b3f1 0a2cc78c4`, decisions 4 and 15, and ruling 1 and 2. Also check every `GetTable` caller without a lead renders as master at 80 and 40.
- [ ] **Step 2:** As Task 1 Step 2.

### Task 4: Review stage 4 (commits 11 to 20: the 3b-3 mock screens, copy, names, docs)

- [ ] **Step 1:** As Task 1 Step 1, on `git diff 0a2cc78c4 0178c209c`, the 3b-3 mock drawings and notes, decisions 1, 2, 6, 16 to 20, and ruling 4 (the mock drew some names pulled left; the code must split them at a space). Also check the patch note against `dogmud-player-copy` (no raw numbers, no dashes, ESL-clear) and every new file against `docs/README.md`.
- [ ] **Step 2:** As Task 1 Step 2.

---

### Task 5: Verify and ship

- [ ] **Step 1: Full gates.**

```bash
cd C:/tmp/dogmud-3b3
gofmt -l internal/ modules/
go build ./...
go test . ./internal/... ./modules/... -count=1 2>&1 | grep -vE '^ok|no test files' | tail -20
for t in tools/jstest/*.test.js tools/webclient-tests/*.js; do node "$t" > /dev/null 2>&1 || echo "RED $t"; done; echo done
golangci-lint run --new-from-merge-base=origin/master
```

Expected: no gofmt output, the build passes, no test lines, `done` with no `RED`, `0 issues.`

- [ ] **Step 2: Commit the plan** into the branch: `cp` it from the main checkout to `docs/superpowers/plans/`, add its row to `docs/README.md` beside the 3b-2 plan row, commit both with named paths.

- [ ] **Step 3: Boot check** on private ports (the `dogmud-shipping` skill), building to `C:/tmp/dogmud-3b3/boot-check.exe`, with the room and mob instance saves wiped in the worktree and the owner's dev accounts copied in (`_datafiles/world/dogmud/users/`, gitignored), so existing characters log in.

- [ ] **Step 4: STOP for the owner's manual test.** Leave the server running on its private ports and hand the owner this punchlist, telnet at `set linewidth 40`, `42` and `80`, then the web at 80:
  1. `mail` yourself a message with gold attached, then read it (`inbox`).
  2. Finish a quest step or gain a skill or stat (or `spawn` a quest giver); watch the banner.
  3. Fight until a combat stat or skill banner shows.
  4. `auction` an item from one character, bid from another; both watch at different widths.
  5. `craft` and `craft list` in the alchemy room (471).
  6. `guild list` and `guild info` (create a guild with a long name if none).
  7. Log out and in: the MOTD and the online list.
  8. `history`, `who`, `party` (invite a second character), `keyring`, `spells`, `list` at a shop, `auction history`.
  9. `achievements`, `scan`, `macros` (set one long `say`), `channels`, `skills extra` after a special move.
  10. Try to create a character with a 29-letter name (refused) and log in as an existing character.

  Nothing is pushed until the owner reports back. Fix what the owner finds (test first), re-run Step 1, and stop again.

- [ ] **Step 5: Push and open the PR** with `gh pr create --repo pruuk/DOGMud --base master --head feature/webclient-base-3b3 --body-file "$S/pr3b3-body.md"`: what players see at 80 and on narrow screens, the decisions above, what was tested, "Part of #783", no closing keyword, ending with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`. Deploy note: `NameSizeMax` is a shipped config value; check prod's `~/mud-config/config-production.yaml` does not override `Validation.NameSizeMax`. Merge only when the owner says.
