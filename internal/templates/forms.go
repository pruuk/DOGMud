package templates

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Narrow forms (web client base spec, section 2). A template may have forms
// drawn for narrower screens beside it, named for the width they are drawn
// to: character/status.w42.template, character/status.w30.template. When the
// base output is wider than the reader's screen, Process renders the forms
// widest first and uses the first whose longest line fits, or the narrowest
// if none does (ruling 22: the widest form that fits). A base that fits is
// always used, so a desktop sees a form only where the base was already
// wider than its 80 columns.

// FullWidth is the widest a screen is laid out for.
const FullWidth = users.MaxAutoWrapWidth

// FormWidths are the widths a form may be drawn to, widest first. A file
// named for any other width is never read; internal/guards holds that.
var FormWidths = []int{42, 40, 30}

// formCache remembers which forms each template has, so a template with none
// costs nothing after its first narrow render. A form file added while the
// server runs is seen after a restart. SetFSForTest clears it.
var formCache sync.Map // fname -> []int, widest first

// FormPath is the file name of fname's form drawn to width.
func FormPath(fname string, width int) string {
	return util.FilePath(`templates/`, fname+`.w`+strconv.Itoa(width)+`.template`)
}

// formsOf lists the widths fname has a form drawn to, widest first.
func formsOf(fname string) []int {
	if have, ok := formCache.Load(fname); ok {
		return have.([]int)
	}
	have := []int{}
	for _, width := range FormWidths {
		if formExists(FormPath(fname, width)) {
			have = append(have, width)
		}
	}
	formCache.Store(fname, have)
	return have
}

// formExists looks for path where Process would read it: a registered
// filesystem first, then the data files on disk.
func formExists(path string) bool {
	for _, f := range fileSystems {
		if file, err := f.Open(path); err == nil {
			_ = file.Close()
			return true
		}
	}
	_, err := os.Stat(util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/`, path))
	return err == nil
}

// readerWidth is the width a reader's screen has room for: the user's
// WrapWidth, or FullWidth when there is no reader.
func readerWidth(userId int) int {
	if userId > 0 {
		if u := users.GetByUserId(userId); u != nil {
			return u.WrapWidth()
		}
	}
	return FullWidth
}

// widthFuncs replaces the screenwidth template function for one render, so a
// template can fit a rule or a wrapped paragraph to its reader. Layouts are
// drawn for at most FullWidth, so a telnet reader with a wider saved line
// width sees them as before.
func widthFuncs(width int) template.FuncMap {
	width = min(width, FullWidth)
	return template.FuncMap{"screenwidth": func() int { return width }}
}

// WidestLine is the display width of the longest line in s, colour tags
// not counted.
func WidestLine(s string) int {
	widest := 0
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		widest = max(widest, util.VisibleWidth(line))
	}
	return widest
}

// ProcessForWidth renders fname for every reader whose screen is width
// columns wide and who does, or does not, use a screen reader. It picks the
// form by width alone (formFor) and measures nothing, so a broadcast renders
// once per distinct (screen reader, width) among its readers, never once per
// reader (spec ruling 11, plan 3b-3). A screen a player asks for uses
// Process, which measures and so picks the widest form that fits.
func ProcessForWidth(fname string, data any, screenReader bool, width int) (string, error) {
	return processPicked(fname, data, templateConfig{ScreenReader: screenReader}, width, false)
}

// ProcessAtWidth renders fname as Process does for a reader whose screen is
// width columns wide, without a screen reader: it measures, so the widest
// form that fits is used. For one reader who has no user to measure for
// (the connect splash, login prompts, the goodbye; internal/inputhandlers
// and main.go), and for guards that hold every template to the width it is
// shown at (internal/guards). A broadcast uses ProcessForWidth instead.
func ProcessAtWidth(fname string, data any, width int) (string, error) {
	return process(fname, data, templateConfig{}, width)
}

// formFor is the form a render by width alone uses: 0 (the base) at
// FullWidth and wider or when there are no forms, else the widest form
// drawn to no more than width, or the narrowest when width is narrower than
// them all. forms is widest first, as formsOf returns it.
func formFor(forms []int, width int) int {
	if width >= FullWidth || len(forms) == 0 {
		return 0
	}
	for _, formWidth := range forms {
		if formWidth <= width {
			return formWidth
		}
	}
	return forms[len(forms)-1]
}

// narrowestFit renders fname's forms widest first and returns the first that
// fits width, or the narrowest one. base is the base form's output, returned
// when the template has no forms.
func narrowestFit(fname string, data any, tplConfig templateConfig, width int, base string, forms []int) string {
	best := base
	for _, formWidth := range forms {
		out, found, err := renderFile(templateDetails{name: fname, path: FormPath(fname, formWidth)}, data, tplConfig, width)
		if !found || err != nil {
			continue
		}
		best = out
		if WidestLine(out) <= width {
			return out
		}
	}
	return best
}
