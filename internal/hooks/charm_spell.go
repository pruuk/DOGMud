package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// applyMobEffect_charm binds a creature whose contest has ALREADY been decided.
//
// Charm used to run its own RunContest here, on hand-built scores, on top of
// the channel contest the cast had already run and then discarded in
// applyMobEffect_default. One cast therefore resolved twice, and the player
// saw both narrations -- a resist line and a success line for the same spell.
// The contest now happens once, in the seam, on ChannelSocial, and this reads
// its result.
//
// Returns 0: charm deals no damage.
func applyMobEffect_charm(
	user *users.UserRecord,
	targetMob *mobs.Mob,
	room *rooms.Room,
	spellData *spells.SpellData,
	out combat.ChannelDefenceResult,
	mName string,
) int {
	// applyMobEffect is reached with a nil user when a MOB casts (see its
	// docstring and resolveMobSpellAgainstMob). No mob carries charm today --
	// the behaviour tree skips it -- but every sibling arm guards, and a switch
	// arm that depends on an exclusion elsewhere is a landmine.
	if user == nil || targetMob == nil {
		return 0
	}
	ch := user.Character

	if targetMob.CharmImmune {
		user.SendText(messaging.CategorySpellMental, `That creature's mind is impervious to charm.`)
		return 0
	}

	// The defence won. Narrate through the SHARED path every other effect arm
	// uses, so defy renders its own triad to the caster, the target and the
	// room. The old code sent a bespoke pair of lines here and the seam sent
	// another, which is where the double narration came from.
	if out.Defended {
		sendSpellChannelDefenceMessages(room, spellSchoolCategory(spellData), out,
			spellDefenceIdentity(ch, user, room), mName, spellData.Name, user, nil)
		return 0
	}

	// ── Reservation + budget gate ──────────────────────────────────────
	// The reserve is FLAT and does not scale with what you charmed: a sewer rat
	// and an Elemental King tie up the same conviction. That is a deliberate
	// decision (spec 3.8), not an oversight and not a leftover from the
	// pet-multiplier path. Charm is already a risky game -- the creature trains
	// while it serves you and keeps the gear you hand it. A power-scaled price
	// would add bookkeeping without adding tension, so the cost of charming
	// something enormous is the DANGER, not the invoice.
	reserve := ch.CalcCompanionReserve(characters.CompanionReserveBase(0))
	if len(ch.Companions) >= ch.GetMaxCompanions() {
		user.SendText(messaging.CategorySystem,
			`You are already sustaining as many companions as your will can hold.`)
		return 0
	}
	if ch.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		user.SendText(messaging.CategorySystem, ch.ReservationRefusal(characters.PoolConviction, reserve))
		return 0
	}

	targetName := targetMob.Character.Name

	// The bond has a clock, and the player is never told how long it is. That
	// uncertainty is the mechanic: a bond you cannot plan around is the whole
	// risk of charming something dangerous.
	//
	// Do NOT substitute characters.CharmPermanent here. That sentinel means
	// "never expires" and would restore exactly the permanence this replaces.
	margin := out.AttackerNormalizedMargin
	if out.AttackerCrit && margin == 0 {
		// A FORCED crit -- a sleeping victim -- returns from the seam above its
		// margin assignment, so it reads zero, which maps to the MINIMUM
		// duration. That would make the most decisive charm in the game buy the
		// shortest bond, while a scrappy win against the same creature awake
		// bought the longest. Read it as the ceiling instead (spec 15).
		//
		// Corrected here rather than in the seam on purpose: a forced crit
		// bypasses the contest, so there genuinely is no opposed margin to
		// report, and inventing one for every channel to satisfy charm would be
		// the tail wagging the dog. combat's
		// TestAttackerNormalizedMargin_ZeroOnForcedCritWin_KNOWN pins the
		// seam's side of that bargain.
		margin = charmMarginCeiling
	}
	targetMob.Character.Charm(user.UserId, charmDurationFor(margin), "")
	targetMob.Character.EndAggro()
	ch.TrackCharmed(targetMob.InstanceId, true)

	info := characters.CompanionInfo{
		MobId:             int(targetMob.MobId),
		InstanceId:        targetMob.InstanceId,
		SourceType:        characters.CompanionCharmed,
		Name:              targetName,
		BaseName:          targetName,
		AutoAssist:        true,
		ConvictionReserve: reserve,
	}
	if !ch.AddCompanion(info) {
		user.SendText(messaging.CategorySystem, `You cannot maintain any more companions.`)
		return 0
	}
	// Apply the new reservation immediately so usable Conviction drops now.
	ch.RecalculateStats()

	// Clear aggro from existing companions toward the new mob
	for _, charmId := range ch.GetCharmIds() {
		if charmId == targetMob.InstanceId {
			continue
		}
		if companion := mobs.GetInstance(charmId); companion != nil {
			if companion.Character.IsInCombat() &&
				companion.Character.CurrentCombatTarget().MobInstanceId == targetMob.InstanceId {
				companion.Character.EndAggro()
			}
		}
	}

	// Clear the owner's own aggro if targeting the new mob
	if ch.IsInCombat() && ch.CurrentCombatTarget().MobInstanceId == targetMob.InstanceId {
		ch.EndAggro()
	}

	user.SendText(messaging.CategorySpellMental, fmt.Sprintf(
		`<ansi fg="cyan">%s's eyes glaze as your will takes hold. It is yours.</ansi>`,
		mName))
	sendVisualRoomText(room, messaging.CategorySpellMental, fmt.Sprintf(
		`<ansi fg="cyan"><ansi fg="username">%s</ansi> bends %s to their will!</ansi>`,
		user.Character.Name, mName), user.UserId)

	return 0
}
