package actions

import (
	"fmt"
	"maps"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/worldevents"
)

// EarnMutation spends one earned mutation event for a player (#680). The
// combat tick calls it when progress crosses the threshold; the `earnmutation`
// admin command calls it directly, so a playtest forces the code a fight runs.
//
// It deepens or acquires as the tick always has, through CanDeepen,
// RollDeepening and GetGraphPool, all of which honour CanGain. A deepening
// that finds nothing falls to acquiring. Progress is spent, and the
// mutation-graph drift fades, only on a grant: when nothing fits, both carry
// to the next event. Returns the mutation id granted, or "".
func EarnMutation(user *users.UserRecord) string {
	c := user.Character
	mb := configs.GetBalanceConfig()
	canAcquire := len(c.Mutations) < int(mb.MutationMaxCount)
	canDeepen := mutations.CanDeepen(c.Mutations)

	// Decide: deepen an existing mutation or acquire a new one.
	doDeepen := false
	if canAcquire && canDeepen {
		// Both possible: coin flip weighted toward deepening.
		doDeepen = util.Rand(100) < int(mb.MutationDeepenChance*100)
	} else if canDeepen {
		// At max count: must deepen.
		doDeepen = true
	}

	if doDeepen {
		if id := mutations.RollDeepening(c.Mutations); id != `` {
			c.MutationProgress = 0
			// Mutation-graph drift fades on each mutation event so recent behavior dominates.
			mutations.DecayAffinity(c.ClusterAffinity, float64(mb.MutationAffinityDecay))
			c.Mutations[id]++
			events.AddToQueue(mutations.Gained{
				UserId:     user.UserId,
				MutationId: id,
				Rank:       c.Mutations[id],
				IsNew:      false,
			})
			return id
		}
		// No owned mutation could take another rank: fall through to acquiring.
	}

	if !canAcquire {
		return ``
	}

	// The roll reads the drift as faded by this mutation event; the faded
	// copy is kept only if something is granted.
	decayed := maps.Clone(c.ClusterAffinity)
	mutations.DecayAffinity(decayed, float64(mb.MutationAffinityDecay))
	// Pass the user's species so body-part requirements gate the pool.
	sp := species.GetSpecies(c.SpeciesId)
	aff := mutations.EffectiveAffinity(c.Mutations, decayed)
	pool := mutations.GetGraphPool(c.Mutations, aff, sp)
	if len(pool) == 0 {
		return ``
	}
	id := mutations.RollAcquisition(pool)
	c.ClusterAffinity = decayed
	c.MutationProgress = 0
	if c.Mutations == nil {
		c.Mutations = make(map[string]int)
	}
	c.Mutations[id] = 1
	events.AddToQueue(mutations.Gained{
		UserId:     user.UserId,
		MutationId: id,
		Rank:       1,
		IsNew:      true,
	})
	emitMutationMilestone(user, id)
	return id
}

// emitMutationMilestone tells the gossip system a player acquired a new
// mutation: regional news, global for rarity 8 and up.
func emitMutationMilestone(user *users.UserRecord, mutId string) {
	spec := mutations.GetMutation(mutId)
	if spec == nil {
		return
	}
	sig := worldevents.Regional
	if spec.Rarity >= 8 {
		sig = worldevents.Global
	}
	zone := user.Character.Zone
	region := ""
	if zCfg := rooms.GetZoneConfig(zone); zCfg != nil {
		region = zCfg.Region
	}
	worldevents.EmitWorldEvent(worldevents.WorldEvent{
		Type:         worldevents.PlayerMutationMilestone,
		Significance: sig,
		ZoneName:     zone,
		RegionName:   region,
		PlayerName:   user.Character.Name,
		Description: fmt.Sprintf("%s has undergone a mutation: %s.",
			user.Character.Name, spec.Name),
	})
}
