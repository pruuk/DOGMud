# slam.yaml {itemname} audit

Source read: `origin/master` at 90f707183 (`_datafiles/world/dogmud/combat-messages/slam.yaml`).
Local `master` is stale (eb0d57624, Oct 12) and has an older 479-line slam.yaml; `origin/master`
and the `feature/species-corrections` working tree are byte-identical for this file
(`git diff --stat origin/master HEAD -- slam.yaml` is empty), 407 lines.

## The two kinds of unarmedname

The slam species split into two groups, and that split drives every flag:

- **Body-part nouns, plural:** stone fists, servo-driven fists, pseudopods, spore tendrils, hooves.
- **Attack-name nouns, singular:** watery surge, slicing gust, scorching strike, molten slam,
  abrasive whirl, arcing strike, crystal strike, ember lash.

A frame that reads right for both: `with your {itemname}` / `with their {itemname}`
("with their hooves", "with their molten slam", "with their watery surge").
Frames that fail: `{itemname}` as the object of hurl / smash / drive / flick / commit
("smash your molten slam into", "hurl your hooves"), or as a verb subject (number
agreement breaks between "hooves" and "gust").

Proposed text below writes names as `{actor}` / `{actee}` for width; in the YAML they keep their
existing `<ansi fg="{actortype}">{actor}</ansi>` / `<ansi fg="{acteetype}">{actee}</ansi>` wrapping and
`''` apostrophe escaping. Widths measured by script with 20-char actor and actee, itemname
"servo-driven fists" (longest, 18), stance "aggressive", position "grappling on the ground",
momentum "on the offensive", bodypart "right shoulder" (longest in `combat.bodyParts`).
Every proposed line is 80 or fewer visible columns. Rows marked "companion" have no
`{itemname}` but are rewritten so the record's three roles still describe one moment.

| slam.yaml line | role | intensity/tier | today | proposed | why |
|---|---|---|---|---|---|
| 19 | actor | prepare / beginner | You rear up, preparing to hurl your {itemname} at {actee}. | You rear up, preparing to slam into {actee}. | "hurl your hooves / stone fists / molten slam": thrown. Now matches actee and observer ("preparing to slam"). 57 cols |
| 54 | actor | prepare / master | {momentum}, you assess {actee}''s balance before committing your {itemname}. | You study {actee}''s balance before committing your weight. | "committing your molten slam" wrong; old line was 102 cols. Drops {momentum} (not needed; its lowercase lead was also unpolished). 71 cols |
| 55 | actee | prepare / master | {actor} assesses your balance before committing to the {itemname}. | {actor} studies your balance before committing their weight. | "committing to the pseudopods / hooves" reads as choosing an object. 73 cols |
| 56 | observer | prepare / master | {actor} assesses {actee}''s balance before committing to the {itemname}. | {actor} studies {actee}''s balance before committing. | Same; old line 105 cols. 78 cols |
| 77 | actor | wait / expert | While {position}, you time your breathing, waiting to unleash your full {itemname}. | While {position}, you wait to unleash your full weight. | "your full hooves / full molten slam" wrong; breathing is a body function slime, fungal colony, automaton and elementals lack; old line 104 cols. 68 cols |
| 78 | actee (companion) | wait / expert | {actor} times their breathing, waiting to unleash their full weight. | {actor} waits to unleash their full weight. | Breathing removed to match line 77. 56 cols |
| 79 | observer (companion) | wait / expert | {actor} times their breathing, waiting to unleash their full weight on {actee}. | {actor} waits to unleash their full weight on {actee}. | Same. 80 cols |
| 103 | actor | miss / beginner | You hurl your {itemname} at {actee}, but crash into empty air! | You hurl your bulk at {actee}, but crash into empty air! | Thrown. Observer already says "hurls their bulk". 69 cols |
| 104 | actee | miss / beginner | {actor} hurls their {itemname} at you but crashes into empty air! | {actor} hurls their bulk at you, but crashes into empty air! | Thrown; adds the missing comma. 73 cols |
| 145 | actor | weak / beginner | You glance your {itemname} off {actee}. | You glance off {actee} with your {itemname}. | "glance your scorching strike off" wrong. Now mirrors lines 146-147. 65 cols |
| 180 | actor | weak / master | You flick your {itemname} at {actee}''s {bodypart} with precise restraint. | You tap {actee}''s {bodypart} lightly, with careful control. | "flick your stone fists / molten slam": gripped or flung. "precise restraint" is hard ESL. Old line 98 cols. 76 cols |
| 181 | actee (companion) | weak / master | {actor} flicks their bulk at you while {momentum}. | {actor} taps your {bodypart} lightly, with careful control. | Same moment as 180; "flicks their bulk" is odd for any body. 76 cols |
| 182 | observer (companion) | weak / master | {actor} dances in while flicking bulk at {actee}. | {actor} darts in and taps {actee} lightly. | Same moment as 180. 68 cols |
| 193 | actor | normal / beginner | You smash your {itemname} into {actee}! | You smash into {actee} with your {itemname}! | "smash your molten slam / watery surge into" wrong. 65 cols |
| 194 | actee | normal / beginner | {actor} smashes their {itemname} into you! | {actor} smashes into you with their {itemname}! | Same. 68 cols |
| 195 | observer | normal / beginner | {actor} smashes their {itemname} into {actee}! | {actor} smashes {actee} with their {itemname}! | Same. 80 cols (tight) |
| 206 | actor | normal / expert | You step through and drive your {itemname} into {actee} from your {stance} stance! | You step through and drive your weight into {actee}! | "drive your arcing strike into" wrong; actee and observer already say weight. Old line 105 cols; drops {stance} (not needed, other roles lack it). 65 cols |
| 216 | actee | normal / expert | {actor} feints and crashes their {itemname} home! | {actor} feints and crashes their weight into your {bodypart}! | "crashes their molten slam home" wrong; now matches actor ("crash your bulk into ...'s {bodypart}"). 78 cols |
| 218 | actor | normal / expert | You draw their guard aside and smash your {itemname} home! | You draw {actee}''s guard aside and smash through! | "smash your watery surge home" wrong; now matches actee and observer ("smashes through"). Names the actee instead of a loose "their". 62 cols |
| 240 | actor | normal / master | You crash your {itemname} home and maintain pressure with masterful control! | You crash home with your {itemname}, in masterful control! | "crash your slicing gust home" wrong; old 84 cols. 66 cols |
| 241 | actee | normal / master | {actor} crashes their {itemname} home with masterful control! | {actor} crashes into you with masterful control! | Same; itemname dropped to fit 80. 61 cols |
| 242 | observer | normal / master | {actor} crashes their {itemname} into {actee} with masterful control! | {actor} crashes into {actee} with masterful control! | Same; old 103 cols. 78 cols |
| 253 | actor | heavy / beginner | You smash your {itemname} violently into {actee}! | You smash violently into {actee} with your {itemname}! | As line 193. 75 cols |
| 254 | actee | heavy / beginner | {actor} smashes their {itemname} violently into you! | {actor} smashes violently into you with their {itemname}! | As line 194. 78 cols |
| 255 | observer | heavy / beginner | {actor} smashes their {itemname} violently into {actee}! | {actor} smashes violently into {actee}! | As line 195; old 90 cols, and both names plus itemname cannot fit with "violently". 65 cols |
| 266 | actor | heavy / expert | You step in and drive your {itemname} into {actee} with brutal {stance} force! | You step in and drive brutally into {actee}''s {bodypart}! | "drive your crystal strike into" wrong; now mirrors actee line 267. Old 101 cols. 74 cols |
| 285 | actor | heavy / master | While {position} and {momentum}, you crash forward relentlessly, {itemname} driving deep! | While {position}, you crash forward, your bulk driving deep! | "molten slam driving deep" wrong; actee and observer already say bulk. Old 116 cols. 73 cols |
| 317 | actee | critical / beginner | {actor} CRITICALLY SMASHES their {itemname} into you! | {actor} CRITICALLY SMASHES you with their {itemname}! | "smashes their watery surge into you" wrong; now mirrors actor 316 and observer 318. Keep the `cyan-bold` tag. 74 cols |

28 rows: 24 flagged `{itemname}` roles and 4 companion roles (78, 79, 181, 182).

### `{itemname}` lines that pass for every species

146, 147 ("glances off ... with their {itemname}"), 316, 318 ("CRITICALLY SMASH ... with your/their
{itemname}"), 329, 360 ("OBLITERATE ... with your {itemname}"). All use the `with your/their`
frame. Of these, 147 (84 cols), 318 (91) and 360 (99) are over 80 columns with long names; that
is width, not wording, so they are not in the table. Fix them in the same PR if it covers width.

### Out of scope, noticed in passing (no `{itemname}`)

Body parts some slam species lack: "plant your feet" (22-24), "lower your head" (28-30), "rumble
low in your chest" (67-69), "glancing shoulder" (158-160), "footwork" (74-76, 129-131). Slime,
fungal colony and the air, water, smoke and storm elementals have none of these. Not flagged per
the brief.

The file header (lines 1-12) lists tokens only; it does not describe the record layout and omits
`{bodypart}`, which lines 174-291 use.

## Species checked

`natural_attack: slam` grepped across `C:\tmp\dogmud-species\_datafiles\world\dogmud\species\*.yaml`
(13 files) and cross-checked against the main checkout (12, no horse):

| species file | unarmedname |
|---|---|
| 15-fungal_colony.yaml | spore tendrils |
| 16-slime.yaml | pseudopods |
| 26-horse.yaml (worktree only) | hooves |
| 36-water_elemental.yaml | watery surge |
| 37-earth_elemental.yaml | stone fists |
| 38-air_elemental.yaml | slicing gust |
| 39-fire_elemental.yaml | scorching strike |
| 40-magma_elemental.yaml | molten slam |
| 41-sand_elemental.yaml | abrasive whirl |
| 42-storm_elemental.yaml | arcing strike |
| 43-ice_elemental.yaml | crystal strike |
| 44-smoke_elemental.yaml | ember lash |
| 45-automaton.yaml | servo-driven fists |

read 407 of 407 lines (slam.yaml, chunks 1-400 and 400-407). 30 lines contain `{itemname}` in
records (lines 19, 54, 55, 56, 77, 103, 104, 145, 146, 147, 180, 193, 194, 195, 206, 216, 218, 240,
241, 242, 253, 254, 255, 266, 285, 316, 317, 318, 329, 360), plus the header comment on line 2.
