package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// chrysifier_homunculus.go — the Chrysifier apex companion. A "crafted" copy of
// the player: a boss-tier body whose statpool scales off how much the owner has
// crafted, inheriting their non-crafting skills + a preset physical-mutation
// loadout, reserving heavy Conviction (usually the owner's only companion).

const homunculusMobId = 9612

// homunculusCraftSkills are the skills whose summed levels drive the homunculus
// statpool (and which are NOT inherited by it — it's the bruiser, not the maker).
var homunculusCraftSkills = []string{
	"blacksmithing", "alchemy", "tailoring", "cooking",
	"jewelcrafting", "enchanting", "salvage",
}

// homunculusStatPool = round(craftSum * scale), floored so a novice still gets a
// real (if modest) body.
func homunculusStatPool(craftSum int, scale float64) int {
	if craftSum < 1 {
		craftSum = 1
	}
	return int(float64(craftSum)*scale + 0.5)
}

// homunculusCraftSum sums the owner's crafting-skill levels. Uses the raw
// Skills map (not GetSkillLevel, which floors unset skills to 1) so only actual
// investment drives the homunculus's power.
func homunculusCraftSum(c *characters.Character) int {
	sum := 0
	for _, s := range homunculusCraftSkills {
		sum += c.Skills[s]
	}
	return sum
}

// inheritedHomunculusSkills returns the owner's NON-crafting skills, so the
// homunculus fights the way its maker does (crafting skills are excluded — the
// homunculus doesn't craft, it fights).
func inheritedHomunculusSkills(c *characters.Character) map[string]int {
	craftSet := make(map[string]bool, len(homunculusCraftSkills))
	for _, s := range homunculusCraftSkills {
		craftSet[s] = true
	}
	out := make(map[string]int)
	for sk, lvl := range c.Skills {
		if lvl > 0 && !craftSet[sk] {
			out[sk] = lvl
		}
	}
	return out
}

// hasLiveHomunculus reports whether the owner currently has a live homunculus
// companion (registered AND its mob instance still exists).
func hasLiveHomunculus(ch *characters.Character) bool {
	for i := range ch.Companions {
		if ch.Companions[i].MobId == homunculusMobId &&
			mobs.GetInstance(ch.Companions[i].InstanceId) != nil {
			return true
		}
	}
	return false
}

// tickHomunculus keeps a Homunculus-apex owner supplied with their crafted twin:
// if they hold the apex and have no live homunculus, it (re)forges one after a
// short respawn cooldown. Called per-round from UserRoundTick.
//
// U7b added the reservation gate this path never had. Its old docstring said
// "There is NO affordability gate -- the homunculus is the owner's apex identity
// and always manifests", which was true and is no longer: an ungated write into
// a capped pool is exactly what the ceiling exists to stop.
//
// The homunculus is a CRAFTING apex whose owner has no particular reason to
// have invested in manifestation. At the old base of 1000 the cap would have
// made it unfieldable by exactly the character it is built for, while leaving
// it fieldable by a summoner who does not need it. Owner decision 2026-08-15:
// the base drops to 300. Only one homunculus can exist at a time regardless,
// which hasLiveHomunculus already enforces.
//
// STILL WATCH THIS IN PLAYTEST. 300 fits a 66% cap only from roughly 455
// Conviction max upward, and nearer 500 once the rank-0 rider penalty applies,
// so a low-Conviction crafter can still be refused. The refusal is spoken
// rather than silent precisely so that shows up as a report instead of a
// mystery; the lever, if it bites, is HomunculusConvictionReserve.
func tickHomunculus(user *users.UserRecord, room *rooms.Room) {
	if room == nil {
		return
	}
	ch := user.Character
	if !mutations.HasHomunculus(ch.Mutations) {
		return
	}
	if hasLiveHomunculus(ch) {
		return
	}
	// Respawn delay: the "homunculus-respawn" cooldown is set when the twin
	// FALLS (see CompanionCleanup), so it reforges a while after death rather
	// than instantly. On first acquisition no cooldown exists → immediate forge.
	if ch.GetCooldown("homunculus-respawn") > 0 {
		return
	}
	if spawnHomunculus(user, room) == nil {
		// Couldn't manifest (e.g. the owner is at the companion cap) — back off
		// so we don't retry every single round.
		ch.TryCooldown("homunculus-respawn", "10 rounds")
	}
}

// spawnHomunculus forges the owner's homunculus companion into `room` and
// registers it (with its snapshotted Conviction reservation). Returns the mob,
// or nil on failure.
func spawnHomunculus(user *users.UserRecord, room *rooms.Room) *mobs.Mob {
	ch := user.Character
	cfg := configs.GetBalanceConfig()

	reserve := ch.CalcCompanionReserve(int(cfg.HomunculusConvictionReserve))
	if ch.WouldBreachReservationCap(characters.PoolConviction, reserve) {
		// Spoken, not silent. The caller backs off for ten rounds on nil, so a
		// silent refusal here would look to the player exactly like the apex
		// being broken.
		user.SendText(messaging.CategorySpellManifestation,
			`Your homunculus stirs and will not hold its shape. `+
				ch.ReservationRefusal(characters.PoolConviction, reserve))
		return nil
	}

	pool := homunculusStatPool(homunculusCraftSum(ch), float64(cfg.HomunculusCraftScale))
	mob := mobs.NewMobByIdFresh(mobs.MobId(homunculusMobId), room.RoomId, pool)
	if mob == nil {
		return nil
	}
	room.AddMob(mob.InstanceId)

	// A copy of its maker: same name-root, same non-crafting skills.
	mob.Character.Name = ch.Name + "'s Homunculus"
	if mob.Character.Skills == nil {
		mob.Character.Skills = make(map[string]int)
	}
	for sk, lvl := range inheritedHomunculusSkills(ch) {
		mob.Character.Skills[sk] = lvl
	}
	// A preset physical-cluster (Colossus) loadout — the bruiser the crafter
	// themselves is not. First-pass set; tune in playtest.
	mob.Character.Mutations = map[string]int{"titan-growth": 2}
	mob.Character.RecalculateStats()

	// Bind as a permanent companion of the owner.
	mob.Character.Charm(user.UserId, 99999, "")
	targeting.Release(&mob.Character, targeting.ReasonDisengage)
	ch.TrackCharmed(mob.InstanceId, true)

	info := characters.CompanionInfo{
		MobId:             homunculusMobId,
		InstanceId:        mob.InstanceId,
		SourceType:        characters.CompanionSummoned,
		Name:              mob.Character.Name,
		BaseName:          mob.Character.Name,
		AutoAssist:        true,
		ConvictionReserve: reserve,
	}
	if !ch.AddCompanion(info) {
		// At the soft companion cap — undo the spawn so we don't leak an
		// orphaned charmed mob that hasLiveHomunculus can never see.
		ch.TrackCharmed(mob.InstanceId, false)
		room.RemoveMob(mob.InstanceId)
		mobs.DestroyInstance(mob.InstanceId)
		return nil
	}
	ch.RecalculateStats() // apply the new reservation immediately
	return mob
}
