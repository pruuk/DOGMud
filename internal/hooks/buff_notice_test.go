package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Buff ids clear of the fixture and of the narration buffs (7001-7007).
const (
	quietBuffId  = 7101 // no authored text at all
	hushedBuffId = 7102 // secret, with authored text that must never show
)

func seedNoticeBuffs() func() {
	return buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		quietBuffId:  {BuffId: quietBuffId, Name: "Test Quiet", RoundInterval: 5, TriggerCount: 3},
		hushedBuffId: {BuffId: hushedBuffId, Name: "Test Hushed", Secret: true, RoundInterval: 5, TriggerCount: 3, StartUserText: "You should never read this.", EndUserText: "Nor this."},
	})
}

func TestBuffNotice_HolderReadsTheGenericStartLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	drainPlain(1)
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: quietBuffId}))
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Quiet takes effect."))
	assert.Equal(t, 0, countContaining(drainPlain(2), "Test Quiet"), "no room line was authored, so the room hears nothing")
}

func TestBuffNotice_HolderReadsTheGenericEndLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(quietBuffId, false))
	expire(t, holder.Character.Buffs.List, quietBuffId)
	drainPlain(1)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Quiet has expired."))
}

func TestBuffNotice_SecretBuffIsSilentAtBothEnds(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	holder := users.GetByUserId(1)
	drainPlain(1)

	ApplyBuffs(events.Buff{UserId: 1, BuffId: hushedBuffId})
	assert.Empty(t, drainPlain(1), "a secret buff's authored start text must not be sent")

	expire(t, holder.Character.Buffs.List, hushedBuffId)
	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Empty(t, drainPlain(1), "a secret buff's authored end text must not be sent")
}
