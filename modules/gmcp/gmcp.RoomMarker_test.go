package gmcp

import (
	"testing"

	_ "github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestRoomInfo_NpcCarriesTheViewersTalkMarker(t *testing.T) {
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict([]byte(`mobid: 1
zone: SightZone
topics:
  - key: secret
    kind: info
    label: "Tell me a secret."
    text: "Under the altar."
    marked: true
`), &df))
	t.Cleanup(dialogue.InstallForTest(&df))

	viewer := roomSightFixture(t, 60)
	npcs := roomInfoFor(t, viewer).Contents.Npcs
	require.Len(t, npcs, 1)
	require.Equal(t, "[*]", npcs[0].Marker)

	viewer.Character.MarkHeard("1.secret")
	npcs = roomInfoFor(t, viewer).Contents.Npcs
	require.Len(t, npcs, 1)
	require.Equal(t, "", npcs[0].Marker)
}
