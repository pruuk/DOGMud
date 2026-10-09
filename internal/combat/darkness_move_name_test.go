package combat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// Spec F2 (ruling R8) hides a WEAPON from a reader below full sight. The
// defence templates tag {attack} as an item, but {attack} names a move on
// most paths: the unarmed "strike", and every channel defence (kick, aimed
// shot, firebomb, a spell). These tests render the SHIPPED defence pools so
// a move name is never read as "weapon", while an armed swing still is.

// seedShippedDefencePools loads the dogmud defense-messages store through
// the production loader and installs it for the test.
func seedShippedDefencePools(t *testing.T) {
	t.Helper()
	pools, err := fileloader.LoadAllFlatFiles[items.DefencePool, *items.DefenseMessageGroup]("../../_datafiles/world/dogmud/defense-messages")
	if err != nil {
		t.Fatal(err)
	}
	if pools[items.DefencePoolFor(combatvocab.DefenceDodge)] == nil {
		t.Fatal("shipped dodge pool did not load")
	}
	t.Cleanup(items.SeedDefenseMessagesForTest(pools))
}

// testDefenceBands covers the three bands RenderDefenseMessage can select.
var testDefenceBands = []defenceBand{{crit: false, margin: 0}, {crit: false, margin: 10}, {crit: true, margin: 0}}

// shapesSpectator runs a room line through the real per-recipient pipeline
// for a reader who sees shapes only.
func shapesSpectator(m TaggedMessage) string {
	return messaging.RenderForRecipient(messaging.RenderInput{
		Category:      m.Category,
		Text:          m.Text,
		Channel:       messaging.ChannelVisual,
		SightDecision: messaging.SightShapes,
		LineWidth:     400,
	})
}

func hasWeaponWord(s string) bool {
	return strings.Contains(strings.ToLower(s), messaging.WeaponWord)
}

// (a) A kick that is dodged, watched at shapes: the spectator reads "kick".
func TestShapesSpectator_DodgedKickKeepsTheMoveName(t *testing.T) {
	seedShippedDefencePools(t)
	sawKick := false
	for _, band := range testDefenceBands {
		out := ChannelDefenceResult{Defended: true, Defence: combatvocab.DefenceDodge, DefensiveCrit: band.crit, NormalizedDefenceMargin: band.margin}
		for i := 0; i < 20; i++ {
			triad := RenderChannelDefenceMessages(out, ChannelDefenceIdentities{
				Attacker: `<ansi fg="mobname">Kesh</ansi>`,
				Defender: `<ansi fg="username">Sil</ansi>`,
			}, "kick", i)
			got := shapesSpectator(TaggedMessage{Category: messaging.CategoryKick, Text: string(triad.ToRoom)})
			if hasWeaponWord(got) {
				t.Fatalf("band %+v index %d: a kick was called a weapon: %q", band, i, got)
			}
			sawKick = sawKick || strings.Contains(got, "kick")
		}
	}
	if !sawKick {
		t.Fatal("no shipped dodge line named the kick; the probe cannot fail")
	}
}

// (b) An unarmed strike that is dodged, read by the attacker at shapes: the
// attacker's own line says "strike", never "your weapon".
func TestShapesAttacker_DodgedUnarmedStrikeKeepsTheMoveName(t *testing.T) {
	seedShippedDefencePools(t)
	atk := characters.New()
	atk.Name = "Ordel"
	def := characters.New()
	def.Name = "Fold"
	best := bestDefenseResult{defenseType: combatvocab.DefenceDodge, defRoll: dice.RollResult{StdDev: 10}}

	sawStrike := false
	for _, band := range testDefenceBands {
		for trial := 0; trial < 60; trial++ {
			res := &AttackResult{}
			sendDefenseMessages(res, best, atk, def, false, false, band)
			hideIdentitiesInPersonalLines(res, atk, def, combatContext{sourceSight: messaging.SightShapes, targetSight: messaging.SightShapes})
			lines := append(append([]TaggedMessage{}, res.MessagesToSource...), res.MessagesToTarget...)
			for _, m := range res.MessagesToSourceRoom {
				lines = append(lines, TaggedMessage{Category: m.Category, Text: shapesSpectator(m)})
			}
			for _, m := range lines {
				if hasWeaponWord(m.Text) {
					t.Fatalf("band %+v: an unarmed strike was called a weapon: %q", band, m.Text)
				}
			}
			for _, m := range res.MessagesToSource {
				sawStrike = sawStrike || strings.Contains(m.Text, "strike")
			}
		}
	}
	if !sawStrike {
		t.Fatal("no attacker line named the strike; the probe cannot fail")
	}
}

// A body part is never an item: no shipped combat line may tag {bodypart} as
// one, or a shapes reader reads "catches your weapon" for "catches your arm".
func TestShippedCombatLinesNeverTagABodyPartAsAnItem(t *testing.T) {
	for _, dir := range []string{"combat-messages", "defense-messages"} {
		path := filepath.Join("..", "..", "_datafiles", "world", "dogmud", dir)
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 0 {
			t.Fatalf("%s is empty; the scan cannot fail", path)
		}
		for _, e := range entries {
			raw, err := os.ReadFile(filepath.Join(path, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(raw), `<ansi fg="item">{bodypart}</ansi>`); n > 0 {
				t.Errorf("%s/%s tags {bodypart} as an item %d times", dir, e.Name(), n)
			}
		}
	}
}

// (c) An armed swing that is dodged, at shapes: the defender and a spectator
// read "weapon", never the attacker's weapon's name. The attacker keeps it.
func TestShapes_DodgedArmedSwingHidesTheWeaponName(t *testing.T) {
	seedShippedDefencePools(t)
	atk := characters.New()
	atk.Name = "Ordel"
	atk.Equipment.Weapon = items.Item{ItemId: 999901, Spec: &items.ItemSpec{Name: "iron longsword"}}
	def := characters.New()
	def.Name = "Fold"
	best := bestDefenseResult{defenseType: combatvocab.DefenceDodge, defRoll: dice.RollResult{StdDev: 10}}

	sawWeapon, sawOwn := false, false
	for _, band := range testDefenceBands {
		for trial := 0; trial < 60; trial++ {
			res := &AttackResult{}
			sendDefenseMessages(res, best, atk, def, false, false, band)
			hideIdentitiesInPersonalLines(res, atk, def, combatContext{sourceSight: messaging.SightShapes, targetSight: messaging.SightShapes})
			readers := append([]TaggedMessage{}, res.MessagesToTarget...)
			for _, m := range res.MessagesToSourceRoom {
				readers = append(readers, TaggedMessage{Category: m.Category, Text: shapesSpectator(m)})
			}
			for _, m := range readers {
				if strings.Contains(m.Text, "longsword") {
					t.Fatalf("band %+v: a shapes reader learned the weapon: %q", band, m.Text)
				}
				sawWeapon = sawWeapon || hasWeaponWord(m.Text)
			}
			for _, m := range res.MessagesToSource {
				sawOwn = sawOwn || strings.Contains(m.Text, "iron longsword")
			}
		}
	}
	if !sawWeapon {
		t.Fatal("no shapes reader read \"weapon\"; the probe cannot fail")
	}
	if !sawOwn {
		t.Fatal("the attacker never read their own weapon's name")
	}
}
