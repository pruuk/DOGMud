package users

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// RoutePromptInput gives one input line to the open command prompt (#375).
// It returns the line the caller runs next and whether a prompt was open.
//
//   - No prompt: the line runs unchanged.
//   - A prompt with no pending question is finished: it is cleared and the
//     line runs unchanged.
//   - A queued line (typed false: a follow step, a script, a macro the engine
//     expanded from untyped input) never answers a prompt; it runs unchanged.
//   - A non-modal prompt whose menu question does not recognise a typed line
//     closes, and the line runs as a command.
//   - Otherwise the typed line answers the question, and the prompt's own
//     command (Command + " " + Rest) runs next so its handler reads the
//     answer or asks again. A refused menu answer says why first.
func (u *UserRecord) RoutePromptInput(line string, typed bool) (next string, hadPrompt bool) {
	p := u.GetPrompt()
	if p == nil {
		return line, false
	}
	q := p.GetNextQuestion()
	if q == nil {
		u.ClearPrompt()
		return line, true
	}
	if !typed {
		return line, true
	}

	var res menu.Result
	if q.Menu != nil {
		res = q.Resolve(line)
		if p.NonModal && res.Kind == menu.NotAnAnswer {
			u.ClearPrompt()
			// Said before the line runs, so a draft never vanishes silently.
			u.sayPromptSetAside(p)
			return line, true
		}
	}

	q.Answer(line)
	if q.Menu != nil {
		switch q.LastResult {
		case menu.NoSuchNumber:
			u.SendText(messaging.CategoryRefusal, fmt.Sprintf("There is no choice %s.", strings.TrimSpace(line)))
		case menu.Ambiguous:
			u.SendText(messaging.CategoryRefusal, ambiguousLine(res.Matches))
		case menu.NotAnAnswer, menu.Empty:
			// NotAnAnswer arrives only from a modal menu (a non-modal one
			// closed above). Empty, Enter on a menu with no Default and no
			// Leave, arrives from either kind (picklock is non-modal). The
			// command redraws its menu after this line either way.
			u.SendText(messaging.CategoryRefusal, "Pick a number from the list.")
		}
	}

	if p.Command == `` {
		return ``, true
	}
	return p.Command + " " + p.Rest, true
}

// ClearFinishedPrompt clears a prompt whose questions are all answered, so
// Char.Menu reports it closed at once rather than on the next input (#375).
// It does nothing when no prompt is open or a question is still pending.
func (u *UserRecord) ClearFinishedPrompt() {
	if p := u.GetPrompt(); p != nil && p.GetNextQuestion() == nil {
		u.ClearPrompt()
	}
}

// ambiguousLine names the entries a prefix matched, the same refusal a game
// menu gives.
func ambiguousLine(matches []menu.Entry) string {
	labels := make([]string, 0, len(matches))
	for _, e := range matches {
		labels = append(labels, e.Label)
	}
	return "Which one? " + strings.Join(labels, ", ") + "."
}
