package actions

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crimes"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/justice"
	"github.com/GoMudEngine/GoMud/internal/knowledge"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/opinions"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// RecordAssaultCrime records an assault crime against each defined faction the
// mob belongs to, and bumps player rep with each (only when the perpetrator is
// identified).
//
// Lives here rather than in internal/usercommands because the melee special
// moves engage through AcquireMeleeTarget in this package, and usercommands
// already depends on actions. Callers: attack, target, shoot (all in
// usercommands) and SeedAggression below.
func RecordAssaultCrime(user *users.UserRecord, mob *mobs.Mob, room *rooms.Room) {
	factionIds := factions.FactionsForMob(mob)
	if len(factionIds) == 0 {
		return
	}
	// All witnesses including the victim (excludeInstanceId=0) drive
	// perp/rep determination (victim is alive and a self-witness).
	witnesses := crimes.WitnessesInRoom(factionIds, room, 0)
	perp := crimes.IdentifiedPerp(user.UserId, witnesses)
	// External witnesses: same call but exclude the victim — used to
	// set HadExternalWitness so the murder-upgrade path knows whether
	// the assault was seen by someone other than the victim.
	externalWitnesses := crimes.WitnessesInRoom(factionIds, room, mob.InstanceId)
	hadExternal := len(externalWitnesses) > 0
	delta := int(configs.GetBalanceConfig().CrimeRepDeltaAssault)
	for _, fid := range factionIds {
		crimeIds := crimes.Record([]string{fid}, crimes.KindAssault, perp,
			mob, mob.InstanceId, room.RoomId, mob.Character.Zone, hadExternal)
		if perp.Type == crimes.PerpPlayer {
			factions.BumpRep(fid, user.UserId, delta)
			justice.MaybeDeclareBounty(fid, user.UserId, crimes.KindAssault)
			// Knowledge: each witness records the player as the perp of
			// these crimes.
			subject := knowledge.PlayerSubject(user.UserId)
			for _, witnessInstId := range witnesses {
				w := mobs.GetInstance(witnessInstId)
				if w == nil {
					continue
				}
				for _, crimeId := range crimeIds {
					knowledge.RecordCrimeWitnessed(int(w.MobId), subject, crimeId)
				}
				knowledge.RecordMet(int(w.MobId), subject, room.RoomId,
					knowledge.SourceWitnessed)
			}
		}
	}
}

// SeedAggression records that a player just committed an aggressive act
// against a mob. Every player-initiated attack path must call it, or that
// attack is invisible to the revenge, opinion and justice systems.
//
// This gap was real and wide: before 2026-08-14 only `attack`, `shoot` and
// (partially) `taunt` seeded anything, so the ten melee special moves and
// `throw` could open on a faction NPC with no assault recorded, no rep hit, no
// bounty check and no witness knowledge. Killing the mob was still caught by
// the death hook's murder record, but assaulting and walking away was free.
//
// The two halves fire on deliberately different conditions, copied from
// attack.go rather than reinvented:
//
//   - PlayerAttackedMob fires on EVERY commitment, fresh or not, because
//     seeder rules 6 and 9 (revenge, combat-assist opinion) want to see
//     repeated aggression, not just the opening blow.
//   - The opinion bump and the assault crime fire ONLY on fresh aggression.
//     Recording them every round would re-log a crime and re-bump rep on every
//     kick of a long fight, spamming bounties off a single engagement.
//
// freshAggro is the caller's judgement because "fresh" differs by shape: a
// single-target move compares the attacker's prior aggro against this mob,
// while an AoE like `throw` has to decide per mob it hits. See engageAfterThrow.
func SeedAggression(user *users.UserRecord, mob *mobs.Mob, room *rooms.Room, freshAggro bool) {
	if user == nil || mob == nil || room == nil {
		return
	}

	events.AddToQueue(events.PlayerAttackedMob{
		UserId:        user.UserId,
		MobInstanceId: mob.InstanceId,
	})

	if !freshAggro {
		return
	}

	opinions.Bump(int(mob.MobId), user.UserId,
		int(configs.GetBalanceConfig().OpinionAttackBump))
	RecordAssaultCrime(user, mob, room)
}
