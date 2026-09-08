package messaging

// Line is one audience's view of an event: what they are told, and under which
// category.
//
// THE CATEGORY RIDES ON THE LINE, NOT ON THE TRIO, because the three roles of
// one event do not share one. Counted across the twelve player-side special
// move verbs on 2026-09-08: ten send their personal lines as CategorySystem
// and the room line as something verb-specific (CategoryBash, CategoryKick,
// CategoryTrip, CategoryGrappleFlow, CategoryHitNaturalSharp); shoot uses four
// categories on the personal side; throw uses three on each side.
//
// A single-Category seam would have silently recategorised two verbs while
// claiming to change nothing. Category feeds the verbosity suppression
// allowlists in verbosity.go, so that reaches any player not on full
// verbosity.
type Line struct {
	Text string
	Cat  Category
}

// Say builds a Line. It exists so call sites read as prose rather than as
// struct literals.
func Say(cat Category, text string) Line { return Line{Text: text, Cat: cat} }

// NoLine marks a viewpoint that deliberately has nothing to say.
//
// It is the zero Line, spelled out so the guard can tell a considered absence
// from a forgotten one. Every defect in the M1 viewpoint audit
// (docs/superpowers/audits/2026-09-07-narration-viewpoint-audit.md) was a
// duplicated code path that copied a mechanical effect and dropped the
// narration next to it, so "did the author mean this silence?" is the question
// this value exists to answer.
var NoLine = Line{}

// Trio is one narrated event as its three audiences see it.
//
// Actor is the one acting. Actee is the one acted upon. Observer is everyone
// else in the room.
type Trio struct{ Actor, Actee, Observer Line }

// Recipient is anything that can be sent a categorized line. Satisfied by
// *users.UserRecord and by actions.Actor without either changing.
type Recipient interface {
	SendText(cat Category, text string)
}

// Broadcaster is anything that can broadcast to a room minus some user ids.
// Satisfied by *rooms.Room without change.
type Broadcaster interface {
	SendTextVisual(cat Category, txt string, excludeUserIds ...int)
}

// Audience is who is present for one narrated event.
//
// The ids are passed rather than derived from the Recipients because
// users.UserRecord carries UserId as a FIELD while actions.Actor exposes it as
// GetUserId(), so no single interface can reach both.
//
// A nil Actor is the mob side, where the actor has no client. A nil Actee is
// an actee that is a mob, or an event with no actee at all.
//
// ⚠️ ASSIGN A NIL RECIPIENT AS THE INTERFACE, NOT AS A TYPED NIL POINTER. A
// (*users.UserRecord)(nil) stored here is a non-nil interface value, so the
// guards below would call through it and panic. Declare the local as
// messaging.Recipient and leave it unset.
type Audience struct {
	Actor   Recipient
	ActorId int
	Actee   Recipient
	ActeeId int
	Room    Broadcaster
}

// SendTrio delivers one narrated event to everyone entitled to it.
//
// A line is delivered only if it has BOTH text and a recipient; either half
// being absent is a correct, silent skip. The room broadcast always excludes
// the actor and the actee, so a caller can no longer get the exclusion list
// wrong by hand.
//
// It does not render, choose, band or tokenise anything. Callers pass finished
// strings, which is also what lets a caller hand it text the caller has
// already anonymized itself -- see mobcommands/skill_move_defence.go, where
// the personal line must be anonymized at the call site because the audio
// channel does not do it.
func SendTrio(t Trio, aud Audience) {
	if aud.Actor != nil && t.Actor.Text != "" {
		aud.Actor.SendText(t.Actor.Cat, t.Actor.Text)
	}
	if aud.Actee != nil && t.Actee.Text != "" {
		aud.Actee.SendText(t.Actee.Cat, t.Actee.Text)
	}
	if aud.Room != nil && t.Observer.Text != "" {
		aud.Room.SendTextVisual(t.Observer.Cat, t.Observer.Text, trioExclusions(aud)...)
	}
}

// trioExclusions builds the room broadcast's exclusion list. Zero ids are
// omitted: a mob has no user id, and passing 0 would be a no-op that reads
// like a bug.
func trioExclusions(aud Audience) []int {
	ids := make([]int, 0, 2)
	if aud.ActorId > 0 {
		ids = append(ids, aud.ActorId)
	}
	if aud.ActeeId > 0 {
		ids = append(ids, aud.ActeeId)
	}
	return ids
}
