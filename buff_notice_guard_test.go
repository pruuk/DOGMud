package main

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Slice C: no buff may apply or expire in silence for its holder. The engine
// falls back to a generic "takes effect" / "has expired" line
// (buffs.StartUserNotice / EndUserNotice), but that is a runtime net, never
// the shipped experience, so every non-secret buff in the dogmud world must
// carry authored start_user_text and end_user_text, and a secret buff must
// carry no player text at all. Forty-three buffs were fully silent and six
// half-silent on 2026-09-12;
// this keeps the count at zero and names the file that regresses it.
//
// Two flags opt a buff out of one half of that rule, by design, not by
// oversight:
//   - silent-start: the applier narrates the start. Warcry and rally are
//     applied via Character.AddBuff, which never queues the buff event, so no
//     start line could reach the holder; the bloom detox drink does reach
//     Buff_ApplyBuffs on the unscaled drink path and the flag stops the
//     purge narration being doubled. Only start_user_text is waived.
//   - hidden: the holder must never learn when their cover lapsed, so no end
//     notice is allowed to exist at all. Only end_user_text is waived.
func TestEveryDogmudBuffHasAuthoredNotices(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "buffs", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no buff files found: %v", err)
	}
	sort.Strings(files)
	var problems []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var b struct {
			Name            string   `yaml:"name"`
			Secret          bool     `yaml:"secret"`
			Flags           []string `yaml:"flags"`
			StartUserText   string   `yaml:"start_user_text"`
			StartRoomText   string   `yaml:"start_room_text"`
			TriggerUserText string   `yaml:"trigger_user_text"`
			TriggerRoomText string   `yaml:"trigger_room_text"`
			EndUserText     string   `yaml:"end_user_text"`
			EndRoomText     string   `yaml:"end_room_text"`
		}
		if err := yaml.Unmarshal(raw, &b); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		base := filepath.Base(f)
		if b.Secret {
			if b.StartUserText+b.StartRoomText+b.TriggerUserText+b.TriggerRoomText+b.EndUserText+b.EndRoomText != "" {
				problems = append(problems, base+": secret buff carries player text")
			}
			continue
		}
		silentStart := slices.Contains(b.Flags, "silent-start")
		hidden := slices.Contains(b.Flags, "hidden")
		if strings.TrimSpace(b.Name) == "" {
			problems = append(problems, base+": non-secret buff has no name (the generic notice would be blank)")
		}
		if strings.TrimSpace(b.StartUserText) == "" && !silentStart {
			problems = append(problems, base+": missing start_user_text (holder would read the generic line)")
		}
		if strings.TrimSpace(b.EndUserText) == "" && !hidden {
			problems = append(problems, base+": missing end_user_text (holder would read the generic line)")
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d buff notice problems:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}
