package usercommands

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// helpDataRoot walks up from the test working directory to find the
// _datafiles folder and returns the dogmud world root. It validates that
// the found _datafiles contains world/dogmud to avoid false positives.
func helpDataRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get working directory: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "_datafiles")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			// Verify this is the correct _datafiles by checking for world/dogmud
			worldDogmud := filepath.Join(candidate, "world", "dogmud")
			if info, err := os.Stat(worldDogmud); err == nil && info.IsDir() {
				return worldDogmud
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("cannot find _datafiles/world/dogmud directory from %s", dir)
		}
		dir = parent
	}
}

// helpFileExistsAt checks whether a file exists at one of the given paths.
// Returns true if any of them exists.
func helpFileExistsAt(paths ...string) bool {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// commandHelpAliases maps a command name to another command name whose
// help file it shares. Used when two commands map to the same handler
// function and a single help template covers both.
var commandHelpAliases = map[string]string{
	"companions": "companion",
	"stomp":      "kick",
	"knee":       "kick",
	"tailsweep":  "trip",
	"rep":        "report",
	"unhood":     "hood",
}

// commandHelpSkip lists internal/debug commands that don't need
// player-facing help files. These are not typed directly by players
// (e.g. context dispatchers, no-ops, account-creation flow commands).
var commandHelpSkip = map[string]bool{
	"default":   true, // context-sensitive dispatcher
	"noop":      true, // does nothing
	"start":     true, // account creation only
	"zombieact": true, // internal zombie AI
	"print":     true, // debug echo
	"printline": true, // debug echo
}

// TestHelpFileCompleteness_Commands ensures every registered user command
// has a matching help template. Regular commands live at
// help/<name>.template. Admin commands live at
// admincommands/help/command.<name>.template (or .md).
func TestHelpFileCompleteness_Commands(t *testing.T) {
	root := helpDataRoot(t)
	userHelpDir := filepath.Join(root, "templates", "help")
	adminHelpDir := filepath.Join(root, "templates", "admincommands", "help")

	var missing []string
	for name, info := range userCommands {
		if commandHelpSkip[name] {
			continue
		}
		target := name
		if alias, ok := commandHelpAliases[name]; ok {
			target = alias
		}

		if info.AdminOnly {
			tmpl := filepath.Join(adminHelpDir, "command."+target+".template")
			md := filepath.Join(adminHelpDir, "command."+target+".md")
			if !helpFileExistsAt(tmpl, md) {
				missing = append(missing, name+" (admin) — expected "+
					filepath.Join("admincommands", "help", "command."+target+".template"))
			}
		} else {
			tmpl := filepath.Join(userHelpDir, target+".template")
			md := filepath.Join(userHelpDir, target+".md")
			if !helpFileExistsAt(tmpl, md) {
				missing = append(missing, name+" (user) — expected "+
					filepath.Join("help", target+".template"))
			}
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("commands missing help files (%d):\n  %s\n\n"+
			"Add a template (a short stub is fine) or add the command to commandHelpSkip "+
			"if it's truly internal, or commandHelpAliases if it shares a help file with "+
			"another command.",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// moduleUserCommandCall and moduleRegisterCommandCall match the two ways a
// module registers a user command:
//
//	plug.AddUserCommand(`name`, handler, allowWhenDowned, isAdminOnly)
//	usercommands.RegisterCommand(`name`, handler, disabledWhenDowned, allowedInCombat, isAdminOnly)
var (
	moduleUserCommandCall = regexp.MustCompile(
		"AddUserCommand\\(\\s*[`\"]([^`\"]+)[`\"]\\s*,\\s*[^,]+,\\s*(?:true|false)\\s*,\\s*(true|false)\\s*\\)")
	moduleRegisterCommandCall = regexp.MustCompile(
		"usercommands\\.RegisterCommand\\(\\s*[`\"]([^`\"]+)[`\"]\\s*,\\s*[^,]+,\\s*(?:true|false)\\s*,\\s*(?:true|false)\\s*,\\s*(true|false)\\s*\\)")
)

// TestHelpFileCompleteness_ModuleCommands is TestHelpFileCompleteness_Commands
// for the commands modules register (#362). Those reach userCommands only when
// plugins.Load runs at boot, never in a test binary, so they are read from the
// module sources. Their help may live in the world templates or in the
// module's own embedded templates, which the help command reads first.
func TestHelpFileCompleteness_ModuleCommands(t *testing.T) {
	root := helpDataRoot(t) // <repo>/_datafiles/world/dogmud
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(root)))
	modulesDir := filepath.Join(repoRoot, "modules")

	type moduleCommand struct {
		name, module string
		admin        bool
	}
	var found []moduleCommand
	err := filepath.WalkDir(modulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(modulesDir, path)
		if err != nil {
			return err
		}
		module := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range moduleUserCommandCall.FindAllStringSubmatch(string(src), -1) {
			found = append(found, moduleCommand{name: m[1], module: module, admin: m[2] == "true"})
		}
		for _, m := range moduleRegisterCommandCall.FindAllStringSubmatch(string(src), -1) {
			found = append(found, moduleCommand{name: m[1], module: module, admin: m[2] == "true"})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", modulesDir, err)
	}

	// The scan must be able to find what is there: one plugin-helper
	// command, one admin one, and one direct registration.
	seen := map[string]bool{}
	for _, c := range found {
		seen[c.name] = true
	}
	for _, want := range []string{"auction", "ai-flag", "companion-stay"} {
		if !seen[want] {
			t.Fatalf("the module scan did not find %q; the patterns no longer match how modules register commands (found %d)", want, len(found))
		}
	}

	userHelpDir := filepath.Join(root, "templates", "help")
	adminHelpDir := filepath.Join(root, "templates", "admincommands", "help")
	var missing []string
	for _, c := range found {
		if commandHelpSkip[c.name] {
			continue
		}
		target := c.name
		if alias, ok := commandHelpAliases[c.name]; ok {
			target = alias
		}
		moduleTemplates := filepath.Join(modulesDir, c.module, "files", "datafiles", "templates")
		if c.admin {
			if !helpFileExistsAt(
				filepath.Join(adminHelpDir, "command."+target+".template"),
				filepath.Join(adminHelpDir, "command."+target+".md"),
				filepath.Join(moduleTemplates, "admincommands", "help", "command."+target+".template"),
			) {
				missing = append(missing, c.name+" (admin, module "+c.module+")")
			}
			continue
		}
		if !helpFileExistsAt(
			filepath.Join(userHelpDir, target+".template"),
			filepath.Join(userHelpDir, target+".md"),
			filepath.Join(moduleTemplates, "help", target+".template"),
		) {
			missing = append(missing, c.name+" (module "+c.module+") expected "+
				filepath.Join("modules", c.module, "files", "datafiles", "templates", "help", target+".template"))
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("module commands missing help files (%d):\n  %s", len(missing), strings.Join(missing, "\n  "))
	}
}

// TestHelpFileCompleteness_IndexListsEveryCommand: `help` lists only what
// keywords.yaml indexes, so a command with a help file but no index entry is
// found only by a player who already knows its name. trade and tutorial were
// (#236). Every non-admin command with a help file is listed, directly,
// through the help file it shares (commandHelpAliases), or as a help alias.
func TestHelpFileCompleteness_IndexListsEveryCommand(t *testing.T) {
	root := helpDataRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "keywords.yaml"))
	if err != nil {
		t.Fatalf("reading keywords.yaml: %v", err)
	}
	var kw struct {
		Help        map[string]map[string][]string `yaml:"help"`
		HelpAliases map[string][]string            `yaml:"help-aliases"`
	}
	if err := yaml.Unmarshal(raw, &kw); err != nil {
		t.Fatalf("parsing keywords.yaml: %v", err)
	}
	indexed := map[string]bool{}
	for _, categories := range kw.Help {
		for _, list := range categories {
			for _, c := range list {
				indexed[strings.ToLower(c)] = true
			}
		}
	}
	aliasOf := map[string]string{}
	for topic, list := range kw.HelpAliases {
		for _, a := range list {
			aliasOf[strings.ToLower(a)] = strings.ToLower(topic)
		}
	}
	if len(indexed) < 100 {
		t.Fatalf("read only %d indexed topics; the keywords.yaml shape changed", len(indexed))
	}

	userHelpDir := filepath.Join(root, "templates", "help")
	var unlisted []string
	checked := 0
	for name, info := range userCommands {
		if info.AdminOnly || commandHelpSkip[name] {
			continue
		}
		target := name
		if alias, ok := commandHelpAliases[name]; ok {
			target = alias
		}
		if !helpFileExistsAt(filepath.Join(userHelpDir, target+".template"), filepath.Join(userHelpDir, target+".md")) {
			continue // TestHelpFileCompleteness_Commands reports a missing file
		}
		checked++
		if indexed[name] || indexed[target] || (aliasOf[name] != "" && indexed[aliasOf[name]]) {
			continue
		}
		unlisted = append(unlisted, name)
	}
	if checked < 100 {
		t.Fatalf("checked only %d commands; the scan cannot see the commands it guards", checked)
	}
	if len(unlisted) > 0 {
		sort.Strings(unlisted)
		t.Errorf("commands with a help file that `help` never lists (%d): %s\n"+
			"Add each under a category in _datafiles/world/dogmud/keywords.yaml.",
			len(unlisted), strings.Join(unlisted, ", "))
	}
}
