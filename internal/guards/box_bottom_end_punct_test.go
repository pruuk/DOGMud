package guards

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/util"
)

var boxBottomTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

// TestBoxedScreensEndWithoutAStop renders every boxed screen the 2026-10-17
// playtests saw end in `+----...----+.` (conditions, cooldowns, quests all,
// skills, the shop list, and the login who table, which is tables/generic)
// from the live templates, sends each through the pipeline as its command
// does (CategorySystem), downgrades it as an ASCII client receives it, and
// checks the last line is still the box's bottom edge with no stop after it
// (#460).
func TestBoxedScreensEndWithoutAStop(t *testing.T) {
	// Process tries a template's .md variant on disk under DataFiles before
	// the registered FS's .template, so DataFiles must name the live world
	// too. SetConfigForTest restores the config afterwards; an overlay would
	// outlive the test and send a later test's ReloadConfig to the
	// vestigial world.
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dogmudDataFilesRoot)
	configs.SetConfigForTest(t, cfg)
	templates.RegisterFS(os.DirFS(dogmudDataFilesRoot).(fs.ReadFileFS))

	type condition struct {
		Name        string
		Description string
		RoundsLeft  int
		Permanent   bool
	}
	table := templates.GetTable("Wares", []string{"Qty", "Name", "Type", "Price"},
		[][]string{{"3", "Torch", "light", "1"}, {"1", "Oil Lantern", "light", "1"}})
	online := templates.GetTable("Online", []string{"Name", "Level", "Alignment"},
		[][]string{{"Fixturewick", "1", "neutral"}})

	screens := []struct {
		name, tpl string
		data      any
	}{
		{"conditions", "character/conditions", []condition{{Name: "Bleeding", Description: "You are losing blood.", RoundsLeft: 4}}},
		{"conditions empty", "character/conditions", []condition{}},
		{"cooldowns", "character/cooldowns", map[string]int{"bash": 3}},
		{"quests all", "character/quests", map[string]any{
			"QuestsFound": 1, "QuestsTotal": 1,
			"Records": []map[string]any{{"Name": "Hedgerow Toll", "Description": "Pay the toll.",
				"BarFull": "##", "BarEmpty": "..", "Completion": "50%"}},
		}},
		{"skills", "character/skills", map[string]any{
			"SkillList": map[string]int{"blacksmithing": 1}, "SkillCooldowns": map[string]int{"blacksmithing": 0},
			"SkillBlurbs": map[string]string{}, "NameWidth": 13,
		}},
		{"shop list", "tables/shoplist", table},
		{"who at login", "tables/generic", online},
	}
	for _, s := range screens {
		rendered, err := templates.Process(s.tpl, s.data)
		if err != nil {
			t.Fatalf("%s: templates.Process(%s): %v", s.name, s.tpl, err)
		}
		out := messaging.RenderForRecipient(messaging.RenderInput{
			Category:  messaging.CategorySystem,
			Text:      rendered,
			Channel:   messaging.ChannelAudio,
			LineWidth: 80,
		})
		for _, mode := range []string{"utf-8", "ascii"} {
			text := boxBottomTagRE.ReplaceAllString(out, "")
			if mode == "ascii" {
				text = util.ConvertToAscii(text)
			}
			var last string
			for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
				if strings.TrimSpace(l) != "" {
					last = strings.TrimRight(l, " ")
				}
			}
			if strings.HasSuffix(last, ".") {
				t.Errorf("%s (%s) ends with a stop after the box: %q", s.name, mode, last)
			}
			edge := strings.TrimLeft(last, " ")
			if !strings.HasPrefix(edge, "└") && !strings.HasPrefix(edge, "+") {
				t.Errorf("%s (%s): the last line is not the box bottom, so this check proves nothing: %q", s.name, mode, last)
			}
		}
	}
}
