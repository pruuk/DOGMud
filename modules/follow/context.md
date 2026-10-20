# Follow Module Context

## Purpose

`modules/follow` implements `follow` for both players and mobs: a follower
moves when its target moves. It is a thin relationship map plus a large set of
event listeners whose real job is **breaking the link at the right moments**.

## Types

```go
type followId struct { /* user or mob identity in one value */ }
type FollowModule struct { /* target → followers */ }
```

`followId` unifies users and mobs so one map serves both, with
`getFollowIdInstance()` resolving back to whichever is real.

## Behaviour

```go
startFollow(target, source followId, cutoffRound ...uint64)
stopFollowing(source followId) followId
loseFollowers(target followId) []followId
isFollowing(followId) bool
getFollowers(target followId) []followId
```

`startFollow` takes an optional **cutoff round** — a follow that expires — which
is how temporary escorting is expressed without a separate mechanism.

## Where the work actually is

The listener set is the substance of the module:

- `roomChangeHandler` — the actual following.
- `onNewRound` — expiry and reconciliation.
- `onPartyChange` — party membership changes the relationship.
- `idleMobHandler` — a following mob acts on its own idle tick.
- `playerDespawnHandler`, `onPlayerDeath`, `onMobDeath` — break the link.

Every one of those exists because a stale follow edge produces a mob trailing a
player who no longer exists, or a follower that cannot be shaken off.

## Sight (#454)

The player `follow <name>` runs the typed name through `actions.AimBySight`
before it resolves, like every command that names a creature: full sight
resolves names, shapes only resolves `shape` / `N.shape` / `shape#N` (and
`@N` / `#N`) and answers a typed name with a hint, no sight resolves nothing
("You can't find them here."). `follow stop` and `follow lose` name no one and
need no sight. The lines that follow a start hide each name at its reader's
sight (`messaging.HideNames` for the follower, `HideSpeakerNames` for the
target, who reads "Someone is following you."). `follow stop` tells the
followed user the follower's name the same way ("Someone stopped following
you." in the dark), and the follower's own line hides the followed name at
the follower's sight. The mob `follow` is not gated: mobs act on shapes.
The line a player reads when a mob starts following them ("Rat is following
you.") hides the mob's name at that player's sight (`readerSight`, #554).
Every line a player reads about a follow ending hides the other party's
name at that player's sight in their own room (`readerSight`): a mob's
`follow stop` ("Rat stopped following you.", which once printed the
player's own name) and `follow lose`, and the round expiry lines in
`onNewRound` for player and player, mob and player, player and mob. A mob
subject reads "Something", a player subject "Someone".

## Light and parties (owner ruling 2026-10-17)

Following keys off the FOLLOWED person's light. `roomChangeHandler` moves a
player follower only when `rooms.(*Room).DepartureSight(follower, mover)` is
not `SightNone`: the room the mover left, lit as if the mover still stood
in it. An unlit follower follows a lit leader; in true darkness with no
light, or while blinded, the follower reads `FollowLostLine` and the follow
ends (`loseSightOf`, which tells the followed player "X stopped following
you." at their sight). Party auto-follow in `usercommands.Go` uses the same
gate and says `usercommands.PartyFollowLostLine`. Mob followers are not
gated.

`follow <name>` works inside a party (it once refused there). Following
someone you already follow, by `follow` or as a member of the party they
lead, answers "You're already following X." (`alreadyFollows`), the name
hidden at the follower's sight. `movingWithParty` skips a follower whose
party is walking together (the follower and the mover share a party whose
leader now stands where the mover went), so party auto-follow does not
walk them twice or end their follow. Known edge: a member following
another member who walks alone into the room where the leader already
stands is not moved.

`help follow` ships here (`files/datafiles/templates/help/follow.template`)
and is indexed under `parties` by `files/data-overlays/keywords.yaml`; it
and `help party` link to each other.

## Menu pull (#375 phase 0)

When a follower who is a player is about to step after their leader, and
passes the sight check above, the module calls
`actions.BreakOffMenuToFollow(user, moverCharacter(evt), evt.UserId, fromRoom)`
before queuing the follower's step command. An open game menu or non-modal
prompt closes and the three break-off lines are spoken, judged with the
leader's own light. A follower who lost sight of the leader never reaches this
call: the follow ends instead. Party auto-follow in `usercommands.Go` makes the
same call for party members.

## Gotchas

- **Any new way for an actor to leave the world needs a listener here.**
  Despawn, death, and disconnect are covered; a future teleport-out or
  instance-transfer is not automatically.
- **`loseFollowers` returns the dropped followers** so the caller can message
  them. Discarding the return leaves players silently un-followed.
- **Follow is not the same as party.** They interact (`onPartyChange`,
  `alreadyFollows`, `movingWithParty`) but a party member is not a follower
  in this module's maps; party auto-follow lives in `usercommands.Go`.
- **Mobs follow via their idle tick**, so a followed player moving faster than
  the mob's tick can outrun it. That is intended.

## Dependencies

`plugins`, `events`, `users`, `mobs`, `rooms`.

## Consumers

Registered as a plugin; provides the user and mob `follow` commands.
