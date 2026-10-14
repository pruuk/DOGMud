# M6 Rows 26-27: Combat Lines as One Record per Blow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every combat-message slot shows the attacker, defender and room the same blow, and the store's layout makes a mismatch impossible to author.

**Architecture:** A yaml.v2 decoder on `TogetherMessages` / `SeparateMessages` reads a tier-then-records layout into the existing per-role pools, so render code is untouched. Each batch of files is first converted losslessly (golden byte-identical), then audited by file, reviewed by the owner on a page, and fixed by swap, move or approved rewrite; a checker proves nothing else changed and a banner-word guard catches mismatches the layout cannot.

**Tech Stack:** Go (`internal/items`, root guards), yaml.v2, Python read-only tools in `tools/`.

**Spec:** `docs/superpowers/specs/2026-10-13-messaging-m6-rows26-27-combat-triads-design.md` (facts table at the top).

---

## Rules for every task (restate in every subagent brief)

- Edit/Write tools only for text edits. No `sed -i`, no shell heredoc appends, no Python that writes a file, no `gofmt -w`. A YAML file is converted by writing it with the Write tool.
- Named-path `git add` only. No em or en dashes in prose or copy.
- Worktree `C:/tmp/dogmud-m6r26`, branch `feature/m6-row26-tier-bleed` for batch 1 (later batches: a new branch from master per batch, named in the batch brief). Do not push. Do not start or stop servers.
- After every task run the touched packages' tests AND `go test .` (root guards).
- Commit messages end with a blank line then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Python tools are READ ONLY over YAML (never `yaml.dump`).

## File map

| File | Responsibility |
|---|---|
| `internal/items/attack_records.go` (new) | `decodeTieredRecords`, role and tier name lists, `UnmarshalYAML` for `AttackTypes`, `TogetherMessages`, `SeparateMessages` |
| `internal/items/attack_records_test.go` (new) | decoder tests |
| `combat_banner_agreement_guard_test.go` (new, repo root) | banner-word guard and its unit tests |
| `tools/combat_message_pad_check.py` | record layout, `--ignore-tier`, `--manifest` |
| `tools/combat_message_pool_audit.py` | record layout |
| `internal/combat/coup_de_grace_test.go` | decode through `items.WeaponAttackMessageGroup` |
| `_datafiles/world/dogmud/combat-messages/<file>.yaml` | converted, then fixed, per batch |
| `docs/superpowers/audits/m6-rows26-27-batch-<N>.json` (new per batch) | approved proposals; its `rewrite` entries are the checker manifest |
| `internal/narration/testdata/stores/combat_messages.golden` | regenerated after fixes |

---

## Part A: tooling (batch 1 only)

### Task 1: Record decoder in `items` (TDD)

**Files:** Create `internal/items/attack_records.go`, `internal/items/attack_records_test.go`.

- [ ] **Step 1: Failing tests.** Write `internal/items/attack_records_test.go`:

```go
package items

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

const roleFirstDoc = `
optionid: test
options:
  weak:
    together:
      actor:
        beginner: ['A0', 'A1']
        master: ['A2']
      actee:
        beginner: ['D0', 'D1']
        master: ['D2']
      observer:
        beginner: ['R0', 'R1']
        master: ['R2']
    separate:
      actor: {beginner: ['SA0']}
      actee: {beginner: ['SD0']}
      observer: {beginner: ['SR0']}
      remote_observer: {beginner: ['SX0']}
`

const recordDoc = `
optionid: test
options:
  weak:
    together:
      beginner:
      - {actor: 'A0', actee: 'D0', observer: 'R0'}
      - {actor: 'A1', actee: 'D1', observer: 'R1'}
      master:
      - {actor: 'A2', actee: 'D2', observer: 'R2'}
    separate:
      beginner:
      - {actor: 'SA0', actee: 'SD0', observer: 'SR0', remote_observer: 'SX0'}
`

func decodeGroup(t *testing.T, doc string) (WeaponAttackMessageGroup, error) {
	t.Helper()
	var g WeaponAttackMessageGroup
	err := yaml.Unmarshal([]byte(doc), &g)
	return g, err
}

func TestRecordLayoutDecodesToTheSamePoolsAsRoleFirst(t *testing.T) {
	a, err := decodeGroup(t, roleFirstDoc)
	if err != nil {
		t.Fatalf("role-first: %v", err)
	}
	b, err := decodeGroup(t, recordDoc)
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	for _, lvl := range []int{0, 34, 67} {
		at, bt := a.Options[Weak].Together, b.Options[Weak].Together
		for name, pair := range map[string][2][]string{
			"actor":    {at.ToAttacker.PoolFor(lvl), bt.ToAttacker.PoolFor(lvl)},
			"actee":    {at.ToDefender.PoolFor(lvl), bt.ToDefender.PoolFor(lvl)},
			"observer": {at.ToRoom.PoolFor(lvl), bt.ToRoom.PoolFor(lvl)},
		} {
			if strings.Join(pair[0], "|") != strings.Join(pair[1], "|") {
				t.Errorf("level %d %s: role-first %v, records %v", lvl, name, pair[0], pair[1])
			}
		}
		as, bs := a.Options[Weak].Separate, b.Options[Weak].Separate
		if strings.Join(as.ToDefenderRoom.PoolFor(lvl), "|") != strings.Join(bs.ToDefenderRoom.PoolFor(lvl), "|") {
			t.Errorf("level %d remote_observer differs", lvl)
		}
	}
}

func TestRecordLayoutRejectsBadRecords(t *testing.T) {
	cases := map[string]struct{ doc, want string }{
		"missing role": {`
optionid: test
options:
  weak:
    together:
      master:
      - {actor: 'A', actee: 'D'}
`, "option[`weak`]: together: master record 0: missing role \"observer\""},
		"empty line": {`
optionid: test
options:
  weak:
    together:
      beginner:
      - {actor: 'A', actee: '  ', observer: 'R'}
`, "option[`weak`]: together: beginner record 0: empty \"actee\" line"},
		"unknown key": {`
optionid: test
options:
  weak:
    together:
      beginner:
      - {actor: 'A', acter: 'D', actee: 'D', observer: 'R'}
`, "option[`weak`]: together: beginner record 0: unknown key \"acter\""},
		"separate needs remote_observer": {`
optionid: test
options:
  weak:
    separate:
      beginner:
      - {actor: 'A', actee: 'D', observer: 'R'}
`, "option[`weak`]: separate: beginner record 0: missing role \"remote_observer\""},
		"mixed layout": {`
optionid: test
options:
  weak:
    together:
      actor: {beginner: ['A']}
      beginner:
      - {actor: 'A', actee: 'D', observer: 'R'}
`, "option[`weak`]: together: mixes role keys and tier keys"},
		"unknown tier": {`
optionid: test
options:
  weak:
    together:
      grandmaster:
      - {actor: 'A', actee: 'D', observer: 'R'}
`, "option[`weak`]: together: unknown key \"grandmaster\""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeGroup(t, c.doc)
			if err == nil {
				t.Fatalf("decoded without error, want %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not contain %q", err.Error(), c.want)
			}
		})
	}
}
```

  Run: `go test ./internal/items/ -run 'TestRecordLayout' -v`. Expected: FAIL (records decode to empty pools; no errors raised).

- [ ] **Step 2: Implement.** Create `internal/items/attack_records.go`:

```go
package items

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// The record layout (#370, M6 rows 26-27): each split holds tier -> list of
// records, and each record carries every role's line for ONE blow. The old
// role-first layout paired separate per-role lists only by position, so a
// reorder in one list silently narrated different blows to different
// audiences. Records make that unrepresentable: moving a record moves all of
// its roles. The decoder fills the same per-role SkillTieredMessages the rest
// of the package has always used, so PoolFor and Render are unchanged.

var (
	tierNames     = []string{"beginner", "expert", "master"}
	togetherRoles = []string{"actor", "actee", "observer"}
	separateRoles = []string{"actor", "actee", "observer", "remote_observer"}
)

func isOneOf(s string, set []string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// decodeTieredRecords reads one split in the record layout and returns its
// pools keyed by role. roleFirst is true when the node is instead the legacy
// role -> tier -> list layout, which the caller decodes itself.
func decodeTieredRecords(unmarshal func(interface{}) error, roles []string) (pools map[string]SkillTieredMessages, roleFirst bool, err error) {
	var probe map[string]interface{}
	if err := unmarshal(&probe); err != nil {
		return nil, false, err
	}
	sawRole, sawTier := false, false
	for k := range probe {
		switch {
		case isOneOf(k, separateRoles):
			sawRole = true
		case isOneOf(k, tierNames):
			sawTier = true
		default:
			return nil, false, fmt.Errorf("unknown key %q", k)
		}
	}
	if sawRole && sawTier {
		return nil, false, fmt.Errorf("mixes role keys and tier keys")
	}
	if sawRole {
		return nil, true, nil
	}

	var tiers map[string][]map[string]string
	if err := unmarshal(&tiers); err != nil {
		return nil, false, err
	}
	pools = make(map[string]SkillTieredMessages, len(roles))
	for _, tier := range tierNames {
		for i, rec := range tiers[tier] {
			for k := range rec {
				if !isOneOf(k, roles) {
					return nil, false, fmt.Errorf("%s record %d: unknown key %q", tier, i, k)
				}
			}
			for _, role := range roles {
				line, ok := rec[role]
				if !ok {
					return nil, false, fmt.Errorf("%s record %d: missing role %q", tier, i, role)
				}
				if strings.TrimSpace(line) == "" {
					return nil, false, fmt.Errorf("%s record %d: empty %q line", tier, i, role)
				}
				p := pools[role]
				switch tier {
				case "beginner":
					p.Beginner = append(p.Beginner, ItemMessage(line))
				case "expert":
					p.Expert = append(p.Expert, ItemMessage(line))
				case "master":
					p.Master = append(p.Master, ItemMessage(line))
				}
				pools[role] = p
			}
		}
	}
	return pools, false, nil
}

// UnmarshalYAML accepts the record layout and, until the last batch is
// converted, the legacy role-first layout.
func (m *TogetherMessages) UnmarshalYAML(unmarshal func(interface{}) error) error {
	pools, roleFirst, err := decodeTieredRecords(unmarshal, togetherRoles)
	if err != nil {
		return fmt.Errorf("together: %w", err)
	}
	if roleFirst {
		type plain TogetherMessages
		return unmarshal((*plain)(m))
	}
	m.ToAttacker, m.ToDefender, m.ToRoom = pools["actor"], pools["actee"], pools["observer"]
	return nil
}

// UnmarshalYAML accepts the record layout and, until the last batch is
// converted, the legacy role-first layout.
func (m *SeparateMessages) UnmarshalYAML(unmarshal func(interface{}) error) error {
	pools, roleFirst, err := decodeTieredRecords(unmarshal, separateRoles)
	if err != nil {
		return fmt.Errorf("separate: %w", err)
	}
	if roleFirst {
		type plain SeparateMessages
		return unmarshal((*plain)(m))
	}
	m.ToAttacker, m.ToDefender = pools["actor"], pools["actee"]
	m.ToAttackerRoom, m.ToDefenderRoom = pools["observer"], pools["remote_observer"]
	return nil
}

// UnmarshalYAML decodes each intensity on its own so a record error names the
// intensity it came from; the file loader adds the path.
func (a *AttackTypes) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw map[Intensity]interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	out := make(AttackTypes, len(raw))
	for intensity, node := range raw {
		b, err := yaml.Marshal(node)
		if err != nil {
			return fmt.Errorf("option[`%s`]: %w", intensity, err)
		}
		var opts AttackOptions
		if err := yaml.Unmarshal(b, &opts); err != nil {
			return fmt.Errorf("option[`%s`]: %w", intensity, err)
		}
		out[intensity] = opts
	}
	*a = out
	return nil
}
```

  Note: a `together` or `separate` key that is absent or null leaves the struct zero, as before (the decoder is not called for an absent key; a null node probes to an empty map and yields empty pools).

- [ ] **Step 3: Run.** `go test ./internal/items/ -run 'TestRecordLayout' -v`: PASS. Then `go test ./internal/items/ ./internal/narration/ ./internal/combat/ .`: PASS, and the combat-messages golden is unchanged (no file is converted yet, so every pool must decode exactly as before). If the golden test fails, the legacy path is broken: fix it, never `-update`.
- [ ] **Step 4: Commit** `internal/items/attack_records.go`, `internal/items/attack_records_test.go`: `feat(items): combat messages accept one record per blow (#370)`.

### Task 2: Banner-word guard (TDD)

**Files:** Create `combat_banner_agreement_guard_test.go` (repo root, `package main`).

- [ ] **Step 1: Write the guard and its unit tests** in one file. The guard runs over `bannerGuardFiles`, which each batch extends; the final batch replaces it with every file.

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"gopkg.in/yaml.v2"
)

// Rows 26-27 of the M6 ledger (#370): the record layout stops a line pairing
// with the wrong blow by position, but a record can still be written with
// mismatched roles. The bold banner word (SHRED, FLAWLESS BITE) is the one
// part of a strong hit that every role repeats, so it must agree across the
// roles of a slot.

// bannerGuardFiles are the converted combat-message files. Each batch adds its
// files; the last batch replaces this list with every file in the store.
var bannerGuardFiles = []string{"bite", "claws", "gore", "maul"}

var bannerTag = regexp.MustCompile(`<ansi fg="[a-z]+-bold">([^<]+)</ansi>`)

func bannerWords(line string) []string {
	var out []string
	for _, m := range bannerTag.FindAllStringSubmatch(line, -1) {
		out = append(out, strings.Fields(strings.ToUpper(m[1]))...)
	}
	return out
}

// wordsAgree: the shorter word (three letters or more) is a prefix of the
// longer, or both share their first five letters. SHRED/SHREDS and
// EVISCERATE/EVISCERATION agree; CRUSHES/DEVASTATED do not.
func wordsAgree(x, y string) bool {
	s, l := x, y
	if len(s) > len(l) {
		s, l = l, s
	}
	if len(s) >= 3 && strings.HasPrefix(l, s) {
		return true
	}
	return len(x) >= 5 && len(y) >= 5 && x[:5] == y[:5]
}

func bannersAgree(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !wordsAgree(a[i], b[i]) {
			return false
		}
	}
	return true
}

// slotFailures compares every role of each slot against the actor's line.
func slotFailures(where string, roles map[string][]string) []string {
	var out []string
	actor := roles["actor"]
	for i := range actor {
		want := bannerWords(actor[i])
		for role, lines := range roles {
			if role == "actor" || i >= len(lines) {
				continue
			}
			got := bannerWords(lines[i])
			if (len(want) > 0 || len(got) > 0) && !bannersAgree(want, got) {
				out = append(out, fmt.Sprintf("%s[%d] %s %v against actor %v", where, i, role, got, want))
			}
		}
	}
	return out
}

func bannerFailures(name string, g *items.WeaponAttackMessageGroup) []string {
	var out []string
	tiers := []struct {
		name string
		get  func(items.SkillTieredMessages) items.MessageOptions
	}{
		{"beginner", func(s items.SkillTieredMessages) items.MessageOptions { return s.Beginner }},
		{"expert", func(s items.SkillTieredMessages) items.MessageOptions { return s.Expert }},
		{"master", func(s items.SkillTieredMessages) items.MessageOptions { return s.Master }},
	}
	strs := func(m items.MessageOptions) []string {
		r := make([]string, len(m))
		for i, x := range m {
			r[i] = string(x)
		}
		return r
	}
	for intensity, opts := range g.Options {
		for _, tier := range tiers {
			out = append(out, slotFailures(fmt.Sprintf("%s %s together %s", name, intensity, tier.name), map[string][]string{
				"actor":    strs(tier.get(opts.Together.ToAttacker)),
				"actee":    strs(tier.get(opts.Together.ToDefender)),
				"observer": strs(tier.get(opts.Together.ToRoom)),
			})...)
			out = append(out, slotFailures(fmt.Sprintf("%s %s separate %s", name, intensity, tier.name), map[string][]string{
				"actor":           strs(tier.get(opts.Separate.ToAttacker)),
				"actee":           strs(tier.get(opts.Separate.ToDefender)),
				"observer":        strs(tier.get(opts.Separate.ToAttackerRoom)),
				"remote_observer": strs(tier.get(opts.Separate.ToDefenderRoom)),
			})...)
		}
	}
	return out
}

func TestCombatBannerWordsAgreeAcrossRoles(t *testing.T) {
	for _, name := range bannerGuardFiles {
		path := filepath.Join("_datafiles", "world", "dogmud", "combat-messages", name+".yaml")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var g items.WeaponAttackMessageGroup
		if err := yaml.Unmarshal(raw, &g); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, f := range bannerFailures(name, &g) {
			t.Errorf("banner mismatch: %s", f)
		}
	}
}

func TestBannerWordsAgreeRule(t *testing.T) {
	agree := [][2]string{{"SHRED", "SHREDS"}, {"MAUL", "MAULED"}, {"EVISCERATE", "EVISCERATION"}, {"TEAR", "TEARS"}, {"HEW", "HEWS"}}
	differ := [][2]string{{"CRUSHES", "DEVASTATED"}, {"CHOP", "CLEAVE"}, {"DEVASTATING", "FLAWLESS"}}
	for _, p := range agree {
		if !wordsAgree(p[0], p[1]) {
			t.Errorf("%s and %s should agree", p[0], p[1])
		}
	}
	for _, p := range differ {
		if wordsAgree(p[0], p[1]) {
			t.Errorf("%s and %s should differ", p[0], p[1])
		}
	}
}

func TestBannerGuardCatchesAMismatchedSlot(t *testing.T) {
	b := func(w string) string { return `x <ansi fg="cyan-bold">` + w + `</ansi>!` }
	roles := map[string][]string{
		"actor":    {b("DEVASTATING CRITICAL"), b("SHRED")},
		"actee":    {b("FLAWLESS BITE"), b("SHREDS")},
		"observer": {b("FLAWLESS BITE"), "no banner here"},
	}
	got := slotFailures("probe", roles)
	if len(got) != 3 {
		t.Fatalf("want 3 failures (two roles at [0], the missing banner at [1]), got %d: %v", len(got), got)
	}
}
```

- [ ] **Step 2: Run.** `go test . -run 'Banner' -v`. Expected: the two unit tests PASS; `TestCombatBannerWordsAgreeAcrossRoles` FAILS on master content (bite, claws, gore and maul each carry failures, for example bite critical together master [1]). That red is the batch's target and turns green in Task 8. Record the failure count in the task report.
- [ ] **Step 3: Commit** with the guard's file list temporarily EMPTY (`var bannerGuardFiles = []string{}`) so the branch stays green; the batch's fix task fills it. Message: `test: banner words must agree across a combat slot's roles (#370)`.

### Task 3: Checker and pool audit learn records

**Files:** Modify `tools/combat_message_pad_check.py`, `tools/combat_message_pool_audit.py`.

- [ ] **Step 1: `pools()` reads both layouts.** In `tools/combat_message_pad_check.py`, replace the `pools` function with:

```python
ROLES = ["actor", "actee", "observer", "remote_observer"]


def pools(doc):
    """Return {(verb, split, role, tier): [lines]} for either layout.

    Role-first: split -> role -> tier -> [lines].
    Records:    split -> tier -> [{role: line}], one record per blow.
    """
    out = {}
    for verb, verb_body in ((doc or {}).get("options") or {}).items():
        for split, split_body in (verb_body or {}).items():
            if not split_body:
                continue
            if any(k in TIERS for k in split_body):
                for tier in TIERS:
                    for rec in split_body.get(tier) or []:
                        for role, line in (rec or {}).items():
                            out.setdefault((verb, split, role, tier), []).append(line)
                continue
            for role, role_body in split_body.items():
                if not isinstance(role_body, dict):
                    continue
                for tier in TIERS:
                    lines = role_body.get(tier) or []
                    if lines:
                        out[(verb, split, role, tier)] = list(lines)
    return out
```

- [ ] **Step 2: `--ignore-tier` and `--manifest`.** Change usage to `python tools/combat_message_pad_check.py <baseline-ref> [subtype] [--ignore-tier] [--manifest <file.json>]`. Parse flags with `argparse` (positional `ref`, optional positional `subtype`). With `--ignore-tier`, collapse keys to `(verb, split, role)` by concatenating tiers in TIERS order before comparing. With `--manifest`, load the JSON (`{"proposals": [...]}`); for each proposal with `"kind": "rewrite"`, subtract one `old` from the baseline multiset and one `new` from the after multiset of `(verb=intensity, split, role)` in file `file`, before the LOST check; a manifest entry whose `old` is not in the baseline, or whose `new` is not in the after file, is itself a violation (print `MANIFEST <file> <key>: ...`). Update the module docstring to describe both flags and that a swap or a tier move keeps every multiset intact. `moved` counting stays as is.

- [ ] **Step 3: Probe both directions** from Bash, standalone commands, in a scratch copy of the store, NOT the worktree: copy `_datafiles/world/dogmud/combat-messages/bite.yaml` to `$TMP/probe/`, then (a) hand-edit one character of a line with the Edit tool in the scratch copy and point the tool at it by temporarily setting `STORE` through a `--store` flag you add (default unchanged); expect exit 1 naming the line; (b) add a manifest entry for exactly that edit; expect exit 0; (c) swap two lines of one role between tiers; expect exit 1 without `--ignore-tier` and exit 0 with it. Report all four results.

- [ ] **Step 4: Pool audit.** In `tools/combat_message_pool_audit.py`'s `groups()`, when a split body's keys are tier names, yield `(verb, split, tier, {role: count})` counting each role across that tier's records (a record missing a role then shows as a short pool, which the Go decoder would also reject). Update its docstring: records keep pools equal by construction; the audit still reports any short pool.

- [ ] **Step 5: Run** `python tools/combat_message_pool_audit.py` (exit 0 on master) and `python tools/combat_message_pad_check.py origin/master` (exit 0, nothing changed). Commit both tools: `tools: combat-message checkers read the record layout (#370)`.

### Task 4: `coup_de_grace_test.go` decodes through the items type

**Files:** Modify `internal/combat/coup_de_grace_test.go` (`TestGenericYamlDefinesCoupDeGrace`, lines 73-120).

- [ ] **Step 1:** Replace the local role-first struct decode with `var g items.WeaponAttackMessageGroup` and `yaml.Unmarshal(raw, &g)`, then `cdg, ok := g.Options[items.CoupDeGrace]`. Replace the two key loops with checks that `len(cdg.Together.ToAttacker.Beginner)`, `len(cdg.Together.ToDefender.Beginner)`, `len(cdg.Together.ToRoom.Beginner)`, `len(cdg.Separate.ToAttacker.Beginner)` and `len(cdg.Separate.ToDefender.Beginner)` are each above zero, with the existing error wording (`coupdegrace.together.actor has no beginner messages`, and so on). Keep the "defines no coupdegrace block" fatal.
- [ ] **Step 2:** `go test ./internal/combat/ -run TestGenericYamlDefinesCoupDeGrace -v`: PASS. Prove it can fail: temporarily change the looked-up key to `items.Intensity("coupdegracex")`, run (expect the "defines no coupdegrace block" fatal), revert. Commit: `test(combat): coup de grace test reads either combat-message layout (#370)`.

---

## Part B: the batch procedure (every batch)

Batch file lists:

| Batch | Files | Branch |
|---|---|---|
| 1 | bite, claws, gore, maul | `feature/m6-row26-tier-bleed` (with Part A) |
| 2 | slashing, bludgeoning, stabbing, slam | `feature/m6-r26-batch2` |
| 3 | throttle, cleaving, whipping, pounce | `feature/m6-r26-batch3` |
| 4 | sting, siphon (was drain), staff, sceptre, wand | `feature/m6-r26-batch4` |
| 5 | generic, shooting, unarmed | `feature/m6-r26-batch5` (plus Part C) |

### Task 5: Convert the batch's files losslessly

One subagent per file, sequentially, because every file's proof runs the same golden test.

- [ ] **Step 1: Write the file in record form** with the Write tool. Keep the token comment header and `optionid:` verbatim. For every intensity and split, emit `<tier>:` then one record per index `i`, in index order, holding role lines at index `i` of that tier. Roles in this order: `actor`, `actee`, `observer`, then `remote_observer` for `separate`. Each role on its own line, single-quoted exactly as the source quoted it (copy the quoted scalar byte for byte, including doubled `''`):

```yaml
    together:
      beginner:
      - actor: 'You prepare to enter into mortal combat with <ansi fg="{acteetype}">{actee}</ansi>.'
        actee: '<ansi fg="{actortype}">{actor}</ansi> prepares to fight you with their <ansi fg="item">{itemname}</ansi>.'
        observer: '<ansi fg="{actortype}">{actor}</ansi> prepares to attack <ansi fg="{acteetype}">{actee}</ansi> with their <ansi fg="item">{itemname}</ansi>.'
```

  Keep in-file `#` comments that sit above an intensity or split, at the same place. Drop comments that sat inside a single role list only if they describe that list's position (report each dropped comment verbatim).
- [ ] **Step 2: Prove lossless.** `go test ./internal/narration/ -run TestSnapshotStores` must PASS with NO `-update`: pools decode identically, so the golden is byte-identical. Also `python tools/combat_message_pad_check.py HEAD <file>`: exit 0 with 0 lines added. Then `go test ./internal/items/ ./internal/combat/ .`.
- [ ] **Step 3: Commit** the one file: `content(combat): <file> as one record per blow, no text change (#370)`.

### Task 6: Audit (read only, one Opus subagent per file, may run in parallel)

- [ ] **Step 1:** Brief each auditor with: the converted file path; the scan hints for that file (the banner guard's failures from `go test . -run TestCombatBannerWordsAgreeAcrossRoles` after adding the file to `bannerGuardFiles` locally, without committing, and the word-overlap scan rows); the spec's three mismatch shapes; and these fix kinds, all within one intensity and split:
  - `swap`: exchange one role's line between two records (any tiers). Text unchanged.
  - `move`: move a whole record to another tier. Text unchanged. Changes which skill level sees it.
  - `rewrite`: replace one role's line in one record. Only for a line with no partner anywhere in that intensity and split, or a banner that needs aligning. Copy rules: no em or en dashes, at most 80 visible characters once tags are stripped, only tokens already used in the file, an actor's weapon is "their", never "its", no new idioms, same voice and tense as the role's other lines (actor second person, actee "you" as target, observer third person).
  - No deletions.
  - **Token ownership** (batch 1 playtest): in attack lines `{position}`, `{stance}` and `{momentum}` are the ATTACKER's. A defender or room line that gives them to the defender ("while you remain {position}") is a mismatch: rewrite it.
  - **Room lines name the target** (owner 2026-10-13, from batch 2): an `observer` (and `remote_observer`) line that never names `{actee}` leaves onlookers unable to tell who was hit; rewrite it to name `{actee}` where it fits in 80 visible characters, and list any that cannot fit as copy notes. Batch 5 also sweeps batch 1's four files for this.
- [ ] **Step 2:** Each auditor reads EVERY record (not only hinted ones) and asks: do all roles describe the same blow, with the same action, result and banner? It returns JSON only:

```json
{"file": "bite", "proposals": [
  {"intensity": "critical", "split": "together", "tier": "master", "index": 1,
   "kind": "rewrite", "role": "actee",
   "old": "<exact current line>", "new": "<exact new line>",
   "why": "actor is DEVASTATING CRITICAL; the FLAWLESS BITE triad already exists in expert record 4"},
  {"intensity": "weak", "split": "together", "tier": "expert", "index": 2,
   "kind": "swap", "role": "observer", "with_tier": "expert", "with_index": 3,
   "why": "observer lines at 2 and 3 are each other's partners"},
  {"intensity": "heavy", "split": "together", "tier": "expert", "index": 0,
   "kind": "move", "to_tier": "master", "why": "..."}
]}
```

  Plus, separately, a short list of records it judged fine but awkward (copy notes for the owner, not proposals).
- [ ] **Step 3:** The controller merges the files' outputs into `docs/superpowers/audits/m6-rows26-27-batch-<N>.json` (`{"batch": N, "proposals": [...]}`, each proposal carrying its `file`) and sanity-checks it: every `old` exists verbatim in the converted file at the named slot; every `rewrite` passes the copy rules (dash grep, visible length).

### Task 7: Review page (row 27)

- [ ] **Step 1:** The controller generates one HTML page in the session scratchpad (a page generator script there may write only that scratch HTML file): per file, per intensity, split and tier, a table with one row per record and one column per role. Records touched by a proposal get a flag and show before and after; rewrites highlight the new text; banner words are bolded. A summary at the top counts proposals by kind per file and lists the auditors' copy notes. Follow the artifact-design rules (tokens, both themes, phone width, tables in their own scroll containers).
- [ ] **Step 2:** Publish it with the Artifact tool (first batch: new page; later batches: new page per batch) and give the owner the link. Wait for approval or comments per file. Apply the owner's changes to the proposals JSON (drop, amend or add proposals). Do not proceed on a file without approval.

### Task 8: Apply the approved proposals

One subagent per file, sequentially.

- [ ] **Step 1:** Apply the file's approved proposals with the Edit tool: a `swap` exchanges the two role lines; a `move` cuts the record and pastes it at the end of the target tier's list (create the tier key if absent, in tier order); a `rewrite` replaces the line. Text of swapped and moved lines stays byte-identical.
- [ ] **Step 1b:** `python tools/combat_message_proposals_check.py <converted-commit> <file> docs/superpowers/audits/m6-rows26-27-batch-<N>.json`: exit 0 (the working tree file equals the converted file with the approved proposals applied, slot for slot).
- [ ] **Step 2:** Add the file to `bannerGuardFiles` in `combat_banner_agreement_guard_test.go`. Run `go test . -run 'Banner' -v`: PASS. A remaining failure means a proposal is missing: report it to the controller, who takes it back to the owner (no allowlist).
- [ ] **Step 3:** `python tools/combat_message_pad_check.py <converted-commit> <file> --ignore-tier --manifest docs/superpowers/audits/m6-rows26-27-batch-<N>.json`: exit 0.
- [ ] **Step 4:** `go test ./internal/narration/ -run TestSnapshotStores -update`, then read the golden diff: every changed row must trace to a proposal (a swap or move changes rows at both ends; a rewrite changes its own row). Then `go test ./internal/items/ ./internal/combat/ ./internal/narration/ .`.
- [ ] **Step 5: Commit** the YAML file, the guard file, the batch JSON (first file of the batch adds it) and the golden: `content(combat): <file> pairs every role with its blow (#370)`.

### Task 9: Docs, gates, review, PR, playtest, merge

- [ ] **Step 1: Docs.** `internal/items/context.md`: the record layout, `attack_records.go`, the transition rule (batch 1; batch 5 removes the legacy note). `docs/PATCH_NOTES.md` (batch 1 and batch 5 only): "Combat lines now always show the attacker, the defender and onlookers the same blow; a few strong hits used to describe a different strike to each." The ledger rows 26 and 27 status cell: progress per batch (`batch N of 5 merged: <files>`). Batch 1 adds this plan, the spec and the batch JSON to `docs/README.md`; later batches add their JSON. Update `tools` rows in `docs/README.md` for the new flags (batch 1).
- [ ] **Step 2: Gates**, each standalone: `gofmt -l internal/ modules/` and `gofmt -l *.go`; `go build ./...`; `go vet ./...`; `go test ./... > $TMP/gotest.log 2>&1` then `grep -- "--- FAIL" $TMP/gotest.log` (report names verbatim; rerun a failing package once); `golangci-lint run --new-from-merge-base=origin/master ./...`; `python tools/combat_message_pool_audit.py`.
- [ ] **Step 3:** Whole-branch review against the spec (Opus); fix findings. Isolated boot per `dogmud-shipping` (fixed-path `boot-check.exe`, ports off the owner's, stop by PID).
- [ ] **Step 4:** Push; `gh pr create --repo pruuk/DOGMud --base master`, body: "Part of #370 (rows 26 and 27), batch N of 5", the files, the proposal counts by kind, the review page link, and the follow-up issues #519 to #522. Do not write a closing keyword for #370.
- [ ] **Step 5: Playtest (batch 1 and batch 5).** Per `dogmud-playtesting`: a mob whose natural weapon is in the batch (batch 1: bite, claws, gore or maul) fights a player while a second player watches in the same room. The attack tier comes from the attacker's `GetCombatSkillLevel()` (`internal/combat/combat_helpers.go:1766`), so pick or set a mob at 67 or more for master lines. Over several rounds the defender seat and the room seat must read the same blow each round, banners included. Batch 5 adds a player with a generic weapon at master skill and a `shooting` round if the #497 fixture exists.
- [ ] **Step 6:** Merge on green with the owner's go-ahead: `gh pr merge <n> --repo pruuk/DOGMud --merge --delete-branch`.

---

## Part C: final batch extras (batch 5 only)

### Task 10: Remove the legacy layout and widen the guard

**Files:** `internal/items/attack_records.go`, `internal/items/attack_records_test.go`, `combat_banner_agreement_guard_test.go`, `internal/items/context.md`, ledger.

- [ ] **Step 1: Failing test.** In `attack_records_test.go`, replace `TestRecordLayoutDecodesToTheSamePoolsAsRoleFirst` with `TestRoleFirstLayoutIsRejected`: decoding `roleFirstDoc` must fail with an error containing `together: role-first layout is retired; write one record per blow`. Run: FAIL.
- [ ] **Step 2: Implement.** In `decodeTieredRecords`, return `nil, false, fmt.Errorf("role-first layout is retired; write one record per blow")` when `sawRole`; drop the `roleFirst` return value and the `type plain` fallbacks in both `UnmarshalYAML` methods. Run: PASS.
- [ ] **Step 3: Guard over every file.** Replace `bannerGuardFiles` with a glob of `_datafiles/world/dogmud/combat-messages/*.yaml` (names without extension), keeping a `len(files) == 20` fatal so an empty glob cannot pass. Run `go test . -run Banner`: PASS.
- [ ] **Step 4: Validate.** `Validate`'s per-tier equality check stays as it is (records make it always pass; it still guards a hand-built group in tests). Update its comment to say so.
- [ ] **Step 5: Docs.** `internal/items/context.md`: legacy layout gone. Ledger rows 26 and 27: `done: record layout plus banner guard, five batches (PRs <list>)`. Note in the PR that the default world's combat files (old `toattacker` keys, world does not boot) were left untouched and fail to decode if that world is ever loaded (true from batch 1, not only after batch 5: the decoder rejects the old keys as unknown, so `items.LoadDataFilesE` fails at the combat-messages step; it used to load empty pools silently). Commit.

---

## Self-review notes

- Spec coverage: decoder and its errors (Task 1), transition and removal (Tasks 1, 10), banner guard (Tasks 2, 8, 10), audit, page, apply, checker (Tasks 3, 5 to 8), batches (Part B table), coup de grace test (Task 4), golden handling (Tasks 5, 8), playtest (Task 9), default world note (Task 10), follow-ups #519 to #522 (Task 9 PR body).
- Conversion before audit is a refinement of the spec's order: it lets the golden prove the conversion lossless and lets auditors read records instead of parallel lists.
