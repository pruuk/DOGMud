package actions

import (
	"regexp"
	"strings"
	"testing"

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
	assert.Contains(t, refusal, `<ansi fg="command">kick shape</ansi>`)
	assert.Contains(t, refusal, `<ansi fg="command">kick 2.shape</ansi>`)
	assert.NotContains(t, strings.ToLower(refusal), "witness")
}

func TestAimBySight_ShapesResolvesAShapeToItsFigure(t *testing.T) {
	a, room := castSightScene(t, "cave")
	giveCasterInfrared(t, a)
	name, refusal := AimBySight(a.GetCharacter(), 7811, room, "2.shape", "kick")
	assert.Equal(t, "", refusal)
	assert.Equal(t, "@7813", name, "figures are the other players in room order")

	name, refusal = AimBySight(a.GetCharacter(), 7811, room, "4.shape", "kick")
	assert.Equal(t, AimNotHereLine, refusal, "there are only two figures")
	assert.Equal(t, "4.shape", name)
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
		{"witness", AimNotHereLine},
		{"@7812", AimNotHereLine},
		{"shape", AimNothingLine},
		{"2.shape", AimNothingLine},
		{"all.shape", AimNothingLine},
		{"", AimNothingLine},
	} {
		_, refusal := AimBySight(a.GetCharacter(), 7811, room, tc.name, "kick")
		assert.Equal(t, tc.want, refusal, "%q", tc.name)
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
