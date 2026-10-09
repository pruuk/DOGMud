package templates

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #455: maps/map.template builds the legend's colour tag with mapslug, the
// same slug the minimap and the map command use, so "Deep Water" is
// map-deep-water there too and not the unparseable "map-deep water".
func TestMapSlugFuncHyphenatesTheLegendName(t *testing.T) {
	tpl, err := template.New("legend").Funcs(funcMap).Parse(`fg="map-{{mapslug .}}"`)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, "Deep Water"))
	assert.Equal(t, `fg="map-deep-water"`, buf.String())
}
