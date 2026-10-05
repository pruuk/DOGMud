# lookdetail module Context

## Purpose

The model side of generated closer looks (`internal/lookdetail`). It installs
the `lookdetail.Generator` that writes what a closer look at something in a
room's description shows, on the looker's own key. It never writes one on
the server's key.

On by default (`Enabled: true`); it costs nothing unless the looker has
their own key up in the companion's key relay and has left "Make the world
livelier" ticked. It lends the key under its own purpose,
`apiframework.PurposeLookDetail`, with its own breaker and allowance
(`lookdetail.keyholder`, consumer `lookdetail`).

## Files

- **lookdetail.go**: `LookDetailModule`, `feature` (its `lively.Feature`),
  registration, `onLoad`, `onNewRound`, `onSave`, `configure`, and the
  `Generator` methods `Reserve` (`lively.Turns`) and `Generate`
  (`lively.Ask`, then `lively.Moderate`).
- **config.go**: `Config`, `buildConfig`, the live readers
  `minSecondsPerPlayer`, `dailyTokensPerUser`, and `limits`.
- **prompt.go**: `PromptVersion`, the system prompt (scenery only: nothing to
  take, use or act on; no other players), `buildMessages`.

## Config (`Modules.lookdetail` in config.yaml)

`Enabled` (true), `MinSecondsPerPlayer` (5, live), `TimeoutSeconds` (20),
`MaxCompletionTokens` (800), `DailyTokensPerUser` (20000, live),
`ModerateOutput` (true), `ModerationModel` (omni-moderation-latest),
`LogRequests` (false). No chance setting: every eligible look is answered.

## Dependencies

`internal/apiframework`, `internal/events`, `internal/lively`,
`internal/lookdetail`, `internal/mudlog`, `internal/plugins`.
