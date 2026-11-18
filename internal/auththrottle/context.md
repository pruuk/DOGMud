# auththrottle

Counts login attempts per source and locks a source out after too many.

## Surface

- `Tracker`, made with `New(name)`; `name` heads the lockout warning in the log.
- `(*Tracker).Attempt(ip, now) bool`: refuse the source, or count one attempt
  for it, in one step. Call it BEFORE any password work. A source may have
  `MaxFailures` (5) attempts in `FailureWindow` (5 min) that no success gave
  back; the next is refused and locks it out for `LockoutDur` (1 min).
- `(*Tracker).Refund(ip)`: a success gives back its own attempt. It never
  lifts a lockout in force.
- `(*Tracker).Clear(ip)`: forget a source. Only /admin uses it, where a
  success needs an admin's own credentials.
- `(*Tracker).Throttled(ip, now)`: is the source locked out (counts nothing).
- `(*Tracker).NoteFailure(ip, now)`: count a known failure Attempt did not.
- `SourceKey(ip)`: the key every method uses. IPv4 as is; IPv6 by its /64.
- `(*Tracker).Len()`, `(*Tracker).ResetForTest()`.

## Users

- `internal/web` `adminThrottle`: the /admin Basic Auth check, keyed by the
  proxy-resolved client IP. Throttled first (answers credential-less requests
  too), Attempt for a presented credential, Clear on an admin login.
- `internal/inputhandlers` `gameLoginThrottle`: the game login (telnet, AI
  port, web client), keyed by `ConnectionDetails.ClientIP()`. Attempt before
  the password check, Refund on a good login.

The two trackers are separate, so a flood on one login never locks the other.

## Rules

- Count before the password check, in one step with the lockout check: a
  check-then-count lets a burst of parallel connections all pass before any
  failure is counted.
- Never clear the game login's count on a success: anyone can make an account,
  so a clear would let a guesser reset the count between guesses. Refund only.
- Key by source, never by username: a per-username lockout would let anyone
  lock a real player out of their own account.
- Keys come from attacker-controlled connection data, so a tracker is capped
  at `MaxEntries` (4096). It sweeps expired records on every write and, when
  full, evicts the least-recently-failed record that is not locked out, so a
  flood of new sources cannot lift a lockout.
- A shared address (a carrier NAT, a campus, or every web player if
  `Network.TrustedProxies` is wrong) shares one count.
- Safe for concurrent use; it takes no other lock.
