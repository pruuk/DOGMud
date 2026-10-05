# lively Context

## Purpose

What every "make the world livelier" feature shares to write something on a
player's own key. A lively feature (today `modules/npcidle`,
`modules/roomlife`, `modules/lookdetail` and the room writer in
`modules/rifts`) brings its own purpose, consumer, allowance dimension,
prompt and reply schema; this package does the rest the same way for all of
them. It never touches the server's budget or key, except to moderate.

## Files

- **lively.go**: `Feature` (`Name`, `Purpose`, `Consumer`, `Dim`,
  `ModerationBreaker`), `Limits`, `Turns` (`NewTurns`, `Reserve`, `Release`,
  `Busy`, the `Now` and `Relay` seams), `Ask`, `Moderate`, `SchemaOverhead`,
  `ErrNoRelay`, `ErrNotModerated`, `ErrUnusable`, and the config readers
  `Getter`, `String`, `Bool`, `Int`.

## How a lively call goes

1. `Turns.Reserve(f, limits, userId)`, under the mud lock: the relay answers
   `Model(userId, f.Purpose)` (the player's "Make the world livelier" box,
   `apiframework.IsLively`), f's allowance for them is not spent, and they
   have no call for f in flight or within `MinSecondsPerPlayer`. It takes
   their turn. Each feature has its own `Turns`, so features are spaced apart
   from each other only by their own settings.
2. `Ask`, off the lock: the player's own model, held against f's allowance
   alone (`apiframework.Reserve(f.Consumer, ..., spendServer=false, ...)`),
   sent as `CarriesNoPlayerData`, charged and settled as a relayed call.
   `decode` judges the content: an error wrapping `ErrUnusable` is the rules
   refusing what the model wrote (reported to f's breaker as a success: the
   key answered); any other error is the schema ignored (a failure). A
   provider error is a failure. A refused reservation, a relay that went
   away, a call given up on count for nothing.
3. The feature calls `Turns.Release` however it ends (deferred).
4. `Moderate`, the baubles policy for text a player's key wrote: checked
   through the server key when it can be; a flag or failed check is
   `ErrNotModerated` or an error (the feature shows its set text instead);
   not checkable: `keyholderOnly`; `ModerateOutput` off: everyone, unchecked.

## Adding a lively feature

1. `apiframework`: a `LivelyPurpose` constant, a consumer and an allowance
   dimension.
2. Its schema name in `LIVELY_SCHEMAS`
   (`modules/aicompanion/relayweb/relay.js`), with a test that reads it there.
3. An engine seam (an `internal/<feature>` package with a `Generator`
   interface and the call site) and a module that installs it, using a
   `Feature`, a `Turns`, `Ask` and `Moderate`. Deliver through the engine's
   own senders so the light rules hold.
4. A config block in `config.yaml`, and the module's tests added to
   `apiframework`'s key guard.

## Dependencies

`internal/apiframework`. Imported by `modules/npcidle`, `modules/roomlife`,
`modules/lookdetail` and `modules/rifts`.
