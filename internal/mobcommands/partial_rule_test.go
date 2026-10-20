package mobcommands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const specialMoveStoreDir = "../../_datafiles/world/dogmud/narration/special-moves"

// ─── The stores carry the tier the rule draws ──────────────────────────────

// missingPartialRoles reports, for one store, every role a "<prefix>hit"
// event carries that its "<prefix>partial" sibling does not, as
// "<partial event> lacks <role>". sendPartialMove sends every role of the
// partial event, so a role the hit tells and the partial does not would go
// silent on a partial (#631).
func missingPartialRoles(events map[movenarration.EventKey]*movenarration.EventMessages) []string {
	var out []string
	for key, ev := range events {
		k := string(key)
		if !strings.HasSuffix(k, "partial") || ev == nil {
			continue
		}
		hit := events[movenarration.EventKey(strings.TrimSuffix(k, "partial")+"hit")]
		if hit == nil {
			out = append(out, k+" has no hit sibling")
			continue
		}
		for _, r := range []struct {
			name      string
			hit, part []string
		}{
			{"actor", hit.Actor, ev.Actor},
			{"actee", hit.Actee, ev.Actee},
			{"observer", hit.Observer, ev.Observer},
		} {
			if len(r.hit) > 0 && len(r.part) == 0 {
				out = append(out, k+" lacks "+r.name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func TestMissingPartialRoles_CanFail(t *testing.T) {
	got := missingPartialRoles(map[movenarration.EventKey]*movenarration.EventMessages{
		"hit":     {Actee: []string{"a"}, Observer: []string{"o"}},
		"partial": {Actee: []string{"a"}},
	})
	require.Equal(t, []string{"partial lacks observer"}, got)
}

// partialVerbs lists the store each sendPartialMove call site in both
// command packages narrates from, read from the source so a new call site is
// covered without editing this test.
var partialCall = regexp.MustCompile(`sendPartialMove\("([a-z]+)"`)

func partialVerbs(t *testing.T) map[string]bool {
	t.Helper()
	verbs := map[string]bool{}
	for _, dir := range []string{".", "../usercommands"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			raw, err := os.ReadFile(f)
			require.NoError(t, err)
			for _, m := range partialCall.FindAllStringSubmatch(string(raw), -1) {
				verbs[m[1]] = true
			}
		}
	}
	return verbs
}

// #631, owner ruling 2026-10-19: every special-move store has, for each hit
// event, a partial event that tells every role the hit tells; and every move
// that narrates a partial through the rule has one.
func TestSpecialMoveStoresCarryEveryTierTheRuleDraws(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(specialMoveStoreDir, "*.yaml"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 12, "the guard would check too little")
	hasPartial := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		var g movenarration.MoveNarrationGroup
		require.NoError(t, yaml.Unmarshal(raw, &g), f)
		for _, miss := range missingPartialRoles(g.Events) {
			t.Errorf("%s: %s", filepath.Base(f), miss)
		}
		for key := range g.Events {
			if strings.HasSuffix(string(key), "partial") {
				hasPartial[g.MoveId] = true
			}
		}
	}
	verbs := partialVerbs(t)
	require.GreaterOrEqual(t, len(verbs), 12, "found only %v calling sendPartialMove", verbs)
	for verb := range verbs {
		if !hasPartial[verb] {
			t.Errorf("%s narrates a partial through the rule but its store has no partial event", verb)
		}
	}
}

// ─── Every call site goes through the rule ─────────────────────────────────

// handRolledPartial is the pre-#631 shape: a partial event rendered by hand
// so its room line could be swapped for the defence triad's.
var handRolledPartial = regexp.MustCompile(`renderMoveEvent\("([a-z]+)", [^\n]*partial`)

// partialExempt names the call sites that compose a partial by hand for a
// reason the rule does not cover, each with that reason.
var partialExempt = map[string]string{
	// shoot.yaml authors no observer role: a shot's room learns of it
	// through the fire and arrival events, and each target line is chosen
	// per reader (sneak, cross-room). Nothing is swapped.
	"shoot": "no observer role; per-target composition",
	// A firebomb's partial_hit authors only the thrower's line; the room
	// still reads the defence triad. Not a beast move, so outside the #631
	// ruling; reported to the owner as the one sibling left.
	"throw": "actor-only partial_hit; outside the #631 beast-move ruling",
}

func TestPartialMovesNarrateThroughTheRule(t *testing.T) {
	sites := 0
	for _, dir := range []string{".", "../usercommands"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			raw, err := os.ReadFile(f)
			require.NoError(t, err)
			src := string(raw)
			sites += strings.Count(src, "sendPartialMove(") - strings.Count(src, "func sendPartialMove(")
			for _, m := range handRolledPartial.FindAllStringSubmatch(src, -1) {
				if _, ok := partialExempt[m[1]]; !ok {
					t.Errorf("%s renders the %s partial by hand: narrate it with sendPartialMove", f, m[1])
				}
			}
		}
	}
	// 12 mob moves and 9 player moves (#631).
	require.GreaterOrEqual(t, sites, 21, "only %d sendPartialMove call sites", sites)
}

// ─── Behaviour: a partial reads the partial tier in every role ─────────────

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m|<[^>]*>")

func plainLine(s string) string {
	return strings.Join(strings.Fields(ansiEscape.ReplaceAllString(s, " ")), " ")
}

// partialVariants renders every variant of one role of verb's event with
// tokens, plain, so a delivered line can be matched against the whole pool.
func partialVariants(t *testing.T, verb string, event movenarration.EventKey, tokens map[string]string, role func(narration.Roles) string) []string {
	t.Helper()
	g := movenarration.GetMove(verb)
	require.NotNil(t, g)
	v, ok := g.Variants(event)
	require.True(t, ok)
	var out []string
	for i := 0; i < v.Len(); i++ {
		out = append(out, plainLine(role(narration.Render(v, tokens, nil, i))))
	}
	return out
}

func requireOneOf(t *testing.T, got []string, want []string, who string) {
	t.Helper()
	joined := plainLine(strings.Join(got, " "))
	for _, w := range want {
		if w != "" && strings.Contains(joined, w) {
			return
		}
	}
	t.Fatalf("%s read %q, not one of the partial lines %q", who, joined, want)
}

// A mob's rake that a defence blunted but did not stop: the defender and the
// room both read the rake store's partial tier, and the room does not read
// the defence triad (the shortage note is the defender's alone).
func TestMobPartialMove_DefenderAndRoomReadThePartialTier(t *testing.T) {
	require.NoError(t, movenarration.LoadFrom(specialMoveStoreDir))
	cleanup := seedAllRegistries()
	defer cleanup()

	mob, room := getTestMobAndRoom(t)
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	alice, bob := users.GetByUserId(1), users.GetByUserId(2)
	require.Equal(t, messaging.SightFull, messaging.ParticipantSight(bob.Character, room))
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)

	mobName := mob.Character.Name
	ids := moveIdentities{
		Actor:      fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mobName),
		ActorPlain: mobName,
		Actee:      fmt.Sprintf(`<ansi fg="username">%s</ansi>`, alice.Character.Name),
		ActeePlain: alice.Character.Name,
	}
	aud := messaging.Audience{ActorName: mobName, Actee: alice, ActeeId: alice.UserId,
		ActeeName: alice.Character.Name, Room: room}
	tokens := map[string]string{movenarration.TokenDamage: "a scratch"}

	sendPartialMove("rake", "partial", ids, aud, messaging.CategoryHitNaturalSharp, tokens, "")

	render := map[string]string{
		narration.TokenActor: ids.Actor, narration.TokenActorPlain: ids.ActorPlain,
		narration.TokenActee: ids.Actee, narration.TokenActeePlain: ids.ActeePlain,
		movenarration.TokenDamage: "a scratch",
	}
	requireOneOf(t, events.DrainQueuedMessagesForTest(1),
		partialVariants(t, "rake", "partial", render, func(r narration.Roles) string { return r.Actee }), "the defender")
	requireOneOf(t, events.DrainQueuedMessagesForTest(2),
		partialVariants(t, "rake", "partial", render, func(r narration.Roles) string { return r.Observer }), "the room")
}
