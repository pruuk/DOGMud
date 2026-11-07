package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
)

// loadShippedQuestWorld loads the shipped data the quest loop sweeps read,
// skipping unless DOGMUD_BOOT_SMOKE is set.
func loadShippedQuestWorld(t *testing.T) {
	t.Helper()
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the quest loop sweep", bootSmokeEnvVar)
	}
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	func() {
		defer func() { _ = recover() }() // boot failures are the boot test's job
		loadAllDataFiles(false)
	}()
	if len(quests.GetAllQuests()) < 70 {
		t.Fatalf("only %d quests loaded; the sweep would pass vacuously", len(quests.GetAllQuests()))
	}
}

// TestSmoke_ShippedQuestsPassTheLoop runs the quest loop rules, the
// completability walkthrough and the destination rule over shipped content:
// any finding fails. Boot runs Validate and Destinations; only this sweep
// runs the walk.
func TestSmoke_ShippedQuestsPassTheLoop(t *testing.T) {
	loadShippedQuestWorld(t)

	w := questworld.Registry()
	fs := append(questloop.Validate(w), questengine.Walk(w, questworld.Sources())...)
	dest, inferred := questengine.Destinations(w.Quests)
	fs = append(fs, dest...)
	for _, s := range inferred {
		t.Logf("destination inferred: %s", s)
	}
	for _, f := range fs {
		t.Errorf("quest loop: %s", f.String())
	}
	t.Logf("quest loop: %d findings", len(fs))
}

// TestSmoke_WalkSourcesIndexShippedWorld checks the walkthrough's item and
// spawn index over shipped data, one known answer per source kind, and that
// an empty answer is possible.
func TestSmoke_WalkSourcesIndexShippedWorld(t *testing.T) {
	loadShippedQuestWorld(t)
	src := questworld.Sources()

	has := func(id int, prefix string) bool {
		for _, s := range src.ItemSources(id) {
			if strings.HasPrefix(s, prefix) {
				return true
			}
		}
		return false
	}

	// Caravan Guard Garve, spawned by rooms/pothole_coulee/5234.yaml.
	if !src.MobSpawned(9112) {
		t.Error("mob 9112 should be spawned")
	}
	// mobs/greenford/9502-maret_the_miller.yaml stocks 40153 (Maret is
	// spawned by rooms/greenford/6280.yaml).
	if !has(40153, "shop 9502") {
		t.Errorf("item 40153 should come from shop 9502: %v", src.ItemSources(40153))
	}
	// recipes/cooking/antidote-broth.yaml makes 30027.
	if !has(30027, "recipe antidote-broth") {
		t.Errorf("item 30027 should come from recipe antidote-broth: %v", src.ItemSources(30027))
	}
	// forager.ForageYields["forest"] lists 40067 (pine pitch).
	if !has(40067, "forage forest") {
		t.Errorf("item 40067 should come from forage forest: %v", src.ItemSources(40067))
	}
	if got := src.ItemSources(999999999); len(got) != 0 {
		t.Errorf("item 999999999 should have no source: %v", got)
	}
	// Ledger 21 comes only from Harn's lend_ledger topic
	// (dialogue/watchers_crossing/86.yaml), which opens on 5-start: it is
	// quest 5's own, open once 5-start is held. Tally 40100 comes only from
	// quest 63's own give_item actions (quests/63-dock_rat.yaml): it is 63's
	// own, and outside for any other quest.
	none := func(string) bool { return false }
	if src.ItemAvailable(21, 5, none) {
		t.Errorf("item 21 should be quest 5's own: %v", src.ItemSources(21))
	}
	if !src.ItemAvailable(21, 5, func(tok string) bool { return tok == "5-start" }) {
		t.Errorf("item 21 should be open on 5-start: %v", src.ItemSources(21))
	}
	if src.ItemAvailable(40100, 63, none) {
		t.Errorf("item 40100 should be quest 63's own: %v", src.ItemSources(40100))
	}
	if !src.ItemAvailable(40100, 5, none) {
		t.Error("item 40100 should count as outside quest 5")
	}
	if src.MobSpawned(999999999) {
		t.Error("mob 999999999 should not be spawned")
	}
	// spells/conviction-spike.yaml has effect_type damage, and it is a
	// starter spell (characters.StarterSpells).
	if got := src.SpellEffect("conviction-spike"); got != "damage" {
		t.Errorf("conviction-spike effect should be damage: %q", got)
	}
	if got := src.SpellEffect("no-such-spell"); got != "" {
		t.Errorf("an unknown spell should have no effect: %q", got)
	}
	if !slices.Contains(src.StarterSpells, "conviction-spike") {
		t.Errorf("conviction-spike should be a starter spell: %v", src.StarterSpells)
	}

	// External grants: Hadwen's rite tree grants 30-rite on topic begin or
	// rite_wait (behaviors/pothole_coulee/9100-cleric_hadwen.yaml), and the
	// tally stick grants 14-explore
	// (items/materials-40000/40035-tally_stick.yaml).
	// They live on the loop world (questworld.Registry), which boot, CI and
	// the quest editor all build.
	ext := map[string]questloop.ExternalGrant{}
	for _, g := range questworld.Registry().External {
		ext[g.Where] = g
		t.Logf("external: %s -> %s topics %v item %d", g.Where, g.Token, g.Topics, g.ItemId)
	}
	if g := ext["mob 9100 tree grant_quest 30-rite"]; g.Token != "30-rite" || g.MobId != 9100 ||
		!slices.Equal(g.Topics, []string{"begin", "rite_wait"}) {
		t.Errorf("Hadwen's 30-rite tree grant: %+v", g)
	}
	if g, ok := ext["room 407 tree grant_quest 4-investigate"]; ok {
		t.Errorf("room 407's tree no longer grants a step: %+v", g)
	}
	if g := ext["item 40035 questtoken"]; g.Token != "14-explore" || g.ItemId != 40035 {
		t.Errorf("item 40035 questtoken: %+v", g)
	}
}

// TestQuestDoor_RefusesAnyFinding: a finding panics at boot and is an error
// on a reload; no findings pass.
func TestQuestDoor_RefusesAnyFinding(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	one := func() []questloop.Finding {
		return []questloop.Finding{{Rule: "hint", QuestId: 1, Where: "step start", Msg: "no hint"}}
	}
	none := func() []questloop.Finding { return nil }
	if err := questDoorFrom(true, one); err == nil {
		t.Error("a reload must return an error")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("boot must panic")
			}
		}()
		_ = questDoorFrom(false, one)
	}()
	if err := questDoorFrom(false, none); err != nil {
		t.Errorf("no findings must not refuse boot: %v", err)
	}
}

// TestDestinationDoor_RefusesAnyFinding: the same refusal for a destination
// finding; inferred destinations alone pass.
func TestDestinationDoor_RefusesAnyFinding(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	one := func() ([]questloop.Finding, []string) {
		return []questloop.Finding{{Rule: "walk-destination", QuestId: 1, Where: "step a", Msg: "m"}}, nil
	}
	inferredOnly := func() ([]questloop.Finding, []string) { return nil, []string{"quest 1 step a"} }
	if err := destinationDoorFrom(true, one); err == nil {
		t.Error("a reload must return an error")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("boot must panic")
			}
		}()
		_ = destinationDoorFrom(false, one)
	}()
	if err := destinationDoorFrom(false, inferredOnly); err != nil {
		t.Errorf("inferred destinations must not refuse boot: %v", err)
	}
}
