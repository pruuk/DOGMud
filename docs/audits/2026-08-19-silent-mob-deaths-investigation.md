# Silent Mob Deaths Investigation (2026-08-19, run b65871f4d0de7371)

Investigates BUG-3 from `tools/playtest/reports/2026-08-19-local-bug-finder-u6b-flip-sweep.md`:
Bandit Captain (The Watch Room, 5242) and Bandit Squatter (Cracked Cistern,
5235) each observed "alive at 100% on look, corpse within a minute, zero
combat output"; a captain corpse reportedly pre-dating the player's first
arrival in a fresh ephemeral world; and bandit corpses "throughout the zone
beyond what was killed."

Evidence: full server log
`tools/playtest/.run/b65871f4d0de7371/server.log` (2529 lines, session
05:07:48 to 05:31:45), world data on branch `feature/u6b-finish-the-flip`
(commit `6340733d7` at test time), and the code paths enumerated below.

## Verdict

**There were no silent mob deaths. The playtest character killed every
bandit whose corpse it found, including both "anomaly" mobs, and did not
see itself doing it.** The perception of spontaneous death was manufactured
by a stack of narration/targeting bugs, the largest of which (BUG-1,
commanded actions produce no narration) is suspected to be introduced by
this branch. The deaths themselves are ordinary player kills, fully
attributed in combat analytics.

There is no engine path that killed a bandit in this session without
combat. Every candidate silent-kill mechanism was checked and ruled out
with evidence (section "Ruled out" below). One real, latent silent-kill
path was found in the code (`pathto home` stuck-mob HP drain) but it
demonstrably did not fire here and cannot explain corpses in the mobs'
own home rooms.

## The actual timeline (from server.log)

The playtest character is logged as `Veteran Pathfinder (user)` (userId 1).
Companions (Rocky/Fleshy, flesh golems) were dismissed at 05:18:00 and
05:18:28 (`msg="despawn" mobname="Rocky the flesh golem" reason=""`,
lines 1453/1466), before either anomaly.

### Bandit Captain, The Watch Room (5242)

The first mention of any Bandit Captain in the entire log is its own taunt,
12 seconds after its goals file loaded (spawn on player approach):

```
1694: time=05:20:33 msg=goals.mergeSeedFromArche mob_id=9115 type=survival blocked_by=g1 blocker_prio=80
1701: time=05:20:45 msg="Progression" event=skill_use skill=rhetoric bonus=1.00 character="Bandit Captain"
1704: time=05:20:49 msg="calculateCombat" Swing=1/4 Weapon="Drowned Claws (Masterwork)" Source="Veteran Pathfinder (user)" Target="Bandit Captain (mob)"
      ... (12 swings total, all Source=player, Target=captain, at 05:20:49)
1705: time=05:20:49 msg="AttackCrit" zScore=0.22 threshold=2.00 source="Veteran Pathfinder" target="Bandit Captain"
```

Twelve player swings (three 4-swing rounds, timestamps batched at mudlog
granularity), with `CritDamage rawDmg=90.1..109.4` lines interleaved. The
captain never appears in the log again: no progression, no combat, no idle
skill use. It died to the player's auto-melee at ~05:20:49. That is the ONE
captain death of the session, and it produced the ONE captain corpse
(corpses are created exclusively by the death pipeline,
`dropMobLootAndSetCorpse` in `internal/hooks/Death_MobLoot.go:83`, fired
from `Death_MobInstanceCleanup`; nothing else in the codebase calls
`room.AddCorpse`).

Consequences:

- The "pre-existing corpse at first arrival" claim is contradicted by the
  log. No captain existed, fought, died, or despawned before 05:20:33.
  There is no authored corpse item in `rooms/pothole_coulee/5242.yaml`.
  The corpse the agent attributed to "before my arrival" is the corpse of
  the captain it killed itself, unwitnessed.
- The "alive at 100%, corpse ~30s later, no combat output" pair fits the
  log exactly: look on arrival (~05:20:33-45, captain alive), silent
  engagement (the report's own BUG-1 list includes a taunt attempt on the
  Bandit Captain that printed nothing; the captain is also `hostile` +
  `aiprofile: aggressive` and engages on sight), three unwitnessed melee
  rounds, corpse by ~05:21.
- A respawned captain #2 was possible only from ~05:30:49 (respawn timer
  runs from death: `Prepare` gates on
  `spawnInfo.DespawnedRound + RespawnRate` in `internal/rooms/rooms.go:788`,
  and the captain's `respawnrate` is `10 real minutes`). If the agent's
  look pair happened on a return visit in the 05:30:30-05:31:45 window
  (gmcp.Zone pings at 05:30:30 and 05:31:06; telnet EOF 05:31:45), look #1
  caught captain #2 fresh from respawn and look #2 lost it to hiding or to
  the respawned-mob desync (BLOCKER 1 in the same report), with corpse #1
  still on the floor. Either reading ends at the same place: one death,
  player-caused.

### Bandit Squatter, Cracked Cistern (5235)

The last combat of the entire session is the player killing a Bandit
Squatter, four rounds spanning 05:25:25-05:25:34, immediately after that
squatter successfully raised its skullduggery (hide) skill:

```
2041: time=05:25:30 msg="Progression" event=skill_use skill=skullduggery bonus=1.00 character="Bandit Squatter"
2042: time=05:25:30 msg="Progression" check=skill result=PROGRESS skill=skullduggery rank=0 chance=12.00% roll=1160 threshold=1200 character="Bandit Squatter"
2046: time=05:25:34 msg="AttackCrit" zScore=-0.34 threshold=2.00 source="Veteran Pathfinder" target="Bandit Squatter"
      ... (12 swings, DistDamage dmgMean=86.6..110.7, all at 05:25:34)
```

After 05:25:34 the log contains zero combat, zero damage, and zero death
events of any kind until EOF, only NPC search/cooking progression, gmcp
pings, and RoundsWaiting lines. So the corpse the agent later found in the
cistern can only be this kill. The report itself corroborates the
mechanism without realizing it: under BUG-1 it records
"`bash <target>`: two attempts (dummy, squatter). No bash line ever
printed. Combat engaged; kill came from auto-swings."

The mob was actively hiding mid-fight (skullduggery use, and the zone's
bandits "hide constantly" per the report's C-5), which feeds both the
sparks fizzle and the missing narration (see the 08-15
"attack the darkness" finding: a hidden mob resolves to nothing and all
target-naming feedback disappears).

### The AoE question: can a "finds no targets" sparks cast kill silently?

**No.** In `internal/hooks/spell_resolution.go`, HarmArea populates its
target list from `room.GetMobs(rooms.FindAll)` and damage is applied only
inside the per-target resolve loop (`resolveAgainstMob`, which calls
`ApplyHarm` at lines 512/624). The message
`Your spell erupts outward but finds no targets.` (line 177) fires only
when `targetsResolved == 0`, i.e. when that loop resolved nothing. Zero
resolutions means zero damage, structurally. The cast still charging CP on
a fizzle is real but separate (report C-1).

Why the visible squatter wasn't targeted: each candidate is skipped when
`mob.Character.RoomId != room.RoomId` (line 134) or when hidden-mob/desync
resolution fails. This is the same family as BLOCKER 1 (respawned mobs
untargetable until room re-entry) whose suspected root the report already
noted: `FindAll = 0b111111111` covers only 9 bits while 11 find-flags now
exist (`FindHasPet` = bit 10, `FindNative` = bit 11,
`internal/rooms/rooms.go:45-63`), plus room-list/instance RoomId desync on
respawn.

### "Corpses throughout the zone"

All of them are accounted for by player kills. Combat pair totals for the
session (every `calculateCombat` line, grouped):

```
90 x Veteran Pathfinder (user) -> Bandit Scout (mob)     (multiple scouts: rooms 5232, 5233 x2, 5236, 5238)
45 x Veteran Pathfinder (user) -> Training Dummy (mob)
36 x Veteran Pathfinder (user) -> Bandit Squatter (mob)  (3 engagements: 05:16, 05:23, 05:25)
12 x Veteran Pathfinder (user) -> Bandit Captain (mob)
12 x Veteran Pathfinder (user) -> Bandit Bruiser (mob)
(+ mob-to-player return swings; no mob-vs-mob pairs exist in the log)
```

With `CorpseDecayTime: 4 hours` and bandit `respawnrate: 5 real minutes`
(10 for the captain), every kill site quickly shows a corpse AND a live
(often hiding) bandit together. The agent undercounted its own kills
because several of them were invisible to it (BUG-1 silent engagement +
hidden-target messaging), so the corpse count "exceeded" its tally.

## Ruled out, with evidence

1. **Presence despawn (Dormant -> Despawning after
   `PresenceMobDespawnAfterRounds`=60).** Every removal on this path goes
   through the `despawn` mob command, which unconditionally logs
   `mudlog.Info("despawn", ...)` (`internal/mobcommands/despawn.go:11`;
   issued as `despawn presence_despawning` from
   `internal/hooks/NewRound_IdleMobs.go:55`). The log contains exactly two
   despawn lines, both the player's dismissed flesh golems (lines
   1453/1466). Additionally, despawn leaves NO corpse (it calls
   `DestroyInstance`, never the death pipeline), so it could not produce
   these observations even if it had fired.
2. **Mob-vs-mob combat.** Every `calculateCombat` Source/Target pair in
   the log involves the player. No mob ever swung at another mob.
3. **U9 regen going negative (the known Heal()/ComputeTickAmount trap).**
   The mob regen block (`internal/hooks/NewRound_AutoHeal.go:305-360`)
   uses `ApplyRestore` exclusively, with every amount floored to a minimum
   of +1, and `ApplyRestore` no-ops on non-positive input
   (`internal/characters/pools.go`). The buff tick path
   (`tickMobBuffs`, `internal/hooks/NewRound_MobRoundTick.go:204-238`)
   explicitly sign-splits: positive TickAmount -> ApplyRestore, negative
   -> ApplyHarm. That negative branch is the intended DoT delivery path
   (poison buffs), not a regen bug, and no DoT buff was on these mobs
   (DoT conditions require combat infliction; the poison/bleed condition
   ticks at `NewRound_AutoHeal.go:384/395` require
   `ConditionPoisoned`/`ConditionBleeding`, never set on them).
4. **Weather / environment damage.** No such damage path exists for these
   rooms; room mutator regen multipliers only scale regen (still floored
   at +1 restore). Nothing in the log suggests otherwise.
5. **AoE silent damage.** Structurally impossible on the no-targets path,
   see above.
6. **Corpse wandering/duplication.** Corpses are room-local
   `rooms.Corpse` records created only at death in the room where the mob
   died (`Death_MobLoot.go`); no code relocates or copies them. A cistern
   corpse cannot be a corpse from a kill in another room. (The MOB
   wandering away while a separate corpse appears also fails here: the
   corpse still requires a death in THAT room, and the only cistern-area
   death is the 05:25:34 kill.)
7. **`pathto home` stuck-mob drain (latent, did NOT fire).** Real
   silent-kill path found during this investigation:
   `internal/mobcommands/pathto.go:19-29` drains 10% of HealthMax per
   invocation, with no log line and no room narration, once
   `home-impossible` tempdata is set (set when `mapper.GetPath` fails
   while the mob is displaced). Ten idle ticks later the mob dies an
   unattributed death and the U5c sweep reaps it
   (`NewRound_MobRoundTick.go:133`), leaving a corpse. It did not fire
   here: it requires a displaced mob whose pathing failed, both anomaly
   corpses are in the mobs' HOME rooms (captain `maxwander: 0` never
   wanders; a displaced mob with impossible pathing cannot return home to
   die there), and the zone's plain reciprocal cardinal exits give
   `GetPath` no reason to fail. Note also the check-order flaw: the drain
   check runs BEFORE the same-room early return, so a mob that somehow
   got home with the flag still set would keep draining at home.

## Why the player saw nothing (the real bug surface)

The deaths were silent to the OBSERVER, not to the engine. Stacked causes,
in rough order of contribution:

1. **BUG-1 (this branch, suspected Task 11 wrapper regression):** taunt /
   bash / grapple / throw / same-room shoot produce zero narration while
   still engaging combat. Both anomaly kills began with silent commanded
   actions (taunt on the captain, bash on the squatter, per the report's
   own BUG-1 evidence).
2. **Hidden-target messaging (pre-existing, 08-15 finding):** both mobs
   hide (squatter skullduggery PROGRESS at 05:25:30 mid-fight; C-5 "hide
   constantly"). Combat and feedback lines that name an unseeable target
   degrade or vanish, and "You attack the darkness!" mislabels the cause.
3. **Respawned-mob desync / FindAll bit-width (pre-existing, BLOCKER 1):**
   look lists the mob while targeting paths (attack/cast resolve) do not
   see it, which is exactly the sparks "finds no targets" fizzle with a
   visible squatter.
4. **Output loss in transit:** every swing against these outclassed mobs
   printed a crit banner (report C-3, confirmed by the log's near-100%
   crit z-scores at huge score gaps), the AI port caps lines per round,
   and the harness's own bridge cursor is documented (in the agent's
   notes) to skip lines when snapped after show. The one line that would
   have settled everything, `Bandit Captain has died.`
   (`Death_MobBroadcast.go`, sight-gated only by room light, so it WAS
   sent), evidently never survived to the agent's transcript.

## Is it a real bug? Did this branch introduce it?

- The mob deaths: **not a bug**. Normal, attributed player kills.
- The silence: **real bugs, already filed as BUG-1/BLOCKER-1/C-1/C-5 in
  the run report**. BUG-1 (commanded-action narration silence) is the one
  suspected to be introduced by `feature/u6b-finish-the-flip` (Task 11
  wrapper); the targeting/hidden-mob defects predate the branch.
- The `pathto home` 10% drain: **pre-existing latent hazard** (inherited
  engine behavior), not implicated in this session, but it is a genuine
  narration-free kill path that will eventually produce exactly this
  class of "mystery corpse" report.

## Recommended fixes (locations only, per the filed bugs)

1. BUG-1 root-cause: the Task 11 commanded-action messaging wrapper on
   this branch (taunt/bash/grapple/throw/shoot user-command paths). This
   is the branch blocker and the primary manufacturer of "silent" kills.
2. Respawn targetability: widen `FindAll` to cover all 11 flag bits
   (`internal/rooms/rooms.go:63`) and fix the respawn room/instance
   RoomId desync (BLOCKER 1 repro: fixed by room re-entry, i.e.
   `Room.Prepare`'s orphan reattach).
3. Hidden-target feedback: split the "attack the darkness" message per
   the 08-15 memo (empty-arg vs not-found vs cannot-see) so hidden mobs
   stop reading as empty rooms; refund or refuse CP on no-target AoE
   fizzles (`spell_resolution.go:177` region).
4. `internal/mobcommands/pathto.go:19-29`: add a `mudlog.Info` (mirroring
   `despawn`) and a room-visible narration when the stuck-mob drain
   fires, and move the drain check after the same-room early return so a
   mob at home can never drain. Consider clearing `home-impossible` on
   arrival home.
5. Harness side: bridge cursor snap-before-show (the agent's own note),
   and revisit the AI-port per-round line cap, both of which can eat a
   death broadcast.

## Status

Investigation complete. No code changed. This file is intentionally left
untracked.
