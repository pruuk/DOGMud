# Comms tabs and sleeping shopkeepers: plan

**Date:** 2026-11-07. **Branch:** `fix/webclient-icon-and-channel-hints`.
**Source:** Calabe's rulings in session, 2026-11-07, and Malia's #808.
Subagent-driven; the controller keeps facts, merging and checks.

## Rulings (Calabe, 2026-11-07)

1. **Channel hints (#808).** While a Chat channel window has no messages, a
   short hint shows at the top of it; the first message removes it. Laptops
   also get the same text as the channel button's hover tooltip. Copy approved
   as in Task 1.
2. **Whisper echo.** A whisper you send shows in your own Whisper tab, as
   "You (to Ana): hi".
3. **Broadcast is an announcement, admin only.** Only admins and the tips
   timer post to Broadcasts. Every online player receives it, deafened players
   included. Tips also reach deafened players (`set tips` still turns them
   off). The `` ` `` shortcut moves to `chat`.
4. **Shout shows in the Chat sub-tab** of the Chat panel ("shout is loud, it
   is not just room speech").
5. **Sleeping shopkeepers wake for customers.** This replaces the 2026-07-30
   rule ("a refusal rather than a wake-up", `actions/sleeping_target.go`) for
   shopkeepers only. A keeper asleep in their shop wakes when a player who is
   not hidden walks in, and wakes and serves a customer already in the room
   who uses a shop command. A hidden player does not wake them.
6. **`wake <name>`** wakes a sleeping NPC; the asleep hint becomes
   `wake <name>`. Shout still wakes everyone.
7. **Light mode stays visible** (spec open item closed). **The 512 icon's
   see-through strip** is filled by us.

## Facts verified against source (2026-11-07)

| Fact | Where |
|---|---|
| `broadcast` command is `sendChannel(user, "chat", rest)` | `internal/usercommands/broadcast.go:12-14` |
| `broadcast` access row `{Broadcast, true, true, false}`: AllowedWhenDowned, AllowedInCombat, AdminOnly | `internal/usercommands/usercommands.go:86`, struct `:34-39` |
| `` ` `` aliases `broadcast` | `_datafiles/world/dogmud/keywords.yaml:324` |
| `broadcast` is a player help topic under `communication` | `keywords.yaml:46`; admin topics under `admin: all:` at `:201` |
| Only the tips timer sends CommType `broadcast`, one per player | `internal/hooks/NewRound_BroadcastTips.go:47-52` |
| Tips skip deafened players and `set tips` off | `NewRound_BroadcastTips.go:34-40` |
| GMCP `broadcast` reaches the target, or every online user if untargeted | `modules/gmcp/gmcp.Comm.go` `broadcastRecipients` |
| Web client files a Comm message into `comm-tab-<channel>` / `comm-<channel>`; unknown channels are dropped | `_datafiles/html/public/webclient-pure.html` Comm handler (~`:617-668`) |
| Shout sends text lines only, no Communication event; next door hears "Someone shouts from the <dir> direction" | `internal/actions/shout.go:30-71` |
| Shout wakes every sleeper in the room | `actions/shout.go:75-94` `wakeSleepers` |
| A player arriving with a light wakes every sleeper in the room | `internal/usercommands/go.go:503-525` |
| `OnSleeperWoken` stamps `schedule_wake_round`; the keeper stays up `ScheduleWakeGraceRounds` (50, Go default, key absent from `config.yaml`) | `internal/mobs/sleeper.go:18-27`, `internal/hooks/NewRound_IdleMobs_schedule.go:106-112`, `internal/configs/config.balance.mobs.go:38-39` |
| Shop commands gate on `RefuseIfShopClosed`: list, buy, sell, appraise, offer | `internal/usercommands/{list.go:35,buy.go:24,sell.go:26,appraise.go:25,offer.go:37}` |
| `ShopClosedForSleep` returns a sleeper only when every merchant here is asleep | `internal/actions/sleeping_target.go:81-101` |
| Asleep refusal text: "... is fast asleep. You could make some noise." + hint `shout wake up` | `internal/refusal/busy.go:23-24` |
| `RefuseIfAsleep` callers besides shops: ask, give, talk | `internal/usercommands/{ask.go,give.go,talk.go}` |
| `wake` is an alias of `noop` (from the original world setup) | `keywords.yaml:356`, `internal/usercommands/noop.go` |
| Night-trade guard exempts sleeping keepers because the shop refuses first | `internal/guards/shop_night_trade_guard_test.go:52-58` |
| Whisper GMCP went to the target only | `modules/gmcp/gmcp.Comm.go` (now `whisperDeliveries`, Task 2) |

## Tasks

**Task 0, done on the branch (uncommitted):** icon strip filled, shipped as
`icon-moons-512.png` (manifest, `BRAND_STANDARDS.md`, `docs/README.md`);
spec open item "Light stays visible" recorded.

**Task 1, channel hints and tooltips (client).** In `webclient-pure.html`
each `.chat-window` starts with a `.chat-hint` block; the Comm handler removes
it before appending the first message (so the 200-message trim never counts
it). Each channel button gets a `title` with the same words. Hint colour from
an existing token that already passes 4.5 in Dark and Light (`--quest-progress`
measured 4.95 / 4.97), commands in `<code>`. jstest: every channel window has
a hint and its button a title; the handler removes the hint.

| Channel | Hint |
|---|---|
| Say | What people say in the room you are in. To talk, type `say hello`. |
| Whisper | Private messages between you and one player, wherever they are. Type `whisper <name> <message>`. `reply <message>` answers the last one. |
| Party | Only your party hears this. Type `party say <message>`. No party yet? `party create`, then `party invite <name>`. |
| Broadcasts | Game tips and announcements. No one can post here. `set tips` turns tips off or on. |
| Chat | Everyone online, for general talk. Type `chat <message>`. Shouts from nearby show here too. `channels` turns a channel off. |
| Newbie | Everyone online. Ask anything about the game, and other players will help. No question is too small. Type `newbie <message>`. |
| Trade | Everyone online, for buying and selling. Type `trade <message>`. |

(Broadcasts and Chat lines changed from the approved copy only to match
rulings 3 and 4.)

**Task 2, whisper echo (server).** Done on the branch: `whisperDeliveries`
in `modules/gmcp/gmcp.Comm.go`, tests in `gmcp.Comm_whisper_test.go` (the
echo test fails with the echo off).

**Task 3, broadcast as an announcement (server).**
- `usercommands.go:86` AdminOnly true. `Broadcast` sends to every online
  user, deafened included: a text line `(announcement) <name>: <message>` in
  a system category, and one Communication `broadcast` per player (targeted,
  as tips are) so GMCP reaches each. Escapes ANSI as `sendChannel` does.
- `keywords.yaml`: `` ` `` moves to `chat`; `broadcast` moves from
  `communication` to `admin: all:`. Help template rewritten for admins.
- Tips: drop the deafened skip; keep `set tips`.
- Tests: a non-admin gets the unknown-command path; an admin's broadcast
  reaches a deafened player on both lanes; tips reach a deafened player and
  skip one with tips off; `` ` hi `` lands on chat.

**Task 4, shout in the Chat sub-tab (server and client).**
- `actions.Shout` fires a Communication `shout` (source, text, hidden flag).
- GMCP: a `shoutDeliveries` like `sayDeliveries`: the speaker's room reads
  the name by sight (the speaker reads their own), each adjacent room reads
  "Someone (<direction>)" as the text lane does; a player's shout skips
  deafened listeners, a mob's does not.
- Client: Comm `shout` is filed into the Chat window as "Kesh shouts: ...".
- Tests mirror the say tests.

**Task 5, shopkeepers wake for customers (server).**
- `go.go`: a player who is not hidden entering a room wakes every sleeping
  merchant there (`rooms.FindMerchant` mobs, keeper in its own shop), with a
  room line "Marn wakes as you come in." through `OnSleeperWoken`.
- `RefuseIfShopClosed`: when every merchant is asleep, wake one (same line)
  and let the command go on, instead of refusing.
- Night-trade guard: a woken keeper must be able to trade at every hour it
  can sleep in its shop (the lighting rule: keepers carry their own light
  while awake). Extend the guard to sleeping segments; fix any keeper it
  finds.
- Add `ScheduleWakeGraceRounds: 50` to `config.yaml` (no change in value).
- Tests: entry wakes, hidden entry does not, `list` wakes and lists, keeper
  dozes off again after the grace.

**Task 6, `wake <name>` (server).**
- New `wake` command: `wake <name>` wakes that sleeper in the room (NPC or
  player), through `OnSleeperWoken`, with a room line. Bare `wake` keeps
  today's behaviour (check first how a sleeping player wakes themselves; the
  `noop` alias exists for that).
- Asleep refusal hint becomes `wake <name>`; `refusal/busy_test.go` updated.
- Help template `wake`, `keywords.yaml` help entry, alias removed.

**Task 7, docs and gates.** Patch notes, `docs/README.md` rows for new
files, rulings recorded where enforced (tests above) and in the rulings
memory. Gates: gofmt, build, tests for touched packages, golangci-lint,
every jstest and webclient-test, boot check, then PR with `--repo
pruuk/DOGMud`.
