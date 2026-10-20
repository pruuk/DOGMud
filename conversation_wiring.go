package main

import (
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// wireConversationHooks installs the callbacks internal/rooms cannot import
// directly (internal/conversation imports rooms). Called once at boot; the
// server does not rely on conversation's init being linked in by another
// package.
func wireConversationHooks() {
	rooms.NpcMarker = conversation.Marker
}
