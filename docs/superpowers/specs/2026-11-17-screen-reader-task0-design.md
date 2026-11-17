# Screen reader support, Task 0 of the touch arc

Status: direction approved by the owner 2026-11-17 in conversation; this file awaits his review.
Issue: #733 (this spec delivers its first part; the issue stays open for the rest). Epic: #783, as Task 0 before any child.
Prior art report: in the conversation of 2026-11-17 (summarised under "Prior art" below).

## Facts verified against source

Read 2026-11-17 from master `e3343562b`. `WP` is `_datafiles/html/public/webclient-pure.html`; `CSS` is `_datafiles/html/public/static/css/dashboard.css`; `XT` is `_datafiles/html/public/static/js/xterm.4.19.0.js`.

| Fact | Where |
|---|---|
| Players type into `#command-input`, a plain text input with a visible placeholder "Enter command..." and no `aria-label` | `WP:310` |
| The page connects by itself on load, writes "Type your character name in the box below and press Enter. New here? Type new." and focuses the command box | `WP:2928-2931` |
| xterm is built without `screenReaderMode`; the page has no `aria-live` region anywhere | `WP:478-504`; grep of `WP` and `JS/` |
| xterm 4.19 switches `screenReaderMode` at runtime: `setOption('screenReaderMode', true)` creates its `AccessibilityManager` (hidden live region and row mirror), `false` disposes it | `XT`, `setOption` and the `case"screenReaderMode"` branch |
| The client already intercepts one typed command before sending: `!net` | `WP` Enter handler, `:3342-3357` |
| A password prompt switches the command box to `type="password"` through the `TEXTMASK:` webclient command | `WP:3001-3011` |
| The client reports its width over GMCP `Client.Display` as `{cols: n}`; the server stores it per connection, before or after login | `WP:543`; `modules/gmcp/gmcp.go:431`; `modules/gmcp/gmcp.ClientDisplay.go`; `internal/users/wrapwidth.go:38` |
| Per-connection client facts live in `connections.ClientSettings` (`Display`, `MSPEnabled`, `SendTelnetGoAhead`, `AsciiMode`) | `internal/connections/clientsettings.go:10-15` |
| Device-remembered settings use `localStorage` inside try/catch (sound on) | `WP:3573`, `:3736` |
| A `.visually-hidden` utility exists ("Text for screen readers only") | `CSS:1959-1968` |
| Creation asks "Are you using a screen reader?" as step `screen-reader-new`; the answer sets `UserRecord.ScreenReader` | `internal/inputhandlers/login.go:307-320`, `:160` |
| Login step conditions see only the results map, not the connection: `type ConditionFunc func(results map[string]string) bool` | `internal/inputhandlers/login_prompt_handler.go:49` |
| `set screenreader` flips `UserRecord.ScreenReader`, prints "ScreenReader mode toggled ON/OFF." and fires `UserSettingChanged{Name: "screenreader"}` | `internal/usercommands/set.go:216-230` |
| With the flag on, the server strips art characters from output, picks `.screenreader.template` variants and sends splash screens as words only | `internal/hooks/Message_SendMessages.go:37`, `:97`; `internal/templates/templates.go:191`, `:289`; `internal/hooks/Splash_Deliver.go:44-51` |
| `Char.Info` carries `account`, `name`, `class`, `race` | `modules/gmcp/gmcp.Char.go:492-502`, `:942-947` |
| Usernames match `^[a-zA-Z0-9_]+$`, so a phrase with a space can never be a username | `_datafiles/config.yaml:2737` |
| Telnet: Terminal Type (option 24) is defined but never negotiated; connect sends MSP, MSSP and suppress go-ahead only; NAWS replies are parsed into `ClientSettings` | `internal/term/telnet.go:83`; `main.go:735-752`; `internal/inputhandlers/term_iac.go:142-160` |

## Prior art

- No MUD found solves this in a browser. Alter Aeon and Materia Magica point blind players at a desktop client; Icesus makes its whole web output a live region (a plain HTML client, not xterm).
- A browser cannot detect a screen reader; W3C guidance (2020) says it must not without consent.
- Microsoft's Monaco editor puts the opt-in in the field's spoken label: screen reader users hear which key turns on screen reader mode, sighted users see nothing. Users complained when the choice was not remembered.
- MTTS, the MUD telnet terminal-type standard, has bit 64 "screen reader"; Evennia decodes it.

## Rulings (owner, 2026-11-17)

1. Sighted players must not pay for this: no checkbox, nothing visible, and xterm's screen reader mode stays off unless a player asks for it.
2. The opt-in lives in what a screen reader speaks before the first prompt: the command box's label.
3. Telnet MTTS detection is in scope.
4. The web client does not ask the creation question; the opt-in is the answer. Telnet keeps the question unless MTTS already answered it.

## What players get

- A blind player opening the web page hears, before anything else: "Game commands. Screen reader users: type screen reader on, then press Enter." Typing it turns on speech at once, so the login and creation prompts are read aloud, and the device remembers it.
- A sighted player sees and hears nothing new.
- A blind player on Mudlet, TinTin++ or another client that declares a screen reader is not asked the question; the account gets screen reader output from the start.
- Every player is asked about a screen reader at most once, at creation, and only on a telnet client that does not answer MTTS. Web players are never asked.

## Design

### 1. The spoken label (`WP`)

- `#command-input` gets `aria-label="Game commands. Screen reader users: type screen reader on, then press Enter."` The visible placeholder is unchanged.
- A `.visually-hidden` paragraph is the first element in `body` with the same sentence. iOS VoiceOver does not always follow programmatic focus on load and starts at the top of the page, so the sentence must be the first thing there too. Sighted players see nothing.
- Once screen reader mode is on, the label shortens to "Game commands." and the hidden paragraph says "Screen reader mode is on. Type screen reader off to turn it off."

### 2. The phrase (`WP` Enter handler)

- The client intercepts `screen reader on` and `screen reader off` the way it intercepts `!net`: matched case-insensitively with spaces collapsed, never sent to the server, never added to command history.
- Not intercepted while the box is `type="password"`.
- On: `term.setOption('screenReaderMode', true)`, remember on this device, report to the server (section 4), and write one confirming line into the game window: "Screen reader mode is on." Off: the reverse, with "Screen reader mode is off."
- After login, `screen reader on/off` also changes the account, by sending `set screenreader` only when the account's current value differs (the client knows it from section 5). So the phrase and `set screenreader` are two ways to the same switch.

### 3. Remembered on the device (`WP`)

- `localStorage` key `screenReader`, inside try/catch like `soundOn`. On load, when it is true, the client turns xterm's mode on before connecting and reports it, so a returning player never types the phrase again.

### 4. The client reports it (`WP`, `modules/gmcp`, `internal/connections`)

- `Client.Display` gains a field: `{cols: n, screenreader: true|false}`, sent when the phrase is typed, on load when remembered, and with every width report.
- `ClientSettings` gains `ScreenReader bool`. The `Client.Display` handler stores it for websocket connections only, as it does `cols`. A missing field leaves the stored value as it is.

### 5. The server tells the client (`modules/gmcp`)

- `Char.Info` gains `screenreader` (a bool always sent, never `omitempty`, so false turns another device off).
- `set screenreader` pushes `Char.Info` again. The client sets xterm's mode and the device memory to match whenever `Char.Info` arrives.

### 6. Login and creation (`internal/inputhandlers`)

- Creation: the `screen-reader-new` step is skipped when the connection is a websocket, or when the telnet client answered MTTS at all (with bit 64 set or not: an MTTS reply without it is a "no", and asking again would ask twice). When skipped, the answer is the connection's `ClientSettings.ScreenReader`. So every player is asked at most once, at creation, and a web player never.
- `ClientSettings` also gains `MTTSReported bool`, set when an MTTS reply is parsed.
- Conditions cannot see the connection (`ConditionFunc` takes only results), so the prompt handler writes the connection's facts into the results map under reserved keys before evaluating each step: `_conn_websocket`, `_conn_mtts` and `_conn_screenreader` ("y" or "n"). Keys starting `_conn_` are reserved; no step may use one as its ID (guarded by a test).
- Login to an existing account: when the connection reports a screen reader and the account flag is off, the flag is turned on and the player is told: "Screen reader mode is on for this character. Type set screenreader to turn it off." A connection that does not report one never turns the flag off.

### 7. Telnet MTTS (`internal/term`, `internal/inputhandlers`, `main.go`)

- Connect sends `IAC DO TTYPE` alongside the existing offers.
- On `IAC WILL TTYPE` the server sends `IAC SB TTYPE SEND IAC SE` and repeats on each reply, up to three times, stopping early when a reply repeats the previous one (the MTTS cycle).
- A reply of the form `MTTS <n>` sets `ClientSettings.ScreenReader` from bit 64. Other replies (client name, terminal type) are logged at debug level and otherwise ignored.
- A client that answers `WONT` or never answers changes nothing, and the creation question is asked as today.

## Decisions taken without asking

- **The phrase is `screen reader on`.** Plain words, easy to dictate on a phone, and the space keeps it from ever being a username. If wrong: any other phrase works, but a single word could collide with a name at the first prompt.
- **The phrase rather than a key combination.** Phone screen readers have no Alt key. If wrong: a key combination could be added beside the phrase for desktop.
- **A connection's report can turn an account's flag on but never off.** A blind player logging in from a new device must not be left without speech by an old "no". The cost: a sighted person on a device a blind player uses gets screen reader output once, and the message says how to turn it off. If wrong: the account flag would win at login, and a blind player could be silenced by an old answer.
- **Device memory comes before the account.** The device remembers the last state, so the login screen speaks for a returning blind player before the server knows who they are.
- **The flag's server effects are unchanged.** It still strips art and picks screen reader templates for both web and telnet. If wrong: web players might want xterm's speech without the art stripping, which would split the flag in two.
- **xterm's own screen reader mode, not a hand-built live region.** It is what the owner asked to turn on, and it switches at runtime. #733's question list (combat bursts, prompt repeats, vitals, comms, a re-read key) is answered after real use, in #733.
- **The confirmation line is written locally into the game window,** so it is read by the newly enabled live region and is visible in the log.

## Out of scope

The rest of #733: comms panels as `role="log"`, connection status as `role="status"`, toasts, combat burst handling, prompt repetition, vitals announcements, a re-read key. #734 (Tab trap). The red colour on both ON and OFF in the `set screenreader` message.

## Testing

- **Go:** the `Client.Display` handler stores `screenreader` for websockets and ignores it for telnet; a missing field keeps the stored value. MTTS parsing: bit 64 set and unset, non-MTTS replies, a repeating cycle stops, `WONT` changes nothing. Creation skips the question on websocket and on any MTTS reply (bit 64 set and unset), takes the right answer, and still asks a telnet client that sent no MTTS. Existing-account login turns the flag on and never off. A guard that no login step ID starts `_conn_`. `Char.Info` always carries `screenreader`, false included.
- **JS (`tools/jstest`):** the phrase matcher (case, extra spaces, not in password mode, not sent, not in history).
- **Headless (Playwright):** sighted path: no `.xterm-accessibility` element exists after load and login. Opt-in path: after typing the phrase the element exists, the label has shortened, the phrase never reached the socket, and a reload with the remembered value turns it on before connecting.
- **Owner:** Windows Narrator (built in) with Chrome: open the page, hear the instruction, type the phrase, create a character by ear. The question must not be asked.
- **Malia (#834):** VoiceOver on iPhone, TalkBack on Android: the instruction is heard from the top of the page.
- **Telnet:** TinTin++ and Mudlet with their screen reader settings on, if they send MTTS bit 64 (not verified for either client).
