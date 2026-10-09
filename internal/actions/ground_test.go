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
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/require"
)

// #382 playtest: "On the Ground: corpse of a figure, ..." printed one line of
// 116 columns. Like the roster (#430) it goes out as room-description or
// system text, which the pipeline does not wrap. RenderGround wraps it to the
// reader's width.
func TestRenderGround_WrapsToTheLineWidth(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud")
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(root)
	configs.SetConfigForTest(t, cfg)
	templates.SetFSForTest(t, os.DirFS(root).(fs.ReadFileFS))

	out := RenderGround([]string{
		"corpse of a figure",
		"corpse of a figure",
		"Iron Longsword",
		"Torch",
		"Cotton Shirt",
		"Worn Boots",
	}, true, false, 0)
	require.Contains(t, out, "On the Ground:")
	require.Contains(t, out, "Worn Boots")

	tags := regexp.MustCompile(`<[^>]*>`)
	plain := tags.ReplaceAllString(out, "")
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	require.Greater(t, len(lines), 1, "a ground list this long must wrap: %q", plain)
	for _, line := range lines {
		require.LessOrEqual(t, len([]rune(line)), 80, "ground line too wide: %q", line)
	}

	require.Empty(t, RenderGround(nil, false, false, 0), "an empty floor prints nothing")
}

// Every ground send goes through RenderGround, so no caller can print the
// list unwrapped again. It walks internal/ and modules/, and matches the
// template name in double quotes or backticks.
func TestOnTheGroundTemplateRenderedOnlyByRenderGround(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	internalDir := filepath.Join(filepath.Dir(here), "..")
	modulesDir := filepath.Join(internalDir, "..", "modules")
	for _, dir := range []string{internalDir, modulesDir} {
		scanned := 0
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			src := string(data)
			names := strings.Contains(src, `"descriptions/ontheground"`) ||
				strings.Contains(src, "`descriptions/ontheground`")
			if names && filepath.Base(path) != "roster.go" {
				t.Errorf("%s renders descriptions/ontheground directly; use actions.RenderGround", path)
			}
			return nil
		})
		require.NoError(t, err)
		require.NotZero(t, scanned, "the walk of %s found no Go files, so it proves nothing", dir)
	}
}
