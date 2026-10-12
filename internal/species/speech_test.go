package species

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestSpeechDefaultsToTrueWhenAbsent(t *testing.T) {
	var s Species
	if err := yaml.Unmarshal([]byte("speciesid: 900\nname: test\n"), &s); err != nil {
		t.Fatal(err)
	}
	if !s.CanSpeak() {
		t.Fatal("a species with no speech key must be able to speak")
	}
}

func TestSpeechFalseIsRead(t *testing.T) {
	var s Species
	if err := yaml.Unmarshal([]byte("speciesid: 901\nname: test\nspeech: false\n"), &s); err != nil {
		t.Fatal(err)
	}
	if s.CanSpeak() {
		t.Fatal("speech: false must make CanSpeak false")
	}
}

func TestSpeechNilSpeciesSpeaks(t *testing.T) {
	var s *Species
	if !s.CanSpeak() {
		t.Fatal("a nil species must read as able to speak")
	}
}

// The shipped dogmud roster: beasts and constructs have no voice, the
// speaking peoples do. Reads the files directly (see dogmudSpecies).
func TestShippedSpeechRoster(t *testing.T) {
	roster := dogmudSpecies(t)
	silent := map[int]string{2: "canine", 3: "bear", 16: "slime", 20: "orb", 30: "skeleton", 39: "fire_elemental", 45: "automaton"}
	for id, name := range silent {
		sp := roster[id]
		if sp == nil {
			t.Fatalf("species %d (%s) missing from the roster", id, name)
		}
		if sp.CanSpeak() {
			t.Errorf("species %d (%s) should have speech: false", id, name)
		}
	}
	speaking := map[int]string{0: "ghostly_spirit", 1: "human", 4: "troll", 5: "goblin", 32: "wraith", 33: "spectre", 34: "vampire", 99: "ascended"}
	for id, name := range speaking {
		sp := roster[id]
		if sp == nil {
			t.Fatalf("species %d (%s) missing from the roster", id, name)
		}
		if !sp.CanSpeak() {
			t.Errorf("species %d (%s) must be able to speak", id, name)
		}
	}
}
