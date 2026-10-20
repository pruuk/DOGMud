package users

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
)

// builderPrompt opens a modal menu prompt the way an admin builder does.
func builderPrompt(u *UserRecord) {
	p, _ := u.StartPrompt("room", "edit containers")
	p.AskMenu(menu.Menu{Heading: "Choose one:", Entries: []menu.Entry{{Id: "new", Label: "new"}, {Id: "chest", Label: "chest"}}})
}

func TestRoutePromptInput_NoPromptRunsTheLine(t *testing.T) {
	u := menuUser()
	next, had := u.RoutePromptInput("look", true)
	if next != "look" || had {
		t.Fatalf("next %q had %v", next, had)
	}
}

func TestRoutePromptInput_TypedNumberAnswersAndRerunsTheCommand(t *testing.T) {
	u := menuUser()
	builderPrompt(u)
	next, had := u.RoutePromptInput("2", true)
	if next != "room edit containers" || !had {
		t.Fatalf("next %q had %v", next, had)
	}
	if q := u.GetPrompt().Questions[0]; !q.Done || q.Response != "chest" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestRoutePromptInput_QueuedLineNeverAnswers(t *testing.T) {
	u := menuUser()
	builderPrompt(u)
	next, had := u.RoutePromptInput("1", false)
	if next != "1" || !had {
		t.Fatalf("next %q had %v", next, had)
	}
	if u.GetPrompt() == nil || u.GetPrompt().Questions[0].Done {
		t.Fatal("a queued line answered or closed the prompt")
	}
}

func TestRoutePromptInput_ModalMenuKeepsAnUnknownLine(t *testing.T) {
	u := menuUser()
	builderPrompt(u)
	next, _ := u.RoutePromptInput("north", true)
	if next != "room edit containers" || u.GetPrompt() == nil {
		t.Fatalf("next %q prompt %v", next, u.GetPrompt())
	}
}

func TestRoutePromptInput_NonModalClosesOnANonAnswerAndRunsIt(t *testing.T) {
	u := menuUser()
	builderPrompt(u)
	u.GetPrompt().NonModal = true
	next, had := u.RoutePromptInput("north", true)
	if next != "north" || !had || u.GetPrompt() != nil {
		t.Fatalf("next %q had %v prompt %v", next, had, u.GetPrompt())
	}
}

func TestRoutePromptInput_NonModalStillTakesAnAnswer(t *testing.T) {
	u := menuUser()
	builderPrompt(u)
	u.GetPrompt().NonModal = true
	next, _ := u.RoutePromptInput("1", true)
	if next != "room edit containers" || !u.GetPrompt().Questions[0].Done {
		t.Fatalf("next %q", next)
	}
}

func TestRoutePromptInput_FinishedPromptIsClearedAndTheLineRuns(t *testing.T) {
	u := menuUser()
	builderPrompt(u)
	u.GetPrompt().Questions[0].Answer("1")
	next, had := u.RoutePromptInput("look", true)
	if next != "look" || !had || u.GetPrompt() != nil {
		t.Fatalf("next %q had %v prompt %v", next, had, u.GetPrompt())
	}
}

func TestClearFinishedPrompt(t *testing.T) {
	u := menuUser()
	u.ClearFinishedPrompt() // no prompt: nothing to do
	builderPrompt(u)
	u.ClearFinishedPrompt()
	if u.GetPrompt() == nil {
		t.Fatal("an unanswered prompt was cleared")
	}
	u.GetPrompt().Questions[0].Answer("1")
	u.ClearFinishedPrompt()
	if u.GetPrompt() != nil {
		t.Fatal("a finished prompt was kept")
	}
}
