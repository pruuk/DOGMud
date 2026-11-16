package rooms

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
)

// #442: the one statement of "a hidden container is visible only once this
// character has discovered it in this room".
func TestContainerVisibleTo(t *testing.T) {
	r := &Room{RoomId: 4420, Containers: map[string]Container{
		"chest":    {},
		"cache":    {Hidden: true},
		"cupboard": {},
	}}
	finder := characters.New()
	stranger := characters.New()
	finder.AddDiscovery(4420, "cache")

	cases := []struct {
		name string
		char *characters.Character
		want bool
	}{
		{"chest", nil, true},
		{"chest", stranger, true},
		{"cache", nil, false},
		{"cache", stranger, false},
		{"cache", finder, true},
		{"nothing", finder, false},
	}
	for _, c := range cases {
		if got := r.ContainerVisibleTo(c.char, c.name); got != c.want {
			t.Errorf("ContainerVisibleTo(%v, %q) = %v, want %v", c.char != nil, c.name, got, c.want)
		}
	}

	if got := r.FindVisibleContainerByName(stranger, "cache"); got != "" {
		t.Errorf("a stranger finds the undiscovered cache: %q", got)
	}
	if got := r.FindVisibleContainerByName(finder, "cache"); got != "cache" {
		t.Errorf("the finder finds the discovered cache: %q", got)
	}
	if got := r.FindVisibleContainerByName(nil, "chest"); got != "chest" {
		t.Errorf("anyone finds the plain chest: %q", got)
	}

	// The discovery belongs to this room: the same name elsewhere is not found.
	elsewhere := &Room{RoomId: 4421, Containers: map[string]Container{"cache": {Hidden: true}}}
	if elsewhere.ContainerVisibleTo(finder, "cache") {
		t.Error("a discovery in room 4420 shows a hidden cache in room 4421")
	}
}
