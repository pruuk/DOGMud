package inputhandlers

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/stretchr/testify/require"
)

// A screen reader's connect splash is plain words at every width: no art,
// no rule of = signs, no spaced-out title.
func TestConnectSplash_ScreenReaderIsPlainWords(t *testing.T) {
	loginRenderSetup(t)
	for _, w := range []int{30, 42, 80} {
		out, err := templates.ProcessForWidth("login/connect-splash", nil, true, w)
		require.NoError(t, err)
		require.Contains(t, out, `Delusions of Grandeur.`, w)
		require.NotContains(t, out, `===`, w)
		require.NotContains(t, out, `D e l u s`, w)
		require.False(t, strings.Contains(out, `*`), "no star art at %d", w)
	}
}
