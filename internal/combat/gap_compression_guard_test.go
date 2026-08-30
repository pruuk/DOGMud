package combat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compression must be applied in exactly ONE place. A second site would double
// compress; a bypassing site would opt that contest out of the change without
// anybody noticing, which is precisely the failure mode the arc's single-seam
// design exists to prevent.
func TestCompressionHasExactlyOneCallSite(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	sites := map[string]int{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		if n := strings.Count(string(src), "compressContestGap("); n > 0 {
			sites[e.Name()] = n
		}
	}

	assert.Equal(t, map[string]int{"run_contest.go": 2}, sites,
		"compressContestGap must appear only in run_contest.go: once as its "+
			"definition and once at the RunContest call. A second call site "+
			"double-compresses; a contest that bypasses RunContest opts out of "+
			"the change silently.")
}
