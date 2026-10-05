# PR #208 review: housing, unification lens

Blind reviewer `housing:unification`, run 2026-10-05 against PR head `3ce674451`. Findings were then checked by independent verifiers (three for critical and high, one otherwise). Severity is the verifiers' median. Back to the [synthesized report](../README.md).

## Reviewer summary

I reviewed the housing piece (f546918ec..f7310ed77) as it stands at the PR head, looking for unification and reuse problems. For the most part the contributor reused the shared mechanisms well:
- Persistence goes through util.Save, util.ReadLivingState (with ErrStateAbsent) and util.QuarantineCorrupt. Writes happen before the in-memory registry is updated. The house directory is gitignored and dockerignored.
- Authored buildings load through fileloader.LoadAllFlatFiles.
- The standing gate uses factions.TierFor and opinions.Tier.
- Strongboxes are ordinary rooms.Container entries under a gamelock.Lock (plus a Sealed flag), so the existing lock checks in look, get, put and the aicompanion code refuse them without any housing code.
- Stations set the existing rooms.Room.Station field that the craft commands already read.
- The bed works through a condition (131) with the existing recovery statmods.
- Naming prompts use user.StartPrompt. Player-written descriptions go through util.EscapeAnsiTags.
- The landlord list is drawn with renderShopTable, and buying is gated by actions.ShopSightRefusal.
- Room lines use SendTextVisual.
- Houses are registered with the bauble catalog sweep and with the item-walker guard.
- Floor decay, scavengers and the untaken-bauble sweep skip unit rooms through the new IsPrivateRoom hook.
- Gold changes raise events.EquipmentChange and events.ItemOwnership.

The routing hooks (ExitRouter, EntryGuard, RoomOverlay, RoomSaveHook) are new, but I found nothing per-player in the baseline they duplicate. ExitsTemp and instanceRegistry.IsAuthorized do different jobs.

The divergences, worst first:
1. **Landlord speech (medium).** Every landlord line goes through the asynchronous mob.Command("say ..."). For merchants the project already replaced that with the synchronous actions.Say (merchantSay) to fix delayed refusals. Here it also breaks message order on a purchase, and needs a semicolon ban that is incomplete.
2. **Furnishing nouns (medium).** A hand-kept list of station nouns has already drifted from the station list derived from recipes (it is missing "rack" for tanning_rack). Fixing it naively would lock owners out of homes that hold a container named "rack".
3. **Prices outside config (medium).** Housing prices and limits are copied into four building YAML files rather than kept as balance knobs. The precedent is GuildFoundingCost.

Lower findings: a parallel "never bought" item registry instead of the existing spec.NotSalable flag and buy rules; the bed's "twice as fast" claim depends on a config value it does not read; small re-implemented helpers (direction deltas, capitalise, a placeholder filler, gold-then-bank charging); and one landlord's name hardcoded in the shared ask parser.

## Coverage

Read in full at the PR head: internal/housing/persistence.go, purchase.go, terms.go, offers.go, containers.go, door.go, overlay.go, voice.go, and housing.go; furnishings.go (most of it); registry.go lines 1-250; guests.go lines 1-140; use_items.go lines 25-110 and 300-318; internal/rooms/routing_hooks.go; internal/usercommands/housing_shop.go; internal/behaviortree/actions_housing.go.

Also read the housing-related diffs in roommanager.go, save_and_load.go, autosave_prepare.go, baubles_untaken.go, go.go, picklock.go, unlock.go, use.go, usercommands.go, sleep.go (actions and usercommands), steal.go, mobcommands.go, items.go, never_bought.go, sell.go, main.go, world.go, bauble_sweep.go, item_walker_guard_test.go, messaging_surface_guard_test.go, .gitignore and the dockerignore.

Checked the baseline (c696c117a) for existing mechanisms: gold-then-bank charging, NotSalable, shops.EvaluateBuyRules, mapper.GetDelta, narration.Substitute, tier-name parsers, capitalise helpers, merchantSay, guild cost knobs, the sleep regen knobs and the instance access gate.

Not read: house.go beyond its function list, crafted.go, the guests.go revoke/leave/eject bodies, the remainder of use_items.go (extension placement and the redecorate flow), behaviortree conditions_state.go, mobcommands/unlock.go, the get.go and remove.go internals beyond their diffs, tools/housing_units.py, and every test file. I did not assess security or concurrency in depth: the global mutex is held during an fsync on every command in a lodging, and Capture marshals YAML for every item after each command there. Both are noted but not evaluated.

## Findings (7)

<a id="f022"></a>
### F022 [medium] furnishingNouns is a hand-kept copy of station nouns and has already drifted from the recipe-derived stationTypes

`internal/housing/furnishings.go:70` · status **confirmed** · reported as medium

Which stations a deed can install comes from the recipes (stationTypes reads crafting.GetAll()), and a station's room noun is the last word of its id (stationNoun). The nouns housing reserves against container names, and the nouns stripFurnishings removes, come from a separate hardcoded list: bed, forge, bench, circle, fire, loom, station. The recipe data at the PR head also uses tanning_rack (7 recipes, added by the wilderness-trades commits in this same PR), whose noun is "rack". "rack" is not in the list. So it is not reserved as a container name, and stripFurnishings never removes a "rack" noun the overlay added. The list is also checked at load: checkLinks calls validContainerName, which uses reservedContainerNames, built from furnishingNouns in init. The obvious fix, adding "rack" to the list, would therefore make every existing house file with a container called "rack" fail checkLinks at boot. Those houses would be rejected and held, locking their owners out of their own lodgings.

**Failure scenario.** An owner names a container "rack" in one room (allowed today). Later a maintainer notices the gap and adds `rack` to furnishingNouns. On the next boot, loadOneHouse calls checkLinks, which returns "container \"rack\": \"rack\" means something else here", and reject() holds every room of that house and sets heldOwners. GuardEntry then refuses the owner. Separately, a vacated unit keeps its "rack" noun and its temp key after stripFurnishings, so the next lodger sees a phantom tanning rack noun.

**Existing mechanism.** housing's own stationTypes() (furnishings.go:39), derived from crafting.GetAll(), is the authoritative station registry

**Suggested fix.** Derive the reserved and stripped nouns from stationTypes() via stationNoun, plus "bed". In stripFurnishings, remove whatever nouns carry the furnishTextKey+".noun." temp marker instead of iterating a fixed list. Do not apply new reservations retroactively in checkLinks: validate only new names, or grandfather existing ones, so that a change to the noun set can never cause houses to be held.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
furnishings.go:39 `var stationTypes = func() []string { ... crafting.GetAll() ... r.Station ...}`; :60 stationNoun returns the last word; :70 `var furnishingNouns = []string{`bed`, `forge`, `bench`, `circle`, `fire`, `loom`, `station`}`; :396 stripFurnishings loops only over furnishingNouns; containers.go:46-50 init reserves furnishingNouns; housing.go checkLinks calls validContainerName. `grep station: _datafiles` shows `7 station: tanning_rack`, introduced by commit 46766d13b in this PR.
```

- **confirmed** (medium): The drift is real at PR head. At baseline c696c117a there are no tanning_rack recipes (`git grep -c tanning_rack c696c117a -- _datafiles` returns nothing). At head, 7 recipes use `station: tanning_rack`, added by e42c5f520 and 46766d13b in this PR.  stationTypes() builds its list from crafting.GetAll(), and useStationDeed (furnishings.go:262) offers every entry in that list. So a station deed can install a tanning rack. When furnishRoom (furnishings.go:379) lays the station over the room, it calls setNoun with stationNoun("tanning_rack"), which is "rack", and records the temp key `housing.furnishings.noun.rack`.  stripFurnishings (furnishings.go:396) only walks the hardcoded furnishingNouns list (bed, forge, bench, circle, fire, loom, station). "rack" is not in it, so the rack noun and its temp key survive a vacate or a house change. The next occupant sees a phantom tanning rack description. r.Station is cleared, so it cannot be used to craft.  containers.go init reserves only furnishingNouns, so "rack" is also not a reserved container name. The station-install check at furnishings.go:296 blocks installing a rack where a container called "rack" already exists. The reverse order, naming a container "rack" in a room that already has the rack, is not stopped by validContainerName.  The boot-lockout scenario is real as a hazard: checkLinks validates container names through reservedContainerNames, so adding "rack" to the list later would reject existing houses with a "rack" container. But it depends on a future edit and is not a current failure. The current, reproducible harm is the stale noun and the name collision. The underlying issue matches the claim: a hand-kept list that has drifted from the recipe-derived registry. Medium is fair. A fix should derive the nouns from stationTypes() plus "bed", and handle load validation of names already saved.

</details>

<a id="f023"></a>
### F023 [medium] Landlord speaks through the async mob.Command("say ...") pipeline that merchants were deliberately moved off

`internal/usercommands/housing_shop.go:93` · status **confirmed** · reported as medium

Every line a housing landlord says (refusals, sale confirmations and terms) is sent as mob.Command(`say ` + line). That queues events.Input on the mob's asynchronous command pipeline. The project already found and fixed exactly this for merchants: actions.merchantSay (sell.go:91-110) speaks synchronously through actions.Say. Its comment explains that mob.Command can delay the line by many turns on a busy NPC, so the player never connects it with what they did. Housing brings that problem back for landlords. Purchase also mixes queued landlord lines with synchronous user.SendText lines, so they arrive out of order. Finally, mob.Command splits its input on ';' (mobs.go:978), which is why the contributor had to forbid semicolons in building words and voice lines (housing.go:229-243, voice.go). That workaround does not cover every value that reaches a spoken line: Tier.Name ({tier}), Building.Name ({name}) and DoorExit ({door}) are never checked for ';'. With actions.Say the whole class of problem would not exist.

**Failure scenario.** 1) A player buys a home. purchase.go:145 queues "Stamped. Welcome to ..." and lines 146-151 send "You count out 500 gold... You now have a room..." at once. The player reads the purchase receipt before the landlord's welcome, which runs a turn or more later. If the landlord has a schedule or patrol queued, lastCommandTurn may be far ahead and refusals such as "Every room's let" arrive many turns late, which is the silent-failure symptom merchantSay was written to fix. 2) An author writes a tier name like `a room; north`. Validate accepts it. When a broke player types `buy home`, the landlord says "It's 500 gold for a room" and then runs `north` as a mob command.

**Existing mechanism.** actions.Say(&actions.MobActor{Mob, Room}, line), as used by actions.merchantSay (internal/actions/sell.go:104) and usercommands.merchantSay (offer.go)

**Suggested fix.** Build the say callback as func(line string){ actions.Say(&actions.MobActor{Mob: mob, Room: room}, line) } in housing_shop.go and in both behaviortree actions. The semicolon validation then has no purpose and can be dropped.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
housing_shop.go:93 `housing.Buy(user, func(line string) { mob.Command(`say ` + line) }, ...)`; actions_housing.go:38 and :155 do the same. sell.go:91-104: "Merchant refusals ... were previously delivered via mob.Command(\"say ...\"), which enqueues the line on the mob's ASYNC command pipeline ... Speak synchronously instead". offer.go also routes through actions.Say. mobs.go:978 `for i, cmd := range strings.Split(inputTxt, `;`)`. housing.go:229 `words := map[string]string{`proprietor`: ..., `location`: ..., `outside_text`: ...}` (no Name, no Tier.Name). purchase.go:145-151 queued say followed by synchronous SendText.
```

- **confirmed** (medium): The code matches the claim. housing_shop.go:93 passes `func(line string) { mob.Command("say " + line) }` to housing.Buy. behaviortree/actions_housing.go:38 and :155 do the same. (The claim cites the file as actions_housing.go; it is in internal/behaviortree, not internal/actions.)  Mob.Command (mobs.go:960-990) queues events.Input at or after m.lastCommandTurn and splits the input on ';'. That is the async delay that actions.merchantSay (sell.go:91-110) was written to avoid. merchantSay speaks synchronously through actions.Say(&MobActor{...}).  In purchase.go, say(b.Line("home.welcome")) is queued, and the two synchronous user.SendText receipt lines go out straight after it. So the receipt reaches the player before the landlord's welcome.  Housing.go:229-243 checks only proprietor, location, outside_text, vouched_by and standing_hint for ';'. voice.go:156 checks the voice lines. Tier.Name ({tier} in home.no_gold) and Building.Name ({name} in home.welcome) are never checked. So an authored tier name containing ';' would split into a second mob command.  Mitigating points: - The claim overstates "merchants were deliberately moved off". Only the sell path moved. actions/buy.go still uses shopMob.Command("say ...") in about nine places, so housing matches the unconverted buy path. - The ';' issue needs a content author to write it. DoorExit is an exit name players must type, so a ';' there is implausible.  Even so, a synchronous mechanism (actions.Say) already exists and fixes both problems. The PR also added a ';' validation workaround for problems that mechanism would have avoided, and that workaround is incomplete. Medium stands as a unification and consistency finding.

</details>

<a id="f076"></a>
### F076 [low] Bed's "twice as fast" is an absolute +2% statmod, tied to PlayerHealthRegenPct by a comment rather than a knob

`internal/actions/sleep.go:22` · status **confirmed** · reported as low

The bed bonus is condition 131 with recovery statmods of 2. HealthPerRound reads those as +0.02 added to the per-round regen fraction, so the bed doubles regen only while config.yaml keeps PlayerHealthRegenPct, PlayerStaminaRegenPct and PlayerConvictionRegenPct at 0.02. The condition YAML says so in a comment. Five pieces of player-facing copy state the result as fact ("rest twice as fast"): furnishings.go:168, :251 and :374, and voice.go:25 and :87. Sleep regen is already a balance knob, SleepRegenMultiplier, applied in the same function. The bed adds a second, hidden coupling instead of a sibling knob.

**Failure scenario.** The owner retunes PlayerHealthRegenPct to 0.03, a normal config edit. The bed now gives 0.05 against 0.03, about 1.67x, while the bed noun, the landlord and the deed text still promise "twice as fast". Nothing fails to flag it.

**Existing mechanism.** BalanceConfig.SleepRegenMultiplier, applied in characters.HealthPerRound/StaminaPerRound/ConvictionPerRound

**Suggested fix.** Add a BedSleepRegenMultiplier balance knob applied next to SleepRegenMultiplier when the bed condition flag is present, and drop the absolute statmods. Or at minimum make the copy not promise an exact ratio.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
_datafiles/world/dogmud/conditions/131-sleeping_in_a_bed.yaml comment: "players regen 0.02 of each pool a round (PlayerHealthRegenPct ...), so 2 doubles it"; characters/resources.go:385-398 `pct += StatMod(HealthRecovery)/100.0` and then `SleepRegenMultiplier`; config.yaml:1198-1200 at 0.02; the "twice as fast" strings appear at the five lines listed.
```

- **confirmed** (low): The code matches the claim. Condition 131 has a flat +2 on healthrecovery, staminarecovery and convictionrecovery. HealthPerRound, StaminaPerRound and ConvictionPerRound each add StatMod/100 to the base PlayerXRegenPct, then apply SleepRegenMultiplier to the result. So the bed doubles regen only while each PlayerXRegenPct is exactly 0.02, and the shipped config.yaml has all three at 0.02. Retuning any of them would make the "twice as fast" promise wrong, and nothing would flag it: there is no bed knob in internal/configs, and the only link is a YAML comment. Five player-facing strings state "twice as fast" as fact. Nothing breaks at the shipped values, so the bug is latent and low severity is right.

</details>

<a id="f077"></a>
### F077 [low] Shared ask-to-buy parser hardcodes one landlord's name

`internal/behaviortree/actions_housing.go:72` · status **confirmed** · reported as low

isPlainPurchase is the shared parser for every building's buy_housing action. Its lead-in list hardcodes `hobb`, `mr` and `pennock`, the Back Court landlord (mob 9801). The other three landlords (Aubric Sallow 9802, Brannoc Tull 9803, Old Brock 9804) get no such allowance, although the landlord's identity is already available from building data (LandlordMobId, and landlordName via the mob spec). This is parallel per-building data hidden in shared code.

**Failure scenario.** `ask brannoc Brannoc, I'd like to buy a room`: the first word "brannoc" is not a lead-in, so isPlainPurchase returns false. The landlord only recites his terms and the player must ask again. The same sentence addressed to Hobb ("Hobb, I'd like to buy a room") completes the purchase.

**Existing mechanism.** Building.LandlordMobId and the mob spec name (housing.landlordName)

**Suggested fix.** Remove the hardcoded names. Have actBuyHousing pass the landlord's name words, from mob.Character.Name, into isPlainPurchase as extra lead-ins.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
actions_housing.go:67-73 `var purchaseLeadIns = map[string]bool{ ... `hobb`: true, `mr`: true, `pennock`: true, }`; landlord_mob_id 9801-9804 across the four housing_buildings YAMLs.
```

- **confirmed** (low): The claim holds as written. purchaseLeadIns (actions_housing.go:67-73) includes `hobb`, `mr` and `pennock` alongside generic filler words. isPlainPurchase skips any lead-in words and returns true only when the first word after them is buy or purchase. buy_housing is the shared action for all four landlord trees (9801 to 9804), and only Hobb's name gets skipped. The ask command (usercommands/ask.go:124-189) removes the target mob and an optional leading "to" or "about", then passes the rest as Event.Text. So `ask brannoc Brannoc, I'd like to buy a room` reaches the action as "Brannoc, I'd like to buy a room". The comma is trimmed, "brannoc" is not a lead-in, and isPlainPurchase returns false. Brannoc then recites his terms through DescribeTerms and nothing is bought. The same sentence starting "Hobb," completes the purchase. This also fits the unification complaint: per-landlord data is hardcoded in shared code, while the building's landlord_mob_id and the mob's name are already available. Severity stays low: the failure is safe (no charge, the terms are shown, and they explain how to buy) and only affects players who address the landlord by name inside their sentence.

</details>

<a id="f078"></a>
### F078 [low] Housing prices and limits copied into each building YAML instead of being balance knobs

`internal/housing/housing.go:98` · status **confirmed** · reported as medium

Every economic number housing uses is a per-building authored field: tier price, extension_price_multiplier, max_rooms, redecorate_price, guest_key_price, max_guests, container_price, strongbox_price, max_containers_per_room, bed_price and station_price. The four shipped buildings repeat the same values (500, x3, 8, 500, 100, 5, 250, 500, 6, 250, 1500) three times over. Hollow Oak's values are hand-computed quarters (125, 25, 63, 125, 63, 375), with a comment saying "Every price is a quarter". The project rule is that balance numbers live in config.balance.go and are surfaced through config.yaml. The nearest precedent, a one-off gold sink for founding a guild, is a balance knob (GuildFoundingCost, read in guild.go:124). Retuning the housing economy now means editing four content files in step and recomputing the quarter ratio by hand, with nothing to keep them consistent.

**Failure scenario.** The owner decides city lodgings are too cheap and raises the home price to 800 via config.yaml, the documented retuning path. Nothing changes, because no knob exists. A content edit to back_court_lodgings.yaml alone then leaves the Quillhouse and Burrows at 500, and Hollow Oak stays at 125 instead of 200. Players in different cities pay inconsistent prices, and nothing flags the drift.

**Existing mechanism.** configs.BalanceConfig knobs in internal/configs/config.balance.go, surfaced in _datafiles/config.yaml (precedent: GuildFoundingCost)

**Suggested fix.** Add balance knobs for the base prices, the extension multiplier and the limits. Let a building carry an optional price_scale (Hollow Oak 0.25) and optional per-field overrides, and compute prices from the knobs.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
grep of _datafiles/world/dogmud/housing_buildings/*.yaml: back_court, quillhouse and the_burrows each have `price: 500`, `extension_price_multiplier: 3`, `max_rooms: 8`, `guest_key_price: 100`, `container_price: 250`, `strongbox_price: 500`, `bed_price: 250`, `station_price: 1500`; hollow_oak.yaml:7 "Every price is a quarter" with 125/25/63/125/63/375. guild.go:124 `cost := int(configs.GetBalanceConfig().GuildFoundingCost)`. `grep -n Housing internal/configs/config.balance.go` returns nothing.
```

- **confirmed** (low): The facts in the claim are accurate. internal/housing/housing.go (added by the PR, so it was not in c696c117a) declares every economic number as a per-building YAML field: Tier.Price, ExtensionPriceMultiplier, MaxRooms, RedecoratePrice, GuestKeyPrice, MaxGuests, ContainerPrice, StrongboxPrice, MaxContainersPerRoom, BedPrice and StationPrice. back_court_lodgings, quillhouse and the_burrows all repeat 500/x3/8/500/100/5/250/500/6/250/1500. hollow_oak.yaml carries hand-computed quarters (125/25/63/125/63/375) under the comment "Every price is a quarter of a city's". Neither config.balance.go nor any config.balance*.go file has a Housing knob.  The framing as a medium unification defect is overstated, for three reasons. (1) GuildFoundingCost is a weak precedent. It is one global value for one action. Housing buildings are separate content entities, and their prices are meant to differ: Hollow Oak is cheaper by design. (2) The codebase already authors per-entity prices on content YAML at scale. Items (materials, weapons, consumables, armor), spells and mutations all carry price, cost or value fields in YAML. So per-building pricing in a building YAML follows an existing pattern rather than rewriting one. (3) The dogmud-balance-config rule targets hardcoded Go numbers. Data-driven content values are not hardcoded. Validate() also rejects non-positive or incoherent values (price <= 0, multiplier < 1, max_rooms < tier rooms).  The failure scenario needs the owner to expect a config knob that was never documented for housing, so nothing silently breaks.  What does reproduce is a maintainability problem. The three city buildings duplicate identical numbers with no shared default, and the quarter ratio lives only in a comment, so a retune has to edit several files in step. That is real duplication but low severity. A shared default block, or a config multiplier for the building tier, would address it.

</details>

<a id="f079"></a>
### F079 [low] Small re-implementations: direction deltas, capitalise, placeholder filling and gold-then-bank charging

`internal/housing/housing.go:550` · status **confirmed** · reported as low

Housing redefines helpers that already exist elsewhere:
- directionDeltas and oppositeDirection repeat the mapper's posDeltas and mapper.GetDelta in the same engine frame (north is y-1, up is z+1). The comment even says it matches the mapper.
- capitalise (terms.go:68) duplicates capitalize helpers found in four other packages.
- Building.Line does its own regex placeholder substitution, while narration.Substitute already handles brace tokens in a single pass.
- chargeGold (purchase.go:156) repeats jail.go's fine-paying logic (carried gold first, then bank). Housing's version also raises EquipmentChange; jail's does not.
None of these is broken today. The risk is that a change to one copy, such as a new direction or a change to the delta frame, is not mirrored in the others.

**Failure scenario.** If the mapper's frame or its direction set changes (for example adding the diagonal directions it already supports to house growth), housing's offsets() and checkLinks() keep the old table. House rooms are then laid out differently from what the mapper draws, and checkLinks may hold valid houses.

**Existing mechanism.** mapper.GetDelta (internal/mapper/mapper.go:163); narration.Substitute (internal/narration/render.go:146)

**Suggested fix.** Use mapper.GetDelta, with a small opposite-direction helper next to it. Fill placeholders with narration.Substitute while keeping housing's validation. Lift chargeGold into a shared character method that jail.go also uses.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
housing.go:550 `var directionDeltas = map[string][3]int{ `north`: {0, -1, 0}, ...}` vs mapper.go:42 `"north": {0, -1, 0, '│'}` and `func GetDelta(exitName string) (x, y, z int)` at mapper.go:163; terms.go:68 `func capitalise`; voice.go placeholderRe vs narration/render.go:146 `func Substitute`; purchase.go:156 chargeGold vs usercommands/jail.go:71-79.
```

- **confirmed** (low): Every cited duplicate exists as described, and every one is new code from this PR (git diff c696c117a..HEAD shows internal/housing and internal/usercommands/house.go as pure additions). The direction table copies the mapper's: housing's directionDeltas (housing.go:550) holds the same six values as mapper posDeltas (mapper.go:41-51). The mapper already exports GetDelta (mapper.go:163) and IsValidExitDirection, and the comment at housing.go:548 says the table matches the mapper. Building.Line (voice.go:115-138) fills placeholders with its own regex, though narration.Substitute (render.go:146) already does single-pass brace-token replacement; the one thing Line adds is leaving unknown tokens untouched, which Substitute's Replacer also does. chargeGold (purchase.go:156) repeats jail.go's gold-then-bank fine logic (jail.go:71-79), and housing's version also queues an EquipmentChange (purchase.go:129). The capitalise duplication is slightly worse than the claim says: the PR adds two copies of its own, capitalise in housing/terms.go:68 and capitaliseFirst in usercommands/house.go:154, next to the existing ones in ferry, messaging, achievements, aicompanion and weather. None of this is broken today, so the risk is only that a future change to one copy is not made in the others. Low severity is right. One caveat on the failure scenario: GetDelta returns 0,0,0 for an unknown name, so housing would still need its own allowed-direction set (or IsValidExitDirection). Pulling in the mapper's diagonals would also change house growth on purpose, so for that example the fix is a design choice, not a mechanical swap.

</details>

<a id="f136"></a>
### F136 [low] items.SetNeverBought is a parallel runtime registry alongside the existing spec.NotSalable flag and EvaluateBuyRules

`internal/items/never_bought.go:16` · status **refuted** · reported as low

To stop housing items being sold, the PR adds a new global set (items.SetNeverBought / IsNeverBought), filled at runtime by housing.registerNeverBought. It also adds a housing-specific refusal line to the generic sellOneToMerchant. The item spec already has NotSalable, and housing's own loader panics unless every housing item has `not_salable: true` and no vendor_categories. With no vendor_categories, the canonical buy gate shops.EvaluateBuyRules already refuses the item (buyrules.go: `if len(spec.VendorCategories) == 0 { return BuyOffer{} }`). So the new registry duplicates a gate that already holds, and leaves NotSalable as a validation-only flag that sell never reads. The "what may a merchant buy" decision is now split across the spec flag, the buy rules and a registry that exists only while housing is loaded. offer.go does not consult the registry, so `offer deed` and `sell deed` give different refusals.

**Failure scenario.** A future system adds its own gold-sink items and marks them not_salable, the existing spec flag. Sell ignores the flag, so the author must also discover and call items.SetNeverBought. That call replaces the whole set, so two subsystems calling it would erase each other's entries: housing's rebuild on /reload would wipe the other system's ids. Meanwhile `offer <deed>` says the merchant is not interested, while `sell <deed>` says "That's lodging-house paper".

**Existing mechanism.** ItemSpec.NotSalable (internal/items/itemspec.go) together with shops.EvaluateBuyRules (internal/shops/buyrules.go)

**Suggested fix.** Have sellOneToMerchant and offer refuse spec.NotSalable items, with a generic line or an optional per-spec refusal string. Then delete the never_bought registry and the housing-specific text in the generic sell path.

<details><summary>Evidence and verifier votes</summary>

**Evidence.**

```
never_bought.go:16 `func SetNeverBought(itemIds []int) { ... neverBought = set` (whole-set replace); registry.go:114 registerNeverBought; sell.go:339-341 `if items.IsNeverBought(itemSpec.ItemId) { merchantSay(room, mob, "That's lodging-house paper...")`; registry.go:235 `if !spec.NotSalable || len(spec.VendorCategories) > 0 { panic(...) }`; `git grep NotSalable HEAD` shows no reader in actions or shops; shops/buyrules.go rejects empty VendorCategories.
```

- **refuted** (none): The claim's main premise is that EvaluateBuyRules "already refuses" housing items, which would make the new registry a duplicate. That premise is false. In internal/actions/sell.go, sellOneToMerchant reaches shops.EvaluateBuyRules on only one branch: the item is not affixed and shopInv != nil. Two other paths pay for an item without ever checking VendorCategories or NotSalable: (1) When shopInv == nil, the price comes from mob.GetSellPrice (internal/mobs/mobs.go:1116). That function prices from the merchant's stock types and item.GetSpec().Value. registry.go's own comment says the loader replaces an authored value of 0 with an automatic one, so a merchant that stocks items of the same type would pay for a deed. (2) Stolen items go to sellStolenToFence, which pays FencePrice(spec.Value) and also skips the buy rules. The IsNeverBought check runs before both of these branches, so it closes holes that the buy rules do not cover.  Making sell read spec.NotSalable instead is not an obvious drop-in fix. itemspec.go documents NotSalable as a flag for lore, flavor and legacy items "excluded from vendor economy validation". Gating sell on it would change how every such item sells, not only housing items.  The rest of the claim does not hold up either: - Two subsystems erasing each other's entries is hypothetical. SetNeverBought has exactly one caller (housing.registerNeverBought). - The `offer deed` scenario is wrong. offer.go uses mob.GetSellPrice, so `offer` may quote gold for a deed rather than say "not interested". That gap in offer existed before this PR: offer never used EvaluateBuyRules, so it is not a split this registry created.  What is left is a style nit: a global replace-semantics set with one caller. It is not a duplicate of a gate that already holds.

</details>
