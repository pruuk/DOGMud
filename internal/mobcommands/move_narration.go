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
//
// observerSound is what an observer who sees nothing hears of the event,
// sent under cat: a line that names nobody, for an event heard as well as
// seen (a cast interrupt, #242 owner ruling R4). It mirrors the player
// side's moveCategories.ObserverSound. Omitted, the usual case, the event
// stays silent to them.
func sendMoveEvent(verb string, event movenarration.EventKey, ids moveIdentities, aud messaging.Audience, cat messaging.Category, extra map[string]string, observerSound ...string) {
	roles, ok := renderMoveEvent(verb, event, ids, extra)
	if !ok {
		return
	}
	sound := messaging.NoLine
	if len(observerSound) > 0 {
		sound = lineOrNone(cat, observerSound[0])
	}
	messaging.SendTrio(messaging.Trio{
		Actor:          lineOrNone(cat, roles.Actor),
		Actee:          lineOrNone(cat, roles.Actee),
		Observer:       lineOrNone(cat, roles.Observer),
		RemoteObserver: lineOrNone(cat, roles.ActeeObserver),
		ObserverSound:  sound,
	}, aud)
}

// sendMoveRolesJudgedBefore delivers rendered move roles as sendMoveEvent
// does, except that the actee's line is judged at acteeSightBefore, the sight
// the actee had before the move changed it, rather than at their sight now.
// Owner rule (2026-10-05): a line announcing a change is judged against the
// state before it resolves. Smother blinds its target in the same tick as the
// hit, so judged now, a sighted target would read "Something pours into your
// eyes" from a mob they watched strike.
//
// The actee line is hidden here, the sanctioned exception acteeDefenceLine
// (skill_move_defence.go) also uses: SendTrio is handed text already hidden
// by the reader's sight verdict, and that send carries NoName for the actor
// so SendTrio does not judge it a second time at the new sight. Every other
// role goes through SendTrio as usual, each observer judged by their own
// sight now (the move changed the actee's sight, not theirs).
func sendMoveRolesJudgedBefore(roles narration.Roles, aud messaging.Audience, cat messaging.Category, acteeSightBefore messaging.SightDecision, observerSound ...string) {
	if roles.Actee != "" {
		acteeAud := aud
		acteeAud.ActorName = messaging.NoName
		messaging.SendTrio(messaging.Trio{
			Actor:    messaging.NoLine,
			Actee:    messaging.Say(cat, messaging.HideNames(roles.Actee, []string{aud.ActorName}, acteeSightBefore)),
			Observer: messaging.NoLine,
		}, acteeAud)
	}
	sound := messaging.NoLine
	if len(observerSound) > 0 {
		sound = lineOrNone(cat, observerSound[0])
	}
	messaging.SendTrio(messaging.Trio{
		Actor:          lineOrNone(cat, roles.Actor),
		Actee:          messaging.NoLine,
		Observer:       lineOrNone(cat, roles.Observer),
		RemoteObserver: lineOrNone(cat, roles.ActeeObserver),
		ObserverSound:  sound,
	}, aud)
}

// lineOrNone keeps an unauthored role silent rather than sending an empty line.
func lineOrNone(cat messaging.Category, text string) messaging.Line {
	if text == "" {
		return messaging.NoLine
	}
	return messaging.Say(cat, text)
}
