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

// Broadcaster is a room that can deliver an event's observer line and judge a
// participant's sight. Satisfied by *rooms.Room.
type Broadcaster interface {
	// SendTextVisualHidingNames broadcasts a sight-gated line, excluding some
	// user ids. An observer who makes out shapes only reads each of names as
	// "a figure".
	SendTextVisualHidingNames(cat Category, txt string, names []string, excludeUserIds ...int)
	// ParticipantSight is ParticipantSight for the user in this room.
	ParticipantSight(userId int) SightDecision
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
	// ActorName and ActeeName are the names exactly as the lines print them,
	// plain or inside an identity tag. SendTrio hides each from a reader who
	// cannot see that party: "a figure" for infrared, "something" otherwise.
	// Write NoName for a side with nobody on it; the root guard requires both.
	ActorName string
	Actee     Recipient
	ActeeId   int
	ActeeName string
	Room      Broadcaster
}

// SendTrio delivers one narrated event to everyone entitled to it.
//
// A line is delivered only if it has BOTH text and a recipient; either half
// being absent is a correct, silent skip. The room broadcast always excludes
// the actor and the actee, so a caller can no longer get the exclusion list
// wrong by hand.
//
// Each role is rendered for its own reader. The actor's line hides ActeeName
// and the actee's line hides ActorName, judged by that reader's
// ParticipantSight; the observer line hides both, judged per observer by the
// room. It chooses, bands and tokenises nothing: callers still pass finished
// strings, which is also what lets a caller hand it text the caller has
// already anonymized itself -- see mobcommands/skill_move_defence.go.
func SendTrio(t Trio, aud Audience) {
	if aud.Actor != nil && t.Actor.Text != "" {
		aud.Actor.SendText(t.Actor.Cat, hideForReader(aud, aud.ActorId, t.Actor.Text, aud.ActeeName))
	}
	if aud.Actee != nil && t.Actee.Text != "" {
		aud.Actee.SendText(t.Actee.Cat, hideForReader(aud, aud.ActeeId, t.Actee.Text, aud.ActorName))
	}
	if aud.Room != nil && t.Observer.Text != "" {
		aud.Room.SendTextVisualHidingNames(t.Observer.Cat, t.Observer.Text,
			[]string{aud.ActorName, aud.ActeeName}, trioExclusions(aud)...)
	}
}

// hideForReader hides the other party's name from one participant by that
// participant's sight in the room. With no room there is no light to judge.
func hideForReader(aud Audience, readerId int, text, otherName string) string {
	if aud.Room == nil || otherName == NoName {
		return text
	}
	return HideNames(text, []string{otherName}, aud.Room.ParticipantSight(readerId))
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
