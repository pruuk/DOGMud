package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/GoMudEngine/GoMud/internal/playtestenv"
	"github.com/GoMudEngine/GoMud/internal/playtestrun"
)

type fakeEnv struct {
	startCalled bool
	startOpts   playtestenv.StartOptions
	startRes    playtestenv.Result
	startErr    error
	stopCalled  bool
	stopOpts    playtestenv.RunOptions
	stopErrs    []error // consumed one per Stop call; nil entries succeed
	stopCalls   int
	sweepCalled bool
	sweepLeft   []playtestenv.ResourceRef
	sweepErr    error
}

func (f *fakeEnv) SweepRun(_ context.Context, opts playtestenv.RunOptions) ([]playtestenv.ResourceRef, error) {
	f.sweepCalled = true
	f.stopOpts = opts
	return f.sweepLeft, f.sweepErr
}

func (f *fakeEnv) Start(_ context.Context, opts playtestenv.StartOptions) (playtestenv.Result, error) {
	f.startCalled = true
	f.startOpts = opts
	return f.startRes, f.startErr
}

func (f *fakeEnv) Stop(_ context.Context, opts playtestenv.RunOptions) (playtestenv.Result, error) {
	f.stopCalled = true
	f.stopOpts = opts
	var err error
	if f.stopCalls < len(f.stopErrs) {
		err = f.stopErrs[f.stopCalls]
	}
	f.stopCalls++
	return playtestenv.Result{Operation: "stop"}, err
}

func (f *fakeEnv) Status(context.Context, playtestenv.RunOptions) (playtestenv.Result, error) {
	return playtestenv.Result{Operation: "status"}, nil
}

func TestCLI_StopWritesSignalIdempotent(t *testing.T) {
	checkout := t.TempDir()
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"stop", "--checkout", checkout, "--run", "r1"}, ioDiscard(), &stderr, &fakeEnv{})
	require.Equal(t, exitOK, code)
	stopPath := filepath.Join(playtestrun.BridgeDirPath(checkout, "r1"), "stop")
	require.FileExists(t, stopPath)
	code = run(context.Background(), []string{"stop", "--checkout", checkout, "--run", "r1"}, ioDiscard(), &stderr, &fakeEnv{})
	require.Equal(t, exitOK, code)
}

func stopArgs(checkout, runID string) []string {
	return []string{"stop", "--checkout", checkout, "--run", runID}
}

// The supervising process may be dead: stop must tear the run down itself.
func TestCLI_StopTearsDownAndConfirms(t *testing.T) {
	checkout := t.TempDir()
	env := &fakeEnv{}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), stopArgs(checkout, "r1"), &stdout, &stderr, env)
	require.Equal(t, exitOK, code, stderr.String())
	require.True(t, env.stopCalled, "stop must call Supervisor.Stop itself")
	require.Equal(t, "r1", env.stopOpts.RunID)
	require.Equal(t, checkout, env.stopOpts.Checkout)
	require.True(t, env.sweepCalled, "stop must confirm no container remains")
	require.Contains(t, stdout.String(), "no containers remain")
}

func TestCLI_StopFailsLoudlyWhenContainerSurvives(t *testing.T) {
	checkout := t.TempDir()
	env := &fakeEnv{sweepLeft: []playtestenv.ResourceRef{{Kind: "container", ID: "abc123"}}}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), stopArgs(checkout, "r1"), &stdout, &stderr, env)
	require.Equal(t, exitFail, code)
	require.Contains(t, stderr.String(), "still up")
	require.Contains(t, stderr.String(), "abc123")
	require.NotContains(t, stdout.String(), "no containers remain")
}

func TestCLI_StopFailsWhenTeardownErrors(t *testing.T) {
	checkout := t.TempDir()
	env := &fakeEnv{stopErrs: []error{errors.New("cleanup incomplete: boom")}}
	var stderr bytes.Buffer
	code := run(context.Background(), stopArgs(checkout, "r1"), ioDiscard(), &stderr, env)
	require.Equal(t, exitFail, code)
	require.Contains(t, stderr.String(), "boom")
	require.True(t, env.sweepCalled, "sweep still runs so the report names what is up")
}

func TestCLI_StopFailsWhenConfirmationImpossible(t *testing.T) {
	checkout := t.TempDir()
	env := &fakeEnv{sweepErr: errors.New("docker unreachable")}
	var stderr bytes.Buffer
	code := run(context.Background(), stopArgs(checkout, "r1"), ioDiscard(), &stderr, env)
	require.Equal(t, exitFail, code)
	require.Contains(t, stderr.String(), "could not confirm")
}

func TestCLI_StopRetriesWhenSupervisorHoldsTheLock(t *testing.T) {
	checkout := t.TempDir()
	busy := fmt.Errorf("wrapped: %w", playtestenv.ErrLockBusy)
	env := &fakeEnv{stopErrs: []error{busy, busy, nil}}
	var stderr bytes.Buffer
	code := run(context.Background(), stopArgs(checkout, "r1"), ioDiscard(), &stderr, env)
	require.Equal(t, exitOK, code, stderr.String())
	require.Equal(t, 3, env.stopCalls)
}

func TestCLI_StopMarksDeadSupervisorsSidecarStopped(t *testing.T) {
	checkout := t.TempDir()
	_, err := playtestrun.WriteSidecar(checkout, playtestrun.SessionSidecar{
		RunID: "r1", Checkout: checkout, Status: playtestrun.StatusReady,
	})
	require.NoError(t, err)
	code := run(context.Background(), stopArgs(checkout, "r1"), ioDiscard(), ioDiscard(), &fakeEnv{})
	require.Equal(t, exitOK, code)
	sc, err := playtestrun.ReadSidecar(checkout, "r1")
	require.NoError(t, err)
	require.Equal(t, playtestrun.StatusStopped, sc.Status)
}

func TestCLI_StatusPrintsSidecar(t *testing.T) {
	checkout := t.TempDir()
	_, err := playtestrun.WriteSidecar(checkout, playtestrun.SessionSidecar{
		RunID:     "r2",
		Checkout:  checkout,
		Budgets:   playtestrun.SessionBudgets{WallClock: "30m"},
		Status:    playtestrun.StatusReady,
		BridgeDir: playtestrun.BridgeDirPath(checkout, "r2"),
	})
	require.NoError(t, err)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"status", "--checkout", checkout, "--run", "r2"}, &stdout, &stderr, &fakeEnv{})
	require.Equal(t, exitOK, code)
	var sc playtestrun.SessionSidecar
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &sc))
	require.Equal(t, "r2", sc.RunID)
	require.Equal(t, "30m", sc.Budgets.WallClock)
}

func TestCLI_RunMissingPersonality(t *testing.T) {
	env := &fakeEnv{}
	var stderr bytes.Buffer
	code := run(context.Background(), []string{
		"run", "--checkout", t.TempDir(), "--goals", "g.yaml",
	}, ioDiscard(), &stderr, env)
	require.Equal(t, exitUsage, code)
	require.False(t, env.startCalled)
}

func TestCLI_RunMissingCheckout(t *testing.T) {
	env := &fakeEnv{}
	var stderr bytes.Buffer
	code := run(context.Background(), []string{
		"run", "--goals", "g.yaml", "--personality", "bug-finder",
	}, ioDiscard(), &stderr, env)
	require.Equal(t, exitUsage, code)
	require.False(t, env.startCalled)
}

func ioDiscard() *bytes.Buffer { return &bytes.Buffer{} }
