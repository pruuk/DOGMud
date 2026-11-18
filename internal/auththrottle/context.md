# auththrottle

Counts failed logins per source and locks a source out after too many.

## Surface

- `Tracker`, made with `New(name)`; `name` heads the lockout warning in the log.
- `(*Tracker).Throttled(key, now) (bool, time.Duration)`: is `key` locked out,
  and for how long. Ask it BEFORE any bcrypt work so a lockout saves the CPU.
- `(*Tracker).NoteFailure(key, now)`: count a failure; `MaxFailures` (5)
  inside `FailureWindow` (5 min) lock `key` out for `LockoutDur` (1 min).
- `(*Tracker).Clear(key)`: forget a source after a good login.
- `(*Tracker).Len()`, `(*Tracker).ResetForTest()`.

## Users

- `internal/web` `adminThrottle`: the /admin Basic Auth check, keyed by the
  proxy-resolved client IP.
- `internal/inputhandlers` `gameLoginThrottle`: the game login (telnet, AI
  port, web client), keyed by `ConnectionDetails.ClientIP()`.

The two trackers are separate, so a flood on one login never locks the other.

## Rules

- Keys come from attacker-controlled connection data, so a tracker is capped
  at `MaxEntries` (4096): it sweeps expired records on every write and evicts
  the least-recently-failed record when full.
- Key by source, never by username: a per-username lockout would let anyone
  lock a real player out of their own account.
- Safe for concurrent use; it takes no other lock.
