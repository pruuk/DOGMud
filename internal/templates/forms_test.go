package templates

import (
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// formsFS holds a template "t" 50 columns wide with forms drawn to 42 and 30,
// and "plain" with no forms.
func formsFS() fstest.MapFS {
	files := map[string]string{
		"templates/t.template":        strings.Repeat("b", 50),
		"templates/t.w42.template":    strings.Repeat("f", 40),
		"templates/t.w30.template":    strings.Repeat("n", 30),
		"templates/plain.template":    strings.Repeat("p", 50),
		"templates/wide.template":     strings.Repeat("w", 50),
		"templates/wide.w30.template": strings.Repeat("x", 35),
		"templates/sw.template":       `{{ screenwidth }}`,
	}
	// Template paths are built with the OS separator (util.FilePath).
	fsys := fstest.MapFS{}
	for name, data := range files {
		fsys[util.FilePath(name)] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

func TestFormsPickTheWidestThatFits(t *testing.T) {
	SetFSForTest(t, formsFS())
	for _, tc := range []struct {
		width int
		want  string
	}{
		{80, strings.Repeat("b", 50)}, // a base that fits is always used
		{60, strings.Repeat("b", 50)}, // the base fits
		{45, strings.Repeat("f", 40)}, // a form drawn to 42 can fit 45
		{42, strings.Repeat("f", 40)},
		{35, strings.Repeat("n", 30)},
		{30, strings.Repeat("n", 30)},
	} {
		got, err := process("t", nil, templateConfig{}, tc.width)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got, "width %d", tc.width)
	}
}

func TestFormsFallBackToTheNarrowest(t *testing.T) {
	SetFSForTest(t, formsFS())
	got, err := process("wide", nil, templateConfig{}, 30)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("x", 35), got, "no form fits, so the narrowest one found")

	got, err = process("plain", nil, templateConfig{}, 30)
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("p", 50), got, "a template with no forms is unchanged")
}

func TestScreenReaderFormComesFirst(t *testing.T) {
	fsys := formsFS()
	fsys[util.FilePath("templates/t.screenreader.template")] = &fstest.MapFile{Data: []byte("spoken")}
	SetFSForTest(t, fsys)
	got, err := process("t", nil, templateConfig{ScreenReader: true}, 30)
	require.NoError(t, err)
	assert.Equal(t, "spoken", got)
}

func TestScreenwidthIsTheReaderWidth(t *testing.T) {
	SetFSForTest(t, formsFS())
	for _, w := range []int{30, 42, 80} {
		got, err := process("sw", nil, templateConfig{}, w)
		require.NoError(t, err)
		assert.Equal(t, strconv.Itoa(w), got)
	}
	got, err := process("sw", nil, templateConfig{}, 120)
	require.NoError(t, err)
	assert.Equal(t, "80", got, "layouts are drawn for at most 80, so a wider telnet line sees them as before")

	got, err = ProcessText(`{{ screenwidth }}`, nil)
	require.NoError(t, err)
	assert.Equal(t, "80", got, "text rendered with no reader is laid out full width")
}

func TestFormsAreFoundOnceAndRemembered(t *testing.T) {
	SetFSForTest(t, formsFS())
	assert.Equal(t, []int{42, 30}, formsOf("t"))
	assert.Equal(t, []int{}, formsOf("plain"))
	if have, ok := formCache.Load("t"); !ok || len(have.([]int)) != 2 {
		t.Fatalf("formsOf did not remember t's forms: %v %v", have, ok)
	}
}

func TestWidestLineIgnoresColourTags(t *testing.T) {
	assert.Equal(t, 5, WidestLine("<ansi fg=\"red\">abc</ansi>\r\nabcde\nab"))
	assert.Equal(t, 0, WidestLine(""))
}

func TestFormWidthsAreWidestFirst(t *testing.T) {
	for i := 1; i < len(FormWidths); i++ {
		assert.Greater(t, FormWidths[i-1], FormWidths[i])
	}
	assert.Less(t, FormWidths[0], FullWidth)
}
