package mutations

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// CanGain is the one eligibility rule for an earned mutation grant (#680).
// newRank 1 is an acquisition; above 1 deepens an owned mutation one step.
// Every pool and every deepening pick goes through it, so a rank the body
// cannot reserve for is never rolled and the grant falls to something else.
//
// Authored sets (species intrinsics, mob spawn lists, presets) do not come
// through here: prerequisites govern earned growth only (owner, 2026-10-26),
// and a guard checks authored sets against the budget.
func CanGain(owned map[string]int, id string, newRank int, sp *species.Species) bool {
	spec := GetMutation(id)
	if spec == nil || newRank < 1 || newRank > MaxRankOf(id) {
		return false
	}
	if newRank == 1 {
		if _, has := owned[id]; has {
			return false
		}
		if HasConflict(owned, id) || !spec.CanApplyTo(sp) || !PrereqsMet(owned, spec) {
			return false
		}
		// Moon-gated flavor (Reflect Skin): only the current-bucket variant.
		if spec.MoonFlavor != 0 && spec.MoonFlavor != gametime.CurrentMoonFlavorBucket() {
			return false
		}
	} else if owned[id] != newRank-1 {
		return false
	}
	return FitsReserveBudget(owned, id, newRank)
}

// MaxRankOf is the deepest rank a mutation can reach: its max_rank, or the
// global MutationMaxLevel. Bloom used the global cap alone (#259).
func MaxRankOf(id string) int {
	return effectiveMax(id, int(configs.GetBalanceConfig().MutationMaxLevel))
}

// RelatedFit picks the replacement for a refused id from the candidates that
// do fit: one sharing a cluster with the refused id if any exists, otherwise
// any candidate, otherwise "".
func RelatedFit(refused string, fit []string) string {
	var same []string
	for _, id := range fit {
		if sharesCluster(refused, id) {
			same = append(same, id)
		}
	}
	if len(same) > 0 {
		return same[util.Rand(len(same))]
	}
	if len(fit) > 0 {
		return fit[util.Rand(len(fit))]
	}
	return ""
}

func sharesCluster(a, b string) bool {
	sa, sb := GetMutation(a), GetMutation(b)
	if sa == nil || sb == nil {
		return false
	}
	for _, ca := range sa.Clusters {
		for _, cb := range sb.Clusters {
			if ca == cb {
				return true
			}
		}
	}
	return false
}
