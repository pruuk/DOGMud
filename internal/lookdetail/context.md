# lookdetail Context

## Purpose

The engine side of generated closer looks. When a player's `look at X`
finds nothing while they see the room clearly, and X is named in the room's
description, a model writes what a closer look shows. The model side is
`modules/lookdetail`, which installs the `Generator`. With none installed and
nothing cached, the look answers as it always has.

Only ever on the looker's own key, while they leave "Make the world
livelier" ticked (`apiframework.PurposeLookDetail`).

## Files

- **lookdetail.go**: `Generator` (`Reserve`, `Generate`), `SetGenerator`,
  `Result` (`Text`, `KeyholderOnly`), `MaxGenerateTime`, `CacheEntries`,
  `TryLook`, `Pending`, `ResetCacheForTest`, `InlineForTest`; the delivery
  (`startDelivery`, `run`, `finish`) and `show`.
- **match.go**: `Phrase` (what may be looked at: 3 to 40 characters, at most
  4 words, letters, spaces, `'` and `-`, a leading article dropped, no bare
  stopword) and `Find` (whole words in the plain description, with the plain
  plural or singular of the last word; returns the words as the description
  has them and the sentence they are in).
- **request.go**: `Request`, `Snapshot`, `ReplySchemaName` (`look_detail`),
  `ReplySchema`, `ParseReply`, `CleanResult`, `ErrUnusable`, `MinTextRunes`,
  `MaxTextRunes`.

## Where it is called

`usercommands.Look`, last, just before "Look at what???", and only at
`messaging.SightFull`: every sight rule of the look (`actions.ResolveLook`)
has already run, so the dark ("You can't see anything!") and shapes ("You
can only make out shapes here.") answers are unchanged, and every real
target (creature, container, carried noun, item, room noun, hidden noun,
pet, corpse, floor item) answers first.

## How a look goes

1. `Phrase`, then `Find` in `Room.GetDescription()`. Not named: false, the
   look answers as always.
2. The cache (an LRU of `CacheEntries`, keyed by room, a hash of the
   description and the words as named): a hit is shown at once, to anyone,
   key or not.
3. Otherwise a generator, the looker's turn (`Reserve`) and one look on its
   way per player (a second look meanwhile waits on it, silently). No key:
   false, the look answers as always.
4. A delivery goroutine calls `Generate` off the lock and `CleanResult`,
   then `finish` under the lock: a detail anyone may read is cached; then,
   for a looker still in the room, the light judged NOW by
   `messaging.ParticipantSight` (the look command's own words for the dark
   and for shapes), else the detail, or "You see nothing special about the
   X." when none came.
5. `show` words it as a room noun's answer is worded, with the same "is
   examining the X" line for onlookers through the room's sight-gated
   sender, unless the looker is hidden.

## Gotchas

- **Scenery only.** The prompt forbids anything a player could take, use or
  act on. Nothing generated becomes a noun, item or exit.
- **Untrusted output.** `CleanResult`: printable ASCII without markup, one
  paragraph of 20 to 700 characters; dashes become commas. "You" is allowed:
  the look is the looker's own.
- A `KeyholderOnly` detail is never cached.
- `ReplySchemaName` must be in `LIVELY_SCHEMAS` in relay.js
  (`modules/lookdetail` `TestTheRelayKnowsTheSchema`).

## Dependencies

`internal/baubles` (`PlainText`), `internal/gametime`, `internal/messaging`,
`internal/mobs`, `internal/mudlog`, `internal/rooms`, `internal/users`,
`internal/util`, `github.com/hashicorp/golang-lru/v2`. Imported by
`internal/usercommands` and `modules/lookdetail`.
