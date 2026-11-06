package gmcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// TestMobEditor_ItemDropChanceKeepsAbsentApartFromZero pins #585 through the
// builder: a mob with no itemdropchance key goes out as null and a save that
// echoes it back keeps the key absent (the default, every worn item drops),
// while an authored 0 goes out as 0 and stays a set 0 (never). Before the fix
// the wire field was a plain int, so an editor save wrote 0 for an absent key.
func TestMobEditor_ItemDropChanceKeepsAbsentApartFromZero(t *testing.T) {
	zero := 0
	for _, c := range []struct {
		name     string
		chance   *int
		wireWant string
		wantSet  bool
	}{
		{"absent", nil, `"itemDropChance":null`, false},
		{"explicit zero", &zero, `"itemDropChance":0`, true},
	} {
		w := newFakeMobWorld()
		base := &mobs.Mob{MobId: 90585, Zone: "Testzone", ItemDropChance: c.chance}
		base.Character.Name = "Drop Probe"
		base.Character.Description = "d"
		base.Character.SpeciesId = 1
		w.specs[90585] = base

		wire, err := json.Marshal(mobToReq(base))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(wire), c.wireWant) {
			t.Fatalf("%s: the editor payload must carry %s, got %s", c.name, c.wireWant, wire)
		}
		var req mobUpdateReq
		if err := json.Unmarshal(wire, &req); err != nil {
			t.Fatal(err)
		}
		if res := buildMobUpdate(w.deps(), req); !res.Ok {
			t.Fatalf("%s: save should succeed, got %+v", c.name, res)
		}
		got := w.saved[0]
		if (got.ItemDropChance != nil) != c.wantSet || (c.wantSet && got.EquippedDropChance() != 0) {
			t.Fatalf("%s: after an editor save set=%v chance=%d, want set=%v",
				c.name, got.ItemDropChance != nil, got.EquippedDropChance(), c.wantSet)
		}
	}
}
