package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestThrowInterruptAudienceNamesTheActee proves the cast-interrupt event's
// Audience carries the mob's name, so SendTrio can hide it from a reader who
// cannot see.
//
// throw is area-effect and genuinely actee-less, so it builds ONE Audience with
// ActeeName: NoName and reuses it for every event. cast_interrupt is the
// exception: it renders a single mob's name into throw.yaml's {actee_plain}.
// NoName means "nobody to hide" at trio.go:127 and hidenames.go:48, so before
// this fix the mob's name reached every reader at every sight tier.
func TestThrowInterruptAudienceNamesTheActee(t *testing.T) {
	base := messaging.Audience{
		ActorName: "Aliceia",
		ActeeName: messaging.NoName,
	}

	got := throwInterruptAudience(base, "Cave Crawler")

	require.Equal(t, messaging.NoName, base.ActeeName,
		"the base Audience must not be mutated: every other throw event depends on it staying actee-less")
	assert.Equal(t, "Cave Crawler", got.ActeeName,
		"the interrupt event must name the mob so SendTrio can hide it")
	assert.Equal(t, "Aliceia", got.ActorName,
		"the copy must keep every other field")
}
