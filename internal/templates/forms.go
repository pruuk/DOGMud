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
