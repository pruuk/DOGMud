package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #461: `room info` rendered [TEMPLATE ERROR] because roominfo.template read
// Room.SkillTraining, a field removed in 0f83dfc96. This renders the shipped
// template against a real room so a later field removal fails here.
func TestRoomInfo_ShippedTemplateRenders(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)
	admin, room := getTestUserAndRoom(t)
	oldRole := admin.Role
	admin.Role = users.RoleAdmin
	t.Cleanup(func() { admin.Role = oldRole })
	events.DrainQueuedMessagesForTest(admin.UserId)

	handled, err := adminRoom_Info([]string{"info"}, admin, room)
	require.NoError(t, err)
	require.True(t, handled)

	out := strings.Join(events.DrainQueuedMessagesForTest(admin.UserId), "\n")
	assert.NotContains(t, out, "[TEMPLATE ERROR]")
	assert.Contains(t, out, "RoomId:")
}
