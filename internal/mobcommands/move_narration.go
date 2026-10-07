package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
)

// moveIdentities carries the display forms of the two parties. The tagged
// forms are what the wording interpolates; the plain forms are what a
// possessive or a bare mention needs.
//
// The tags live at the CALL SITE rather than in this helper because they vary:
// kick tags its target as a username, shoot tags its target as a mobname.
type moveIdentities struct {
	Actor      string // ansi-tagged
	ActorPlain string
	Actee      string // ansi-tagged
	ActeePlain string
}

// renderMoveEvent renders one special-move event from the shipped store
// without delivering it.
//
// sendMoveEvent covers the ordinary case, where every role goes out as
// rendered. It is not enough for a channel-defended partial: the actee line
// still comes from the store, but the observer line is replaced by the
// defence triad's ToRoom text when a defence actually fired. That call site
// needs the rendered roles to build its own Trio, so the render step is
// split out here rather than folded into sendMoveEvent.
func renderMoveEvent(verb string, event movenarration.EventKey, ids moveIdentities, extra map[string]string) (narration.Roles, bool) {
	g := movenarration.GetMove(verb)
	if g == nil {
		mudlog.Error("movenarration", "error", "store not loaded", "verb", verb)
		return narration.Roles{}, false
	}
	v, ok := g.Variants(event)
	if !ok {
		mudlog.Error("movenarration", "error", "unknown event", "verb", verb, "event", event)
		return narration.Roles{}, false
	}
	tokens := map[string]string{
		narration.TokenActor:      ids.Actor,
		narration.TokenActorPlain: ids.ActorPlain,
		narration.TokenActee:      ids.Actee,
		narration.TokenActeePlain: ids.ActeePlain,
	}
	for k, val := range extra {
		tokens[k] = val
	}
	roles := narration.Render(v, tokens, narration.DefaultPicker)
	// "from beyond the {exitname}" reads "from somewhere above" for up (#430).
	if name := tokens[movenarration.TokenExitName]; name != "" {
		roles.Actor = exit.PhraseVerticalExits(roles.Actor, name)
		roles.Actee = exit.PhraseVerticalExits(roles.Actee, name)
		roles.Observer = exit.PhraseVerticalExits(roles.Observer, name)
		roles.ActeeObserver = exit.PhraseVerticalExits(roles.ActeeObserver, name)
	}
	return roles, true
}

// sendMoveEvent renders one special-move event from the shipped store and
// delivers it to every audience.
//
// It deliberately has no darkness branch. SendTrio hides each party's name
// from a reader who cannot make them out, judged by that reader's sight, so a
// hand-rolled anonymous twin would be a second, cruder mechanism on top of a
// working one.
func sendMoveEvent(verb string, event movenarration.EventKey, ids moveIdentities, aud messaging.Audience, cat messaging.Category, extra map[string]string) {
	roles, ok := renderMoveEvent(verb, event, ids, extra)
	if !ok {
		return
	}
	messaging.SendTrio(messaging.Trio{
		Actor:          lineOrNone(cat, roles.Actor),
		Actee:          lineOrNone(cat, roles.Actee),
		Observer:       lineOrNone(cat, roles.Observer),
		RemoteObserver: lineOrNone(cat, roles.ActeeObserver),
	}, aud)
}

// lineOrNone keeps an unauthored role silent rather than sending an empty line.
func lineOrNone(cat messaging.Category, text string) messaging.Line {
	if text == "" {
		return messaging.NoLine
	}
	return messaging.Say(cat, text)
}
