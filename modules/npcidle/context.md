# npcidle module Context

## Purpose

The model side of generated idle moments (`internal/npcidle`). It installs
the `npcidle.Generator` that writes an NPC a fresh idle emote or say on the
own key of a player in the room. It never writes one on the server's key.

On by default (`Enabled: true`), and it costs nothing unless a player has
their own key up in the companion's key relay and has left "Make the world
livelier" ticked on the key page (the shared lively permission, which starts
ticked). It lends the key under its own purpose, `apiframework.PurposeNPCIdle`,
so it has its own breaker and allowance apart from other lively features.

## Files

- **npcidle.go**: `NpcIdleModule`, `feature` (its `lively.Feature`, with
  moderation breaker `npcidle-moderation`), registration, `onLoad`,
  `onNewRound` (refreshes the server settings and the live settings),
  `onSave` (`apiframework.SaveBudget`), `configure`, and the `Generator`
  methods `Chance`, `Reserve` (`lively.Turns`) and `Generate`
  (`lively.Ask`, then `lively.Moderate`).
- **config.go**: `Config`, `buildConfig`, the live readers `chance`,
  `minSecondsPerPlayer`, `dailyTokensPerUser`, and `limits`.

The relay call, the turn-taking and the moderation are `internal/lively`'s,
shared with every lively feature; this module brings the prompt, the
schema and its own counters.
- **prompt.go**: `PromptVersion`, the system prompt, `buildMessages`.

## How a call goes

1. `Reserve(userId)`, under the mud lock: the relay answers
   `Model(userId, PurposeNPCIdle)`, their `npcidle.keyholder` allowance is
   not spent, they have no moment in flight, and `MinSecondsPerPlayer` has
   passed since their last one started. It takes their turn.
2. `Generate`, off the lock: the prompt (`buildMessages`) under the
   `npc_idle` strict schema; held against their allowance only
   (`apiframework.Reserve(ConsumerNPCIdle, ..., spendServer=false, ...)`);
   sent through the relay as `CarriesNoPlayerData`; charged and settled as
   a relayed call. The turn is always given back.
3. The outcome feeds this feature's own breaker on the key only
   (`Relay.Result(userId, PurposeNPCIdle, err)`): a provider error or an
   answer outside the schema counts; an answer that parses but that
   `npcidle.CleanResult` refuses does not. A refused reservation, a relay
   that went away, a call given up on count for nothing.
4. `moderate`, the baubles policy for text a player's key wrote: with
   `ModerateOutput` and a server key (and the provider and moderation
   breakers closed), checked through the server's key; a flag or a failed
   check keeps it out (the set line runs). When it cannot be checked, the
   moment is `KeyholderOnly`. `ModerateOutput` off: shown to all unchecked.

## Config (`Modules.npcidle` in config.yaml)

`Enabled` (true), `Chance` (5, 0 to 100, live), `MinSecondsPerPlayer` (30,
live), `TimeoutSeconds` (20, 3 to 40), `MaxCompletionTokens` (600, 100 to
2000), `DailyTokensPerUser` (20000, 0 no cap, live), `ModerateOutput`
(true), `ModerationModel` (omni-moderation-latest), `LogRequests` (false).
Live settings are re-read each round, so `server set` takes effect without a
restart; for a playtest, set `Chance` to 100.

## Gotchas

- **Needs the companion's relay.** With the aicompanion module off, or
  `PlayerKeys` off, or no valid `RelayOrigin`, `apiframework.PlayerRelay`
  lends nothing and no moment is ever written.
- **The key** is the player's and stays in their browser; nothing here sees
  it. The server's key is used only by `apiframework.Moderate`.
- `apiframework`'s key guard reads this package's tests.

## Dependencies

`internal/apiframework`, `internal/events`, `internal/lively`,
`internal/mudlog`, `internal/npcidle`, `internal/plugins`.
