# internal/ambient

## Purpose

The one gate every unprompted ("ambient") line passes on its way to players
(#317): the per-listener budget and the once-per-visit rule for a room with
a single idle line. Leaf package: it imports `configs`, `messaging`,
`rooms`, `users` and `util`, and nothing that imports it (hooks,
behaviortree, mobcommands, usercommands, the weather engine) may be
imported back. Check with `go list -deps ./internal/ambient/`.

## Files

- **ambient.go**: `Source` and `Sources` (`room_idle`, `mob_chatter`,
  `weather`, `gossip`, `item_chatter`), `ParseSource`, `Line`, `Routing`,
  `Plan`, `Sees`, `AllHaveBudget`, `SingleLineHeard`,
  `MarkSingleLineHeard`, `RoundTick`.
- **testing.go**: `ResetForTest`, `ChargeForTest` for tests in any package.

## How a send site uses it

    plan := ambient.Plan(ambient.Line{Room: room, Source: ambient.SourceWeather})
    room.SendText(messaging.CategoryWeather, line, plan.Exclude...)

`Plan` charges every player who takes the line in and returns, in
`Exclude`, the caller's own exclusions plus every player over budget. The
caller keeps its own sender, so sight gating, name hiding and categories
stay where they were. Pass `Perceives: ambient.Sees(room)` for a visual
line, so a player who cannot see it is not charged. A player the caller
already excludes is never charged.

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
