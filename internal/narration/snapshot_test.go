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
//   - _datafiles/world/dogmud/messaging/             2 files (grapple_outcomes.yaml,
//     position_control.yaml as of M4b-1)
//   - _datafiles/world/dogmud/casting-messages.yaml  1 file  (bare file at tree root, 24 lines)
//   (itemvoices/ is retired since item behaviour slice 2, 2026-10-06: sentient
//   item speech lives in behaviors/items/ trees, pinned by
//   internal/hooks/testdata/item_voice_parity.golden.)
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
// Tokens (e.g. {actee}, {itemname}) are substituted with fixed stand-ins
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
//   - defense-messages: type x band(weak/normal/heavy) x role(actee/
//     actor/observer, all 3 from one coordinated call). PLUS the EMPTY
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

	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/gossip"
	"github.com/GoMudEngine/GoMud/internal/grapplemessaging"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/lightnotice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"github.com/GoMudEngine/GoMud/internal/tips"
	"github.com/GoMudEngine/GoMud/modules/weather/content"
	"github.com/GoMudEngine/GoMud/modules/weather/sim"
	"gopkg.in/yaml.v3"
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
	// Condition 0 (Meditating) derives its TriggerCount from this at Validate time
	// and refuses 0; the shipped config.yaml says 3. Set it explicitly rather
	// than load config.yaml: that file is skip-worktree and differs per
	// machine, and a golden must not have a per-machine input. DataFiles and
	// LogoutRounds are the only config keys the three loaders read (verified
	// 2026-09-12); every other knob is a Go zero value here. The loaded condition,
	// spell and quest maps stay populated after this test; nothing else in
	// this package reads them.
	cfg.Network.LogoutRounds = 3
	configs.SetConfigForTest(t, cfg)

	items.LoadDataFiles()
	combat.LoadTauntMessageFiles()
	spells.LoadCastingMessages()

	// Kind B stores (M3 item 5b). Their loaders read the same configured data
	// path, so the golden sees exactly what a booted dogmud world sees.
	conditions.LoadDataFiles()
	spells.LoadSpellFiles()
	quests.LoadDataFiles()

	// M3 item 6: recipes load from the same configured data path.
	crafting.LoadRecipeFiles()

	// M3 item 7: gossip templates load from the same configured data path.
	gossip.Load()
	tips.Load()
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

// There are TWO stand-in maps because there is now ONE token vocabulary.
//
// Before M4a the combat and taunt stores spelled the two participants
// {source}/{target} and the defence store spelled them {attacker}/{defender},
// so a single map could give each store its own stand-in names, and the
// goldens were written with "Source"/"Target" in one file and
// "Attacker"/"Defender" in the other. M4a collapsed both spellings onto
// {actor}/{actee}, which one map cannot serve twice.
//
// Splitting the map is what keeps the goldens byte-identical across the flip,
// and it costs nothing: each store's golden is built by its own function, and
// each passes its own map. Keeping the stand-in WORDS per store is also worth
// something on its own -- a defence row reading "Attacker" still says which
// participant it names without the reader consulting the role mapping.
var tokenStandins = map[items.TokenName]string{
	items.TokenItemName:     "Weapon",
	items.TokenActor:        "Source",
	items.TokenActorType:    "User",
	items.TokenActee:        "Target",
	items.TokenActeeType:    "Mob",
	items.TokenUsesLeft:     "3",
	items.TokenDamage:       "ModerateWounds",
	items.TokenEntranceName: "South",
	items.TokenExitName:     "North",
	items.TokenWeapon:       "Weapon",
	items.TokenAttack:       "Strike",
	items.TokenStance:       "Balanced",
	items.TokenPosition:     "Standing",
	items.TokenMomentum:     "InControl",
	items.TokenBodyPart:     "Arm",
}

// defenseStandins is the defence store's half of the same vocabulary. Same
// two tokens, the store's own stand-in words.
var defenseStandins = map[items.TokenName]string{
	items.TokenActor:    "Attacker",
	items.TokenActee:    "Defender",
	items.TokenWeapon:   "Weapon",
	items.TokenAttack:   "Strike",
	items.TokenStance:   "Balanced",
	items.TokenPosition: "Standing",
	items.TokenMomentum: "InControl",
}

func substituteWith(standins map[items.TokenName]string, s string) string {
	for tok, val := range standins {
		s = strings.ReplaceAll(s, string(tok), val)
	}
	return s
}

func substituteTokens(s string) string {
	return substituteWith(tokenStandins, s)
}

func substituteDefenseTokens(s string) string {
	return substituteWith(defenseStandins, s)
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
	fmt.Fprintf(&b, "# dimensions: subtype x intensity x section x role x tier x index, every authored line exactly once\n")
	fmt.Fprintf(&b, "# separate section present only for: generic, shooting (per source at time of writing)\n")
	fmt.Fprintf(&b, "# coupdegrace recorded for generic only: it is authored nowhere else, and every\n")
	fmt.Fprintf(&b, "# other subtype reaches it through GetPreAttackMessage's fallback to Generic\n")
	fmt.Fprintf(&b, "# PR 1 of M3 item 8 widened this from index 0 only; PR 2 re-keys it to coordinated rows\n\n")

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
		{"actor", func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToAttacker }},
		{"actee", func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToDefender }},
		{"observer", func(m items.TogetherMessages) items.SkillTieredMessages { return m.ToRoom }},
	}
	separateRoles := []struct {
		name string
		get  func(items.SeparateMessages) items.SkillTieredMessages
	}{
		{"actor", func(m items.SeparateMessages) items.SkillTieredMessages { return m.ToAttacker }},
		{"actee", func(m items.SeparateMessages) items.SkillTieredMessages { return m.ToDefender }},
		{"observer", func(m items.SeparateMessages) items.SkillTieredMessages { return m.ToAttackerRoom }},
		{"remote_observer", func(m items.SeparateMessages) items.SkillTieredMessages { return m.ToDefenderRoom }},
	}

	hasSeparate := map[string]bool{"generic": true, "shooting": true}

	// messageTexts renders one tier's pool with tokens substituted.
	messageTexts := func(mo items.MessageOptions) []string {
		out := make([]string, len(mo))
		for i, m := range mo {
			out[i] = substituteTokens(string(m))
		}
		return out
	}
	// poolAt is the empty string past the end of a short pool. After the M3
	// item 8 pad no pool is short, so a "<none>" here would be a real find.
	poolAt := func(pool []string, idx int) string {
		if idx < len(pool) {
			return pool[idx]
		}
		return "<none>"
	}

	for _, subtype := range subtypes {
		// coupdegrace is authored only in generic.yaml. Looping it over every
		// subtype would record generic's lines 19 extra times, because
		// GetPreAttackMessage falls back to Generic for a missing intensity.
		intensityList := intensities
		if subtype == "generic" {
			intensityList = append(append([]items.Intensity{}, intensities...), items.CoupDeGrace)
		}

		for _, intensity := range intensityList {
			opts := items.GetPreAttackMessage(items.ItemSubType(subtype), intensity)

			// One row per COORDINATED VARIANT, with every role on it. The
			// grouping is the thing under test now: production renders all
			// audiences from a single index, so a row that reads coherently
			// across its roles is the property, and a role swap or a lost
			// coordination shows as a moved string rather than as nothing.
			//
			// Rows stay keyed by the AUTHORED role name inside the row, not
			// the core's Actor/Actee vocabulary, for the same reason
			// defense_messages.golden does (narration/context.md:134-139):
			// it is what makes a swap of which authored pool lands in which
			// role visible.
			for _, tier := range tiers {
				pools := map[string][]string{}
				widest := 0
				for _, role := range togetherRoles {
					pool := messageTexts(tier.get(role.get(opts.Together)))
					pools[role.name] = pool
					if len(pool) > widest {
						widest = len(pool)
					}
				}
				for idx := 0; idx < widest; idx++ {
					fmt.Fprintf(&b, "%s|%s|together|%s|%d =>", subtype, intensity, tier.name, idx)
					for _, role := range togetherRoles {
						fmt.Fprintf(&b, " %s=%s", role.name, poolAt(pools[role.name], idx))
					}
					fmt.Fprintf(&b, "\n")
				}
			}

			if hasSeparate[subtype] {
				for _, tier := range tiers {
					pools := map[string][]string{}
					widest := 0
					for _, role := range separateRoles {
						pool := messageTexts(tier.get(role.get(opts.Separate)))
						pools[role.name] = pool
						if len(pool) > widest {
							widest = len(pool)
						}
					}
					for idx := 0; idx < widest; idx++ {
						fmt.Fprintf(&b, "%s|%s|separate|%s|%d =>", subtype, intensity, tier.name, idx)
						for _, role := range separateRoles {
							fmt.Fprintf(&b, " %s=%s", role.name, poolAt(pools[role.name], idx))
						}
						fmt.Fprintf(&b, "\n")
					}
				}
			}
		}
	}

	// Derived-selection sanity check, on one representative
	// subtype/intensity/role. This is the seam production actually calls for
	// the core combat loop.
	//
	// It used to freeze GetForSkillLevelWith's index-0 pick. That method went
	// with the ConsistentAttackMessages deletion in M3 item 8, and PoolFor is
	// the equivalent seam: the store assembles the cumulative tier union and
	// hands it to the core. Freezing the union's SIZE as well as its first
	// entry says more than the old row did, because the size is what the
	// coordinated index is taken modulo of.
	fmt.Fprintf(&b, "\n# derived-selection (PoolFor), bite/weak/actor, cumulative tier union\n")
	opts := items.GetPreAttackMessage(items.Bite, items.Weak)
	for _, skillLevel := range []int{10, 50, 90} {
		pool := opts.Together.ToAttacker.PoolFor(skillLevel)
		first := ""
		if len(pool) > 0 {
			first = substituteTokens(pool[0])
		}
		fmt.Fprintf(&b, "derived|bite|weak|actor|skill=%d|n=%d => %s\n", skillLevel, len(pool), first)
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
	fmt.Fprintf(&b, "# dimensions: defense-type x band(weak/normal/heavy) -- all 3 roles (actee/actor/observer)\n")
	fmt.Fprintf(&b, "# come from ONE coordinated RenderDefenseMessage call (same index across the triad).\n")
	fmt.Fprintf(&b, "# RenderDefenseMessage has no picker param (util.Rand only) -- pinned via indexOverride=0,\n")
	fmt.Fprintf(&b, "# the fresh-SequencePicker-first-pick equivalent for this seam.\n\n")
	fmt.Fprintf(&b, "# The melee path no longer has a banding rule of its own (M4c): internal/combat\n")
	fmt.Fprintf(&b, "# calls RenderDefenseMessage like every other defence. Its production-path matrix\n")
	fmt.Fprintf(&b, "# is frozen by internal/combat/testdata/melee_defence_bands.golden; the rows that\n")
	fmt.Fprintf(&b, "# used to sit here were byte-identical duplicates of the store rows above.\n")

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
			triad := items.RenderDefenseMessage(items.DefencePool(dt), band.crit, band.margin, defenseStandins, 0)
			fmt.Fprintf(&b, "%s|%s|actee => %s\n", dt, band.name, substituteDefenseTokens(string(triad.ToDefender)))
			fmt.Fprintf(&b, "%s|%s|actor => %s\n", dt, band.name, substituteDefenseTokens(string(triad.ToAttacker)))
			fmt.Fprintf(&b, "%s|%s|observer => %s\n", dt, band.name, substituteDefenseTokens(string(triad.ToRoom)))
		}
	}

	// EMPTY CASE: an unregistered defense type returns an all-empty triad.
	fmt.Fprintf(&b, "\n# EMPTY CASE: unregistered defense type -> empty triad\n")
	emptyTriad := items.RenderDefenseMessage(items.DefencePool("nonexistent-defense-type"), false, 0.6, defenseStandins, 0)
	fmt.Fprintf(&b, "nonexistent-defense-type|normal|actee => %q\n", string(emptyTriad.ToDefender))
	fmt.Fprintf(&b, "nonexistent-defense-type|normal|actor => %q\n", string(emptyTriad.ToAttacker))
	fmt.Fprintf(&b, "nonexistent-defense-type|normal|observer => %q\n", string(emptyTriad.ToRoom))

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
	fmt.Fprintf(&b, "# dimensions: intensity(hit/miss/critical/fumble) x perspective(actor/actee/observer)\n")
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
		fmt.Fprintf(&b, "rhetoric|%s|actor => %s\n", intensity, triad.ToAttacker)
		fmt.Fprintf(&b, "rhetoric|%s|actee => %s\n", intensity, triad.ToDefender)
		fmt.Fprintf(&b, "rhetoric|%s|observer => %s\n", intensity, triad.ToRoom)
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

func sortedKeysIndoorPool(m map[string]content.IndoorPool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysTableSection(m map[string]content.TableSection) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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
	fmt.Fprintf(&b, "# and gradients BOTH use role in {actor,actee,observer}: M4b-1 merged the two authored vocabularies\n")
	fmt.Fprintf(&b, "# (controller/controlled/observers and self/partner/observers). striking_apex is single-speaker (no role).\n")
	fmt.Fprintf(&b, "# Each tuple: fresh cooldowns map + fresh SequencePicker via PickTemplate, then narration.Substitute\n")
	fmt.Fprintf(&b, "# with fixed stand-ins Controller/Controlled.\n\n")

	renderPool := func(pool []string) string {
		tmpl := grapplemessaging.PickTemplate(pool, map[string]bool{}, "snapshot", narration.SequencePicker())
		return narration.Substitute(tmpl, map[string]string{
			narration.TokenActor: "Controller",
			narration.TokenActee: "Controlled",
		})
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
			fmt.Fprintf(&b, "%s|%s|actor => %s\n", cat.name, key, renderPool(triad.Controller))
			fmt.Fprintf(&b, "%s|%s|actee => %s\n", cat.name, key, renderPool(triad.Controlled))
			fmt.Fprintf(&b, "%s|%s|observer => %s\n", cat.name, key, renderPool(triad.Observers))
		}
	}

	for _, key := range sortedKeysStrSlice(lib.StrikingApex) {
		fmt.Fprintf(&b, "striking_apex|%s|(single-speaker) => %s\n", key, renderPool(lib.StrikingApex[key]))
	}

	for _, key := range sortedKeysGradient(lib.Gradients) {
		g := lib.Gradients[key]
		fmt.Fprintf(&b, "gradients|%s|actor => %s\n", key, renderPool(g.Self))
		fmt.Fprintf(&b, "gradients|%s|actee => %s\n", key, renderPool(g.Partner))
		fmt.Fprintf(&b, "gradients|%s|observer => %s\n", key, renderPool(g.Observers))
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
	t.Run("conditions", func(t *testing.T) {
		checkGolden(t, "conditions.golden", buildConditionsGolden(t))
	})
	t.Run("spells", func(t *testing.T) {
		checkGolden(t, "spells.golden", buildSpellsGolden(t))
	})
	t.Run("quests", func(t *testing.T) {
		checkGolden(t, "quests.golden", buildQuestsGolden(t))
	})
	t.Run("crafting", func(t *testing.T) {
		checkGolden(t, "crafting.golden", buildCraftingGolden(t))
	})
	t.Run("gossip", func(t *testing.T) {
		checkGolden(t, "gossip.golden", buildGossipGolden(t))
	})
	t.Run("tips", func(t *testing.T) {
		checkGolden(t, "tips.golden", buildTipsGolden(t))
	})
	t.Run("weather_emotes", func(t *testing.T) {
		checkGolden(t, "weather_emotes.golden", buildWeatherEmotesGolden(t))
	})
	t.Run("post_pipeline", func(t *testing.T) {
		checkGolden(t, "post_pipeline.golden", buildPostPipelineGolden(t))
	})
	t.Run("position_control", func(t *testing.T) {
		checkGolden(t, "position_control.golden", buildPositionControlGolden(t))
	})
	t.Run("special_moves", func(t *testing.T) {
		checkGolden(t, "special_moves.golden", buildSpecialMovesGolden(t))
	})
	t.Run("light_notices", func(t *testing.T) {
		checkGolden(t, "light_notices.golden", buildLightNoticesGolden(t))
	})
}

// ---------------------------------------------------------------------
// Kind B stores (M3 item 5b): conditions, spells, quests. Single strings per
// lifecycle phase, no pool, so no picker is involved: the golden freezes the
// substitution and the notice logic, keyed by the AUTHORED key name so a
// swapped role shows up as a changed row.
// ---------------------------------------------------------------------

// kindBSource is the stand-in name set. The source is a player and the target
// a mob, so the two tags differ and a swap of {actor} for {actee} would
// change the golden.
var kindBSource = textutil.TokenContext{
	ActorName:      `<ansi fg="username">Aliceia</ansi>`,
	ActorPlainName: "Aliceia",
	ActeeName:      `<ansi fg="mobname">Targetticus</ansi>`,
	ActeePlainName: "Targetticus",
}

// kindBNoTarget is the same source with no target, which is how every condition
// site and the quest bridge render: they never know a target.
var kindBNoTarget = textutil.TokenContext{
	ActorName:      kindBSource.ActorName,
	ActorPlainName: kindBSource.ActorPlainName,
}

// Store 8: conditions (internal/conditions, six <phase>_actee / <phase>_observer
// fields)
//
// Since M3 item 5b this builder reads through ConditionSpec.Narrate and
// AuthoredStartLine; the emitted rows, their order and the header are
// unchanged from the pre-migration recording, which is the byte-identity
// proof. M4b-1 renamed the authored keys these rows are labelled with
// (start_user_text became start_actee, and so on), which moves the LABEL half
// of every row and nothing else.
func buildConditionsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# conditions store snapshot (internal/conditions)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code. The *_actee rows record what the\n")
	fmt.Fprintf(&b, "# HOLDER is sent: for start and end that is StartUserNotice / EndUserNotice (authored\n")
	fmt.Fprintf(&b, "# line, else the generic fallback, else nothing for a secret condition). A row exists only\n")
	fmt.Fprintf(&b, "# when the sent line is non-empty. authored_start_line is the raw start_actee of a\n")
	fmt.Fprintf(&b, "# silent-start condition, recorded for every silent-start condition whether or not a site sends it\n")
	fmt.Fprintf(&b, "# today: sleep (15), arrest (88), stun (84) and broken limb (83) have a sender; throttled\n")
	fmt.Fprintf(&b, "# (89) does not, its move narrates the choke itself.\n")
	fmt.Fprintf(&b, "# dimensions: condition id x authored key; source only, conditions never know a target\n\n")

	ids := conditions.GetAllConditionIds()
	sort.Ints(ids)
	if len(ids) == 0 {
		t.Fatal("no conditions loaded; setupRealStores must call conditions.LoadDataFiles()")
	}
	for _, id := range ids {
		spec := conditions.GetConditionSpec(id)
		if spec == nil {
			t.Fatalf("condition %d has no spec", id)
		}
		phases := []struct {
			p                conditions.Phase
			userKey, roomKey string
		}{
			{conditions.PhaseStart, "start_actee", "start_observer"},
			{conditions.PhaseTrigger, "trigger_actee", "trigger_observer"},
			{conditions.PhaseEnd, "end_actee", "end_observer"},
		}
		for _, ph := range phases {
			// Conditions take the HOLDER, not a context: the store puts it in the
			// Actee slot itself. Same stand-in name as every other store, so the
			// rendered rows stay comparable.
			roles := spec.Narrate(ph.p, kindBNoTarget.ActorName, kindBNoTarget.ActorPlainName)
			if roles.Actee != "" {
				fmt.Fprintf(&b, "condition|%d|%s => %s\n", id, ph.userKey, roles.Actee)
			}
			if roles.Observer != "" {
				fmt.Fprintf(&b, "condition|%d|%s => %s\n", id, ph.roomKey, roles.Observer)
			}
		}
		if slices.Contains(spec.Flags, conditions.SilentStart) {
			if line := spec.AuthoredStartLine(kindBNoTarget.ActorName, kindBNoTarget.ActorPlainName); line != "" {
				fmt.Fprintf(&b, "condition|%d|authored_start_line => %s\n", id, line)
			}
		}
	}
	return b.String()
}

// Store 9: spells (internal/spells, six cast/wait/magic x actor/observer fields)
//
// Since M3 item 5b this builder reads through SpellData.Narrate; the emitted
// rows, their order and the header are unchanged from the pre-migration
// recording, which is the byte-identity proof. M4b-1 renamed the authored keys
// these rows are labelled with (cast_user_text became cast_actor, and so on),
// which moves the LABEL half of every row and nothing else. The two probe rows still go
// through textutil.SubstituteTokens directly, since they freeze the token
// contract itself, not a store.
func buildSpellsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# spells store snapshot (internal/spells)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code: textutil.SubstituteTokens over each raw\n")
	fmt.Fprintf(&b, "# field with a source AND a target. A |notarget row follows any line whose rendering\n")
	fmt.Fprintf(&b, "# changes when the target is absent, freezing the empty substitution.\n")
	fmt.Fprintf(&b, "# probe rows freeze the token contract itself; they are not authored content\n")
	fmt.Fprintf(&b, "# dimensions: spell id x authored key [x notarget]\n\n")

	// Probe rows: no shipped line carries {actee_plain} or an unknown token,
	// so this fixed string freezes the whole substitution contract, including
	// passthrough of an unknown token and the empty actee.
	const probe = "{actor} and {actor_plain} at {actee} and {actee_plain}; {unknown} stays; {actor} again"
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
		phases := []struct {
			p                spells.Phase
			userKey, roomKey string
		}{
			{spells.PhaseCast, "cast_actor", "cast_observer"},
			{spells.PhaseWait, "wait_actor", "wait_observer"},
			{spells.PhaseMagic, "magic_actor", "magic_observer"},
		}
		for _, ph := range phases {
			with := s.Narrate(ph.p, kindBSource)
			without := s.Narrate(ph.p, kindBNoTarget)
			if with.Actor != "" {
				fmt.Fprintf(&b, "spell|%s|%s => %s\n", id, ph.userKey, with.Actor)
				if without.Actor != with.Actor {
					fmt.Fprintf(&b, "spell|%s|%s|notarget => %s\n", id, ph.userKey, without.Actor)
				}
			}
			if with.Observer != "" {
				fmt.Fprintf(&b, "spell|%s|%s => %s\n", id, ph.roomKey, with.Observer)
				if without.Observer != with.Observer {
					fmt.Fprintf(&b, "spell|%s|%s|notarget => %s\n", id, ph.roomKey, without.Observer)
				}
			}
		}
	}
	return b.String()
}

// Store 10: quests (internal/quests: reward actor/observer, and the actor /
// observer actions, nested sequences included)
//
// Since M3 item 5b this builder reads through ActionDef.Narrate and
// QuestReward.Narrate; the emitted rows, their order and the header are
// unchanged from the pre-migration recording, which is the byte-identity
// proof. The header's "actor line RAW (no substitution)" describes the retired
// recording, not production: since the same slice, questengine.ExecuteAction
// renders the actor line through GameBridge.Narrate, which substitutes. The
// bytes did not change because no shipped quest action or reward line carries
// a token.
//
// M4b-1 renamed all four authored keys (rewards playermessage/roommessage and
// actions send_text/room_text all became actor/observer), which moves the
// LABEL half of every row and nothing else. The reward and action rows are
// still told apart by the `rewards` / `trigger<i>|action<j>` field ahead of
// the label, so collapsing four spellings into two loses no dimension.
func buildQuestsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# quests store snapshot (internal/quests)\n")
	fmt.Fprintf(&b, "# Built 2026-09-12 from PRE-migration code, sending what each site sends today:\n")
	fmt.Fprintf(&b, "# reward actor/observer and action actor RAW (no substitution),\n")
	fmt.Fprintf(&b, "# action observer through textutil.SubstituteTokens with the player as {actor}.\n")
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
			roles := a.Narrate(kindBNoTarget)
			if roles.Actor != "" {
				fmt.Fprintf(&b, "%s|actor => %s\n", aw, roles.Actor)
			}
			if roles.Observer != "" {
				fmt.Fprintf(&b, "%s|observer => %s\n", aw, roles.Observer)
			}
			if a.Sequence != nil {
				walk(aw+"|sequence", a.Sequence.OnComplete)
			}
		}
	}
	for _, q := range all {
		reward := q.Rewards.Narrate(kindBNoTarget)
		if reward.Actor != "" {
			fmt.Fprintf(&b, "quest|%d|rewards|actor => %s\n", q.QuestId, reward.Actor)
		}
		if reward.Observer != "" {
			fmt.Fprintf(&b, "quest|%d|rewards|observer => %s\n", q.QuestId, reward.Observer)
		}
		for i, tr := range q.Triggers {
			walk(fmt.Sprintf("quest|%d|trigger%d", q.QuestId, i), tr.Actions)
		}
	}
	return b.String()
}

// Store 11: crafting (internal/crafting: success_actor / failure_actor and the
// optional *_observer slot)
//
// Recorded 2026-09-15 from PRE-migration code (the raw field, color-wrapped as
// the four player sites sent it). Since M3 item 6 this builder reads through
// RecipeSpec.Narrate; the rows, their order and the header are unchanged, which
// is the byte-identity proof. Rows are keyed by the AUTHORED key, so a swapped
// Actor and Observer shows as a changed row. M4b-1 renamed those keys
// (success_message became success_actor, success_room_message became
// success_observer, and so on), which moves the LABEL half of every row and
// nothing else.
func buildCraftingGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# crafting store snapshot (internal/crafting)\n")
	fmt.Fprintf(&b, "# Built 2026-09-15 from PRE-migration code: what the CRAFTER is sent, color wrap\n")
	fmt.Fprintf(&b, "# included (success green, failure red, CategorySystem). Recipes had no room line\n")
	fmt.Fprintf(&b, "# before M3 item 6; *_observer rows appear only once one is authored.\n")
	fmt.Fprintf(&b, "# dimensions: recipe id x authored key; source only, a craft has no target\n\n")

	all := crafting.GetAll()
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		t.Fatal("no recipes loaded; setupRealStores must call crafting.LoadRecipeFiles()")
	}
	for _, id := range ids {
		r := all[id]
		success := r.Narrate(crafting.PhaseSuccess, kindBNoTarget)
		failure := r.Narrate(crafting.PhaseFailure, kindBNoTarget)
		fmt.Fprintf(&b, "recipe|%s|success_actor => %s\n", id, fmt.Sprintf(`<ansi fg="green">%s</ansi>`, success.Actor))
		if success.Observer != "" {
			fmt.Fprintf(&b, "recipe|%s|success_observer => %s\n", id, success.Observer)
		}
		fmt.Fprintf(&b, "recipe|%s|failure_actor => %s\n", id, fmt.Sprintf(`<ansi fg="red">%s</ansi>`, failure.Actor))
		if failure.Observer != "" {
			fmt.Fprintf(&b, "recipe|%s|failure_observer => %s\n", id, failure.Observer)
		}
	}
	return b.String()
}

// Store 12: gossip templates (_datafiles/world/dogmud/gossip_templates.yaml)
//
// Built from PRE-migration data and code: the YAML is parsed here and each
// variant is substituted exactly as internal/hooks does it today. Today's code
// picks with util.Rand, which no test can pin, so this golden freezes every
// variant's substitution, not a pick. Since M3 item 7 Task 2 it reads through
// the gossip store; rows, order and header are unchanged from the
// pre-migration recording, which is the byte-identity proof.
const gossipStandIn = "<the stand-in event>"

func buildGossipGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# gossip store snapshot\n")
	fmt.Fprintf(&b, "# Built 2026-09-15 from PRE-migration code. Every variant of every key, substituted\n")
	fmt.Fprintf(&b, "# as the gossiper sends it: event keys replace {desc} once, fact- keys replace every\n")
	fmt.Fprintf(&b, "# {description}, fallback lines are sent as written. The stand-in description is\n")
	fmt.Fprintf(&b, "# %q. The pick itself (util.Rand) is not frozen; no test can pin it.\n", gossipStandIn)
	fmt.Fprintf(&b, "# dimensions: key x variant index\n\n")

	keys := gossip.Keys()
	if len(keys) == 0 {
		t.Fatal("no gossip keys loaded; setupRealStores must call gossip.Load()")
	}
	for _, key := range keys {
		token, value := "{desc}", gossipStandIn
		switch {
		case key == "fallback":
			token, value = "", ""
		case strings.HasPrefix(key, "fact-"):
			token = "{description}"
		}
		pool := gossip.Pool(key)
		for i := range pool {
			index := i
			rendered := gossip.RenderWithForTest(pool, token, value, func(int) int { return index })
			fmt.Fprintf(&b, "gossip|%s|%d => %s\n", key, i, rendered)
		}
	}
	return b.String()
}

// Store 13: tips (the periodic broadcast; hints.yaml before M3 item 7)
//
// Recorded pre-migration from the `hints:` list of hints.yaml, in file order,
// which was the broadcast's rotation order. Since M3 item 7 Task 3 this
// builder reads through the tips store (internal/tips); rows and header are
// unchanged from the pre-migration recording, which is the byte-identity proof.
func buildTipsGolden(t *testing.T) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "# tips store snapshot\n")
	fmt.Fprintf(&b, "# Built 2026-09-15 from PRE-migration data. Every tip in rotation order, as the text\n")
	fmt.Fprintf(&b, "# after the [Tip] prefix. dimensions: rotation index\n\n")

	defer tips.SeedForTest(tips.All())()
	n := tips.Count()
	if n == 0 {
		t.Fatal("no tips loaded; setupRealStores must call tips.Load()")
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "tip|%d => %s\n", i, tips.Next())
	}

	return b.String()
}

// ---------------------------------------------------------------------
// Store 14: weather emotes (modules/weather/content)
//
// The ONLY actorless store in the arc: an ambient line has no Actor, so it is
// rendered with narration.Variants{Observer: lines} and the other three roles
// stay empty. Dimensions: weather type x section(outdoor/sheltered) x biome
// (authored keys UNION a representative set, on BOTH sections) x season(base
// + each authored variant), then the seasonal-ambience tables by (track,
// season). The mild/strong BAND axis applies to sheltered rows only; outdoor
// lines are never felt-banded, so it is not a dimension of the outdoor rows.
//
// The sheltered axis is named for the ROOM, not for the section it resolves
// to, because item 9 PR 2 splits that one section into indoor and underground
// and the row keys must survive it.
//
// Recorded 2026-09-16 from PRE-migration code. Weather already had a picker
// seam (Pick takes `roll func(int) int`, and narration.Picker has that exact
// underlying type, so SequencePicker is assignable with no production change),
// which is why this baseline needed no `*With` variant the way itemvoices did.
//
// Indoor bands are forced by the felt value, not by naming a band: felt 0.0 is
// below the 0.5 strongFeltThreshold this golden passes to every Pick call and
// selects Mild; felt 1.0 is at or
// above it and selects Strong. An empty Mild pool rendering "" is DELIBERATE
// (light weather is inaudible through walls) and that emptiness is frozen here
// too, so a pool silently disappearing shows as a row changing from text to "".
func buildWeatherEmotesGolden(t *testing.T) string {
	t.Helper()
	root := os.DirFS(dogmudDataDir(t))

	tables, err := content.LoadEmotes(root, "weather/emotes")
	if err != nil {
		t.Fatalf("LoadEmotes: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("no weather emote tables loaded; the golden would be vacuous")
	}
	seasonal, err := content.LoadSeasonalEmotes(root, "weather/emotes/seasons")
	if err != nil {
		t.Fatalf("LoadSeasonalEmotes: %v", err)
	}
	if len(seasonal) == 0 {
		t.Fatal("no seasonal ambience tables loaded; the golden would be vacuous")
	}

	bands := []struct {
		name string
		felt float64
	}{{"mild", 0.0}, {"strong", 1.0}}

	// Representative biomes, swept IN ADDITION to the authored keys.
	//
	// THIS IS WHAT MAKES THE GOLDEN ABLE TO SEE PR 2. The authored indoor
	// keys are "default" only, so sweeping authored keys alone would never
	// exercise a cave, and the three-way split would land with no diff to
	// inspect. Today all of these resolve to indoor["default"]; after PR 2
	// classifies them, cave and dungeon must move to the underground section
	// and the others must not, which shows up here as a diff on exactly
	// those rows.
	repBiomes := []string{"cave", "dungeon", "house", "fort", "spiderweb", "forest"}

	// union merges the authored keys with the representative set, de-duplicated
	// and sorted, so every row is stable across runs.
	union := func(authored []string) []string {
		seen := map[string]bool{}
		out := []string{}
		for _, k := range append(append([]string{}, authored...), repBiomes...) {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
		sort.Strings(out)
		return out
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# weather emotes store snapshot\n")
	fmt.Fprintf(&b, "# weather tables: %d   seasonal-ambience tables: %d\n", len(tables), len(seasonal))
	fmt.Fprintf(&b, "# Recorded 2026-09-16 from PRE-migration code: a baseline of existing behaviour,\n")
	fmt.Fprintf(&b, "# not a record of what a later migration produced.\n")
	fmt.Fprintf(&b, "# dimensions: type x section(outdoor/sheltered) x biome x band x season, then\n")
	fmt.Fprintf(&b, "# (track,season) ambience. Band applies to sheltered rows only; outdoor lines\n")
	fmt.Fprintf(&b, "# are never felt-banded.\n")
	fmt.Fprintf(&b, "# \"sheltered\" names the ROOM, not the section: item 9 PR 2 splits it into\n")
	fmt.Fprintf(&b, "# indoor and underground, and these row keys must survive that split.\n")
	fmt.Fprintf(&b, "# Biome is authored keys UNION a representative set (cave, dungeon, house,\n")
	fmt.Fprintf(&b, "# fort, spiderweb, forest) on both axes: the only authored sheltered key is\n")
	fmt.Fprintf(&b, "# \"default\", so without this set the golden would never see a cave and PR 2's\n")
	fmt.Fprintf(&b, "# split would land with no diff; the same union on outdoor exercises its own\n")
	fmt.Fprintf(&b, "# biome-to-default fallback.\n")
	fmt.Fprintf(&b, "# Single role (the room is told), no tokens authored anywhere in this store.\n")
	fmt.Fprintf(&b, "# A fresh SequencePicker per row pins index 0.\n")
	fmt.Fprintf(&b, "# An empty row (\"\") in a mild band is deliberate silence, not a missing pool.\n\n")

	types := make([]string, 0, len(tables))
	for wt := range tables {
		types = append(types, string(wt))
	}
	sort.Strings(types)

	for _, wt := range types {
		w := sim.WeatherType(wt)
		tbl := tables[w]

		// union() here too: the outdoor axis has its own biome-to-default
		// fallback, and none of the representative biomes (cave, dungeon,
		// house, fort, spiderweb) is ever authored outdoors -- the union
		// exists to exercise that fallback, not because a cave is ever
		// outdoors.
		for _, biome := range union(sortedKeysStrSlice(tbl.Outdoor)) {
			fmt.Fprintf(&b, "%s|base|outdoor|%s => %q\n", wt, biome,
				tables.Pick(w, biome, false, 0, 0.5, "", narration.SequencePicker()))
		}
		// "sheltered" rather than "indoor": after PR 2 this axis covers two
		// prose classes, and the row key must not have to be renamed then.
		for _, biome := range union(sortedKeysIndoorPool(tbl.Indoor)) {
			for _, bd := range bands {
				fmt.Fprintf(&b, "%s|base|sheltered|%s|%s => %q\n", wt, biome, bd.name,
					tables.Pick(w, biome, true, bd.felt, 0.5, "", narration.SequencePicker()))
			}
		}
		for _, season := range sortedKeysTableSection(tbl.Seasonal) {
			sec := tbl.Seasonal[season]
			for _, biome := range union(sortedKeysStrSlice(sec.Outdoor)) {
				fmt.Fprintf(&b, "%s|season:%s|outdoor|%s => %q\n", wt, season, biome,
					tables.Pick(w, biome, false, 0, 0.5, season, narration.SequencePicker()))
			}
			for _, biome := range union(sortedKeysIndoorPool(sec.Indoor)) {
				for _, bd := range bands {
					fmt.Fprintf(&b, "%s|season:%s|sheltered|%s|%s => %q\n", wt, season, biome, bd.name,
						tables.Pick(w, biome, true, bd.felt, 0.5, season, narration.SequencePicker()))
				}
			}
		}
	}

	// Seasonal ambience: the persistent voice of a season in CALM weather.
	fmt.Fprintf(&b, "\n# seasonal ambience tables, keyed (track, season)\n")
	keys := make([]string, 0, len(seasonal))
	index := map[string]content.SeasonalKey{}
	for k := range seasonal {
		flat := k.Track + "/" + k.Season
		keys = append(keys, flat)
		index[flat] = k
	}
	sort.Strings(keys)
	for _, flat := range keys {
		k := index[flat]
		sec := seasonal[k]
		for _, biome := range union(sortedKeysStrSlice(sec.Outdoor)) {
			fmt.Fprintf(&b, "%s|%s|outdoor|%s => %q\n", k.Track, k.Season, biome,
				seasonal.Pick(k.Track, k.Season, biome, false, 0, 0.5, narration.SequencePicker()))
		}
		for _, biome := range union(sortedKeysIndoorPool(sec.Indoor)) {
			for _, bd := range bands {
				fmt.Fprintf(&b, "%s|%s|sheltered|%s|%s => %q\n", k.Track, k.Season, biome, bd.name,
					seasonal.Pick(k.Track, k.Season, biome, true, bd.felt, 0.5, narration.SequencePicker()))
			}
		}
	}

	// EMPTY CASES, frozen deliberately.
	fmt.Fprintf(&b, "\n# EMPTY CASE: unknown weather type -> \"\"\n")
	fmt.Fprintf(&b, "bogus-weather|base|outdoor|default => %q\n",
		tables.Pick(sim.WeatherType("bogus-weather"), "default", false, 0, 0.5, "", narration.SequencePicker()))
	fmt.Fprintf(&b, "\n# EMPTY CASE: unknown (track,season) ambience -> \"\"\n")
	fmt.Fprintf(&b, "bogus-track|bogus-season|outdoor|default => %q\n",
		seasonal.Pick("bogus-track", "bogus-season", "default", false, 0, 0.5, narration.SequencePicker()))

	return b.String()
}

// ---------------------------------------------------------------------
// Store 15: position_control
// (_datafiles/world/dogmud/messaging/position_control.yaml)
//
// The TENTH message store. It used to live at _datafiles/messages/, outside
// _datafiles/world/dogmud, which is the only tree the M0 surface guard walks,
// and that is why it reached M4a with no golden and no guard at all. M4b-1
// moved it under the world tree beside grapple_outcomes.yaml.
//
// The golden's HEADER still names the old path, deliberately: golden header
// lines are frozen bytes recorded once (see internal/narration/context.md), and
// moving a file is not a content change. Re-recording to chase a path would
// spend the one signal that says these rows have not drifted.
//
// Recorded from PRE-migration code (M4a task 6), when production rendered this
// store through a third hand-rolled engine, the local substitute() in
// internal/hooks/Position_Messaging.go. THREE name vocabularies were authored
// here: {attacker}/{target} in the submission block, {Controller}/{Controlled}
// in the gradient and transition blocks, and {Character} in the stamina
// warning. M4a task 6 collapsed all three onto {actor}/{actee} and deleted
// that engine; this builder now reads through narration.Substitute, and the
// rows, their order and the header are unchanged from the pre-migration
// recording, which is the byte-identity proof.
//
// M4b-1 then did the same to the store's three authored ROLE KEY vocabularies:
// attacker/target/room in the submission block, controller/controlled on the
// gradient and transition sides, and self/room in the stamina warning, all now
// actor/actee/observer. That rename moves the golden's row LABELS, so
// byte-identity cannot prove it; the proof is tools/messaging_role_key_check.py,
// which translates the pre-rename golden's labels and requires equality.
//
// The gradient and transition SIDE field is a role and moved with the rest.
// The gradient STATE spelled `controlled` is authored data and did not: it is
// why both the rewrite table and the check table carry a scope for that one
// spelling.
//
// dimensions: block x key [x subtype] x role. Every authored line exactly
// once, empty lines included: an authored "" is deliberate silence in this
// store (the controlled side of a gradient has no room line) and freezing it
// means a pool appearing or vanishing shows as a changed row.
//
// PARTIAL NET, stated plainly: gradient_messages and transition_messages are
// recorded here but are NOT read by any Go code. hooks.positionMessageTemplates
// parses only stamina_warning and submission; the live gradient and transition
// prose comes from internal/grapplemessaging and messaging_grapple.yaml. Those
// rows therefore guard the DATA, not a render path.

// posSelfRoom is an actor/observer pair (gradient, transition, stamina
// blocks). Its keys were self/room until M4b-1's role-key rename.
type posSelfRoom struct {
	Self string `yaml:"actor"`
	Room string `yaml:"observer"`
}

// posTriple is an actor/actee/observer triple (the submission block). Its keys
// were attacker/target/room until M4b-1's role-key rename.
type posTriple struct {
	Attacker string `yaml:"actor"`
	Target   string `yaml:"actee"`
	Room     string `yaml:"observer"`
}

// positionControlFile mirrors the shipped file's shape with maps rather than
// named fields, so a key added to or removed from the YAML moves the golden
// instead of being silently dropped by an unmarshal into a fixed struct.
type positionControlFile struct {
	Gradient   map[string]map[string]posSelfRoom `yaml:"gradient_messages"`
	Transition map[string]posSelfRoom            `yaml:"transition_messages"`
	Stamina    posSelfRoom                       `yaml:"stamina_warning"`
	Submission map[string]yaml.Node              `yaml:"submission"`
}

func loadPositionControlForSnapshot(t *testing.T) positionControlFile {
	t.Helper()
	path := filepath.Join(dogmudDataDir(t), "messaging", "position_control.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out positionControlFile
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}

// positionControlStandins is the fixed stand-in map, on the canonical
// vocabulary since M4a.
//
// The pre-migration recording used the same two people under the store's three
// old spellings ({Character} and {Controller} both Actorius, {Controlled} and
// {target} both Acteeus, {attacker} Actorius), which is why the flip to
// {actor}/{actee} leaves the golden byte-identical.
var positionControlStandins = map[string]string{
	"{position}":         "side control",
	"{old_position}":     "guard",
	"{new_position}":     "side control",
	narration.TokenActor: "Actorius",
	narration.TokenActee: "Acteeus",
}

func sortedMapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func buildPositionControlGolden(t *testing.T) string {
	t.Helper()
	tpl := loadPositionControlForSnapshot(t)

	var b strings.Builder
	fmt.Fprintf(&b, "# position_control store snapshot (_datafiles/messages/position_control.yaml)\n")
	fmt.Fprintf(&b, "# Recorded 2026-09-17 from PRE-migration code: the local substitute() in\n")
	fmt.Fprintf(&b, "# internal/hooks/Position_Messaging.go, a {key} loop over the three authored name\n")
	fmt.Fprintf(&b, "# vocabularies {attacker}/{target}, {Controller}/{Controlled} and {Character}.\n")
	fmt.Fprintf(&b, "# Stand-ins: Actorius is the actor side, Acteeus the actee side.\n")
	fmt.Fprintf(&b, "# dimensions: block x key [x subtype] x role, role in {actor,actee,observer}:\n")
	fmt.Fprintf(&b, "# M4b-1 collapsed this store's three authored KEY vocabularies (attacker/target/\n")
	fmt.Fprintf(&b, "# room, controller/controlled and self/room) the way M4a collapsed its three\n")
	fmt.Fprintf(&b, "# token ones. In the gradient and transition rows the SIDE field is a role too;\n")
	fmt.Fprintf(&b, "# the gradient state spelled `controlled` is authored data and keeps its name.\n")
	fmt.Fprintf(&b, "# Empty rows are authored silence.\n")
	fmt.Fprintf(&b, "# gradient_messages and transition_messages have NO Go reader today; those rows\n")
	fmt.Fprintf(&b, "# guard the data, not a render path.\n\n")

	render := func(s string) string {
		return narration.Substitute(s, positionControlStandins)
	}
	emitSelfRoom := func(key string, pair posSelfRoom) {
		fmt.Fprintf(&b, "%s|actor => %q\n", key, render(pair.Self))
		fmt.Fprintf(&b, "%s|observer => %q\n", key, render(pair.Room))
	}
	emitTriple := func(key string, tri posTriple) {
		fmt.Fprintf(&b, "%s|actor => %q\n", key, render(tri.Attacker))
		fmt.Fprintf(&b, "%s|actee => %q\n", key, render(tri.Target))
		fmt.Fprintf(&b, "%s|observer => %q\n", key, render(tri.Room))
	}

	if len(tpl.Gradient) == 0 {
		t.Fatal("gradient_messages parsed empty; the golden would be vacuous")
	}
	for _, side := range sortedMapKeys(tpl.Gradient) {
		for _, key := range sortedMapKeys(tpl.Gradient[side]) {
			emitSelfRoom(fmt.Sprintf("gradient|%s|%s", side, key), tpl.Gradient[side][key])
		}
	}

	if len(tpl.Transition) == 0 {
		t.Fatal("transition_messages parsed empty; the golden would be vacuous")
	}
	for _, side := range sortedMapKeys(tpl.Transition) {
		emitSelfRoom("transition|"+side, tpl.Transition[side])
	}

	if tpl.Stamina.Self == "" {
		t.Fatal("stamina_warning.self parsed empty; the golden would be vacuous")
	}
	emitSelfRoom("stamina", tpl.Stamina)

	if len(tpl.Submission) == 0 {
		t.Fatal("submission parsed empty; the golden would be vacuous")
	}
	for _, key := range sortedMapKeys(tpl.Submission) {
		node := tpl.Submission[key]
		if key == "opening" {
			var opening map[string]posTriple
			if err := node.Decode(&opening); err != nil {
				t.Fatalf("decode submission.opening: %v", err)
			}
			for _, sub := range sortedMapKeys(opening) {
				emitTriple("submission|opening|"+sub, opening[sub])
			}
			continue
		}
		var tri posTriple
		if err := node.Decode(&tri); err != nil {
			t.Fatalf("decode submission.%s: %v", key, err)
		}
		emitTriple("submission|"+key, tri)
	}

	return b.String()
}

// ---------------------------------------------------------------------
// Store 17: light notices (internal/lightnotice)
// ---------------------------------------------------------------------

// buildLightNoticesGolden freezes every line of every cause, transition and
// setting, every index, so a dropped, reordered or reworded line goes red.
func buildLightNoticesGolden(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(dogmudDataDir(t), "narration", "light-notices")
	groups, err := fileloader.LoadAllFlatFiles[string, *lightnotice.CauseGroup](dir)
	if err != nil {
		t.Fatalf("loading %s: %v", dir, err)
	}
	if len(groups) == 0 {
		t.Fatal("no light-notice files loaded; the golden would be vacuous")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# light-notices store snapshot (lighting plan 3d)\n")
	fmt.Fprintf(&b, "# dimensions: cause x transition x setting(any/outdoor/indoor) x index\n\n")
	for _, c := range lightnotice.Causes() {
		g := groups[string(c)]
		if g == nil {
			t.Fatalf("cause %q has no file", c)
		}
		for _, tr := range lightnotice.Transitions() {
			p := g.Transitions[tr]
			if p == nil {
				t.Fatalf("cause %q lost transition %q", c, tr)
			}
			for _, set := range []struct {
				name string
				pool []string
			}{{"any", p.Any}, {"outdoor", p.Outdoor}, {"indoor", p.Indoor}} {
				for i, text := range set.pool {
					fmt.Fprintf(&b, "%s|%s|%s|%d => %s\n", c, tr, set.name, i, text)
				}
			}
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------
// Store 16: special-move narration (internal/movenarration)
// ---------------------------------------------------------------------

func buildSpecialMovesGolden(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(dogmudDataDir(t), "narration", "special-moves")
	groups, err := fileloader.LoadAllFlatFiles[string, *movenarration.MoveNarrationGroup](dir)
	if err != nil {
		t.Fatalf("loading %s: %v", dir, err)
	}
	verbs := make([]string, 0, len(groups))
	for id := range groups {
		verbs = append(verbs, id)
	}
	sort.Strings(verbs)

	tokens := map[string]string{
		narration.TokenActor:        `<ansi fg="mobname">ACTOR</ansi>`,
		narration.TokenActee:        `<ansi fg="username">ACTEE</ansi>`,
		narration.TokenActorPlain:   `ACTOR`,
		narration.TokenActeePlain:   `ACTEE`,
		movenarration.TokenDamage:   `DAMAGE`,
		movenarration.TokenLabel:    `LABEL`,
		movenarration.TokenWith:     `WITH`,
		movenarration.TokenVerb:     `VERB`,
		movenarration.TokenWeapon:   `WEAPON`,
		movenarration.TokenExitName: `EXITNAME`,
		movenarration.TokenPosition: `POSITION`,
		movenarration.TokenItem:     `ITEM`,
	}

	var b strings.Builder
	for _, verb := range verbs {
		g := groups[verb]
		keys := make([]string, 0, len(g.Events))
		for k := range g.Events {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)
		for _, key := range keys {
			v, ok := g.Variants(movenarration.EventKey(key))
			if !ok {
				t.Fatalf("verb %q lost event %q between listing and lookup", verb, key)
			}
			// Every index, not just index 0: a golden that renders one index
			// cannot fail on a variant added or reordered behind it.
			for i := 0; i < v.Len(); i++ {
				r := narration.Render(v, tokens, narration.DefaultPicker, i)
				fmt.Fprintf(&b, "%s|%s|%d|actor= %s\n", verb, key, i, r.Actor)
				fmt.Fprintf(&b, "%s|%s|%d|actee= %s\n", verb, key, i, r.Actee)
				fmt.Fprintf(&b, "%s|%s|%d|observer= %s\n", verb, key, i, r.Observer)
				fmt.Fprintf(&b, "%s|%s|%d|remote_observer= %s\n", verb, key, i, r.ActeeObserver)
			}
		}
	}
	return b.String()
}
