# internal/ambient

## Purpose

The one gate every unprompted ("ambient") line passes on its way to players
(#317): the per-listener budget, the once-per-visit rule for a room with
a single idle line, and the switchable telemetry that measures both. Leaf
package: it imports `conditions`, `configs`, `messaging`, `mudlog`, `rooms`,
`users` and `util`, and nothing that imports it (hooks,
behaviortree, mobcommands, usercommands, the weather engine) may be
imported back. Check with `go list -deps ./internal/ambient/`.

## Files

- **ambient.go**: `Source` and `Sources` (`room_idle`, `mob_chatter`,
  `weather`, `gossip`, `item_chatter`, `say_echo`), `ParseSource`, `Line`,
  `Routing`, `Plan`, `Sees`, `HearsQuiet`, `AllHaveBudget`, `SingleLineHeard`,
  `MarkSingleLineHeard`, `RoundTick`.
- **telemetry.go**: `MoveKind` and `MoveKinds` (`move_idle`, `move_track`,
  `move_path`), `TelemetryMessage`, `TelemetryEnabled`, `CountMove`; the
  unexported counters, `countLine` (called by `Plan`) and `telemetryTick`
  (called by `RoundTick`).
- **testing.go**: `ResetForTest` (budget, visits and telemetry),
  `ChargeForTest`, `MovesForTest`, for tests in any package.

## How a send site uses it

    plan := ambient.Plan(ambient.Line{Room: room, Source: ambient.SourceWeather})
    room.SendText(messaging.CategoryWeather, line, plan.Exclude...)

`Plan` charges every player who takes the line in and returns, in
`Exclude`, the caller's own exclusions plus every player over budget. The
caller keeps its own sender, so sight gating, name hiding and categories
stay where they were. Pass `Perceives: ambient.Sees(room)` for a visual
line, so a player who cannot see it is not charged. A player the caller
already excludes is never charged.

An ambient say's echo next door ("You hear someone talking.", owner
2026-10-30) is planned once per neighbouring room, from
`mobcommands.Ambient` through `actions.MobActor.EchoExclude` and
`rooms.Room.SendTextToExitsExcluding`, with source `say_echo` and
`Perceives: ambient.HearsQuiet`. The echo is a quiet message, which the
delivery hook gives only to a player with the `superhearing` condition
flag, so only such a player is charged; no shipped condition grants it
today. Every other say's echo never comes here.

`AllHaveBudget` reports, without charging, whether every player present
could hear a line now; an NPC conversation start is gated on it so none is
cut off halfway. `SingleLineHeard` and `MarkSingleLineHeard` hold the
once-per-visit rule for a room with one idle line. `RoundTick` runs once a
round: it ends visits of players who left or logged out and prunes spent
budget entries.

## Rules

- The budget window is `Balance.AmbientListenerBudgetRounds` divided by the
  room's zone `ambientmultiplier` (`rooms.ZoneAmbientMultiplier`, 1.0 when
  unset), rounded, at least one round. A knob of 0 turns the budget off
  (the Go default; `config.yaml` ships 5). A zone multiplier of 0 with the
  budget on makes the window endless, so no budgeted line gets through.
- An over-budget line is skipped for that player only; everyone else in
  the room still gets it.
- State is in memory only (`budgetNext`, `singleHeard`), the shape of the
  item chatter listener cap. A restart clears it, which is harmless. Both
  maps are mutex guarded, so calls are safe from the game loop.

## Telemetry

`Balance.AmbientTelemetry` (ships `false`) switches it on;
`Balance.AmbientTelemetryReportMinutes` (ships 5) is the window in real
minutes, turned into rounds through `Timing.RoundSeconds`. It stays in the
code permanently, for the next retune.

- **Off**, nothing is counted or kept: `countLine` and `CountMove` return
  after the switch check, and `telemetryTick` drops anything left from an
  earlier on period once, then costs one atomic load a round.
- **On**, each `Plan` counts per room and source, per listener taking the
  line in: offered = delivered + suppressed (suppressed means over budget or
  hushed). A player the caller excluded or who does not perceive the line is
  not counted. `say_echo` is counted in the room that receives the echo,
  since its `Plan` names that room.
- `CountMove(roomId, kind)` counts a committed wander step, in the room the
  mob leaves, at the point the step is committed (a rooted mob or a locked
  exit can still refuse the queued `go`, so a count is a step taken, not a
  guaranteed arrival):
  - `MoveIdle`: `mobcommands.Wander` once its step is paid for (not when
    it is scattered, a pack follower, sent home, over budget, too tired or
    facing an exit out of its zone), once more for each pack follower that
    goes with it, and `mobcommands.Go` for a step through a mob's own dark
    portal (`Portal` only opens it). The idle roll in
    `hooks/MobIdle_HandleIdleMobs.go` counts nothing, since it only queues
    the command.
  - `MovePath`: the path walker (`hooks/NewRound_IdleMobs.go`), which also
    carries goal planner `pathto` walks.
  - `MoveTrack`: `move_toward_tracked` (`behaviortree/actions_scout.go`).
  `sneak` rides the idle movement roll, so the chatter scale does not
  change how often thieves hide, but it hides in place: it is not a move
  and is never counted.
- Each round adds the players present to every occupied room's listener
  rounds. A window is exactly `window_rounds` rounds, counting the round it
  starts on.
- When the window is up, one Info line per room with any listener rounds,
  line or move is written, sorted by room id, and every counter resets.
  The line's message is `ambient.telemetry` and its keys, in order, are
  `window`, `room`, `zone`, `round_seconds`, `window_rounds`,
  `listener_rounds`, then `<source>_offered`, `<source>_delivered`,
  `<source>_suppressed` for each source in `Sources` order, then
  `move_idle`, `move_track`, `move_path`. `tools/ambient_report.py` reads
  this format, so keep it stable.
