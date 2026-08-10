package moderation

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

type AccountBan struct {
	Username  string    `yaml:"username"`
	Reason    string    `yaml:"reason"`
	BannedBy  string    `yaml:"banned_by"`
	Timestamp time.Time `yaml:"timestamp"`
}

type IPBan struct {
	IP        string    `yaml:"ip"`
	Reason    string    `yaml:"reason"`
	BannedBy  string    `yaml:"banned_by"`
	Timestamp time.Time `yaml:"timestamp"`
}

// banFile is the on-disk shape of bans.yaml.
type banFile struct {
	Accounts []AccountBan `yaml:"accounts"`
	IPs      []IPBan      `yaml:"ips"`
}

var (
	accountBans = map[string]AccountBan{} // key: lowercased username
	ipBans      = map[string]IPBan{}      // key: exact host/ip
)

func bansPath() string { return filepath.Join(moderationDir(), "bans.yaml") }

// normAccountKey is the single source of truth for the account-ban map key, so
// the store, lookup, and reload paths can never drift (a drift would silently
// break bans across a restart).
func normAccountKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// BanAccount records an account ban. The in-memory map is rolled back if the
// save fails, so memory can never claim a ban that is not on disk: such a ban
// would silently disappear on the next restart while staff believed it applied
// (review finding 7).
func BanAccount(username, reason, by string) error {
	mu.Lock()
	defer mu.Unlock()
	trimmed := strings.TrimSpace(username)
	key := normAccountKey(trimmed)
	prev, had := accountBans[key]
	accountBans[key] = AccountBan{Username: trimmed, Reason: reason, BannedBy: by, Timestamp: now()}
	if err := saveBansLocked(); err != nil {
		restoreAccountBan(key, prev, had)
		return err
	}
	return nil
}

// Unban lifts an account ban, rolling back if the save fails. An unban that
// did not reach disk must not look applied: the ban would return on restart.
func Unban(username string) error {
	mu.Lock()
	defer mu.Unlock()
	key := normAccountKey(username)
	prev, had := accountBans[key]
	delete(accountBans, key)
	if err := saveBansLocked(); err != nil {
		restoreAccountBan(key, prev, had)
		return err
	}
	return nil
}

func IsAccountBanned(username string) (reason string, banned bool) {
	mu.Lock()
	defer mu.Unlock()
	if b, ok := accountBans[normAccountKey(username)]; ok {
		return b.Reason, true
	}
	return "", false
}

// BanIP records an IP ban, rolling back if the save fails. See BanAccount.
func BanIP(ip, reason, by string) error {
	mu.Lock()
	defer mu.Unlock()
	ip = strings.TrimSpace(ip)
	prev, had := ipBans[ip]
	ipBans[ip] = IPBan{IP: ip, Reason: reason, BannedBy: by, Timestamp: now()}
	if err := saveBansLocked(); err != nil {
		restoreIPBan(ip, prev, had)
		return err
	}
	return nil
}

// UnbanIP lifts an IP ban, rolling back if the save fails. See Unban.
func UnbanIP(ip string) error {
	mu.Lock()
	defer mu.Unlock()
	key := strings.TrimSpace(ip)
	prev, had := ipBans[key]
	delete(ipBans, key)
	if err := saveBansLocked(); err != nil {
		restoreIPBan(key, prev, had)
		return err
	}
	return nil
}

// restoreAccountBan / restoreIPBan put a map entry back exactly as it was.
// "had" distinguishes "there was no entry" from "there was a zero-valued
// entry", so a rollback cannot invent a ban that never existed.
// Callers must already hold mu.
func restoreAccountBan(key string, prev AccountBan, had bool) {
	if had {
		accountBans[key] = prev
		return
	}
	delete(accountBans, key)
}

func restoreIPBan(key string, prev IPBan, had bool) {
	if had {
		ipBans[key] = prev
		return
	}
	delete(ipBans, key)
}

func IsIPBanned(host string) (reason string, banned bool) {
	mu.Lock()
	defer mu.Unlock()
	if b, ok := ipBans[strings.TrimSpace(host)]; ok {
		return b.Reason, true
	}
	return "", false
}

func saveBansLocked() error {
	if err := os.MkdirAll(moderationDir(), 0755); err != nil {
		mudlog.Error("moderation.saveBans", "error", err.Error())
		return err
	}
	bf := banFile{}
	for _, b := range accountBans {
		bf.Accounts = append(bf.Accounts, b)
	}
	for _, b := range ipBans {
		bf.IPs = append(bf.IPs, b)
	}
	out, err := yaml.Marshal(bf)
	if err != nil {
		mudlog.Error("moderation.saveBans", "error", err.Error())
		return err
	}
	// Durable atomic write: bans are living state, and a torn bans.yaml would
	// resurrect unbanned accounts or drop active bans (living-state contract,
	// internal/util/livingstate.go).
	if err := util.Save(bansPath(), out); err != nil {
		mudlog.Error("moderation.saveBans", "error", err.Error())
		return err
	}
	return nil
}

// loadBans returns the number of account bans loaded.
func loadBans() int {
	mu.Lock()
	defer mu.Unlock()
	accountBans = map[string]AccountBan{}
	ipBans = map[string]IPBan{}
	b, err := os.ReadFile(bansPath())
	if err != nil {
		return 0
	}
	var bf banFile
	if err := yaml.Unmarshal(b, &bf); err != nil {
		mudlog.Error("moderation.loadBans", "error", err.Error())
		return 0
	}
	for _, a := range bf.Accounts {
		accountBans[normAccountKey(a.Username)] = a
	}
	for _, i := range bf.IPs {
		ipBans[i.IP] = i
	}
	return len(accountBans)
}
