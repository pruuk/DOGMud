package actions

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// DrinkActor is an Actor that can receive a potion's conditions at a scaled
// duration or an exact magnitude. It is its own interface, not two new Actor
// methods, because eight test fakes implement Actor and none of them drinks.
type DrinkActor interface {
	Actor
	AddConditionScaled(conditionId int, durationMult float64, source string)
	AddConditionMagnitude(conditionId int, triggers int, magnitude float64, source string)
}

// DrinkRefusal says why a drink did not happen.
type DrinkRefusal int

const (
	DrinkOK DrinkRefusal = iota
	DrinkRefuseBusy
	DrinkRefuseGrappled
	DrinkRefuseNotFound
	DrinkRefuseNotDrinkable
	DrinkRefuseToxicity
)

// DrinkResult reports one drink. Drank is true when the item was consumed,
// including a spoiled potion.
type DrinkResult struct {
	Drank   bool
	Spoiled bool
	ItemId  int
	Refusal DrinkRefusal
}

// bloomWaferItemId is the item ID for the Bloom Wafer (40108).
// The wafer's effect (Communion condition, addiction tick, mutation roll) is
// handled as a special case in Drink rather than through the generic conditionids
// path, because it also needs to stamp BloomLastDoseRound and call
// BloomAdvanceMutation.
const bloomWaferItemId = 40108

// ysoldesPurgeItemId is the item ID for Ysolde's Purge (40109).
// The purge carries a heavy toxicity load (26) and condition 93 (Bloom Detox)
// through the normal drink path. Its special-case here drives the addiction
// step-down. It also bypasses the toxicity pre-check: addicts presenting
// for detox are expected to already have elevated toxicity, and the flood is
// the mechanism, not a mistake.
const ysoldesPurgeItemId = 40109

// purgingDraughtItemId is the item ID for the Purging Draught (30052), the
// other detox besides Ysolde's Purge.
const purgingDraughtItemId = 30052

// The original potion-effect conditions occupy a contiguous id block. It
// stays as the floor of what a purge strips (so a test binary with no items
// loaded keeps today's behaviour), and purgeableConditionIds adds every
// condition only a potion grants (lighting plan 5c closed the leak: 7, 44, 47,
// 48, 49, 51 and 82 had escaped the block). 76 is the purge's own weakness harmful
// condition: it is APPLIED by a purge, never stripped by one. 70 is the
// draught's flavour condition, which carries no statmods and expires after a
// round; a purge leaves it alone so the drinker still sees that they drank
// something.
const (
	potionConditionIdMin       = 54
	potionConditionIdMax       = 75
	purgingDraughtConditionId  = 70
	purgingWeaknessConditionId = 76
)

// bypassesToxicityGate reports whether an item skips the pre-check that refuses
// a potion which would push the drinker past their tolerance.
//
// Detox items MUST skip it. They are drunk precisely when toxicity is high, so
// gating them denies the cure to the poisoned -- and since the Purging Draught
// costs more than a fresh character's entire tolerance, gating it makes the
// item unusable outright rather than merely awkward.
func bypassesToxicityGate(itemId int) bool {
	return itemId == ysoldesPurgeItemId || itemId == purgingDraughtItemId
}

// purgeableConditionIds is what a purge strips: the original block plus every
// condition only a potion grants, minus the conditions of both detox items
// (one detox does not undo another) and the weakness a purge applies.
func purgeableConditionIds() map[int]bool {
	set := items.PotionEffectConditionIds()
	for id := potionConditionIdMin; id <= potionConditionIdMax; id++ {
		set[id] = true
	}
	for _, detoxId := range []int{purgingDraughtItemId, ysoldesPurgeItemId} {
		if spec := items.GetItemSpec(detoxId); spec != nil {
			for _, id := range spec.ConditionIds {
				delete(set, id)
			}
		}
	}
	delete(set, purgingDraughtConditionId)
	delete(set, purgingWeaknessConditionId)
	return set
}

// applyPurgeEffects undoes alchemy: it strips the potion effects the drinker is
// carrying, clears the toxicity those potions cost, and leaves them weakened
// for it.
//
// All three have to happen here. The draught declares only condition 70, which has a
// description and no statmods, and there is no condition scripting layer -- so none
// of the item's advertised behaviour existed. Condition 76 was authored with the
// intended penalty and wired to nothing.
//
// It takes the actor, not the character, because the weakness has to be
// added through the event path: applying it with Character.AddConditionScaled queued
// nothing, so Condition_ApplyConditions never ran and the drinker read no line for a
// fifty-round harmful condition. The strips and the toxicity clear stay synchronous.
func applyPurgeEffects(actor DrinkActor) {
	c := actor.GetCharacter()
	for id := range purgeableConditionIds() {
		c.RemoveCondition(id)
	}
	c.Toxicity = 0
	actor.AddConditionScaled(purgingWeaknessConditionId, 1.0, `drink`)
}

// catalystOfUnmakingItemId is #22 crash-site: drinking it scours ALL mutations
// back to species intrinsics and biases re-acquisition hard toward rare.
const catalystOfUnmakingItemId = 30067

// scourRerollCharges is how many rare-biased reroll charges the Catalyst grants.
const scourRerollCharges = 3

// phialOfSecondBirthItemId is the pinnacle remort potion (Stage 2 authors the
// item YAML with this ID). Scours ALL mutations to species base, then grants
// exactly one mutation from a rarity-floored pool -- a repeatable gold sink.
const phialOfSecondBirthItemId = 40181

// phialRarityFloor is the minimum Rarity a mutation must have to be eligible
// for the phial's grant.
const phialRarityFloor = 5

// Drink is the one drink body for players and mobs, the AI companion included
// (drink path unification). It was usercommands.Drink; the mob command was a
// separate 49-line copy with no toxicity, no aging or crafter scaling and no
// special potions. Every special potion applies fully to a mob (owner ruling,
// 2026-09-28). SendText is a no-op for a mob, so a mob drinks silently apart
// from the room line.
func Drink(actor DrinkActor, rest string) DrinkResult {
	char := actor.GetCharacter()

	room := actor.GetRoom()
	if room == nil {
		room = rooms.LoadRoom(char.RoomId)
	}

	// The room line names a player in the username colour and excludes them
	// (they read their own line); a mob is named in the mob colour and the
	// line goes to everyone, as the mob path always sent it. Both room lines
	// go out through SendTextVisualHidingNames with the drinker's name, so an
	// observer at shapes reads "a figure" even if the name ever loses its tag.
	nameColor := `mobname`
	var roomExclude []int
	if actor.IsPlayer() {
		nameColor = `username`
		roomExclude = []int{actor.GetUserId()}
	}

	// Nothing named: ask what, as get, drop and sell do (2026-10-17
	// playtest: a bare `drink` read "You aren't carrying that.").
	if strings.TrimSpace(rest) == `` {
		refusal.Prompt(actor, `drink`, refusal.What, `drink <item>`)
		return DrinkResult{Refusal: DrinkRefuseNotFound}
	}

	if char.IsActing() {
		refusal.WhileBusy(actor, `drink`)
		return DrinkResult{Refusal: DrinkRefuseBusy}
	}

	// Chunk 4e: can't drink while grappled, both hands committed.
	if char.Position != nil && char.Position.IsGrappling() {
		actor.SendText(messaging.CategoryRefusal, `<ansi fg="red">Your hands are committed to the grapple: you can't reach for that.</ansi>`)
		return DrinkResult{Refusal: DrinkRefuseGrappled}
	}

	// Search bandolier first (oldest first), then backpack. The backpack
	// pass is drinkable-first (skip same-noun non-drinkables) with an
	// unfiltered fallback so the "can't drink that" rejection still fires
	// when nothing drinkable matches.
	fromBandolier := false
	matchItem, found := char.FindInPotions(rest)
	if found {
		fromBandolier = true
	} else {
		matchItem, found = char.FindInBackpackWhere(rest, func(it items.Item) bool {
			return it.GetSpec().Subtype == items.Drinkable
		})
		if !found {
			matchItem, found = char.FindInBackpack(rest)
		}
	}

	if !found {
		refusal.NotCarrying(actor, rest)
		return DrinkResult{Refusal: DrinkRefuseNotFound}
	}

	itemSpec := matchItem.GetSpec()

	if itemSpec.Subtype != items.Drinkable {
		actor.SendText(messaging.CategoryRefusal,
			fmt.Sprintf(`You can't drink <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()),
		)
		return DrinkResult{Refusal: DrinkRefuseNotDrinkable, ItemId: matchItem.ItemId}
	}

	// Compute aging phase if the potion has aging data
	var phase items.AgingPhase
	var potencyMult float64 = 1.0
	hasAging := itemSpec.Aging.HasAging() && matchItem.CraftedRound > 0

	if hasAging {
		elapsed := util.GetRoundCount() - matchItem.CraftedRound
		bottleMult := matchItem.BottleMultiplier
		if bottleMult <= 0 {
			bottleMult = itemSpec.BottleAgingMultiplier
		}
		effSpeed := items.CalcEffectiveAgingSpeed(bottleMult, matchItem.CraftSkill)
		phase, potencyMult = items.GetAgingPhase(elapsed, itemSpec.Aging, effSpeed)
	}

	// Handle spoiled potions
	if hasAging && phase == items.PhaseSpoiled {
		// Spoiled potions apply 3x toxicity
		spoiledTox := float64(itemSpec.Toxicity) * 3.0
		char.AddToxicity(spoiledTox)

		char.CancelConditionsWithFlag(conditions.Hidden)

		// Consume the item
		if fromBandolier {
			char.UseItemFromPotions(matchItem)
		} else {
			char.UseItem(matchItem)
		}

		actor.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You drink the <ansi fg="itemname">%s</ansi>...`, matchItem.DisplayName()))
		actor.SendText(messaging.CategorySystem,
			`<ansi fg="red">The potion has gone bad! You retch as the foul liquid burns your throat.</ansi>`)
		if room != nil {
			room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(
				`<ansi fg="%s">%s</ansi> drinks something and immediately gags.`,
				nameColor, char.Name), []string{char.Name}, roomExclude...)
		}

		// Apply nausea harmful condition (condition 75) through the event, so the holder reads
		// its start line.
		actor.AddCondition(75, `drink`)

		// Recipe discovery chance: 10% + (alchemySkill * 0.5)%
		alchSkill := char.GetSkillLevel(skills.Alchemy)
		discoveryChance := 10.0 + float64(alchSkill)*0.5
		if float64(util.Rand(100)) < discoveryChance {
			actor.SendText(messaging.CategorySystem,
				`<ansi fg="yellow">The foul taste teaches you something about how the ingredients interact...</ansi>`)
		}

		return DrinkResult{Drank: true, Spoiled: true, ItemId: matchItem.ItemId}
	}

	// Check toxicity before consuming.
	// Exception: both detox items (Ysolde's Purge and the Purging Draught)
	// bypass this cap -- the toxicity flood is intentional and a detox must be
	// drinkable even at high toxicity, or it can never be drunk at all.
	if itemSpec.Toxicity > 0 && !bypassesToxicityGate(itemSpec.ItemId) {
		toxCost := float64(itemSpec.Toxicity)
		if char.Toxicity+toxCost > char.GetToxicityMax() {
			actor.SendText(messaging.CategoryRefusal,
				`<ansi fg="red">Your body rejects the potion: too much toxicity.</ansi>`)
			return DrinkResult{Refusal: DrinkRefuseToxicity, ItemId: matchItem.ItemId}
		}
	}

	char.CancelConditionsWithFlag(conditions.Hidden)

	// Consume the item
	if fromBandolier {
		char.UseItemFromPotions(matchItem)
	} else {
		char.UseItem(matchItem)
	}

	// Apply toxicity
	if itemSpec.Toxicity > 0 {
		char.AddToxicity(float64(itemSpec.Toxicity))
	}

	// Quest engine: command notification. A successful drink advances
	// "drink a potion" quest steps (e.g. the Spoke C alchemy cert). Only a
	// player has quests.
	if ua, ok := actor.(*UserActor); ok && room != nil {
		questBridge := questengine.NewGameBridge(ua.User, room.RoomId)
		questengine.GetEngine().Notify("command", questengine.EventDetails{
			UserId:  ua.User.UserId,
			RoomId:  room.RoomId,
			Command: "drink",
		}, questBridge, questBridge)
	}

	actor.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You drink the <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()))
	if room != nil {
		room.SendTextVisualHidingNames(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="%s">%s</ansi> drinks <ansi fg="itemname">%s</ansi>.`,
			nameColor, char.Name, matchItem.DisplayName()), []string{char.Name}, roomExclude...)
	}

	// Aging quality message
	if hasAging {
		switch phase {
		case items.PhaseFresh:
			actor.SendText(messaging.CategorySystem, `The potion is freshly brewed. It should do the job.`)
		case items.PhaseFermented:
			actor.SendText(messaging.CategorySystem, `The potion has fermented nicely. You feel it working stronger than expected.`)
		case items.PhasePeak:
			actor.SendText(messaging.CategorySystem, `<ansi fg="green">The potion is at its peak. You feel its full potency.</ansi>`)
		case items.PhaseDeclining:
			actor.SendText(messaging.CategorySystem, `The potion tastes a bit stale. Its effects are diminished.`)
		}
	}

	// Calculate final duration multiplier:
	// potencyMult (from aging phase) * craftSkill scaling
	durationMult := potencyMult
	if matchItem.CraftSkill > 0 {
		durationMult *= 1.0 + float64(matchItem.CraftSkill)/100.0
	}

	// Apply conditions with scaled duration. Scaled and unscaled both go through the
	// actor, so Condition_ApplyConditions runs and the drinker reads the condition's start line;
	// the multiplier rides on the event. Applying the scaled case through
	// Character.AddConditionScaled instead is what made Purging Weakness silent.
	for _, conditionId := range itemSpec.ConditionIds {
		conditionSpec := conditions.GetConditionSpec(conditionId)
		if mag, trig, ok := items.PotionMagnitudeApplication(&itemSpec, conditionSpec, durationMult); ok {
			// A magnitude-scaled potion (lighting plan 5c) queues through the
			// same event door with its value and count; the holder still reads
			// the start line.
			actor.AddConditionMagnitude(conditionId, trig, mag, `drink`)
		} else {
			actor.AddConditionScaled(conditionId, durationMult, `drink`)
		}
		// No tick snapshot here: a tick_pool condition's per-round amount is
		// computed where it lands (Condition_ApplyConditions), at scale 1.0
		// for a potion, which passes none.
	}

	// ── Ysolde's Purge special-case ──────────────────────────────────────────
	// Toxicity (26) and the detox harmful condition (condition 93) are applied by the normal
	// drink path above. Here we drive the addiction step-down -- the brutal-
	// fast path to clean.
	if itemSpec.ItemId == ysoldesPurgeItemId {
		char.AddBloomAddiction(-5)
		actor.SendText(messaging.CategoryWarning,
			`The purge takes hold. Your body convulses as it expels the `+
				`Bloom. It is violent, and it is fast.`)
	}

	// ── Purging Draught special-case ─────────────────────────────────────────
	// The draught declares only condition 70, which is a flavour line with no
	// statmods, so every effect it advertises has to be wired here -- exactly
	// as Ysolde's Purge and the Bloom Wafer are. Its own toxicity was applied
	// by the normal path above and is cleared again here, which is correct: you
	// cannot pay a toxicity price for the thing that removes toxicity.
	if itemSpec.ItemId == purgingDraughtItemId {
		applyPurgeEffects(actor)
		// Warning is not a category the pipeline wraps (messaging.shouldWrap),
		// so the line is wrapped here, the way the command package wrapped prose.
		actor.SendText(messaging.CategoryWarning, util.SplitStringNL(
			`The draught tears through you. Every trace of potion work is `+
				`scoured out, and you are left shaking and hollow.`, 80))
	}

	// ── Bloom Wafer special-case ──────────────────────────────────────────────
	// The wafer has no conditionids in its YAML; all Bloom effects are wired here.
	// Toxicity (20) was already applied by the normal path above; don't
	// apply it again. The order relative to the condition loop above doesn't matter
	// since the loop is empty for this item.
	if itemSpec.ItemId == bloomWaferItemId {
		bal := configs.GetBalanceConfig()

		// Communion high. Condition 90's YAML baseline is 30 rounds; scale it by the
		// BloomCommunionRounds knob so config actually tunes the duration.
		communionMult := float64(bal.BloomCommunionRounds) / 30.0
		if communionMult <= 0 {
			communionMult = 1.0
		}
		actor.AddConditionScaled(90, communionMult, `drink`)

		// Tick addiction counter.
		char.AddBloomAddiction(int(bal.BloomAddictionPerDose))

		// Stamp the dose round for withdrawal / decay timing.
		char.BloomLastDoseRound = util.GetRoundCount()

		// Mutation acceleration. First roll the (small) BloomNewMutationChance to
		// push a brand-new change even if the drinker already has mutations (Bloom's
		// "occasionally something wholly new" variety). Otherwise roll the (larger)
		// BloomMutationAdvanceChance to deepen the strongest existing mutation
		// (which falls through to seeding when the drinker has none / all are capped).
		var mutId string
		if rand.Float64() < float64(bal.BloomNewMutationChance) {
			mutId, _ = char.BloomSeedNewMutation(nil)
		} else if rand.Float64() < float64(bal.BloomMutationAdvanceChance) {
			mutId, _ = char.BloomAdvanceMutation(nil)
		}
		if mutId != "" {
			actor.SendText(messaging.CategoryWarning,
				`Something under your skin shifts and settles differently.`)
		}

		// Euphoric onset message. It follows the generic "you drink" that was
		// already sent above, and is sent last so it reads as the climax of the
		// consume sequence.
		actor.SendText(messaging.CategoryWarning,
			`The wafer dissolves to nothing on your tongue and the world goes `+
				`warm and wide: communion.`)
	}

	// ── Catalyst of Unmaking special-case ─────────────────────────────────────
	// #22 crash-site "remort": scour every acquired mutation back to species
	// intrinsics and grant rare-biased reroll charges. The potion was already
	// consumed by the normal path above; here we only apply the effect and
	// send the onset message (mirrors the Bloom Wafer block's shape).
	if itemSpec.ItemId == catalystOfUnmakingItemId {
		char.ScourMutations(scourRerollCharges)
		actor.SendText(messaging.CategoryWarning,
			`<ansi fg="magenta">You drink the Catalyst. For one breath you are only what you `+
				`were born as: every woken thing in your blood goes still and gone. Then the `+
				`cold lets go, and the hunger comes back stronger than before.</ansi>`)
	}

	// ── Phial of Second Birth special-case ────────────────────────────────────
	// Pinnacle remort potion: scour every acquired mutation back to species
	// intrinsics (no reroll charges -- the grant below is immediate), then
	// grant exactly one mutation from the rarity-floored pool.
	if itemSpec.ItemId == phialOfSecondBirthItemId {
		char.ScourMutations(0)
		granted := char.GrantRandomMutationRare(phialRarityFloor)
		if granted != "" {
			if spec := mutations.GetMutation(granted); spec != nil {
				actor.SendText(messaging.CategoryWarning, fmt.Sprintf(
					`<ansi fg="magenta">Your flesh unwrites itself: every change the Chrysalis ever made dissolves. Then, from the stillness, something singular takes root: <ansi fg="yellow">%s</ansi>.</ansi>`, spec.Name))
			}
		} else {
			actor.SendText(messaging.CategoryWarning,
				`<ansi fg="magenta">Your flesh unwrites itself: every change dissolves. The stillness holds; nothing new takes root.</ansi>`)
		}
	}

	return DrinkResult{Drank: true, ItemId: matchItem.ItemId}
}
