# DOGMud House Rules

These are the hard rules for code, content and text in DOGMud. Reviewers
check every pull request against them. Each rule is short here; the
`.claude/skills/` file named after it holds the detail and the history behind
it. How to get work merged is in
[`.github/CONTRIBUTING.md`](../../.github/CONTRIBUTING.md).

## 1. Reuse before you build

Before writing a mechanism, find out how the engine already does that job.
The project has spent months merging duplicate systems into one path each, so
a second path is a regression even when it works. If you truly need
something new, say why in the pull request.

| The job | Use this | Not this |
|---|---|---|
| Any opposed contest (attacker against defenders) | `combat.RunContest` | a hand-rolled roll or percent chance |
| A check against a fixed difficulty | `contest.AgainstDifficulty` | `rand` against a threshold |
| The chance a stat or skill improves | `Character.ProgressionChanceForStat`, `Character.ProgressionChanceForSkill` | recomputing the formula |
| Picking a target by name | `actions.ResolveTargetActor` | your own name matching |
| Telling actor, target and room what happened | `messaging.SendTrio` | separate `SendText` calls per viewer |
| Whether one character can see another | `Character.Perceives`, `messaging.SightMult` | your own visibility test |
| Hiding names from those who cannot see | name tags passed through `messaging.Anonymize` | a bare name in `fmt.Sprintf` |
| Whether a shop can trade right now | `actions.ShopSightRefusal` and the shared buy/sell gates | a new merchant lookup that skips them |
| Shop prices | `shops.PricingBaseline` and the shop pricing knobs | a new price formula |
| Saving state that must survive a restart | `util.Save`, `util.ReadLivingState`, `util.QuarantineCorrupt` | `os.WriteFile` or a hand-rolled temp-and-rename |
| Temporary rooms | `rooms.CreateEphemeralRoomIds` and the ephemeral room system | a separate room id allocator |
| Words for damage and healing | `combat.GetDamageDescription`, `combat.GetHealDescription` | printing the number |
| Text a player's API key wrote, shown to others | `baubles.CheckPlayerKeyText` | a looser filter of your own |

When you change one path, change its siblings too. A guard added to the
player `put` command also belongs on the mob `put` command; a gate added to
`buy` also belongs on `repair`. Skill: `dogmud-refactoring`.

## 2. Balance numbers live in config

- Every tuning number (chances, multipliers, durations, prices, caps) is a
  knob declared in `internal/configs/config.balance*.go` and given its
  shipped value in `_datafiles/config.yaml`. Retuning is a config edit, never
  a code change.
- The Go default and the shipped value must make the game work on their
  own. Tests load only the Go defaults, so a knob that defaults to 0 makes
  its feature silently do nothing in tests and on any server whose config
  lacks the key.
- 0 is a legal value for many knobs. Do not write validation that replaces 0
  with a default unless 0 is truly meaningless, and if a knob can switch a
  feature off, 0 must switch it off.
- Prefer multipliers over flat bonuses; they keep working as stats grow.
- Per-building or per-item numbers in content YAML are fine when they are
  content (a house's price), not when they are a global rule.

Skill: `dogmud-balance-config`. Combat numbers: `dogmud-combat`. Stats and
skills: `dogmud-progression-model`.

## 3. Persistence

- Living state (players, shops, guilds, houses, anything a player earned)
  is written atomically with `util.Save`, read with `util.ReadLivingState`,
  and quarantined, never deleted, when it is corrupt. Write it before you
  change the in-memory copy.
- Living state is never committed to git, and never wiped by the
  instance-save cleanup. Add a new store's folder to `.gitignore` and
  `.dockerignore`.
- Anything that holds items must be reachable by the bauble sweep (the
  `bauble_sweep_guard_test.go` failure tells you how).
- Migrations must be safe to run more than once. A run-once marker on a
  character does not protect the bank, which every alt shares, so prefer a
  migration that recognises already-migrated data and leaves it alone. World
  state migrations go in `internal/migration/`.

Skill: `dogmud-persistence`.

## 4. Security and the economy

Assume a player is trying to break your feature, with a second account
helping. Before opening a pull request, check:

- **Gold and items.** Every source of gold or items has a matching sink or
  limit. Nothing can be bought, made or repaired and then sold for more.
  Nothing can be harvested, salvaged or looted twice. A crash or a dropped
  connection at the worst moment does not duplicate or erase anything.
- **Permissions.** Mob, companion and charmed-mob command paths enforce the
  same rules as the player command. Staff-only and admin paths check the
  role.
- **Input.** Player text never reaches a command string, a file name, or a
  template unfiltered. `;` chains commands, and `<ansi>` and `{{` are markup.
  Names and descriptions players choose have a length limit and an allowlist.
- **Load.** Work done every round or every command is bounded. No full scans
  or full YAML marshals of unbounded data per command.
- **Outside APIs.** See CONTRIBUTING, section 4. Off by default, opt-in for
  players, model output treated as player input.

## 5. Text the player reads

- **No raw numbers.** Damage, healing, armour, durations and chances are
  described in words ("serious wounds", "bolsters your defences"), never
  shown as numbers. The `status` sheet is the deliberate exception. Help
  pages follow the same rule; a test enforces it.
- **Wrap at 80 characters.** Room descriptions, help files and templates are
  hard-wrapped near 78 to 80 columns.
- **Plain English.** Part of the audience speaks English as a second
  language. Avoid idioms whose meaning cannot be built from their words
  ("has teeth", "cut to the chase"). NPCs may use idioms as character voice.
- **No em or en dashes** in prose. Use commas, full stops or parentheses.
- **No semicolons in anything an NPC says** through a command; `;` ends the
  command.
- **Combat advice is loadout advice.** Players cannot swap weapons mid-fight,
  so help text says "carry a dagger in your offhand", not "swap to a dagger
  when grappled".
- **Respect sight.** What a player cannot see is not named to them: hidden
  mobs, items in the dark, people they do not recognise.
- **NPC dialogue is first person.** Hints that are not speech are narrator
  text.

Skill: `dogmud-player-copy`. NPC text and quests: `dogmud-authoring-quests`.

## 6. World and flavour

The world bible is [`docs/world.md`](../world.md). New Plymouth canon is in
[`docs/worldbuilding/new_plymouth_canon.md`](../worldbuilding/new_plymouth_canon.md).
The essentials:

- **Medieval, with magic that is really biology.** Technology is medieval
  Europe. "Magic" and mutations come from the Chrysalis, a symbiotic
  organism that makes what people believe physically real. Belief matters
  mechanically and in the fiction.
- **The sci-fi stays hidden.** The three moons (Swiftmoon, the Wanderer, the
  Eye) are really colony ships, and the settlers came from Earth 10,000 years
  ago. Nobody in the world knows this. Old technology appears only as
  misunderstood relics, myths and legends. Never use modern words or
  concepts in world text.
- **Darkness is rare outdoors**, because a moon is usually up. A moonless
  night is the exception, not the norm.
- **NPCs are people.** They follow the same rules as players: same stats,
  skills, progression and mutations. Do not give them special-case powers.
- **Places sound like where they are.** Use the region's materials,
  trades and factions. Check the zone's existing rooms and the canon before
  naming a person, place or faction, and do not reuse an existing
  character's name.

## 7. World content (YAML)

- Run `tools/id_inventory.py` before picking any room, mob or item id.
- A file's name matches its `name:` field, and zone folders use
  underscores; a mismatch panics at boot.
- A colon followed by a space inside ordinary YAML text (idle lines,
  dialogue, noun descriptions) panics the server at boot. Rephrase or use a
  comma; `>` and `|` block fields are safe.
- Exits are cardinal and reciprocal, and room coordinates must not collide
  with any other room in the world.
- Loot comes from mobs and containers, never from items placed on the
  ground through `spawninfo`.
- New content gets a playtest before it ships, not just a clean boot.

Skills: `dogmud-authoring-content`, `dogmud-authoring-quests`.

## 8. Tests

- New code comes with tests, using testify.
- A test that checks a negative ("nothing happened") must be shown to fail
  when the bad thing does happen.
- Pin any value that `config.yaml` can move in the test itself; tests do not
  read `config.yaml`.
- Combat has a fumble rate, so "this attack must land" tests flake. Use the
  patterns in the skill.

Skill: `dogmud-writing-tests`.

## 9. Documentation

- Every package under `internal/` and `modules/` has a `context.md` that
  describes what it is and how to use it. A new package ships one; a changed
  API updates it. Every symbol it names must exist
  (`tools/context_md_audit.py` checks).
- New docs are listed in [`docs/README.md`](../README.md).
- Player-visible changes get a line in [`docs/PATCH_NOTES.md`](../PATCH_NOTES.md).
- New commands get a help page.
