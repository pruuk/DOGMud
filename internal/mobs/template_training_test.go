package mobs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// After U10b-0 Phase A, no mob TEMPLATE may carry authored stat training.
//
// Training means gains-since-spawn. A template has not spawned yet, so any
// nonzero value there is authored baseline sitting in the wrong field — and
// under U10b-0 the progression curve reads Training as its rank, so a mob whose
// authored stats live there starts partway down the decay curve and the Phase C
// gain cap can freeze it at spawn. Authored stats belong in base:.
//
// This is a permanent gate, not a one-off migration check: it fails if a builder
// or the mob web editor reintroduces the old convention. Before Phase A there
// were 599 such templates under _datafiles/world/dogmud/mobs.
//
// It reads the YAML text rather than loading through the engine on purpose.
// Loading would report a mob id, but a builder who trips this needs the file
// path; it would also need the species roster and would leave the package-level
// mob registry filled for every later test in this binary.
func TestNoMobTemplateCarriesAuthoredTraining(t *testing.T) {
	root := mobDataRoot(t)

	// `training:` is only meaningful inside a stat block, but a mob YAML has no
	// other key by that name, so a plain match anywhere on the line is both
	// sufficient and impossible to fool with nesting. It must not be anchored to
	// the start of the line: six templates authored their stats in flow style
	// (`strength:    {training: 35}`), and an anchored pattern silently passed
	// all 36 of those entries.
	trainingLine := regexp.MustCompile(`\btraining:\s*(-?\d+)`)

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(body), "\n") {
			if m := trainingLine.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				offenders = append(offenders, fmt.Sprintf("%s:%d: training: %s", filepath.ToSlash(rel), i+1, m[1]))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d mob template stat(s) carry authored training; move the value into base: instead\n(base_new = the stat's existing base, else its species base, plus the authored training)\n\n%s",
			len(offenders), strings.Join(offenders, "\n"))
	}
}

// mobDataRoot returns the mobs directory the running game actually loads.
//
// Go test binaries run with the working directory set to their own package
// directory, and the filepaths config defaults to the upstream world rather than
// this one, so both have to be corrected. configs.SetConfigForTest snapshots the
// config and self-registers the restore, so the reload does not leak into later
// tests in this binary.
func mobDataRoot(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Join(cwd, "..", "..")
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("reload config: %v", err)
	}

	root := filepath.Join(repoRoot, configs.GetFilePathsConfig().DataFiles.String(), "mobs")
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		t.Fatalf("mob data root %q is not a directory: %v", root, err)
	}
	return root
}
