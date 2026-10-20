package prompt

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/menu"
)

func TestAskMenu_ChoosingSetsResponseToTheEntryId(t *testing.T) {
	p := New("room edit containers", "")
	m := menu.Menu{Heading: "Choose one:", Entries: []menu.Entry{{Id: "new", Label: "new"}, {Id: "chest", Label: "chest"}}, Default: "new"}
	q := p.AskMenu(m)
	q.Answer("2")
	if !q.Done || q.Response != "chest" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestAskMenu_EnterUsesTheDefault(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "new", Label: "new"}}, Default: "new"})
	q.Answer("")
	if !q.Done || q.Response != "new" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestAskMenu_LeaveUsesTheLeaveResponse(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a"}}, Leave: "Done"}, "0")
	q.Answer("2")
	if !q.Done || q.Response != "0" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestAskMenu_BadAnswerLeavesItOpenAndRecordsWhy(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a"}}})
	q.Answer("7")
	if q.Done || q.LastResult != menu.NoSuchNumber {
		t.Fatalf("done %v last %v", q.Done, q.LastResult)
	}
}

func TestAskMenu_SameHeadingReplacesTheMenu(t *testing.T) {
	p := New("x", "")
	p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a"}}})
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a", Marker: menu.MarkerChecked}}})
	if len(p.Questions) != 1 || q.Menu.Entries[0].Marker != menu.MarkerChecked {
		t.Fatalf("questions %d marker %q", len(p.Questions), q.Menu.Entries[0].Marker)
	}
}

func TestAskMenu_AcceptTextTakesAnUnmatchedLine(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a"}}, AcceptText: true})
	q.Answer("  a brass key  ")
	if !q.Done || q.Response != "a brass key" || q.LastResult != menu.Text {
		t.Fatalf("done %v response %q last %v", q.Done, q.Response, q.LastResult)
	}
}

func TestAskConfirm(t *testing.T) {
	p := New("guild", "")
	q := p.AskConfirm("Disband?", "Yes, disband it", "No, keep it")
	q.Answer("y")
	if !q.Done || q.Response != "yes" {
		t.Fatalf("done %v response %q", q.Done, q.Response)
	}
}

func TestQuestionString_MenuShowsABarePrompt(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "A long heading", Entries: []menu.Entry{{Id: "a", Label: "a"}}})
	if s := q.String(); s != `<ansi fg="black-bold">.:</ansi> ` {
		t.Fatalf("menu prompt line: %q", s)
	}
}

func TestResolve_ReportsWithoutAnswering(t *testing.T) {
	p := New("x", "")
	q := p.AskMenu(menu.Menu{Heading: "h", Entries: []menu.Entry{{Id: "a", Label: "a"}}})
	if r := q.Resolve("north"); r.Kind != menu.NotAnAnswer {
		t.Fatalf("north resolved as %v", r.Kind)
	}
	if r := q.Resolve("1"); r.Kind != menu.Chosen || q.Done {
		t.Fatalf("1 resolved as %v, done %v", r.Kind, q.Done)
	}
	free := p.Ask("Name?", nil)
	if r := free.Resolve(" Bob "); r.Kind != menu.Text || r.Text != "Bob" {
		t.Fatalf("free text resolved as %v %q", r.Kind, r.Text)
	}
}
