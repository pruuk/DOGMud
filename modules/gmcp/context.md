# GMCP Module Context

## Overview

The `modules/gmcp` package implements the GMCP (Generic Mud Communication
Protocol) sub-negotiation layer that pushes structured JSON payloads from
the server to GMCP-aware clients (web client, Mudlet, etc.). Each
`gmcp.<Name>.go` file is a self-contained module: it registers event
listeners in its `init()` function and emits `GMCPOut` events to deliver
payloads to connected clients. All modules follow the same
listener/payload pattern; this document covers the `Char.Automation`
module (Phase 1–4) and the `Char.Vitals` additions from Phase 3.

## Char.Automation Module (`gmcp.Automation.go`)

### Purpose

Keeps the web-client automation panel in sync with the player's
server-side macro and alias data. On any change the module pushes a
fresh `Char.Automation` payload over GMCP so the panel reflects the
current state without a full page reload.

### Outbound Payload

Module name (as sent over GMCP): **`Char.Automation`**

```json
{
  "macros":    [{ "key": "=1", "commands": "wave;say hi" }, ...],
  "aliases":   [{ "name": "ms", "command": "cast mind-spike" }, ...],
  "ticks":     [{ "id": "abc123", "name": "My Timer", "commands": "forage;rest",
                  "intervalSec": 30, "enabled": true }, ...],
  "triggers":  [{ "id": "def456", "name": "Auto-heal", "pattern": "* hits you*",
                  "commands": "cast heal", "enabled": true,
                  "queueMode": "back",
                  "condition": {
                    "sourceKind": "pool_pct",
                    "sourceKey":  "hp",
                    "op":         "below",
                    "values":     ["40"]
                  },
                  "thenCommands": "cast heal",
                  "elseCommands": "" }, ...]
}
```

All arrays are sorted by key/name/id for stable ordering. Fields:

| Field                       | Type     | Description                                        |
|-----------------------------|----------|----------------------------------------------------|
| `macros[].key`              | string   | Macro slot identifier (e.g. `"=1"`)               |
| `macros[].commands`         | string   | Semicolon-delimited command string                 |
| `aliases[].name`            | string   | Alias name (e.g. `"ms"`)                          |
| `aliases[].command`         | string   | Expanded command string                            |
| `ticks[].id`                | string   | Unique identifier for the tick                     |
| `ticks[].name`              | string   | Human-readable label shown in the panel            |
| `ticks[].commands`          | string   | Semicolon-delimited command string                 |
| `ticks[].intervalSec`       | int      | Fire interval in seconds (minimum 1)               |
| `ticks[].enabled`           | bool     | Whether the tick is currently active               |
| `triggers[].id`             | string   | Unique identifier for the trigger                  |
| `triggers[].name`           | string   | Human-readable label shown in the panel            |
| `triggers[].pattern`        | string   | Wildcard pattern; `*` captures into `$1`, `$2`, … |
| `triggers[].commands`       | string   | Commands run when pattern matches (no condition)   |
| `triggers[].enabled`        | bool     | Whether the trigger is currently active            |
| `triggers[].condition`      | object?  | Optional condition; `null` when not set            |
| `triggers[].thenCommands`   | string   | Commands run when condition is true (may be empty) |
| `triggers[].elseCommands`   | string   | Commands run when condition is false (may be empty)|
| `triggers[].queueMode`      | string   | `""` (fire immediately, default), `"back"` (add to end of action queue), or `"front"` (add to front — priority) |

**`triggers[].condition` shape** (when present):

| Field        | Type     | Values / notes                                          |
|--------------|----------|---------------------------------------------------------|
| `sourceKind` | string   | `"pool_pct"` / `"conditions"` / `"capture"` / `"target"` / `"cooldown"` |
| `sourceKey`  | string   | Disambiguator: pool name (`"hp"`, `"sp"`, `"cp"`), capture index (`"$1"`), ability name, or empty for others |
| `op`         | string   | Operator string (see table below)                       |
| `values`     | []string | Operand(s); single-element for most sources, multi for target list |

Operators by `sourceKind`:

Read off `evalTriggerCondition` in `webclient-pure.html`, which is the only
thing that interprets these; `users.TriggerCondition`'s docstring is looser
than the code.

| `sourceKind`  | `op` values                          | `values` element(s)             |
|---------------|--------------------------------------|---------------------------------|
| `pool`        | `below`, `above`, anything else means equals | `["40"]` (percent, 0 to 100); `sourceKey` is `hp`, `sp` or `cp` |
| `status`      | `exclude`, anything else means include | `["poisoned"]`, matched case-insensitively against each `Char.Conditions` map KEY with any `#n` suffix stripped (not the entry's `name`, which can carry a stack count) |
| `capture`     | `contains`, anything else means equals | `["some text"]`; `sourceKey` is `$1`, `$2`, ... |
| `target`      | `notoneof`, anything else means oneof | `["troll","orc","goblin"]`, matched against the engaged enemy's name |
| `cooldown`    | `notready`, anything else means ready | ignored; the check is the one shared `special-move` tag |

When a condition is present, `commands` is ignored; `thenCommands` and
`elseCommands` drive behaviour. An empty string for either branch means
"do nothing" for that outcome.

**Available-pool-% note:** pool percentages for the `pool_pct` source
are computed against the player's usable (unreserved) pool, not the
raw total. The client derives this from `Char.Vitals` fields:
`hp_reserved`, `stamina_reserved`, `conviction_reserved` (see
`Char.Vitals` section below).

### Push Triggers

The module registers two event listeners in `init()`:

| Event                        | When it fires                                         |
|------------------------------|-------------------------------------------------------|
| `events.PlayerSpawn{}`       | Player logs in — sends the full current state         |
| `events.AutomationChanged{}` | Any macro, alias, tick, or trigger is changed/removed |

Both listeners call `sendAutomation(userId)`, which:
1. Looks up the `UserRecord` for the given `UserId`.
2. Skips silently if GMCP is not enabled for that connection
   (`isGMCPEnabled(connectionId)`).
3. Calls `buildAutomationPayload(...)` to assemble a sorted, stable
   payload including macros, aliases, ticks, and triggers.
4. Enqueues a `GMCPOut{UserId, Module: "Char.Automation", Payload: ...}`
   event for delivery.

### Event Emitters

`events.AutomationChanged{UserId}` is emitted by:

- `internal/usercommands/set.go` — `cmdSetMacro`: after any `set =#`
  or `set =# command` that adds, updates, or clears a macro slot.
- `internal/usercommands/alias.go` — `Alias`: after any
  `alias name=value` or `alias name=` that creates, updates, or
  removes a custom alias.

### Inbound GMCP

Ticks and triggers are managed exclusively through the web panel via
inbound GMCP messages — there is no typed command for either. The web
client sends binary GMCP frames to `gmcp.go`'s `HandleIAC` switch:

| Inbound message          | `kind` gate  | Action                                          |
|--------------------------|--------------|-------------------------------------------------|
| `Char.Automation.Set`    | `"tick"`     | Create or update a tick on the `UserRecord`     |
| `Char.Automation.Remove` | `"tick"`     | Delete a tick by `id` from the `UserRecord`     |
| `Char.Automation.Set`    | `"trigger"`  | Create or update a trigger on the `UserRecord`  |
| `Char.Automation.Remove` | `"trigger"`  | Delete a trigger by `id` from the `UserRecord`  |

`Char.Automation.Set` payload shape for a **tick**:
```json
{ "kind": "tick", "id": "abc123", "name": "My Timer",
  "commands": "forage;rest", "intervalSec": 30, "enabled": true }
```

`Char.Automation.Set` payload shape for a **trigger**:
```json
{ "kind": "trigger", "id": "def456", "name": "Auto-heal",
  "pattern": "* hits you*", "commands": "cast heal", "enabled": true,
  "queueMode": "front",
  "condition": { "sourceKind": "pool_pct", "sourceKey": "hp",
                 "op": "below", "values": ["40"] },
  "thenCommands": "cast heal", "elseCommands": "" }
```

`Char.Automation.Remove` payload shape (same for tick and trigger):
```json
{ "kind": "trigger", "id": "def456" }
```

After processing, either handler emits `events.AutomationChanged{UserId}`
to re-push the full `Char.Automation` payload to the client. Unknown
`kind` values are silently ignored.

See `docs/superpowers/specs/completed/2026-06-07-web-client-automation-panel-design.md`
for the full Phase 2–3 design.

## Char.Automation — Action Queue (Phase 4)

### Purpose

Phase 4 added a client-side FIFO action queue so that multiple triggers
firing at once (e.g. several conditions expiring simultaneously) do not stomp
each other. Instead of sending all commands immediately, each trigger with
a non-empty `queueMode` pushes its resolved command onto the queue, which
drains one entry per shared ability cooldown.

### `queueMode` field

The `queueMode` field is included on every trigger in both the outbound
`Char.Automation` payload and the inbound `Char.Automation.Set` message.

| Value      | Behaviour                                                         |
|------------|-------------------------------------------------------------------|
| `""`       | Fire immediately (default, backward-compatible)                   |
| `"back"`   | Append to the end of the action queue                             |
| `"front"`  | Prepend to the front of the queue (priority); multiple concurrent |
|            | `"front"` entries preserve their arrival order (priority FIFO)   |

### Client-side queue behaviour

The queue is implemented entirely in the web client (not on the server).
Key rules:

- **Shared cooldown gate.** Kick, trip, bash, grapple, taunt, rally,
  warcry, and every spell share one cooldown tracked as
  `Commands.State.cooldowns["special-move"]`. The queue drains one entry
  only when that cooldown is free.
- **Dedup.** A trigger whose resolved command is already present in the
  queue will not add a second entry.
- **Cap.** The queue holds at most 10 entries.
- **Ephemeral.** The queue is cleared on: the Clear button in the panel,
  `Commands.State.mode == "downed"` (death), and any page reload or
  reconnect. It is never persisted to the server.
- **No auto-retry.** If a queued command fails (e.g. concentration lost),
  the queue does not re-add it. Authors should write a companion trigger
  that matches the failure message and re-queues the command.

### Panel UI

Triggers that are currently in the queue are highlighted and floated to
the top of the Triggers tab. A badge on each shows its position (1 =
next to fire).

## Char.Vitals — Reserved-Pool Fields (Phase 3)

### Purpose

The `Char.Vitals` payload was extended in Phase 3 to expose the
**reserved** portion of each resource pool. The web client uses these
values to compute a player's _usable_ pool for trigger condition
evaluation: pool-percent conditions compare against the unreserved
fraction, not the raw total.

### Additional Fields

These three fields were appended to the existing `Char.Vitals` payload
(`gmcp.Char.go`) in Phase 3:

| Field               | Type | Description                                        |
|---------------------|------|----------------------------------------------------|
| `hp_reserved`       | int  | HP currently set aside and unavailable for use     |
| `stamina_reserved`  | int  | Stamina currently set aside and unavailable         |
| `conviction_reserved` | int | Conviction currently set aside and unavailable    |

### No field in Char.Vitals carries `omitempty`

`Char.Vitals` is a full-state **snapshot**, republished whenever a pool moves,
and every field is always sent. This is load-bearing, not tidiness.

Under `omitempty` a value of exactly zero dropped the key from the JSON
entirely, and a client that merges each payload over the previous one (the
normal Mudlet idiom) then kept rendering the last non-zero reading. A U7
playtest caught it live: the payload arrived as
`{"hp":118,"hp_max":437,"stamina_max":429,...}` with no `stamina` key at all
while stamina was genuinely exhausted, so the client's stamina bar stayed put
at the exact moment the number mattered most.

The three `*_reserved` fields follow the same rule even though a zero there
means "no reservation" rather than "an empty pool": a reservation that **ends**
must be observable, and omitting the key transmits that transition as silence,
leaving a stale reserved band drawn on the bar.

`TestCharVitals_ZeroPoolsAreSentAsZero` (`gmcp.Vitals_test.go`) asserts key
PRESENCE, not value, and fails if `omitempty` is reintroduced on any field.
Do not add it back.

### Client Usage

The client trigger engine reads these fields from each incoming
`Char.Vitals` event and caches them alongside the raw pool values.
When evaluating a `pool_pct` condition the available (usable) value is:

```
usable     = current - reserved
usable_max = max - reserved
usable_pct = usable / usable_max   (clamped 0–1)
```

For example, if `hp = 120`, `hp_max = 200`, and `hp_reserved = 100`,
then `usable = 20`, `usable_max = 100`, and `usable_pct = 20%`. A
condition of "HP below 30%" would therefore be true in this state, even
though raw HP (60%) is well above 30%.

## Char.Conditions: the one timed-state list (conditions unification, 2026-09-12)

`Char.Conditions` (`gmcp.Char.go`) is a map keyed by each held record's plain
spec name, one entry per listed record, built by `buildConditionsPayload(ch)`. There used to be TWO payloads that
overlapped: `Char.Affects`, built from the conditions collection, and an older `Char.Conditions`
list built from the combat condition slice. Those condition records ARE the conditions
now, so there is one payload, carrying the whole `Char.Affects` shape plus the
qualitative `duration` word the old list contributed.

`GMCPCondition`, one entry:

| JSON key | Go field | What it carries |
|---|---|---|
| `name` | `Name` | `conditions.DisplayName`: the spec name, plus the live stack count above one ("Bleeding (3)") |
| `description` | `Description` | its description |
| `duration_max` | `DurationMax` | total duration in SECONDS (`RoundsToSeconds`), or -1 for a permanent condition |
| `duration_cur` | `DurationLeft` | remaining duration in seconds, or -1 for a permanent condition |
| `duration` | `Duration` | the qualitative word: `sustained`, `briefly`, `for a while`, `extended` |
| `type` | `Type` | the INSTANCE's `Condition.Source` (the applier's own string: "spell", "heal spell", "prone recovery", ...), falling back to `unknown` |
| `affects` | `Mods` | the spec's `StatMods`, name to int |

Things worth knowing before touching it:

- **`type` is the source, not a category.** It is whatever the applier passed
  as `source`, so it is stable per producer and not an enum. This is what
  `Char.Affects` always put there; the retired `Char.Conditions` list put
  `ConditionType.DisplayName()` there instead, so a client that switched on
  `type` to identify a condition must stop.
- **`duration_cur` is read off the INSTANCE; `duration_max` is not.**
  `conditions.GetDurations` derives the remaining rounds from the held record's
  `TriggersLeft`, not the spec's authored `triggercount`. Reading the spec
  default is what made a 50-round Enchant Withdrawal on a 10-trigger record
  report a negative duration. The total it returns alongside is still the
  spec's `triggercount × round interval`, floored at the instance's remaining
  rounds (`max(totalRounds, roundsLeft)`) so that same 50-round withdrawal
  reports a total of 50 rather than 10.
- **A repeated name gets a `#n` suffix**, because the map is keyed by name and
  two records can present the same spec name. `nameIncrement` is ONE
  counter for the whole payload, not per name: the first collision anywhere
  takes `#1`, the next `#2`, whatever names they were.
- **Hidden and secret records are left out**, by `ConditionSpec.Listed`, the same
  predicate the `conditions` command uses (owner ruling 2026-09-14): being
  told you are hidden is a tell you should not get, and a secret record is
  bookkeeping the player is not meant to see. The map is built by
  `buildConditionsPayload(ch)`. The key is the plain spec name.
- **Client triggers match the KEY, not `name`.** A stacking record's `name`
  carries its live count ("Bleeding (2)"), so `evalTriggerCondition` in
  `webclient-pure.html` strips any `#n` suffix from each map key and compares
  that, case-insensitively. `tools/webclient-tests/trigger-status-key-match.js`
  pins it.
- **Push triggers.** It ships with `Char.Vitals` on `CharacterVitalsChanged`
  (`vitalsChangedHandler`), so any pool move republishes it, and on its own
  from `conditionTriggeredHandler` on `events.ConditionsTriggered`.
- **`Char.Affects` is retired.** There is no builder for it. A client that
  requests it falls through to the bottom of `GetCharNode`
  (`gmcp.Char.go`) and the server logs `Bad module requested`, exactly as for
  any unknown module. Two places in the web client read `Char.Conditions`:
  `updateStatusPanel` renders the chips, and `evalTriggerCondition` reads the
  same map to gate a `status`-kind trigger by condition name.

## Char.Enemies: sight-gated (M4d PR 2, 2026-09-20)

`Char.Enemies` (`gmcp.Char.go`) lists every mob fighting in the viewer's
room (`roomInfo.GetMobs(rooms.FindFighting)`), one `GMCPCharModule_Enemy`
per mob:

| JSON key | Go field | What it carries |
|---|---|---|
| `id` | `Id` | `mob.ShorthandId()` |
| `name` | `Name` | the mob's name, or `an unseen foe` (see below) |
| `hp` | `Hp` | current HP, or `0` when unseen |
| `hp_max` | `MaxHp` | max HP, or `0` when unseen |
| `engaged` | `Engaged` | true for the viewer's current combat target |

**Owner ruling 4 (2026-09-20): this payload rides the same sight gate as
the fight prompt's `{target}` token** —
`messaging.CanSeeClearly(user.Character, roomInfo)`, the same predicate
`userrecord.prompt.go`'s `canSeeTargetForPrompt` reads. Before this ruling,
GMCP undid the whole darkness effort: room text read "Something slashes
you!" while a modern client's enemy panel showed the mob by exact name with
a live HP bar. `canSee` is computed ONCE per call (it depends only on the
viewer and room, not on which mob), not once per mob in the loop.

When `canSee` is false, the ROW STAYS (a scripted client still needs to
know the fight is ongoing) but identity and current numbers are stripped,
matching what the prompt withholds:

- `Name` becomes the literal `an unseen foe`.
- `Hp` and `MaxHp` are both set to **`0`**, not omitted.

**Zeroed, not omitted, is deliberate.** `TestCharVitals_ZeroPoolsAreSentAsZero`
(`gmcp.Vitals_test.go`) documents the real incident this pattern exists to
avoid: a client that merges each payload over the last one kept rendering a
stale prior HP reading when a field simply dropped out of the JSON.
Omitting `Hp`/`MaxHp` here would reopen that same hole from the enemy
panel — a caching client would keep showing the last SIGHTED reading
straight through a blind stretch, which is a worse leak than a uniform
`0`.

**This is a binary gate, not a three-verdict one.** Unlike combat's
narration, which reads the three-verdict `SightDecision` and treats
`SightShapes` as its own tier (`internal/combat/context.md`),
`Char.Enemies` has no "a figure" middle ground: it matches the prompt's
existing binary behaviour (`CanSeeClearly`).

## Item names are per-viewer (finder-only baubles, owner ruling 2026-09-29)

Item names in the backpack, the bandolier, the component bag and
`Room.Info.Contents.Items` are the recipient's own view
(`items.Item.NameFor(user.UserId)`), so a finder reads their own
finder-only bauble; every other payload uses the viewer-agnostic name.

## Char.Sight: the player's light band (lighting plan 5d, 2026-09-30)

`Char.Sight` is one field, `{"band": "dark" | "shapes" | "faces" |
"dazzled"}` (`GMCPCharModule_Payload_Sight`), built in `GetCharNode` from
`messaging.LightBand` for the player in their current room (`sightBand`; an
unloaded room reads faces, passed as an untyped nil) and carried in the full
`Char` payload. The web client tints the Game window's border by it
(`webclient-pure.html`, `"Char.Sight"` handler; `dashboard.css`
`#panel-feed.sight-*`; `tools/webclient-tests/sight-border.js`).

- **Push trigger.** `sightBandChangedHandler` answers
  `events.SightBandChanged`, which `internal/lightnotice.Check` queues only
  when the band changes: the 3d notice cadence (every command, combat round,
  move, login, and the light commands). Not on `Char.Vitals`, which rides
  every pool change and would recompute the room's light on each.
- **No room contents.** The band is the player's own, and the text already
  tells them how well they see.

## Room.Info and Comm say follow sight (sight gates close-out, #252, 2026-10-08)

`GetRoomNode` (`gmcp.Room.go`) reads `messaging.ParticipantSight(user.Character, room)` once per
call, for every lane including the sub-module requests:

- **SightNone:** `name` and `description` are empty; `exits` and `exitsv2` are empty maps;
  `Contents.Items`, `Containers`, `Players` and `Npcs` are empty lists. Empty, not omitted, the
  Char.Enemies rule, so a merging client drops the last sighted reading. The room id, `area`,
  `environment`, `symbol` and `coords` stay: they place the player on a map they hold.
- **SightShapes:** the room reads, but every occupant is `unseenOccupant`: name `a figure`
  (`messaging.UnseenNoun`), empty `id` and `adjectives`, `aggro` kept.
- **Rosters and containers follow the text lane:** players and NPCs are listed only when
  `Character.Perceives` them (the `rooms.GetDetails` rule), and a hidden container the viewer has
  not discovered (`HasDiscovery`) is not listed, as in `look`.
- **Re-push on light:** `sightBandChangedHandler` (`gmcp.Room.go`) answers
  `events.SightBandChanged` with Room.Info, and, when the player now makes out the room,
  `actions.MarkRoomMappedIfSeen` plus a `GMCPZoneUpdate`. `gmcp.Char.go` answers the same event
  with Char.Sight; both listeners run.
- **Map (owner ruling R5):** `go.go` maps a room only through `actions.MarkRoomMappedIfSeen`, so
  a room walked in the dark stays off `Zone.Map` until seen; rooms already mapped stay.

`Comm.Channel` say goes through `sayDeliveries` (`gmcp.Comm.go`), one payload per listener,
matching the text lane (`actions.sendSpoken`): `sender` is the name at clear sight, `A figure`
at shapes, `Someone` at none or when `events.Communication.SpeakerHidden`; a player speaker's
words skip deafened listeners, an NPC's do not (ruling 6); a say with `TargetUserId` reaches
only that player. Other channels are unchanged.

## Module index

Every `gmcp.<Name>.go` file follows the same shape: register in `init()`, emit
via `GMCPOut`. `gmcp.go` holds the shared plumbing. Read the file for its exact
payload schema and push triggers — this table is the map, not the schema.

| File | Package | What it carries |
|------|---------|-----------------|
| `gmcp.go` | — | Module registration, `GMCPOut`, shared helpers |
| `gmcp.Char.go` | `Char` | Vitals, stats, status — the core player feed |
| `gmcp.Automation.go` | `Char.Automation` | Triggers, macros, aliases, the action queue |
| `gmcp.Room.go` | `Room` | Current room: description, exits, occupants |
| `gmcp.Zone.go` | `Zone` | The `Zone.Map` snapshot the web mapper draws, plus `party` |
| `gmcp.Party.go` | `Party` | Party roster and member vitals |
| `gmcp.Comm.go` | `Comm` | Channel messages |
| `gmcp.Item.go` / `gmcp.Item_refs.go` | `Char.Items` | Inventory and equipment, plus item reference data |
| `gmcp.Quest.go` / `gmcp.Quest_refs.go` | `Quest` | Quest tracker state and quest reference data |
| `gmcp.Mob.go` | `Mob` | Mob presence and vitals for the room panel |
| `gmcp.Mutation.go` | `Mutation` | Mutation roster and reveal events |
| `gmcp.Dialogue.go` | `Dialogue` | NPC conversation state for the client |
| `gmcp.Action.go` | `Action` | Clickable action affordances |
| `gmcp.Commands.go` | `Commands` | Command state, modes, cooldowns |
| `gmcp.Behavior.go` | `Behavior` | Behaviour-tree state (admin/builder tooling) |
| `gmcp.Build.go` | `Build` | The admin web building tools' data feed |
| `gmcp.CharOp.go` | — | `GMCPCharOp` + `handleCharOp`: inbound state-touching `Char.*` ops, deferred to MainWorker |
| `gmcp.Relay.go` | `Companion.Relay.*` | The companion key relay: installs `companionai.SetRelaySender`, forwards inbound relay messages to `companionai.RelayInbound` |
| `gmcp.Game.go` | `Game` | Game-level metadata |
| `gmcp.World.go` | `World` | World-level state (weather, time) |
| `gmcp.Mudlet.go` | `Client.GUI` | Mudlet-specific package download support |

The `_refs` files exist because reference data (item and quest definitions) is
large and static: it is pushed once rather than on every update, and the live
payloads carry ids that index into it.

## Gotchas

- **The web client is the primary consumer of every one of these.** A payload
  change is a client change; check `_datafiles/html/public/static/js/gmcp.js`
  before altering a field name.
- **`Zone.Map` is fog-of-war filtered** by `Character.VisitedRooms` before it is
  sent; see `internal/mapper`'s `Snapshot`. A room joins `VisitedRooms` only
  when the player makes something out there (`actions.MarkRoomMappedIfSeen`,
  #252 R5). `gmcp.Room.go`'s `sightBandChangedHandler` answers
  `events.SightBandChanged` too: it maps the room if now seen, re-sends the
  map, and re-sends `Room.Info`, so lighting a torch in place refreshes both.
  `zoneMapVisited` (`gmcp.Zone.go`) builds the snapshot's room set and adds
  the current room only when the player's `ParticipantSight` is not
  `SightNone`, so a login in the dark does not name the room (#382 playtest).
- **Emit only on change where you can.** These push on room change and round
  boundaries; adding an unconditional per-round emit to a large payload is a
  bandwidth regression that will not show up in a local test.
- **`HandleIAC` runs on the per-connection goroutine, not MainWorker.** Any
  inbound handler that reads or writes shared game state (`rooms.*`, `mobs.*`,
  `u.Character`, the room manager) races the world tick and can trigger Go's
  uncatchable `fatal error: concurrent map read and map write`. Do not do that
  work inline: queue an event and handle it on MainWorker, as `GMCPBuildOp` /
  `handleBuildOp` (`gmcp.Build.go`) and `GMCPCharOp` / `handleCharOp`
  (`gmcp.CharOp.go`) both do. **Copy the payload** when you queue it — it
  aliases the IAC read buffer, which is reused once `HandleIAC` returns.
- **`Companion.Relay.*` is the one inbound family handled inline.**
  `Companion.Relay.Response`, `.Ready` and `.Gone` touch no game state, so
  `relayInbound` (`gmcp.Relay.go`) resolves the connection's user and hands a
  COPY of the payload to `companionai.RelayInbound` on the connection
  goroutine; the aicompanion module's pending-request table has its own lock.
  Outbound, `relaySend` returns false for a user with no connection or one
  that has not finished GMCP negotiation, because `dispatchGMCP` would drop
  the message silently. Web socket clients are negotiated on connect
  (`onNetConnect`), and `Core.Supports.Set` gates nothing: `EnabledModules`
  is recorded but never read by `dispatchGMCP`.
- **An inbound web socket frame is capped at 64 KiB** (`wsMaxMessageBytes`
  in `internal/web/web.go`); a bigger one closes the player's connection.
  Anything a client sends back, such as a relay reply, must stay under it.

## Room.Info.Contents.Items leaves out fixtures (lighting 5e)

The GMCP twin of "On the Ground" skips fixtures (`items.Item.IsFixture`,
owner ruling R9): a fixture is part of the room, shown in its look.
