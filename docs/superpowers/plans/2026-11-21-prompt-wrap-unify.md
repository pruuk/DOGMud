# Prompt wrapping through the shared wrapper: plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development.
> Edit files with Edit/Write only (guard.py H3). Worktree `C:/tmp/dogmud-prompt-wrap`,
> branch `fix/prompt-wrap-unify`, off master `23dad3411`.

**Goal.** Every in-game prompt (the pager's stop line, the player's prompt with its
HP/SP/CP bars, a question's prompt) is word-wrapped by the game's one prompt wrapper,
`messaging.WrapPrompt`, at the width game text uses. On Malia's S21 (38 columns) the
stop line "More (page 1 of 3): press Enter for the next page, or type q to stop." and
the health prompt currently break mid-word or mid-bar, because nothing wraps a prompt.

**Cause.** The #842 pager (built by Claude, 2026-11-18) sent its stop line as one
unbroken prompt and wrote its own column counter, instead of wiring into
`messaging.WrapPrompt`, which login prompts already use.

## Facts verified against source (2026-11-21, master `23dad3411`)

| Fact | Source |
|---|---|
| `messaging.WrapPrompt(text, maxWidth)` wraps like `WrapAnsi` and keeps trailing spaces on the last line; returns a fitting text byte for byte | `internal/messaging/wrap.go:294` |
| Its one caller is login text for web clients | `internal/inputhandlers/login_prompt_handler.go:458` (`FitLoginText`) |
| The stop line is "More (page %d of %d): press Enter for the next page, or type q to stop. " | `internal/pager/pager.go:148` |
| The pager spec chose "the stop line is a prompt (no line break)" | `docs/superpowers/specs/completed/2026-11-18-general-pager-design.md:213` |
| `CommandPromptBody` returns question, else stop line, else the player's prompt, then unsent text, then a telnet go-ahead; it wraps nothing | `internal/users/userrecord.prompt.go:378-470` |
| Every prompt send goes through `users.SendCommandPrompt` -> `connections.SendPrompt(id, b, PromptCols(prompt))` | `userrecord.prompt.go:362-372`; callers `main.go:956,1090,1250,1272` (`sendPromptFromConnection`), `hooks/RedrawPrompt_SendRedraw.go:21` |
| The #922 prompt record keeps `cols` and `width`, and the erase clears `ceil(cols/width)` rows; `SendEcho` adds typed columns to `cols` | `internal/connections/prompt.go:30-34, 72-80, 141-157` |
| `PromptCols` measures one unbroken line with `connections.MinCols` (a newline counts 0 columns, so a multi-line prompt is mis-measured) | `userrecord.prompt.go:368-372`, `prompt.go:51` |
| Game text wraps at `UserRecord.WrapWidth()` (web: reported cols clamped 30..80 and `set linewidth`; telnet: linewidth capped by NAWS) | `internal/users/wrapwidth.go:141-157`, `hooks/Message_SendMessages.go:132` |
| The pager counts rows at `PagerWidth()` (reported width, else `WrapWidth`) with its own counter `pager.Rows` (own escape regexp + `runewidth`) | `userrecord.pager.go:74-79`, `pager.go:37-50` |
| The pager reserves `Rows(StopLine(..), width)` rows for the stop line | `pager.go:60, 85-92` |
| `menu.wrap` is a fourth word wrapper (rune count, blind to colour and wide characters), 4 call sites | `internal/menu/menu.go:84,103,140,166,215` |
| `messaging` does not import `users`, `pager` or `menu`, so all three can import it | `go list` of the three packages |

## Tasks

1. **Wrap every prompt in one place.** In `CommandPromptBody`, wrap `promptOut` (question,
   stop line or player prompt; before unsent text and go-ahead) with
   `messaging.WrapPrompt(promptOut, u.WrapWidth())`. Test at 38 columns: the stop line and
   the default fight prompt break only between words; a prompt that fits is byte for byte
   unchanged (today's 80-column output is identical).
2. **The prompt record counts wrapped lines.** `promptRecord` gains `above` (lines before the
   last); `cols` becomes the last line's columns. `PromptCols` returns both. The erase
   clears `above + rows(cols)` rows; `SendEcho` still extends `cols`. Tests: a two-line
   prompt is fully cleared by the next text; a one-line prompt clears exactly as today
   (existing #922 tests unchanged).
3. **The pager reserves the wrapped stop line's real height.** `pager.Rows(StopLine(..))` is
   replaced by the line count of `messaging.WrapPrompt(StopLine(..), width)`. `pager.Rows`
   for page lines keeps its job but loses its private regexp: it uses the one shared
   escape-sequence pattern (`connections.escapeSeqRe` moved to `term`, used by both).
4. **`menu.wrap` goes, and the heading wraps.** Its four callers use `messaging.WrapAnsi`
   (split on "\n"). `Render` writes `Heading` unwrapped today (`menu.go:76-78`), so a talk
   menu's heading breaks mid-word on phones; it is wrapped the same way. Preview (real output
   at every width, 2026-11-21): https://claude.ai/artifact/9T7DDNBvwhgKcVrLR3a3Ag. The swap
   alone changed nothing visible in the two sample menus; the heading is the visible change.
4b. **A shorter More line** (owner, 2026-11-21, wording A). `pager.StopLine` becomes
   `Page %d of %d: Enter, q to stop ` (30 columns at page 1 of 3, 32 at page 10 of 12), so it
   fits one row on every surface down to the 33-column phone, where today's 70-column line
   takes two rows of the page it exists to save. "1 of 3", never "1/3", which a screen
   reader can read as "one third". The pager rules are unchanged (Enter next page, `q`
   stops, anything else runs). Update `pager_test.go`, the `help` text that quotes the line
   if any (grep "More (page"), and the patch note.
5. **Guard test** in `internal/guards/`: an AST scan of `internal/` and `modules/` for
   functions that word-wrap (name matches `(?i)wrap|splitstring`) outside `internal/messaging`.
   Each existing one is on an allowlist with its reason; a new one fails the build. Proven
   able to fail by adding a scratch wrapper.
6. **Ship.** Patch note ("Long prompts and the More line now wrap between words on narrow
   screens"), pre-push gates, boot check, a 38-column screenshot before and after, PR.

## Out of scope: the owner's call after this lands

`util.SplitString` / `SplitStringNL` and the `splitstring` template function are a second
full word wrapper, used in about 30 files and 8 templates (room descriptions, help, look,
companion, auctions). `templates.wrapText` already notes it breaks brackets that
`WrapAnsi` keeps. Folding it into `WrapAnsi` changes player-visible text across the game
and needs snapshot review, so it is a separate change. The guard allowlists it with that
reason until then.

## Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| Prompts wrap at `WrapWidth()`, the width game text uses, not `PagerWidth()` | A prompt is text the player reads beside game text; one width keeps them aligned, and `set linewidth` applies to both | Wrap at the reported screen width |
| Telnet prompts wrap too (at `WrapWidth`, which is the NAWS width when reported) | The 11-19 narrow-telnet fix made game text fit the reported screen; prompts should match | Leave telnet prompts unwrapped as before |
| The player's own custom prompt is wrapped like the default | It breaks mid-bar on a phone the same way | Wrap only the stop line and questions |
| The erase stays a lower bound: lines above the last are counted as one row each | Same rule as #922: a stray row is harmless, climbing too far erases game text | Count each above line's rows at the current width |
| `util.SplitString` is not folded in here | Changes text across the whole game; needs its own snapshot review | Fold it in now |
