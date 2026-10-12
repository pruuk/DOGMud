# Special-move pairing audit (M6 slice 2, Task 6)

- **Date:** 2026-10-12
- **Audited from:** branch `feat/m6-slice2-combat-copy` at
  `0b4dc6bab19838f4c119e82eb7e2bec7d4f7be69`, after the pool merge
  (`a726d93a0`) that put mob and player variants into one pool per event.
- **Scope:** every index of every event in the fourteen files under
  `_datafiles/world/dogmud/narration/special-moves/`. Closes ledger rows 57,
  58 and 59 (#370).

## Rules

A row is one index of one event: variant N of every authored role
(`actor`, `actee`, `observer`, `remote_observer`). It passes when:

- **(a)** every role describes the same moment;
- **(b)** a room line (`observer`, `remote_observer`) states the same outcome
  as the personal lines: a hit or partial room line says the blow landed, a
  miss says it missed (owner ruling, 2026-10-12);
- **(c)** it reads right for both a mob and a player actor, since both twins
  now draw every variant: the actor is "their", never "its", and a line names
  only a body part or gear every actor of that verb has.

A failing row was fixed by reordering lines of one role within the event,
by rewriting the failing line, or, where nothing fit, by authoring one.

## Body part and gear judgements (rule c)

Who can use each verb, read from `internal/combat/ai.go` (`CanUse*`) and the
species files:

| Verb | Who uses it | Judgement |
|---|---|---|
| bash | players with a shield; mobs with a shield, or `naturalbash` species (flesh golem, earth and magma elementals, automaton) | "shield bash" and "with their shield" did not fit the natural bashers. Every line now names the instrument through `{label}` / `{verb}` / `{with}`, which both callers fill (player: shield bash / bashes / with their shield; natural basher: crushing slam / slams into / with tremendous force) |
| drain | `lifedrain` species: wraith, spectre, vampire | "claws" did not fit spectres or wraiths. Lines are now body-neutral (seize, touch, reach, latch onto) |
| hamstring | mob only; legged beasts with a bite **or** claws | "fangs" did not fit clawed beasts. Lines are now body-neutral (tears into your legs) |
| pounce | legged beasts with fangs or claws | "raking into them" implied claws; now "slam into" |
| kick | any actor with legs, including wolves, rats and insects | "boot" and "heel" did not fit beasts. Every kick line now says "foot". "knee" stays: knee lines fire only in a grapple, and any legged actor has a knee |
| gore | horned beasts only | horns fit every actor |
| maul, throttle | fanged beasts only | fangs, teeth and jaws fit every actor |
| rake | clawed beasts only | claws fit every actor |
| trip (tailsweep) | tailed species only | the tail fits every actor |
| trip (trip), grapple, shoot, charge, throw | no body part or gear named beyond legs, arms or the weapon token | no change |

## Tables

### bash

| event | index | verdict | note |
|---|---|---|---|
| knockdown | 0 | ok |  |
| knockdown | 1 | rewritten | hardcoded shield bash now `{label}`, worded apart from index 0 |
| hit | 0 | ok |  |
| hit | 1 | rewritten | hardcoded shield bash now `{label}` in every role |
| partial | 0 | rewritten | room line's who-clause hung off `{with}`; now says the bash failed to floor them |
| partial | 1 | rewritten | hardcoded shield bash now `{label}`; room line states the hit |
| miss | 0 | ok |  |
| miss | 1 | rewritten | hardcoded shield bash now `{label}` |

### charge

| event | index | verdict | note |
|---|---|---|---|
| knockdown | 0 | ok |  |
| hit | 0 | rewritten | room line now states the charge landed |
| partial | 0 | ok |  |
| miss | 0 | ok |  |
| whiff_on_prone | 0 | ok | mob only; "overruns" is left for the retired-idiom task |

### drain

| event | index | verdict | note |
|---|---|---|---|
| hit | 0 | ok |  |
| hit | 1 | rewritten | claws removed; spectres and wraiths drain too |
| hit | 2 | ok |  |
| hit | 3 | rewritten | row 58: doubled apostrophe fixed; claws removed from the actor line; row 57: the tear-into room line moved out and a touch-drain room line written, since the line displaced from index 4 did not fit here either |
| hit | 4 | reordered | row 57: "tears into ... feeds hungrily" room line moved here from index 3 |
| hit_heal | 0 | ok | actor only |
| hit_heal | 1 | ok | actor only |
| hit_heal | 2 | ok | actor only |
| partial | 0 | rewritten | "its", "slip most" and "sliver" in the actee line |
| partial | 1 | rewritten | claws and "sliver" removed |
| partial | 2 | ok |  |
| partial_heal | 0 | ok | actor only |
| partial_heal | 1 | ok | actor only |
| miss | 0 | ok |  |
| miss | 1 | rewritten | claws removed from the actee line |
| miss | 2 | ok |  |
| miss | 3 | ok |  |

### gore

| event | index | verdict | note |
|---|---|---|---|
| knockdown | 0 | rewritten | "its" in the actee line |
| knockdown | 1 | ok |  |
| knockdown | 2 | ok |  |
| knockdown | 3 | ok |  |
| knockdown | 4 | ok |  |
| hit | 0 | rewritten | "its" in the actee and room lines |
| hit | 1 | ok |  |
| hit | 2 | ok |  |
| hit | 3 | ok |  |
| hit | 4 | ok |  |
| partial | 0 | ok |  |
| partial | 1 | ok |  |
| partial | 2 | ok |  |
| miss | 0 | ok |  |
| miss | 1 | ok |  |
| miss | 2 | ok |  |
| miss | 3 | ok |  |

### grapple

| event | index | verdict | note |
|---|---|---|---|
| success | 0 | ok |  |
| success | 1 | ok |  |
| fail | 0 | ok |  |
| fail | 1 | ok |  |
| crit_failure | 0 | ok | actee line carries a " - " dash, left for the copy task |
| disarm | 0 | ok |  |
| prone_penalty | 0 | ok | actor only; carries a " - " dash, left for the copy task |
| defense_exposed | 0 | ok | actor only |

### hamstring

| event | index | verdict | note |
|---|---|---|---|
| hit | 0 | rewritten | fangs and "its" removed; clawed beasts hamstring too |
| partial | 0 | rewritten | fangs removed |
| miss | 0 | ok |  |

### kick

| event | index | verdict | note |
|---|---|---|---|
| standard_knockdown | 0 | ok |  |
| standard_knockdown | 1 | ok |  |
| standard_knockdown | 2 | ok |  |
| standard_knockdown | 3 | ok |  |
| standard_hit | 0 | ok |  |
| standard_hit | 1 | ok |  |
| standard_hit | 2 | ok |  |
| standard_hit | 3 | authored | row 59: room line authored for the foot-connects moment; boot now foot |
| standard_hit | 4 | reordered | row 59: room line moved here from index 3; the outcome-less "snaps a kick at" line retired, index 5 already states that kick landing |
| standard_hit | 5 | ok |  |
| standard_hit | 6 | ok |  |
| standard_partial | 0 | rewritten | boot now foot; actee line rid of "slip most" and "clips" |
| standard_partial | 1 | rewritten | boot and "clips" removed; actee line now pairs with the actor's guard |
| standard_partial | 2 | ok |  |
| standard_miss | 0 | ok |  |
| standard_miss | 1 | ok |  |
| standard_miss | 2 | ok |  |
| standard_miss | 3 | ok |  |
| stomp_hit | 0 | ok |  |
| stomp_hit | 1 | rewritten | heel now foot |
| stomp_hit | 2 | rewritten | boot now foot |
| stomp_hit | 3 | rewritten | heel now foot |
| stomp_hit | 4 | reordered | room lines 4 and 5 swapped so "crushes underfoot" sits with its actee; the actor's boot line now "crush underfoot" |
| stomp_hit | 5 | reordered | room lines 4 and 5 swapped so "viciously stomps" sits with "a vicious stomp" |
| stomp_hit | 6 | rewritten | heel and boot now foot |
| stomp_partial | 0 | rewritten | heel now foot |
| stomp_partial | 1 | rewritten | heel now foot |
| stomp_miss | 0 | ok |  |
| stomp_miss | 1 | ok |  |
| stomp_miss | 2 | ok |  |
| stomp_miss | 3 | ok |  |
| knee_hit | 0 | ok |  |
| knee_hit | 1 | ok |  |
| knee_hit | 2 | ok |  |
| knee_hit | 3 | ok |  |
| knee_hit | 4 | ok |  |
| knee_hit | 5 | ok |  |
| knee_hit | 6 | ok |  |
| knee_partial | 0 | ok |  |
| knee_partial | 1 | ok |  |
| knee_miss | 0 | ok |  |
| knee_miss | 1 | ok |  |
| knee_miss | 2 | ok |  |
| knee_miss | 3 | ok |  |

### maul

| event | index | verdict | note |
|---|---|---|---|
| hit | 0 | ok |  |
| hit | 1 | ok |  |
| hit | 2 | ok |  |
| hit | 3 | ok |  |
| hit | 4 | ok |  |
| hit | 5 | ok |  |
| partial | 0 | rewritten | "its" in the room line |
| partial | 1 | ok |  |
| partial | 2 | ok |  |
| miss | 0 | rewritten | "its" in the actee and room lines |
| miss | 1 | ok |  |
| miss | 2 | ok |  |
| miss | 3 | ok |  |

### pounce

| event | index | verdict | note |
|---|---|---|---|
| knockdown | 0 | ok |  |
| knockdown | 1 | ok |  |
| knockdown | 2 | ok |  |
| knockdown | 3 | ok |  |
| knockdown | 4 | ok |  |
| hit | 0 | ok |  |
| hit | 1 | rewritten | "raking" dropped; fanged pouncers have no claws |
| hit | 2 | ok |  |
| hit | 3 | ok |  |
| hit | 4 | ok |  |
| partial | 0 | ok |  |
| partial | 1 | ok |  |
| partial | 2 | ok |  |
| miss | 0 | ok |  |
| miss | 1 | ok |  |
| miss | 2 | ok |  |
| miss | 3 | ok |  |

### rake

| event | index | verdict | note |
|---|---|---|---|
| hit | 0 | rewritten | "its" in the actee and room lines |
| hit | 1 | ok |  |
| hit | 2 | ok |  |
| hit | 3 | ok |  |
| hit | 4 | ok |  |
| hit | 5 | ok |  |
| partial | 0 | rewritten | "its" in the actee and room lines |
| partial | 1 | ok |  |
| partial | 2 | ok |  |
| miss | 0 | rewritten | "its" in the actee and room lines |
| miss | 1 | ok |  |
| miss | 2 | ok |  |
| miss | 3 | ok |  |

### shoot

| event | index | verdict | note |
|---|---|---|---|
| hit | 0 | ok | actor and actee only; the room reads `fire_announce` |
| hit | 1 | ok |  |
| partial | 0 | ok |  |
| partial | 1 | ok |  |
| miss | 0 | ok |  |
| miss | 1 | ok |  |
| surprise_hit | 0 | ok | actor only |
| surprise_miss | 0 | ok | actor only |
| fire_announce | 0 | ok |  |
| fire_depart | 0 | ok |  |
| arrival_unknown_hit | 0 | ok |  |
| arrival_unknown_partial | 0 | ok |  |
| arrival_unknown_miss | 0 | ok |  |
| arrival_known_hit | 0 | ok |  |
| arrival_known_partial | 0 | ok |  |
| arrival_known_miss | 0 | ok |  |

### throttle

| event | index | verdict | note |
|---|---|---|---|
| hit | 0 | ok |  |
| hit | 1 | ok |  |
| hit | 2 | ok |  |
| hit | 3 | ok |  |
| hit | 4 | ok |  |
| cast_interrupt | 0 | ok | the mob path now also sends the "chant breaks off" sound to observers who cannot see |
| cast_interrupt | 1 | ok |  |
| partial | 0 | ok |  |
| partial | 1 | ok |  |
| partial | 2 | ok |  |
| miss | 0 | ok |  |
| miss | 1 | ok |  |
| miss | 2 | ok |  |
| miss | 3 | ok |  |

### throw

| event | index | verdict | note |
|---|---|---|---|
| hurl | 0 | ok | player only |
| fumble | 0 | ok | player only |
| cast_interrupt | 0 | ok | player only; "its" names the interrupted mob, not the actor; carries a " -- " dash, left for the copy task |
| partial_hit | 0 | ok | actor only |

### trip

| event | index | verdict | note |
|---|---|---|---|
| tailsweep_knockdown | 0 | ok |  |
| tailsweep_knockdown | 1 | ok |  |
| tailsweep_hit | 0 | ok |  |
| tailsweep_hit | 1 | ok |  |
| tailsweep_partial | 0 | rewritten | "keep their feet" now "stay on their feet"; reads apart from the hit row |
| tailsweep_partial | 1 | ok |  |
| tailsweep_miss | 0 | ok |  |
| tailsweep_miss | 1 | ok |  |
| trip_knockdown | 0 | ok |  |
| trip_knockdown | 1 | ok |  |
| trip_hit | 0 | rewritten | room line "attempts to trip" stated no outcome; every role now states the hit |
| trip_hit | 1 | rewritten | room line stated no outcome; actee now states the hit too |
| trip_partial | 0 | rewritten | "keep their feet" now "stay on their feet"; reads apart from the hit row |
| trip_partial | 1 | ok |  |
| trip_miss | 0 | ok |  |
| trip_miss | 1 | ok |  |
| whiff_on_prone | 0 | ok | mob only; "carries straight past" is left for the retired-idiom task |

## Counts

| | rows |
|---|---|
| Rows checked | 196 |
| ok | 158 |
| rewritten | 33 |
| reordered | 4 |
| authored | 1 |
| Rows fixed | 38 |

Lines rewritten or authored meet the player-copy rules: at most 80 visible
columns with an eight-letter name and "moderate injuries" in place, no
numbers, no dashes, none of the retired idioms. One exception is deliberate:
`drain/hit/3` actee was fixed for the doubled apostrophe only and kept byte
for byte otherwise, as row 58 asks, so it stays at its earlier length.
