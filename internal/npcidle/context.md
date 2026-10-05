# npcidle Context

## Purpose

The engine side of generated idle moments. Now and then, when an NPC is
about to run one of its set idle lines (an `emote` or a `say`), a model
writes it a fresh one from its own description, the room's, and who and what
is there, and the NPC acts that instead. The model side is
`modules/npcidle`, which installs the `Generator`; with none installed
nothing here does anything and every idle line is the set one.

It only ever runs on a player's own key (the companion's key relay), for a
player in the room who left "Make the world livelier" ticked and sees the
NPC clearly. It is the first lively feature (`apiframework.PurposeNPCIdle`).

## Files

- **npcidle.go**: `Generator` (`Chance`, `Reserve`, `Generate`),
  `SetGenerator`, `Result` (`Kind`, `Text`, `KeyholderOnly`),
  `MaxGenerateTime`, `IsFlavor`, `Excluded`, `TryReplace`, `Pending`; the
  delivery (`startDelivery`, `run`, `finish`) and the test seams `roll`,
  `shuffle`, `canSee`.
- **request.go**: `Request`, `NPC`, `Place`, `Snapshot` (the request for a
  mob in a room), `sendToKeyholder`, `seesClearly`.
- **reply.go**: `KindEmote`, `KindSay`, `ReplySchemaName` (`npc_idle`),
  `ReplySchema`, `ParseReply`, `CleanResult`, `ErrUnusable`, `MinTextRunes`,
  `MaxTextRunes`.

## Where it is called

`TryReplace(mob, cmd)` is called, under the mud lock, by every path that
runs an NPC's idle flavor, with the set line it was about to run:

- `hooks.HandleIdleMobs` (the legacy idle: `Mob.GetIdleCommand`, which also
  serves schedules, since a schedule swaps the mob's `IdleCommands`);
- `behaviortree` `actSay` and `actEmote`, through `idleOrSet`, ONLY when the
  tree's event is `mob_idle` (greetings, answers and every other event keep
  their set line);
- `behaviortree` `scavengerIdle` (a scavenger lingering).

When it returns true the caller does not run the set line. Room-tree
scripted lines (`actMobSay`, `actMobEmote`) and gossip are never replaced.

## How a moment goes

1. `IsFlavor`: only a single `emote` or `say` with text. Empty slots,
   `lookfortrouble`, wandering, casting and compound commands are left alone.
2. The chance roll (`Generator.Chance`, percent).
3. `Excluded`: AI companions (bonded, any instance of a companion's template
   via `companionai.DrivesBonded`, or waiting in the Hollow, `mobs.HollowGroup`)
   and charmed mobs. Dormant mobs (dead, fighting, asleep) are skipped.
4. One moment per NPC at a time (`pending`).
5. A keyholder: the players in the room, shuffled, the first who SEES THE
   ROOM CLEARLY (`messaging.CanSeeClearly`: light, blindness, sleep) and
   whose key the generator lets be used now (`Generator.Reserve`, which
   takes their turn). None: the set line runs as normal, under its own sight
   rules.
6. `Snapshot` (authored text only; players counted, never named; floor items
   by `items.Item.ModelName`), then a delivery goroutine calls `Generate`
   off the lock (capped at `MaxGenerateTime`), runs `CleanResult`, takes the
   mud lock and `finish`es.
7. `finish`: if the mob is gone, left the room, or is now dead, fighting or
   asleep, nothing. On any error the SET LINE runs. Otherwise the mob runs
   its own `emote` or `say` command with the moment, so every reader sees or
   hears it under exactly the rules a set line follows (`actions.SendSeen`
   for an emote: nothing in the dark, "a figure" at shapes; the heard path
   for a say). A `KeyholderOnly` moment (the server could not moderate it)
   goes only to its keyholder through `sendToKeyholder`: the room's own
   sight-gated sender (`Room.SendTextVisualHidingNames`) with every other
   player excluded, judged at that moment, and a say in this mode is shown as
   a seen mutter (`actions.Say` owns the say line, and would reach the whole
   room). Nobody else sees anything.

## Gotchas

- **Light.** Do not send a moment by any path that skips the sight rules,
  and never queue a raw `events.Message` or format a say line here (the
  repo's `raw_events_message_guard_test.go` and `speech_wrapper_guard_test.go`
  fail it).
  `TestAPlayerWhoCannotSeeWritesNothing` and
  `TestAKeyholdersMomentFollowsTheLightRules` fail when the gate is removed.
- **No player data.** Nothing a player typed or is called goes into a
  `Request`; `TestTheRequestCarriesTheNPCAndRoomButNoPlayer` holds it.
- **Output is untrusted.** A player's own provider wrote it. `CleanResult`
  keeps it to printable ASCII without markup, one line of 8 to 240
  characters; an emote may not start with the NPC's name (stripped), be one
  word (an emote alias) or say "you". Dashes become commas.
- **The relay's schema list.** `ReplySchemaName` must be listed in
  `LIVELY_SCHEMAS` in `modules/aicompanion/relayweb/relay.js`
  (`modules/npcidle` `TestTheRelayKnowsTheSchema`).

## Dependencies

`internal/actions`, `internal/baubles` (`PlainText`), `internal/companionai`,
`internal/conditions`, `internal/gametime`,
`internal/items`, `internal/messaging`, `internal/mobs`, `internal/mudlog`,
`internal/rooms`, `internal/shops`, `internal/users`, `internal/util`.
Imported by `internal/hooks`, `internal/behaviortree` and `modules/npcidle`.
