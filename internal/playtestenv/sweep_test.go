package playtestenv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// sweepDocker is a stateful Docker double: it holds containers with labels,
// honours `ps --filter label=...`, and removes on `rm --force` unless stubborn.
type sweepDocker struct {
	mu         sync.Mutex
	containers map[string]map[string]string // id -> labels
	stubborn   map[string]bool              // ids that survive rm
	removed    []string
	// ignoreFilters makes ps return every container, as if the label filter
	// were not honoured, to prove the client-side label guard on its own.
	ignoreFilters bool
}

func newSweepDocker() *sweepDocker {
	return &sweepDocker{containers: map[string]map[string]string{}, stubborn: map[string]bool{}}
}

func (d *sweepDocker) add(id, runID string, managed bool) {
	labels := map[string]string{labelRunID: runID}
	if managed {
		labels[labelManaged] = labelManagedValue
	}
	d.containers[id] = labels
}

func (d *sweepDocker) Run(_ context.Context, spec CommandSpec) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	args := spec.Args
	// Strip the --context prefix pair.
	if len(args) > 2 && args[0] == "--context" {
		args = args[2:]
	}
	switch {
	case len(args) > 0 && args[0] == "ps":
		var want []string
		for i, a := range args {
			if a == "--filter" && i+1 < len(args) {
				want = append(want, strings.TrimPrefix(args[i+1], "label="))
			}
		}
		for id, labels := range d.containers {
			ok := true
			for _, w := range want {
				k, v, _ := strings.Cut(w, "=")
				if labels[k] != v {
					ok = false
				}
			}
			if !ok && !d.ignoreFilters {
				continue
			}
			parts := []string{}
			for k, v := range labels {
				parts = append(parts, k+"="+v)
			}
			row, _ := json.Marshal(map[string]string{"ID": id, "Labels": strings.Join(parts, ",")})
			_, _ = io.WriteString(spec.Stdout, string(row)+"\n")
		}
		return nil
	case len(args) > 2 && args[0] == "rm" && args[1] == "--force":
		id := args[2]
		d.removed = append(d.removed, id)
		if !d.stubborn[id] {
			delete(d.containers, id)
		}
		return nil
	}
	return fmt.Errorf("sweepDocker: unexpected docker %v", args)
}

func newSweepSupervisor(d Runner) *Supervisor {
	return newSupervisor(supervisorDeps{
		runner: d,
		resolveDocker: func(context.Context, Runner) (dockerContext, error) {
			return dockerContext{name: opsTestCtx, env: []string{"PATH=/usr/bin"}}, nil
		},
	})
}

func TestSweepRunRemovesOnlyThisRunsLabelledContainers(t *testing.T) {
	d := newSweepDocker()
	d.add("mine1", "run-a", true)
	d.add("mine2", "run-a", true)
	d.add("other-run", "run-b", true)
	d.add("owner-unmanaged", "run-a", false) // run-id label alone is not ours
	s := newSweepSupervisor(d)

	left, err := s.SweepRun(context.Background(), RunOptions{RunID: "run-a"})
	require.NoError(t, err)
	require.Empty(t, left)
	require.ElementsMatch(t, []string{"mine1", "mine2"}, d.removed)
	require.Contains(t, d.containers, "other-run")
	require.Contains(t, d.containers, "owner-unmanaged")
}

func TestSweepRunLabelGuardHoldsEvenIfDockerIgnoresFilters(t *testing.T) {
	d := newSweepDocker()
	d.ignoreFilters = true
	d.add("mine", "run-a", true)
	d.add("other-run", "run-b", true)
	d.add("owner-unmanaged", "run-a", false)
	s := newSweepSupervisor(d)

	left, err := s.SweepRun(context.Background(), RunOptions{RunID: "run-a"})
	require.NoError(t, err)
	require.Empty(t, left)
	require.Equal(t, []string{"mine"}, d.removed)
}

func TestSweepRunReportsContainerThatSurvivesRemoval(t *testing.T) {
	d := newSweepDocker()
	d.add("stuck", "run-a", true)
	d.stubborn["stuck"] = true
	s := newSweepSupervisor(d)

	left, err := s.SweepRun(context.Background(), RunOptions{RunID: "run-a"})
	require.NoError(t, err)
	require.Equal(t, []ResourceRef{{Kind: "container", ID: "stuck"}}, left)
}

func TestSweepRunNothingToDo(t *testing.T) {
	d := newSweepDocker()
	d.add("other-run", "run-b", true)
	s := newSweepSupervisor(d)

	left, err := s.SweepRun(context.Background(), RunOptions{RunID: "run-a"})
	require.NoError(t, err)
	require.Empty(t, left)
	require.Empty(t, d.removed)
}

func TestSweepRunRejectsInvalidRunID(t *testing.T) {
	s := newSweepSupervisor(newSweepDocker())
	_, err := s.SweepRun(context.Background(), RunOptions{RunID: "../x"})
	require.ErrorIs(t, err, ErrInvalidRunID)
}
