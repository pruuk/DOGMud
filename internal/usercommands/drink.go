package usercommands

import (
	"fmt"
	"math/rand"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// bloomWaferItemId is the item ID for the Bloom Wafer (40108).
// The wafer's effect (Communion buff, addiction tick, mutation roll) is
// handled as a special case in Drink rather than through the generic buffids
// path, because it also needs to stamp BloomLastDoseRound and call
// BloomAdvanceMutation.
const bloomWaferItemId = 40108

// ysoldesPurgeItemId is the item ID for Ysolde's Purge (40109).
// The purge carries a heavy toxicity load (26) and buff 93 (Bloom Detox)
// through the normal drink path. Its special-case here drives the addiction
// step-down. It also bypasses the toxicity pre-check — addicts presenting
// for detox are expected to already have elevated toxicity, and the flood is
// the mechanism, not a mistake.
const ysoldesPurgeItemId = 40109

// purgingDraughtItemId is the item ID for the Purging Draught (30052), the
// other detox besides Ysolde's Purge.
const purgingDraughtItemId = 30052

// Potion-effect buffs occupy a contiguous id block. 76 is the purge's own
// weakness debuff: it is APPLIED by a purge, never stripped by one. 70 is the
// draught's flavour buff, which carries no statmods and expires after a round;
// a purge leaves it alone so the drinker still sees that they drank something.
const (
	potionBuffIdMin       = 54
	potionBuffIdMax       = 75
	purgingDraughtBuffId  = 70
	purgingWeaknessBuffId = 76
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

// applyPurgeEffects undoes alchemy: it strips the potion effects the drinker is
// carrying, clears the toxicity those potions cost, and leaves them weakened
// for it.
//
// All three have to happen here. The draught declares only buff 70, which has a
// description and no statmods, and there is no buff scripting layer -- so none
// of the item's advertised behaviour existed. Buff 76 was authored with the
// intended penalty and wired to nothing.
//
// It takes the user record, not the character, because the weakness has to be
// added through the event path: applying it with Character.AddBuffScaled queued
// nothing, so Buff_ApplyBuffs never ran and the drinker read no line for a
// fifty-round debuff. The strips and the toxicity clear stay synchronous.
func applyPurgeEffects(u *users.UserRecord) {
	c := u.Character
	for id := potionBuffIdMin; id <= potionBuffIdMax; id++ {
		if id == purgingDraughtBuffId {
			continue
		}
		c.RemoveBuff(id)
	}
	c.Toxicity = 0
	u.AddBuffScaled(purgingWeaknessBuffId, 1.0, `drink`)
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

func Drink(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	if refuseWhileBusy(user, `drink`) {
		return true, nil
	}

	// Chunk 4e: can't drink while grappled — both hands committed.
	if user.Character.Position != nil && user.Character.Position.IsGrappling() {
		user.SendText(messaging.CategorySystem, `<ansi fg="red">Your hands are committed to the grapple — you can't reach for that.</ansi>`)
		return true, nil
	}

	// Search bandolier first (oldest first), then backpack. The backpack
	// pass is drinkable-first — skip same-noun non-drinkables — with an
	// unfiltered fallback so the "can't drink that" rejection still fires
	// when nothing drinkable matches.
	fromBandolier := false
	matchItem, found := user.Character.FindInPotions(rest)
	if found {
		fromBandolier = true
	} else {
		matchItem, found = user.Character.FindInBackpackWhere(rest, func(it items.Item) bool {
			return it.GetSpec().Subtype == items.Drinkable
		})
		if !found {
			matchItem, found = user.Character.FindInBackpack(rest)
		}
	}

	if !found {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You don't have a "%s" to drink.`, rest))
		return true, nil
	}

	itemSpec := matchItem.GetSpec()

	if itemSpec.Subtype != items.Drinkable {
		user.SendText(messaging.CategorySystem,
			fmt.Sprintf(`You can't drink <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()),
		)
		return true, nil
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
		user.Character.AddToxicity(spoiledTox)

		user.Character.CancelBuffsWithFlag(buffs.Hidden)

		// Consume the item
		if fromBandolier {
			user.Character.UseItemFromPotions(matchItem)
		} else {
			user.Character.UseItem(matchItem)
		}

		user.SendText(messaging.CategorySystem, fmt.Sprintf(
			`You drink the <ansi fg="itemname">%s</ansi>...`, matchItem.DisplayName()))
		user.SendText(messaging.CategorySystem,
			`<ansi fg="red">The potion has gone bad! You retch as the foul liquid burns your throat.</ansi>`)
		room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
			`<ansi fg="username">%s</ansi> drinks something and immediately gags.`,
			user.Character.Name), user.UserId)

		// Apply nausea debuff (buff 75) through the event, so the holder reads
		// its start line.
		user.AddBuff(75, `drink`)

		// Recipe discovery chance: 10% + (alchemySkill * 0.5)%
		alchSkill := user.Character.GetSkillLevel(skills.Alchemy)
		discoveryChance := 10.0 + float64(alchSkill)*0.5
		if float64(util.Rand(100)) < discoveryChance {
			user.SendText(messaging.CategorySystem,
				`<ansi fg="yellow">The foul taste teaches you something about how the ingredients interact...</ansi>`)
		}

		return true, nil
	}

	// Check toxicity before consuming.
	// Exception: both detox items (Ysolde's Purge and the Purging Draught)
	// bypass this cap -- the toxicity flood is intentional and a detox must be
	// drinkable even at high toxicity, or it can never be drunk at all.
	if itemSpec.Toxicity > 0 && !bypassesToxicityGate(itemSpec.ItemId) {
		toxCost := float64(itemSpec.Toxicity)
		if user.Character.Toxicity+toxCost > user.Character.GetToxicityMax() {
			user.SendText(messaging.CategorySystem,
				`<ansi fg="red">Your body rejects the potion — too much toxicity.</ansi>`)
			return true, nil
		}
	}

	user.Character.CancelBuffsWithFlag(buffs.Hidden)

	// Consume the item
	if fromBandolier {
		user.Character.UseItemFromPotions(matchItem)
	} else {
		user.Character.UseItem(matchItem)
	}

	// Apply toxicity
	if itemSpec.Toxicity > 0 {
		user.Character.AddToxicity(float64(itemSpec.Toxicity))
	}

	// Quest engine: command notification — a successful drink advances
	// "drink a potion" quest steps (e.g. the Spoke C alchemy cert).
	questBridge := questengine.NewGameBridge(user, room.RoomId)
	questengine.GetEngine().Notify("command", questengine.EventDetails{
		UserId:  user.UserId,
		RoomId:  room.RoomId,
		Command: "drink",
	}, questBridge, questBridge)

	user.SendText(messaging.CategorySystem, fmt.Sprintf(
		`You drink the <ansi fg="itemname">%s</ansi>.`, matchItem.DisplayName()))
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> drinks <ansi fg="itemname">%s</ansi>.`,
		user.Character.Name, matchItem.DisplayName()), user.UserId)

	// Aging quality message
	if hasAging {
		switch phase {
		case items.PhaseFresh:
			user.SendText(messaging.CategorySystem, `The potion is freshly brewed — it should do the job.`)
		case items.PhaseFermented:
			user.SendText(messaging.CategorySystem, `The potion has fermented nicely — you feel it working stronger than expected.`)
		case items.PhasePeak:
			user.SendText(messaging.CategorySystem, `<ansi fg="green">The potion is at its peak — you feel its full potency.</ansi>`)
		case items.PhaseDeclining:
			user.SendText(messaging.CategorySystem, `The potion tastes a bit stale — its effects are diminished.`)
		}
	}

	// Calculate final duration multiplier:
	// potencyMult (from aging phase) * craftSkill scaling
	durationMult := potencyMult
	if matchItem.CraftSkill > 0 {
		durationMult *= 1.0 + float64(matchItem.CraftSkill)/100.0
	}

	// Apply buffs with scaled duration. Scaled and unscaled both go through the
	// user, so Buff_ApplyBuffs runs and the drinker reads the buff's start line;
	// the multiplier rides on the event. Applying the scaled case through
	// Character.AddBuffScaled instead is what made Purging Weakness silent.
	for _, buffId := range itemSpec.BuffIds {
		user.AddBuffScaled(buffId, durationMult, `drink`)
		// Compute tick snapshot for config-driven buffs (no stat scaling for
		// potions). SetTickAmount below is live on a RE-drink, where the buff is
		// still held and its index hits; it is a no-op only on the first
		// application, because the apply above is queued and the buff is not in
		// the list yet. Either way the amount is the same: NewRound_UserRoundTick
		// recomputes a tick_pool buff whose TickAmount is still 0 with the same
		// scalingMult of 1.0, and of the three tick_pool buffs a drinkable can
		// apply (5, 7, 47) none declares tick_variance, so the recomputation is
		// deterministic, while no tick_pool buff carries a max-pool statmod, so
		// it reads the same pool.
		if buffSpec := buffs.GetBuffSpec(buffId); buffSpec != nil && buffSpec.TickPool != "" {
			var maxPool int
			switch buffSpec.TickPool {
			case "health":
				maxPool = user.Character.HealthMax.Value
			case "stamina":
				maxPool = user.Character.StaminaMax.Value
			case "conviction":
				maxPool = user.Character.ConvictionMax.Value
			}
			tickAmt := buffs.ComputeTickAmount(maxPool, buffSpec.TickPercent, buffSpec.TickVariance, buffSpec.TickMin, 1.0)
			user.Character.Buffs.SetTickAmount(buffId, tickAmt)
		}
	}

	// ── Ysolde's Purge special-case ──────────────────────────────────────────
	// Toxicity (26) and the detox debuff (buff 93) are applied by the normal
	// drink path above. Here we drive the addiction step-down -- the brutal-
	// fast path to clean.
	if itemSpec.ItemId == ysoldesPurgeItemId {
		user.Character.AddBloomAddiction(-5)
		user.SendText(messaging.CategoryWarning,
			`The purge takes hold -- your body convulses as it expels the `+
				`Bloom. It is violent, and it is fast.`)
	}

	// ── Purging Draught special-case ─────────────────────────────────────────
	// The draught declares only buff 70, which is a flavour line with no
	// statmods, so every effect it advertises has to be wired here -- exactly
	// as Ysolde's Purge and the Bloom Wafer are. Its own toxicity was applied
	// by the normal path above and is cleared again here, which is correct: you
	// cannot pay a toxicity price for the thing that removes toxicity.
	if itemSpec.ItemId == purgingDraughtItemId {
		applyPurgeEffects(user)
		user.SendText(messaging.CategoryWarning,
			`The draught tears through you. Every trace of potion work is `+
				`scoured out, and you are left shaking and hollow.`)
	}

	// ── Bloom Wafer special-case ──────────────────────────────────────────────
	// The wafer has no buffids in its YAML; all Bloom effects are wired here.
	// Toxicity (20) was already applied by the normal path above — don't
	// apply it again. The order relative to the buff loop above doesn't matter
	// since the loop is empty for this item.
	if itemSpec.ItemId == bloomWaferItemId {
		bal := configs.GetBalanceConfig()

		// Communion high. Buff 90's YAML baseline is 30 rounds; scale it by the
		// BloomCommunionRounds knob so config actually tunes the duration.
		communionMult := float64(bal.BloomCommunionRounds) / 30.0
		if communionMult <= 0 {
			communionMult = 1.0
		}
		user.AddBuffScaled(90, communionMult, `drink`)

		// Tick addiction counter.
		user.Character.AddBloomAddiction(int(bal.BloomAddictionPerDose))

		// Stamp the dose round for withdrawal / decay timing.
		user.Character.BloomLastDoseRound = util.GetRoundCount()

		// Mutation acceleration. First roll the (small) BloomNewMutationChance to
		// push a brand-new change even if the user already has mutations — Bloom's
		// "occasionally something wholly new" variety. Otherwise roll the (larger)
		// BloomMutationAdvanceChance to deepen the strongest existing mutation
		// (which falls through to seeding when the user has none / all are capped).
		var mutId string
		if rand.Float64() < float64(bal.BloomNewMutationChance) {
			mutId, _ = user.Character.BloomSeedNewMutation(nil)
		} else if rand.Float64() < float64(bal.BloomMutationAdvanceChance) {
			mutId, _ = user.Character.BloomAdvanceMutation(nil)
		}
		if mutId != "" {
			user.SendText(messaging.CategoryWarning,
				`Something under your skin shifts and settles differently.`)
		}

		// Euphoric onset message — replaces the generic "you drink" that was
		// already sent above. Sent last so it reads as the climax of the
		// consume sequence.
		user.SendText(messaging.CategoryWarning,
			`The wafer dissolves to nothing on your tongue and the world goes `+
				`warm and wide — communion.`)
	}

	// ── Catalyst of Unmaking special-case ─────────────────────────────────────
	// #22 crash-site "remort": scour every acquired mutation back to species
	// intrinsics and grant rare-biased reroll charges. The potion was already
	// consumed by the normal path above; here we only apply the effect and
	// send the onset message (mirrors the Bloom Wafer block's shape).
	if itemSpec.ItemId == catalystOfUnmakingItemId {
		user.Character.ScourMutations(scourRerollCharges)
		user.SendText(messaging.CategoryWarning,
			`<ansi fg="magenta">You drink the Catalyst. For one breath you are only what you `+
				`were born as — every woken thing in your blood goes still and gone. Then the `+
				`cold lets go, and the hunger comes back stronger than before.</ansi>`)
	}

	// ── Phial of Second Birth special-case ────────────────────────────────────
	// Pinnacle remort potion: scour every acquired mutation back to species
	// intrinsics (no reroll charges -- the grant below is immediate), then
	// grant exactly one mutation from the rarity-floored pool.
	if itemSpec.ItemId == phialOfSecondBirthItemId {
		user.Character.ScourMutations(0)
		granted := user.Character.GrantRandomMutationRare(phialRarityFloor)
		if granted != "" {
			if spec := mutations.GetMutation(granted); spec != nil {
				user.SendText(messaging.CategoryWarning, fmt.Sprintf(
					`<ansi fg="magenta">Your flesh unwrites itself — every change the Chrysalis ever made dissolves. Then, from the stillness, something singular takes root: <ansi fg="yellow">%s</ansi>.</ansi>`, spec.Name))
			}
		} else {
			user.SendText(messaging.CategoryWarning,
				`<ansi fg="magenta">Your flesh unwrites itself — every change dissolves. The stillness holds; nothing new takes root.</ansi>`)
		}
	}

	return true, nil
}
