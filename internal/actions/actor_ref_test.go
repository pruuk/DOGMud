package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
)

// idStubActor is stubActor with an identity, so a test can see which actor a
// bleed's caster names. stubActor itself answers 0 for both ids.
type idStubActor struct {
	*stubActor
	userId, mobInstanceId int
}

func (a *idStubActor) GetUserId() int        { return a.userId }
func (a *idStubActor) GetMobInstanceId() int { return a.mobInstanceId }

func TestActorRefOf(t *testing.T) {
	assert.Equal(t, state.ActorRef{UserId: 5}, ActorRefOf(&idStubActor{stubActor: newStubActor(nil, nil), userId: 5}))
	assert.Equal(t, state.ActorRef{MobInstanceId: 9}, ActorRefOf(&idStubActor{stubActor: newStubActor(nil, nil), mobInstanceId: 9}))
	assert.True(t, ActorRefOf(nil).IsZero())
}
