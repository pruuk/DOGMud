package narration_test

// This file freezes what every message store emits TODAY, before the M1
// narration-unification refactor touches any of them. It is a NET, not a
// spec: a later refactor that silently changes what a player reads must make
// one of these goldens go red. Run with -update to regenerate after an
// INTENTIONAL content change; never run -update to make a red test green
// without first understanding why it went red.
//
// Store inventory verified against source 2026-09-07 (file counts; re-check
// before trusting these if this comment gets stale):
//   - _datafiles/world/dogmud/combat-messages/     20 files (weapon subtypes)
//   - _datafiles/world/dogmud/defense-messages/     9 files (defense types)
//   - _datafiles/world/dogmud/taunt-messages/        1 file  (rhetoric.yaml)
//   - _datafiles/world/dogmud/messaging/             1 file  (grapple_outcomes.yaml)
//   - _datafiles/world/dogmud/casting-messages.yaml  1 file  (bare file at tree root, 24 lines)
//   - _datafiles/world/dogmud/itemvoices/            2 files (sentient item voices)
// A shrinking file count against these numbers means a store was deleted or
// merged — the golden for that store will also shrink, so silence is not
// possible, but this comment is the first place to look for "why".
//
// Determinism: every render call below is fed a FRESH
// narration.SequencePicker() (or, for RenderDefenseMessage, which has no
// picker parameter at all, an equivalent indexOverride=0). A fresh
// SequencePicker always yields index 0 on its first call, so every tuple
// below captures "the first authored variant" for that exact
// (store, key, band, role[, tier]) combination — never util.Rand, so the
// goldens are stable across repeated runs and process restarts.
//
// Tokens (e.g. {target}, {itemname}) are substituted with fixed stand-ins
// (see substituteTokens) so a change to token *rendering* shows as a diff
// distinct from a change to the underlying prose.
//
// Dimensions swept per store (see the header written into each golden file
// for the authoritative, in-file record):
//   - combat-messages: subtype x intensity(8) x section(together always,
//     separate for generic+shooting only) x role x tier(beginner/expert/master).
//     PLUS a small "derived-selection" section exercising
//     SkillTieredMessages.GetForSkillLevelWith directly (the seam production
//     actually calls) at 3 representative skill levels, to freeze that its
//     index-0 pick is (and stays) Beginner[0] regardless of skill level.
//   - defense-messages: type x band(weak/normal/heavy) x role(todefender/
//     toattacker/toroom, all 3 from one coordinated call). PLUS the EMPTY
//     case (unregistered defense type -> empty triad).
//   - taunt-messages: intensity(4) x perspective(3). PLUS the EMPTY case
//     (unrecognized perspective string -> "").
//   - messaging (grapple): every advancement/degradation/reversal/escape/
//     hold key x role(controller/controlled/observers), every striking_apex
//     key (single-speaker), every gradient key x role(self/partner/
//     observers). PLUS the EMPTY-pool fallback case.
//   - casting-messages: 4 categories. PLUS the unknown-category fallback
//     case (NOT empty string — GetCastMessage always returns *something*;
//     the golden records the literal fallback sentence).
//   - itemvoices: voiceid x the full 8-event valid set. Single role, no band,
//     no tokens. Events a voice does not author render "" and that emptiness
//     is frozen too, so a pool silently disappearing shows as a line changing
//     from text to "". Added 2026-09-09: this store had no picker seam at all
//     (Line called util.Rand directly) and was therefore the one message store
//     with no golden. PLUS the unknown-event case.
//   - post-pipeline (messaging.RenderForRecipient): 3 representative sample
//     lines x SightDecision(full/shapes/none) x Channel(visual/audio). This
//     sweeps a REPRESENTATIVE subset of Category (~15 of the real ~60+;
//     categoryMax is unexported so the full enum cannot be walked from this
//     package) — explicitly a partial net, not a complete one. PLUS a
//     Verbosity.Suppresses(category) truth table over the same representative
//     categories x all 3 Verbosity tiers, since verbosity gating is a
//     delivery-affecting seam distinct from RenderForRecipient itself.
//
// NOT covered (state plainly, per the task's "honest partial net" rule):
//   - The full Category enum for post-pipeline (see above).
//   - Every (skill level) point for GetForSkillLevelWith — only 3
//     representative levels (10/50/90), on a single representative
//     subtype/intensity/role, not the full combat-messages cross product.
//   - WrapAnsi's own wrapping behavior in isolation: shouldWrap() returns
//     false for every Category today, so RenderForRecipient never reaches
//     the wrap stage in this snapshot. WrapAnsi has its own dedicated tests
//     (wrap_test.go) — out of scope for this net, which freezes the message
//     STORES and the delivery pipeline's stage sequencing, not the wrap
//     algorithm itself.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/grapplemessaging"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/itemvoices"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

var update = flag.Bool("update", false, "update golden snapshot files under testdata/stores")

// ---------------------------------------------------------------------
// Test setup
// ---------------------------------------------------------------------

func repoRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/narration/snapshot_test.go -> repo root is two levels up.
	return filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
}

func dogmudDataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud")
}

// setupRealStores points the engine's data-file config at the real DOGMud
// world data and loads the combat/defense and taunt stores through their
// real production loaders, mirroring the pattern established in
// internal/items/defensive_messages_integration_test.go.
func setupRealStores(t *testing.T) {
	t.Helper()
	mudlog.SetupLogger(nil, "", "", false)

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dogmudDataDir(t))
	// Buff 0 (Meditating) derives its TriggerCount from this at Validate time
	// and refuses 0; the shipped config.yaml says 3. Set it explicitly rather
	// than load config.yaml: that file is skip-worktree and differs per
	// machine, and a golden must not have a per-machine input. DataFiles and
	// LogoutRounds are the only config keys the three loaders read (verified
	// 2026-09-12); every other knob is a Go zero value here. The loaded buff,
	// spell and quest maps stay populated after this test; nothing else in
	// this package reads them.
	cfg.Network.LogoutRounds = 3
	configs.SetConfigForTest(t, cfg)

	items.LoadDataFiles()
	combat.LoadTauntMessageFiles()
	itemvoices.LoadDataFiles()
	spells.LoadCastingMessages()

	// Kind B stores (M3 item 5b). Their loaders read the same configured data
	// path, so the golden sees exactly what a booted dogmud world sees.
	buffs.LoadDataFiles()
	spells.LoadSpellFiles()
	quests.LoadDataFiles()
}

// ---------------------------------------------------------------------
// Golden file plumbing
// ---------------------------------------------------------------------

func goldenPath(t *testing.T, name string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(here), "testdata", "stores", name)
}

func checkGolden(t *testing.T, name string, got string) {
	t.Helper()
	path := goldenPath(t, name)

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata/stores: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run `go test ./internal/narration/... -run TestSnapshotStores -update` to create it)", path, err)
	}
	want := string(wantBytes)
	if want != got {
		t.Errorf("golden mismatch for store %q (%s)\n%s\nRun with -update ONLY after confirming the change is intentional.",
			name, path, firstDiffLine(want, got))
	}
}

// firstDiffLine reports the first line at which want and got diverge, for a
// readable failure message instead of dumping two enormous strings.
func firstDiffLine(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	max := len(wl)
	if len(gl) > max {
		max = len(gl)
	}
	for i := 0; i < max; i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("first differing line (%d):\n  want: %s\n  got:  %s", i+1, w, g)
		}
	}
	if len(wl) != len(gl) {
		return fmt.Sprintf("line count differs: want %d, got %d", len(wl), len(gl))
	}
	return "(strings differ but no line-level diff found — check trailing whitespace)"
}

// ---------------------------------------------------------------------
// Token substitution — fixed stand-ins so token-rendering changes show as a
// diff distinct from prose changes.
// ---------------------------------------------------------------------

var tokenStandins = map[items.TokenName]string{
	items.TokenItemName:     "Weapon",
	items.TokenSource:       "Source",
	items.TokenSourceType:   "User",
	items.TokenTarget:       "Target",
	items.TokenTargetType:   "Mob",
	items.TokenUsesLeft:     "3",
	items.TokenDamage:       "ModerateWounds",
	items.TokenEntranceName: "South",
	items.TokenExitName:     "North",
	items.TokenDefender:     "Defender",
	items.TokenAttacker:     "Attacker",
	items.TokenWeapon:       "Weapon",
	items.TokenAttack:       "Strike",
	items.TokenStance:       "Balanced",
	items.TokenPosition:     "Standing",
	items.TokenMomentum:     "InControl",
	items.TokenBodyPart:     "Arm",
}

func substituteTokens(s string) string {
	for tok, val := range tokenStandins {
		s = strings.ReplaceAll(s, string(tok), val)
	}
	return s
}

// ---------------------------------------------------------------------
// Directory listing helper (subtype/defense-type key discovery)
// ---------------------------------------------------------------------

func yamlKeysInDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var keys []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		keys = append(keys, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	sort.Strings(keys)
	return keys
}

// ---------------------------------------------------------------------
// Store 1: combat-messages (internal/items — attack messages)
// ---------------------------------------------------------------------

func buildCombatMessagesGolden(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(dogmudDataDir(t), "combat-messages")
	subtypes := yamlKeysInDir(t, dir)

	var b strings.Builder
	fmt.Fprintf(&b, "# combat-messages store snapshot\n")
	fmt.Fprintf(&b, "# subtype files at time of writing: %d\n", len(subtypes))
	fmt.Fprintf(&b, "# subtypes: %s\n", strings.Join(subtypes, ","))
	fmt.Fprintf(&b, "# dimensions: subtype x intensity x section x role x tier, fresh SequencePicker per tuple (always index 0)\n")
	fmt.Fprintf(&b, "# separate section present only for: generic, shooting (per source at time of writing)\n\n")

	intensities := []items.Intensity{items.Prepare, items.Wait, items.Miss, items.Weak, items.Normal, items.Heavy, items.Critical, items.Fumble}
	tiers := []struct {
		name string
		get  func(items.SkillTieredMessages) items.MessageOptions
	}{
		{"beginner", func(s items.SkillTieredMessages) items.MessageOptions { return s.Beginner }},
		{"expert", func(s items.SkillTieredMessages) items.MessageOptions { return s.Expert }},
		{"master", func(s items.SkillTieredMessages) items.MessageOptions { return s.Master }},
	}
	togetherRoles := []struct {
		name string
		get  func(items.TogetherMessages) items.SkillTieredMessages
	}{
		{"toattacker", func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToAttacker }},
		{"todefender", func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToDefender }},
		{"toroom", func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToRoom }},
	}
	separateRoles := []struct {
		name string
		get  func(items.SeparateMessages) items.SkillTieredMessages
	}{
		{"toattacker", func(m items.SeparateMessages) items.SkillTieredMessages { return m.ToAttacker }},
		{"todefender", func(m items.SeparateMessages) items.SkillTieredMessages { return m.ToDefender }},
		{"toattackerroom", func(m items.SeparateMessages) items.SkillTieredMessages { return m.ToAttackerRoom }},
		{"todefenderroom", func(m items.SeparateMessages) items.SkillTieredMessages { return m.ToDefenderRoom }},
	}

	hasSeparate := map[string]bool{"generic": true, "shooting": true}

	for _, subtype := range subtypes {
		for _, intensity := range intensities {
			opts := items.GetPreAttackMessage(items.ItemSubType(subtype), intensity)

			for _, role := range togetherRoles {
				stm := role.get(opts.Together)
				for _, tier := range tiers {
					mo := tier.get(stm)
					text := substituteTokens(string(mo.GetWith(narration.SequencePicker())))
					fmt.Fprintf(&b, "%s|%s|together|%s|%s => %s\n", subtype, intensity, role.name, tier.name, text)
				}
			}

			if hasSeparate[subtype] {
				for _, role := range separateRoles {
					stm := role.get(opts.Separate)
					for _, tier := range tiers {
						mo := tier.get(stm)
						text := substituteTokens(string(mo.GetWith(narration.SequencePicker())))
						fmt.Fprintf(&b, "%s|%s|separate|%s|%s => %s\n", subtype, intensity, role.name, tier.name, text)
					}
				}
			}
		}
	}

	// Derived-selection sanity check: freeze that GetForSkillLevelWith's
	// index-0 pick under a fresh picker is Beginner[0] regardless of skill
	// level, on one representative subtype/intensity/role. This is the seam
	// production actually calls for the core combat loop.
	fmt.Fprintf(&b, "\n# derived-selection (GetForSkillLevelWith), bite/weak/toattacker, fresh picker per call\n")
	opts := items.GetPreAttackMessage(items.Bite, items.Weak)
	for _, skillLevel := range []int{10, 50, 90} {
		text := substituteTokens(string(opts.Together.ToAttacker.GetForSkillLevelWith(narration.SequencePicker(), skillLevel)))
		fmt.Fprintf(&b, "derived|bite|weak|toattacker|skill=%d => %s\n", skillLevel, text)
	}

	return b.String()
}

// ---------------------------------------------------------------------
// Store 2: defense-messages (internal/items — RenderDefenseMessage)
// ---------------------------------------------------------------------

func buildDefenseMessagesGolden(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(dogmudDataDir(t), "defense-messages")
	types := yamlKeysInDir(t, dir)

	var b strings.Builder
	fmt.Fprintf(&b, "# defense-messages store snapshot\n")
	fmt.Fprintf(&b, "# defense-type files at time of writing: %d\n", len(types))
	fmt.Fprintf(&b, "# defense-types: %s\n", strings.Join(types, ","))
	fmt.Fprintf(&b, "# dimensions: defense-type x band(weak/normal/heavy) -- all 3 roles (todefender/toattacker/toroom)\n")
	fmt.Fprintf(&b, "# come from ONE coordinated RenderDefenseMessage call (same index across the triad).\n")
	fmt.Fprintf(&b, "# RenderDefenseMessage has no picker param (util.Rand only) -- pinned via indexOverride=0,\n")
	fmt.Fprintf(&b, "# the fresh-SequencePicker-first-pick equivalent for this seam.\n\n")
	fmt.Fprintf(&b, "# ALSO covers the MELEE seam (internal/combat sendDefenseMessages), which does its own\n")
	fmt.Fprintf(&b, "# zScore banding via items.GetDefenseMessage and used to pick each of the 3 roles with an\n")
	fmt.Fprintf(&b, "# INDEPENDENT MessageOptions.Get() call -- three unrelated random indices describing three\n")
	fmt.Fprintf(&b, "# different events. That bug shipped invisibly because this snapshot only ever exercised\n")
	fmt.Fprintf(&b, "# RenderDefenseMessage, never GetDefenseMessage. The melee| rows below freeze\n")
	fmt.Fprintf(&b, "# GetDefenseMessage(...).RenderTriad(...) (a fresh SequencePicker per tuple, so index 0 every\n")
	fmt.Fprintf(&b, "# time) so a regression back to three independent picks shows up here again.\n\n")

	bands := []struct {
		name   string
		crit   bool
		margin float64
	}{
		{"weak", false, 0.0},
		{"normal", false, 0.6},
		{"heavy", true, 0.0},
	}

	for _, dt := range types {
		for _, band := range bands {
			triad := items.RenderDefenseMessage(items.DefenseType(dt), band.crit, band.margin, tokenStandins, 0)
			fmt.Fprintf(&b, "%s|%s|todefender => %s\n", dt, band.name, substituteTokens(string(triad.ToDefender)))
			fmt.Fprintf(&b, "%s|%s|toattacker => %s\n", dt, band.name, substituteTokens(string(triad.ToAttacker)))
			fmt.Fprintf(&b, "%s|%s|toroom => %s\n", dt, band.name, substituteTokens(string(triad.ToRoom)))
		}
	}

	// EMPTY CASE: an unregistered defense type returns an all-empty triad.
	fmt.Fprintf(&b, "\n# EMPTY CASE: unregistered defense type -> empty triad\n")
	emptyTriad := items.RenderDefenseMessage(items.DefenseType("nonexistent-defense-type"), false, 0.6, tokenStandins, 0)
	fmt.Fprintf(&b, "nonexistent-defense-type|normal|todefender => %q\n", string(emptyTriad.ToDefender))
	fmt.Fprintf(&b, "nonexistent-defense-type|normal|toattacker => %q\n", string(emptyTriad.ToAttacker))
	fmt.Fprintf(&b, "nonexistent-defense-type|normal|toroom => %q\n", string(emptyTriad.ToRoom))

	// MELEE SEAM: GetDefenseMessage's own zScore banding (>=2.0 heavy, >=0.5
	// normal, else weak -- see internal/combat/combat_helpers.go), feeding the
	// same RenderTriad coordination step. A fresh SequencePicker per tuple
	// always yields index 0 on its first call, same convention as the rest of
	// this file.
	fmt.Fprintf(&b, "\n# MELEE SEAM: items.GetDefenseMessage(type, zScore).RenderTriad(...), fresh SequencePicker per tuple\n")
	meleeBands := []struct {
		name   string
		zScore float64
	}{
		{"weak", 0.0},
		{"normal", 0.6},
		{"heavy", 2.5},
	}
	for _, dt := range types {
		for _, band := range meleeBands {
			options := items.GetDefenseMessage(items.DefenseType(dt), band.zScore)
			triad := options.RenderTriad(tokenStandins, narration.SequencePicker())
			fmt.Fprintf(&b, "melee|%s|%s|todefender => %s\n", dt, band.name, substituteTokens(string(triad.ToDefender)))
			fmt.Fprintf(&b, "melee|%s|%s|toattacker => %s\n", dt, band.name, substituteTokens(string(triad.ToAttacker)))
			fmt.Fprintf(&b, "melee|%s|%s|toroom => %s\n", dt, band.name, substituteTokens(string(triad.ToRoom)))
		}
	}

	// EMPTY CASE (melee seam): an unregistered defense type.
	fmt.Fprintf(&b, "\n# EMPTY CASE (melee seam): unregistered defense type -> empty triad\n")
	emptyMeleeTriad := items.GetDefenseMessage(items.DefenseType("nonexistent-defense-type"), 0.6).RenderTriad(tokenStandins, narration.SequencePicker())
	fmt.Fprintf(&b, "melee|nonexistent-defense-type|normal|todefender => %q\n", string(emptyMeleeTriad.ToDefender))
	fmt.Fprintf(&b, "melee|nonexistent-defense-type|normal|toattacker => %q\n", string(emptyMeleeTriad.ToAttacker))
	fmt.Fprintf(&b, "melee|nonexistent-defense-type|normal|toroom => %q\n", string(emptyMeleeTriad.ToRoom))

	return b.String()
}

// ---------------------------------------------------------------------
// Store 3: taunt-messages (internal/combat — GetTauntMessage)
// ---------------------------------------------------------------------

func buildTauntMessagesGolden(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(dogmudDataDir(t), "taunt-messages")
	files := yamlKeysInDir(t, dir)

	var b strings.Builder
	fmt.Fprintf(&b, "# taunt-messages store snapshot\n")
	fmt.Fprintf(&b, "# files at time of writing: %d (%s)\n", len(files), strings.Join(files, ","))
	fmt.Fprintf(&b, "# dimensions: intensity(hit/miss/critical/fumble) x perspective(toattacker/todefender/toroom)\n")
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# All three perspectives of one intensity come from ONE coordinated GetTauntTriad call\n")
	fmt.Fprintf(&b, "# (same variant index across the triad). Until 2026-09-09 usercommands/taunt.go called a\n")
	fmt.Fprintf(&b, "# per-perspective getter three times and got three INDEPENDENT indices, so the three\n")
	fmt.Fprintf(&b, "# audiences were narrated three different moments -- the same defect PR #112 fixed for\n")
	fmt.Fprintf(&b, "# melee defence. A fresh SequencePicker per intensity pins index 0.\n")
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# This snapshot pins index 0 only, so it CANNOT see a regression back to independent\n")
	fmt.Fprintf(&b, "# picks (all three would still read index 0). That is guarded by\n")
	fmt.Fprintf(&b, "# TestGetTauntTriadUsesOneCoordinatedIndex in internal/combat, which shares one\n")
	fmt.Fprintf(&b, "# SequencePicker across the triad so three picks yield 0/1/2 and one pick yields 0/0/0.\n\n")

	intensities := []combat.TauntIntensity{combat.TauntHit, combat.TauntMiss, combat.TauntCritical, combat.TauntFumble}

	for _, intensity := range intensities {
		triad := combat.GetTauntTriad(intensity, "Source", "Target", "User", "Mob", "ModerateWounds", narration.SequencePicker())
		fmt.Fprintf(&b, "rhetoric|%s|toattacker => %s\n", intensity, triad.ToAttacker)
		fmt.Fprintf(&b, "rhetoric|%s|todefender => %s\n", intensity, triad.ToDefender)
		fmt.Fprintf(&b, "rhetoric|%s|toroom => %s\n", intensity, triad.ToRoom)
	}

	return b.String()
}

// ---------------------------------------------------------------------
// Store 4: messaging/grapple_outcomes.yaml (internal/grapplemessaging)
// ---------------------------------------------------------------------

func sortedKeysTriad(m map[string]grapplemessaging.TemplateTriad) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysStrSlice(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysGradient(m map[string]grapplemessaging.GradientTriad) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildGrappleMessagingGolden(t *testing.T) string {
	t.Helper()
	path := filepath.Join(dogmudDataDir(t), "messaging", "grapple_outcomes.yaml")
	lib, err := grapplemessaging.Load(path)
	if err != nil {
		t.Fatalf("grapplemessaging.Load(%s): %v", path, err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# messaging/grapple_outcomes.yaml store snapshot\n")
	fmt.Fprintf(&b, "# category counts at time of writing: advancements=%d degradations=%d reversals=%d escapes=%d holds=%d striking_apex=%d gradients=%d\n",
		len(lib.Advancements), len(lib.Degradations), len(lib.Reversals), len(lib.Escapes), len(lib.Holds), len(lib.StrikingApex), len(lib.Gradients))
	fmt.Fprintf(&b, "# dimensions: category x key x role. Triad categories (advancements/degradations/reversals/escapes/holds)\n")
	fmt.Fprintf(&b, "# use role in {controller,controlled,observers}; striking_apex is single-speaker (no role); gradients use\n")
	fmt.Fprintf(&b, "# role in {self,partner,observers}. Each tuple: fresh cooldowns map + fresh SequencePicker via PickTemplate,\n")
	fmt.Fprintf(&b, "# then RenderTemplate with fixed stand-ins Controller/Controlled.\n\n")

	renderPool := func(pool []string) string {
		tmpl := grapplemessaging.PickTemplate(pool, map[string]bool{}, "snapshot", narration.SequencePicker())
		return grapplemessaging.RenderTemplate(tmpl, "Controller", "Controlled")
	}

	triadCategories := []struct {
		name string
		m    map[string]grapplemessaging.TemplateTriad
	}{
		{"advancements", lib.Advancements},
		{"degradations", lib.Degradations},
		{"reversals", lib.Reversals},
		{"escapes", lib.Escapes},
		{"holds", lib.Holds},
	}
	for _, cat := range triadCategories {
		for _, key := range sortedKeysTriad(cat.m) {
			triad := cat.m[key]
			fmt.Fprintf(&b, "%s|%s|controller => %s\n", cat.name, key, renderPool(triad.Controller))
			fmt.Fprintf(&b, "%s|%s|controlled => %s\n", cat.name, key, renderPool(triad.Controlled))
			fmt.Fprintf(&b, "%s|%s|observers => %s\n", cat.name, key, renderPool(triad.Observers))
		}
	}

	for _, key := range sortedKeysStrSlice(lib.StrikingApex) {
		fmt.Fprintf(&b, "striking_apex|%s|(single-speaker) => %s\n", key, renderPool(lib.StrikingApex[key]))
	}

	for _, key := range sortedKeysGradient(lib.Gradients) {
		g := lib.Gradients[key]
		fmt.Fprintf(&b, "gradients|%s|self => %s\n", key, renderPool(g.Self))
		fmt.Fprintf(&b, "gradients|%s|partner => %s\n", key, renderPool(g.Partner))
		fmt.Fprintf(&b, "gradients|%s|observers => %s\n", key, renderPool(g.Observers))
	}

	// EMPTY CASE: PickTemplate on an empty pool returns a benign fallback
	// sentinel (never an actual empty string).
	fmt.Fprintf(&b, "\n# EMPTY CASE: PickTemplate on a nil pool -> fallback sentinel, not \"\"\n")
	fmt.Fprintf(&b, "(nil-pool)|n/a|n/a => %q\n", grapplemessaging.PickTemplate(nil, map[string]bool{}, "snapshot-empty", narration.SequencePicker()))

	return b.String()
}

// ---------------------------------------------------------------------
// Store 5: casting-messages.yaml (internal/spells — GetCastMessage)
// ---------------------------------------------------------------------

func buildCastingMessagesGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# casting-messages.yaml store snapshot\n")
	fmt.Fprintf(&b, "# bare file at tree root (not a directory), 24 lines at time of writing\n")
	fmt.Fprintf(&b, "# GetCastMessage used stdlib math/rand directly until 2026-09-07 (unreachable by any seam); this\n")
	fmt.Fprintf(&b, "# task's Task 2 predecessor added the trailing picker param used here.\n")
	fmt.Fprintf(&b, "# dimensions: category (already_casting/cast_started/cast_continuing/concentration_slipped)\n\n")

	categories := []string{"already_casting", "cast_started", "cast_continuing", "concentration_slipped"}
	for _, cat := range categories {
		text := spells.GetCastMessage(cat, "Testspell", narration.SequencePicker())
		fmt.Fprintf(&b, "casting|%s => %s\n", cat, text)
	}

	// FALLBACK CASE (not the empty string): an unrecognized category yields
	// an empty pool, and GetCastMessage's own fallback fires.
	fmt.Fprintf(&b, "\n# FALLBACK CASE: unrecognized category -> \"Something stirs with <spell>.\" (not \"\")\n")
	fallback := spells.GetCastMessage("bogus-category", "Testspell", narration.SequencePicker())
	fmt.Fprintf(&b, "casting|bogus-category => %s\n", fallback)

	return b.String()
}

// ---------------------------------------------------------------------
// Store 6 (post-pipeline): internal/messaging.RenderForRecipient +
// Verbosity.Suppresses
// ---------------------------------------------------------------------

func buildPostPipelineGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# post-pipeline snapshot: internal/messaging.RenderForRecipient + Verbosity.Suppresses\n")
	fmt.Fprintf(&b, "# This catches the class a raw-store snapshot cannot see: the text survived unchanged but\n")
	fmt.Fprintf(&b, "# WHO receives it, or what a blinded/deafened player sees, quietly changed.\n")
	fmt.Fprintf(&b, "# PARTIAL NET: sweeps a representative subset of Category (~15 of the real ~60+ — categoryMax\n")
	fmt.Fprintf(&b, "# is unexported so the full enum cannot be walked from outside internal/messaging).\n")
	fmt.Fprintf(&b, "# dimensions (RenderForRecipient section): sample-line x SightDecision(full/shapes/none) x\n")
	fmt.Fprintf(&b, "# Channel(visual/audio). dimensions (Verbosity section): representative-category x\n")
	fmt.Fprintf(&b, "# Verbosity(full/medium/light) -> bool suppressed.\n\n")

	samples := []struct {
		name string
		cat  messaging.Category
		text string
	}{
		{"hit-melee-with-name-tags", messaging.CategoryHitMelee, `<ansi fg="username">Target</ansi> is struck hard by <ansi fg="username">Source</ansi>.`},
		{"room-description", messaging.CategoryRoomDescription, "The cave is dark and damp, water dripping steadily from unseen cracks."},
		{"dodge", messaging.CategoryDodge, "you dodge the attack"},
	}
	sightDecisions := []struct {
		name string
		sd   messaging.SightDecision
	}{
		{"full", messaging.SightFull},
		{"shapes", messaging.SightShapes},
		{"none", messaging.SightNone},
	}
	channels := []struct {
		name string
		ch   messaging.Channel
	}{
		{"visual", messaging.ChannelVisual},
		{"audio", messaging.ChannelAudio},
	}

	for _, sample := range samples {
		for _, ch := range channels {
			for _, sd := range sightDecisions {
				out := messaging.RenderForRecipient(messaging.RenderInput{
					Category:      sample.cat,
					Text:          sample.text,
					Channel:       ch.ch,
					SightDecision: sd.sd,
					LineWidth:     80,
				})
				fmt.Fprintf(&b, "render|%s|channel=%s|sight=%s => %q\n", sample.name, ch.name, sd.name, out)
			}
		}
	}

	fmt.Fprintf(&b, "\n# Verbosity.Suppresses truth table (representative categories x all 3 tiers)\n")
	verbosities := []struct {
		name string
		v    messaging.Verbosity
	}{
		{"full", messaging.VerbosityFull},
		{"medium", messaging.VerbosityMedium},
		{"light", messaging.VerbosityLight},
	}
	repCategories := []struct {
		name string
		cat  messaging.Category
	}{
		{"default", messaging.CategoryDefault},
		{"dodge", messaging.CategoryDodge},
		{"parry", messaging.CategoryParry},
		{"block", messaging.CategoryBlock},
		{"hit-melee", messaging.CategoryHitMelee},
		{"hit-blunt", messaging.CategoryHitBlunt},
		{"hit-natural-sharp", messaging.CategoryHitNaturalSharp},
		{"hit-ranged", messaging.CategoryHitRanged},
		{"hit-caster", messaging.CategoryHitCaster},
		{"hit-unarmed", messaging.CategoryHitUnarmed},
		{"speech", messaging.CategorySpeech},
		{"spell-fold", messaging.CategorySpellFold},
		{"room-description", messaging.CategoryRoomDescription},
		{"loot", messaging.CategoryLoot},
		{"toxin", messaging.CategoryToxin},
	}
	for _, v := range verbosities {
		for _, cat := range repCategories {
			suppressed := v.v.Suppresses(cat.cat)
			fmt.Fprintf(&b, "verbosity|%s|%s => suppressed=%t\n", v.name, cat.name, suppressed)
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------
// The test
// ---------------------------------------------------------------------

func TestSnapshotStores(t *testing.T) {
	setupRealStores(t)

	t.Run("combat_messages", func(t *testing.T) {
		checkGolden(t, "combat_messages.golden", buildCombatMessagesGolden(t))
	})
	t.Run("defense_messages", func(t *testing.T) {
		checkGolden(t, "defense_messages.golden", buildDefenseMessagesGolden(t))
	})
	t.Run("taunt_messages", func(t *testing.T) {
		checkGolden(t, "taunt_messages.golden", buildTauntMessagesGolden(t))
	})
	t.Run("messaging_grapple", func(t *testing.T) {
		checkGolden(t, "messaging_grapple.golden", buildGrappleMessagingGolden(t))
	})
	t.Run("casting_messages", func(t *testing.T) {
		checkGolden(t, "casting_messages.golden", buildCastingMessagesGolden(t))
	})
	t.Run("itemvoices", func(t *testing.T) {
		checkGolden(t, "itemvoices.golden", buildItemVoicesGolden(t))
	})
	t.Run("buffs", func(t *testing.T) {
		checkGolden(t, "buffs.golden", buildBuffsGolden(t))
	})
	t.Run("spells", func(t *testing.T) {
		checkGolden(t, "spells.golden", buildSpellsGolden(t))
	})
	t.Run("quests", func(t *testing.T) {
		checkGolden(t, "quests.golden", buildQuestsGolden(t))
	})
	t.Run("post_pipeline", func(t *testing.T) {
		checkGolden(t, "post_pipeline.golden", buildPostPipelineGolden(t))
	})
}

// ---------------------------------------------------------------------
// Store 6: itemvoices (internal/itemvoices)
// ---------------------------------------------------------------------

func buildItemVoicesGolden(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(dogmudDataDir(t), "itemvoices")
	files := yamlKeysInDir(t, dir)

	var b strings.Builder
	fmt.Fprintf(&b, "# itemvoices store snapshot\n")
	fmt.Fprintf(&b, "# voice files at time of writing: %d (%s)\n", len(files), strings.Join(files, ","))
	fmt.Fprintf(&b, "# dimensions: voiceid x event. Single role (the item speaks), no band, no tokens.\n")
	fmt.Fprintf(&b, "#\n")
	fmt.Fprintf(&b, "# This store had NO picker seam and NO golden until 2026-09-09: VoiceSpec.Line\n")
	fmt.Fprintf(&b, "# called util.Rand directly, so nothing could pin its output. LineWith was added\n")
	fmt.Fprintf(&b, "# FIRST, so this golden is a baseline of PRE-migration behaviour rather than a\n")
	fmt.Fprintf(&b, "# record of whatever the M3 migration produced. A fresh SequencePicker per tuple\n")
	fmt.Fprintf(&b, "# pins index 0.\n\n")

	// The full valid event set, from itemvoices.validVoiceEvents. Events a
	// voice does not author render empty, and that emptiness is itself frozen:
	// a voice silently losing an event pool shows up here as a line changing
	// from text to "".
	events := []string{
		"on_equip", "on_unequip", "on_kill", "on_idle",
		"on_hunger_warning", "on_hunger_feeding", "on_taunt", "on_grudge",
	}

	ids := itemvoices.AllVoiceIds()
	if len(ids) == 0 {
		t.Fatal("no voices loaded; setupRealStores must call itemvoices.LoadDataFiles()")
	}

	for _, id := range ids {
		v := itemvoices.GetVoice(id)
		if v == nil {
			t.Fatalf("voice %q listed by AllVoiceIds but GetVoice returned nil", id)
		}
		for _, event := range events {
			fmt.Fprintf(&b, "%s|%s => %q\n", id, event, v.LineWith(narration.SequencePicker(), event))
		}
	}

	// EMPTY CASE: an event outside the valid set entirely.
	fmt.Fprintf(&b, "\n# EMPTY CASE: unknown event -> \"\"\n")
	first := itemvoices.GetVoice(ids[0])
	fmt.Fprintf(&b, "%s|bogus-event => %q\n", ids[0], first.LineWith(narration.SequencePicker(), "bogus-event"))

	return b.String()
}

// ---------------------------------------------------------------------
// Kind B stores (M3 item 5b): buffs, spells, quests. Single strings per
// lifecycle phase, no pool, so no picker is involved: the golden freezes the
// substitution and the notice logic, keyed by the AUTHORED key name so a
// swapped role shows up as a changed row.
// ---------------------------------------------------------------------

// kindBSource is the stand-in name set. The source is a player and the target
// a mob, so the two tags differ and a swap of {source} for {target} would
// change the golden.
var kindBSource = textutil.TokenContext{
	SourceName:      `<ansi fg="username">Aliceia</ansi>`,
	SourcePlainName: "Aliceia",
	TargetName:      `<ansi fg="mobname">Targetticus</ansi>`,
	TargetPlainName: "Targetticus",
}

// kindBNoTarget is the same source with no target, which is how every buff
// site and the quest bridge render: they never know a target.
var kindBNoTarget = textutil.TokenContext{
	SourceName:      kindBSource.SourceName,
	SourcePlainName: kindBSource.SourcePlainName,
}

// Store 8: buffs (internal/buffs, six *_user_text / *_room_text fields)
func buildBuffsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# buffs store snapshot (internal/buffs)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code. The *_user_text rows record what the\n")
	fmt.Fprintf(&b, "# HOLDER is sent: for start and end that is StartUserNotice / EndUserNotice (authored\n")
	fmt.Fprintf(&b, "# line, else the generic fallback, else nothing for a secret buff). A row exists only\n")
	fmt.Fprintf(&b, "# when the sent line is non-empty. authored_start_line is the raw start_user_text of a\n")
	fmt.Fprintf(&b, "# silent-start buff, recorded for every silent-start buff whether or not a site sends it\n")
	fmt.Fprintf(&b, "# today: sleep (15), arrest (88), stun (84) and broken limb (83) have a sender; throttled\n")
	fmt.Fprintf(&b, "# (89) does not, its move narrates the choke itself.\n")
	fmt.Fprintf(&b, "# dimensions: buff id x authored key; source only, buffs never know a target\n\n")

	ids := buffs.GetAllBuffIds()
	sort.Ints(ids)
	if len(ids) == 0 {
		t.Fatal("no buffs loaded; setupRealStores must call buffs.LoadDataFiles()")
	}
	for _, id := range ids {
		spec := buffs.GetBuffSpec(id)
		if spec == nil {
			t.Fatalf("buff %d has no spec", id)
		}
		rows := []struct{ key, text string }{
			{"start_user_text", spec.StartUserNotice()},
			{"start_room_text", spec.StartRoomText},
			{"trigger_user_text", spec.TriggerUserText},
			{"trigger_room_text", spec.TriggerRoomText},
			{"end_user_text", spec.EndUserNotice()},
			{"end_room_text", spec.EndRoomText},
		}
		for _, r := range rows {
			if line := textutil.SubstituteTokens(r.text, kindBNoTarget); line != "" {
				fmt.Fprintf(&b, "buff|%d|%s => %s\n", id, r.key, line)
			}
		}
		if slices.Contains(spec.Flags, buffs.SilentStart) && spec.StartUserText != "" {
			fmt.Fprintf(&b, "buff|%d|authored_start_line => %s\n", id, spec.StartUserText)
		}
	}
	return b.String()
}

// Store 9: spells (internal/spells, six cast/wait/magic x user/room fields)
func buildSpellsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# spells store snapshot (internal/spells)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code: textutil.SubstituteTokens over each raw\n")
	fmt.Fprintf(&b, "# field with a source AND a target. A |notarget row follows any line whose rendering\n")
	fmt.Fprintf(&b, "# changes when the target is absent, freezing the empty substitution.\n")
	fmt.Fprintf(&b, "# probe rows freeze the token contract itself; they are not authored content\n")
	fmt.Fprintf(&b, "# dimensions: spell id x authored key [x notarget]\n\n")

	// Probe rows: no shipped line carries {target_plain} or an unknown token,
	// so this fixed string freezes the whole substitution contract, including
	// passthrough of an unknown token and the empty target.
	const probe = "{source} and {source_plain} at {target} and {target_plain}; {unknown} stays; {source} again"
	fmt.Fprintf(&b, "probe|all_tokens => %s\n", textutil.SubstituteTokens(probe, kindBSource))
	fmt.Fprintf(&b, "probe|all_tokens|notarget => %s\n\n", textutil.SubstituteTokens(probe, kindBNoTarget))

	all := spells.GetAllSpells()
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		t.Fatal("no spells loaded; setupRealStores must call spells.LoadSpellFiles()")
	}
	for _, id := range ids {
		s := all[id]
		rows := []struct{ key, text string }{
			{"cast_user_text", s.CastUserText},
			{"cast_room_text", s.CastRoomText},
			{"wait_user_text", s.WaitUserText},
			{"wait_room_text", s.WaitRoomText},
			{"magic_user_text", s.MagicUserText},
			{"magic_room_text", s.MagicRoomText},
		}
		for _, r := range rows {
			line := textutil.SubstituteTokens(r.text, kindBSource)
			if line == "" {
				continue
			}
			fmt.Fprintf(&b, "spell|%s|%s => %s\n", id, r.key, line)
			if noTarget := textutil.SubstituteTokens(r.text, kindBNoTarget); noTarget != line {
				fmt.Fprintf(&b, "spell|%s|%s|notarget => %s\n", id, r.key, noTarget)
			}
		}
	}
	return b.String()
}

// Store 10: quests (internal/quests: reward playermessage/roommessage, and the
// send_text / room_text actions, nested sequences included)
func buildQuestsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# quests store snapshot (internal/quests)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code, sending what each site sends today:\n")
	fmt.Fprintf(&b, "# rewards playermessage/roommessage and action send_text RAW (no substitution),\n")
	fmt.Fprintf(&b, "# action room_text through textutil.SubstituteTokens with the player as {source}.\n")
	fmt.Fprintf(&b, "# dimensions: quest id x rewards | trigger<i>|action<j>[|sequence|action<k>...] x key\n\n")

	all := quests.GetAllQuests()
	sort.Slice(all, func(i, j int) bool { return all[i].QuestId < all[j].QuestId })
	if len(all) == 0 {
		t.Fatal("no quests loaded; setupRealStores must call quests.LoadDataFiles()")
	}

	var walk func(where string, actions []quests.ActionDef)
	walk = func(where string, actions []quests.ActionDef) {
		for j, a := range actions {
			aw := fmt.Sprintf("%s|action%d", where, j)
			if a.SendText != "" {
				fmt.Fprintf(&b, "%s|send_text => %s\n", aw, a.SendText)
			}
			if a.RoomText != "" {
				fmt.Fprintf(&b, "%s|room_text => %s\n", aw, textutil.SubstituteTokens(a.RoomText, kindBNoTarget))
			}
			if a.Sequence != nil {
				walk(aw+"|sequence", a.Sequence.OnComplete)
			}
		}
	}
	for _, q := range all {
		if q.Rewards.PlayerMessage != "" {
			fmt.Fprintf(&b, "quest|%d|rewards|playermessage => %s\n", q.QuestId, q.Rewards.PlayerMessage)
		}
		if q.Rewards.RoomMessage != "" {
			fmt.Fprintf(&b, "quest|%d|rewards|roommessage => %s\n", q.QuestId, q.Rewards.RoomMessage)
		}
		for i, tr := range q.Triggers {
			walk(fmt.Sprintf("quest|%d|trigger%d", q.QuestId, i), tr.Actions)
		}
	}
	return b.String()
}
