package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/auththrottle"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
)

func TestMain(m *testing.M) {
	mudlog.SetupLogger(nil, "", "", false)
	os.Exit(m.Run())
}

func TestDoBasicAuthRequiresAdminRole(t *testing.T) {
	const password = "correct-password"

	setupAuthTestUsers(t, password, map[string]string{
		"adminuser":   users.RoleAdmin,
		"builderuser": "builder",
		"normaluser":  users.RoleUser,
	})

	tests := []struct {
		name       string
		username   string
		wantStatus int
		wantCalled bool
	}{
		{
			name:       "admin accepted",
			username:   "adminuser",
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name:       "builder rejected",
			username:   "builderuser",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "user rejected",
			username:   "normaluser",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAuthStateForTest()
			called := false
			handler := doBasicAuth(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
			req.SetBasicAuth(tt.username, password)
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Fatalf("handler called = %t, want %t", called, tt.wantCalled)
			}
		})
	}
}

// setupAuthTestUsers wires up a temporary DataFiles directory with an index
// and per-user YAML files, then redirects the configs DataFiles path to it
// for the duration of the test.
func setupAuthTestUsers(t *testing.T, password string, roles map[string]string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	// ReloadConfig reads _datafiles/config.yaml from CWD; we must be at repo root.
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir(%q): %v", repoRoot, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("Chdir cleanup: %v", err)
		}
	})

	// Restore real config after the test (registered before t.Setenv so it
	// runs after the env var is restored, i.e. without the override path set).
	t.Cleanup(func() {
		resetAuthStateForTest()
	})

	root := t.TempDir()
	dataDir := filepath.Join(root, "world")
	usersDir := filepath.Join(dataDir, "users")
	if err := os.MkdirAll(usersDir, 0700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", usersDir, err)
	}

	// Use forward slashes so the YAML scalar is unambiguous on all platforms.
	dataDirSlash := filepath.ToSlash(dataDir)
	overridePath := filepath.Join(root, "config-overrides.yaml")
	overrideBytes := []byte("FilePaths:\n  DataFiles: " + dataDirSlash + "\n  CarefulSaveFiles: false\n")
	if err := os.WriteFile(overridePath, overrideBytes, 0600); err != nil {
		t.Fatalf("WriteFile(%q): %v", overridePath, err)
	}

	t.Setenv("CONFIG_PATH", filepath.ToSlash(overridePath))
	if err := configs.ReloadConfigForTest(t); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	// Build the binary user index now that DataFiles points to our temp dir.
	idx := users.NewUserIndex()
	if err := idx.Create(); err != nil {
		t.Fatalf("Create user index: %v", err)
	}

	userID := 1
	for username, role := range roles {
		if err := idx.AddUser(userID, username); err != nil {
			t.Fatalf("AddUser(%q): %v", username, err)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("GenerateFromPassword: %v", err)
		}

		u := users.NewUserRecord(userID, 0)
		u.Username = username
		u.Role = role
		u.Password = string(hash)

		data, err := yaml.Marshal(u)
		if err != nil {
			t.Fatalf("Marshal user %q: %v", username, err)
		}
		userPath := filepath.Join(usersDir, strconv.Itoa(userID)+".yaml")
		if err := os.WriteFile(userPath, data, 0600); err != nil {
			t.Fatalf("WriteFile(%q): %v", userPath, err)
		}

		userID++
	}
}

func resetAuthStateForTest() {
	authMu.Lock()
	defer authMu.Unlock()
	authCache = map[string]authCacheEntry{}
	adminThrottle.ResetForTest()
}

// rewriteAuthTestUser overwrites an existing test user's YAML in place, which
// is how a password change / role demotion / rotation looks to LoadUser.
func rewriteAuthTestUser(t *testing.T, userID int, username, role, password string) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	u := users.NewUserRecord(userID, 0)
	u.Username = username
	u.Role = role
	u.Password = string(hash)

	data, err := yaml.Marshal(u)
	if err != nil {
		t.Fatalf("Marshal user %q: %v", username, err)
	}

	usersDir := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "users")
	userPath := filepath.Join(usersDir, strconv.Itoa(userID)+".yaml")
	if err := os.WriteFile(userPath, data, 0600); err != nil {
		t.Fatalf("WriteFile(%q): %v", userPath, err)
	}
}

// forceAuthRevalidation ages every cached grant so the next lookup re-reads the
// user record. Without this the test would have to sleep for
// authRevalidateEvery.
func forceAuthRevalidation() {
	authMu.Lock()
	defer authMu.Unlock()
	for k, e := range authCache {
		e.nextRevalidate = time.Now().Add(-time.Second)
		authCache[k] = e
	}
}

func doAuthRequest(t *testing.T, username, password, remoteAddr string) (int, bool) {
	t.Helper()

	called := false
	handler := doBasicAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/build", nil)
	req.SetBasicAuth(username, password)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	rec := httptest.NewRecorder()
	handler(rec, req)

	return rec.Code, called
}

// A cached grant must not outlive the credential it was issued against. The old
// implementation keyed on the raw Authorization header and never looked at the
// account again, so a rotated password kept working for the rest of the TTL.
func TestAuthCacheInvalidatedByPasswordChange(t *testing.T) {
	const password = "correct-password"
	setupAuthTestUsers(t, password, map[string]string{"adminuser": users.RoleAdmin})
	resetAuthStateForTest()

	if code, called := doAuthRequest(t, "adminuser", password, "203.0.113.10:5000"); code != http.StatusOK || !called {
		t.Fatalf("initial auth: status=%d called=%t, want 200/true", code, called)
	}

	// Served from cache without touching bcrypt.
	if code, called := doAuthRequest(t, "adminuser", password, "203.0.113.10:5000"); code != http.StatusOK || !called {
		t.Fatalf("cached auth: status=%d called=%t, want 200/true", code, called)
	}

	rewriteAuthTestUser(t, 1, "adminuser", users.RoleAdmin, "brand-new-password")
	forceAuthRevalidation()

	if code, called := doAuthRequest(t, "adminuser", password, "203.0.113.10:5000"); code != http.StatusUnauthorized || called {
		t.Fatalf("after password change: status=%d called=%t, want 401/false", code, called)
	}
}

// A demotion out of RoleAdmin must void an outstanding grant too.
func TestAuthCacheInvalidatedByRoleDemotion(t *testing.T) {
	const password = "correct-password"
	setupAuthTestUsers(t, password, map[string]string{"adminuser": users.RoleAdmin})
	resetAuthStateForTest()

	if code, called := doAuthRequest(t, "adminuser", password, "203.0.113.11:5000"); code != http.StatusOK || !called {
		t.Fatalf("initial auth: status=%d called=%t, want 200/true", code, called)
	}

	rewriteAuthTestUser(t, 1, "adminuser", users.RoleUser, password)
	forceAuthRevalidation()

	if code, called := doAuthRequest(t, "adminuser", password, "203.0.113.11:5000"); code != http.StatusUnauthorized || called {
		t.Fatalf("after demotion: status=%d called=%t, want 401/false", code, called)
	}
}

// bcrypt is deliberately expensive; unthrottled, doBasicAuth is a guessing
// oracle and a CPU-exhaustion vector. After authMaxFailures the source must be
// rejected with 429 before any bcrypt work happens.
func TestFailedAuthIsThrottledPerSource(t *testing.T) {
	const password = "correct-password"
	setupAuthTestUsers(t, password, map[string]string{"adminuser": users.RoleAdmin})
	resetAuthStateForTest()

	const attacker = "198.51.100.7:6000"

	for i := range auththrottle.MaxFailures {
		if code, _ := doAuthRequest(t, "adminuser", "wrong-password", attacker); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status=%d, want 401", i+1, code)
		}
	}

	code, called := doAuthRequest(t, "adminuser", "wrong-password", attacker)
	if code != http.StatusTooManyRequests || called {
		t.Fatalf("post-lockout: status=%d called=%t, want 429/false", code, called)
	}

	// Even the correct password is refused while locked out...
	if code, _ := doAuthRequest(t, "adminuser", password, attacker); code != http.StatusTooManyRequests {
		t.Fatalf("locked-out correct password: status=%d, want 429", code)
	}

	// ...but a different source is unaffected.
	if code, called := doAuthRequest(t, "adminuser", password, "198.51.100.8:6000"); code != http.StatusOK || !called {
		t.Fatalf("other source: status=%d called=%t, want 200/true", code, called)
	}
}

// Admin requests must come from the site's own pages. A browser re-sends cached
// Basic credentials on its own, so a valid credential proves nothing about
// which page sent a state-changing request. The dashboards' own POSTs are
// same-origin fetch calls and must keep working; reads stay allowed from
// anywhere because they change nothing.
func TestAdminRefusesCrossSiteStateChanges(t *testing.T) {
	const password = "correct-password"
	setupAuthTestUsers(t, password, map[string]string{"adminuser": users.RoleAdmin})

	cases := []struct {
		name       string
		method     string
		headers    map[string]string
		wantStatus int
		wantCalled bool
	}{
		{"cross-site POST by Sec-Fetch-Site", http.MethodPost,
			map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://other.example"},
			http.StatusForbidden, false},
		{"cross-site POST by Origin alone", http.MethodPost,
			map[string]string{"Origin": "https://other.example"},
			http.StatusForbidden, false},
		{"same-origin POST from a dashboard", http.MethodPost,
			map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://example.com"},
			http.StatusOK, true},
		{"same-origin POST by Origin alone", http.MethodPost,
			map[string]string{"Origin": "http://example.com"},
			http.StatusOK, true},
		{"POST from a non-browser client", http.MethodPost,
			nil, http.StatusOK, true},
		{"cross-site GET reads", http.MethodGet,
			map[string]string{"Sec-Fetch-Site": "cross-site"},
			http.StatusOK, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetAuthStateForTest()
			called := false
			h := doBasicAuth(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			// httptest requests carry Host example.com.
			req := httptest.NewRequest(tc.method, "/admin/api/combat-stats/reset", nil)
			req.SetBasicAuth("adminuser", password)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h(rec, req)

			if rec.Code != tc.wantStatus || called != tc.wantCalled {
				t.Fatalf("status=%d called=%t, want %d/%t", rec.Code, called, tc.wantStatus, tc.wantCalled)
			}
		})
	}
}

// No admin response may be framed by another page. The policy is set before
// anything else, so it rides on a refusal and a challenge as well as on a page,
// and it survives the world-lock wrapper's buffered response.
func TestAdminResponsesForbidFraming(t *testing.T) {
	const password = "correct-password"
	setupAuthTestUsers(t, password, map[string]string{"adminuser": users.RoleAdmin})
	resetAuthStateForTest()

	page := doBasicAuth(RunWithMUDLocked(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	}))

	authed := httptest.NewRequest(http.MethodGet, "/admin/items/", nil)
	authed.SetBasicAuth("adminuser", password)

	challenged := httptest.NewRequest(http.MethodGet, "/admin/items/", nil)

	refused := httptest.NewRequest(http.MethodPost, "/admin/api/economy/snapshot", nil)
	refused.SetBasicAuth("adminuser", password)
	refused.Header.Set("Sec-Fetch-Site", "cross-site")

	for name, req := range map[string]*http.Request{"page": authed, "challenge": challenged, "refusal": refused} {
		rec := httptest.NewRecorder()
		page(rec, req)
		if got := rec.Header().Get("Content-Security-Policy"); got != `frame-ancestors 'none'` {
			t.Errorf("%s (status %d): Content-Security-Policy = %q, want frame-ancestors 'none'", name, rec.Code, got)
		}
	}
}
