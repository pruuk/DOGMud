package dialogue

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWalkAllFiles_VisitsEveryZoneFile(t *testing.T) {
	dir := overrideDataFilesDir(t)

	write := func(zone string, mobId int, body string) {
		p := filepath.Join(dir, "dialogue", zone)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, fmt.Sprintf("%d.yaml", mobId)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("townzone", 9, "mobid: 9\nzone: townzone\nopener:\n  text: \"Hi.\"\ntopics:\n  - key: hello\n    kind: info\n    label: \"Hello.\"\n    text: \"Hi.\"\n")
	write("wildzone", 12, "mobid: 12\nzone: wildzone\nopener:\n  text: \"Hiss.\"\ntopics:\n  - key: snake\n    kind: info\n    label: \"A snake?\"\n    text: \"Hiss.\"\n")
	// An old-format file no longer decodes strictly, so the walker skips it.
	write("oldzone", 13, "mobid: 13\nzone: oldzone\npatterns:\n  - keywords: [\"hello\"]\n    responses: [\"Hi.\"]\n")

	seen := map[int]string{}
	WalkAllFiles(func(mobId int, zone string, df *DialogueFile) {
		seen[mobId] = zone
		if df == nil || len(df.Topics) != 1 {
			t.Errorf("mob %d: file not parsed", mobId)
		}
	})

	if seen[9] != "townzone" || seen[12] != "wildzone" {
		t.Fatalf("walker missed files: %v", seen)
	}
	if _, ok := seen[13]; ok {
		t.Error("an old-format file must not decode now that patterns are gone")
	}
}
