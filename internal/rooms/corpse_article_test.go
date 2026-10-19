package rooms

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

// #572: a mob named with its own article ("A Culvert Slime") once left a
// corpse reading "4 A Culvert Slime corpses" and "the corpse of A Culvert
// Slime". The listing and DisplayName drop the name's article, since a count
// or "the" stands before every corpse; ObservedName lowercases it.
func TestCorpse_MobNameArticle(t *testing.T) {
	slime := Corpse{MobId: 9564}
	slime.Character.Name = "A Culvert Slime"

	assert.Equal(t, "Culvert Slime corpse", slime.DisplayName())
	assert.Equal(t, "corpse of a Culvert Slime", slime.ObservedName())
	// The hider is handed the data spelling and must still hide the name.
	assert.Equal(t, "corpse of a figure", plainCorpseText(
		messaging.HideNames(slime.ObservedName(), []string{slime.Character.Name}, messaging.SightShapes)))
}

func TestGetDetails_CorpseCountDropsMobArticle(t *testing.T) {
	r, viewer := detailsSightRoom(t, "cave")
	orig := r.Corpses
	t.Cleanup(func() { r.Corpses = orig })
	slime := Corpse{MobId: 9564}
	slime.Character.Name = "A Culvert Slime"
	r.Corpses = []Corpse{slime, slime}
	lamp := 90
	r.Lamp = &lamp

	joined := strings.Join(GetDetails(r, viewer).VisibleCorpses, " ")
	want := `<ansi fg="mob-corpse">2 Culvert Slime corpses</ansi>`
	if !strings.Contains(joined, want) {
		t.Errorf("corpses %q are missing %q", joined, want)
	}

	// The shown name is a name that reaches the corpse.
	if idx := r.FindCorpseIndex("culvert slime corpse", viewer.Character); idx < 0 {
		t.Errorf("FindCorpseIndex could not reach the corpse by its shown name")
	}
}
