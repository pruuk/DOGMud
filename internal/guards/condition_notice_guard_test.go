package guards

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Slice C: no condition may apply or expire in silence for its holder. The
// engine falls back to a generic "takes effect" / "has expired" line
// (conditions.StartUserNotice / EndUserNotice), but that is a runtime net,
// never the shipped experience, so every non-secret condition in the dogmud
// world must carry authored start_actee and end_actee, and a secret
// condition must carry no player text at all. Forty-three conditions were
// fully silent and six half-silent on 2026-09-12; this keeps the count at
// zero and names the file that regresses it.
//
// Two flags opt a condition out of one half of that rule, by design, not by
// oversight:
//   - silent-start: the applier narrates the start. Warcry and rally are
//     applied via Character.AddCondition, which never queues the condition
//     event, so no start line could reach the holder; the bloom detox drink
//     does reach Condition_ApplyConditions on the unscaled drink path and the
//     flag stops the purge narration being doubled. Only start_actee is
//     waived.
//   - hidden: the holder must never learn when their cover lapsed, so no end
//     notice is allowed to exist at all. Only end_actee is waived.
//   - quiet: listed but never announced, for a record reapplied every round
//     it persists.
func TestEveryDogmudConditionHasAuthoredNotices(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "conditions", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no condition files found: %v", err)
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
			StartUserText   string   `yaml:"start_actee"`
			StartRoomText   string   `yaml:"start_observer"`
			TriggerUserText string   `yaml:"trigger_actee"`
			TriggerRoomText string   `yaml:"trigger_observer"`
			EndUserText     string   `yaml:"end_actee"`
			EndRoomText     string   `yaml:"end_observer"`
		}
		if err := yaml.Unmarshal(raw, &b); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		base := filepath.Base(f)
		if b.Secret {
			if b.StartUserText+b.StartRoomText+b.TriggerUserText+b.TriggerRoomText+b.EndUserText+b.EndRoomText != "" {
				problems = append(problems, base+": secret condition carries player text")
			}
			continue
		}
		silentStart := slices.Contains(b.Flags, "silent-start")
		hidden := slices.Contains(b.Flags, "hidden")
		quiet := slices.Contains(b.Flags, "quiet")
		if strings.TrimSpace(b.Name) == "" {
			problems = append(problems, base+": non-secret condition has no name (the generic notice would be blank)")
		}
		if strings.TrimSpace(b.StartUserText) == "" && !silentStart && !quiet {
			problems = append(problems, base+": missing start_actee (holder would read the generic line)")
		}
		if strings.TrimSpace(b.EndUserText) == "" && !hidden && !quiet {
			problems = append(problems, base+": missing end_actee (holder would read the generic line)")
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d condition notice problems:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}
