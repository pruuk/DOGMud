package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
)

func TestActorRef_Player(t *testing.T) {
	c := &Character{}
	c.SetUserId(7)
	assert.Equal(t, state.ActorRef{UserId: 7}, c.ActorRef())
	assert.False(t, c.ActorRef().IsZero())
}

// A mob's ref must be non-zero. This is the exact condition that made
// RecordInboundAttacker early-return on ActorRef.IsZero() and is why a
// repaired registry alone would still never have recorded a mob attacker.
func TestActorRef_Mob(t *testing.T) {
	c := &Character{MobInstanceId: 42}
	assert.Equal(t, state.ActorRef{MobInstanceId: 42}, c.ActorRef())
	assert.False(t, c.ActorRef().IsZero())
}

func TestActorRef_UnidentifiedIsZero(t *testing.T) {
	c := &Character{}
	assert.True(t, c.ActorRef().IsZero())
}
