package hooks

import (
	"sort"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// creditMobKill awards kill credit for a mob that has just died to:
//   - Each player whose UserId appears in DeadData.DamageMap
//   - Each party member of those players who is in the mob's room
//     (killCreditUserIds; a member elsewhere used to be credited too)
//
// Also ends combat aggro for any killer who is currently targeting
// this mob.
//
// It is called by scheduleMobDespawnFromLife while the instance is still
// alive, not wired as its own Life observer. It used to be one
// (wireMobKillCredit), but AfterTransition cascades run in init() order,
// which is alphabetical by filename, so Death_MobInstanceCleanup.go ran
// first, destroyed the instance, and this credit found nothing and returned.
// No player's kill count ever moved (#468). It is the same ordering trap the
// loot drop hit, solved the same way.
//
// Migrated from internal/mobcommands/suicide.go lines 133-178 as
// part of chunk-2 Task 10.
func creditMobKill(m *mobs.Mob) {
	d, _ := m.Character.Life.DeadData()
	if len(d.DamageMap) == 0 {
		return
	}

	inTraining := m.Character.Zone == `Training`

	// End combat aggro for any direct attacker still targeting this mob.
	for uid := range d.DamageMap {
		user := users.GetByUserId(uid)
		if user == nil {
			continue
		}
		if user.Character.IsInCombat() {
			if user.Character.CurrentCombatTarget().MobInstanceId == m.InstanceId {
				targeting.Release(user.Character, targeting.ReasonDisengage)
			}
		}
	}

	if inTraining {
		return
	}
	// U10b-1 Task 20 deleted the first-kill progression call that stood
	// here: it was not a resolved action, and it progressed a skill named
	// "combat" that does not exist. KD.AddMobKill stays: it is the
	// kill-count stat (it feeds powerscore), not progression.
	for _, uid := range killCreditUserIds(d.DamageMap, m.Character.RoomId) {
		if user := users.GetByUserId(uid); user != nil {
			user.Character.KD.AddMobKill(int(m.MobId))
		}
	}
}

// killCreditUserIds is who gets the kill when a mob dies, for quest
// mob_death triggers and kill stats (owner ruling 2026-10-17): every
// player who damaged it, plus every party member of any of them who is in
// the room where it died. Each id appears once: damagers first, then
// party members, each group in ascending id order.
//
// Bounty claims, faction reputation, loot and gold, crime and progression
// do not read this set: a bounty pays only the top damager, once.
func killCreditUserIds(damage map[int]int, roomId int) []int {
	seen := map[int]bool{}
	damagers := make([]int, 0, len(damage))
	for uid := range damage {
		if uid > 0 && !seen[uid] {
			seen[uid] = true
			damagers = append(damagers, uid)
		}
	}
	sort.Ints(damagers)

	var members []int
	for _, uid := range damagers {
		p := parties.Get(uid)
		if p == nil {
			continue
		}
		for _, memberId := range p.GetMembers() {
			if seen[memberId] {
				continue
			}
			u := users.GetByUserId(memberId)
			if u == nil || u.Character.RoomId != roomId {
				continue
			}
			seen[memberId] = true
			members = append(members, memberId)
		}
	}
	sort.Ints(members)
	return append(damagers, members...)
}
