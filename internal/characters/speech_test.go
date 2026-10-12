package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/species"
)

const (
	speechSpeakerId   = 9401
	speechBeastId     = 9402
	speechSilencedCid = 9403
)

func TestSpeechState(t *testing.T) {
	no := false
	species.SetSpeciesForTest(t, map[int]*species.Species{
		speechSpeakerId: {SpeciesId: speechSpeakerId, Name: "speaker"},
		speechBeastId:   {SpeciesId: speechBeastId, Name: "beast", Speech: &no},
	})
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		speechSilencedCid: {ConditionId: speechSilencedCid, Name: "Test Silenced", Flags: []conditions.Flag{conditions.Silenced}},
	}))

	cases := []struct {
		name     string
		species  int
		silenced bool
		want     SpeechState
	}{
		{"speaking species, no flag", speechSpeakerId, false, Speaks},
		{"speechless species", speechBeastId, false, Speechless},
		{"speaking species, silenced", speechSpeakerId, true, Silenced},
		{"speechless species, silenced: silence wins", speechBeastId, true, Silenced},
		{"unknown species id", 987654, false, Speaks},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New()
			c.SpeciesId = tc.species
			if tc.silenced {
				if err := c.AddCondition(speechSilencedCid, true); err != nil {
					t.Fatalf("applying silence: %v", err)
				}
			}
			if got := c.SpeechState(); got != tc.want {
				t.Fatalf("SpeechState() = %v, want %v", got, tc.want)
			}
		})
	}
}
