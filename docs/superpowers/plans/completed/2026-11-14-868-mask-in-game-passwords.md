# #868: in-game password prompts are masked (spec and plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** While the game asks an in-game secret question (the `password` command's current, new and confirm questions), what the player types is echoed as `*`, kept out of the server's input history, and typed into a password field in the web client. Login is unchanged.

**Architecture:** A prompt question carries a mask flag (the existing, unused `Question.Flags`). `UserRecord.InputMasked()` says whether the player's next line answers a masked question. The connection-layer echo and history handlers ask it. `UserRecord.SyncTextMask()` sends `TEXTMASK:true/false` to a web client when that state changes, after each line is handled and whenever a prompt is cleared.

**Tech Stack:** Go (`internal/prompt`, `internal/users`, `internal/inputhandlers`, `internal/usercommands`, `world.go`).

Issue: #868 (bug, live on production, confirmed by the owner 2026-11-14). Design approved by the owner 2026-11-14 in conversation, with this combined document instead of a separate spec and plan. Branch `fix/868-mask-password-prompts` off master; independent of the plan 6 and 7 PRs (#869, #870).

Every agent: Edit and Write tools only for files, never a shell write into a tracked file; `git add` named paths only; never stash; commit through a quoted heredoc ending with a blank line and `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; no servers except where a task says so, killed by PID.

---

## Spec

### What happens today

`password` asks its questions through an in-game prompt (`internal/usercommands/password.go:20`, `:32`, `:39`). The server echoes every typed line back (`internal/inputhandlers/echo.go:28-31`, per keystroke on telnet, per line on the web), saves it in the input history (`internal/inputhandlers/inputhistory.go:8-12`), and the web client's command box stays a text field. So the typed passwords show in the game window and scrollback and can be recalled with the up arrow. Login masks all of this itself (`internal/inputhandlers/login_prompt_handler.go:229-249`, `:261-279`, `:383-393`), so the login screen is not affected.

### Rule

A question that asks for a secret is masked. Today that is `password`'s three questions. `deletecharacter`'s confirmation asks for the character's name, which is not a secret, and stays plain. A future secret question sets the flag; nothing else is needed.

### Behaviour

- Telnet: while the next answer is masked, each printable keystroke echoes as `*`; editing keys (backspace) echo as today, so the line still edits. The session already runs with the server echoing (`main.go:720`, WILL ECHO), so the client shows nothing else.
- Web: the line echoes as one `*` per character; the client gets `TEXTMASK:true` and turns its command box into a password field (it already handles `TEXTMASK` and leaves password-field lines out of its own history); `TEXTMASK:false` follows when the masked question is answered, abandoned or replaced.
- Neither kind of client gets the line saved in the server's input history.
- `TEXTMASK` is sent only on a change, after each line is handled and whenever a prompt is cleared. A reconnect starts unmasked on the client, so the server's record of what it last sent resets on a new connection.

### Facts verified against source

Read 2026-11-14 from master `95e61dddf`.

| Fact | Where |
|---|---|
| `Question` has `Flags int // Mask reply etc`, unused by any code | `internal/prompt/prompt.go:34-45`; grep finds no reader |
| `Ask` returns an existing question with the same text, else appends a new one | `internal/prompt/prompt.go:81-118` |
| `GetNextQuestion` returns the last question while any is not done, else nil | `internal/prompt/prompt.go:160-170` |
| `password` asks three questions with `cmdPrompt.Ask(...)` and checks `.Done` | `internal/usercommands/password.go:20`, `:32`, `:39` |
| `UserRecord` holds `activePrompt` and `activeMenu` under `sessionMu`; `menuOrPromptOpenLocked` | `internal/users/userrecord.menu.go:36-46` |
| `StartPrompt`, `GetPrompt`, `ClearPrompt` (locks `sessionMu`, signals menu state) | `internal/users/userrecord.go:803-856` |
| `ClearFinishedPrompt` calls `ClearPrompt` | `internal/users/userrecord.promptroute.go:74-78` |
| `users.GetByConnectionId(connectionId)` | `internal/users/users.go:209` |
| `inputhandlers` already imports `users` | `internal/inputhandlers/login.go:15` and others |
| `EchoInputHandler` echoes `DataIn` (keystroke or line), then CRLF on Enter; item-handle lines are silent | `internal/inputhandlers/echo.go:11-42` |
| `HistoryInputHandler` adds the buffer on Enter | `internal/inputhandlers/inputhistory.go:6-15` |
| Login sends `TEXTMASK` with `connections.SendTo` before its prompt, for websocket connections only | `internal/inputhandlers/login_prompt_handler.go:383-393` |
| The web client handles `TEXTMASK:` frames and skips password-field lines in its history | `_datafiles/html/public/webclient-pure.html` (grep `TEXTMASK`, `textInput.type !== "password"`) |
| `processInput` handles every line; the user is looked up first | `world.go:973-981` |

### Decisions taken without asking

| Decision | Why | If wrong |
|---|---|---|
| The flag lives in the existing `Question.Flags` (`FlagMaskInput`) with `Masked()` and a chainable `MaskInput()` | The field was reserved for exactly this ("Mask reply etc") | A bool field instead |
| A menu open over a prompt means not masked | A menu takes the line first (`RouteMenuInput`) | None known |
| Editing keys echo unmasked on telnet | The login handler does the same; masking them would break line editing | None known |
| `TEXTMASK` sent with `connections.SendTo` (synchronous), as login does | Login moved off the event queue to avoid a race where the field switched after the player began typing | None known |

### Out of scope

Login (already masked). Any other prompt that later asks for a secret sets the flag itself.

---

## Plan

### Task 0: Worktree

- [ ] From the main checkout (on `fix/868-mask-password-prompts` with this document): commit this document on the branch, switch the main checkout back to master, and `git worktree add C:/tmp/dogmud-868 fix/868-mask-password-prompts`. Record the worktree in memory beside #868.

### Task 1: The mask flag; `password` uses it

**Files:** `internal/prompt/prompt.go`, `internal/prompt/prompt_test.go` (create if absent; follow the package's test style), `internal/usercommands/password.go`, a test in `internal/usercommands`.

- [ ] **Failing tests.** In `internal/prompt`:

```go
func TestQuestionMaskInput(t *testing.T) {
	p := New(`password`, ``)
	q := p.Ask(`What is your current password?`, []string{})
	require.False(t, q.Masked())
	require.Same(t, q, q.MaskInput(), "chainable")
	require.True(t, q.Masked())
	require.True(t, p.Ask(`What is your current password?`, []string{}).Masked(), "asking again returns the same, masked question")
	var none *Question
	require.False(t, none.Masked(), "a nil question is not masked")
}
```

  In `internal/usercommands` (model the user and room setup on an existing usercommands test that calls a command with a prompt; grep `StartPrompt` in `*_test.go`): run `Password("", user, room, 0)` for a user with a password set and assert `user.GetPrompt().GetNextQuestion().Masked()`; answer the first question (set `Response`/`Done` as `RoutePromptInput` would, or drive it through `RoutePromptInput`) and assert the next asked question is masked too.

- [ ] **Implement** in `internal/prompt/prompt.go`, after the `Question` type:

```go
// Question flags (Question.Flags).
const (
	// FlagMaskInput marks a question whose answer is a secret, such as a
	// password: the player's typing echoes as *, stays out of input
	// history, and a web client types it into a password field (#868).
	FlagMaskInput = 1 << iota
)

// Masked reports whether the question's answer is a secret (#868).
func (q *Question) Masked() bool { return q != nil && q.Flags&FlagMaskInput != 0 }

// MaskInput marks the question's answer as a secret and returns the question.
func (q *Question) MaskInput() *Question {
	q.Flags |= FlagMaskInput
	return q
}
```

  Update the `Flags` field comment to name `FlagMaskInput`. In `password.go` each of the three `cmdPrompt.Ask(...)` calls becomes `cmdPrompt.Ask(...).MaskInput()`.

- [ ] Tests pass (`go test -count=1 ./internal/prompt/ ./internal/usercommands/`). Commit `fix(password): the password questions are marked as secret (#868)`.

### Task 2: The user knows; the web client is told

**Files:** `internal/users/userrecord.go` (or a new `internal/users/userrecord.textmask.go` beside `userrecord.menu.go`), `world.go`, a test in `internal/users`.

- [ ] **Failing tests** in `internal/users` (use `NewTestUser`): with no prompt `InputMasked()` is false; after `StartPrompt` and an `Ask(...).MaskInput()` it is true; with a plain question it is false; with a game menu open over the prompt it is false. For `SyncTextMask`, inject the sender through a package variable seam (below) and assert: first sync while masked sends `true` once; a second sync sends nothing; after `ClearPrompt` (which syncs) `false` is sent once; a non-websocket connection sends nothing (the seam receives the "is websocket" decision too, or the test sets a fake); a new connection id resets what was last sent.

- [ ] **Implement:**

```go
// inputMaskedLocked is InputMasked for a caller holding sessionMu.
func (u *UserRecord) inputMaskedLocked() bool {
	if u.activeMenu != nil || u.activePrompt == nil {
		return false
	}
	return u.activePrompt.GetNextQuestion().Masked()
}

// InputMasked reports whether the player's next line answers a secret
// question, such as a password (#868): the echo shows * and the line stays
// out of input history.
func (u *UserRecord) InputMasked() bool {
	u.sessionMu.Lock()
	defer u.sessionMu.Unlock()
	return u.inputMaskedLocked()
}

// sendTextMask tells a web client to switch its command box to a password
// field or back. A seam for tests.
var sendTextMask = func(connId connections.ConnectionId, masked bool) {
	if !connections.IsWebsocket(connId) {
		return // telnet: the server's echo is already masked
	}
	cmd := "TEXTMASK:false"
	if masked {
		cmd = "TEXTMASK:true"
	}
	connections.SendTo([]byte(cmd), connId)
}

// SyncTextMask sends the web client TEXTMASK when the masked state changed
// since the last send on this connection (#868). Called after each line is
// handled and whenever a prompt is cleared.
func (u *UserRecord) SyncTextMask() {
	connId := u.ConnectionId()
	u.sessionMu.Lock()
	if u.textMaskConn != connId { // a new connection starts unmasked
		u.textMaskConn, u.textMaskSent = connId, false
	}
	masked := u.inputMaskedLocked()
	changed := masked != u.textMaskSent
	u.textMaskSent = masked
	u.sessionMu.Unlock()
	if changed {
		sendTextMask(connId, masked)
	}
}
```

  `UserRecord` gains unexported `textMaskSent bool` and `textMaskConn connections.ConnectionId` (check `ConnectionId()`'s return type and that it does not take `sessionMu`; if it does, read it before locking as written). `ClearPrompt` calls `u.SyncTextMask()` after it unlocks (restructure its `defer`). In `world.go` `processInput`, right after the nil-user return: `defer user.SyncTextMask()`.

- [ ] Tests pass (`go test -count=1 ./internal/users/ .`). Commit `fix(users): a web client gets a password field while a secret question is open (#868)`.

### Task 3: Echo and history

**Files:** `internal/inputhandlers/echo.go`, `internal/inputhandlers/inputhistory.go`, a test in `internal/inputhandlers`.

- [ ] **Failing tests:** with a seam `var inputMasked = func(id connections.ConnectionId) bool` (default: `u := users.GetByConnectionId(id); return u != nil && u.InputMasked()`), set it to true in a test and drive `EchoInputHandler` with a printable keystroke, a full websocket line and a backspace, capturing what is sent (find how existing inputhandlers tests capture `connections.SendTo` output; if none, add a second seam `var echoSend = connections.SendTo` and capture through it). Assert `*` per printable character, the backspace echoed as before, and CRLF on Enter; with the seam false, the raw text as today. `HistoryInputHandler` with the seam true adds nothing; false adds the line.

- [ ] **Implement:** in `EchoInputHandler`, after the item-handle branch, when `inputMasked(clientInput.ConnectionId)` is true, echo one `*` for each printable byte of `DataIn` (bytes 32 to 126 and UTF-8 starts; skip continuation bytes so a multi-byte letter shows one `*`) and pass other bytes through unchanged, mirroring the login handler (`login_prompt_handler.go:229-249`). In `HistoryInputHandler`, skip `History.Add` when masked. Comment both with #868.

- [ ] Tests pass (`go test -count=1 ./internal/inputhandlers/`). Commit `fix(input): a secret answer echoes as * and stays out of history (#868)`.

### Task 4: Check it live, docs, gates, PR

- [ ] **Live check.** Boot the worktree on private ports (the `dogmud-shipping` recipe, `HttpPort: 18090`), create a character through the real web client with Playwright (`new`, name, password twice, Enter, `n`, `y`), then: `password`, observe a `TEXTMASK:true` frame and the command box `type` `password`; type a wrong current password; record every websocket frame and the terminal buffer and confirm the typed text appears in neither (only `*`); observe `TEXTMASK:false` and the box back to `text`; press the up arrow in the command box and confirm the wrong password is not recalled. Telnet: connect to the private telnet port, log in, `password`, type a wrong password, confirm the received bytes hold `*` and not the text. Stop the server by its task.
- [ ] **Docs:** `internal/prompt/context.md` (the flag), `internal/users/context.md` (`InputMasked`, `SyncTextMask`), `internal/inputhandlers/context.md` (echo and history masking), `docs/PATCH_NOTES.md` (player framing: "Changing your password in game no longer shows what you type."), and `docs/README.md` only if a new file was created that its sections list.
- [ ] **Gates** (`dogmud-shipping`): gofmt, build, golangci-lint 0 issues, `go test ./...`, CI's JavaScript checks (`node --check`, every `tools/jstest` and `tools/webclient-tests` script; nothing JS changes, but run them), boot Server Ready with 0 panics, `LogToFile: false`. Opus review of the branch.
- [ ] **PR** against master, `--repo pruuk/DOGMud`, body: the bug, the fix, the live check results, deploy note (no migration, no config); it may say "Closes #868" because this PR fixes it. Ends with the Claude Code attribution line.
