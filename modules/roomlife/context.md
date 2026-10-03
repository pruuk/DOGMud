# roomlife module Context

## Purpose

The model side of generated ambient events (`internal/roomlife`). It installs
the `roomlife.Generator` that writes a room a fresh event, seen or heard, on
the own key of a player in it. It never writes one on the server's key.

On by default (`Enabled: true`); it costs nothing unless a player has their
own key up in the companion's key relay and has left "Make the world
livelier" ticked. It lends the key under its own purpose,
`apiframework.PurposeRoomLife`, with its own breaker and allowance
(`roomlife.keyholder`, consumer `roomlife`), apart from idle moments.

## Files

- **roomlife.go**: `RoomLifeModule`, `feature` (its `lively.Feature`),
  registration, `onLoad`, `onNewRound` (server settings and live settings),
  `onSave`, `configure`, and the `Generator` methods `Chance`, `Reserve`
  (`lively.Turns`) and `Generate` (`lively.Ask`, then `lively.Moderate`).
- **config.go**: `Config`, `buildConfig`, the live readers `chance`,
  `minSecondsPerPlayer`, `dailyTokensPerUser`, and `limits`.
- **prompt.go**: `PromptVersion`, the system prompt, `buildMessages`.

## Config (`Modules.roomlife` in config.yaml)

`Enabled` (true), `Chance` (10, live), `MinSecondsPerPlayer` (60, live),
`TimeoutSeconds` (20), `MaxCompletionTokens` (600), `DailyTokensPerUser`
(20000, live), `ModerateOutput` (true), `ModerationModel`
(omni-moderation-latest), `LogRequests` (false). For a playtest, `server set`
`Chance` to 100; the room's own ambient roll (5% a round) still decides when
an event is due.

A room claimed by a `roomlife.PlaceHook` (a rift) uses that place's own
chance instead of `Chance` (while `Chance` is above zero), and its
`Request.Setting` is sent as `setting`; the system prompt says a setting
overrides Gaius (prompt version 2). Such a room may send no `time_of_day`.

## Gotchas

- A seen event written for a keyholder who cannot see is refused by
  `roomlife.CleanResult`; that is the rules, not the key failing, so its
  breaker hears a success (`lively.ErrUnusable`).
- Needs the companion's relay (aicompanion on, `PlayerKeys`, a valid
  `RelayOrigin`); without it no event is ever written.
- `apiframework`'s key guard reads this package's tests.

## Dependencies

`internal/apiframework`, `internal/events`, `internal/lively`,
`internal/mudlog`, `internal/plugins`, `internal/roomlife`.
