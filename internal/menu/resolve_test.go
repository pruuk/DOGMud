package menu

import "testing"

func TestResolve(t *testing.T) {
	m := talkMenu()
	cases := []struct {
		name  string
		input string
		kind  ResultKind
		id    string
	}{
		{"number", "2", Chosen, "pass"},
		{"number with spaces", "  1 ", Chosen, "feud"},
		{"leave by number", "4", Leave, ""},
		{"out of range high", "5", NoSuchNumber, ""},
		{"zero", "0", NoSuchNumber, ""},
		{"enter leaves", "", Leave, ""},
		{"exact label", "what do you carve?", Chosen, "carve"},
		{"exact id", "feud", Chosen, "feud"},
		{"exact leave label", "leave", Leave, ""},
		{"prefix is not an answer in a game menu", "how", NotAnAnswer, ""},
		{"a command is not an answer", "north", NotAnAnswer, ""},
		{"l is look, not leave", "l", NotAnAnswer, ""},
	}
	for _, c := range cases {
		r := Resolve(m, c.input)
		if r.Kind != c.kind {
			t.Errorf("%s: kind %v, want %v", c.name, r.Kind, c.kind)
			continue
		}
		if c.id != "" && (r.Entry == nil || r.Entry.Id != c.id) {
			t.Errorf("%s: entry %+v, want id %q", c.name, r.Entry, c.id)
		}
	}
}

func TestResolve_PrefixInBuilderMenus(t *testing.T) {
	m := Menu{Prefix: true, Entries: []Entry{{Id: "weapon", Label: "weapon"}, {Id: "wearable", Label: "wearable"}, {Id: "food", Label: "food"}}}
	if r := Resolve(m, "fo"); r.Kind != Chosen || r.Entry.Id != "food" {
		t.Fatalf("unique prefix: %+v", r)
	}
	r := Resolve(m, "we")
	if r.Kind != Ambiguous || len(r.Matches) != 2 {
		t.Fatalf("ambiguous prefix: %+v", r)
	}
}

func TestResolve_EnterPicksDefaultBeforeLeave(t *testing.T) {
	m := Menu{Entries: []Entry{{Id: "a", Label: "A"}, {Id: "b", Label: "B"}}, Default: "b", Leave: "Cancel"}
	if r := Resolve(m, ""); r.Kind != Chosen || r.Entry.Id != "b" {
		t.Fatalf("enter with default: %+v", r)
	}
	if r := Resolve(Menu{Entries: []Entry{{Id: "a", Label: "A"}}}, ""); r.Kind != Empty {
		t.Fatalf("enter with neither: %+v", r)
	}
}

func TestResolve_AcceptText(t *testing.T) {
	m := Menu{AcceptText: true, Entries: []Entry{{Id: "server", Label: "server"}}}
	if r := Resolve(m, "server.motd"); r.Kind != Text || r.Text != "server.motd" {
		t.Fatalf("free text: %+v", r)
	}
	if r := Resolve(m, "1"); r.Kind != Chosen {
		t.Fatalf("number still chooses: %+v", r)
	}
}

func TestConfirm(t *testing.T) {
	m := Confirm(`Disband "Iron Wolves"? This cannot be undone.`, "Yes, disband it", "No, keep it")
	for in, want := range map[string]string{"1": "yes", "2": "no", "": "no", "y": "yes", "yes": "yes", "n": "no", "no": "no", "Yes, disband it": "yes"} {
		r := Resolve(m, in)
		if r.Kind != Chosen || r.Entry.Id != want {
			t.Errorf("confirm %q: %+v, want %s", in, r, want)
		}
	}
	if r := Resolve(m, "north"); r.Kind != NotAnAnswer {
		t.Errorf("confirm north: %+v", r)
	}
	if m.Leave != "" {
		t.Errorf("a confirm has no Leave entry")
	}
	if plain(Render(m)) != "Disband \"Iron Wolves\"? This cannot be undone.\n\n  1.     Yes, disband it\n  2.     No, keep it\n\nType a number, or press Enter for 2." {
		t.Errorf("confirm render: %q", plain(Render(m)))
	}
}
