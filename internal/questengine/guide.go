package questengine

import "github.com/GoMudEngine/GoMud/internal/mapper"

// GuideLine is the one destination line (#375 phase 4, spec section 4): the
// `quest` command prints it, and the GMCP quest payload carries it as `guide`
// so the web quest panel can say the same words. The web client does not read
// `guide` yet; it shows the line from phase 4 slice 2. Plain text: the panel
// will escape markup.
func GuideLine(d Destination) string {
	switch d.Kind {
	case DestNone:
		return ""
	case DestAnywhere:
		return "You can do this anywhere."
	case DestUnknown:
		return "Destination unknown."
	}
	if d.Inside {
		switch {
		case d.Label != "":
			return "You are in " + d.Label + "."
		case d.Fallback && d.Npc != "":
			return "Usually found around here: " + d.Npc + "." // names carry no article, so they follow a colon
		}
		return "You are where you need to be."
	}
	if d.NextDir == "" {
		return "You can't find a way there from here."
	}
	if mapper.IsCompassDirection(d.NextDir) {
		return "Head " + d.NextDir + "."
	}
	return "Take the " + d.NextDir + "."
}
