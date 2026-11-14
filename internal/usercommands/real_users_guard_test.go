package usercommands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// realUsersDir is the shipped world's users folder (gitignored living state).
// No test in this package may write to it (#826): a local server would load
// the test account as user 1.
const realUsersDir = "../../_datafiles/world/dogmud/users"

// snapshotRealUsers lists every entry in the real users folder with its size
// and modification time, so a create, rewrite or delete all show up.
func snapshotRealUsers() string {
	entries, err := os.ReadDir(realUsersDir)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	var lines []string
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s size=%d mtime=%d", filepath.Base(e.Name()), info.Size(), info.ModTime().UnixNano()))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
