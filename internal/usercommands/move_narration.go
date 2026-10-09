package usercommands

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
// The tags live at the CALL SITE rather than in this helper because they
// vary: kick tags its target as a username, shoot tags its target as a
// mobname.
//
// This mirrors internal/mobcommands/move_narration.go's struct of the same
// name, but the player side has a real Actor recipient where the mob side
// never does: the acting PLAYER has a client and receives their own personal
// line, so every call site's messaging.Audience carries Actor, ActorId and
// ActorName alongside Actee/ActeeId/ActeeName. That difference lives in the
// Audience literal at each call site, not in this struct.
type moveIdentities struct {
	Actor      string // ansi-tagged
	ActorPlain string
	Actee      string // ansi-tagged
	ActeePlain string
}

// moveCategories lets each rendered role ride its own messaging.Category.
//
// Each special move's personal lines ride the move's own category, the same
// as its room line (#449): CategorySystem never wraps (messaging.shouldWrap),
// so a long actor or actee line ran past 80 columns while the command's
// defence lines, already on the move category, wrapped. A Category decides a
// line's colour (messaging/pipeline.go applyCategoryColor) and whether it
// wraps; combat verbosity (messaging.Verbosity.Suppresses) is applied only to
// the round's combat drains in internal/hooks, never to SendTrio's seats, so
// a light-verbosity player still reads their own move. The struct stays
// because the roles can still differ (throw's actee-less events,
// RemoteObserver). sameMoveCategory below covers a call site whose lines
// share one category throughout.
type moveCategories struct {
	Actor          messaging.Category
	Actee          messaging.Category
	Observer       messaging.Category
	RemoteObserver messaging.Category
	// ObserverSound is what an observer who sees nothing hears of the
	// event, sent under the Observer category: a line that names nobody,
	// for an event heard as well as seen (a cast interrupt, #242 owner
	// ruling R4). Empty, the usual case, keeps it silent to them.
	ObserverSound string
}

// sameMoveCategory returns a moveCategories with cat on every role, for a
// call site whose pre-migration lines shared one category throughout.
func sameMoveCategory(cat messaging.Category) moveCategories {
	return moveCategories{Actor: cat, Actee: cat, Observer: cat, RemoteObserver: cat}
}

// renderMoveEvent renders one special-move event from the shipped store
// without delivering it.
//
// sendMoveEvent covers the ordinary case, where every role goes out as
// rendered. It is not enough for a channel-defended partial: the actor and
// actee lines still come from the store, but the observer line is replaced
// by the defence triad's ToRoom text when a defence actually fired. That
// call site needs the rendered roles to build its own Trio, so the render
// step is split out here rather than folded into sendMoveEvent.
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
func sendMoveEvent(verb string, event movenarration.EventKey, ids moveIdentities, aud messaging.Audience, cats moveCategories, extra map[string]string) {
	roles, ok := renderMoveEvent(verb, event, ids, extra)
	if !ok {
		return
	}
	messaging.SendTrio(messaging.Trio{
		Actor:          lineOrNone(cats.Actor, roles.Actor),
		Actee:          lineOrNone(cats.Actee, roles.Actee),
		Observer:       lineOrNone(cats.Observer, roles.Observer),
		RemoteObserver: lineOrNone(cats.RemoteObserver, roles.ActeeObserver),
		ObserverSound:  lineOrNone(cats.Observer, cats.ObserverSound),
	}, aud)
}

// lineOrNone keeps an unauthored role silent rather than sending an empty line.
func lineOrNone(cat messaging.Category, text string) messaging.Line {
	if text == "" {
		return messaging.NoLine
	}
	return messaging.Say(cat, text)
}
