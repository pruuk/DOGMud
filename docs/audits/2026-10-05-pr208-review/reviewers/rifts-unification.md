# PR #208 review: rifts, unification lens

Blind reviewer `rifts:unification`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

Unification review of the rift micro-dungeon framework (internal/rifts, modules/rifts) and the "inconsistency fixes" in 26b5fd8fc..a4f372715, judged at PR head a4f372715. Much of it reuses existing mechanisms properly. Seven divergences survived verification.

The most serious is rift death forfeiture. It takes all carried items, all gold and 25% of worn gear, and it runs entirely outside the GamePlay.Death penalty system. In particular it skips the ProtectionSkillRanks newbie gate, which ships at 10. Two findings are narration bypasses that leak information past the sight gate:
- The lens table drops bare, untagged player names into fmt.Sprintf lines, so Anonymize cannot hide them.
- The hunter's visual lines go out through the ungated room.SendText.

The rest are smaller duplicates of existing helpers:
- sharesGroup copies mobs.ConsidersAnAlly, with different semantics.
- GetRandomExitFor copies GetRandomExit.
- SpotHiddenPlayer copies newcomerSpots.
- The new OutputCount() rule was not carried into the NPC crafter and shop paths.
- A bespoke strict YAML loader stands in for fileloader.

Correct reuse, verified in code:
- Hidden detection: the hunter runs the same contest primitives as entry detection (combat.RunContest, CalcDetectionScore, CalcSneakScoreVsObserver), and mobs hide through actions.Sneak via the vanish action.
- Persistence: plugin ReadIntoStruct/WriteStruct with util.ErrStateAbsent, and it refuses to save over a record it could not read. Generated rooms are written with util.Save.
- Lost items: registered as a bauble sweep live source and in the item-walker guard.
- Rubble finds: go through actions.StartBaubleFind with baubles.ValueTier.
- Forage: ore extends forager.ForageCore through a room overlay rather than a parallel roll.
- No-recall: reuses the existing allow_recall temp data, now behind a rooms.NoRecall helper.
- Rooms and mobs: the rift chunk extends the ephemeral chunk allocator and plane registry, and mobs come from mobs.NewMobByIdFresh with a statpool.
- Mob spells: the multi-hit and spares-allies options route through resolveMobSpell and mobAreaHarmTargets.
- Generation settings: knobs use the lively.* getters and the apiframework budget and moderation, the same pattern as roomlife. Ambient events plug into roomlife's place hook.
- Routing: exit routing, the entry guard and picklock extend the existing housing routing hooks, adding extra slots instead of a parallel registry.
- Utilities: text wrapping uses util.SplitStringNL, and randomness goes through util.Rand.

There is also a parallel I did not raise as a finding. The rift hunter keeps its own one-hunter-per-player registry and statpool scaling (base + depth*per_depth). internal/bountyhunter already has an active-hunt registry and a scaledStatpool. The rift's teleport-between-rooms pursuit is plausibly justified, because rift doors refuse mobs, so I left it out.

## Coverage

Read in full: internal/rifts/roll.go, memory.go, lost.go, instance.go, hunter.go; most of runtime.go (room build and spawn, about lines 280-480), portal.go lines 1-120, events.go lines 140-236, data.go (loader, validation, wrap), gen.go (addGenerated, ForceGenerate, shuffle), keys.go giveItem, the ClaimRubble part of lore.go, and the puzzle.go outline.
Also read: modules/rifts/rifts.go (wiring, load/save, rubble hook), the GenConfig part of modules/rifts/gen.go, and the module config overlay.
Diffs read in this range: rooms/ephemeral.go, ephemeral_owned.go, routing_hooks.go; actions flee/move/search/search_feature/forage/craft/salvage/combat_drain/divergences; forager/forage_core.go; crafting/crafting.go; hooks spell_resolution/mob_area_harm/NewRound_DoCombat_helpers/NewRound_MobRoundTick/NewRound_UserRoundTick/spell_fold*; mobcommands cast/go; usercommands usercommands/tutorial/picklock/shoot; behaviortree vanish/actions_room/conditions_player; targeting/select.go; characters sight/validate; questengine/bridge.go; the bauble and condition guard tests.
Baseline mechanisms checked: the death hooks (Death_PlayerCleanup, Death_PlayerCorpse, Death_PlayerAnnouncement), GamePlay.Death config and the shipped config.yaml values, messaging Anonymize and the room SendTextVisual* family, narration context, fileloader, mobs.ConsidersAnAlly, bountyhunter context, the zone instance registry, and GetRandomExit.
Not read in depth: router.go, the rest of portal.go (site rotation and DailyTick), the full lore.go, and the gen.go prompt and validation pipeline. Only the reuse side of modules/rifts/gen.go and prompt.go was checked; API consent and security belong to other reviewers. I also did not cover apiframework/relay changes, roomlife place.go, the relay.js changes, rift content YAML beyond the obelisk profile and mobs, or the test files.
I could not pin down whether a lamp-40 cave rift room is shapes-only for infrared observers. The name-leak finding is therefore stated conditionally: it applies to any observer whose sight is shapes-only.

## Findings (7)

<a id="f042"></a>
### F042 [medium] Lens-table narration puts bare player names into fmt.Sprintf lines, bypassing the name-tag and token contract, so the sight gate cannot hide them

`internal/rifts/memory.go:267` · status **confirmed** · reported as medium

TurnLenses builds the room line with fmt.Sprintf(m.Seen, turnerName(u)). turnerName returns the raw u.Character.Name, with no <ansi fg="username"> tag. tellOthers then delivers it with room.SendTextVisualToUser. For a shapes-only observer (SightShapes) that pipeline runs messaging.Anonymize, which only matches tagged username, mobname and petname spans. An untagged name passes through unchanged; Anonymize's own doc comment calls bare names its v1 limitation. The messaging arc's answer to this is one token engine (narration.Substitute with the canonical {actor}/{actee} tokens) plus tagged names, or SendTextVisualHidingNames with a names list. Rifts instead adds a new narration store of positional %s/%d strings (MemorySpec, LossSpec, Profile.Messages, validated by counting %s). It is registered only as a 'messages' key in the surface guard and has no golden snapshot in internal/narration/testdata/stores.

**Failure scenario.** A player in a rift lens room has infrared, or any sight that comes out shapes-only in that room's light. Another player in the room types `1c`. The infrared player receives "Alice turns a lens on the table." (or "Alice fiddles with the lenses on the table."), naming a character whose name the sight pipeline would turn into "a figure" on every other surface.

**Existing mechanism.** internal/narration Substitute and Render with TokenActor {actor}; rooms.(*Room).SendTextVisualHidingNames (internal/rooms/rooms.go:278); the tagged-name convention `<ansi fg="username">%s</ansi>` that Anonymize keys on

**Suggested fix.** Author the lines with {actor} and render them through narration.Substitute using a tagged ActorName. Alternatively, keep the per-reader split but send through SendTextVisualHidingNames(..., []string{u.Character.Name}, ...) so a shapes-only observer reads "a figure".

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
memory.go:267 `tellOthers(run, room, u, fmt.Sprintf(m.Seen, turnerName(u)), fmt.Sprintf(m.SeenUnread, turnerName(u)))`. memory.go:281-286 `func turnerName(u *users.UserRecord) string { if u.Character.IsHidden() { return "Someone" }; return u.Character.Name }`. memory.go:304 `room.SendTextVisualToUser(other, messaging.CategoryRoomDescription, wrap(line))`. messaging/anonymize.go: nameTagPattern matches only `<ansi fg="(username|mobname)...">`, and the doc reads "v1 limitation: bare-name occurrences (names embedded in prose without an ANSI tag) leak through". obelisk.yaml:193 `seen: "%s turns a lens on the table."`.
```

- **confirmed** (medium): The claim holds when you read the code. memory.go is new in the PR (487 added lines since c696c117a). TurnLenses calls `tellOthers(run, room, u, fmt.Sprintf(m.Seen, turnerName(u)), fmt.Sprintf(m.SeenUnread, turnerName(u)))`. turnerName returns either "Someone" (when the turner is hidden) or the bare `u.Character.Name`, with no username ANSI tag. tellOthers sends each other player's line through `room.SendTextVisualToUser`. That function (rooms.go:512) works out SightFull, SightShapes or SightNone and calls RenderForRecipient with no names list. For SightShapes, the pipeline (pipeline.go:63-65) runs only `anonymize`, and nameTagPattern matches only `<ansi fg="username|mobname...|petname">` spans. Anonymize's own doc comment says bare names leak through. The existing fix for this is SendTextVisualHidingNames / sendTextVisualJudgedBy, which applies HideNames(Anonymize(txt), names, SightShapes) to catch bare names, but tellOthers does not use it.  The leak is also more reachable than the reviewer's infrared scenario. obelisk.yaml sets `lamp: 40`, and runtime.go:245-247 applies it to every rift room. The defaults are LightBlindBelow 25 and LightDimBelow 50, and config.yaml does not override either. So with no carried light, a normal-sighted observer in a rift room reads 40 as SightShapes, and SightThroughWindow returns SightShapes when blind <= light < dim. Any such player sees "Alice turns a lens on the table." with the name intact.  The hidden-turner case is handled ("Someone"), so the leak covers only visible turners seen by shapes-only observers. That is a name disclosure, not a security or data breach, so medium is a fair severity.

</details>

<a id="f098"></a>
### F098 [low] OutputCount() introduced for craft quantity but the NPC crafter and shop-pricing paths still read raw Output.Quantity

`internal/crafting/crafting.go:669` · status **confirmed** · reported as low

The commit adds RecipeSpec.OutputCount() (quantity below 1 counts as 1) and switches the player instant, player timed and mob tick craft paths to it. The other baseline consumers of the same field were not swept: mobs/crafter.go:556 and 615 loop `i < recipe.Output.Quantity`, and shops/craftdecision.go:65 values a craft at `price * recipe.Output.Quantity`. The project now has two rules for how many items one craft yields.

**Failure scenario.** A recipe authored without output.quantity, or with quantity 0, gives a player or tick-crafting mob 1 item. A shop crafter NPC stocks 0 for the same recipe (the loop never runs), and craftdecision values it at 0 gold, so the NPC never chooses to craft it. Every non-enchanting recipe shipped today sets a quantity, so this is latent, but it is exactly the drift the helper was meant to prevent.

**Existing mechanism.** The commit's own crafting.(*RecipeSpec).OutputCount, which should be applied at every Output.Quantity consumer

**Suggested fix.** Replace the three remaining raw Output.Quantity reads with recipe.OutputCount().

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
crafting.go:668-673 `func (r *RecipeSpec) OutputCount() int { if r.Output.Quantity < 1 { return 1 }; return r.Output.Quantity }`. At head, mobs/crafter.go:556 `for i := 0; i < recipe.Output.Quantity; i++ { shopInv.AddStockAtRound(...` and :615 `for i := 0; i < recipe.Output.Quantity; i++ { mob.Character.Shop.StockItem(...`; shops/craftdecision.go:65 `return float64(price) * float64(recipe.Output.Quantity)`.
```

- **confirmed** (low): The claim checks out against the code at head. PR #208 adds RecipeSpec.OutputCount() (crafting.go:668-673), which clamps a quantity below 1 up to 1. The player instant path (actions/craft.go:252), the player timed path (NewRound_UserRoundTick.go:734), the mob tick path (NewRound_MobRoundTick.go:604) and SalvageIngredients (crafting.go:682) now call it. Before the PR, the player instant path always made exactly one item, from a single items.New call that ignored Quantity.  Three consumers of the same field still read it raw. mobs/crafter.go:556 and :615 run `for i := 0; i < recipe.Output.Quantity; i++`, and shops/craftdecision.go:65 returns `float64(price) * float64(recipe.Output.Quantity)`. So the codebase now has two rules for how many items one craft yields, and the commit's own helper was not applied everywhere.  The defect is latent. I checked all 245 recipe YAMLs: 226 set output.quantity to 1 to 4. The 19 with quantity 0 are all enchanting recipes with item_id 0. For those, the crafter.go loops sit behind `if recipe.Output.ItemId > 0`, and craftdecision returns 0 first when items.GetItemSpec(0) is nil. No shipped recipe produces a divergent result today. This is a real unification inconsistency with no current player-visible effect, so low is right.

</details>

<a id="f099"></a>
### F099 [low] sharesGroup duplicates the mob ally predicate mobs.(*Mob).ConsidersAnAlly with different semantics

`internal/hooks/mob_area_harm.go:130` · status **confirmed** · reported as low

The new SparesAllies option for area harm decides 'own side' with a new sharesGroup helper (any common entry in Groups). The codebase already has the mob-ally predicate mobs.(*Mob).ConsidersAnAlly (mobs.go:1259), which callforhelp uses. Its rules differ: same MobId is always an ally, a mob that hates the other's groups is never an ally, and same SpeciesId makes allies. hooks/MobIdle_HandleIdleMobs.go also already has a mobHasGroup helper. So 'who is on my side' now has two answers that disagree.

**Failure scenario.** Consider a spares_allies boss and an add of the same species and MobId but with no groups, or with a different group. callforhelp treats them as allies (the add comes to help), but the boss's blast still hits the add. The reverse case: a mob sharing a group but hating the caster's group is spared by the blast even though ConsidersAnAlly says it is not an ally. The shipped rift mobs all share group 'obelisk' and speciesid 46, so this is latent today and will drift as other content adopts spares_allies.

**Existing mechanism.** mobs.(*Mob).ConsidersAnAlly (internal/mobs/mobs.go:1259)

**Suggested fix.** Call caster.ConsidersAnAlly(m), or deliberately extend that one predicate, instead of adding sharesGroup.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
mob_area_harm.go:63-66 `if spareAllies && charmedByUserId == 0 && !bonded && !m.Character.IsCharmed() && sharesGroup(caster, m) && ...`; mob_area_harm.go:130-142 `func sharesGroup(a, b *mobs.Mob) bool { for _, ga := range a.Groups {... if ga == gb ...`. mobs.go:1259-1278 `func (r *Mob) ConsidersAnAlly(m *Mob) bool { if m.MobId == r.MobId { return true } ... if r.hatesAnyGroup(m.Groups) || m.hatesAnyGroup(r.Groups) { return false } ... SpeciesId ...`. callforhelp.go:84 `if !mobInfo.ConsidersAnAlly(mob)`.
```

- **confirmed** (low): The code bears the claim out. The PR adds sharesGroup in internal/hooks/mob_area_harm.go:130, a check for any non-empty group the two mobs have in common. It is used only by mobAreaHarmTargetsSparing (line 63-64), which spell_resolution.go:613 calls with spellData.SparesAllies. The codebase already has a predicate for "who is on my side": mobs.(*Mob).ConsidersAnAlly (mobs.go:1259), which callforhelp.go:84 uses. Its rules differ: the same MobId is always an ally, a hate in either direction rules out an ally, and the same SpeciesId (when nonzero) makes an ally. Groups count only through Hates. So the two predicates really can disagree. Example: a same-MobId add with no groups is an ally to callforhelp but is not spared by the blast. In the reverse case, a mob that shares a group but has a mutual hate is spared, unless it is already targeting the caster (the CurrentCombatTarget check covers that case). Mitigations: the choice is deliberate and documented in the SparesAllies field comment (spells.go:60-62, "sharing one of its groups"). The only content using it is lattice-burn.yaml and obelisk-convergence.yaml, both rift Lenses, so this is latent today. One part of the claim is weaker: mobHasGroup (MobIdle_HandleIdleMobs.go:414) checks membership in one named group and is not an ally predicate, so it is not a true duplicate. Overall this is a real unification and consistency finding: a parallel "own side" definition next to an existing one. It is not a live bug, so low severity is right.

</details>

<a id="f100"></a>
### F100 [low] Rift data uses a bespoke strict YAML loader that can panic boot, instead of fileloader

`internal/rifts/data.go:434` · status **confirmed** · reported as low

Rift profiles and room templates are loaded with a private readStrict (os.ReadFile plus yaml.UnmarshalStrict) and a hand-written id-must-match-filename check. The project's world-data loader, fileloader.LoadAllFlatFiles and LoadFlatFile (Filepath() match, Validate(), duplicate-id check), already does the filename check. It decodes leniently and reports unknown keys through StrictDecodeProbe, which the boot smoke test installs. Rift data takes the opposite policy, and any error panics in LoadDataFiles. Generated rooms are written with util.Save rather than fileloader.SaveFlatFile.

**Failure scenario.** An authored rift room YAML gains a key the Template struct does not know, for example a content author adding `zone:` or misspelling `descripton:`. Every other world file would load and the smoke test's strict probe would flag the key. This file instead hits panic("rifts.LoadDataFiles: ...") at boot (main.go:1744) and the server does not start. Generated gen-* files are exempt, so the hard failure falls only on hand-authored content.

**Existing mechanism.** internal/fileloader LoadAllFlatFiles, LoadFlatFile and SaveFlatFile, with StrictDecodeProbe

**Suggested fix.** Give Profile and Template Id(), Filepath() and Validate() methods and load them with fileloader, so rift data follows the same strictness policy and smoke-test probe as the rest of the world. Or document why rift data deliberately panics on unknown keys.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
data.go:434-443 `func readStrict(path string, out any) error { b, err := os.ReadFile(path) ... yaml.UnmarshalStrict(b, out)`; data.go:374-378 `loaded, err := loadFrom(DataDir()); if err != nil { panic(...) }`; data.go:400-402 `if want := strings.TrimSuffix(filepath.Base(f), ".yaml"); p.Id != want { return nil, fmt.Errorf(...must match the file name...` . By contrast, fileloader.go LoadAllFlatFiles does `yaml.Unmarshal(bytes, &loaded); probeStrict[T](path, bytes)` with the Filepath() suffix check and Validate().
```

- **confirmed** (low): The claim is accurate as written, but its consequence is weaker than the framing suggests.  What it gets right: rifts loads with a private readStrict, which is os.ReadFile followed by yaml.UnmarshalStrict (data.go:434-443). It also hand-rolls the id-must-match-filename check (data.go:400-402 and 411-413). LoadDataFiles panics on any loadFrom or validate error (data.go:374-383), and main.go:1744 calls it at boot. fileloader.LoadFlatFile and LoadAllFlatFiles decode leniently with yaml.Unmarshal and only report unknown keys through StrictDecodeProbe, which is nil in production (fileloader.go:46-68, 95-96, 209-210). They already do the Filepath() suffix check, Validate() and the duplicate-id check. So a hand-authored room template with an unknown key, such as a misspelled `descripton:`, does panic boot. The same typo in a fileloader-loaded world file would load silently. Generated gen-* files are exempt because they are warned and skipped (data.go:414-420).  Mitigating facts: (1) The panic is a deliberate fail-fast choice. The doc comment says "Like the other boot loaders it panics on bad data". fileloader errors also end in boot panics at their callers. The only difference is unknown keys, and the fileloader comment itself (the `hostile:` incident) argues that lenient decoding is the riskier policy. The worst case is that a content author's typo stops boot locally before it ships, not silent misbehaviour in production. (2) Bypassing fileloader is not unusual in the baseline. At c696c117a, 86 non-test yaml.Unmarshal call sites across dozens of internal packages (achievements, dialogue, conversations, behaviortree, factions and others) use bespoke loaders. Only 28 call sites use fileloader.LoadAllFlatFiles. (3) This PR also adds the strict-decode policy to mining, timber, scavenger and merchantchests. Rifts is consistent with its sibling packages in the PR, so the inconsistency is between this PR and fileloader, and it is not specific to rifts.  Verdict: the claim reproduces from the code as a real reuse and consistency gap with real fail-fast-at-boot behaviour. That makes it a low-severity unification finding, not a correctness or availability bug.

</details>

<a id="f101"></a>
### F101 [low] Rift death forfeiture is a parallel death-penalty system that skips the ProtectionSkillRanks newbie gate

`internal/rifts/events.go:164` · status **confirmed** · reported as high

OnPlayerDeath calls forfeitOnDeath, and lost.go:150-199 then removes every carried item, all carried gold (gold_on_death) and each worn item at the profile's worn_chance. The project already owns death penalties in GamePlay.Death (config.gameplay.go GameplayDeath: EquipmentDropChance, AlwaysDropBackpack, ProtectionSkillRanks). The shared death-cleanup hook only applies penalties when the character's total skill ranks exceed ProtectionSkillRanks (Death_PlayerCleanup.go:50). The rift path checks neither that gate nor anything else in GamePlay.Death. Its loss rules live in content YAML (obelisk.yaml losses: worn_chance 25, gold_on_death true), outside config.balance and config.yaml. The rift has no level or rank gate on entry: router, portal, instance and events contain no level or skill-rank check. Portal sites appear in wilderness biomes, and only the module's ExcludeZones list keeps them out of particular towns.

**Failure scenario.** A new character with 10 or fewer total skill ranks dies inside a rift. The shipped config sets ProtectionSkillRanks: 10, so everywhere else in the game this character would lose nothing (stat decay and skill rust are waived). Here they lose their whole pack and all their gold, and each worn piece goes with 25% probability. Separately, an operator who tunes or disables death item loss through GamePlay.Death (EquipmentDropChance: 0, AlwaysDropBackpack: false, as shipped) gets no effect on rifts.

**Existing mechanism.** configs.GameplayDeath (internal/configs/config.gameplay.go: EquipmentDropChance, AlwaysDropBackpack, ProtectionSkillRanks) and the allowPenalties gate in internal/hooks/Death_PlayerCleanup.go wirePlayerDeathCleanup

**Suggested fix.** Gate forfeit on the same predicate as the shared hook. Lift `GetTotalSkillRanks() > ProtectionSkillRanks` into one helper that both call. Put the rift loss rates in config.balance and config.yaml, or at least make the profile values scale the GamePlay.Death knobs instead of replacing them.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
events.go:150-165: `func OnPlayerDeath(userId, roomId int) { run := RunForRoom(roomId) ... PurgeKeys(u) ... forfeitOnDeath(u, run.Profile) }`. lost.go:165-183: `if how == "death" { for _, itm := range u.Character.GetAllWornItems() { if keepable(itm, riftOnly) && rng(100) < l.WornChance && ...` then `for _, itm := range u.Character.GetAllCarriedItems() {...take(itm)}`, and `if how == "death" && l.GoldOnDeath && u.Character.Gold > 0 { gold = u.Character.Gold; u.Character.Gold = 0`. Death_PlayerCleanup.go:50: `allowPenalties := u.Character.GetTotalSkillRanks() > int(config.Death.ProtectionSkillRanks)`. _datafiles/config.yaml:290-311: `EquipmentDropChance: 0`, `AlwaysDropBackpack: false`, `ProtectionSkillRanks: 10`. A grep for level, skillranks and Protection in rifts router.go, portal.go, instance.go, events.go and data.go returned nothing.
```

- **confirmed** (low): Only the narrow fact holds. Rift death forfeiture never checks ProtectionSkillRanks, so a character with 10 or fewer total skill ranks who dies in a rift does lose their pack and gold, and each worn piece at 25%.  The unification framing does not hold. No item-loss mechanism exists to reuse or bypass. In the PR head, GameplayDeath.EquipmentDropChance and AlwaysDropBackpack are read nowhere in Go: the only hits are the struct declaration and the range validation in config.gameplay.go (line 58 even says "Ignore OnDeathAlwaysDropBackpack"). Players drop no items on a normal death in this codebase. So the claim that an operator tuning or disabling item loss through GamePlay.Death gets no effect on rifts is refuted: those knobs affect nothing anywhere.  ProtectionSkillRanks gates only stat decay and skill rust in Death_PlayerCleanup.go:50-55. It is a grace period on deprogression, not a gate on items.  The rift losses are a documented, deliberate rift-specific risk (internal/rifts/context.md "Losses", line 362), configured per profile in YAML. They are not a duplicate of an existing system.  What is left is a design question: should rift item loss also honor the newbie grace period, given that entry has no level or rank gate? That is a product call, not a reuse defect, so the severity drops to low.

- **refuted** (low): The claim's main point is wrong. It says PR #208 built a parallel death penalty instead of reusing GamePlay.Death, but there is no item-loss-on-death mechanism to reuse. EquipmentDropChance and AlwaysDropBackpack are dead config: no Go code outside config.gameplay.go reads them (it only validates them). So "an operator who tunes GamePlay.Death gets no effect on rifts" is true of the whole game, not only of rifts. ProtectionSkillRanks gates one thing, the stat decay and skill rust in Death_PlayerCleanup.go:50. It was never an item-loss or gold-loss gate, so nothing was skipped. The only existing death-time gold-loss path is transferPartialGold in Death_PlayerCorpse.go (submission outcomes), and it has no skill-rank gate either. The rift loss rules are a deliberate, documented content design: obelisk.yaml:219-228 says dying in the Obelisk costs everything carried loose, all carried gold and worn pieces at worn_chance. One fact in the claim does hold: a character at or below ProtectionSkillRanks (10 shipped) gets no newbie waiver on rift loss, and nothing gates rift entry by rank. That is a balance question for the owner (should rift forfeiture respect the newbie threshold?). It is not a defect where an existing mechanism was bypassed or rewritten, so the unification/high framing does not hold. Residual severity: low, as a design question.

- **confirmed** (low): The code does what the claim says. Rift death forfeiture (events.go:150-165, lost.go:147-199) takes all keepable carried items, all gold when gold_on_death is set, and each worn item at worn_chance. It never checks ProtectionSkillRanks. A character with 10 or fewer total skill ranks who dies in a rift does lose those things.  The claim's main point does not hold, though. It says this is a parallel system that skips an existing death-item-loss mechanism and overrides operator tuning. That is wrong. GameplayDeath.EquipmentDropChance and AlwaysDropBackpack are dead knobs: no non-test Go file reads them except their declaration and the clamp in config.gameplay.go:39-62. The game has no existing item-drop-on-death mechanism to reuse or bypass, so "an operator who sets EquipmentDropChance 0 gets no effect on rifts" is true of every path, not a rift-specific bypass.  ProtectionSkillRanks has exactly one consumer, Death_PlayerCleanup.go:50. It gates only stat decay and skill rust. It is not a general newbie death-penalty shield over item or gold loss.  Rift loss is also deliberate and disclosed to players. It is documented in the profile header in obelisk.yaml, it is a per-profile content knob, and the 2026-10-02 patch notes say: "Die in there and it keeps everything you carry, all the gold on you, and maybe some of what you wear."  What remains is a design question: whether rift entry or rift loss should honor the newbie protection threshold, given that portals spawn in ordinary wilderness biomes with no rank gate. That is worth raising, but it is a low-severity balance/UX point, not a high-severity unification defect.

</details>

<a id="f102"></a>
### F102 [low] Hunter appearance, withdrawal and destruction lines bypass the sight gate (room.SendText), unlike the rest of the package

`internal/rifts/hunter.go:263` · status **confirmed** · reported as medium

The hunter's arrival, withdrawal and destruction are visual events, and their authored text names the mob ('The Facet Hunter steps back into the wall and is gone', 'something tall and faceless steps out of the crystal'). They are sent with the unconditional room.SendText at hunter.go:126, 156, 224 and 263. The same package sends its other visual room events through the sight-gated room.SendTextVisual (events.go:139, 143; portal.go:234, 260). The baseline has SendTextVisual, and SendTextVisualWithAudio for events that also make a sound, exactly so that blind or dark-room observers get the version their senses allow.

**Failure scenario.** A blinded player, or one standing in a dark rift room without a Glowstone, is in the room when their party member's hunter withdraws. They read "The Facet Hunter steps back into the wall and is gone. The facets keep turning, searching." They learn the mob's identity and movement through a channel the sight pipeline would have suppressed or anonymised for them.

**Existing mechanism.** rooms.(*Room).SendTextVisual and SendTextVisualWithAudio (internal/rooms/rooms.go:271, 451)

**Suggested fix.** Use SendTextVisualWithAudio, with a sound-only variant line in the profile ("every facet in the walls rings at once"), or plain SendTextVisual, consistent with doors_open and portal_open.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
hunter.go:126 `room.SendText(messaging.CategoryRoomDescription, run.Profile.Msg("hunter_withdraws"))`; hunter.go:156 `room.SendText(... run.Profile.Msg("hunter_destroyed"))`; hunter.go:224 `room.SendText(... "hunter_withdraws")`; hunter.go:263 `room.SendText(messaging.CategoryRoomDescription, p.Msg("hunter_arrives"))`. By contrast, events.go:143 `room.SendTextVisual(messaging.CategoryRoomDescription, p.Msg("doors_open"))`. obelisk.yaml:570-572 `hunter_withdraws: The Facet Hunter steps back into the wall and is gone.`
```

- **confirmed** (low): The claim reproduces from the code. All four hunter room lines go through Room.SendText, which renders every player in the room on messaging.ChannelAudio and has no sight check (rooms.go:248-271). The same package sends its other visual room events through SendTextVisual (events.go:139, 143; portal.go:234, 260). Elsewhere the engine sends mob arrivals and departures through SendTextVisualWithAudio (actions/relocate_mob.go:104,110; mobcommands/go.go:38; usercommands/go.go:387), and that function's doc comment describes exactly this leak: a pitch-dark observer being told a name. hunter.go is entirely new in this PR (331 added lines vs c696c117a), so this is PR-introduced and not inherited. So the blind or dark observer really does get "The Facet Hunter steps back into the wall and is gone" and "The Facet Hunter comes apart into a rain of black glass", which name the mob. I am lowering the severity from medium to low for three reasons. The arrival line ("one cold note ... something tall and faceless") is already part sound and does not name the mob. The hunter's quarry has already been told privately that it is hunted (hunter.go:93). And the observer is usually fighting the hunter, so its identity is learned anyway. What remains is a consistency gap with the unified sight pipeline and a small name leak, not a meaningful exploit. The fix is to use SendTextVisualWithAudio with an audio-only variant (or SendTextVisual for withdraw and destroy).

</details>

<a id="f103"></a>
### F103 [low] GetRandomExitFor and SpotHiddenPlayer are copy-pastes of existing functions instead of parameterising them

`internal/rooms/routing_hooks.go:169` · status **confirmed** · reported as low

GetRandomExitFor re-implements GetRandomExit (rooms.go:1400): the same secret and locked filter, the same mutator-exit loop and the same util.Rand pick, plus a router check. Mob wander (mobcommands/wander.go:29) and callforhelp (callforhelp.go:103) still call the old copy. Likewise, actions.SpotHiddenPlayer (move.go:424-451) duplicates the player branch of newcomerSpots (move.go ~346-381) line for line: the same CalcDetectionScore/CalcSneakScoreVsObserver/RunContest, the Awareness transition, the sneaking-flag clear and the AwardResolved call. newcomerSpots was not refactored to call it. The contest-site guard needed a new owner entry for the second copy.

**Failure scenario.** The next change to exit eligibility, for example a new exit flag that must be skipped, is made in GetRandomExit and silently misses GetRandomExitFor, or the reverse, so fleeing and wandering diverge on which exits count. Similarly, a future fix to detection, such as the patrol over-training of Search already noted in memory or a reveal-message fix, lands in newcomerSpots and the rift hunter keeps the old behaviour.

**Existing mechanism.** rooms.(*Room).GetRandomExit (internal/rooms/rooms.go:1400); actions.newcomerSpots (internal/actions/move.go)

**Suggested fix.** Implement GetRandomExit as GetRandomExitFor with no routing filter, or give it an optional filter, so there is one body. Have newcomerSpots call SpotHiddenPlayer, or a shared spotOne helper, for each hidden player.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
routing_hooks.go:169-205 `func (r *Room) GetRandomExitFor(userId int) ... add := func(name string, info exit.RoomExit) { if info.Secret || info.Lock.IsLocked() { return } ...} for mut := range r.ActiveMutators {...} pick := util.Rand(len(candidates))`, compared with rooms.go:1400-1438 GetRandomExit doing the same. move.go:440-443 `success := combat.RunContest(observerScore, []contest.Entry{{Score: hiddenScore}}).Success; watcher.AwardResolved(...); ... TransitionToRevealing(...TriggerObserverSearch)`, identical to move.go:354-359 in newcomerSpots. contest_site_guard_test.go adds an owner entry for `move.go:SpotHiddenPlayer` described as "the same contest as newcomerSpots".
```

- **confirmed** (low): The duplication is real, but it is partial. GetRandomExitFor (routing_hooks.go:169-205) repeats GetRandomExit's logic (rooms.go:1400-1438) and adds a RouteExit lookup: the same Secret/IsLocked skip, the same base-exit plus ActiveMutators loop, and the same util.Rand map-walk pick. GetRandomExit was not reimplemented on top of the new function, for example as a thin wrapper or an unrouted variant. It still has two callers: mobcommands/wander.go:29 and callforhelp.go:103. Only flee.go:231 uses the new copy. So a future change to exit eligibility has to be made in two places. One difference is intended: GetRandomExitFor reroutes exits, so the two functions cannot simply be merged without a flag. That is why I am keeping this at low severity rather than treating it as a bug today. SpotHiddenPlayer (move.go:424-451) repeats the core of the player branch of newcomerSpots (move.go:345-374): CalcDetectionScore, CalcSneakScoreVsObserver, combat.RunContest, Awareness.TransitionToRevealing(TriggerObserverSearch), clearing the sneaking flag, and AwardResolved(Search). Calling it "line for line" overstates it. The messages differ (the hunter's "turns, and its attention settles on you" against "enters the room and notices you"), light is derived from room.LightLevel() rather than passed in, the order differs (the award comes before the reveal), and SpotHiddenPlayer does not message the watcher. Still, the shared contest and reveal core was not factored out, and newcomerSpots does not call any shared helper. contest_site_guard_test.go:74 registers a second owner entry and itself calls it "the same contest as newcomerSpots". This is a maintainability and divergence risk with no current misbehavior, so low severity is right.

</details>
