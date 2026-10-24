package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/require"
)

func TestWireConversationHooks_InstallsTheNpcMarker(t *testing.T) {
	prev := rooms.NpcMarker
	t.Cleanup(func() { rooms.NpcMarker = prev })

	rooms.NpcMarker = nil
	wireConversationHooks()
	require.NotNil(t, rooms.NpcMarker, "boot must install the NPC marker hook")
}
