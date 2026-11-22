package characters

import "testing"

// #781, owner rulings 1, 2 and 13: a secret exit is known once found, and
// KnowsSecretExit is the one rule every exit list and every mover asks. It
// reads the same discovery record `search` writes.
func TestKnowsSecretExit_OnlyTheExitFoundInTheRoomItWasFound(t *testing.T) {
	c := &Character{}
	if c.KnowsSecretExit(10, "west") {
		t.Fatal("a fresh character knows no secret exit")
	}
	c.LearnSecretExit(10, "west")
	if !c.KnowsSecretExit(10, "west") {
		t.Fatal("a learned secret exit must be known")
	}
	if !c.HasDiscovery(10, SecretExitDiscoveryKey("west")) {
		t.Error("LearnSecretExit must write the record search writes")
	}
	if c.KnowsSecretExit(10, "north") {
		t.Error("learning west must not reveal north")
	}
	if c.KnowsSecretExit(11, "west") {
		t.Error("learning west in room 10 must not reveal west in room 11")
	}
	c.LearnSecretExit(10, "west")
	if got := len(c.Discoveries[10]); got != 1 {
		t.Errorf("learning twice stored %d records, want 1", got)
	}
}

func TestKnowsSecretExit_NilCharacterKnowsNone(t *testing.T) {
	var c *Character
	if c.KnowsSecretExit(10, "west") {
		t.Error("a nil character knows no secret exit")
	}
}
