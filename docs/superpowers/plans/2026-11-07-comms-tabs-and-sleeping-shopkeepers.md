# Comms tabs and sleeping shopkeepers: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the approved spec `docs/superpowers/specs/2026-11-07-comms-tabs-and-sleeping-shopkeepers-design.md`: Chat channel hints, the whisper echo, `broadcast` as an admin announcement, shouts in the Chat tab, shopkeepers who wake for customers, and a `wake <name>` command.

**Architecture:** Server events stay the source of truth: every Chat-tab line is a `Comm.Channel` GMCP payload built in `modules/gmcp/gmcp.Comm.go` from an `events.Communication`; every text line goes through the existing room and broadcast fan-out. Shopkeeper waking lives in `internal/usercommands` because it must relight the keeper's lantern through `behaviortree`, which `internal/actions` cannot import (`behaviortree` imports `actions`).

**Tech Stack:** Go (engine, tests with `testing` and `testify/require`), the web client page `webclient-pure.html` (vanilla JS), Node jstests under `tools/jstest/`.

**Branch:** `feat/comms-tabs-and-sleeping-shopkeepers`, rebased on master `375015999`, in the worktree `C:/tmp/dogmud-comms` (remove after merge). The whisper echo is already committed there (`3eed75cd1`).

**Rules for every task:** edit files only with the Edit and Write tools (no `sed -i`, no shell redirection into tracked files, no Python writes); `git add` by name; no em or en dashes in code comments or copy; every new test is shown to FAIL against the code before its fix (break the fix with `false &&` or equivalent, run, restore) and that is reported.

---

## Facts verified against source (master `375015999`, 2026-11-07)

| Fact | Where |
|---|---|
| Web client Comm handler starts `"Comm": function()`; looks up `comm-tab-<channel>` / `comm-<channel>`; appends the line at `panel.appendChild(p)`; trims at `childElementCount > 200` | `_datafiles/html/public/webclient-pure.html:619`, `:632`, `:658`, `:661` |
| Chat channel buttons and windows (7 each, `comm-tab-say` ... `comm-trade`) | `webclient-pure.html:359-374` |
| `.chat-window` style block | `webclient-pure.html:3693` |
| Whisper echo already built: `whisperDeliveries`, tests `gmcp.Comm_whisper_test.go` | `modules/gmcp/gmcp.Comm.go`, branch commit `3eed75cd1` |
| `sayDeliveries(evt, base) []commDelivery` derives the speaker's room, applies sight, deafen and `ExcludeUserIds` | `modules/gmcp/gmcp.Comm.go:166` |
| `events.Communication` fields: `SourceUserId, SourceMobInstanceId, TargetUserId, CommType, Name, Message, SpeakerHidden, ExcludeUserIds`; its comment already lists `shout` | `internal/events/eventtypes.go:205-218` |
| Only `say` makes the AI companion react; other CommTypes are ignored there | `modules/aicompanion/listeners.go:57` |
| Player `broadcast` = `sendChannel(user, "chat", rest)`; registry row `{Broadcast, true, true, false}` (AllowedWhenDowned, AllowedInCombat, AdminOnly) | `internal/usercommands/broadcast.go`, `internal/usercommands/usercommands.go:87` |
| Mobs also have `broadcast` (sends `events.Broadcast` text to everyone); no shipped content, script or Go caller uses it; it appears in the smoke list `mobcommands_test.go:1027`; `portalstake_test.go:104` seeds `` ` `` as a `broadcast` alias | `internal/mobcommands/mobcommands.go:32`, `internal/mobcommands/broadcast.go` |
| `events.Broadcast{Text, TextScreenReader, IsCommunication, SourceIsMod, SkipLineRefresh}`; `Broadcast_SendToAll` skips a deafened player only when `IsCommunication && !SourceIsMod` | `internal/events/eventtypes.go:164`, `internal/hooks/hooks.go:146` |
| `` ` `` aliases `broadcast`; `broadcast` is listed under the player help topic `communication` | `_datafiles/world/dogmud/keywords.yaml:324`, `:46` |
| Admin help templates live at `templates/admincommands/help/command.<name>.template` | `_datafiles/world/dogmud/templates/admincommands/help/` |
| Tips skip deafened players at `if u.Deafened { continue }`; `set tips` checked next | `internal/hooks/NewRound_BroadcastTips.go:34-40` |
| The tips test asserts a deafened player gets no tip | `internal/hooks/broadcast_tips_test.go:40-42` |
| Event drain helpers exist per type (`DrainQueuedMessagesForTest`, `DrainQueuedCharacterDiedForTest` ...); none for `Communication` or `Broadcast` | `internal/events/events.go:270`, `:442` |
| `actions.Shout(actor, text)` sends the room line via `sendSpoken`, next-door lines via `room.ForEachAdjacentRoom(func(otherRoom *rooms.Room, sourceExit string))` ("Someone shouts from the <exit> direction"), then `wakeSleepers`; fires no Communication event | `internal/actions/shout.go:29-71` |
| Light arrival wakes every sleeper in the destination room | `internal/usercommands/go.go:503-525` |
| `rooms.FindMerchant` matches a mob with `HasShop() && !OffShift()` | `internal/rooms/rooms.go:1903` |
| Shop commands gate on `actions.RefuseIfShopClosed(room, user)`: `list`, `buy`, `sell`, `appraise`, `offer` | `internal/usercommands/{list.go:35,buy.go:24,sell.go:26,appraise.go:25,offer.go:37}` |
| `ShopClosedForSleep(room)` returns a sleeper only when every merchant there is asleep | `internal/actions/sleeping_target.go:81` |
| `behaviortree` imports `actions`, so `actions` cannot call item trees | `go list` on `internal/behaviortree` |
| A keeper's lantern goes dark asleep and relights through its light-slot tree on `item_idle` (`behaviortree.TryItemBehavior(behaviortree.EventContext{EventType: "item_idle"}, behaviortree.ItemSubject{UUID, ItemId, MobInstanceId, Slot: "light"})`) | `internal/hooks/NewRound_ItemRoundTick.go:57-81`, `internal/guards/shop_night_trade_guard_test.go:233-235` |
| Schedule sleep decision: `if seg.Activity == "sleeping" && mob.Character.RoomId == seg.TargetRoom { lastWoken ...; grace ...; if lastWoken == 0 \|\| ... >= grace { plan.WantsSleep = true } }` in `scheduleTickPlan(mob, hour24)` | `internal/hooks/NewRound_IdleMobs_schedule.go:106-113` |
| `ScheduleWakeGraceRounds` (Go default 50) and `ScheduleMaxPathRetries` are absent from `config.yaml` | `internal/configs/config.balance.go:765`, `config.balance.mobs.go:38-39` |
| `MobBTreeReactionBase` sits at `config.yaml:2128` in the Balance block | `_datafiles/config.yaml:2128` |
| Night-trade guard: per hour, sets Sleeping (condition 15) for a sleeping segment, runs the lantern tree, and checks `actions.ShopSightRefusal(observer, r)` only in `scheduledRoom`, which returns nothing for a sleeping segment | `internal/guards/shop_night_trade_guard_test.go:216-268`, `:344-358` |
| Asleep refusal text: `... is fast asleep. You could make some noise.` + hint `shout wake up`; its test | `internal/refusal/busy.go:23-24`, `internal/refusal/busy_test.go:27` |
| `wake` is an alias of `noop`; any command wakes a sleeping player first except `sit` and `rest` (`keepsSleep`) | `keywords.yaml:356`, `internal/usercommands/usercommands.go:55-66` |
| `sleep` is listed in the player help topics at `keywords.yaml:39` | `_datafiles/world/dogmud/keywords.yaml` |
| Player name lookup that respects sight: `room.FindByNameSeenBy(&user.Character, name)` returns `(playerId, mobInstanceId)` | `internal/rooms/rooms.go:2153` |
| Command parity guard expects only `throw`; any new one-sided command needs an allowlist entry | `internal/guards/command_parity_guard_test.go`, `internal/actions/divergences.go` |
| Help completeness test: every user command needs `templates/help/<name>.template`; admin commands need `templates/admincommands/help/command.<name>.template` | `internal/usercommands/helpfile_completeness_test.go` |
| Test helpers: `users.NewTestUser(id, user, char, conn)`, `users.SeedUsersForTest`, `mobs.SeedMobsForTest(specs, instances)`, `rooms.SeedRoomsForTest(rooms, zones)`, `conditions.SeedConditionsForTest` with `Flags: []conditions.Flag{conditions.Sleeping}` | `internal/actions/craft_phase_lines_test.go:77-79`, `internal/mobs/test_helpers.go:8` |

---

## File structure

- `webclient-pure.html`: hint blocks in the 7 channel windows, `title` on the 7 buttons, the Comm handler removes a window's hint and files `shout` into the Chat window.
- `static/css/dashboard.css`: `.chat-hint` style.
- `tools/jstest/chat-hints.test.js` (new): hints, titles, handler.
- `internal/events/events.go`: `DrainQueuedCommunicationsForTest`, `DrainQueuedBroadcastsForTest`.
- `internal/usercommands/broadcast.go`: the admin announcement. `usercommands.go`: AdminOnly; the `wake` row.
- `internal/hooks/NewRound_BroadcastTips.go`: tips reach deafened players.
- `internal/actions/shout.go`: fires `Communication{CommType: shout}`.
- `modules/gmcp/gmcp.Comm.go`: `commSpeakerRoom`, `shoutDeliveries`.
- `internal/usercommands/shop_wake.go` (new): `wakeKeeper`, `wakeKeepersOnEntry`, `shopOpenFor`.
- `internal/usercommands/go.go`, `list.go`, `buy.go`, `sell.go`, `appraise.go`, `offer.go`: call the new helpers.
- `internal/hooks/NewRound_IdleMobs_schedule.go`: no dozing with a customer in the shop.
- `internal/usercommands/wake.go` (new): `wake <name>`.
- `internal/refusal/busy.go`: the asleep hint.
- Content: `keywords.yaml`, `templates/help/wake.template` (new), `templates/admincommands/help/command.broadcast.template` (new), `templates/help/broadcast.template` (deleted), `templates/help/sleep.template` check, `config.yaml`.
- Docs: `docs/PATCH_NOTES.md`, `docs/README.md`, `modules/gmcp/context.md`, `internal/usercommands/context.md`, `internal/hooks/context.md`, `internal/refusal/context.md` if it names the hint.

---

### Task 1: Chat channel hints and tooltips

**Files:**
- Modify: `_datafiles/html/public/webclient-pure.html:359-374` (buttons and windows), `:632-658` (Comm handler)
- Modify: `_datafiles/html/public/static/css/dashboard.css` (end of the Chat section)
- Create: `tools/jstest/chat-hints.test.js`

- [ ] **Step 1: Write the failing test** `tools/jstest/chat-hints.test.js`:

```js
// Chat channel hints (#808, Calabe 2026-11-07): each channel window starts
// with a hint the first message removes, and each button carries the same
// words as its tooltip.
'use strict';
var fs = require('fs');
var path = require('path');
var page = fs.readFileSync(path.join(__dirname, '..', '..', '_datafiles', 'html', 'public', 'webclient-pure.html'), 'utf8');
var failures = 0;
function check(name, got, want) {
    if (got === want) { console.log('ok   ' + name); return; }
    failures++; console.log('FAIL ' + name + '\n  got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want));
}
var hints = {
    say: 'What people say in the room you are in. To talk, type <code>say hello</code>.',
    whisper: 'Private messages between you and one player, wherever they are. Type <code>whisper &lt;name&gt; &lt;message&gt;</code>. <code>reply &lt;message&gt;</code> answers the last one.',
    party: 'Only your party hears this. Type <code>party say &lt;message&gt;</code>. No party yet? <code>party create</code>, then <code>party invite &lt;name&gt;</code>.',
    broadcast: 'Game tips and announcements. No one can post here. <code>set tips</code> turns tips off or on.',
    chat: 'Everyone online, for general talk. Type <code>chat &lt;message&gt;</code>. Shouts from nearby show here too. <code>channels</code> turns a channel off.',
    newbie: 'Everyone online. Ask anything about the game, and other players will help. No question is too small. Type <code>newbie &lt;message&gt;</code>.',
    trade: 'Everyone online, for buying and selling. Type <code>trade &lt;message&gt;</code>.'
};
function plain(html) { return html.replace(/<\/?code>/g, '').replace(/&lt;/g, '<').replace(/&gt;/g, '>'); }
Object.keys(hints).forEach(function (ch) {
    var win = new RegExp('<div id="comm-' + ch + '" class="chat-window[^"]*"><div class="chat-hint">([\\s\\S]*?)</div></div>').exec(page);
    check(ch + ' window starts with its hint', win ? win[1] : null, hints[ch]);
    var btn = new RegExp('<button[^>]*id="comm-tab-' + ch + '"[^>]*title="([^"]*)"').exec(page);
    check(ch + ' button tooltip', btn ? btn[1].replace(/&lt;/g, '<').replace(/&gt;/g, '>') : null, plain(hints[ch]));
});
var handlerAt = page.indexOf('"Comm": function()');
var handler = page.slice(handlerAt, page.indexOf('"Room": function()', handlerAt));
check('the handler removes a window\'s hint before the first line',
    /var hint = panel\.querySelector\('\.chat-hint'\);\s*if \(hint\) panel\.removeChild\(hint\);/.test(handler) &&
    handler.indexOf("querySelector('.chat-hint')") < handler.indexOf('panel.appendChild(p)'), true);
console.log(failures ? failures + ' checks FAILED' : 'all checks passed');
process.exit(failures ? 1 : 0);
```

- [ ] **Step 2: Run it to see it fail.** `node tools/jstest/chat-hints.test.js` Expected: 15 FAIL lines, exit 1.

- [ ] **Step 3: Add the hints and titles.** In `webclient-pure.html:359-365` add a `title` to each button, for example the Say button becomes:

```html
<button type="button" id="comm-tab-say" class="tab-button subtab active" aria-pressed="true" data-tab="comm-say" data-label="Say" data-unread="0" title="What people say in the room you are in. To talk, type say hello.">Say</button>
```

The titles are the plain-text forms of the hints in Step 1 (no `<code>`; `<` and `>` written as `&lt;` `&gt;`). In `:368-374` put each hint inside its window, for example:

```html
<div id="comm-say" class="chat-window say tab-content active"><div class="chat-hint">What people say in the room you are in. To talk, type <code>say hello</code>.</div></div>
```

using exactly the hint HTML from Step 1 for each channel.

- [ ] **Step 4: The handler removes the hint.** In the Comm handler, immediately before `panel.appendChild(p);` insert:

```js
                // The channel's hint goes once it has a line (#808).
                var hint = panel.querySelector('.chat-hint');
                if (hint) panel.removeChild(hint);
```

- [ ] **Step 5: Style.** Append to `dashboard.css` after the `.subtab` rules:

```css
/* Chat channel hints (#808, Calabe 2026-11-07): shown until the channel's
   first line. The quest hint colour: 4.95 on the dark panel, 4.97 in Light. */
.chat-hint {
  margin: 6px 8px;
  font-family: var(--font-body);
  font-size: 0.85rem;
  line-height: 1.4;
  color: var(--quest-progress);
}
.chat-hint code { font-family: var(--font-mono); }
```

- [ ] **Step 6: Run.** `node tools/jstest/chat-hints.test.js` Expected: `all checks passed`. Then every jstest the way CI does: `for t in tools/jstest/*.test.js tools/webclient-tests/*.js; do node "$t" >/dev/null || echo "FAIL $t"; done` Expected: no FAIL lines.

- [ ] **Step 7: Commit** `git add _datafiles/html/public/webclient-pure.html _datafiles/html/public/static/css/dashboard.css tools/jstest/chat-hints.test.js` and commit `feat(webclient): a hint and tooltip for each Chat channel (#808)`.

---

### Task 2: Event drain helpers for tests

**Files:** Modify `internal/events/events.go` (after `DrainQueuedCharacterDiedForTest`).

- [ ] **Step 1: Add the helpers**, the same shape as `DrainQueuedCharacterDiedForTest`:

```go
// DrainQueuedCommunicationsForTest removes every queued Communication event and
// returns them in queue order. FOR TEST USE ONLY.
func DrainQueuedCommunicationsForTest() []Communication {
	qLock.Lock()
	defer qLock.Unlock()

	var found []Communication
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if c, ok := pe.event.(Communication); ok {
			found = append(found, c)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}

// DrainQueuedBroadcastsForTest removes every queued Broadcast event and returns
// them in queue order. FOR TEST USE ONLY.
func DrainQueuedBroadcastsForTest() []Broadcast {
	qLock.Lock()
	defer qLock.Unlock()

	var found []Broadcast
	remaining := make(priorityQueue, 0, len(globalQueue))
	for _, pe := range globalQueue {
		if b, ok := pe.event.(Broadcast); ok {
			found = append(found, b)
			continue
		}
		remaining = append(remaining, pe)
	}
	globalQueue = remaining
	heap.Init(&globalQueue)
	return found
}
```

- [ ] **Step 2:** `go build ./internal/events/` Expected: no output. Commit `test(events): drain helpers for Communication and Broadcast`.

---

### Task 3: `broadcast` is an admin announcement; tips reach deafened players

**Files:**
- Modify: `internal/usercommands/broadcast.go`, `internal/usercommands/usercommands.go:87`
- Modify: `internal/hooks/NewRound_BroadcastTips.go:34-36`, `internal/hooks/broadcast_tips_test.go:40-42`
- Modify: `_datafiles/world/dogmud/keywords.yaml:46`, `:202` (admin list), `:324`
- Create: `_datafiles/world/dogmud/templates/admincommands/help/command.broadcast.template`
- Delete: `_datafiles/world/dogmud/templates/help/broadcast.template`
- Test: `internal/usercommands/broadcast_test.go` (new)

- [ ] **Step 1: Write the failing test** `internal/usercommands/broadcast_test.go`:

```go
package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-11-07: broadcast is an admin announcement. Every online
// player gets it, deafened players included, in the game text and the
// Broadcasts tab, as "(announcement) <admin>: <message>".
func TestBroadcastIsAnAdminAnnouncementEveryoneGets(t *testing.T) {
	require.True(t, userCommands[`broadcast`].AdminOnly, "only admins post to Broadcasts")

	admin := users.NewTestUser(9811, "calabe", "Calabe", 99811)
	deaf := users.NewTestUser(9812, "deaf", "Deaf", 99812)
	deaf.Deafened = true
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9811: admin, 9812: deaf}))
	events.DrainQueuedBroadcastsForTest()
	events.DrainQueuedCommunicationsForTest()

	_, err := Broadcast(`The server restarts in five minutes.`, admin, nil, 0)
	require.NoError(t, err)

	b := events.DrainQueuedBroadcastsForTest()
	require.Len(t, b, 1)
	require.Contains(t, b[0].Text, `(announcement)`)
	require.Contains(t, b[0].Text, `Calabe`)
	require.True(t, b[0].IsCommunication && b[0].SourceIsMod, "a mod's communication reaches deafened players")

	got := map[int]string{}
	for _, c := range events.DrainQueuedCommunicationsForTest() {
		require.Equal(t, `broadcast`, c.CommType)
		got[c.TargetUserId] = c.Name + `: ` + c.Message
	}
	require.Equal(t, `(announcement) Calabe: The server restarts in five minutes.`, got[9812])
	require.Equal(t, `(announcement) Calabe: The server restarts in five minutes.`, got[9811])
}

func TestBroadcastWithNothingToSayPostsNothing(t *testing.T) {
	admin := users.NewTestUser(9813, "calabe2", "Calabe", 99813)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9813: admin}))
	events.DrainQueuedBroadcastsForTest()
	events.DrainQueuedMessagesForTest(9813)
	_, err := Broadcast(`   `, admin, nil, 0)
	require.NoError(t, err)
	require.Empty(t, events.DrainQueuedBroadcastsForTest())
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(9813), ""), `Usage`)
}
```

- [ ] **Step 2:** `go test ./internal/usercommands/ -run 'Broadcast' -count=1` Expected: FAIL (`AdminOnly` false, no Broadcast event).

- [ ] **Step 3: Implement** `internal/usercommands/broadcast.go`:

```go
package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Broadcast is an admin announcement (owner 2026-11-07): every online player
// gets it, deafened players included, in the game text and in the web
// client's Broadcasts tab, which only admins and the tips timer post to.
// Player chat is the chat command (its shortcut is `).
func Broadcast(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	rest = strings.TrimSpace(rest)
	if rest == `` {
		user.SendText(messaging.CategoryUsage, `Usage: <ansi fg="command">broadcast</ansi> <message>`)
		return true, nil
	}
	rest = util.EscapeAnsiTags(rest)

	events.AddToQueue(events.Broadcast{
		Text: fmt.Sprintf(`<ansi fg="black-bold">(announcement)</ansi> <ansi fg="username">%s</ansi>: <ansi fg="yellow">%s</ansi>%s`,
			user.Character.Name, rest, term.CRLFStr),
		IsCommunication: true,
		SourceIsMod:     true,
	})

	// One Comm per player, aimed at them, as the tips are: an untargeted
	// broadcast would reach the web client of everyone online, deafened or not,
	// but this keeps the two lanes built the same way.
	for _, u := range users.GetAllActiveUsers() {
		events.AddToQueue(events.Communication{
			SourceUserId: user.UserId,
			TargetUserId: u.UserId,
			CommType:     `broadcast`,
			Name:         `(announcement) ` + user.Character.Name,
			Message:      rest,
		})
	}
	return true, nil
}
```

Check the import paths compile (`internal/term` is where `term.CRLFStr` lives, as `internal/usercommands/channel.go` uses it; copy that file's import line if it differs). In `usercommands.go:87` change the row to `{Broadcast, true, true, true}, // Admin only: an announcement to everyone (owner 2026-11-07)`.

- [ ] **Step 4: Tips reach deafened players.** In `NewRound_BroadcastTips.go` delete

```go
		if u.Deafened {
			continue
		}
```

and change the comment at the top from "to every active player who is not deafened and has not turned tips off" to "to every active player, deafened ones included (owner 2026-11-07), who has not turned tips off". In `broadcast_tips_test.go:40-42` replace the deafened assertion with:

```go
	if got := events.DrainQueuedMessagesForTest(9303); len(got) != 1 || !strings.Contains(got[0], "First tip.") {
		t.Errorf("deafened player got %q, want the tip (owner 2026-11-07)", got)
	}
```

- [ ] **Step 5: Keywords and help.** In `keywords.yaml`:
  - `:324` change `broadcast:          ['`']` to `chat:               ['`']` (the shortcut moves to chat).
  - `:46` delete `      - broadcast` from the `communication` list.
  - In `admin:` → `all:` (`:202`) add `      - broadcast` in alphabetical order (after `badcommands`).
  - `git rm _datafiles/world/dogmud/templates/help/broadcast.template`.
  - Create `templates/admincommands/help/command.broadcast.template`:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">broadcast</ansi>

Send an announcement to everyone online, deafened players included. It
shows in the game text and in the web client's Broadcasts tab as:

  (announcement) <your name>: <your message>

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">broadcast The server restarts in five minutes.</ansi>

Players talk to everyone with <ansi fg="command">chat</ansi> (shortcut <ansi fg="command">`</ansi>).
```

- [ ] **Step 5b: Remove the NPC `broadcast` (Calabe 2026-11-07: it is an admin command now, so parity does not need an NPC one; no content uses it).**
  - `internal/mobcommands/mobcommands.go:32`: delete the `"broadcast":      {Broadcast, false},` row.
  - `git rm internal/mobcommands/broadcast.go`.
  - `internal/mobcommands/mobcommands_test.go:1027`: delete `{"broadcast", "test"},` from the smoke list. (`usercommands_test.go:7328` keeps its `broadcast` line: the player command still exists.)
  - `internal/usercommands/portalstake_test.go:104`: the seeded alias becomes `map[string][]string{"chat": {"`"}}`, matching the new keywords.
  - `internal/actions/divergences.go`, admin section of the user-only list: add `"broadcast":    "admin: an announcement to everyone (owner 2026-11-07)",`.

- [ ] **Step 6: Run.** `go test ./internal/usercommands/ ./internal/mobcommands/ ./internal/hooks/ -count=1` and `go test ./internal/guards/ -run CommandParity -count=1`. Expected: PASS (parity still finds only `throw`). Prove the test can fail: set the row back to `false` AdminOnly, run, see FAIL, restore.

- [ ] **Step 7: Commit** by name and `feat: broadcast is an admin announcement everyone gets; tips reach deafened players`.

---

### Task 4: Shouts show in the Chat window

**Files:**
- Modify: `internal/actions/shout.go:52` (after `sendSpoken`)
- Modify: `modules/gmcp/gmcp.Comm.go` (routing, `commSpeakerRoom`, `shoutDeliveries`)
- Modify: `webclient-pure.html` Comm handler
- Test: `modules/gmcp/gmcp.Comm_shout_test.go` (new), `tools/jstest/chat-hints.test.js` (add checks)

- [ ] **Step 1: Write the failing Go test** `modules/gmcp/gmcp.Comm_shout_test.go`:

```go
package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// shoutFixture: Kesh (9761) and Ana (9762) in lit room 9760; Bo (9763) and the
// deafened Di (9764) next door in 9765, north of 9760. 9765's own exit back
// is south, which is what ForEachAdjacentRoom passes (FindExitTo from the
// listener's room) and what the game text says: "from the south direction".
func shoutFixture(t *testing.T) {
	t.Helper()
	lit := 60
	here := &rooms.Room{RoomId: 9760, Zone: "ShoutZone", Lamp: rooms.LampPtr(lit),
		Exits: map[string]exit.RoomExit{"north": {RoomId: 9765}}}
	there := &rooms.Room{RoomId: 9765, Zone: "ShoutZone", Lamp: rooms.LampPtr(lit),
		Exits: map[string]exit.RoomExit{"south": {RoomId: 9760}}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9760: here, 9765: there},
		map[string]*rooms.ZoneConfig{"ShoutZone": {Name: "ShoutZone", RoomId: 9760}}))
	mk := func(id int, name string, roomId int) *users.UserRecord {
		u := users.NewTestUser(id, name, name, 90000+id)
		u.Character.RoomId = roomId
		return u
	}
	kesh, ana, bo, di := mk(9761, "Kesh", 9760), mk(9762, "Ana", 9760), mk(9763, "Bo", 9765), mk(9764, "Di", 9765)
	di.Deafened = true
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9761: kesh, 9762: ana, 9763: bo, 9764: di}))
	here.AddPlayer(9761)
	here.AddPlayer(9762)
	there.AddPlayer(9763)
	there.AddPlayer(9764)
}

// Spec: a shout reaches the speaker's room by name (by sight, as say) and the
// rooms next door as "Someone (<exit>)", the same words as the game text; a
// player's shout skips deafened listeners.
func TestShoutDeliveriesNearAndNextDoor(t *testing.T) {
	shoutFixture(t)
	got := map[int]string{}
	for _, d := range shoutDeliveries(events.Communication{SourceUserId: 9761, CommType: `shout`, Name: `Kesh`, Message: `HERE I AM!`},
		GMCPCommModule_Payload{Channel: `shout`, Sender: `Kesh`, Text: `HERE I AM!`}) {
		got[d.UserId] = d.Payload.Sender
	}
	require.Equal(t, map[int]string{9761: `Kesh`, 9762: `Kesh`, 9763: `Someone (south)`}, got)
}
```

(`Room.Exits` is `map[string]exit.RoomExit`, `internal/rooms/rooms.go:116`; `rooms.LampPtr` is what `sayFixture` in `gmcp.Comm_test.go` uses. The direction is the listener's exit back toward the speaker, `adjacency.go:38`.)

- [ ] **Step 2:** `go test ./modules/gmcp/ -run Shout -count=1` Expected: FAIL (`shoutDeliveries` undefined).

- [ ] **Step 3: Implement in `gmcp.Comm.go`.** Extract the room lookup from the top of `sayDeliveries` into:

```go
// commSpeakerRoom is the room a say or shout was spoken in, or nil.
func commSpeakerRoom(evt events.Communication) *rooms.Room {
	if evt.SourceUserId > 0 {
		if user := users.GetByUserId(evt.SourceUserId); user != nil {
			return rooms.LoadRoom(user.Character.RoomId)
		}
	}
	if evt.SourceMobInstanceId > 0 {
		if mob := mobs.GetInstance(evt.SourceMobInstanceId); mob != nil {
			return rooms.LoadRoom(mob.Character.RoomId)
		}
	}
	return nil
}
```

and make `sayDeliveries` start with `room := commSpeakerRoom(evt); if room == nil { return nil }`. Then add:

```go
// shoutDeliveries is who receives a shout over GMCP (spec 2026-11-07): the
// speaker's room as for say (by sight), and every adjacent room as
// "Someone (<exit>)", the words the game text uses for a shout heard next
// door. A player's shout skips deafened listeners next door, as the text
// lane does; an NPC's reaches them.
func shoutDeliveries(evt events.Communication, base GMCPCommModule_Payload) []commDelivery {
	room := commSpeakerRoom(evt)
	if room == nil {
		return nil
	}
	deliveries := sayDeliveries(evt, base)
	room.ForEachAdjacentRoom(func(other *rooms.Room, sourceExit string) {
		for _, uid := range other.GetPlayers() {
			u := users.GetByUserId(uid)
			if u == nil || (evt.SourceUserId > 0 && u.Deafened) {
				continue
			}
			p := base
			p.Sender = `Someone (` + sourceExit + `)`
			deliveries = append(deliveries, commDelivery{UserId: uid, Payload: p})
		}
	})
	return deliveries
}
```

In `onComm`, after the `say` branch add:

```go
	} else if evt.CommType == `shout` {

		for _, d := range shoutDeliveries(evt, payload) {
			events.AddToQueue(GMCPOut{
				UserId:  d.UserId,
				Module:  `Comm.Channel`,
				Payload: d.Payload,
			})
		}
		return events.Continue
```

- [ ] **Step 4: Fire the event.** In `internal/actions/shout.go`, directly after `sendSpoken(actor, room, messaging.CategoryShout, ...)`:

```go
	// The web client's Chat window shows shouts too (owner 2026-11-07); gmcp
	// builds each listener's line from this.
	events.AddToQueue(events.Communication{
		SourceUserId:        actor.GetUserId(),
		SourceMobInstanceId: actor.GetMobInstanceId(),
		CommType:            `shout`,
		Name:                actor.GetName(),
		Message:             text,
		SpeakerHidden:       isSneaking,
	})
```

(`actions` already imports `events`, as `say.go` does.)

- [ ] **Step 5: Client.** In the Comm handler replace the two lookups with:

```js
                // Shouts go into the Chat window (Calabe 2026-11-07).
                var windowName = channelName === 'shout' ? 'chat' : channelName;
                var tab   = document.getElementById("comm-tab-" + windowName);
                var panel = document.getElementById("comm-" + windowName);
```

and the name line with:

```js
                nameSpan.textContent = (fromName != null ? fromName : '') + (channelName === 'shout' ? ' shouts' : '');
```

Add to `tools/jstest/chat-hints.test.js` before the final summary:

```js
check('shouts go into the Chat window',
    /var windowName = channelName === 'shout' \? 'chat' : channelName;/.test(handler), true);
check('a shout reads "<name> shouts: <words>"',
    /\(channelName === 'shout' \? ' shouts' : ''\)/.test(handler), true);
```

- [ ] **Step 6: Run** `go test ./modules/gmcp/ ./internal/actions/ -count=1`, `node tools/jstest/chat-hints.test.js`, all jstests. Expected: PASS. Prove the Go test fails with the adjacent loop removed.

- [ ] **Step 7: Commit** `feat: shouts show in the Chat window, near and next door`.

---

### Task 5: Shopkeepers wake for customers

**Files:**
- Create: `internal/usercommands/shop_wake.go`, `internal/usercommands/shop_wake_test.go`
- Modify: `internal/usercommands/go.go:503-525` (after the light block), `list.go:35`, `buy.go:24`, `sell.go:26`, `appraise.go:25`, `offer.go:37`
- Modify: `internal/hooks/NewRound_IdleMobs_schedule.go:106-113`, `internal/hooks/NewRound_IdleMobs_schedule_test.go`
- Modify: `internal/guards/shop_night_trade_guard_test.go:216-268`
- Modify: `_datafiles/config.yaml` (after `MobBTreeReactionBase` at `:2128`)

- [ ] **Step 1: Write the failing test** `internal/usercommands/shop_wake_test.go`:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

const shopWakeSleepId = 9850

// shopScene: Marn (mob instance 851), a shopkeeper asleep in shop room 9851,
// and a customer Kesh (9852).
func shopScene(t *testing.T) (*rooms.Room, *mobs.Mob, *users.UserRecord) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		shopWakeSleepId: {ConditionId: shopWakeSleepId, Name: "Test Sleep", Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	lit := 60
	shop := &rooms.Room{RoomId: 9851, Zone: "ShopZone", Lamp: rooms.LampPtr(lit)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9851: shop},
		map[string]*rooms.ZoneConfig{"ShopZone": {Name: "ShopZone", RoomId: 9851}}))
	marn := &mobs.Mob{MobId: 85, InstanceId: 851, Zone: "ShopZone"}
	marn.Character.Name = "Marn"
	marn.Character.RoomId = 9851
	marn.Character.Shop = characters.Shop{{ItemId: 1, Quantity: 1}}
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{85: {MobId: 85, Zone: "ShopZone"}}, map[int]*mobs.Mob{851: marn}))
	shop.AddMob(851)
	require.True(t, marn.Character.Conditions.AddCondition(shopWakeSleepId, true), "precondition: Marn asleep")
	kesh := users.NewTestUser(9852, "kesh", "Kesh", 99852)
	kesh.Character.RoomId = 9851
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9852: kesh}))
	shop.AddPlayer(9852)
	return shop, marn, kesh
}

// Spec: a customer walking in wakes a sleeping shopkeeper.
func TestEntryWakesASleepingShopkeeper(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	wakeKeepersOnEntry(shop, kesh)
	require.False(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}

// Spec: a hidden player walking in does not.
func TestHiddenEntryLeavesTheShopkeeperAsleep(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	hideForTest(t, kesh)
	wakeKeepersOnEntry(shop, kesh)
	require.True(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}

// Spec: a shop command wakes the keeper and goes on, instead of refusing.
func TestShopCommandWakesTheKeeperAndServes(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	hideForTest(t, kesh)
	require.True(t, shopOpenFor(shop, kesh), "the shop is open once its keeper wakes")
	require.False(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}
```

Add the hide helper to the same file, the calls `phaseHide` uses in `internal/actions/spell_phase_lines_test.go:88-99` (imports `internal/state` and `internal/state/awareness`):

```go
func hideForTest(t *testing.T, u *users.UserRecord) {
	t.Helper()
	c := &u.Character
	c.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "shop_wake_test"}
	if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	c.Awareness.ResolveConcealment(true, reason)
	require.True(t, c.IsHidden(), "fixture is not hidden")
}
```

`shopOpenFor` returns true when the command may go on, so the third test reads `require.True(t, shopOpenFor(shop, kesh))`.

- [ ] **Step 2:** `go test ./internal/usercommands/ -run 'Shopkeeper|ShopCommand' -count=1` Expected: FAIL (undefined).

- [ ] **Step 3: Implement** `internal/usercommands/shop_wake.go`:

```go
package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Shopkeepers wake for customers (owner ruling 2026-11-07). This replaces,
// for shopkeepers only, the 2026-07-30 rule that a sleeping NPC refuses
// rather than wakes (actions.RefuseIfAsleep): a keeper asleep in their own
// shop wakes when a player who is not hidden walks in, and wakes and serves a
// customer's shop command. It lives here, not in actions, because waking must
// relight the keeper's lantern through its item tree, and behaviortree
// imports actions.

// wakeKeeper wakes one sleeping keeper, relights their lantern at once (it
// went dark while they slept), and says so to the room.
func wakeKeeper(m *mobs.Mob, room *rooms.Room, customer *users.UserRecord) {
	m.Character.CancelConditionsWithFlag(conditions.Sleeping)
	mobs.OnSleeperWoken(&m.Character)
	if lamp := m.Character.Equipment.Light; lamp.ItemId > 0 && lamp.HasBehavior() {
		behaviortree.TryItemBehavior(behaviortree.EventContext{EventType: "item_idle"}, behaviortree.ItemSubject{
			UUID: lamp.UUID, ItemId: lamp.ItemId, MobInstanceId: m.InstanceId, Slot: "light"})
	}
	exclude := []int{}
	if customer != nil {
		customer.SendText(messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wakes as you come in.`, m.Character.Name))
		exclude = append(exclude, customer.UserId)
	}
	room.SendText(messaging.CategoryMobEmote,
		fmt.Sprintf(`<ansi fg="mobname">%s</ansi> wakes up.`, m.Character.Name), exclude...)
}

// sleepingKeepers returns the merchants asleep in room.
func sleepingKeepers(room *rooms.Room) []*mobs.Mob {
	var out []*mobs.Mob
	for _, id := range room.GetMobs(rooms.FindMerchant) {
		if m := mobs.GetInstance(id); m != nil && m.Character.HasConditionFlag(conditions.Sleeping) {
			out = append(out, m)
		}
	}
	return out
}

// wakeKeepersOnEntry wakes every sleeping keeper in room when a player who is
// not hidden walks in. A hidden player leaves them asleep (spec).
func wakeKeepersOnEntry(room *rooms.Room, user *users.UserRecord) {
	if room == nil || user == nil || user.Character.IsHidden() {
		return
	}
	for _, m := range sleepingKeepers(room) {
		wakeKeeper(m, room, user)
	}
}

// shopOpenFor wakes a sleeping keeper for a shop command, hidden customer or
// not, then applies the usual closed-shop rules (off shift). It reports
// whether the command may go on, the opposite of actions.RefuseIfShopClosed.
func shopOpenFor(room *rooms.Room, user *users.UserRecord) bool {
	if room != nil && actions.ShopClosedForSleep(room) != nil {
		for _, m := range sleepingKeepers(room) {
			wakeKeeper(m, room, user)
		}
	}
	return !actions.RefuseIfShopClosed(room, user)
}
```

The lamp calls are the ones the night-trade guard makes (`shop_night_trade_guard_test.go:233-235`); `room.SendText(cat, txt, excludeUserIds ...int)` is at `internal/rooms/rooms.go:258`. `wakeKeeper` takes a nil customer from `wake` (Task 6).

- [ ] **Step 4: Call sites.**
  - `go.go`: directly after the light-arrival block (`:503-525`), add `wakeKeepersOnEntry(destRoom, user)`.
  - `list.go:35`, `buy.go:24`, `sell.go:26`, `appraise.go:25`, `offer.go:37`: replace `if actions.RefuseIfShopClosed(room, user) {` with `if !shopOpenFor(room, user) {`. Leave the rest of each block as is. Remove the `actions` import only if the compiler reports it unused.

- [ ] **Step 5: Run** the Step 1 tests. Expected: PASS. Prove each fails with its call removed.

- [ ] **Step 6: No dozing with a customer in the shop.** Add to `NewRound_IdleMobs_schedule_test.go`, using its existing `registerSleepyScheduleForTest` (schedule `sleepy_test`, sleeping at room 1234 at hour 23, as `TestScheduleTick_WantsSleep_AtTargetDuringSleepSegment` uses it):

```go
// Owner 2026-11-07: a shopkeeper does not doze off with a customer (a player
// who is not hidden) in the shop; an empty shop, or a mob with no shop, still
// sleeps on schedule.
func TestScheduleTick_ShopkeeperStaysUpForACustomer(t *testing.T) {
	registerSleepyScheduleForTest(t)
	shop := &rooms.Room{RoomId: 1234, Zone: "SleepyZone"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1234: shop},
		map[string]*rooms.ZoneConfig{"SleepyZone": {Name: "SleepyZone", RoomId: 1234}}))

	keeper := &mobs.Mob{ScheduleId: "sleepy_test"}
	keeper.Character.RoomId = 1234
	keeper.Character.Shop = characters.Shop{{ItemId: 1, Quantity: 1}}
	plain := &mobs.Mob{ScheduleId: "sleepy_test"}
	plain.Character.RoomId = 1234

	if !scheduleTickPlan(keeper, 23).WantsSleep {
		t.Fatal("an empty shop: the keeper should sleep on schedule")
	}

	customer := users.NewTestUser(9861, "customer", "Customer", 99861)
	customer.Character.RoomId = 1234
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9861: customer}))
	shop.AddPlayer(9861)

	if scheduleTickPlan(keeper, 23).WantsSleep {
		t.Error("a customer in the shop: the keeper should stay up")
	}
	if !scheduleTickPlan(plain, 23).WantsSleep {
		t.Error("a mob with no shop sleeps on schedule even with a player present")
	}
}
```

(add `internal/characters`, `internal/rooms` and `internal/users` to that file's imports). Then in `scheduleTickPlan`, change the sleep block to:

```go
	if seg.Activity == "sleeping" && mob.Character.RoomId == seg.TargetRoom && !shopHasCustomer(mob) {
```

and add:

```go
// shopHasCustomer reports whether a shopkeeper has a player who is not
// hidden in the room: a keeper does not doze off with a customer in the shop
// (owner 2026-11-07). Once the room has no such player and the wake grace
// has run, the schedule puts them back to sleep.
func shopHasCustomer(mob *mobs.Mob) bool {
	if !mob.HasShop() {
		return false
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return false
	}
	for _, uid := range room.GetPlayers() {
		if u := users.GetByUserId(uid); u != nil && !u.Character.IsHidden() {
			return true
		}
	}
	return false
}
```

Run `go test ./internal/hooks/ -run Schedule -count=1`. Expected: PASS; prove the new test fails without `!shopHasCustomer(mob)`.

- [ ] **Step 7: The night-trade guard checks woken keepers.** In `shop_night_trade_guard_test.go`, inside the hour loop after the lantern block, add a woken sample for a keeper who sleeps in their own shop:

```go
				// Spec 2026-11-07: a keeper asleep in their own shop wakes for
				// a customer and must be able to trade at once. Wake them as
				// usercommands.wakeKeeper does (Sleeping off, lantern tree run)
				// and ask the sight gate in the shop.
				if asleep && sched != nil {
					if seg := sched.CurrentSegment(hour); seg != nil && seg.TargetRoom == p.room.RoomId {
						inst.Character.Conditions.RemoveCondition(15)
						if lamp := inst.Character.Equipment.Light; lamp.ItemId > 0 && lamp.HasBehavior() {
							behaviortree.TryItemBehavior(behaviortree.EventContext{EventType: "item_idle"}, behaviortree.ItemSubject{
								UUID: lamp.UUID, ItemId: lamp.ItemId, MobInstanceId: inst.InstanceId, Slot: "light"})
						}
						inst.Character.RoomId = p.room.RoomId
						p.room.AddMob(inst.InstanceId)
						if actions.ShopSightRefusal(observer, p.room) {
							if _, ok := dark[p.room.RoomId]; !ok {
								darkOrder = append(darkOrder, p.room.RoomId)
							}
							dark[p.room.RoomId] = append(dark[p.room.RoomId], fmt.Sprintf("%s %02d:00 woken, light %d", d.Name, hour, p.room.LightLevel()))
						}
						p.room.RemoveMob(inst.InstanceId)
						inst.Character.Conditions.AddCondition(15, false)
					}
				}
```

Update the doc comment (lines 52-58) to say a sleeping keeper in their own shop is now checked woken. Run `go test ./internal/guards/ -run TestEveryShopkeeperCanTradeAtNightWhileAwake -count=1`. If it reports keepers, fix the content (each keeper needs its keeper lantern, per the lighting rule), never the guard, and list them in the commit message.

- [ ] **Step 8: Config.** In `config.yaml` after `MobBTreeReactionBase` (`:2128`) add:

```yaml
  ScheduleWakeGraceRounds: 50     # Rounds a woken scheduled mob stays up before its schedule may put it back to sleep (a shopkeeper also stays up while a customer is in the shop)
```

Run `go test ./internal/configs/ -count=1`. Expected: PASS.

- [ ] **Step 9: Commit** by name: `feat: shopkeepers wake for customers and stay up while one is in the shop`.

---

### Task 6: `wake <name>`, and the asleep hint points at it

**Files:**
- Create: `internal/usercommands/wake.go`, `internal/usercommands/wake_test.go`, `_datafiles/world/dogmud/templates/help/wake.template`
- Modify: `internal/usercommands/usercommands.go` (registry), `_datafiles/world/dogmud/keywords.yaml:356` (`noop: ['wake']`), `:39` (help list)
- Modify: `internal/refusal/busy.go:23-24`, `internal/refusal/busy_test.go:27`
- Modify: `internal/actions/divergences.go` (user-only allowlist)

- [ ] **Step 1: Write the failing tests.** `internal/usercommands/wake_test.go`, reusing `shopScene` from Task 5:

```go
package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/stretchr/testify/require"
)

// Spec: wake <name> wakes a sleeping NPC in the room.
func TestWakeByNameWakesTheSleeper(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	_, err := Wake(`marn`, kesh, shop, 0)
	require.NoError(t, err)
	require.False(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}

// Bare wake does nothing more: the dispatcher already woke a sleeping player.
func TestBareWakeIsHarmless(t *testing.T) {
	shop, marn, kesh := shopScene(t)
	_, err := Wake(``, kesh, shop, 0)
	require.NoError(t, err)
	require.True(t, marn.Character.HasConditionFlag(conditions.Sleeping))
}
```

In `internal/refusal/busy_test.go:27` change the expected string to:

```go
	assert.Equal(t, `<ansi fg="mobname">Mira</ansi> is fast asleep. Try <ansi fg="command">wake mira</ansi>.`,
```

- [ ] **Step 2:** run both packages. Expected: FAIL.

- [ ] **Step 3: Implement** `internal/usercommands/wake.go`:

```go
package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Wake wakes a sleeper in the room by name (owner 2026-11-07): `wake marn`.
// Bare `wake` has nothing left to do: a sleeping player is woken by the
// dispatcher before any command but sit and rest runs (keepsSleep).
func Wake(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	name := strings.TrimSpace(rest)
	if name == `` || room == nil {
		return true, nil
	}
	playerId, mobId := room.FindByNameSeenBy(&user.Character, name)
	if mobId > 0 {
		if m := mobs.GetInstance(mobId); m != nil {
			if !m.Character.HasConditionFlag(conditions.Sleeping) {
				user.SendText(messaging.CategoryRefusal, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is not asleep.`, m.Character.Name))
				return true, nil
			}
			if m.HasShop() {
				wakeKeeper(m, room, nil) // relights a keeper's lantern too
			} else {
				m.Character.CancelConditionsWithFlag(conditions.Sleeping)
				mobs.OnSleeperWoken(&m.Character)
			}
			user.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`You wake <ansi fg="mobname">%s</ansi>.`, m.Character.Name))
			room.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes <ansi fg="mobname">%s</ansi>.`, user.Character.Name, m.Character.Name), user.UserId)
			return true, nil
		}
	}
	if playerId > 0 && playerId != user.UserId {
		if other := users.GetByUserId(playerId); other != nil {
			if !other.Character.HasConditionFlag(conditions.Sleeping) {
				user.SendText(messaging.CategoryRefusal, fmt.Sprintf(`<ansi fg="username">%s</ansi> is not asleep.`, other.Character.Name))
				return true, nil
			}
			other.Character.CancelConditionsWithFlag(conditions.Sleeping)
			other.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes you.`, user.Character.Name))
			user.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`You wake <ansi fg="username">%s</ansi>.`, other.Character.Name))
			room.SendText(messaging.CategoryMobEmote, fmt.Sprintf(`<ansi fg="username">%s</ansi> wakes <ansi fg="username">%s</ansi>.`, user.Character.Name, other.Character.Name), user.UserId, other.UserId)
			return true, nil
		}
	}
	refusal.NotHereCreature(user) // "You can't find them here." (internal/refusal/nothere.go:24)
	return true, nil
}
```

(add `internal/refusal` to the imports).

- [ ] **Step 4: Register and wire.**
  - `usercommands.go`: add `` `wake`:            {Wake, false, false, false}, // wake <name> (owner 2026-11-07) `` beside `sleep`.
  - `keywords.yaml:356`: delete `  noop:               ['wake']`.
  - `keywords.yaml:39`: add `      - wake` after `      - sleep`.
  - `divergences.go` user-only list, player mechanics: `"wake":       "player-mechanic: waking a sleeper by name; NPCs wake others by shouting",`.
  - `busy.go:24`:

```go
	return `<ansi fg="mobname">` + name + `</ansi> is fast asleep.` + hint(`wake `+strings.ToLower(name))
```

  (add `strings` to the imports). Update the `RefuseIfAsleep` comment in `internal/actions/sleeping_target.go:31-37` to name `wake <name>` as the way in, and that shopkeepers now wake for customers (`usercommands/shop_wake.go`).
  - Create `templates/help/wake.template`:

```
<ansi fg="black-bold">.:</ansi> <ansi fg="magenta">Help for </ansi><ansi fg="command">wake</ansi>

Wake someone who is asleep in the same room as you.

<ansi fg="yellow">Usage: </ansi>

  <ansi fg="command">wake marn</ansi>

A sleeping shopkeeper wakes on their own when you walk in, so you only need
this for other sleepers. <ansi fg="command">shout</ansi> wakes everyone in the room at once.
```

- [ ] **Step 5: Run** `go test ./internal/usercommands/ ./internal/refusal/ ./internal/actions/ -count=1` and `go test ./internal/guards/ -run CommandParity -count=1`. Expected: PASS. Prove `TestWakeByNameWakesTheSleeper` fails with the mob branch removed.

- [ ] **Step 6: Commit** `feat: wake <name>, and the asleep hint points at it`.

---

### Task 7: Docs, gates, boot, PR

- [ ] **Step 1: Docs.**
  - `docs/PATCH_NOTES.md`, a new top entry `## 2026-11-07: Waking up` (player words, no numbers):

```markdown
## 2026-11-07: Waking up

- Each Chat channel says what it is for until its first message arrives.
  Rest the mouse on a channel's button to read it again.
- Whispers you send now show in your own Whisper tab.
- Shouts show in the Chat tab, from your room and from next door.
- The Broadcasts tab is for game tips and announcements from the staff.
  To talk to everyone, use `chat`, or its shortcut `` ` ``.
- Shopkeepers wake up when you walk in, and stay up while you shop.
- Type `wake` and a name to wake someone who is asleep.
```

  - `docs/README.md`: rows for the spec `superpowers/specs/2026-11-07-comms-tabs-and-sleeping-shopkeepers-design.md`, this plan, `tools/jstest/chat-hints.test.js`, `internal/usercommands/shop_wake.go`, `internal/usercommands/wake.go`, `modules/gmcp/gmcp.Comm_whisper_test.go`, `modules/gmcp/gmcp.Comm_shout_test.go`, and the new admin help template, each one line on what it holds.
  - `modules/gmcp/context.md`: whisper echo, shout lane (`shoutDeliveries`), announcements on the broadcast lane.
  - `internal/usercommands/context.md`: `broadcast` admin-only, `wake`, `shop_wake.go`.
  - `internal/hooks/context.md`: tips reach deafened players; `shopHasCustomer`.
  - Spec: set its status line to "built" in the PR, after merge move to `specs/completed/` at EOD.

- [ ] **Step 2: Gates.** `gofmt -l internal/ modules/` (empty); `go build ./...`; `go test` for `internal/usercommands internal/hooks internal/actions internal/refusal internal/events internal/guards internal/configs modules/gmcp` (all PASS); every jstest and webclient-test; `~/go/bin/golangci-lint run --new-from-merge-base=origin/master` (0 issues); `python -I tools/context_md_audit.py` no worse than master.

- [ ] **Step 3: Boot check** per `dogmud-shipping` (detached worktree, private ports, `Server Ready`, no panic). Then, on the same boot, log in two test characters and check by hand: a shout shows in the other's Chat tab; `broadcast` is refused for the player and works for an admin account; `wake` on a sleeping NPC; a shopkeeper at night wakes on entry. Report what was checked.

- [ ] **Step 4: PR.** Push the branch; `gh pr create --repo pruuk/DOGMud --base master --head feat/comms-tabs-and-sleeping-shopkeepers` with a body listing the rulings, the evidence and the checks; watch both workflows; merge only when Calabe says so. Comment on #808 that the channel descriptions shipped.

---

## Self-review

- Spec coverage: hints and tooltips (Task 1), whisper echo (done, `3eed75cd1`; docs Task 7), announcement and tips (Task 3), shout in Chat (Task 4), shopkeepers wake on entry, on a shop command, stay up with a customer, can trade woken (Task 5), `wake <name>` and the hint (Task 6), the answered questions (admin name in the label: Task 3; no dozing with a customer: Task 5 Step 6). Light visible and the icon shipped in #813.
- The NPC `broadcast` command (unused by content) is removed (Calabe 2026-11-07, Task 3 Step 5b).
- Names used across tasks: `wakeKeeper`, `sleepingKeepers`, `wakeKeepersOnEntry`, `shopOpenFor`, `shopHasCustomer`, `commSpeakerRoom`, `shoutDeliveries`, `DrainQueuedCommunicationsForTest`, `DrainQueuedBroadcastsForTest`, `Wake`.
