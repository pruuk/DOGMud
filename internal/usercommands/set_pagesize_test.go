package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/pager"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

func TestSetPageSize(t *testing.T) {
	user := users.NewTestUser(8802, "pager", "Pager", 18802)
	events.DrainQueuedMessagesForTest(8802)
	defer events.DrainQueuedMessagesForTest(8802)

	cases := []struct {
		arg  string
		want int
		says string
	}{
		{"30", 30, "Pages are 30 lines."},
		{"off", pager.SettingOff, "Paging is off."},
		{"auto", 0, "Pages follow your screen."},
		{"5", 0, "Page size must be a number between 10 and 200"},
		{"tall", 0, "Page size must be a number between 10 and 200"},
	}
	for _, c := range cases {
		user.PageSize = 0
		_, err := Set("pagesize "+c.arg, user, nil, events.EventFlag(0))
		require.NoError(t, err)
		require.Equal(t, c.want, user.PageSize, c.arg)
		sent := strings.Join(events.DrainQueuedMessagesForTest(8802), "")
		require.Contains(t, sent, c.says, c.arg)
	}

	// With no argument it says the size the pager uses now: the connection
	// reports no height, so the default 24 rows less the stop line and
	// prompt; a screen reader account pages at 20.
	user.PageSize = 0
	_, err := Set("pagesize", user, nil, events.EventFlag(0))
	require.NoError(t, err)
	sent := strings.Join(events.DrainQueuedMessagesForTest(8802), "")
	require.Contains(t, sent, "Pages follow your screen")
	require.Contains(t, sent, "Pages follow your screen (22 lines).")

	user.ScreenReader = true
	defer func() { user.ScreenReader = false }()
	_, err = Set("pagesize", user, nil, events.EventFlag(0))
	require.NoError(t, err)
	sent = strings.Join(events.DrainQueuedMessagesForTest(8802), "")
	require.Contains(t, sent, "Pages follow your screen (20 lines).")
}
