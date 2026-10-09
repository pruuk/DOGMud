package usercommands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// #449: the admin view of `online` adds UserId, Zone and RoomId to the
// player columns and ran to 102 columns. It drops Title; players keep it.
func TestOnlineColumns_AdminViewDropsTitle(t *testing.T) {
	require.Equal(t, []string{`UserId`, `User.Name`, `Online`, `Role`, `Zone`, `RoomId`}, onlineColumns(true))
	require.Equal(t, []string{`User.Name`, `Title`, `Online`, `Role`}, onlineColumns(false))
}
