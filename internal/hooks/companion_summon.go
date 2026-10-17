package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// resolveCompanionSummon handles YAML-driven companion summoning spells.
// It replaces 13 nearly-identical JS onMagic scripts with one parameterized
// Go function. Called when spellData.SummonMobId > 0.
//
// Steps: corpse selection, companion cap check, optional component
// consumption, corpse consumption, stat scaling, mob spawn, charm, companion
// registration.
//
// Returns true on success, false on failure (error already sent to user).
func resolveCompanionSummon(user *users.UserRecord, spellData *spells.SpellData, spellRest string, room *rooms.Room) bool {

	ch := user.Character

	// ── 0. Corpse selection (if required) ───────────────────────────────
	// Selection spends nothing, so it runs before the reservation gate and
	// before the component is consumed (#282): a raise with no usable corpse
	// names the corpse, not a reservation the player would free up only to
	// fail again, and it keeps the component. The corpse is removed in step 3,
	// once every other gate has passed. The same gate runs at cast start
	// (usercommands.Cast); this one is the backstop for a corpse that went
	// away while the cast channelled.
	corpseIdx := -1
	var corpsePool int
	if spellData.SummonRequiresCorpse {
		idx, pool, refusalLine := actions.FindRaiseCorpse(room, spellData, spellRest)
		if idx < 0 {
			user.SendText(messaging.CategoryRefusal, refusalLine)
			return false
		}
		corpseIdx, corpsePool = idx, pool
	}

	// ── 1. Reservation + budget gate ────────────────────────────────────
	// Reservation is DERIVED from the pet multiplier (D9), never authored.
	// Fail early, before consuming any component or corpse.
	baseReserve := characters.CompanionReserveBase(spellData.SummonPetMultiplier)
	// Count cap and conviction budget are separate refusals, reported
	// separately so a player at their companion limit isn't wrongly told they
	// lack conviction.
	if len(ch.Companions) >= ch.GetMaxCompanions() {
		refusal.CompanionsFull(user)
		return false
	}
	reserve := ch.CalcCompanionReserve(baseReserve)
	if ch.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		user.SendText(messaging.CategorySpellManifestation,
			ch.ReservationRefusal(characters.PoolConviction, reserve))
		return false
	}

	// ── 2. Component consumption (if required) ──────────────────────────
	if spellData.SummonComponentId > 0 {
		found := false
		for i, itm := range ch.Items {
			if itm.ItemId == spellData.SummonComponentId {
				ch.Items = append(ch.Items[:i], ch.Items[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			user.SendText(messaging.CategorySpellManifestation, "You lack the required component for this summoning.")
			return false
		}
	}

	// ── 3. Corpse consumption (if required) ─────────────────────────────
	// Selected in step 0; nothing since then touches room.Corpses.
	if corpseIdx >= 0 {
		room.Corpses = append(room.Corpses[:corpseIdx], room.Corpses[corpseIdx+1:]...)
	}

	// ── 4. Stat scaling ─────────────────────────────────────────────────
	charisma := ch.Stats.Charisma.ValueAdj
	manifestSkill := ch.GetSkillLevel(skills.Manifestation)

	// corpsePool is 0 unless a corpse was consumed, and CalcCompanionPool reads
	// a non-positive corpse pool as "conjured". The corpse average now lives
	// INSIDE that function -- averaging again here would average twice.
	pool := characters.CalcCompanionPool(charisma, manifestSkill, spellData.SummonPetMultiplier, corpsePool)

	// ── 5. Spawn and register ───────────────────────────────────────────
	mob := mobs.NewMobByIdFresh(mobs.MobId(spellData.SummonMobId), room.RoomId, pool)
	if mob == nil {
		user.SendText(messaging.CategorySpellDisruption, "The summoning fails. Something is wrong.")
		return false
	}
	room.AddMob(mob.InstanceId)

	// Charm permanently (effectively infinite duration)
	mob.Character.Charm(user.UserId, 99999, "")
	targeting.Release(&mob.Character, targeting.ReasonDisengage)
	user.Character.TrackCharmed(mob.InstanceId, true)

	// Determine source type based on whether a corpse was consumed
	sourceType := characters.CompanionSummoned
	if spellData.SummonRequiresCorpse {
		sourceType = characters.CompanionRaised
	}

	// Register as companion
	info := characters.CompanionInfo{
		MobId:             int(mob.MobId),
		InstanceId:        mob.InstanceId,
		SourceType:        sourceType,
		Name:              mob.Character.Name,
		BaseName:          mob.Character.Name,
		AutoAssist:        true,
		ConvictionReserve: reserve,
	}
	if !ch.AddCompanion(info) {
		// Should not happen since we checked the budget above, but be safe
		refusal.CompanionsFull(user)
		return false
	}
	// Apply the new reservation immediately so usable Conviction drops now.
	ch.RecalculateStats()

	// Clear aggro from existing companions toward the new mob
	for _, charmId := range ch.GetCharmIds() {
		if charmId == mob.InstanceId {
			continue
		}
		if companion := mobs.GetInstance(charmId); companion != nil {
			if companion.Character.IsInCombat() &&
				companion.Character.CurrentCombatTarget().MobInstanceId == mob.InstanceId {
				targeting.Release(&companion.Character, targeting.ReasonDisengage)
			}
		}
	}

	// Clear the owner's own aggro if targeting the new mob
	if ch.IsInCombat() && ch.CurrentCombatTarget().MobInstanceId == mob.InstanceId {
		targeting.Release(ch, targeting.ReasonDisengage)
	}

	return true
}
