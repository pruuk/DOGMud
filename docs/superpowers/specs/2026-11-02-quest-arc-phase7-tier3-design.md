# #375 phase 7, tier 3: The Confluence

**Status:** design approved in brainstorm 2026-11-02; awaiting owner review of this file. No plan is
written until the owner approves it, and no build starts until the owner has seen the plan.
**Arc:** #375 quest and dialogue rework, phase 7. Roadmap:
`docs/roadmaps/QUEST_DIALOGUE_ROADMAP.md`; target page:
`docs/roadmaps/QUEST_DIALOGUE_REQUIREMENTS.md` (rule 16: pinnacle recipes are sold, not quested).
Tiers 1 and 2: `docs/superpowers/specs/2026-11-02-quest-arc-phase7-tier1-design.md`, `...-tier2-design.md`.
**Quests:** 73, 74 (the Margin mystery); 78 to 87 (Veyra). **Closes:** #397.

Two PRs from one plan: **3a, the Margin** (content only) and **3b, Veyra** (a small engine addition).

## Owner rulings this design rests on (2026-11-02)

Standing: hand-holding ends at the Coulee, so no new bridges. A quest has to make sense as a story. A
step completes on something the player does, so typed nouns become `search`.

New:
1. **Veyra teaches in her talk menu, and #397 closes.** Being taught a recipe and using a recipe scrip
   are the same thing, so she does not need to be a shop.
2. **She teaches the whole secret:** every recipe it needs, components and materials included.
3. **Each pinnacle item is its own menu line**, always listed until you know it.
4. **She checks your skills.** A secret sells only if you have the skill for its recipe and for every
   component recipe. These are overpowered pinnacle items, and the bar stays high.
5. **When you lack the skill, she says so and names the trades,** with no numbers.
6. **She cannot be attacked or baited into attacking,** so nobody can grief by keeping her dead.
7. **Quist offers The Undercroft** at her hand-in, through the existing chain. No new quest.

## Facts verified against source (master `e8d4c29e1`, 2026-11-02)

| Fact | Source |
|---|---|
| These quests carry 51 backlog lines: 73 (3), 74 (7), 78 (14), 79 to 87 (3 each) | `quests/loop-backlog.txt` |
| 41 of the 51 are Veyra's (78 to 87). Nearly all come from purchases being quests: 20 `end`, 10 `hint`, 9 `chain`, 1 `one-action`, 1 `walk-destination` | as above |
| **Veyra's gold check is already fixed.** Offers use `goldRequired` with `chargesGold`, and the validator requires it. An old note in memory said otherwise | `dialogue/the_confluence/9584.yaml:91-93`; `internal/dialogue/world.go:205-207`; `internal/questengine/bridge.go:317-321` |
| Each secret's recipes, with `skill` and `skill_minimum`: parts 50, assembly 65, all `learn_only`. No recipe in any tree has a crafted ingredient outside its own list | parse of `recipes/` and the quest files; table below |
| Craft refuses when `GetSkillLevel(recipe.Skill) < recipe.SkillMinimum`. Gear bonuses count, since `GetSkillLevel` adds them | `internal/actions/craft.go:188-194`; `internal/characters/skills.go:194` |
| `Character.LearnRecipe`, `HasRecipe` exist; `LearnRecipe` returns whether it was new | `internal/characters/spells.go:133`; `internal/actions/craft.go:183` |
| Today's sale: an `offer` with `chargesGold`, whose quest's `quest_granted` trigger teaches | `9584.yaml`; `quests/80-*.yaml` |
| An offer's pitch and Accept/Leave card: `resolve` and `offerCard`. `Apply` charges first and refunds on failure | `internal/conversation/talk.go:255-262,333-349`; `internal/dialogue/topics.go:214-233` |
| A short purse already refuses with "You don't have enough gold for that." | `internal/conversation/talk.go:158-165` |
| Kinds are `info`, `offer`, `handin`; the validator's switch and `TopicMarker` read them | `internal/dialogue/topic.go:8-10`; `world.go:115,135,154`; `topics.go` |
| Veyra is `non_combatant`, `charm_immune`, `hostile: false` | `mobs/the_confluence/9584-*.yaml` |
| `non_combatant` blocks melee and spells through `HarmBlockedNonCombatant`, ranged at `combat_fire.go:294`, and area harm at `mob_area_harm.go:46`. A failed theft's reaction is skipped at `steal.go:685` | `internal/mobs/harm_authorization.go:49`; `internal/actions/`; `internal/hooks/` |
| Retiring a whole quest: every step under `# retired`, read by `RetiredQuests`, dropped from saves on load | `internal/quests/manifest.go:72-77`; `quests/step-ids.txt` |
| Quest 73 has 6 `room_interact` triggers, 74 has 8: a granting one and a grant-nothing flavour one per record | `quests/73-*.yaml`, `74-*.yaml` |
| Sites: 6226, 6131, 6143 (quest 73); 6189, 6188, 6198, 6215 (quest 74, the last gives rubbing 40142) | as above |
| `q74_grant` sits on Cael (9465) gated on `73-end`, which is the `chain` finding. Quist (9441) holds 73's hand-in `report` | `dialogue/the_confluence/9465.yaml:102-121`; `9441.yaml:112` |
| A hand-in whose NPC has exactly one newly opened offer queues it (`queueNextOffer`) | `internal/conversation/nextoffer.go:36-47`; `talk.go:267` |
| Hand-ins off the last step: `aldrichandoff`, `marginhandoff` need `74-descent`; the step before `end` is `reveal` | `9464.yaml:74-91`; `9454.yaml:64-82` |
| A lost rubbing is already recoverable (`q74_recovery`) | `9465.yaml:142-159` |
| NPC lines saying `look <noun>` for these quests: `9431.yaml:40`, `9435.yaml:39`, `9441.yaml:71,81,91,107`, `9444.yaml:40`, `9465.yaml:54,71,80,115,135` | grep of `dialogue/the_confluence/` |
| The harness can seed skills (`grant_skills`) and gold (`set_gold`). The veteran profile holds 74, 78, 81, 82 | `internal/playtestenv/types.go:112-121`; `tools/playtest/profiles/veteran.yaml:1218-1221` |

## 3b: Veyra

### What the player sees

`talk veyra` lists one line per secret the player does not know yet, labelled "Secret of the Blackrazor" and
so on. Choosing one:

| The player has | What happens |
|---|---|
| Not the skill for every recipe | The player's line, then her "come back" line naming the trades. No card, no price. The line stays on the menu |
| The skill | Her pitch, then a card with the price. Accept charges and teaches every recipe in the secret |
| The skill, not the gold | The same pitch and card. Accept says "You don't have enough gold for that." and charges nothing |
| Every recipe already | The line is not there |

A sale the player qualifies for shows `[*]`. One they cannot afford yet still does, since they can read its
price. One they lack the skill for shows no marker.

### The trades and lines

The `notReadyText` for each, in her voice, naming every trade and no numbers. Prices and pitches stay as
shipped, except each pitch's "Pay, and I teach you" wording stays true.

| Key | Secret | Price | Recipes | `notReadyText` |
|---|---|---|---|---|
| `teach_bandolier` | Vitalis Bandolier | 17,500 | `reinforced-harness`, `preservation-runes`, `assemble-vitalis-bandolier` | Come back when you've practiced at the loom, the enchanting circle and the alchemy bench. |
| `teach_blackrazor` | The Blackrazor | 25,000 | `hungering-guard`, `obsidian-edge-resin`, `assemble-the-blackrazor` | Come back when you've practiced at the jeweler's bench, the alchemy bench and the forge. |
| `teach_wayfarer` | Wayfarer's Bottomless Pack | 12,500 | `reinforced-frame`, `spatial-stitching`, `assemble-wayfarers-pack` | Come back when you've practiced at the forge, the enchanting circle and the loom. |
| `teach_aegis` | Aegis of Mockery | 20,000 | `voice-amber-housing`, `resonance-lacquer`, `assemble-aegis-of-mockery` | Come back when you've practiced at the jeweler's bench, the alchemy bench and the forge. |
| `teach_thornwall` | Thornwall Harness | 15,000 | `barbed-spike-plates`, `anti-corrosion-quench`, `assemble-thornwall-harness` | Come back when you've practiced at the forge, the alchemy bench and the loom. |
| `teach_prism` | Seething Prism | 20,000 | `containment-lattice`, `nutrient-suspension`, `assemble-seething-prism` | Come back when you've practiced at the enchanting circle, the alchemy bench and the jeweler's bench. |
| `teach_zephyr` | Zephyr Treads | 12,500 | `quicksilver-soles`, `windlace-bindings`, `assemble-zephyr-treads` | Come back when you've practiced at the alchemy bench, the enchanting circle and the loom. |
| `teach_choir` | Staff of the Hollow Choir | 22,500 | `conductor-core`, `choir-focus-gems`, `assemble-hollow-choir-staff` | Come back when you've practiced at the forge, the jeweler's bench and the enchanting circle. |
| `teach_phial` | Phial of Second Birth | 15,000 | `reduction-base`, `assemble-phial-of-second-birth` | Come back when you've practiced at the cooking fire and the alchemy bench. |

Each of today's bought lines from a quest ("Paid. The recipes are in your hands...") becomes the sale's
accept line, `paidText`, keeping its words.

### The engine addition

A new topic kind, `sale`, going through the offer's existing path.

```yaml
- key: teach_blackrazor
  kind: sale
  label: Secret of the Blackrazor
  playerLine: Will you teach me the Blackrazor?
  text: The Blackrazor. Not a blade I'd show just anyone...   # shipped pitch
  hints: Accept to pay Veyra's price and learn the recipes...
  notReadyText: Come back when you've practiced at the jeweler's bench, the alchemy bench and the forge.
  paidText: Paid. The recipes are in your hands. Forge it at my stations when you are ready.
  chargesGold: 25000
  teachesRecipes: [hungering-guard, obsidian-edge-resin, assemble-the-blackrazor]
```

- **`Effects.TeachesRecipes`:** applied in `Apply` after the charge, through a new `PlayerState.LearnRecipe`
  backed by `Character.LearnRecipe`. A failed charge teaches nothing.
- **`PlayerState`** gains `KnowsRecipe(id) bool` and `RecipeSkillMet(id) bool`, the second the same test as
  `craft.go:191`. Nil fails closed.
- **`Eligible`:** a `sale` topic is hidden when `KnowsRecipe` is true for all its recipes. Skill and gold do
  not hide it. A player who knows some recipes, but not all, still sees it and pays the full price; no
  shipped path can reach that state.
- **`resolve`:** a `sale` with every `RecipeSkillMet` plays pitch and card, as an offer does, with the card's
  body "Price: 25,000 gold." On a missing skill it plays `playerLine`, then `notReadyText`, and re-shows the
  menu.
- **`TopicMarker`:** `[*]` when every skill is met. Otherwise none.
- **Validator:** a `sale` needs at least one recipe, all of which exist; `chargesGold` above zero;
  `notReadyText`; `paidText`; `playerLine`. The `goldRequired` rule does not apply to it, since Accept is the
  gate. `teachesRecipes`, `notReadyText` and `paidText` are refused on other kinds, and `grantsQuest` is
  refused on a `sale`. All texts get the dash and `;` checks.
- **Other paths:** the companion path reads only `info`, `queueNextOffer` only `offer`, so neither changes.
  The walkthrough and loop rules read `offer` and `handin`, so a `sale` raises no finding.
- **The offer path is kept as it is for quests.** `offerCardBody` and Hook 3 are untouched.

### Her gate and the rest of her file

- Quests 78 to 87 retire whole. Every step id moves to `# retired`, and their 41 backlog lines go.
- `convergence_intro` becomes an ungated `info` topic keeping her lore about six trades, reworded so it does
  not promise she will know you. Its `breadcrumb` goes.
- `secrets_list` moves to `# retired` in `topic-keys.txt`. The menu is the list.
- Opener: the base line stays. The `77-end` variant stays with its gate on `77-end` alone. The masterwork
  variant stays as flavour, gated on `masterworkRequired: 50`, with no `78` gates. The two `78-end` variants
  go.
- `gold` and `confluence` lose their `78-end` gates, and `gold`'s text loses "I don't sell to hands I haven't
  seen work", replaced by "I teach hands that have earned it."
- `internal/guards/veyra_secrets_dialogue_test.go` is rewritten for the sales.

### Old saves

- A character who bought a secret knows its recipes, so that line does not show. They never pay twice.
- A character known to her (`78-end`) loses only the log row. The skill check now decides.
- A character carrying quests 79 to 87 "in progress" loses those log rows and keeps every recipe. Those quests
  pay no reward, so nothing of value is lost.
- Veyra's line when a piece is finished goes with the quests. That is accepted.

### She cannot be griefed

Nothing in her file needs to change. A guard locks it in: `TestVeyraCannotBeHarmed` loads the shipped mob and
asserts `non_combatant`, `charm_immune`, `hostile: false`, `HarmBlockedNonCombatant` from the harm check, and
no `combatcommands`. A playtest seat tries melee, a spell, a thrown item, a shot, an area attack, a failed
theft, and `taunt`, and reports whether she ever retaliates or takes damage.

## 3a: The Margin

### Quests 73 and 74 by `search`

Each record's granting trigger becomes `event: command, command: search, room: N`, keeping its `has`,
`missing`, `grant`, `actor` and `observer`. The grant-nothing flavour triggers go, since the room nouns
describe the things to anyone. The `give_item: 40142` stays on the 6215 trigger.

| Step reached | Room |
|---|---|
| `73-chart` | 6226 |
| `73-scrivener` | 6131 |
| `73-map` | 6143 |
| `74-ledger` | 6189 |
| `74-record` | 6188 |
| `74-survey` | 6198 |
| `74-reveal` | 6215 |

The NPC lines in the facts table become "Type search". Each room's noun text must keep describing the thing
and must not claim a find; the plan reads each one.

### Quist offers The Undercroft

A new offer on Quist, `undercroft_offer`, gated on `73-end`, with `74-start` and `74-end` excluded. At the
hand-in, `queueNextOffer` pitches it.

> Label: The Undercroft. Player: "Where does this lead?"
> Quist: "Under the temple, if you have the stomach for it. Brother Cael keeps the archive on the island, and
> he has been reading the same records alone for years. Tell him the Margin sent you. There are three of the
> temple's own that disagree with its founding story, and he will show you where."

Hint: "Accept, then go to Brother Cael in the Archive on the temple island."

- `q74_grant` moves to `# renamed` in `topic-keys.txt` as `9465.q74_grant -> 9441.undercroft_offer`, if the
  manifest allows a rename across mobs; otherwise `# retired`.
- Cael's text from it becomes a new info topic, `q74_work`, gated on `74-start`, excluding `74-ledger`, marked.
  Its first sentence is unchanged, and it ends "Then come back to me."
- A save at `73-end` without 74 now finds the offer on Quist. Cael's opener variant for that state, if any,
  points there. A save on any 74 step is unaffected.
- Step `start`'s destination stays on Cael.

### Hand-ins and hints

`aldrichandoff` and `marginhandoff` require `74-reveal`. `reveal` gains `destination: {mob: [9464, 9454]}`.

| Step | Hint |
|---|---|
| 73 `start` | First, the damaged chart in the bookseller's shop. From the Margin Hall, go east twice to the Scholars' Gate, then south into the Bookseller's. Type search. |
| 73 `chart` | Next, Tallis's old charts at the Scriveners' Stall, west of the Weighhouse Yard, which is north of the North Cross. Type search. |
| 73 `scrivener` | Last, the marginal note on the great map in the Municipal Hall, south of the Hall Steps, just west of the North Cross. Type search. |
| 74 `start` | First, the building ledger open on the stand in the Archive, beside Brother Cael. Type search. |
| 74 `ledger` | Next, the consecration record in the Chapter House. From the Archive, go west to the North Walk, then west into the Chapter House. Type search. |
| 74 `record` | Last, the mason's survey in the Older East Corridor, south of the Prioress's Oratory, east of the Cloister Garth. Type search. |
| 74 `survey` | Return to Brother Cael in the Archive, east of the North Walk. He sleeps in the archive chair late at night. Type talk cael. |
| 74 `descent` | The Descent Stairhead is east of the Older East Corridor. Go down, west three times, down, west twice, down, then west three times to the Chamber of the Old Sky. Type search. |
| 74 `reveal` | Talk to Aldric and to the Margin scholar before you choose who gets the rubbing. Aldric keeps the Chapter House by day. The scholar is at the Temple Portico. Type talk aldric. |

Tier 2's playtest showed a hint short of a route costs real moves, so each keeps enough to find the place. The
plan checks every direction against the room files.

## Item audit

Clean. Chart, charts and note are records read in place. The rubbing is taken at the site, with a recovery
topic. The fair copy is a reward. Nothing lies loose with a keeper beside it. Reagents for the secrets are out
of scope.

## Testing

- **The ratchet:** all 51 backlog lines deleted with the sweep green, and no line added. A guard holds these
  quests at none, with a fixture proving it can fail.
- **`sale`, in `internal/dialogue`:** Eligible hides only when all recipes are known; the validator's rules,
  each with a malformed fixture; a refused charge teaches nothing; skill checks on every recipe, not just one.
- **`sale`, in `internal/conversation`:** a missing skill plays `notReadyText` and opens no card; the skill
  met opens the card; Accept charges and teaches; a short purse refuses and teaches nothing.
- **Shipped content:** every secret's recipes match the table; each `notReadyText` names exactly the trades of
  its recipes (stations read from the recipe files); no number in a `notReadyText`.
- **Retired quests:** 78 to 87 are in `RetiredQuests`, and no file in `quests/`, `dialogue/` or `behaviors/`
  references their tokens.
- **Old save:** a character with `80-start` and its recipes sees no Blackrazor line.
- **Quests 73 and 74:** no `room_interact`; each site's trigger is `command: search` in the right room; the
  chain from Quist has exactly one next offer; the hand-ins need `74-reveal`.
- **`TestVeyraCannotBeHarmed`.**
- **Gates before push:** `gofmt`, `go build`, `go test ./...`, `DOGMUD_BOOT_SMOKE=1` root package,
  `golangci-lint`, isolated boot.

## Playtest

Four seats, profile `mid`, on a fresh server. Each PR gets the seats that touch it.

| Seat | Fixture | Must show |
|---|---|---|
| Margin | `mid`, in the Margin Hall | 73 and 74 from Quist to one ending, by `search`, on hints and the map. The next offer comes up at the hand-in. Lost rubbing recovered |
| Master crafter | `grant_skills` to meet every trade, `set_gold` 100,000 | Buys the Blackrazor: pitch, card, recipes in `craft list`, line gone. Tries one too dear after spending. Forges nothing |
| Half skilled | `grant_skills` to meet two of the Blackrazor's three trades, `set_gold` 100,000 | Blackrazor refuses and names the trades; one it qualifies for sells; no card on a refusal |
| Griefer | `veteran` | Every way of harming or baiting Veyra listed above, with the server's logs checked for her health |

## Out of scope

- Shops selling recipes in general, and trainers.
- Prices and pitches: unchanged, and retuning belongs to the economy work.
- Reagent sources, #668 and #657.
- Quests 75 to 77 (tier 4).
- Any new bridge or hub.

## Done means

All 51 lines gone (128 to 77), no `room_interact` in 73 or 74, requirement 16's checkbox ticked, both PRs merged,
the playtest ran, the roadmap names tier 3 as done, and #397 closed. `docs/PATCH_NOTES.md` carries a dated entry.
`internal/dialogue/context.md` and `internal/conversation/context.md` describe `sale`, and the
`dogmud-authoring-quests` skill gains it. Findings outside this pass go to issues.
