package web

import (
	"net/http"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Finding 20a — HTTP servers were built with no timeouts at all, so a slow
// client could hold a connection open indefinitely.
// ---------------------------------------------------------------------------

// This is the state the servers shipped in: every timeout field at its zero
// value, which Go reads as "no limit".
func TestApplyServerTimeouts_BareServerHasNoLimits(t *testing.T) {
	srv := &http.Server{Addr: ":8080"}

	require.Zero(t, srv.ReadHeaderTimeout, "guard: a bare server has no header timeout")
	require.Zero(t, srv.ReadTimeout)
	require.Zero(t, srv.WriteTimeout)
	require.Zero(t, srv.IdleTimeout)
}

func TestApplyServerTimeouts_SetsEveryTimeout(t *testing.T) {
	srv := &http.Server{Addr: ":8080"}

	applyServerTimeouts(srv, configs.Network{
		HttpReadHeaderTimeoutSeconds: 5,
		HttpReadTimeoutSeconds:       15,
		HttpWriteTimeoutSeconds:      25,
		HttpIdleTimeoutSeconds:       35,
	})

	assert.Equal(t, 5*time.Second, srv.ReadHeaderTimeout)
	assert.Equal(t, 15*time.Second, srv.ReadTimeout)
	assert.Equal(t, 25*time.Second, srv.WriteTimeout)
	assert.Equal(t, 35*time.Second, srv.IdleTimeout)
}

// The point of the defaults: an operator who never touches config.yaml must
// still get bounded timeouts, not zeroes.
func TestNetworkValidate_TimeoutsDefaultToNonZero(t *testing.T) {
	n := configs.Network{}
	n.Validate()

	assert.Equal(t, configs.ConfigInt(10), n.HttpReadHeaderTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(30), n.HttpReadTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(60), n.HttpWriteTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(120), n.HttpIdleTimeoutSeconds)

	srv := &http.Server{}
	applyServerTimeouts(srv, n)

	assert.NotZero(t, srv.ReadHeaderTimeout, "defaults must reach the server")
	assert.NotZero(t, srv.ReadTimeout)
	assert.NotZero(t, srv.WriteTimeout)
	assert.NotZero(t, srv.IdleTimeout)
}

// A negative or zero value in config.yaml must not silently disable a security
// timeout, so Validate replaces it with the default.
func TestNetworkValidate_NonPositiveTimeoutsFallBackToDefaults(t *testing.T) {
	n := configs.Network{
		HttpReadHeaderTimeoutSeconds: 0,
		HttpReadTimeoutSeconds:       -1,
		HttpWriteTimeoutSeconds:      -99,
		HttpIdleTimeoutSeconds:       0,
	}
	n.Validate()

	assert.Equal(t, configs.ConfigInt(10), n.HttpReadHeaderTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(30), n.HttpReadTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(60), n.HttpWriteTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(120), n.HttpIdleTimeoutSeconds)
}

// An explicitly configured value must survive Validate untouched.
func TestNetworkValidate_ExplicitTimeoutsAreKept(t *testing.T) {
	n := configs.Network{
		HttpReadHeaderTimeoutSeconds: 3,
		HttpReadTimeoutSeconds:       7,
		HttpWriteTimeoutSeconds:      11,
		HttpIdleTimeoutSeconds:       13,
	}
	n.Validate()

	assert.Equal(t, configs.ConfigInt(3), n.HttpReadHeaderTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(7), n.HttpReadTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(11), n.HttpWriteTimeoutSeconds)
	assert.Equal(t, configs.ConfigInt(13), n.HttpIdleTimeoutSeconds)
}

func TestApplyServerTimeouts_NilServerIsSafe(t *testing.T) {
	assert.NotPanics(t, func() {
		applyServerTimeouts(nil, configs.Network{})
	})
}
