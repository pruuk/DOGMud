# Messaging M6 slice 3: beasts, bodies and small pools

**Date:** 2026-10-12
**Issue:** #370 (M6), ledger rows 3, 4, 6, 14, 15, 20, 23, 25, 35 of
`docs/superpowers/audits/messaging-m6-content-ledger.md` (triaged in #498)
**Status:** design approved by the owner in session, 2026-10-12

## Facts verified against source (master after #498, 2026-10-12)

| Fact | Where |
|---|---|
| `howl` hand-rolls six `fmt.Sprintf` lines (pitiful howl, menacing howl hit, howl at a player, turns from prey and snarls, steels yourself, no effect) | `internal/mobcommands/howl.go:15,48-80` |
| Mob `taunt` is the generic conviction attack for tank archetypes (golems, elementals); it narrates from `combat.GetTauntTriad` and falls back to legacy literals when the store is empty | `internal/mobcommands/taunt.go:16-20,107-145` |
| Player `taunt` also narrates from `combat.GetTauntTriad` | `internal/usercommands/taunt.go:20,203` |
| The taunt loader reads every file in `taunt-messages/` keyed by `optionid`, but `GetTauntTriad` hardcodes `tauntMessages["rhetoric"]` | `internal/combat/taunt_messages.go:107,147-148` |
| `rhetoric.yaml` bands: `hit`, `miss`, `critical`, `fumble`; each band holds index-paired `actor`/`actee`/`observer` lists, length-checked by `TauntMessageGroup.Validate` | `_datafiles/world/dogmud/taunt-messages/rhetoric.yaml:16-96`, `taunt_messages.go:37` |
| `species.Species` has no speech field; 42 species files exist (0 ghostly spirit to 99 ascended) | `internal/species/species.go:40-60`, `_datafiles/world/dogmud/species/` |
| No condition flag means silenced; flags are typed `conditions.Flag` constants | `internal/conditions/conditionspec.go:29-110` |
| The mob death line is `"<name> has died."` for readers who make out shapes, `"You hear something collapse to the ground."` otherwise, via `sendVisualElseAudible` | `internal/hooks/Death_MobBroadcast.go:51-55` |
| The player death line lives in `Death_PlayerAnnouncement.go:64-68` | that file |
| A mob corpse is created in `dropMobLootAndSetCorpse`; a player corpse in `wirePlayerCorpse` | `internal/hooks/Death_MobLoot.go:27`, `internal/hooks/Death_PlayerCorpse.go:24` |
| Death hooks run in file-name order, so `Death_MobBroadcast` runs before `Death_MobLoot` | handoff 2026-10-10 trap |
| `cast_started` holds 3 variants | `_datafiles/world/dogmud/casting-messages.yaml:6-9` |
| `deflectedSwingLines` builds one line per defence (dodge, parry, block, default "turns your swing aside") for attacker and defender | `internal/combat/combat_helpers.go:1578-1600` |
| Defence pools: `defense-messages/{dodge,parry,block}.yaml`, `optionid` plus bands `weak`/`normal`/`heavy` | `_datafiles/world/dogmud/defense-messages/` |
| Mob craft start line is fixed text: `"<name> begins working on something."` | `internal/mobcommands/craft.go:65-67` |
| Five unreachable gossip lines under `IronwindEvent-Regional` and `IronwindEvent-Global`; no world event maps to that key | `_datafiles/world/dogmud/gossip_templates.yaml:102-109`, `internal/hooks/MobIdle_HandleIdleMobs.go:389-399` |
| Seven fact descriptions open with a capitalised common word (Boats, Drowned, Taxes, Wolves, Bandits, Folk, Water); #430 lowercases only a leading A/An/The | `_datafiles/world/dogmud/facts.yaml:11,35,59,83,99,108,125`, `internal/gossip/gossip.go:109-175` |
| `counter-parry.yaml` has 45 lines; 22 to 24 name a cut, point, thrust or arm | that file |

## Scope

One PR: ledger rows 3, 4, 6, 14, 15, 20, 23, 25 and 35. Out of scope: row 9
(spell target lines, slice 4), row 7 (recipe room lines) and rows 26 and 27
(combat-message pairing), each their own slice. A Silence spell and condition
that set the new `silenced` flag are #499.

## 1. Speech, silence and the wordless pool (rows 3 and 4)

Owner rulings (2026-10-12): beasts should not taunt with words; speech is a
species property; leave hooks for "use howl instead" and "silenced, so the
command is refused".

- **Species field `speech`** (`bool`, yaml `speech`, default true when
  absent). Set `speech: false` on the species that have no voice: rodent,
  feline, insectoid, fish, carnivorous plant, fungal colony, slime, arachnid,
  worm, dummy, orb, reptile, bat, aberration, mustelid, canine, skeleton,
  zombie, flesh golem, every elemental, automaton, bear, boar, deer, serpent,
  raptor. Speaking species keep the default: ghostly spirit, human, wraith,
  spectre, vampire, troll, goblin, ascended. The plan lists each file.
- **Condition flag `silenced`** (`conditions.Silenced`). Nothing sets it in
  this slice; #499 adds the spell and condition.
- **`characters.SpeechState(c) SpeechState`**, one of `Speaks`,
  `Speechless` (species `speech: false`), `Silenced` (a condition with the
  `silenced` flag; silence wins over species). The single check every taunt
  path asks.
- **Wordless pool** `_datafiles/world/dogmud/taunt-messages/wordless.yaml`,
  `optionid: wordless`, the same four bands and index-paired roles as
  `rhetoric.yaml`, at least six variants per band: snarls, roars, howls,
  hisses, bared teeth, a raised hackle. No line implies words or a specific
  body the species may lack (no "fangs" for a slime).
- **`GetTauntTriad` takes the pool**: a pool name argument (or a sibling
  `GetTauntTriadFrom(pool, ...)`), `rhetoric` by default.
- **`howl`** always narrates from `wordless`, through the same triad seam
  `taunt` uses (names hidden by each reader's sight). Its six literals go.
- **`taunt`** (player and mob): `Speaks` uses `rhetoric`; `Speechless` uses
  `wordless`; `Silenced` refuses. A silenced player reads "You cannot make a
  sound." and pays nothing; a silenced mob skips the taunt that round. The
  refusal happens before any cost is paid.

## 2. A body where it fell (row 15)

Owner ruling: one line after the death line, only when a corpse really
appears.

- Emitted where the corpse is created: `dropMobLootAndSetCorpse` and
  `wirePlayerCorpse`, so a death that leaves no corpse says nothing.
- Readers who see the room clearly read "<name>'s body lies where it fell.";
  shapes readers read "A body lies where it fell."; readers who see nothing
  get no extra line (the collapse sound already told them).
- Sent through the room's visual sight seat so the name is hidden per reader.
  The wording sits beside the existing "has died." literal in Go, matching
  that sibling; death narration moving to a store is M7's question.
- Player corpse names have a known anonymisation gap (`user-corpse` tag, see
  memory `project-corpse-name-leaks-past-anonymize`); the new line names the
  player with the ordinary `username` identity tag, not the corpse tag, so it
  hides cleanly.

## 3. Small pools and lines

- **Cast start (row 6):** `cast_started` grows from 3 to about 10 variants,
  same `{spell}` token, same voice (folds, threads, focus).
- **Deflected swing (row 14):** the wording of `deflectedSwingLines` moves to
  a new `deflected:` section in `dodge.yaml`, `parry.yaml` and `block.yaml`,
  plus a generic fallback section for an empty defence. Each holds
  index-paired `actor` and `actee` lists, at least four variants, tokens
  `{actor}`, `{actee}`, `{damage}`. The current sentences are variant 0. The
  existing defence loader reads the section and validates equal lengths; the
  Go function keeps its selection logic and its contract (both lines name the
  defence and the damage). A missing section falls back to today's text.
- **Mob craft line (row 20):** "begins working on something." becomes
  "begins working on <item>." where the item is the recipe output's name,
  hidden by each observer's sight like the actor name.
- **Ironwind gossip (row 23):** the `IronwindEvent-Regional` and
  `IronwindEvent-Global` blocks are deleted (owner: retire them).
- **Fact lines (row 25):** the seven descriptions are rewritten to start
  lowercase so they read after any lead-in ("Word is, wolves have been
  prowling..."). A test over `facts.yaml` fails on a description that starts
  with a capital unless the word is an article #430 already lowercases or a
  proper noun on a short allowlist (Thornwall, Elgar, Delia today).
- **Parry counters (row 35):** the blade-shaped lines in `counter-parry.yaml`
  are re-toned to fit any parrying weapon (turn, check, drive, strike back),
  keeping index pairing across roles and bands.

## Testing

- `SpeechState`: each of the three states, silence beating a speaking
  species.
- Taunt paths: a speaking mob reads `rhetoric`, a speechless mob reads
  `wordless`, a silenced player is refused with no cost paid, a silenced mob
  skips; `howl` always reads `wordless`.
- Species load: `speech` defaults to true when absent; a shipped speechless
  species loads false.
- Stores: `wordless.yaml` and the `deflected:` sections load and validate
  (equal role lengths, minimum variants, no dashes, tag spacing; the existing
  root guards cover the dirs).
- Corpse line: full sight, shapes and none; a death with no corpse sends
  nothing.
- Facts guard null-probed; goldens re-recorded and read.
- Gate: full `go test ./...`, lint, boot check; a short playtest (a wolf
  howls, a speechless mob taunts, a kill in light and in dim light, a few
  casts).
