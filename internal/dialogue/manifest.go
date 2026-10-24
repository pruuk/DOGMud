package dialogue

import (
	"os"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/manifest"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Manifest is the committed list of shipped topic keys
// (dialogue/topic-keys.txt); the format lives in internal/manifest. Saved
// characters point at these keys (HeardTopics), so a key may leave content
// only through a retired or renamed line.
type Manifest = manifest.Manifest

// ParseManifest reads topic-keys.txt.
func ParseManifest(data []byte) (Manifest, error) {
	return manifest.Parse("topic-keys.txt", data)
}

// HeardRenames reads the shipped manifest's renames for the load migration.
// A missing or unreadable manifest renames nothing.
func HeardRenames() map[string]string {
	path := util.FilePath(string(configs.GetFilePathsConfig().DataFiles) + `/dialogue/topic-keys.txt`)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	m, err := ParseManifest(data)
	if err != nil {
		mudlog.Warn("dialogue.HeardRenames()", "path", path, "error", err)
		return nil
	}
	return m.Renamed
}
