package seeders

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/goals"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

const (
	staleGenericMob = 97001
	staleUniqueMob  = 97002
	staleZone       = "Stale Goal Zone"
	staleRoom       = 97100
)

// A goal file as it stands on a server from before the 2026-11-01 ruling: the
// type's file holds a theft grudge at victim and witness priority, beside
// everything that must survive. g1 is current.
const staleGoalFile = `mob_id: %ID%
next_goal_id: 9
current_goal_id: g1
current_since_round: 5
last_switch_round: 5
seeded_from_archetype: true
goals:
- id: g1
  type: revenge-mob
  priority: 90
  params: {target_kind: player, target_id: 7}
  created_at: 2026-10-01T00:00:00Z
- id: g2
  type: revenge-mob
  priority: 60
  params: {target_kind: player, target_id: 8}
  created_at: 2026-10-01T00:00:00Z
- id: g3
  type: revenge-mob
  priority: 75
  params: {target_kind: player, target_id: 9}
  created_at: 2026-10-01T00:00:00Z
- id: g4
  type: revenge-mob
  priority: 50
  params: {target_kind: player, target_id: 10}
  created_at: 2026-10-01T00:00:00Z
- id: g5
  type: revenge-mob
  priority: 85
  params: {target_kind: mob, target_id: 11}
  created_at: 2026-10-01T00:00:00Z
- id: g6
  type: revenge-mob
  priority: 90
  params: {target_kind: mob, target_id: 12}
  created_at: 2026-10-01T00:00:00Z
- id: g7
  type: wealth-gold
  priority: 90
  params: {target: 100}
  created_at: 2026-10-01T00:00:00Z
- id: g8
  type: upgrade-gear
  priority: 60
  created_at: 2026-10-01T00:00:00Z
- id: g9
  type: befriend
  priority: 90
  params: {target_kind: player, target_id: 13}
  created_at: 2026-10-01T00:00:00Z
`

func writeStaleGoalFile(t *testing.T, dir string, mobId int, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, itoaForTest(mobId)+"-"+name+".yaml")
	body := strings.ReplaceAll(staleGoalFile, "%ID%", itoaForTest(mobId))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoaForTest(n int) string { return userIdAsKey(n) }

func staleGoalWorld(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "goals")
	prev, had := os.LookupEnv("DOGMUD_GOALS_DIR_OVERRIDE")
	os.Setenv("DOGMUD_GOALS_DIR_OVERRIDE", dir)
	t.Cleanup(func() {
		if had {
			os.Setenv("DOGMUD_GOALS_DIR_OVERRIDE", prev)
		} else {
			os.Unsetenv("DOGMUD_GOALS_DIR_OVERRIDE")
		}
	})
	goals.ClearCache()
	t.Cleanup(goals.ClearCache)

	generic := &mobs.Mob{MobId: staleGenericMob, Zone: staleZone}
	generic.Character.Name = "Scree Scavenger"
	unique := &mobs.Mob{MobId: staleUniqueMob, Zone: staleZone}
	unique.Character.Name = "Warden Esk"
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		staleGenericMob: generic,
		staleUniqueMob:  unique,
	}, map[int]*mobs.Mob{}))

	t.Cleanup(rooms.SeedTemplateSpawnsForTest(staleRoom, staleGenericMob, staleUniqueMob))
	t.Cleanup(dialogue.InstallForTest(&dialogue.DialogueFile{MobId: staleUniqueMob, Zone: staleZone}))
	return dir
}

func goalIdsOf(gs []*goals.Goal) map[string]bool {
	out := map[string]bool{}
	for _, g := range gs {
		out[g.Id] = true
	}
	return out
}

// On load, a generic type's file loses its theft grudges and nothing else, the
// selection state pointing at one is cleared, and the file on disk says so.
func TestStaleTheftGoals_LeaveAGenericTypesFileOnLoad(t *testing.T) {
	dir := staleGoalWorld(t)
	path := writeStaleGoalFile(t, dir, staleGenericMob, "scree_scavenger")

	got := goalIdsOf(goals.GoalsOf(staleGenericMob, "scree_scavenger"))

	for _, stale := range []string{"g1", "g2"} {
		if got[stale] {
			t.Errorf("%s, a theft grudge, survived the load", stale)
		}
	}
	for _, kept := range []string{"g3", "g4", "g5", "g6", "g7", "g8", "g9"} {
		if !got[kept] {
			t.Errorf("%s is not a theft grudge and must survive the load", kept)
		}
	}
	if cur := goals.CurrentGoalOf(staleGenericMob, "scree_scavenger"); cur != nil && cur.Id == "g1" {
		t.Errorf("the removed grudge is still current")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"- id: g1", "- id: g2", "current_goal_id: g1"} {
		if strings.Contains(string(body), line) {
			t.Errorf("the saved file still holds %q", line)
		}
	}
	if !strings.Contains(string(body), "- id: g3") {
		t.Errorf("the saved file lost g3")
	}
}

func TestStaleTheftGoals_AOneOfAKindKeepsEveryGoal(t *testing.T) {
	dir := staleGoalWorld(t)
	writeStaleGoalFile(t, dir, staleUniqueMob, "warden_esk")

	got := goalIdsOf(goals.GoalsOf(staleUniqueMob, "warden_esk"))

	for _, kept := range []string{"g1", "g2", "g3", "g4", "g5", "g6", "g7", "g8", "g9"} {
		if !got[kept] {
			t.Errorf("%s was removed from a one-of-a-kind's file", kept)
		}
	}
}

// A template that cannot be read keeps its goals: no judgement without the data.
func TestStaleTheftGoals_AnUnknownTemplateKeepsEveryGoal(t *testing.T) {
	dir := staleGoalWorld(t)
	writeStaleGoalFile(t, dir, 97999, "nobody")
	if got := goals.GoalsOf(97999, "nobody"); len(got) != 9 {
		t.Errorf("goals kept = %d, want all 9", len(got))
	}
}

// The rule the census rests on, spawn and dialogue both, with each half tested
// alone: a sole spawn with no dialogue and a talker with many spawns are generic.
func TestIsOneOfAKind(t *testing.T) {
	t.Cleanup(rooms.SeedTemplateSpawnsForTest(97200, 97300, 97301, 97302))
	t.Cleanup(rooms.SeedTemplateSpawnsForTest(97201, 97302))
	t.Cleanup(dialogue.InstallForTest(&dialogue.DialogueFile{MobId: 97300, Zone: staleZone}))
	t.Cleanup(dialogue.InstallForTest(&dialogue.DialogueFile{MobId: 97302, Zone: staleZone}))
	t.Cleanup(dialogue.InstallForTest(&dialogue.DialogueFile{MobId: 97303, Zone: staleZone}))

	for _, c := range []struct {
		name string
		id   int
		want bool
	}{
		{"one spawn and a dialogue file", 97300, true},
		{"one spawn, silent", 97301, false},
		{"two spawning rooms and a dialogue file", 97302, false},
		{"a dialogue file, never spawned", 97303, false},
	} {
		if got := isOneOfAKind(c.id, staleZone); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
