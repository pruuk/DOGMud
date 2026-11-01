package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
)

// Skill coverage in Pothole Coulee (#375 phase 7 spec, "Skill coverage"): the
// NPCs who teach by talk name each skill's command, and the corrections hold.

// openInfo returns an info topic's text and hints, squashed, and requires it
// to be open to every player (no quest gate).
func openInfo(t *testing.T, dfile, key string) (text, hints string) {
	t.Helper()
	df := loadCouleeTopics(t, dfile)
	tp := df.FindTopic(key)
	require.NotNil(t, tp, "%s has no topic %q", dfile, key)
	require.Equal(t, dialogue.KindInfo, tp.Kind, "%s topic %s", dfile, key)
	require.Empty(t, tp.QuestRequired, "%s topic %s is open to everyone", dfile, key)
	return squash(tp.Text), squash(tp.Hints)
}

func TestVorn9108_TeachesTheFightBeyondTheSwing(t *testing.T) {
	cases := map[string][]string{
		"grapple_help": {"type grapple", "help grapple", "cannot flee"},
		"bash_help":    {"type bash", "help bash", "shield"},
		"kick_help":    {"stomp", "knee", "tailsweep", "help kick"},
		"defence_help": {"dodge", "parry", "block", "help combat"},
	}
	for key, wants := range cases {
		text, hints := openInfo(t, "9108.yaml", key)
		for _, w := range wants {
			require.Contains(t, text+" "+hints, w, "Vorn's %s must say %q", key, w)
		}
	}
}

// Shouts do not come later: rally has no rank gate, and quest 28 already
// teaches warcry (spec "Corrections").
func TestVorn9108_ShoutsAreNotForLater(t *testing.T) {
	text, hints := openInfo(t, "9108.yaml", "shouts_help")
	require.NotContains(t, text+" "+hints, "come to you later")
	require.NotContains(t, text+" "+hints, "come later")
	require.Contains(t, text, "warcry")
	require.Contains(t, hints, "type rally")
}

func TestOnna9104_TeachesTrading(t *testing.T) {
	cases := map[string]string{
		"appraise_help": "type appraise",
		"offer_help":    "type offer",
		"identify_help": "type cast identify",
		"barter_help":   "help bartering",
	}
	for key, want := range cases {
		_, hints := openInfo(t, "9104.yaml", key)
		require.Contains(t, hints, want, "Onna's %s hint names %q", key, want)
	}
	text, _ := openInfo(t, "9104.yaml", "identify_help")
	require.Contains(t, text, "no coin", "the identify spell is free of gold")
}

func TestCrafters_NameAStationRecipe(t *testing.T) {
	cases := []struct{ file, recipe string }{
		{"9605.yaml", "linen tunic"},
		{"9605.yaml", "scab-hide jerkin"},
		{"9606.yaml", "honed-edge"},
		{"9607.yaml", "copper ring"},
		{"9608.yaml", "trail rations"},
	}
	for _, c := range cases {
		text, hints := openInfo(t, c.file, "station_help")
		require.Contains(t, text+" "+hints, c.recipe, "%s names %s", c.file, c.recipe)
		require.Contains(t, hints, "type craft", "%s hint names the craft command", c.file)
	}
	text, _ := openInfo(t, "9606.yaml", "station_help")
	require.Contains(t, text, "disenchant")
	require.Contains(t, text, "in your pack", "disenchant reads the backpack (skill.disenchant.go)")

	// The help page agrees with the code and with Daff: disenchant takes an
	// item from the backpack, not an equipped one.
	help := string(repoFile(t, "_datafiles", "world", "dogmud", "templates", "help", "enchanting.template"))
	require.NotContains(t, help, "from an equipped item")
	require.Contains(t, help, "Remove an enchantment from an item in your pack.")
}

// Enchanter Skell (9606) is renamed so Bowmaster Skell (9169), quest 52's
// boss, keeps the name (spec "Skill coverage" ruling).
func TestEnchanter9606_IsDaff(t *testing.T) {
	mob := string(repoFile(t, "_datafiles", "world", "dogmud", "mobs", "pothole_coulee", "9606-enchanter_daff.yaml"))
	// The working tree is CRLF on Windows, so compare without line endings.
	mob = strings.ReplaceAll(mob, "\r\n", "\n")
	require.Contains(t, mob, "  name: Enchanter Daff\n")
	for _, rel := range [][]string{
		{"_datafiles", "world", "dogmud", "dialogue", "pothole_coulee", "9606.yaml"},
		{"_datafiles", "world", "dogmud", "schedules", "pothole_coulee", "pothole_coulee_enchanter.yaml"},
		{"_datafiles", "world", "dogmud", "mobs", "pothole_coulee", "9607-jeweler_vurl.yaml"},
	} {
		require.NotContains(t, string(repoFile(t, rel...)), "Skell", "%v still names Skell", rel)
	}
}
