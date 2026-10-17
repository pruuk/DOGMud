package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/assert"
)

// #454: a typed name resolves only at full sight. These pin AimBySight, the
// rule every command that names a creature shares with cast. The scene is
// castSightScene's: Caster 7811, Witness 7812 and Other 7813, in "city" (lit)
// or "cave" (pitch dark; infrared makes it shapes only).

func TestAimBySight_ClearSightAdmitsTheNameAsTyped(t *testing.T) {
	a, room := castSightScene(t, "city")
	name, refusal := AimBySight(a.GetCharacter(), 7811, room, "witness", "kick")
	assert.Equal(t, "", refusal)
	assert.Equal(t, "witness", name)
}

func TestAimBySight_ShapesHintsATypedNameAndNamesTheVerb(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	_, refusal := AimBySight(a.GetCharacter(), 7811, room, "witness", "kick")
	assert.Contains(t, refusal, "You can only make out shapes here.")
	assert.Contains(t, refusal, `<ansi fg="command">kick figure</ansi>`)
	assert.Contains(t, refusal, `<ansi fg="command">kick 2.figure</ansi>`)
	assert.NotContains(t, strings.ToLower(refusal), "witness")
}

func TestAimBySight_ShapesResolvesAShapeToItsFigure(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	name, aimRefusal := AimBySight(a.GetCharacter(), 7811, room, "2.shape", "kick")
	assert.Equal(t, "", aimRefusal)
	assert.Equal(t, "@7813", name, "figures are the other players in room order")

	name, aimRefusal = AimBySight(a.GetCharacter(), 7811, room, "4.shape", "kick")
	assert.Equal(t, refusal.NotHereCreatureLine, aimRefusal, "there are only two figures")
	assert.Equal(t, "4.shape", name)
}

// The word the room shows a figure by is the word that aims at it (2026-10-17
// pre-deploy playtest: the room read "Also here: a figure" while only
// `shape` resolved, and the hint named a word the player never saw).
func TestAimBySight_ShapesResolvesTheWordTheRoomShows(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)

	shown := messaging.UnseenNoun(messaging.SightShapes)
	assert.True(t, strings.HasSuffix(shown, " "+messaging.FigureWord),
		"the room roster %q must end in the aim word %q", shown, messaging.FigureWord)

	for _, tc := range []struct{ typed, want string }{
		{"figure", "@7812"},
		{"Figure", "@7812"},
		{"2.figure", "@7813"},
		{"figure#2", "@7813"},
		{"shape", "@7812"},
	} {
		name, aimRefusal := AimBySight(a.GetCharacter(), 7811, room, tc.typed, "consider")
		assert.Equal(t, "", aimRefusal, "%q", tc.typed)
		assert.Equal(t, tc.want, name, "%q", tc.typed)
	}

	_, hint := AimBySight(a.GetCharacter(), 7811, room, "witness", "consider")
	assert.Contains(t, hint, `<ansi fg="command">consider figure</ansi>`)
	assert.Contains(t, hint, `<ansi fg="command">consider 2.figure</ansi>`)
	assert.Contains(t, AimShapesHint(""), `<ansi fg="command">figure</ansi>`)
}

// With no sight the figure word is refused as the shape word is.
func TestAimBySight_NoSightRefusesTheFigureWord(t *testing.T) {
	a, room := castSightScene(t, "cave")
	for _, typed := range []string{"figure", "2.figure", "all.figure"} {
		_, aimRefusal := AimBySight(a.GetCharacter(), 7811, room, typed, "kick")
		assert.Equal(t, refusal.NothingToAimLine, aimRefusal, "%q", typed)
	}
}

// An id form is what a shape becomes, and what party auto-assist types.
func TestAimBySight_ShapesAdmitsAnIdForm(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	for _, id := range []string{"@7812", "#100"} {
		name, refusal := AimBySight(a.GetCharacter(), 7811, room, id, "attack")
		assert.Equal(t, "", refusal, "%q", id)
		assert.Equal(t, id, name)
	}
}

func TestAimBySight_NoSightResolvesNothing(t *testing.T) {
	a, room := castSightScene(t, "cave")
	for _, tc := range []struct{ name, want string }{
		{"witness", refusal.NotHereCreatureLine},
		{"@7812", refusal.NotHereCreatureLine},
		{"shape", refusal.NothingToAimLine},
		{"2.shape", refusal.NothingToAimLine},
		{"all.shape", refusal.NothingToAimLine},
		{"", refusal.NothingToAimLine},
	} {
		_, aimRefusal := AimBySight(a.GetCharacter(), 7811, room, tc.name, "kick")
		assert.Equal(t, tc.want, aimRefusal, "%q", tc.name)
	}
}

// The 80-column rule: every line of the hint fits, and a verb too long for
// the second line drops out of it.
func TestAimShapesHint_FitsEightyColumns(t *testing.T) {
	for _, verb := range []string{"kick", "give iron longsword", "cast empathic-shroud", strings.Repeat("x", 40), ""} {
		hint := AimShapesHint(verb)
		for _, line := range strings.Split(hint, "\n") {
			plain := aimTestTag.ReplaceAllString(line, "")
			assert.LessOrEqual(t, len([]rune(plain)), 80, "verb %q: %q", verb, plain)
		}
	}
	assert.Contains(t, AimShapesHint(strings.Repeat("x", 40)), "in place of a name")
}

var aimTestTag = regexp.MustCompile(`<[^>]*>`)
