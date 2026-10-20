package rooms

import "github.com/GoMudEngine/GoMud/internal/mobs"

// NpcMarker returns the talk marker a viewer sees before mob's name. It is
// installed by internal/conversation (which can build the viewer's dialogue
// state); nil means no markers.
var NpcMarker func(viewerUserId int, mob *mobs.Mob) string
