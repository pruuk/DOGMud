package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// companionBaseReserveFor resolves the base (pre-reduction) Conviction cost a
// companion of this mob id would be charged if created today: the reserve
// derived from the summon spell's pet multiplier when one targets the mob, the
// dedicated bases for the homunculus and brood-floor spawns, else
// CompanionReserveDefault.
func companionBaseReserveFor(mobId int) int {
	switch mobId {
	case homunculusMobId:
		return int(configs.GetBalanceConfig().HomunculusConvictionReserve)
	case broodSpawnMobId:
		return broodFloorReserve
	}
	for _, sp := range spells.GetAllSpells() {
		if sp.SummonMobId == mobId && sp.SummonPetMultiplier > 0 {
			return characters.CompanionReserveBase(sp.SummonPetMultiplier)
		}
	}
	return int(configs.GetBalanceConfig().CompanionReserveDefault)
}

// refreshCompanionReserves recomputes ConvictionReserve on EVERY companion from
// the reserve that companion's mob id would be charged today (D11).
//
// It replaced a backfill that only stamped records loading as 0. Widening it was
// the point: ConvictionReserve is frozen at summon time so it cannot drift while
// a companion is fielded, which is right within a session and wrong across the
// U7b rebase. A legacy zero and a stale pre-rebase figure are the same problem,
// and recomputing at login fixes both. Meirok's two golems rebase from 158 each
// to 126 each on his next login with no action from him.
//
// It NEVER dismisses a companion, whatever the recomputed total comes to
// (D4 grandfathering). Reservation is refused on ADDITION only.
//
// Returns true if any reserve moved, so the caller knows whether to recalculate.
func refreshCompanionReserves(ch *characters.Character) bool {
	changed := false
	for i := range ch.Companions {
		want := ch.CalcCompanionReserve(companionBaseReserveFor(ch.Companions[i].MobId))
		if ch.Companions[i].ConvictionReserve != want {
			ch.Companions[i].ConvictionReserve = want
			changed = true
		}
	}
	return changed
}

// refreshCompanionReservesOnLogin runs the D11 recompute and TELLS the player
// when it left them further past the ceiling than they were.
//
// The disclosure is not optional politeness. Companion reserve is priced partly
// off manifestation, GetSkillLevel counts equipment stat mods, and
// skill_manifestation is in the gold-scaled loot affix pool, so a player who
// sold or lost a +manifestation item sees every companion get dearer at their
// NEXT login, with no action of theirs in between. Silent, that is
// indistinguishable from a bug.
//
// Nothing is dismissed either way (D4). The refusal is on addition only.
func refreshCompanionReservesOnLogin(user *users.UserRecord) {
	ch := user.Character
	before := ch.ReservationOverages()

	if !refreshCompanionReserves(ch) {
		return
	}
	ch.RecalculateStats()

	if _, worse := before.Worsened(ch.ReservationOverages()); !worse {
		return
	}
	user.SendText(messaging.CategorySystem, companionRebaseNotice(ch))
}

// companionRebaseNotice is the login disclosure. Bands only, never a number,
// and it names RESERVATION as the cause: a player has no other way to learn
// that their own bonds are the obstacle and that resting will never clear it.
// It names gear too, because gear is the half of the price that can move
// without them noticing.
func companionRebaseNotice(ch *characters.Character) string {
	max := ch.ConvictionMax.Value
	share := characters.ReserveShareBand(ch.GetPoolReservation("conviction", max), max)
	// The subject names what is REALLY holding the pool. It used to credit the
	// whole figure to companions, which overstated them for anyone also wearing
	// reserving gear -- the mirror of the fixed "Your gear and bonds" phrase the
	// refusal shed on 2026-08-16.
	subject, verb := ch.ReservationHolders(characters.PoolConviction)
	return `Your bonds were re-priced while you were away. What they cost you ` +
		`follows your skill at manifestation and the gear you carry, so it ` +
		`moves when those move. ` + subject + ` now ` + verb + ` ` + share +
		` of your conviction in reserve. Nothing has been taken from you, but ` +
		`you cannot take on more until you make room.`
}
