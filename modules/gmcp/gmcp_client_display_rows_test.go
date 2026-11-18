package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/stretchr/testify/require"
)

func TestReportedRows(t *testing.T) {
	rows, ok := reportedRows([]byte(`{"cols": 60, "rows": 31}`))
	require.True(t, ok)
	require.Equal(t, 31, rows)
	for _, bad := range []string{`{"cols": 60}`, `{"rows": 3.5}`, `{"rows": "31"}`, `{"rows": null}`, `nope`} {
		_, ok := reportedRows([]byte(bad))
		require.False(t, ok, bad)
	}
}

func TestClientDisplay_StoresRowsClamped(t *testing.T) {
	id := clientDisplayConn(t, 80)
	reportCols(id, `{"cols": 60, "rows": 31}`)
	require.Equal(t, 31, connections.GetClientSettings(id).Display.GetScreenHeight())
	reportCols(id, `{"cols": 60, "rows": 4}`)
	require.Equal(t, 10, connections.GetClientSettings(id).Display.GetScreenHeight(), "clamped to the pager's floor")
	reportCols(id, `{"screenreader": true}`)
	require.Equal(t, 10, connections.GetClientSettings(id).Display.GetScreenHeight(), "a report without rows keeps the height")
}
