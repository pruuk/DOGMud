package menu

import (
	"strconv"
	"strings"
)

// ResultKind says what a typed line meant to a menu.
type ResultKind int

const (
	NotAnAnswer  ResultKind = iota // not for this menu: a game menu closes and the line runs as a command
	Chosen                         // Entry is set
	Leave                          // the Leave entry, or Enter on a menu with Leave and no Default
	NoSuchNumber                   // a number outside 1..len
	Ambiguous                      // a prefix matching several entries (Prefix menus only); Matches is set
	Empty                          // Enter on a menu with no Default and no Leave
	Text                           // an unmatched line on an AcceptText menu; Text is set
)

// Result is what Resolve decided.
type Result struct {
	Kind    ResultKind
	Entry   *Entry
	Matches []Entry
	Text    string
}

// Resolve matches a typed line against the menu. Game menus resolve only a
// number, Enter, an exact label, id or alias; a prefix resolves only when
// Prefix is set, because in a non-modal menu "n" or "l" is a movement or look
// command, not a choice.
func Resolve(m Menu, input string) Result {
	in := strings.TrimSpace(input)
	all := m.all()

	if in == "" {
		if m.Default != "" {
			for i := range m.Entries {
				if m.Entries[i].Id == m.Default {
					e := m.Entries[i]
					return Result{Kind: Chosen, Entry: &e}
				}
			}
		}
		if m.Leave != "" {
			return Result{Kind: Leave}
		}
		return Result{Kind: Empty}
	}

	if allDigits(in) {
		n, err := strconv.Atoi(in)
		if err != nil || n < 1 || n > len(all) {
			return Result{Kind: NoSuchNumber}
		}
		if m.Leave != "" && n == len(all) {
			return Result{Kind: Leave}
		}
		e := m.Entries[n-1]
		return Result{Kind: Chosen, Entry: &e}
	}

	for i := range m.Entries {
		e := m.Entries[i]
		if strings.EqualFold(e.Label, in) || strings.EqualFold(e.Id, in) {
			return Result{Kind: Chosen, Entry: &e}
		}
		for _, a := range e.Aliases {
			if strings.EqualFold(a, in) {
				return Result{Kind: Chosen, Entry: &e}
			}
		}
	}
	if m.Leave != "" && strings.EqualFold(m.Leave, in) {
		return Result{Kind: Leave}
	}

	if m.Prefix {
		low := strings.ToLower(in)
		var matches []Entry
		for _, e := range m.Entries {
			if strings.HasPrefix(strings.ToLower(e.Label), low) || strings.HasPrefix(strings.ToLower(e.Id), low) {
				matches = append(matches, e)
			}
		}
		leaveMatch := m.Leave != "" && strings.HasPrefix(strings.ToLower(m.Leave), low)
		switch {
		case len(matches) == 1 && !leaveMatch:
			return Result{Kind: Chosen, Entry: &matches[0]}
		case len(matches) == 0 && leaveMatch:
			return Result{Kind: Leave}
		case len(matches) > 1 || (len(matches) == 1 && leaveMatch):
			if leaveMatch {
				matches = append(matches, Entry{Label: m.Leave})
			}
			return Result{Kind: Ambiguous, Matches: matches}
		}
	}

	if m.AcceptText {
		return Result{Kind: Text, Text: in}
	}
	return Result{Kind: NotAnAnswer}
}

// allDigits reports whether s is non-empty and made only of ASCII digits, so
// "+2" and "-1" are text, not numbers.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Confirm builds the two-entry yes/no menu. No is the way out and Enter picks
// it. For an action that cannot be undone, the labels name the action
// ("Yes, disband it" / "No, keep it"); otherwise pass "Yes" and "No".
func Confirm(question, yesLabel, noLabel string) Menu {
	return Menu{
		Heading: question,
		Entries: []Entry{
			{Id: "yes", Label: yesLabel, Aliases: []string{"y"}},
			{Id: "no", Label: noLabel, Aliases: []string{"n"}},
		},
		Default: "no",
	}
}
