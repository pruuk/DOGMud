package actions

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/require"
)

// #430: the "Also here:" roster is sent as room-description or system text,
// neither of which the messaging pipeline wraps, so a busy tavern printed one
// line well past 80 columns. RenderRoster wraps it to the reader's width.
func TestRenderRoster_WrapsToTheLineWidth(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud")
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(root)
	configs.SetConfigForTest(t, cfg)
	templates.SetFSForTest(t, os.DirFS(root).(fs.ReadFileFS))

	details := rooms.RoomTemplateDetails{
		VisibleMobs: []string{
			"Tavern Keeper Marek (100%)",
			"Blacksmith Kerra (100%|shop|Lit)",
			"Temple Priest Olen (100%)",
			"Barmaid Dal (100%)",
		},
	}
	out := RenderRoster(details, 0)
	require.Contains(t, out, "Also here:")
	require.Contains(t, out, "Barmaid Dal")

	tags := regexp.MustCompile(`<[^>]*>`)
	plain := tags.ReplaceAllString(out, "")
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	require.Greater(t, len(lines), 1, "a roster this long must wrap: %q", plain)
	for _, line := range lines {
		require.LessOrEqual(t, len([]rune(line)), 80, "roster line too wide: %q", line)
	}
}
