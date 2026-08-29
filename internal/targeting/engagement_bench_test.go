package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
)

// BenchmarkEngagementOf measures the seam's hot-path cost. Once U12c lands,
// this runs per actor per round, so an allocation here is an allocation in
// every combat round in the game.
func BenchmarkEngagementOf(b *testing.B) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 42}, ReasonAttack)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EngagementOf(c)
	}
}

// BenchmarkEngagementOf_SpellCast is the allocating path: SpellTargets is a
// slice built per call. Measured separately so the ordinary melee case above
// is not judged by it.
func BenchmarkEngagementOf_SpellCast(b *testing.B) {
	c := characters.New()
	c.SetCast(2, characters.SpellAggroInfo{
		SpellId:              "burst",
		TargetUserIds:        []int{7},
		TargetMobInstanceIds: []int{88},
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EngagementOf(c)
	}
}
