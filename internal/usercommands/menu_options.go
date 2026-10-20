package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/menu"
	"github.com/GoMudEngine/GoMud/internal/templates"
)

// optionsMenu turns a builder's NameDescription list into a menu. An entry's
// Id is the option's Id (as text) when the option carries one, else its
// Name, so a chosen entry arrives as something the builder resolves with
// optionByResponse. Ids matter because names are not unique: two condition
// specs are both named "Stunned", and picking the second by number must not
// select the first. A Marked option shows the [x] marker. columns 2 lays long
// name-only lists side by side, as the retired numbered-list-doubled template
// did. Builders match by prefix.
func optionsMenu(heading string, opts []templates.NameDescription, columns int) menu.Menu {
	m := menu.Menu{Heading: heading, Columns: columns, Prefix: true}
	for _, o := range opts {
		e := menu.Entry{Id: optionId(o), Label: o.Name}
		if columns != 2 {
			e.Detail = o.Description
		}
		if o.Marked {
			e.Marker = menu.MarkerChecked
		}
		m.Entries = append(m.Entries, e)
	}
	return m
}

// optionId is the menu entry Id optionsMenu gives an option.
func optionId(o templates.NameDescription) string {
	if o.Id == nil {
		return o.Name
	}
	return fmt.Sprint(o.Id)
}

// optionByResponse finds the option a menu answer names: first by entry Id,
// then by case-insensitive Name (a typed name on a menu whose Ids differ).
func optionByResponse(opts []templates.NameDescription, response string) (templates.NameDescription, bool) {
	for _, o := range opts {
		if optionId(o) == response {
			return o, true
		}
	}
	for _, o := range opts {
		if strings.EqualFold(o.Name, response) {
			return o, true
		}
	}
	return templates.NameDescription{}, false
}
