package follow

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// `help follow` (owner 2026-10-17): the module ships the topic, registers it
// in the help index, explains party auto-follow and the light rule, links
// back to `help party`, and wraps at 80 columns.

var followHelpTag = regexp.MustCompile(`</?ansi[^>]*>`)

func TestFollowHelp_ResolvesThroughThePluginRegistry(t *testing.T) {
	// main.go registers the plugin registry with templates exactly so; this
	// test binary ran this module's init, so the registry holds `follow`.
	templates.RegisterFS(plugins.GetPluginRegistry())
	require.True(t, templates.Exists("help/follow"), "help follow resolves at runtime")
	out, err := templates.Process("help/follow", nil)
	require.NoError(t, err)
	assert.NotContains(t, out, "[TEMPLATE")
	assert.Contains(t, out, `<ansi fg="command">help party</ansi>`, "help follow links to help party")
}

func TestFollowHelp_CoversPartyAndLightAndWraps(t *testing.T) {
	b, err := files.ReadFile("files/datafiles/templates/help/follow.template")
	require.NoError(t, err)
	visible := followHelpTag.ReplaceAllString(string(b), "")
	for _, want := range []string{"follow stop", "follow lose", "party leader", "carry a light", "carry none"} {
		assert.Contains(t, visible, want)
	}
	for i, line := range strings.Split(visible, "\n") {
		assert.LessOrEqual(t, len([]rune(line)), 80, "line %d is wider than 80 columns: %q", i+1, line)
	}
	assert.NotRegexp(t, `[\x{2013}\x{2014}]`, visible, "no em or en dashes in player copy")
}

func TestFollowHelp_IsInTheHelpIndex(t *testing.T) {
	b, err := files.ReadFile("files/data-overlays/keywords.yaml")
	require.NoError(t, err)
	var kw struct {
		Help map[string]map[string][]string `yaml:"help"`
	}
	require.NoError(t, yaml.Unmarshal(b, &kw))
	assert.Contains(t, kw.Help["command"]["parties"], "follow", "follow is listed beside party in the help index")
}
