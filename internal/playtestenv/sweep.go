package playtestenv

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// SweepRun is the last-line check behind `playtestrun stop`. It lists every
// container carrying this run's managed and run-id labels (running or not),
// force-removes each one, lists again, and returns the containers that are
// still present. Only containers whose own labels name this exact run are ever
// removed, so another run's or the owner's containers are never touched.
//
// Stop already tears down the containers its manifest names. SweepRun covers
// the gap a manifest cannot: a container the manifest does not record (a run
// killed mid-start) or a terminal manifest whose idempotent shortcut skipped
// cleanup. An empty result with a nil error means no container for the run
// remains.
func (s *Supervisor) SweepRun(ctx context.Context, opts RunOptions) ([]ResourceRef, error) {
	if !isValidRunID(opts.RunID) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidRunID, opts.RunID)
	}
	dc, err := s.deps.resolveDocker(ctx, s.deps.runner)
	if err != nil {
		return nil, err
	}
	found, err := s.listRunContainers(ctx, dc, opts.RunID)
	if err != nil {
		return nil, err
	}
	for _, c := range found {
		var stderr strings.Builder
		spec := dockerCommand(dc, []string{"rm", "--force", c.ID}, "", io.Discard, &stderr)
		// A failed rm is judged by the re-list below, which is the evidence.
		_ = s.deps.runner.Run(ctx, spec)
	}
	if len(found) == 0 {
		return nil, nil
	}
	left, err := s.listRunContainers(ctx, dc, opts.RunID)
	if err != nil {
		return nil, err
	}
	refs := make([]ResourceRef, 0, len(left))
	for _, c := range left {
		refs = append(refs, ResourceRef{Kind: "container", ID: c.ID})
	}
	return refs, nil
}

func (s *Supervisor) listRunContainers(ctx context.Context, dc dockerContext, runID string) ([]orphanResource, error) {
	var stdout, stderr strings.Builder
	spec := dockerCommand(dc, []string{
		"ps", "-a",
		"--filter", managedLabelFilter,
		"--filter", "label=" + labelRunID + "=" + runID,
		"--format", "{{json .}}",
	}, "", &stdout, &stderr)
	if err := s.deps.runner.Run(ctx, spec); err != nil {
		return nil, fmt.Errorf("playtestenv: list containers for run %s: %w", runID, err)
	}
	items, err := parseDockerListJSON(stdout.String(), "container")
	if err != nil {
		return nil, err
	}
	// Defence in depth: never act on a row whose labels do not name this run.
	var out []orphanResource
	for _, it := range items {
		if it.Labels[labelRunID] == runID && it.Labels[labelManaged] == labelManagedValue {
			out = append(out, it)
		}
	}
	return out, nil
}
