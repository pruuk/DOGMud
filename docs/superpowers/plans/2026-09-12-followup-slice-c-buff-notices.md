# Follow-up slice C: every buff tells its holder when it starts and ends, implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** No buff applies or expires in silence for its holder: authored flavour on every buff, a generic line underneath, and a guard that fails the build on a silent buff.

**Architecture:** Two resolver methods on `buffs.BuffSpec` become the one door for the player-side start and end line; the apply hook and the player prune pass call them instead of reading raw fields. A root test over the world YAML enforces authored text on every non-secret buff and no text on a secret one. A boot warning lists buffs relying on the fallback. Forty-two buffs get authored lines; four system buffs become secret.

**Tech Stack:** Go, `internal/buffs`, `internal/hooks`, root-package guard tests, YAML world data, the playtest harness.

Spec: `docs/superpowers/specs/2026-09-12-followup-slice-c-buff-notices-design.md`

---

## Read this first

- **Test binaries never load world YAML.** `buffs.GetBuffSpec` returns nil for everything unless a test seeds specs with `buffs.SeedBuffsForTest`. The hooks fixture (`seedAllRegistries()` in `internal/hooks/hooks_test.go:37`) seeds users 1 "Aliceia" and 2 "Bobrick" in room 1 and mob 100; `seedNarrationBuffs()` in `internal/hooks/narration_testhelpers_test.go:51` seeds buffs 7001-7007 and returns a restore func (call it AFTER `defer cleanup()` and `defer` its result). `drainPlain(userId)` returns a user's queued lines tag-stripped; `countContaining(lines, needle)` counts matches.
- **`ApplyBuffs(events.Buff{UserId: 1, BuffId: id})` and `PruneBuffs(events.NewTurn{TurnNumber: 1})`** are how `internal/hooks/buff_room_text_test.go` drives the two hooks; `expire(t, holder.Character.Buffs.List, id)` there forces a buff to be prunable. Copy those patterns.
- **A test must be seen failing before the fix.** If a test passes before the change, the test is wrong.
- **YAML gotchas** (`dogmud-authoring-content`): a value containing a colon or an apostrophe must be quoted; use double quotes and escape nothing else. Keep every authored line under 80 columns including the key. Player copy: second person, no numbers, no em dashes, ESL-clear.
- Commit messages end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>` unless a session reminder names a different line. Never `git add -A`.

## File structure

| File | Change |
|---|---|
| `internal/buffs/notice.go` | Create: `StartUserNotice`, `EndUserNotice`, `SilentNoticeBuffs`, `WarnSilentNotices` |
| `internal/buffs/notice_test.go` | Create: resolver and warning tests |
| `internal/hooks/Buff_ApplyBuffs.go` | Apply path reads `StartUserNotice()` |
| `internal/hooks/NewTurn_PruneBuffs.go` | Player prune path reads `EndUserNotice()` |
| `internal/hooks/buff_notice_test.go` | Create: generic line reaches the holder; secret buff is silent |
| `main.go` | Wire `buffs.WarnSilentNotices()` after `species.ValidateSpeciesBuffIds` |
| `_datafiles/world/dogmud/buffs/*.yaml` | 42 buffs gain `start_user_text` and `end_user_text`; 4 gain `secret: true` |
| `buff_notice_guard_test.go` (repo root) | Create: the root guard |
| `internal/buffs/context.md`, `internal/hooks/context.md`, `docs/PATCH_NOTES.md` | Docs |
| `tools/playtest/goals/2026-09-12-slice-c-buff-notices.yaml` | Create: the playtest lane |

---

### Task 1: The resolver

**Files:** create `internal/buffs/notice.go`, `internal/buffs/notice_test.go`.

- [ ] **Step 1: Failing tests.** Create `internal/buffs/notice_test.go`:

```go
package buffs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// One door for the player-side buff line: authored text first, a generic line
// underneath, silence for a secret buff. Forty-six dogmud buffs had neither a
// start nor an end line before slice C.
func TestBuffNotices(t *testing.T) {
	authored := &BuffSpec{BuffId: 1, Name: "Venom", StartUserText: "Venom burns.", EndUserText: "The venom subsides."}
	assert.Equal(t, "Venom burns.", authored.StartUserNotice())
	assert.Equal(t, "The venom subsides.", authored.EndUserNotice())

	silent := &BuffSpec{BuffId: 2, Name: "Warrior's Brew"}
	assert.Equal(t, "Warrior's Brew takes effect.", silent.StartUserNotice())
	assert.Equal(t, "Warrior's Brew has expired.", silent.EndUserNotice())

	secret := &BuffSpec{BuffId: 3, Name: "Respawn Grace", Secret: true, StartUserText: "never shown"}
	assert.Equal(t, "", secret.StartUserNotice(), "a secret buff says nothing even with authored text")
	assert.Equal(t, "", secret.EndUserNotice())

	nameless := &BuffSpec{BuffId: 4}
	assert.Equal(t, "", nameless.StartUserNotice(), "no name, no generic line; the guard catches this")
	assert.Equal(t, "", nameless.EndUserNotice())
}

func TestSilentNoticeBuffsListsOnlyNonSecretBuffsRelyingOnTheFallback(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		10: {BuffId: 10, Name: "Authored", StartUserText: "a", EndUserText: "b"},
		11: {BuffId: 11, Name: "Half", StartUserText: "a"},
		12: {BuffId: 12, Name: "Bare"},
		13: {BuffId: 13, Name: "Hidden", Secret: true},
	})
	defer restore()
	assert.ElementsMatch(t, []string{"11 Half (end)", "12 Bare (start, end)"}, SilentNoticeBuffs())
}
```

- [ ] **Step 2: Run red.** `go test ./internal/buffs/ -run "TestBuffNotices|TestSilentNoticeBuffs" -v`. Expected: compile FAIL, `undefined` methods.

- [ ] **Step 3: Implement.** Create `internal/buffs/notice.go`:

```go
package buffs

import (
	"fmt"
	"sort"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// StartUserNotice is the line the holder reads when this buff lands: the
// authored start_user_text, or "<Name> takes effect." when none is authored.
// A secret buff says nothing. A buff with no name says nothing either, rather
// than print " takes effect."; the root guard fails the build on that case.
//
// This is the one door for the player-side start line. Buff_ApplyBuffs reads
// it instead of StartUserText, so a buff added without text can no longer
// land in silence.
func (b *BuffSpec) StartUserNotice() string {
	if b.Secret || b.Name == "" {
		return ""
	}
	if b.StartUserText != "" {
		return b.StartUserText
	}
	return fmt.Sprintf("%s takes effect.", b.Name)
}

// EndUserNotice is the line the holder reads when this buff ends: the
// authored end_user_text, or "<Name> has expired." when none is authored.
// A secret or nameless buff says nothing. The player prune pass reads it
// instead of EndUserText.
func (b *BuffSpec) EndUserNotice() string {
	if b.Secret || b.Name == "" {
		return ""
	}
	if b.EndUserText != "" {
		return b.EndUserText
	}
	return fmt.Sprintf("%s has expired.", b.Name)
}

// SilentNoticeBuffs lists every loaded non-secret buff that relies on the
// generic line for its start or end notice, as "<id> <name> (start, end)".
// Sorted by id. The root guard keeps this empty for the shipped world; the
// boot warning reports it for any other world or a hot edit.
func SilentNoticeBuffs() []string {
	ids := GetAllBuffIds()
	sort.Ints(ids)
	out := []string{}
	for _, id := range ids {
		b := GetBuffSpec(id)
		if b == nil || b.Secret {
			continue
		}
		missing := ""
		if b.StartUserText == "" {
			missing = "start"
		}
		if b.EndUserText == "" {
			if missing != "" {
				missing += ", "
			}
			missing += "end"
		}
		if missing != "" {
			out = append(out, fmt.Sprintf("%d %s (%s)", id, b.Name, missing))
		}
	}
	return out
}

// WarnSilentNotices logs one warning per buff relying on the generic notice.
// Wired at boot after the buffs load. A warning, not a panic: the generic
// line exists so play continues; the root guard is what blocks a merge.
func WarnSilentNotices() {
	for _, entry := range SilentNoticeBuffs() {
		mudlog.Warn("buffs.WarnSilentNotices", "buff", entry, "notice", "relies on the generic takes effect / has expired line; author start_user_text and end_user_text")
	}
}
```

Check `GetAllBuffIds()` exists at `internal/buffs/buffspec.go:175` and returns ids of the loaded registry (it does; confirm it reads the same `buffs` map `SeedBuffsForTest` replaces).

- [ ] **Step 4: Green.** `go test ./internal/buffs/` ok; `gofmt -l internal/buffs` empty.

- [ ] **Step 5: Commit.**
```bash
git add internal/buffs/notice.go internal/buffs/notice_test.go
git commit -m "feat(buffs): one door for the player-side start and end notice" -m "StartUserNotice and EndUserNotice return the authored line or a generic takes effect / has expired line, and nothing for a secret buff. SilentNoticeBuffs lists what relies on the fallback." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 2: The apply and prune paths read the resolver

**Files:** modify `internal/hooks/Buff_ApplyBuffs.go` (lines 75-120), `internal/hooks/NewTurn_PruneBuffs.go` (lines 44-58); create `internal/hooks/buff_notice_test.go`.

- [ ] **Step 1: Failing tests.** Create `internal/hooks/buff_notice_test.go`:

```go
package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Buff ids clear of the fixture and of the narration buffs (7001-7007).
const (
	quietBuffId  = 7101 // no authored text at all
	hushedBuffId = 7102 // secret, with authored text that must never show
)

func seedNoticeBuffs() func() {
	return buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		quietBuffId:  {BuffId: quietBuffId, Name: "Test Quiet", RoundInterval: 5, TriggerCount: 3},
		hushedBuffId: {BuffId: hushedBuffId, Name: "Test Hushed", Secret: true, RoundInterval: 5, TriggerCount: 3, StartUserText: "You should never read this.", EndUserText: "Nor this."},
	})
}

func TestBuffNotice_HolderReadsTheGenericStartLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	drainPlain(1)
	drainPlain(2)

	assert.Equal(t, events.Continue, ApplyBuffs(events.Buff{UserId: 1, BuffId: quietBuffId}))
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Quiet takes effect."))
	assert.Equal(t, 0, countContaining(drainPlain(2), "Test Quiet"), "no room line was authored, so the room hears nothing")
}

func TestBuffNotice_HolderReadsTheGenericEndLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	holder := users.GetByUserId(1)
	require.True(t, holder.Character.Buffs.AddBuff(quietBuffId, false))
	expire(t, holder.Character.Buffs.List, quietBuffId)
	drainPlain(1)

	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Equal(t, 1, countContaining(drainPlain(1), "Test Quiet has expired."))
}

func TestBuffNotice_SecretBuffIsSilentAtBothEnds(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNoticeBuffs()
	defer restore()
	holder := users.GetByUserId(1)
	drainPlain(1)

	ApplyBuffs(events.Buff{UserId: 1, BuffId: hushedBuffId})
	assert.Empty(t, drainPlain(1), "a secret buff's authored start text must not be sent")

	expire(t, holder.Character.Buffs.List, hushedBuffId)
	PruneBuffs(events.NewTurn{TurnNumber: 1})
	assert.Empty(t, drainPlain(1), "a secret buff's authored end text must not be sent")
}
```

`expire` lives in `internal/hooks/buff_room_text_test.go`; if it is not package-visible under that name, read that file and use its helper.

- [ ] **Step 2: Run red.** `go test ./internal/hooks/ -run TestBuffNotice -v`. Expected: the two generic-line tests FAIL (nothing received); the secret test FAILS on the start half (authored text is sent today). Paste the lines.

- [ ] **Step 3: Apply path.** In `Buff_ApplyBuffs.go`, the gate at line 75 becomes

```go
	startUser := buffInfo.StartUserNotice()
	if !wasAlreadyActive && (startUser != "" || buffInfo.StartRoomText != "") {
```

and the send at line 120 becomes `textutil.SendPhaseText(startUser, buffInfo.StartRoomText, tCtx, "cyan", cfg)`. Confirm `buffInfo` there is a `*buffs.BuffSpec` (it is the spec looked up above; read lines 40-66). Update the comment above the gate: "Send the start notice (authored, or the generic line; a secret buff is silent) only on first application".

- [ ] **Step 4: Player prune path.** In `NewTurn_PruneBuffs.go`, lines 44 and 58 become

```go
						endUser := endBuffSpec.EndUserNotice()
						if endBuffSpec != nil && (endUser != "" || endBuffSpec.EndRoomText != "") {
```
(compute `endUser` only after the nil check: `endUser := ""; if endBuffSpec != nil { endUser = endBuffSpec.EndUserNotice() }`) and `textutil.SendPhaseText(endUser, endBuffSpec.EndRoomText, tCtx, "cyan", cfg)`. The MOB prune pass (lines 102-120) is untouched.

- [ ] **Step 5: Green and guards.** `go test ./internal/hooks/ -run "TestBuffNotice|TestBuff" -v`; `go test ./internal/hooks/`; the five root guards `go test . -run "TestNarrationSitesMatchViewpointAudit|TestM2RoutingIsFrozen|TestEveryTrioLiteralNamesAllThreeRoles|TestM2LiteralsAreFrozen|TestEveryTextSurfaceIsRegistered"` (add exactly what a guard names if one asks; never loosen); `gofmt -l internal/hooks` empty.

- [ ] **Step 6: Commit.**
```bash
git add internal/hooks/Buff_ApplyBuffs.go internal/hooks/NewTurn_PruneBuffs.go internal/hooks/buff_notice_test.go
git commit -m "fix(buffs): the holder always reads a start and an end line" -m "The apply hook and the player prune pass read StartUserNotice and EndUserNotice, so a buff with no authored text lands and ends with the generic line and a secret buff stays silent." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 3: Boot warning

**Files:** modify `main.go` (after line 1694).

- [ ] **Step 1:** After `species.ValidateSpeciesBuffIds(buffs.HasSpec)` add:
```go
	// Slice C: a non-secret buff without authored start/end text still speaks
	// (the generic notice), but say so at boot. The root guard blocks a merge.
	buffs.WarnSilentNotices()
```
- [ ] **Step 2:** `go build ./...` clean. Boot check comes in Task 7 (it will print 42 warnings until Task 4 lands, then zero).
- [ ] **Step 3: Commit.**
```bash
git add main.go
git commit -m "chore(boot): warn on buffs relying on the generic notice" -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 4: Content, 42 authored buffs and 4 secret ones

> **Correction after the content pass (2026-09-12):** the corpus had 43 fully silent buffs and 6 half-silent ones, not 46; 40 pairs were authored. Meditating is the quit narration and is NOT secret (it gets an end line); Hidden and Empathic Shroud keep no end line by design (the `hidden` flag opts out); Warcry, Rally and Bloom Detox are applied without the buff event, so a new `silent-start` flag opts them out of the start requirement. The resolver, the boot warning and the guard honour both flags. The table below is the plan as written.

**Files:** `_datafiles/world/dogmud/buffs/*.yaml` (46 files).

- [ ] **Step 1: Four secret buffs.** Add `secret: true` (top-level key, next to `name:`) to `0-meditating.yaml`, `81-respawn_grace.yaml`, `85-infraredvision.yaml`, `99-alt_character_mob.yaml`. Do not add text to them.

- [ ] **Step 2: Author 42 pairs.** For each file below add `start_user_text:` and `end_user_text:` (top-level keys, placed after `description:`). Rules: second person; the start line describes the effect landing, the end line the effect fading; name the buff where it reads naturally, never as a bare label; no numbers; no em dashes; under 80 columns including the key; double-quote any value containing an apostrophe or colon. Do NOT add room text except where marked (room) below, and then only `start_room_text`/`end_room_text` describing something a bystander could see, with `{source}` as the subject.

| File | Kind | Start line says | End line says |
|---|---|---|---|
| 102-disrupted | affliction | dissonance grinds, aim wanders | the dissonance fades, your aim steadies |
| 112-paralysed | affliction | silk and dissonance lock your limbs | the field releases you, limbs answer again |
| 116-terrified | affliction | primal dread rolls over you | the dread loosens its grip |
| 24-death_recovery | affliction | you are recovering from dying, weak | you have recovered from dying |
| 25-deaths_shadow | affliction | a dark shadow clings, dulling you | the shadow lifts |
| 29-night_vision | boon (room) | the dark thins to grey shapes | the dark closes in again |
| 42-hearty_meal | meal | warm meal, aches ease | the meal's comfort fades |
| 43-stamina_boost | meal | vigour returns | the vigour ebbs |
| 44-clear_mind | drink | thoughts clear, spirit steadies | the clarity fades |
| 45-well_fed | meal | full and hale | no longer full |
| 46-liquid_courage | drink | tongue loosens, nerve steadies | the courage wears off, nerves return |
| 52-chrysalis_shell | boon (room) | a chrysalis sheen settles over your skin | the sheen flakes away |
| 53-veil_sight | boon | your sight pierces the Veil | the Veil closes to your sight |
| 54-healing_salve | potion | warmth spreads, wounds knit | the salve's warmth fades |
| 55-stamina_tonic | potion | breath steadies, energy surges | the tonic wears off |
| 56-conviction_draught | potion | purpose sharpens your focus | the draught's certainty fades |
| 57-warriors_brew | potion | fire in your veins, muscles mend | the brew fades, limbs feel ordinary |
| 58-preachers_tincture | potion | calm certainty settles | the calm lifts |
| 59-windrunner_draught | potion | lungs open, you could run for hours | your breath is your own again |
| 60-elixir_of_renewal | potion | restored as if from perfect sleep | the renewal fades |
| 61-ironhide_brew | potion | skin toughens like bark, joints stiffen | skin softens, joints loosen |
| 62-mindshield_elixir | potion | a shell of clarity blunts magic, arms heavy | the shell dissolves, arms lighten |
| 63-veilguard_tonic | potion | a bitter film deflects words of power | the film dissolves |
| 64-stone_stomach | potion | nothing can turn your stomach, reflexes slow | your stomach is ordinary, reflexes return |
| 65-cats_eye_draught | potion (room) | the darkness recedes, eyes catch light | the dark returns, the glint fades |
| 66-swiftfoot_essence | potion | the world slows, reflexes sharpen | the world speeds up again |
| 67-berserker_elixir | potion | raw fury floods your muscles | the fury drains away, leaving you spent |
| 68-silver_tongue_oil | potion | words come easily | words come no easier than before |
| 69-battle_trance | potion | senses sharpen to a razor edge | the trance breaks |
| 70-purging_draught | potion | your body convulses, expelling everything | the convulsions stop |
| 71-essence_of_growth | potion | body hums with potential | the hum fades |
| 72-savants_infusion | potion | knowledge crystallises faster | the infusion wears off |
| 73-mutagen_brew | potion | something shifts deep inside | the shifting settles |
| 74-chrysalis_catalyst | potion | chrysalis energy rewrites something | the energy settles |
| 75-nausea | affliction | stomach heaves, vision swims | the nausea passes |
| 76-purging_weakness | affliction | everything aches after the purge | the aching fades |
| 77-flashbang_blindness | affliction | a flash sears your sight | your sight clears |
| 82-steady_hand | boon | hands perfectly still, motion exact | a faint tremor returns to your hands |
| 89-throttled | affliction | a grip crushes your throat | the grip releases, you gasp air |
| 95-hull_dampening | affliction | the buried place presses your gift down | the pressure lifts, your gift stirs |
| 12 others | none | (already authored, untouched) | |

Write each line as finished prose, for example:

```yaml
start_user_text: Fire spreads through your veins as the Warrior's Brew takes hold.
end_user_text: The Warrior's Brew fades, and your limbs feel ordinary again.
```

- [ ] **Step 3: Verify.** Run, standalone:
```bash
python - <<'PY'
import glob,yaml
bad=[]
for f in sorted(glob.glob('_datafiles/world/dogmud/buffs/*.yaml')):
    d=yaml.safe_load(open(f,encoding='utf-8'))
    if d.get('secret'):
        if any(d.get(k) for k in ('start_user_text','end_user_text','start_room_text','end_room_text','trigger_user_text','trigger_room_text')): bad.append((f,'secret with text'))
        continue
    for k in ('start_user_text','end_user_text'):
        if not d.get(k): bad.append((f,'missing '+k))
    for line in open(f,encoding='utf-8'):
        if len(line.rstrip('\n'))>80 and ('_text:' in line): bad.append((f,'over 80: '+line.strip()[:40]))
        if chr(0x2014) in line or chr(0x2013) in line: bad.append((f,'dash'))
print(bad or 'all 102 clean')
PY
```
Expected: `all 102 clean`. Then `go build ./...` and a boot smoke is not needed here (Task 7).

- [ ] **Step 4: Commit.**
```bash
git add _datafiles/world/dogmud/buffs/
git commit -m "content(buffs): every buff tells its holder when it starts and ends" -m "Forty-two buffs gain authored start and end lines in the player-copy style; the logout timer, respawn grace, the synthetic infrared buff and the alt-character gear lock become secret and leave the conditions list." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```
(`git add` of the directory is acceptable here: the diff is exactly the 46 intended files; confirm with `git status --short | wc -l` = 46 before committing.)

---

### Task 5: The root guard

**Files:** create `buff_notice_guard_test.go` at the repo root (package `main`).

- [ ] **Step 1: Write the guard.**

```go
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Slice C: no buff may apply or expire in silence for its holder. The engine
// falls back to a generic "takes effect" / "has expired" line, but that is a
// runtime net, never the shipped experience, so every non-secret buff in the
// dogmud world must carry authored start_user_text and end_user_text, and a
// secret buff must carry no player text at all. Forty-six buffs were silent
// on 2026-09-12; this keeps the count at zero.
func TestEveryDogmudBuffHasAuthoredNotices(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("_datafiles", "world", "dogmud", "buffs", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no buff files found: %v", err)
	}
	sort.Strings(files)
	var problems []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var b struct {
			Name            string `yaml:"name"`
			Secret          bool   `yaml:"secret"`
			StartUserText   string `yaml:"start_user_text"`
			StartRoomText   string `yaml:"start_room_text"`
			TriggerUserText string `yaml:"trigger_user_text"`
			TriggerRoomText string `yaml:"trigger_room_text"`
			EndUserText     string `yaml:"end_user_text"`
			EndRoomText     string `yaml:"end_room_text"`
		}
		if err := yaml.Unmarshal(raw, &b); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		base := filepath.Base(f)
		if b.Secret {
			if b.StartUserText+b.StartRoomText+b.TriggerUserText+b.TriggerRoomText+b.EndUserText+b.EndRoomText != "" {
				problems = append(problems, base+": secret buff carries player text")
			}
			continue
		}
		if strings.TrimSpace(b.Name) == "" {
			problems = append(problems, base+": non-secret buff has no name (the generic notice would be blank)")
		}
		if strings.TrimSpace(b.StartUserText) == "" {
			problems = append(problems, base+": missing start_user_text (holder would read the generic line)")
		}
		if strings.TrimSpace(b.EndUserText) == "" {
			problems = append(problems, base+": missing end_user_text (holder would read the generic line)")
		}
	}
	if len(problems) > 0 {
		t.Fatalf("%d buff notice problems:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}
```

- [ ] **Step 2: Prove it can fail.** With Task 4's content in place it passes at once, so sabotage: temporarily delete the `end_user_text` line from `_datafiles/world/dogmud/buffs/57-warriors_brew.yaml`, run `go test . -run TestEveryDogmudBuffHasAuthoredNotices`, confirm it FAILS naming `57-warriors_brew.yaml: missing end_user_text`, then restore the line (`git checkout -- _datafiles/world/dogmud/buffs/57-warriors_brew.yaml` and then `git status --short` must be clean for that path, since that command also stages). Paste the failing line. Then run green.

- [ ] **Step 3: Commit.**
```bash
git add buff_notice_guard_test.go
git commit -m "test: a silent buff fails the build" -m "Root guard over the dogmud buff files: every non-secret buff must carry authored start and end lines, a secret buff none. Proven red by removing one line." -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 6: Docs

**Files:** `internal/buffs/context.md`, `internal/hooks/context.md`, `docs/PATCH_NOTES.md`.

- [ ] **Step 1:** `internal/buffs/context.md`: in the file list add `notice.go`; in the API surface add `StartUserNotice`, `EndUserNotice`, `SilentNoticeBuffs`, `WarnSilentNotices` with one line each, and a paragraph: "Every non-secret buff must carry authored `start_user_text` and `end_user_text`; the root guard `buff_notice_guard_test.go` enforces it and the boot warning lists any that slip through. The resolver's generic line is the runtime net. `secret: true` silences a buff at both ends and hides it from `conditions`."
- [ ] **Step 2:** `internal/hooks/context.md`: where the buff apply and prune hooks are described (grep `Buff_ApplyBuffs` and `PruneBuffs`), state that the player line comes from `StartUserNotice` / `EndUserNotice`.
- [ ] **Step 3:** `docs/PATCH_NOTES.md`, a new entry above the newest one, 80 columns, no numbers, no dashes:

```markdown
## 2026-09-12: Every potion, meal and affliction now says hello and goodbye

Many effects used to arrive and leave without a word. You would drink a
potion, read that you drank it, and then hear nothing about what it did or
when it stopped. Meals, tonics, draughts and most afflictions were the same.
Every one of them now tells you when it takes hold and when it fades, in its
own words. A few housekeeping effects the game uses behind the scenes no
longer show up in your conditions list at all.
```
- [ ] **Step 4: Commit.**
```bash
git add internal/buffs/context.md internal/hooks/context.md docs/PATCH_NOTES.md
git commit -m "docs: buff notices, the guard and the patch note" -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 7: Gates and the playtest lane

**Files:** create `tools/playtest/goals/2026-09-12-slice-c-buff-notices.yaml`.

- [ ] **Step 1: Gates**, each standalone: `gofmt -l internal/ modules/ *.go` (empty); `go build ./...`; `go test ./...` (no FAIL lines); `golangci-lint run --new-from-rev=master ./...` (0 issues); the isolated boot check per `dogmud-shipping` (exit 124, 0 panics, 1 Server Ready) AND `grep -c "WarnSilentNotices" boot.log` must be 0.

- [ ] **Step 2: Lane.** Create the goals file:

```yaml
# Slice C: every buff tells its holder when it starts and ends.
#
# Two potions are granted: the Warrior's Brew (buff 57, a plain boon) and the
# Purging Draught (buff 70, which is followed by Purging Weakness, buff 76, an
# affliction). Each should print an authored line when it takes hold and
# another when it fades. The four secret buffs (Meditating, Respawn Grace,
# InfraredVision, Alt Character Mob) must never appear in `conditions`.
ephemeral:
  profile: veteran
  start_room: 5343
  overlays:
    grant_items:
      - 30039 # Warrior's Brew
      - 30052 # Purging Draught
  budgets:
    wall_clock: 20m

goals:
  - >-
    Type `conditions` and quote the list verbatim. Then type `inventory` and
    confirm you carry a Warrior's Brew and a Purging Draught.
  - >-
    Type `drink brew`. Quote every line you receive, verbatim. You should read
    the drink line and then a line about the brew taking hold. Type
    `conditions` again and quote it: Warrior's Brew must be listed.
  - >-
    Wait, checking `conditions` every few rounds, until the Warrior's Brew
    leaves the list. Quote the line you receive when it fades, verbatim. A
    silent expiry (the buff leaves `conditions` with no line) is the defect
    this lane exists to find.
  - >-
    Type `drink draught`. Quote every line: the purge taking hold, the purge
    ending, and then the weakness that follows, each with its own line. Wait
    until the weakness fades and quote that line too.
  - >-
    At no point should `conditions` list Meditating, Respawn Grace,
    InfraredVision or Alt Character Mob, and no line should read "takes
    effect." or "has expired." with nothing else (that is the generic
    fallback, which no shipped buff should reach). Report any such line.
```

Run it per `dogmud-playtesting` (`/playtest local --checkout <abs> feature-tester <goals>`), read the transcript, fix anything it finds, re-run. Extract findings to memory (reports are gitignored).

- [ ] **Step 3: Commit.**
```bash
git add tools/playtest/goals/2026-09-12-slice-c-buff-notices.yaml
git commit -m "test(playtest): slice C lane, a potion and an affliction say hello and goodbye" -m "Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Self-review

- Spec: resolver (Task 1), one door in the two hooks (Task 2), boot warning (Task 3), content and four secret buffs (Task 4), root guard (Task 5), docs (Task 6), gates and playtest (Task 7).
- Names agree across tasks: `StartUserNotice`, `EndUserNotice`, `SilentNoticeBuffs`, `WarnSilentNotices`, `TestEveryDogmudBuffHasAuthoredNotices`.
- Task 5 depends on Task 4 (the guard is green only once the content exists); the sabotage step is what proves it capable of failing.
