package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Owner 2026-10-17: `help party` and `help follow` link to each other. The
// follow topic ships from modules/follow (see moduleProvidedHelpTopics);
// modules/follow's own tests check its link back.
func TestHelpParty_LinksToHelpFollow(t *testing.T) {
	useU8DataFiles(t)
	rendered := processU8Help(t, "help/party")
	assert.Contains(t, u8HelpCrossReferenceTopics(rendered), "follow", "help party has a See also: help follow")
	require.NoError(t, validateU8HelpCrossReferences("help/party", rendered))
}
