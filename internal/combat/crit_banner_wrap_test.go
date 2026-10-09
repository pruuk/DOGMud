package combat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"gopkg.in/yaml.v2"
)

// seedShippedGenericForBannerTest installs the shipped generic.yaml as the
// only attack message pool, so a crit swing builds real lines.
func seedShippedGenericForBannerTest(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRootForTest(t),
		"_datafiles", "world", "dogmud", "combat-messages", "generic.yaml"))
	if err != nil {
		t.Fatalf("read generic.yaml: %v", err)
	}
	var g items.WeaponAttackMessageGroup
	if err := yaml.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse generic.yaml: %v", err)
	}
	t.Cleanup(items.SeedAttackMessagesForTest(map[items.ItemSubType]*items.WeaponAttackMessageGroup{
		items.Generic: &g,
	}))
}

// #455: the wrapper breaks at any ASCII space, so a crit line one word too
// long left its closing *** alone on the next line ("... Early Strider!" /
// "***"). The close is now joined to the last word by a no-break space
// (U+00A0), which the wrapper never breaks at.
func TestBuildAttackMessages_CritBannerCloseStaysWithTheLastWord(t *testing.T) {
	seedShippedGenericForBannerTest(t)
	src, tgt := defenceFixture(1000)
	src.Name = "Rurik"
	tgt.Name = "Selka"
	tgt.HealthMax.Base = 100
	tgt.HealthMax.Recalculate()

	result := &AttackResult{Crit: true}
	ws := weaponSetup{weaponName: "Iron Longsword", weaponSubType: items.Generic}
	sdp := swingDamageParams{dmgMean: 20}
	buildAttackMessages(result, src, tgt, ws, sdp,
		30, 0, 0, 0, User, User, "", false, false)

	const closeBanner = `<ansi fg="crit-text">***</ansi>`
	lines := append(append(append([]TaggedMessage{}, result.MessagesToSource...), result.MessagesToTarget...), result.MessagesToSourceRoom...)
	if len(lines) < 3 {
		t.Fatalf("a crit swing built %d lines, want the attacker, defender and room lines", len(lines))
	}
	for _, m := range lines {
		if !strings.Contains(m.Text, "!") && !strings.Contains(m.Text, ".") {
			t.Fatalf("fixture built an empty crit line: %q", m.Text)
		}
		if !strings.HasSuffix(m.Text, string(rune(0x00A0))+closeBanner) {
			t.Errorf("the closing banner is not joined to the last word: %q", m.Text)
		}
		// Wrapped at every width from 20 to 80, no line is the banner alone.
		for width := 20; width <= 80; width++ {
			for _, line := range strings.Split(messaging.WrapAnsi(m.Text, width), "\n") {
				if strings.TrimSpace(remoteRoomHidingTag.ReplaceAllString(line, "")) == "***" {
					t.Errorf("at width %d the closing *** wrapped onto a line of its own: %q", width, m.Text)
				}
			}
		}
	}
}
