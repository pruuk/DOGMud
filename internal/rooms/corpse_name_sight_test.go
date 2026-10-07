package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

// Corpse.NameAt is the one hidden form of a corpse's name (#428 review): the
// ground listing, look, loot and get all write it for a reader below clear
// sight, so a mob's corpse, a player's and a special one read alike.
func TestCorpse_NameAt(t *testing.T) {
	mob := Corpse{MobId: 12}
	mob.Character.Name = "City Beggar"
	special := Corpse{MobId: 13, CorpseName: "wreck of the Grey Wagon"}
	special.Character.Name = "Grey Wagon"

	assert.Equal(t, "City Beggar corpse", mob.NameAt(messaging.SightFull))
	assert.Equal(t, "wreck of the Grey Wagon", special.NameAt(messaging.SightFull))

	for _, c := range []Corpse{mob, special} {
		got := plainCorpseText(c.NameAt(messaging.SightShapes))
		assert.Equal(t, "corpse of a figure", got)
		got = plainCorpseText(c.NameAt(messaging.SightNone))
		assert.Equal(t, "corpse of something", got)
	}
}

// ObservedName is the form a per-reader name hider rewrites: hiding the dead
// one's name in it reads as English.
func TestCorpse_ObservedNameHidesToEnglish(t *testing.T) {
	mob := Corpse{MobId: 12}
	mob.Character.Name = "City Beggar"
	assert.Equal(t, "corpse of City Beggar", mob.ObservedName())
	assert.Equal(t, "corpse of a figure", plainCorpseText(
		messaging.HideNames(mob.ObservedName(), []string{mob.Character.Name}, messaging.SightShapes)))
}

func plainCorpseText(s string) string {
	out := []rune{}
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			out = append(out, r)
		}
	}
	return string(out)
}
