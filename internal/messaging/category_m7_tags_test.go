package messaging

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// M7 slice 4 (#371): the usage, admin and group categories are treated
// exactly as CategorySystem, where their lines rode, so moving them changes
// nothing a player sees until the owner recolours one.
var m7TagCategories = map[Category]string{
	CategoryUsage: "usage",
	CategoryAdmin: "admin",
	CategoryGroup: "group",
}

func TestM7TagCategoriesKeepSystemTreatment(t *testing.T) {
	for c, name := range m7TagCategories {
		require.Equal(t, name, c.String())
		require.Equal(t, skipStages(CategorySystem), skipStages(c), "%s must normalise as System", name)
		require.False(t, shouldWrap(c), "%s must not wrap: System never did", name)
		for _, v := range []Verbosity{VerbosityFull, VerbosityMedium, VerbosityLight} {
			require.False(t, v.Suppresses(c), "verbosity %v suppresses %s", v, name)
		}
	}
}

func TestM7TagAliasesMatchSystem(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "ansi-aliases.yaml"))
	require.NoError(t, err)
	var doc struct {
		Colors map[string]any `yaml:"colors"`
	}
	require.NoError(t, yaml.Unmarshal(data, &doc))
	for _, name := range m7TagCategories {
		require.Contains(t, doc.Colors, name)
		require.Equal(t, doc.Colors["system"], doc.Colors[name], "%s must keep system's colour", name)
	}
}
