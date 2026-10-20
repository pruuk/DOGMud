package characters

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormattedName_MarkerLeadsTheName(t *testing.T) {
	f := FormattedName{Name: "warden velk", Type: "mobname", Marker: "[?]"}
	out := f.String()
	require.True(t, strings.HasPrefix(out, `<ansi fg="questflag">[?]</ansi> `), out)
	f.Marker = ""
	require.NotContains(t, f.String(), "questflag")
}
