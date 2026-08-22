package stats

import (
	"testing"

	"gopkg.in/yaml.v2"
)

// StatInfo.BaseAuthored records whether the YAML actually carried a `base:`
// key. Decoded with yaml.v2 because that is the library internal/fileloader
// uses, and the Unmarshaler interface differs between v2 and v3 -- a v3-shaped
// method compiles and is simply never called on the engine's real load path.
// The key is the only way to tell an explicit `base: 0` apart from an absent
// one. Species hydration keys on that distinction (U10b-0 Phase A): two mob
// stats fold to exactly zero, and without this they would silently hydrate back
// to their species baseline.
func TestUnmarshalYAML_RecordsWhetherBaseWasAuthored(t *testing.T) {
	cases := []struct {
		name         string
		yaml         string
		wantBase     int
		wantTraining int
		wantAuthored bool
	}{
		{"absent", "training: 12\n", 0, 12, false},
		{"explicit zero", "base: 0\n", 0, 0, true},
		{"explicit nonzero", "base: 140\n", 140, 0, true},
		{"explicit negative", "base: -15\n", -15, 0, true},
		{"both keys", "base: 100\ntraining: 5\n", 100, 5, true},
		{"empty mapping", "{}\n", 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var si StatInfo
			if err := yaml.Unmarshal([]byte(tc.yaml), &si); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if si.Base != tc.wantBase {
				t.Errorf("Base = %d, want %d", si.Base, tc.wantBase)
			}
			if si.Training != tc.wantTraining {
				t.Errorf("Training = %d, want %d", si.Training, tc.wantTraining)
			}
			if si.BaseAuthored != tc.wantAuthored {
				t.Errorf("BaseAuthored = %v, want %v", si.BaseAuthored, tc.wantAuthored)
			}
		})
	}
}

// The custom unmarshaler must not change how a whole Statistics block decodes,
// including the stats left out of it.
func TestUnmarshalYAML_NestedInStatistics(t *testing.T) {
	const doc = `
strength:
  base: 120
dexterity:
  training: 8
vitality:
  base: 0
`
	var s Statistics
	if err := yaml.Unmarshal([]byte(doc), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.Strength.Base != 120 || !s.Strength.BaseAuthored {
		t.Errorf("strength = %+v, want Base 120 authored", s.Strength)
	}
	if s.Dexterity.Training != 8 || s.Dexterity.BaseAuthored {
		t.Errorf("dexterity = %+v, want Training 8 and no authored base", s.Dexterity)
	}
	if s.Vitality.Base != 0 || !s.Vitality.BaseAuthored {
		t.Errorf("vitality = %+v, want an explicit base 0", s.Vitality)
	}
	if s.Charisma.BaseAuthored {
		t.Errorf("charisma was absent from the document but reports an authored base")
	}
}

// BaseAuthored is runtime-only. It must never reach a save file, or a mob
// template round-tripped through the web builder would start claiming an
// authored base for stats that never had one.
func TestBaseAuthored_IsNotSerialised(t *testing.T) {
	out, err := yaml.Marshal(StatInfo{Base: 7, BaseAuthored: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(out); got != "base: 7\n" {
		t.Errorf("marshalled %q, want %q", got, "base: 7\n")
	}
}
