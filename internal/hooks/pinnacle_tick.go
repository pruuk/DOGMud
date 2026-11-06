package hooks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// pinnacle_tick.go — the always-on per-round layer for pinnacle items
// (Stage 1, Task 11). Procs (item trees, item_proc_dispatch.go) fire off combat
// chokepoints; THIS file is the passive upkeep that runs once per player per
// round from UserRoundTick: hunger drain, ambient-potion conditions, aging freeze
// and mutation drip. Sentient voices are item trees (the item tick) since slice 2.
//
// Per-round cost discipline: the whole thing is gated by PinnacleItemsEnabled,
// and each sub-tick reads only the belt/weapon/worn slots it needs (GetSpec on
// an empty slot returns a zero ItemSpec, so the flag checks short-circuit for
// players wearing nothing pinnacle-flagged). Real cost on the common path: one
// GetAllWornItems() slice allocation (built once here, read by the mutation
// sub-tick) plus a few MiscData map lookups. The bandolier
// fingerprint build only runs for players wearing an ambient_potions belt.
//
// NOTE (GetSpec semantics): items.Item.GetSpec() returns an ItemSpec *value*,
// never nil (an unknown/empty item yields a zero ItemSpec). So every guard here
// is a field check (`!spec.PreservesContents`, `spec.HungerRounds <= 0`), not a nil
// check — the plan's `spec == nil` sketch would not compile against the real API.

// pinnacleUserTick runs once per player per round from UserRoundTick.
// It early-outs entirely unless the pinnacle-items feature is enabled.
func pinnacleUserTick(user *users.UserRecord, room *rooms.Room) {
	if !bool(configs.GetConfig().GamePlay.PinnacleItemsEnabled) {
		return
	}
	c := user.Character
	now := util.GetRoundCount()
	worn := c.GetAllWornItems() // one pass, for the mutation sub-tick

	tickPreserveContents(user, now)
	tickAmbientPotions(user, now)
	tickHunger(c, user, now)
	tickMutationItems(user, worn, now)
	// Sentient voices are item trees since item behaviour slice 2: the item
	// tick (ItemRoundTick) speaks their ambient lines.
}

// readMiscIntSlice tolerantly reads a []int from MiscData. Like characters.MiscRound,
// it copes with yaml round-tripping: a persisted []int comes back as []any of
// int/int64/float64. Returns nil for absent/other values.
func readMiscIntSlice(v any) []int {
	switch s := v.(type) {
	case []int:
		return s
	case []any:
		out := make([]int, 0, len(s))
		for _, e := range s {
			if n, ok := characters.MiscRound(e); ok {
				out = append(out, int(n))
			}
		}
		return out
	}
	return nil
}

// tickPreserveContents keeps the worn bandolier's rules on its contents
// (Character.PreserveBandolier, #794): a preserves_contents belt holds every
// potion at peak strength by marking it Preserved, so it reads at peak while
// slotted (Character.PotionAging) and rots the moment it leaves. Nothing here
// touches CraftedRound: the hold is a reading, not a clock, so it needs no
// catching up after a logout. An ambient belt lets go of any potion it would
// not take, and the wearer is told. Runs every round and once at login
// (HandleJoin).
func tickPreserveContents(user *users.UserRecord, now uint64) {
	released := user.Character.PreserveBandolier(now)
	if len(released) == 0 {
		return
	}
	beltName := user.Character.Equipment.Belt.DisplayName()
	for _, rel := range released {
		user.SendText(messaging.CategoryWarning, characters.BandolierReleaseLine(beltName, rel.Item.DisplayName(), rel.Why))
	}
}

// tickHunger drains the wielder when a hunger-flagged weapon (Blackrazor) has
// gone too long without a kill. Escalation grows with overdue time, capped at
// 3x, and NEVER kills outright — it clamps at 1 HP (the sword wants a living
// larder; item-suicide would be bad UX).
//
// Stale-anchor note: when the equipped weapon is NOT a hunger item this returns
// immediately, leaving any old pinnacle_hunger_anchor untouched. That is
// harmless — the gate is the *currently equipped* weapon's spec, so a leftover
// anchor is inert until a hunger weapon is wielded again (at which point the
// kill-round advance below re-bases it). No cleanup machinery, consistent with
// the Task 4 decision not to prune stale cooldown keys.
func tickHunger(c *characters.Character, user *users.UserRecord, now uint64) {
	spec := c.Equipment.Weapon.GetSpec()
	if spec.HungerRounds <= 0 || spec.HungerDrainPct <= 0 {
		return
	}
	anchor, ok := characters.MiscRound(c.GetMiscData("pinnacle_hunger_anchor"))
	if !ok {
		// First tick wielding it — the hunger clock starts now.
		c.SetMiscData("pinnacle_hunger_anchor", now)
		return
	}
	// A kill since the anchor resets the clock (the blade was recently fed).
	if kill, ok := characters.MiscRound(c.GetMiscData("pinnacle_last_kill_round")); ok && kill > anchor {
		anchor = kill
		c.SetMiscData("pinnacle_hunger_anchor", kill)
	}
	if now <= anchor {
		return
	}
	elapsed := now - anchor
	if elapsed <= uint64(spec.HungerRounds) {
		return
	}
	overdue := elapsed - uint64(spec.HungerRounds)
	escalation := 1.0 + float64(overdue)/float64(spec.HungerRounds)
	if escalation > 3.0 {
		escalation = 3.0
	}
	drain := int(float64(c.HealthMax.Value) * spec.HungerDrainPct * escalation)
	if drain < 1 {
		drain = 1
	}
	if c.Health-drain < 1 {
		drain = c.Health - 1
	}
	if drain <= 0 {
		return
	}
	// Deliberate design decision: the drain is non-combat attrition through
	// ApplyAttrition. It still trains on damage taken, but it does not wake,
	// aggro, mitigate or interrupt work (#514 R2): the blade's toll, not an attack.
	// Anonymous source: the toll comes from the item, and conditions and items carry no
	// applier the pool primitives can attribute to.
	c.ApplyAttrition(characters.PoolHealth, drain, state.ActorRef{})
	// The drain repeats every overdue round, but the feeding LINE is paced by
	// its own cooldown (HungerFeedingLineCooldownRounds) so an ignored hunger
	// debt doesn't spam the player every round.
	if user != nil {
		if next, ok := characters.MiscRound(c.GetMiscData("pinnacle_hunger_msg_next_round")); !ok || now >= next {
			// The weapon's tree speaks the line when it has one (item
			// behaviour slice 2); otherwise the plain fallback goes out.
			if !fireItemEvent(behaviortree.EventContext{EventType: "on_hunger_feeding"},
				c.Equipment.Weapon, "weapon", c, user.UserId, 0) {
				user.SendText(messaging.CategorySystem,
					`<ansi fg="red">The blade feeds on you, a cold pull beneath your grip.</ansi>`)
			}
			c.SetMiscData("pinnacle_hunger_msg_next_round",
				now+uint64(configs.GetBalanceConfig().HungerFeedingLineCooldownRounds))
		}
	}
}

// tickMutationItems rolls each worn mutation-tick item (the Seething Prism).
// The roll only happens on interval-aligned rounds, then a percent gate, then
// a rarity-floored grant. worn is the caller's single GetAllWornItems() pass.
func tickMutationItems(user *users.UserRecord, worn []items.Item, now uint64) {
	c := user.Character
	for _, itm := range worn {
		spec := itm.GetSpec()
		if spec.MutationTickInterval <= 0 {
			continue
		}
		if now%uint64(spec.MutationTickInterval) != 0 {
			continue
		}
		if util.Rand(100) >= spec.MutationTickChance {
			continue
		}
		granted := c.GrantRandomMutationRare(spec.MutationRarityFloor)
		if granted == "" {
			continue
		}
		events.AddToQueue(mutations.Gained{
			UserId:     user.UserId,
			MutationId: granted,
			Rank:       1,
			IsNew:      true,
		})
	}
}

// ── Ambient potions (Vitalis Bandolier) ─────────────────────────────────────
//
// Attunement mechanism: CONTENT FINGERPRINTING (chosen over per-call-site
// stamping). The tick remembers a fingerprint (belt itemId + sorted potion
// itemIds) in MiscData; whenever it changes — the player drank a slotted
// potion, added/removed one, or swapped the belt — we react. This catches ALL
// mutation paths with zero edits to drink/get/remove/equip commands, paid ONLY
// while an ambient_potions belt is worn (the flag-off path never builds it).
//
// SMARTER RESET (2026-07-14): a contents change no longer revokes everything and
// re-attunes the whole bandolier. Instead we KEEP every already-attuned condition
// active, revoke only the condition of a potion that actually left, and start the
// attunement window ONLY for a genuinely new potion effect. So crafting a second
// potion never drops the regen you already had, and rapid crafting doesn't
// perpetually reset the clock for potions that are already attuned. The player
// is told once when a new potion begins attuning and once when it settles in,
// so the ~40s window reads as intentional rather than broken.
//
// Deferred (Stage 2): the item card's "slotted potions can't be drunk" rule is
// item-level behavior for a later stage; the attunement cooldown is the Stage-1
// mechanical cost. We do NOT touch the toxicity path — ambient conditions never
// apply toxicity by construction.
//
// "Always-on" semantics: an attuned condition is held as a PERMANENT record, the
// way WornConditionIds are (holdAmbient), so it never counts down and never ends
// while its potion stays slotted. Before 2026-10-19 it was added with the potion's
// own duration and re-added after each expiry, so the web client's timer cycled
// and the end line printed while the effect never really fell off. The
// permanent-condition rebuild (Character.reapplyPermanentConditions) sources
// these from the attuned set through Character.AmbientBandolierConditions,
// so an equip or login Validate keeps them, and taking the belt off drops them.

// bandolierFingerprint is a stable string of belt itemId + sorted potion
// itemIds. Changes iff the player-visible bandolier contents change.
func bandolierFingerprint(belt items.Item, potions []items.Item) string {
	ids := make([]int, 0, len(potions))
	for _, p := range potions {
		ids = append(ids, p.ItemId)
	}
	sort.Ints(ids)
	var b strings.Builder
	fmt.Fprintf(&b, "%d:", belt.ItemId)
	for _, id := range ids {
		fmt.Fprintf(&b, "%d,", id)
	}
	return b.String()
}

// tickAmbientPotions keeps slotted potion conditions active at Peak potency while an
// ambient_potions bandolier is worn and attuned. Conditions applied this way are
// recorded (pinnacle_bandolier_conditions) so removal can revoke them.
func tickAmbientPotions(user *users.UserRecord, now uint64) {
	c := user.Character
	belt := c.Equipment.Belt
	spec := belt.GetSpec()

	if !spec.AmbientPotions {
		// Flag-off common path: no fingerprint work at all. Revoke any
		// lingering ambience from a previously-worn ambient bandolier and
		// clear the stored fingerprint so re-equipping one re-attunes.
		if applied := readMiscIntSlice(c.GetMiscData(characters.AmbientBandolierMiscKey)); len(applied) > 0 {
			revokeAmbient(c, applied)
			c.SetMiscData("pinnacle_bandolier_fingerprint", nil)
		}
		return
	}

	applied := readMiscIntSlice(c.GetMiscData(characters.AmbientBandolierMiscKey))
	appliedSet := map[int]bool{}
	for _, id := range applied {
		appliedSet[id] = true
	}
	desired := desiredAmbientConditions(c, now)

	// Content-change detection (see mechanism note above).
	fp := bandolierFingerprint(belt, c.PotionItems)
	prevFp, _ := c.GetMiscData("pinnacle_bandolier_fingerprint").(string)
	if fp != prevFp {
		c.SetMiscData("pinnacle_bandolier_fingerprint", fp)

		// Revoke ONLY the conditions whose potion just left; keep already-attuned
		// conditions active (and held, so none shows a countdown for a round)
		// so adding/removing one potion never drops the others.
		kept := make([]int, 0, len(applied))
		for _, id := range applied {
			if mag, ok := desired[id]; ok {
				kept = append(kept, id)
				holdAmbient(c, id, mag)
			} else {
				releaseAmbient(c, id)
			}
		}

		// Start the attunement window (and announce it once) only when a
		// genuinely new potion effect has appeared — not on a pure removal.
		hasNew := false
		for id := range desired {
			if !appliedSet[id] {
				hasNew = true
				break
			}
		}
		if hasNew {
			c.SetMiscData("pinnacle_bandolier_attune_round",
				now+uint64(configs.GetBalanceConfig().BandolierAttuneRounds))
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="magenta">The %s stirs, drawing in the essence of what you have slotted; it will take a moment to attune.</ansi>`,
				spec.Name))
		}
		c.SetMiscData(characters.AmbientBandolierMiscKey, kept)
		return
	}

	// Contents unchanged. While a new potion is still attuning, keep the
	// already-active conditions refreshed but hold the pending one back.
	if attune, ok := characters.MiscRound(c.GetMiscData("pinnacle_bandolier_attune_round")); ok && now < attune {
		for _, id := range applied {
			if mag, ok := desired[id]; ok {
				holdAmbient(c, id, mag)
			}
		}
		return
	}

	// Attuned (or nothing pending): hold every slotted potion's condition.
	// Announce completion once when a new effect just landed.
	newlyApplied := false
	for id, mag := range desired {
		if !appliedSet[id] {
			newlyApplied = true
		}
		holdAmbient(c, id, mag)
	}
	if _, ok := characters.MiscRound(c.GetMiscData("pinnacle_bandolier_attune_round")); ok {
		if newlyApplied {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(
				`<ansi fg="magenta">The %s settles into resonance. Its stored virtues suffuse you.</ansi>`,
				spec.Name))
		}
		c.SetMiscData("pinnacle_bandolier_attune_round", nil)
	}
	// Persist the active set (= desired; every slotted effect is now applied).
	ids := make([]int, 0, len(desired))
	for id := range desired {
		ids = append(ids, id)
	}
	c.SetMiscData(characters.AmbientBandolierMiscKey, ids)
}

// holdAmbient makes sure the character holds conditionId as a permanent record,
// at magnitude when the potion's condition reads one (a Pitsense Tincture's
// infravision reach). A record already permanent at that magnitude is left
// alone, so the per-round cost is a lookup; a timed record (one applied
// before 2026-10-19, or the same condition drunk by hand) is turned permanent
// in place, and the bandolier's display reads "sustained" again.
func holdAmbient(c *characters.Character, conditionId int, magnitude float64) {
	for _, held := range c.Conditions.GetConditions(conditionId) {
		if held.Permanent && held.Magnitude == magnitude {
			return
		}
	}
	_ = c.AddCondition(conditionId, true)
	if magnitude > 0 {
		for _, held := range c.Conditions.GetConditions(conditionId) {
			held.Magnitude = magnitude
		}
	}
}

// desiredAmbientConditions maps each condition id the bandolier holds on its
// wearer to the magnitude it holds it at (0 for a condition that reads none).
// Only a potion of a helpful kind counts (items.ItemSpec.AmbientBeneficial,
// #794): the bandolier refuses the rest, and this is the backstop for one a
// save carried in. Each potion counts at peak strength (owner ruling
// 2026-11-06), as a drink of it at peak would apply it: the potency of
// Character.PotionAging, raised by its maker's skill. Two potions sharing a
// condition hold it at the stronger magnitude.
func desiredAmbientConditions(c *characters.Character, now uint64) map[int]float64 {
	out := map[int]float64{}
	for _, p := range c.PotionItems {
		spec := p.GetSpec()
		if !spec.AmbientBeneficial() {
			continue
		}
		mult := items.PotionDurationMult(c.PotionAging(p, now).Potency, p.CraftSkill)
		for _, conditionId := range spec.ConditionIds {
			mag := 0.0
			if m, _, ok := items.PotionMagnitudeApplication(&spec, conditions.GetConditionSpec(conditionId), mult); ok {
				mag = m
			}
			if prev, held := out[conditionId]; !held || mag > prev {
				out[conditionId] = mag
			}
		}
	}
	return out
}

// releaseAmbient ends the bandolier's hold on conditionId: its permanent
// record goes. A timed record of the same condition is left to run, since it
// is not the bandolier's: the potion was drunk, and the drink's own effect
// must outlast the potion leaving the bandolier.
func releaseAmbient(c *characters.Character, conditionId int) {
	for _, held := range c.Conditions.GetConditions(conditionId) {
		if held.Permanent {
			c.RemoveCondition(conditionId)
			return
		}
	}
}

// revokeAmbient releases every previously-applied ambient condition and clears
// the tracking key.
func revokeAmbient(c *characters.Character, applied []int) {
	for _, id := range applied {
		releaseAmbient(c, id)
	}
	if len(applied) > 0 {
		c.SetMiscData(characters.AmbientBandolierMiscKey, []int{})
	}
}
