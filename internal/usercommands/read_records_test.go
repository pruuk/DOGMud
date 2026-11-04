package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// #667: reading a carried readable item is remembered, which is what the
// items_read achievement counts. Only true readables with text count.
func TestReadRecordsACarriedReadable(t *testing.T) {
	const pageId, noticeId, blankId, blobId = 999501, 999502, 999503, 999504
	alice, _ := interactScene(t)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		pageId:   {ItemId: pageId, Name: "torn page", Type: items.Readable, Description: "A torn page, closely written."},
		noticeId: {ItemId: noticeId, Name: "pinned notice", Type: items.Object, Subtype: items.Mundane, Description: "A notice about the road."},
		blankId:  {ItemId: blankId, Name: "blank leaf", Type: items.Readable},
		blobId:   {ItemId: blobId, Name: "inked folio", Type: items.Readable},
	}))
	folio := items.New(blobId)
	folio.SetBlob("Written in the blob, not the spec.")
	alice.Character.Items = []items.Item{items.New(pageId), items.New(noticeId), items.New(blankId), folio}

	read := func(name string) {
		t.Helper()
		_, err := TryCommand("read", name, alice.UserId, 0)
		require.NoError(t, err)
	}

	read("torn")
	require.True(t, alice.Character.HasReadItem(pageId), "a readable with text is recorded")

	read("notice")
	require.False(t, alice.Character.HasReadItem(noticeId), "a notice authored as an object is not a page")

	read("blank")
	require.False(t, alice.Character.HasReadItem(blankId), "a readable with no text at all is not recorded")

	read("folio")
	require.True(t, alice.Character.HasReadItem(blobId), "text carried in the blob counts")
}
