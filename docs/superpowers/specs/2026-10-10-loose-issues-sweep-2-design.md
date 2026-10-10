# Loose issues sweep 2 (2026-10-10)

Fourteen more small issues outside the big arcs, in one branch
(`fix/loose-issues-sweep-2`, from master ab027c43d). Nothing here touches the
Messaging M6 slice 1 files (conditions, spell hooks, `characters/combat.go`
and `conditions.go`, `keywords.yaml`, the line-keyed root guards). Refusal and
system wording that M7 will redesign (#264, #261, #283) is left out.

Owner calls 2026-10-10: #286 says "You can't fight X."; #293 drops the spare
copies; #266 drops the waypoint; #253 drops the glyphs.

## Facts verified against source (master ab027c43d)

| Issue | Fact | Where |
|---|---|---|
| #217 | `get all <corpse>` resolves the corpse from the last word only | `internal/usercommands/get.go:194-195` |
| #244 | `party list` lists every charm id, then every companion, so a charmed companion shows twice; `GetInstance`/`LoadRoom` results are used unchecked | `internal/usercommands/party.go:300-345,363-394` |
| #262 | The countered player's own line carries the damage tag from `GetConvictionDamageDescription`, whose bands read "their resolve", "their confidence", "their will" | `internal/combat/counter.go:284-292`, `internal/combat/damage_pipeline.go:172-192` |
| #273 | Mob `emote` passes a leading `@` through; the player command strips it | `internal/mobcommands/emote.go:15-40`, `internal/usercommands/emote.go:55` |
| #277 | Mob `eat` has no spoilage check; the player check is inline | `internal/mobcommands/eat.go:13`, `internal/usercommands/eat.go:50-62` |
| #308 | `give` and `show` help say "The look command"; `sell` and `storage` help lack bulk forms | `templates/help/{give,show,sell,storage}.template` |
| #287 | Two flee refusals and the AI rate-limit line still carry em dashes; the dead lines are already clean | `internal/usercommands/flee.go:18,20`, `main.go:1017`, guard `copy_no_dash_test.go` |
| #346 | `EvaluateBuyRules` reads `ShopGoldReserveRatio` from global config; `PricingConfig` has no field for it; two sibling reads | `internal/shops/buyrules.go:89-94`, `pricing.go:10-16`, `actions/sell_bauble.go:154`, `modules/auctions/npc_buyers.go:176` |
| #266 | Halix's patrol lists room 507, reachable only by a secret locked door | `patrols/ironwind_steppe/steppe_forager_delivery.yaml:14` |
| #286 | `consider` rates any mob, including ones `mobs.CheckPlayerHarm` would refuse | `internal/usercommands/consider.go:15`, `internal/mobs/harm_authorization.go` |
| #293 | Bandit fighter carries spare 20068 and 10026 in `items:`; carried items drop at 100%, equipped ones at `mob.ItemDropChance` | `mobs/*/284-bandit_fighter.yaml:43-45`, `internal/hooks/Death_MobLoot.go:37-58` |
| #339 | `GetBaseCastSuccessChance` has no callers; it is the only reader of `SpellProficiencyCastsPerPoint` | `internal/characters/spells.go:19-50`, `config.balance.go:630`, `config.balance.spells.go:48-49`, `config.yaml:1936-1939` |
| #362 | The helpfile completeness test ranges over `userCommands` only, not module-registered commands | `internal/usercommands/helpfile_completeness_test.go:76-82` |
| #253 | `unicodeToAscii` has no entry for heavy rules, hearts, diamonds, skulls, crossed swords or dashes | `internal/util/util.go:1161-1192` |

## Fixes

- **#217** Resolve the corpse in `get all <corpse>` from the whole name span, the way the single-item path does, so `get all lookout corpse` takes from the lookout. Test with two corpses.
- **#244** Skip companions in the charm loop and nil-check the instance and room lookups. Test: a charmed companion is listed once.
- **#262** Give the countered player a second-person damage tag ("your resolve") while the counterer keeps "their". Test both lines.
- **#273** Strip a leading `@` in mob `emote`, as the player command does. Test.
- **#277** Move the spoilage check into one shared helper; the player keeps its message, a mob silently does not eat spoiled food. Test both.
- **#308** Fix the two "look" copy-pastes; document `sell`'s and `storage`'s bulk forms and cross-link `give`, `show`, `sell`, `storage`. 80 columns.
- **#287** Replace the three dashes; add `flee.go` to the dash guard.
- **#346** Add `GoldReserveRatio` to `PricingConfig`, fill it in `PricingConfigFromBalance`, read it in `EvaluateBuyRules`, and route the two sibling reads through the same value. Test with a non-default ratio.
- **#266** Drop the room 507 waypoint.
- **#286** For a mob you may not harm, `consider` prints "You can't fight X." instead of the odds. Test.
- **#293** Remove the two spare `items:` entries; the equipped copies still drop at the mob's normal chance.
- **#339** Delete `GetBaseCastSuccessChance` and the `SpellProficiencyCastsPerPoint` knob from the two Go config files and `config.yaml` (edited from the committed blob).
- **#362** Include module-registered commands in the helpfile completeness test, and write any help files it finds missing.
- **#253** In ASCII mode, drop ━ ┃ ┏ and the other heavy rules, ♥, ♦, ☠ and ⚔. Em and en dashes become "-" rather than being dropped, because dropping them glues the words either side together. Test.

## Left out

#264, #261, #283 (M7 wording); #241 (only a line order remains, in a line-keyed guard file); #440 (own branch); #249, #236 (M6); everything owner-decision or arc-sized in the triage.

## Testing

Package tests for usercommands, mobcommands, combat, shops, actions, auctions, util, characters, configs and root; the pre-push gate; an isolated boot.
