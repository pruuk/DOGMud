package messaging

import "testing"

// #455: shipped idle emotes ("emote stares at the ground between his
// feet") print with no full stop. CategoryMobEmote keeps its own casing,
// articles and words, and takes only the end-punctuation stage.
func TestNormalizeMobEmoteGetsOnlyEndPunctuation(t *testing.T) {
	in := `<ansi fg="mobname">Beggar Oswin</ansi> <ansi fg="137">stares at the the ground between his feet</ansi>`
	want := `<ansi fg="mobname">Beggar Oswin</ansi> <ansi fg="137">stares at the the ground between his feet.</ansi>`
	if got := Normalize(CategoryMobEmote, in); got != want {
		t.Errorf("mob emote: got %q, want %q", got, want)
	}
	// Lower-case start and "a" before a vowel stay as authored.
	if got := Normalize(CategoryMobEmote, "a eel twitches"); got != "a eel twitches." {
		t.Errorf("mob emote must skip every stage but end punctuation, got %q", got)
	}
	// Idempotent, and an authored stop is not doubled.
	if got := Normalize(CategoryMobEmote, "It nods."); got != "It nods." {
		t.Errorf("mob emote double-punctuated: %q", got)
	}
}

// A player types their own emote: the server never rewrites it.
func TestNormalizePlayerEmoteStaysExempt(t *testing.T) {
	in := `<ansi fg="username">Ordel</ansi> <ansi fg="137">waves</ansi>`
	if got := Normalize(CategoryEmote, in); got != in {
		t.Errorf("player emote rewritten: %q", got)
	}
}
