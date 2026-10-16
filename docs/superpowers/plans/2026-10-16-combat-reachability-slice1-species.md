# Combat reachability slice 1: species corrections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Every subagent brief carries these rules:** edit files with the Edit and Write tools only (`~/.claude/hooks/guard.py` blocks shell writes into any git working tree); `git add` named paths only; never `--no-verify`; no em or en dashes in prose or player text; no colon inside an unquoted YAML value; every `gh` command carries `--repo pruuk/DOGMud`; scratch output goes to the session scratchpad, never `$TMP`. A world-data change runs the FULL `go test . ./internal/... ./modules/...`, not scoped packages (scoped `internal/rooms` tests do not load real zones).

**Goal:** Every creature PR #208 found on the wrong species is the species it looks like, and the `bird` and `horse` species exist, with nothing else from #208 carried along.

**Architecture:** Data only. Two species files are ported from the #208 review worktree with only the fields master's loader reads, and with `speech: false` added; 32 mob files change one `speciesid` value each. Two table tests pin the result. Slice 2 (beast moves as species data) depends on this.

**Tech Stack:** YAML world data, Go tests in `internal/species`.

**Spec:** `docs/superpowers/specs/2026-10-16-combat-message-reachability-design.md`, section 0 and rulings G and H. Source: PR #208 (FinalTwist, open), review worktree `C:/tmp/dogmud-pr208-review` at `3ce674451`, base `c696c117a`.

---

## Facts verified against source (master `779746a31` plus the spec merge, 2026-10-16)

| # | Fact | Source |
|---|---|---|
| 1 | #208 changes `speciesid` on 32 mob files and adds `species/25-bird.yaml` and `species/26-horse.yaml`; each of the 32 mobs still carries #208's base species on master | diff of the review worktree against its base; `git show origin/master:<file>` per mob |
| 2 | Of the 32, only `watchers_crossing/87-river_lurker.yaml` has other #208 changes: it removes the item `40048` from the mob's `items:` and adds a `harvest:` block (venom sac) | per-file diff |
| 3 | The `Species` struct (`internal/species/species.go:20-70`) has no `harvest` field; #208's bird and horse files carry a `harvest:` block (hunting, salvage survey item 8) | file |
| 4 | `Speech *bool` (`species.go:47`), read by `CanSpeak` (`:74`): absent means the species speaks. Every beast species on master sets `speech: false`; only humanoids and undead omit it. #208's bird and horse files omit it | grep `-L speech:` |
| 5 | Bird: `natural_attack: claws`, `unarmedname: beak and claws`, body `[legs, eyes, mouth, skin]`, `intrinsic_mutations: keen-senses: 1`, size small. Horse: `natural_attack: slam`, `unarmedname: hooves`, body `[legs, eyes, mouth, skin, tail]`, `tail: 1`, size large. `mutations/keen-senses.yaml` and `mutations/tail.yaml` exist on master; claws and slam are allowed natural attacks (`validNaturalAttacks`, `species.go:212`) | files |
| 6 | Mob files hold the species at `character.speciesid` | `87-river_lurker.yaml` diff hunk |
| 7 | Test helper `dogmudSpecies(t) map[int]*Species` reads `_datafiles/world/dogmud/species` from package `species` tests | `internal/species/species_night_vision_test.go:20` |
| 8 | A scan of every mob name against species found no other creature on a clearly wrong species (the only hits were names like "Maret" matching "mare") | scratch `speciesscan.py`, `speciesscan2.py` |

## Task 0: Audit of PR #208's species work (done 2026-10-16; owner rules on the table)

The controller read each of the 32 mobs' own descriptions on master against #208's new species, read both new species files whole, and scanned the whole mob roster for anything #208 missed (Fact 8). 24 reassignments are plainly right (wolves, foxes, coyotes and the wild dog to canine; hawks to raptor; crows, sparrows, grouse, the chicken and herons to bird; otters to mustelid; mice and the river rat to rodent; the feral hog to boar; the farm cat to feline; the gar to fish; the sun lizard and the river lurker to reptile; Hob and Bran to horse; the pale cave crawler and spore crawler to insectoid). The rest, and the two species files, are below.

### Decisions (owner)

| # | Item | #208 did | Concern | Options | Recommended |
|---|---|---|---|---|---|
| D1 | Fungal Grub (`labyrinth_of_low_tunnels/77`) | rodent → insectoid | A legless larva ("fat, pale grub ... leaving shallow furrows"); insectoid has legs, so it would qualify for leg moves | insectoid (as #208); worm (18: mouth and skin, no legs) | **worm** |
| D2 | Scavenger Bird (`dustwalk_road/82`) | bat → raptor | A carrion eater ("dropped food, abandoned packs, or ... something that stops moving"); #208 put the two carrion crows on bird, and its own bird species says "not a hunter of other beasts" | raptor (as #208, hunting-bird moves); bird | **bird** |
| D3 | Canal Lurker (`new_plymouth_old_quarter/9393`) | serpent → insectoid | "Between an eel and a crayfish ... too many short legs", in water | insectoid (as #208); a new `crustacean` species | **insectoid** now; a crustacean species only when a second one exists |
| D4 | Cave Lurker (`amber_valley/9409`) | serpent → insectoid | "A low armoured bulk of slate-grey plate and folded limbs", hunts by ambush | insectoid (as #208); arachnid (trapdoor-style) | **insectoid** (owner 2026-10-16: insectoid for now; arachnid preferred later) |
| D5 | Bird's default attack | `claws`, "beak and claws" | Crows and herons peck; claws copy speaks of talons and slashes | claws (as #208); a new `peck` attack with its own combat-message file and copy | **claws** now; `peck` later if birds fight often |
| D6 | Horse's default attack | `slam`, "hooves" | `slam.yaml` was written for heavy bodies and elementals; some lines read oddly with hooves ("You rear up, preparing to hurl your hooves at ...", lines 19, 103-104) | slam (as #208); a new `kick` or `trample` attack later | **slam**; the odd lines are fixed in the follow-up below, between this plan and slice 2 (owner 2026-10-16) |
| D7 | River Lurker's extra changes | removes loot item `40048`, adds a `harvest:` table | Harvesting does not exist on master (Fact 3); removing the item loses its drop | port the species only and keep the item (this plan); port #208's change too | **species only, keep the item** |
| D8 | Jackrabbit (`east_road_to_greenford/9499`) | serpent → rodent | A rabbit is not a rodent | rodent (as #208); a new `lagomorph` species | **rodent** |

### Fixed without a decision (bring back only if you disagree)

- Both new species gain `speech: false` (Fact 4): without it, chickens and horses count as able to speak.
- Both new species drop #208's `harvest:` block (Fact 3; hunting is salvage item 8, not this work). Why #208 has it at all: it extends our corpse salvage with a species fallback (`internal/crafting/corpse_salvage.go`, `LookupCorpseSalvageForMob`) and ALSO adds a separate `carcass` skin and butcher system (`internal/usercommands/carcass.go`, `internal/species/harvest.go`) with per-species `harvest:` tables, tools, rare parts and spoilage. The two do not talk to each other: our review's finding F065 is that skinning and then salvaging the same corpse pays twice. If skinning and butchering are ever wanted, they extend `salvage` rather than sit beside it.

### Final reassignment table (with the recommended decisions)

| # | Mob file | Name | From | To | Note |
|---|---|---|---|---|---|
| 1 | `amber_valley/9407-a_sun_lizard.yaml` | A Sun-Lizard | 8 serpent | 21 reptile |  |
| 2 | `amber_valley/9408-a_pale_cave_crawler.yaml` | A Pale Cave-Crawler | 8 serpent | 12 insectoid |  |
| 3 | `amber_valley/9409-a_cave_lurker.yaml` | A Cave Lurker | 8 serpent | 12 insectoid | D4 |
| 4 | `ashwick/263-forest_fox.yaml` | Forest Fox | 10 rodent | 2 canine |  |
| 5 | `ashwick/264-timber_wolf.yaml` | Timber Wolf | 10 rodent | 2 canine |  |
| 6 | `ashwick/265-briar_hawk.yaml` | Briar Hawk | 20 orb | 9 raptor |  |
| 7 | `ashwick/266-village_chicken.yaml` | Village Chicken | 20 orb | 25 bird |  |
| 8 | `ashwick/267-cottage_mouse.yaml` | Cottage Mouse | 21 reptile | 10 rodent |  |
| 9 | `dustwalk_road/82-scavenger_bird.yaml` | Scavenger Bird | 22 bat | **25 bird** | D2 (#208: 9 raptor) |
| 10 | `east_road_to_greenford/9498-a_wheeling_hawk.yaml` | A Wheeling Hawk | 8 serpent | 9 raptor |  |
| 11 | `east_road_to_greenford/9499-a_jackrabbit.yaml` | A Jackrabbit | 8 serpent | 10 rodent | D8 |
| 12 | `greenford/9507-a_grey_heron.yaml` | A Grey Heron | 8 serpent | 25 bird |  |
| 13 | `greenford/9508-a_river_otter.yaml` | A River Otter | 8 serpent | 24 mustelid |  |
| 14 | `greywater_flats/9194-a_grey_heron.yaml` | A Grey Heron | 9 raptor | 25 bird |  |
| 15 | `ironwind_steppe/201-dust_crow.yaml` | Dust Crow | 9 raptor | 25 bird |  |
| 16 | `ironwind_steppe/214-sage_grouse.yaml` | Sage Grouse | 9 raptor | 25 bird |  |
| 17 | `ironwind_steppe/233-carrion_crow.yaml` | Carrion Crow | 9 raptor | 25 bird |  |
| 18 | `labyrinth_of_low_tunnels/77-fungal_grub.yaml` | Fungal Grub | 10 rodent | **18 worm** | D1 (#208: 12 insectoid) |
| 19 | `labyrinth_of_low_tunnels/78-spore_crawler.yaml` | Spore Crawler | 10 rodent | 12 insectoid |  |
| 20 | `marches_spur_road/255-scrub_coyote.yaml` | Scrub Coyote | 10 rodent | 2 canine |  |
| 21 | `marches_spur_road/256-feral_hog.yaml` | Feral Hog | 7 deer | 6 boar |  |
| 22 | `marches_spur_road/257-field_sparrow.yaml` | Field Sparrow | 20 orb | 25 bird |  |
| 23 | `marches_spur_road/258-farm_cat.yaml` | Farm Cat | 21 reptile | 11 feline |  |
| 24 | `new_plymouth_old_quarter/9393-a_canal_lurker.yaml` | A Canal Lurker | 8 serpent | 12 insectoid | D3 |
| 25 | `north_road/326-river_rat.yaml` | River Rat | 1 human | 10 rodent |  |
| 26 | `north_road/327-wild_dog.yaml` | Wild Dog | 1 human | 2 canine |  |
| 27 | `river_road/9416-a_grey_heron.yaml` | A Grey Heron | 8 serpent | 25 bird |  |
| 28 | `river_road/9417-a_river_otter.yaml` | A River Otter | 8 serpent | 24 mustelid |  |
| 29 | `river_road/9418-a_river_gar.yaml` | A River Gar | 8 serpent | 13 fish |  |
| 30 | `thornwall_city/375-hob.yaml` | Hob | 1 human | 26 horse |  |
| 31 | `thornwall_city/376-bran.yaml` | Bran | 1 human | 26 horse |  |
| 32 | `watchers_crossing/87-river_lurker.yaml` | River Lurker | 1 human | 21 reptile | D7: species only, item `40048` stays |

If the owner overrules a decision, only that row's "To" changes, here and in Task 2's test.

## File structure

| File | Responsibility |
|---|---|
| Create `_datafiles/world/dogmud/species/25-bird.yaml`, `26-horse.yaml` | the two species |
| Create `internal/species/species_new_bird_horse_test.go` | pins the two species |
| Create `internal/species/species_assignment_test.go` | pins the 32 mobs |
| Modify the 32 mob files | one `speciesid` each |
| Modify `internal/species/context.md`, `internal/species/species.go` (comment only), `docs/README.md` | docs (ruling H) |

### Task 1: The bird and horse species

**Files:** Create `internal/species/species_new_bird_horse_test.go`, `_datafiles/world/dogmud/species/25-bird.yaml`, `_datafiles/world/dogmud/species/26-horse.yaml`.

- [ ] **Step 1: Write the failing test.**

```go
package species

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

// Slice 1 of the combat reachability spec (#541): the bird and horse species
// ported from PR #208, speechless beasts like every other animal species.
func TestShippedBirdAndHorseSpecies(t *testing.T) {
	all := dogmudSpecies(t)
	cases := []struct {
		id      int
		name    string
		attack  items.ItemSubType
		unarmed string
		parts   []string
	}{
		{25, "bird", items.Claws, "beak and claws", []string{"legs", "eyes", "mouth", "skin"}},
		{26, "horse", items.Slam, "hooves", []string{"legs", "eyes", "mouth", "skin", "tail"}},
	}
	for _, c := range cases {
		sp := all[c.id]
		require.NotNil(t, sp, "species %d missing", c.id)
		require.Equal(t, c.name, sp.Name)
		require.Equal(t, c.attack, sp.NaturalAttack)
		require.Equal(t, c.unarmed, sp.UnarmedName)
		require.ElementsMatch(t, c.parts, sp.BodyParts)
		require.False(t, sp.CanSpeak(), "%s must set speech: false like every beast", c.name)
		require.False(t, sp.Selectable)
		require.True(t, sp.Tameable)
	}
}
```

Check the field names (`UnarmedName`, `BodyParts`, `Selectable`, `Tameable`) against `species.go` before running; the test must name real fields.

- [ ] **Step 2: Run it.** `go test ./internal/species/ -run TestShippedBirdAndHorseSpecies` Expected: FAIL, "species 25 missing".
- [ ] **Step 3: Port the files.** Write each from the review worktree's copy (`C:/tmp/dogmud-pr208-review/_datafiles/world/dogmud/species/25-bird.yaml`, `26-horse.yaml`; read them whole first) with two changes: add `speech: false` after `tameable:` (the position other beast files use; check `species/11-feline.yaml`), and drop the `harvest:` block with its two comment lines. Keep every other line as #208 wrote it.
- [ ] **Step 4: Run.** `go test ./internal/species/` then the full `go test . ./internal/... ./modules/...` (species load at boot through other packages' tests). Expected: PASS.
- [ ] **Step 5: Commit.** `content(species): bird and horse species, ported from PR #208 (#541)`.

### Task 2: The 32 reassignments

**Files:** Create `internal/species/species_assignment_test.go`; Modify the 32 mob files.

- [ ] **Step 1: Write the failing test.** The `want` map is the final table above (owner decisions applied):

```go
package species

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Slice 1 of the combat reachability spec (#541): creatures PR #208 found on
// the wrong species are the species they look like. Owner decisions D1 to D8
// are recorded in docs/superpowers/plans/2026-10-16-combat-reachability-slice1-species.md.
func TestCreaturesAreTheSpeciesTheyLookLike(t *testing.T) {
	want := map[string]int{
		"amber_valley/9407-a_sun_lizard.yaml":               21,
		"amber_valley/9408-a_pale_cave_crawler.yaml":        12,
		"amber_valley/9409-a_cave_lurker.yaml":              12,
		"ashwick/263-forest_fox.yaml":                       2,
		"ashwick/264-timber_wolf.yaml":                      2,
		"ashwick/265-briar_hawk.yaml":                       9,
		"ashwick/266-village_chicken.yaml":                  25,
		"ashwick/267-cottage_mouse.yaml":                    10,
		"dustwalk_road/82-scavenger_bird.yaml":              25,
		"east_road_to_greenford/9498-a_wheeling_hawk.yaml":  9,
		"east_road_to_greenford/9499-a_jackrabbit.yaml":     10,
		"greenford/9507-a_grey_heron.yaml":                  25,
		"greenford/9508-a_river_otter.yaml":                 24,
		"greywater_flats/9194-a_grey_heron.yaml":            25,
		"ironwind_steppe/201-dust_crow.yaml":                25,
		"ironwind_steppe/214-sage_grouse.yaml":              25,
		"ironwind_steppe/233-carrion_crow.yaml":             25,
		"labyrinth_of_low_tunnels/77-fungal_grub.yaml":      18,
		"labyrinth_of_low_tunnels/78-spore_crawler.yaml":    12,
		"marches_spur_road/255-scrub_coyote.yaml":           2,
		"marches_spur_road/256-feral_hog.yaml":              6,
		"marches_spur_road/257-field_sparrow.yaml":          25,
		"marches_spur_road/258-farm_cat.yaml":               11,
		"new_plymouth_old_quarter/9393-a_canal_lurker.yaml": 12,
		"north_road/326-river_rat.yaml":                     10,
		"north_road/327-wild_dog.yaml":                      2,
		"river_road/9416-a_grey_heron.yaml":                 25,
		"river_road/9417-a_river_otter.yaml":                24,
		"river_road/9418-a_river_gar.yaml":                  13,
		"thornwall_city/375-hob.yaml":                       26,
		"thornwall_city/376-bran.yaml":                      26,
		"watchers_crossing/87-river_lurker.yaml":            21,
	}
	require.Len(t, want, 32)
	species := dogmudSpecies(t)
	dir := filepath.Join("..", "..", "_datafiles", "world", "dogmud", "mobs")
	for rel, id := range want {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		require.NoError(t, err, rel)
		var mob struct {
			Character struct {
				SpeciesId int `yaml:"speciesid"`
			} `yaml:"character"`
		}
		require.NoError(t, yaml.Unmarshal(data, &mob), rel)
		require.Equal(t, id, mob.Character.SpeciesId, "%s", rel)
		require.NotNil(t, species[id], "%s names species %d, which does not exist", rel, id)
	}
}
```

Check that `speciesid` sits under `character:` in a sample of the 32 files (Fact 6 shows it for one); if a file nests it differently, adjust the struct, not the file.

- [ ] **Step 2: Run it.** Expected: FAIL on every row (master still has the old species).
- [ ] **Step 3: Apply.** In each of the 32 files, change the one `speciesid:` line from the table's "From" to its "To" with the Edit tool, after checking the file's current value equals "From". Touch nothing else (D7: the river lurker keeps item `40048` and gets no `harvest:`).
- [ ] **Step 4: Run.** `go test ./internal/species/` then the full `go test . ./internal/... ./modules/...`. Expected: PASS. A test elsewhere that pinned one of these mobs' old species (grep the mob ids in `_test.go` files) is updated and listed.
- [ ] **Step 5: Commit.** `content(mobs): 32 creatures become the species they look like, from PR #208 (#541)`.

### Task 3: Docs and the comment (ruling H)

- [ ] **Step 1:** `internal/species/context.md`: under the natural-attack section, the principle in the owner's words (a species' default attack is what it does most of the time; its special moves are rarer or finishing moves), with a pointer to the spec; the two new species in any species list the file keeps; and a line that a creature's species must match what it is (wolves canine, birds bird or raptor by whether they hunt), with this plan as the record.
- [ ] **Step 2:** `internal/species/species.go`, on the `NaturalAttack` field's comment: one sentence, "The attack this species makes most of the time; special moves are rarer or finishers (spec 2026-10-16 combat-message reachability, ruling E)." Keep the existing comment lines.
- [ ] **Step 3:** `docs/README.md`: a row for this plan. Run `python tools/context_md_audit.py internal/species`.
- [ ] **Step 4:** Commit `docs(species): the default-attack principle; bird and horse (#541)`.

### Task 4: Gate, boot, PR

- [ ] **Step 1:** `gofmt -l internal/ modules/`; `go build ./...`; `golangci-lint run --new-from-merge-base=origin/master` (0 issues); the full `go test . ./internal/... ./modules/...`; a YAML shape check of the 32 files against master (each differs in exactly one line); the boot check per `dogmud-shipping` (exit 124, `Server Ready` 1, no panics).
- [ ] **Step 2:** PR with `--repo pruuk/DOGMud`, body naming slice 1 of the #541 spec, the decision table as ruled, and the evidence. Part of #541 and #539 (no closing keyword).
- [ ] **Step 3:** After merge, comment on PR #208: its species corrections (32 mobs, bird and horse) landed separately in the merge commit, with the two differences (D1 fungal grub on worm, D2 scavenger bird on bird) and the speech and harvest notes, so FinalTwist can drop them from #208.

## Follow-up before slice 2: slam lines that only fit fists (D6, owner 2026-10-16)

A small copy PR after this plan merges and before slice 2 starts.

- [ ] **Step 1: Audit.** Every `slam.yaml` line holding `{itemname}` is read with each slam species' `unarmedname` substituted (the horse's "hooves" and the 11 other slam species: `grep -h "unarmedname" _datafiles/world/dogmud/species/*.yaml` for the slam ones). A line that only works for something thrown or gripped ("You hurl your {itemname} at ...", lines 19, 103-104 today) is listed; a line about the body's weight ("you hurl your weight into ...") is fine and is not.
- [ ] **Step 2: Owner review.** The listed lines and their proposed rewrites go to the owner as a table (today, proposed, why), each rewrite fitting 80 columns and every role of a record describing one moment (the M6 rule).
- [ ] **Step 3: Apply** the approved rewrites, run the combat-message guards (`go test . ./internal/combat/ ./internal/items/`) and the narration golden re-record per its documented procedure, then the full suite, and open the PR (part of #541, #385).

## Self-review

- Spec section 0: species ported with master's fields only (Task 1), 32 changes checked against the base value (Task 2), table test (Task 2), boot (Task 4), #208 told (Task 4). Ruling H (Task 3).
- The owner asked for an audit that does not trust #208: Task 0 records it, with every judgement call in the decision table and the two silent fixes (speech, harvest) stated.
