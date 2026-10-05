# PR #208 review: ai companions, unification lens

Blind reviewer `ai_companions:unification`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Unification/reuse review of the AI companion overhaul (Hollow, roster, parting, finds, combat styles, trades). Mostly the contributor built on the module's own plumbing and the engine's seams. Five places drift from or copy existing mechanisms. (1) The new Hollow model path copies the road speech pipeline but drops its output moderation. With the documented RequirePlayerKey=false setting, speech bought on the server key and prompted by a stranger is broadcast with no moderation and no log. (2) The roster's keptProgress, progressOfMob and applyProgress are self-declared copies of the hooks package's snapshotCompanionProgression and applyCompanionState. They already differ: skills are merged, and the legacy-schema conversion is missing. (3) craftableHere and craftableElsewhere copy InitiateCraft's gate chain by hand and have already drifted: the tool gate that later wilderness-trades commits in this same PR added is missing. (4) The companion's salvage and butcher actions go through the mob salvage path. That path lacks the CarcassTable gate the player salvage command now has, so a companion strips carcasses with flat legacy salvage where a player must skin and butcher, and its "is game" test hardcodes legacy salvage tags. (5) New tuning knobs (CompanionBaubleChance=15, IdleTradeSeconds, HollowSignRoom) ship as Go defaults only and are absent from config.yaml.

Correct reuse worth noting:
- The roster and minds persist through the plugin store (plug.ReadIntoStruct/WriteStruct, with util.ErrStateAbsent and quarantine), not bespoke file I/O. Items are kept out of the roster file on purpose, so the bauble sweep has nothing new to walk.
- The companion bauble roll goes through the shared searchBaubleRoll with the owner's UserId, so it draws on the owner's per-room ration, skill factor, sight penalty and household rules. Delivery goes through startBaubleDelivery, and only model-safe ModelName text reaches the model.
- New engine seams follow the existing companionai Set/Route pattern (RouteShow, RouteSearched, RouteBaubleFound, BaubleSearchFor).
- Every companion act is an ordinary mob command (cast, attack, sneak, craft, salvage, search), so the engine handles rolls, cooldowns and costs. castReady uses actions.SpecialMoveReady.
- Crafting reuses crafting.HasIngredients, CheckOwnComponents, IsEnchantingRecipe, actions.StationSatisfied and actions.TooDarkToCraft.
- The corpse check uses crafting.LookupCorpseSalvageForMob, and corpse targeting uses the existing mobId:round salvage targeting.
- The Hollow waiter uses the existing mob flags (NonCombatant, CharmImmune, PlayerAttackImmune, MaxWander) and IsEssential through a group tag. Hearing uses Character.Perceives.
- Opinion changes go through the module's existing boundDelta/applyOpinion envelope machinery. Model calls use the existing route, reserve and settle budget plumbing.
- factionWords and describePerson are pre-existing module helpers, not new.

Outside this lens, for the consent reviewer: hollowMayTalk (hollow.go:319-342) silently records Consented=true when a player who has read the sign speaks in a Hollow room. consentByCompanionship (commands.go bondTo; roster.go:108 consentHolders) records consent for every roster holder, including admin-granted ones.

## Coverage

Read in full or in the relevant parts, at PR head C:/tmp/dogmud-pr208-review: modules/aicompanion/hollow.go (whole file), roster.go (whole file), finds.go, combat_style.go, cooking.go (craft and trade parts), and the config.go diff. Also the diffs of actions.go, autonomy.go, loot.go and worldmap.go; internal/actions/search.go and search_bauble.go; internal/mobcommands/search.go and salvage.go; internal/companionai/search.go; internal/behaviortree/conditions_player.go and actions_archer.go; internal/mobs/mobs.go (IsEssential); and usercommands/show.go. Compared against: internal/hooks/companion_bonded.go and PlayerSpawn_HandleJoin.go (snapshot/apply), actions/craft.go InitiateCraft, actions/salvage.go, usercommands/salvage.go, crafting/corpse_salvage.go, the baseline dismiss.go and commands.go bondTo, runtime.go dispatch/speak/moderation, and the bauble sweep header. I verified that factionWords existed at the baseline, so it is not reported. Not read in depth: parting.go beyond its function list, meeting.go, prompt.go, profile.go, conversation.go, combat.go, the profile YAMLs (except crafts/archetype for hal), the six companion_* archetype YAMLs (I compared only archer against the existing archer.yaml), the room YAMLs 6880-6885, the help templates, docs and tests. I did not trace whether Hollow waiter mob instances persist across restart (possible duplicate spawn). Consent observations are outside this lens and appear only in the summary.

## Findings (5)

<a id="f032"></a>
### F032 [medium] craftableHere hand-mirrors InitiateCraft's gate chain and has already drifted (no tool gate)

`modules/aicompanion/cooking.go:69` · status **confirmed** · reported as medium

To 'offer nothing the attempt would refuse', craftableHere and craftableElsewhere re-list InitiateCraft's checks one by one: dark, station, ingredients, skill minimum, own components, enchanting, name resolution. InitiateCraft at the PR head also has a tool gate (ToolSatisfied, wilderness trades), which neither copy checks. So the list the model sees and the list autonomy picks from now include recipes the engine refuses with MissingTool. The comment on craftableHere still claims 'the same three tests the player craft command applies'.

**Failure scenario.** Hal (profile crafts: cooking, tailoring; tailoring drive 0.2) stands at a loom carrying hides, with tailoring rank at least the recipe minimum but no scraper. craftableHere lists cure-hide (station: loom, tool: scraper). pastime adds the `craft` option at weight 2+int(0.2*10)=4, and startCraft issues `craft cure hide`. InitiateCraft returns MissingTool, so nothing is made. The prompt also lists the recipe as makeable, so the model can choose it and fail the same way.

**Existing mechanism.** internal/actions/craft.go InitiateCraft (gate sequence) and actions.ToolSatisfied

**Suggested fix.** Extract InitiateCraft's pre-start gates into one exported predicate (for example actions.CraftBlocked(char, room, recipe) returning the CraftResult reason) and call it from InitiateCraft, craftableHere and craftableElsewhere, so a new gate cannot be missed again.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
cooking.go:66-121 checks TooDarkToCraft, StationSatisfied, HasIngredients, SkillMinimum, IsEnchantingRecipe, CheckOwnComponents, craftsAs, with no ToolSatisfied (grep for ToolSatisfied in modules/aicompanion returns nothing). actions/craft.go:209-214 `if !ToolSatisfied(char, recipe) { res.MissingTool = true; return res }`. _datafiles/world/dogmud/recipes/tailoring/cure-hide.yaml: `station: loom`, `tool: scraper`. 10 tailoring, 11 jewelcrafting and 12 woodwork recipes carry `tool:`. hal.yaml:254-256 crafts cooking and tailoring.
```

- **confirmed** (medium): The claim holds. InitiateCraft at the PR head (internal/actions/craft.go:209-214) refuses with MissingTool when `!ToolSatisfied(char, recipe)`. craftableHere (modules/aicompanion/cooking.go:69-124) checks dark, station, ingredients, skill minimum, enchanting, own components and name resolution, but has no tool check. Nothing in modules/aicompanion calls ToolSatisfied: a grep across the repo finds it only in internal/actions, internal/hooks and internal/usercommands. The tool gate itself came in this PR's wilderness-trades commits (git log c696c117a..HEAD on craft.go). So the PR added a gate to InitiateCraft and its own hand-copied gate list in the companion module did not follow. That is the drift and the unification problem the claim describes.  craftableHere feeds four places: - the prompt list (runtime.go:580) - the model's [k] action lookup (actions.go:337) - the pastime `craft` option and its handler (autonomy.go:330 and 436) - tradeAtHand (cooking.go:256)  So all of these can choose a recipe the engine then refuses. The engine already has a single tool rule, ToolSatisfied, whose doc comment calls itself "the one statement of the rule". The player craft list and the ready/locked buckets use it (usercommands/craft.go:284, 462, 712). The companion copy skips it.  One detail in the scenario is wrong: cure-hide's station is `tanning_rack`, not `loom`. That does not change the defect. The recipe needs a scraper, has skill_minimum 0 and is not LearnOnly, so seedRecipes teaches it to Hal (crafts: cooking and tailoring, tailoring 0.2). At a tanning rack with a hide and a salt pouch but no scraper, Hal is offered it and the engine refuses it.  The impact is limited. A refused attempt makes nothing and does no damage, and the lastCraft and lastPastime timers throttle retries. The result is wasted turns, a misleading recipe list in the prompt, and an out-of-date comment. Medium fits the "rewrites an existing mechanism and has already drifted" framing.

</details>

<a id="f033"></a>
### F033 [medium] Companion salvage/butcher bypasses the carcass harvest gate that player salvage now enforces

`modules/aicompanion/finds.go:123` · status **confirmed** · reported as medium

The companion's `salvage` action and its `salvage`/`butcher` pastimes butcher through the mob command `salvage <mobId>:<round>`, which reaches actions.Salvage -> salvageCorpse. At the PR head the player salvage command refuses any corpse with a harvest table (actions.CarcassTable) and sends the player to skin/butcher/harvest, which carry tool, skill, grading and spoilage rules. The mob path has no such gate, so a companion still turns carcasses into the flat legacy LookupCorpseSalvageForMob returns. butcherable also decides what counts as game by hardcoding the legacy salvage tags `raw-meat` and `wild-hare-meat`, instead of asking the harvest tables that now define carcass yields. Companion butchering and player butchering are two different systems for the same body.

**Failure scenario.** A player kills a deer. Its species has a harvest table, so `salvage deer` tells them to skin and butcher it with a knife. Their companion with a butcher pastime weight (or the model choosing `salvage`) issues `salvage <deerMobId>:<round>`. actions.Salvage runs the old table and yields raw-meat x1, leather-strip x2 and sinew x1 with no tool, no skinning skill and no grade, then destroys the corpse. The player gets carcass materials that the trades system meant to gate, simply by letting the companion do it.

**Existing mechanism.** internal/actions/harvest.go CarcassTable plus the skin/butcher/harvest path (internal/usercommands/carcass.go, actions harvest); the gate at internal/usercommands/salvage.go:163

**Suggested fix.** Move the CarcassTable refusal into actions.salvageCorpse so every actor obeys it. Have the companion butcher through the harvest action (a mob `butcher` command over the same actions code), and make butcherable test whether the carcass has a harvest table rather than matching legacy salvage tags.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
finds.go:134 `returns := crafting.LookupCorpseSalvageForMob(...)`; finds.go:145 `if r.ItemTag == \`raw-meat\` || r.ItemTag == \`wild-hare-meat\``; finds.go:173-175 salvageCommand `salvage %d:%d`; actions.go:525-548 and autonomy.go:424-433 issue it; mobcommands/salvage.go:42-50 -> actions.Salvage(TargetCorpse); grep shows CarcassTable is used only in actions/harvest.go, usercommands/carcass.go and usercommands/salvage.go:163, not in actions/salvage.go. usercommands/salvage.go:160-167 refuses carcasses with a harvest table.
```

- **confirmed** (medium): The claim checks out against the code. This PR adds the carcass gate to the player salvage path: usercommands/salvage.go startCorpseSalvage now refuses any corpse where actions.CarcassTable returns a non-empty table, and sends the player to skin/butcher/harvest instead. The PR also adds the companion salvage paths: finds.go is a new 194-line file, and actions.go and autonomy.go now issue `salvage <mobId>:<round>`. Those paths reach mobcommands.Salvage, then actions.Salvage(TargetCorpse), then salvageCorpse. That function picks its target and rolls its yield purely from crafting.LookupCorpseSalvageForMob, with no CarcassTable or harvest check. A grep of non-test files finds CarcassTable only in actions/harvest.go, usercommands/carcass.go and usercommands/salvage.go:163, so the mob path has no gate. butcherable also decides what counts as game by hardcoding the legacy return tags `raw-meat` and `wild-hare-meat`, not by asking the harvest tables. Many species YAMLs ship harvest tables (canine, feline, rodent, reptile and others), so the gate is live for real carcasses that a companion can still salvage the old way. One small point against the impact: storeRecovered puts the materials in the companion's own inventory, not the owner's, so the owner gets them only if the companion hands them over. The inconsistency and the bypass of the trades gating still stand, and the PR created both, so medium is right.

</details>

<a id="f034"></a>
### F034 [medium] Hollow interview path copies the speech pipeline but skips output moderation (ModerateOutput) on server-key calls

`modules/aicompanion/hollow.go:643` · status **confirmed** · reported as medium

dispatchHollow/applyInterview add a second model-call-to-speech path next to runtime.go dispatch/speak. The road path parses and moderates every server-key reply off the game loop. When the speech was prompted by someone other than the owner it uses the strict rule (do not speak if the check cannot be made). The Hollow path calls m.callModel and then parses with parseJSONContent under the lock. It never calls moderateDecision and goes straight to hollowSpeak, which is a line-for-line copy of speak. Every Hollow visitor is a stranger to the companion, which is exactly the case the road path treats as strict. The board text has a dedicated variant (hollowSignServerKey) advertising the server-key mode, so RequirePlayerKey=false is a supported configuration. In that mode the route is routeServer, and logSpeech only logs routeRelay lines. Hollow speech is then neither moderated nor logged.

**Failure scenario.** An operator sets RequirePlayerKey: false (the documented 'let the server's key pay as before' mode) with ModerateOutput: true. A keyless player walks into a Hollow room and coaxes the companion into abusive text. dispatchHollow routes it on the server key, applyInterview -> hollowSpeak issues `say ...` to the whole room with no moderation call, and logSpeech writes nothing for routeServer. On the road the same reply would have been dropped by moderateDecision(strict=true).

**Existing mechanism.** modules/aicompanion/runtime.go dispatch (moderation block, lines 656-663 and 745-756) and AICompanionModule.moderateDecision (openai.go:335); runtime.go speak/spokenLines

**Suggested fix.** Route the Hollow reply through the same parse-and-moderate step as dispatch, off the lock and with strict=true, since the visitor is never the owner. Factor speak's chunking loop into one helper that both speak and hollowSpeak call.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
runtime.go:660 `moderation := m.cfg.ModerateOutput && rt.kind == routeServer`; runtime.go:751-752 `if moderation { res.Moderated = m.moderateDecision(ownerId, &d, ..., strictModeration) }`; grep shows moderateDecision (openai.go:335) is called only from runtime.go. hollow.go:643 `res := m.callModel(call)` then applyInterview -> hollow.go:700 `m.hollowSpeak(w, mob, mind, u, d.Speech, rt)`. hollow.go:813-844 duplicates runtime.go:1162-1206 (speakInChunks, maxSayChunks, delay formula, addLine/addOwnPhrase). runtime.go:1143 speechLogLine returns nil unless rt.kind == routeRelay. hollow.go:385-389 hollowSignServerKey text.
```

- **confirmed** (medium): The claim reproduces from the code. hollow.go is new in the PR (diff vs c696c117a: +1346 lines). dispatchHollow lets keyless visitors through when RequirePlayerKey is false (hollow.go:568), and builds the call through applyRoute. When the visitor has no live relay, route() (tiers.go:278-279) returns routeServer whenever Enabled, a server API key is set and RequirePlayerKey is off. The goroutine calls m.callModel(call) (hollow.go:643). applyInterview then parses with parseJSONContent, runs sanitizeDecision and goes straight to m.hollowSpeak(...) (hollow.go:700). Nothing on that path calls moderateDecision or moderate. A grep shows moderateDecision is called only at runtime.go:752, under `moderation := m.cfg.ModerateOutput && rt.kind == routeServer` (runtime.go:660) with strict set when the asker is not the owner (runtime.go:664). hollowSpeak calls logSpeech, and speechLogLine returns nil unless rt.kind == routeRelay (runtime.go:1143). Its own comment says server-key lines go unlogged because they "passed moderation", which is not true for Hollow lines. hollowSpeak duplicates the speak loop: speakInChunks, maxSayChunks, the delay formulas, addLine and addOwnPhrase. So in server-key mode, Hollow speech is neither moderated nor logged. That mode is supported and advertised to players by hollowSignServerKey (hollow.go:385-389, 402). The impact is limited because the shipped config.yaml:2781 and the Go default (config.go:273) both set RequirePlayerKey: true. That makes the gap latent in the default deploy but real in a documented mode. Medium stands.

</details>

<a id="f131"></a>
### F131 [medium] Roster progress snapshot/apply is a hand copy of the hooks' companion progression code, already diverging

`modules/aicompanion/roster.go:443` · status **refuted** · reported as medium

keptProgress, progressOfMob and applyProgress re-implement internal/hooks snapshotCompanionProgression and the progression half of applyCompanionState. The comments say they 'mirror' those functions and 'must be kept in step with it'. That makes three hand-synced copies of the stat-training and skill-map transfer: saveCompanionState, snapshotCompanionProgression and this one. It already diverges in two ways. applyProgress max-merges template skills where the engine replaces them. applyProgress ignores SchemaVersion and never calls mobs.LegacyTrainingToGains, while progressOfInfo faithfully copies a possibly-legacy ci.SchemaVersion into the roster. Any progression field added to CompanionInfo later, or any change to the legacy conversion, has to be found in this module too or the roster silently drops it when the companion changes owners.

**Failure scenario.** keepFrom (runtime.go:1401-1403) or sendBack (roster.go:336) stores progressOfInfo(comp) for a companion whose record predates U10b-0 Phase C (SchemaVersion < InstanceSchemaVersion, so Training still holds the template and spawn pool). bondTo/spawnWaiting then call applyProgress, which assigns StatTraining raw onto a NewMobByIdFresh mob. That double-counts the authored stats, the exact 'roughly double the pet' bug applyCompanionState guards against. Separately, a new progression field added to CompanionInfo and snapshotCompanionProgression is lost whenever she moves owners, because keptProgress does not have it.

**Existing mechanism.** internal/hooks/companion_bonded.go snapshotCompanionProgression; internal/hooks/PlayerSpawn_HandleJoin.go applyCompanionState; the companionai seam pattern (companionai.SetRespawner/SetSnapshotter installed in hooks.go)

**Suggested fix.** Store kept progress as a characters.CompanionInfo (progression fields only), or expose the hooks pair through companionai the same way RespawnBondedCompanion is installed. Then progressOfMob and applyProgress become calls to the one implementation, and the legacy conversion comes for free. Keep the template-skill floor as a small step applied after the shared apply.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
roster.go:440-442 `// progressOfMob ... mirrors the engine's snapshotCompanionProgression (internal/hooks) and must be kept in step with it.`; roster.go:497-499 `It mirrors the progression half of the engine's applyCompanionState ... no legacy conversion is needed`; roster.go:464-474 progressOfInfo copies `SchemaVersion: ci.SchemaVersion`; roster.go:500-535 applyProgress assigns k.StatTraining with no LegacyTrainingToGains; hooks/PlayerSpawn_HandleJoin.go:127-131 `if comp.SchemaVersion < mobs.InstanceSchemaVersion { saved = mobs.LegacyTrainingToGains(...) }`; hooks/companion_bonded.go:56-76 snapshotCompanionProgression. internal/hooks/hooks.go:17-19 already installs hooks functions into companionai (SetRespawner/SetSnapshotter).
```

- **refuted** (low): The concrete failure, a legacy pre-Phase-C record reaching applyProgress and double-counting stats, cannot happen with the code as written: 1. Bonded companions came after U10b-0 Phase C. Commit 4a1eabe56, "implement new ai companion", is dated 2026-09-23. Commit a237b038e, which added InstanceSchemaVersion, is dated 2026-08-22. 2. Every engine path that writes CompanionInfo.StatTraining or the skill maps sets SchemaVersion = mobs.InstanceSchemaVersion at the same time. That covers PlayerDespawn_HandleLeave.go:41 and snapshotCompanionProgression at companion_bonded.go:60. Grepping for `StatTraining\s*=` across non-test Go finds no other writer except roster.go:488 (toInfo). 3. toInfo copies a SchemaVersion that came from one of two places. progressOfMob always uses InstanceSchemaVersion. progressOfInfo reads a bonded record, which only holds progression if one of the stamped writers above produced it.  A bonded CompanionInfo does start with SchemaVersion 0 (commands.go:273-281 when kept is empty). In that case its progression maps are nil too, so progressOfInfo returns empty() and applyProgress returns early. The roster.go:498 comment, "a roster entry is always saved at the current schema", holds. That makes the claimed legacy double-count a statically impossible state, not a live bug.  The skill max-merge is not accidental drift. It is commented at roster.go:514-515 as deliberate: a companion moving to a new owner keeps at least her template's starting ranks. It differs from same-owner respawn semantics on purpose.  What survives is a maintainability point. keptProgress, progressOfMob and applyProgress are hand-synced copies of the hooks progression code, and their comments admit it. A future CompanionInfo progression field would need adding here too. hooks already has a seam pattern (companionai setters) that could have exposed snapshot/apply helpers. hooks itself already had two such copies (saveCompanionState and snapshotCompanionProgression), so this PR extends an existing pattern rather than creating a divergence that currently drops data. Severity: low, a consistency and unification nit with no reproducible defect.

</details>

<a id="f138"></a>
### F138 [low] New companion tuning knobs ship as Go defaults only, absent from config.yaml

`modules/aicompanion/config.go:224` · status **refuted** · reported as low

CompanionBaubleChance (default 15: the share of companion searches that also roll a bauble on the owner's behalf, which feeds the bauble economy), IdleTradeSeconds (60) and HollowSignRoom (6880) are read with asInt(get(...), default), but none appears in _datafiles/config.yaml. ReleaseAfterDays and AbandonBelow were added there. The project convention is that tuning values live in config.yaml and a Go default is never treated as the live value. An operator reading config.yaml cannot see or retune the companion bauble rate, and HollowSignRoom, the room id whose board carries the consent text, is a hidden hardcoded default.

**Failure scenario.** Bauble finds per player rise once companions roll alongside their owners. The operator looks for the companion bauble rate in config.yaml's Modules.aicompanion block to tune it and finds nothing. The live 15% is set only in config.go:225. Moving the Hollow sign room also needs a code read to discover the key name.

**Existing mechanism.** _datafiles/config.yaml Modules.aicompanion block (where ReleaseAfterDays, AbandonBelow and RequirePlayerKey were added in this same change)

**Suggested fix.** Add CompanionBaubleChance, IdleTradeSeconds and HollowSignRoom, with comments, to the aicompanion block of config.yaml. Build the commit from the HEAD blob because of skip-worktree.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
config.go:224-225 `IdleTradeSeconds: asInt(get(\`IdleTradeSeconds\`), 60), CompanionBaubleChance: asInt(get(\`CompanionBaubleChance\`), 15),`; config.go:276 `HollowSignRoom: asInt(get(\`HollowSignRoom\`), 6880)`; `grep -n "CompanionBaubleChance\|IdleTradeSeconds\|HollowSignRoom\|ReleaseAfterDays" _datafiles/config.yaml` matches only ReleaseAfterDays (line 2784).
```

- **refuted** (none): The narrow fact is correct: in the PR, CompanionBaubleChance, IdleTradeSeconds and HollowSignRoom are new Go defaults in buildConfig and do not appear in _datafiles/config.yaml. The claimed convention violation does not hold, though. The aicompanion module has its own documented convention, and these knobs follow it. The config.yaml header for Modules.aicompanion (around lines 2722-2726) says that every other setting has a default built into buildConfig, is documented setting by setting in docs/aicompanion/settings.md, and can be overridden in config.yaml. Most of the roughly 70 module knobs work this way: Temperature, RecoveryRounds, IdlePastimeChance, CombatVariance, the Breaker* settings and others have Go defaults only, with nothing listed in config.yaml. All three of the cited knobs are documented in docs/aicompanion/settings.md, so the claim that an operator "finds nothing" is wrong. The operator is pointed straight to that file. The rule that "balance numbers come from config.yaml" applies to the internal/configs balance knobs (config.balance.go), not to this opt-in module, which is off by default. The YAML only lists the consent, cost and ownership settings. CompanionBaubleChance is also clamped to 0-100 and IdleTradeSeconds is floored at 10, so a missing or bad value cannot do harm. Listing the bauble rate in the YAML would be a nice-to-have, but this is consistent with how the module already works, not a unification defect.

</details>
