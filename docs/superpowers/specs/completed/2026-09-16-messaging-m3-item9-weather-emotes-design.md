# Messaging M3 item 9: weather emotes

Date: 2026-09-16
Arc: [messaging unification](2026-08-31-messaging-unification-design.md), M3 item 9
Status: designed, not implemented

The last store in M3. The arc spec's row for it reads: *"Adjacent: joins
renderer, tokens and pipeline, keeps its actorless shape"*
(`2026-08-31-messaging-unification-design.md:574`), and the scope section adds
*"Weather stays adjacent, not absorbed. Same renderer, tokens and pipeline; it
keeps its actorless shape rather than pretending an ambient line has an Actor"*
(`:266`).

---

## Facts verified against source

Every row below was read from the source on 2026-09-16, not recalled.

| Fact | Value | Source |
|---|---|---|
| Emote store | `Tables`, `Table`, `TableSection`, `IndoorPool` | `modules/weather/content/emotes.go:50,39,29,21` |
| Pick, weather | `Tables.Pick`, hand-rolled `roll(len(lines))` | `emotes.go:100`, roll at `:119` |
| Pick, seasonal | `SeasonalTables.Pick`, same hand-rolled roll | `emotes.go:196`, roll at `:205` |
| Section resolver | `bandedSectionLines`, biome then `"default"` | `emotes.go:129` |
| Felt threshold | `StrongFeltThreshold = 0.5` | `emotes.go:16` |
| Loader | `LoadEmotes`, `LoadSeasonalEmotes` | `emotes.go:68,167` |
| Emitter | `EmitAmbient`, one line per occupied room per pass | `modules/weather/engine/emotes.go:39` |
| Delivery | `room.SendText(messaging.CategoryWeather, line)` | `engine/emotes.go:80,91` |
| Emitter already passes | `biomeId, indoor` from `room.GetBiome()` | `engine/emotes.go:53-56` |
| Narration core | `Variants`, `Roles`, `Render`, `ValidateVariants`, `Role` | `internal/narration/render.go:23,39,92,202,158` |
| Picker seam | `Picker`, `DefaultPicker`, `SequencePicker`, `FirstPicker` | `internal/narration/picker.go:10,14,28,51` |
| `internal/narration` imports | `render.go`: stdlib only. `picker.go`: `internal/util`. No game state. | both import blocks |
| `modules/weather/content` arch rule | `TestContentPackageStaysPure` FORBIDS any `internal/*` import | `modules/weather/content/arch_test.go` |
| Any `modules/` importing narration | **none today** | grep `internal/narration` over `modules/` |
| Biome record | `BiomeInfo`, `Indoor bool` | `internal/rooms/biomes.go:14,25` |
| Underground concept in code | **none anywhere** | grep `underground` over `internal/` and `modules/` |
| Biome grouping prior art | **none** | grep biome + group/class/categor/kind |
| Tokens in weather content | **zero** | grep proven capable: 717 matches in `defense-messages/`, 0 in `weather/emotes/` |
| ANSI tags in weather content | **zero** | same grep |
| Weather emote files | 9 base + 6 seasonal = 15 | `_datafiles/world/dogmud/weather/emotes/` |
| Authored lines today | 102 | list items across all 15 files |
| Pool depth today | 1 to 4 | per file count |
| Indoor biome keys authored | `default` only, in all 9 files | `awk '/^indoor:/,0'` per file |
| Goldens in the harness | 13, **none for weather** | `internal/narration/testdata/stores/` |
| Shipped-data test precedent | `os.DirFS("../../../_datafiles/world/dogmud")` | `modules/weather/content/shipped_emotes_test.go:14` |

### Indoor biomes, with room counts

| Biome | `indoor` | `darkarea` | Rooms | Description as authored | Class |
|---|---|---|---|---|---|
| `cave` | true | true | **123** | "Dark areas underground." | underground |
| `dungeon` | true | true | 1 | "cave-like underground areas built with a purpose." | underground |
| `fort` | true | false | 22 | "Forts are cities or dwellings that are fortified." | surface indoor |
| `house` | true | false | 15 | "Standard domiciles and other dwellings." | surface indoor |
| `spiderweb` | true | true | **0** | "Sticky strands of web... The domain of spiders." | surface indoor (unused) |

161 indoor rooms, which matches the filed bug
[[project-indoor-weather-prose-written-for-a-house]] exactly.

**`spiderweb` is not underground.** Its darkness comes from webbing, not from
stone. It is classified surface indoor because that is the less wrong fallback,
and because no room uses it, so no prose is authored for it. If it is ever used
it wants its own biome keyed pool rather than either default. This correction
matters: the first pass of this design assumed dark plus indoor meant
underground, and reading the biome description disproved it.

---

## The defect this slice closes

All 31 shipped indoor lines assume a **built structure**. Verbatim samples:

- rain: `"The sound of rain on the roof settles into a rhythmic, soothing patter."`
- storm: `"Wind moans low around the eaves."`
- snow: `"Cold seeps up through the floorboards; the fire seems to give less heat."`
- frost: `"Your breath fogs indoors, and the very nails in the wood feel like ice."`
- dust: `"Fine dust sifts down from the rafters with each gust outside."`

Roofs, eaves, floorboards, nails, rafters, panes, shutters. Since `indoor`
resolves to `default` for every indoor biome, **124 of the 161 indoor rooms are
underground and receive house prose.**

A second defect the owner named: several indoor lines narrate the weather
*outside* rather than what is felt inside, which reads as misplaced outdoor
prose.

- snow: `"Snow whispers against the windows, piling soft in the corners outside."`
- fog: `"The world beyond the glass has simply dissolved into pale nothing."`

---

## Owner decisions, 2026-09-16

1. **Two PRs, item 8's shape**: mechanism, then content.
2. **Three way split**: outdoors, indoors, underground, as a third section in
   the weather schema. Not per biome content keys, and not a new flag on
   `BiomeInfo`.
3. **Classification lives in a Go map** in `modules/weather/content`, beside the
   tables it serves. Not a new YAML load path, and not derived from
   `indoor && darkarea` (that correspondence is a coincidence of two unrelated
   flags, and `spiderweb` proves it wrong).
4. **Full content pass, all three sections**: outdoor pools padded, indoor
   rewritten, underground authored.
5. **Depth 6** for every non empty pool.
6. **Comments and guards** for the biome coupling, because a hardcoded biome
   list with no enforcement reintroduces exactly the defect this slice fixes.

---

## Mechanism (PR 1)

### 1. Three way section split

`TableSection` gains `Underground`. `Table` today duplicates `Outdoor` and
`Indoor` as its own fields alongside a `Seasonal map[string]TableSection`, so
adding a section would mean adding it twice. `Table` instead embeds
`TableSection` inline, and the seasonal variants and the seasonal ambience
tables inherit the new section for free.

```go
type TableSection struct {
    Outdoor     map[string][]string   `yaml:"outdoor"`
    Indoor      map[string]IndoorPool `yaml:"indoor"`
    Underground map[string]IndoorPool `yaml:"underground"`
}

type Table struct {
    Weather      string `yaml:"weather"`
    TableSection `yaml:",inline"`
    Seasonal     map[string]TableSection `yaml:"seasonal"`
}
```

`yaml.v2` supports `,inline` on an embedded struct. The on disk schema of the
existing sections is unchanged, so all 15 files keep parsing.

### 2. Class resolution stays inside the store

```go
// undergroundBiomes names the biomes whose weather is felt through stone
// rather than through walls. Any change to the biome set must be reflected
// here; modules/weather/content/biome_coupling_test.go fails the build
// otherwise.
var undergroundBiomes = map[string]bool{"cave": true, "dungeon": true}

// surfaceIndoorBiomes names the sheltered-but-not-underground biomes. Present
// so that classification is TOTAL and a newly added indoor biome cannot
// silently inherit house prose.
var surfaceIndoorBiomes = map[string]bool{"fort": true, "house": true, "spiderweb": true}
```

`bandedSectionLines` takes the three maps and resolves class from the
`(biome, indoor)` pair that `EmitAmbient` already passes:

- `!indoor` gives outdoor
- `indoor && undergroundBiomes[biome]` gives underground
- otherwise indoor

**`engine.EmitAmbient` therefore needs no change at all**, and the sim side is
untouched. Underground keeps the existing `Mild` / `Strong` felt banding and
the existing rule that indoor never falls back to outdoor, because silence
beats wrong prose.

### 3. Join the narration core

`Tables.Pick` and `SeasonalTables.Pick` replace their hand-rolled
`roll(len(lines))` with the core:

```go
r := narration.Render(narration.Variants{Observer: lines}, tokens, pick)
return r.Observer
```

and take a `narration.Picker` in place of `roll func(int) int`. The two types
are structurally identical (`func(n int) int`), so the call sites in
`EmitAmbient` change only in the type they name.

Weather is **actorless**: `Observer` alone is populated. No Actor, no Actee, no
pretending an ambient line has a subject. This is what the arc spec means by
keeping its shape.

What the join buys, given the content is tokenless today:

- the deterministic picker seam, which is what makes a golden possible at all
- token substitution, so an author can later write `{season}` without new
  machinery
- a real load time validator

`modules/weather/content` gains an import of `internal/narration`, and is the
first `modules/` package to do so. `narration` pulls in stdlib plus
`internal/util` and no game state, so there is no cycle.

⚠️ **There is also an ARCHITECTURE RULE, which this spec originally missed.**
`modules/weather/content/arch_test.go` carries `TestContentPackageStaysPure`,
forbidding the package from importing **any** `internal/*` package: content
parses module data, and engine access belongs in `engine/`. Found during
implementation, when the test failed the build.

The import is still the right call, but it has to be a deliberate, narrow
widening rather than a silent one: the test grows an
`allowedInternalImports` allowlist containing `internal/narration` alone, with
a comment recording why. Every other `internal/` package stays forbidden, so
the boundary still fails loudly the next time someone reaches across it for
rooms, mobs or players.

### 4. Validator: silence is a legal value

`mild: []` is deliberate. Light weather is meant to be inaudible indoors, and
an empty pool is how an author says so. A flat minimum would reject it.

⚠️ **CORRECTION, found during implementation: mild is NOT always empty.**
`storm`, `blizzard` and `dust` each shipped ONE `indoor.mild` line, and the
`IndoorPool` doc comment says "usually empty", not always. Those three are
exactly the types `shipped_emotes_test.go` already treats as severe, which
makes it a real pattern rather than an oversight: a mild storm genuinely is
audible in a house, even when light rain is not.

They are therefore padded to depth 6 rather than emptied. Deleting correct
content to satisfy a depth rule would be backwards, and the rule already has
the right shape for this: empty BY INTENT, or deep enough not to repeat.

The rule is therefore **empty by intent, or at least 6**, applied per pool:

```go
// ValidatePool enforces the shipped depth contract for ONE pool. An empty
// pool is legal and means deliberate silence (light weather is inaudible
// through walls); a non-empty one must be deep enough not to repeat.
func ValidatePool(lines []string) error {
    if len(lines) == 0 {
        return nil
    }
    return narration.ValidateVariants(
        narration.Variants{Observer: lines}, minPoolDepth, narration.RoleObserver)
}
```

with `minPoolDepth = 6`. Weather is the only store with meaningful silence,
which is what justifies a wrapper rather than calling `ValidateVariants`
directly; the wrapper adds the empty case and delegates everything else.

Validation runs at **load** time in `LoadEmotes` and `LoadSeasonalEmotes`. Per
the arc's standing lesson, a policy of "fail on bad data" must run where the
data is loaded, not lazily on first use.

Failure policy: `LoadEmotes` today returns an error and the caller fails soft
(the module logs and runs with empty tables, which is silence). **That is
preserved.** Weather is ambient; a bad emote file should not stop the world
booting. The shipped-data tests below are what actually hold the contract, and
they fail the build rather than the server.

---

## Content (PR 2)

Full pass to depth 6 across all three sections of all 9 weather types, plus the
6 seasonal ambience files.

| Section | Today | After |
|---|---|---|
| `outdoor.default` | 3 to 4 per type | 6 per type |
| `outdoor.<biome>` | 1 to 2 per pool | 6 per pool |
| `indoor.strong` | 3 to 4, house prose | 6, rewritten for a built interior |
| `indoor.mild` | empty for 6 types, **1 line for `storm`, `blizzard`, `dust`** | empty for 6, depth 6 for those three |
| `underground.strong` | absent | 6, authored new |
| `underground.mild` | absent | empty (deliberate) |
| seasonal | 1 to 6 per file | 6 per non empty pool |

Roughly 234 lines authored or rewritten.

**Voice rules**, from the player copy skill and the arc's existing content work:

- 80 character hard wrap, already enforced by `shipped_emotes_test.go`
- no raw numbers
- ESL clear phrasing
- indoor lines describe **what is felt inside**, not what is visible outside
- underground lines use stone, seepage, draught, distant transmitted sound,
  mineral cold. No roofs, panes, shutters, eaves, rafters or floorboards.

Worked example, rain:

```yaml
indoor:
  default:
    mild: []
    strong:
      - "Rain drums steadily overhead, close and constant."
      # ...to depth 6, all describing the interior
underground:
  default:
    mild: []
    strong:
      - "Water finds a seam in the rock and begins a slow, patient dripping."
      - "The stone sweats; every surface turns slick and cold to the touch."
      # ...to depth 6
```

---

## Guards

Four shipped-data tests in
**`modules/weather/content/biome_coupling_test.go`** (new file), following the
`shipped_emotes_test.go` precedent of reading real data through
`os.DirFS("../../../_datafiles/world/dogmud")`. Reading the biome YAML directly
keeps `internal/rooms` out of the production import graph of a content package.

| # | Guard | Catches |
|---|---|---|
| 1 | Every biome with `indoor: true` is in **exactly one** of the two maps | A biome being **added**. The important one. |
| 2 | Every id in either map is a real biome | A biome being **removed or renamed** (dead key) |
| 3 | Every biome key authored in any weather file is a real biome id or `default` | A **typo** such as `forset:`, which today falls back to default silently and forever |
| 4 | Every non empty pool holds at least 6 | Shallow pools reintroducing repetition |

Guard 1 is the one that matters. Without it, adding a `crypt` biome with
`indoor: true` silently serves house prose in a crypt, which is precisely the
defect being fixed here. With it, that is a red build naming the biome and the
two maps to choose between.

Guard 1 must be **proven capable of failing** before it is trusted: add a
throwaway biome YAML, confirm the test goes red and names it, then remove it.
A guard that cannot fail is not a guard.

### Comments

- **`internal/rooms/biomes.go`**, on the `Indoor` field: a note that adding,
  renaming or removing a biome requires classifying it in
  `modules/weather/content/emotes.go` (`undergroundBiomes` /
  `surfaceIndoorBiomes`), naming the file and the maps explicitly rather than
  saying "see the weather module".
- **`internal/rooms/context.md`**: record the coupling.
- **`modules/weather/content/context.md`**: required by the project rule, since
  the data model changes.

Honest limit: guard 1 fires in CI and in `go test ./...`, but someone adding a
biome may only run `go test ./internal/rooms/...` locally. The comment on the
`Indoor` field is what closes that gap, which is why it names the exact file and
maps.

---

## Testing

### The golden comes first, from pre-migration code

There is **no weather golden** among the 13 in
`internal/narration/testdata/stores/`. Item 8's rule applies: a golden recorded
after the refactor bakes the refactor's bugs into the baseline invisibly.

Order is therefore fixed:

1. Record `weather_emotes.golden` against **today's** `Tables.Pick`, using a
   deterministic picker, covering every weather type, every authored biome key,
   both indoor bands, and the seasonal tables.
2. Prove the golden can fail: sabotage the picker, confirm red, revert.
3. Migrate onto the narration core.
4. The golden must come out **byte identical**.

The content PR is where the golden legitimately changes, and it changes there
alone.

### Null probe

A picker sabotage in `Tables.Pick` that **compiles clean under `go vet`** and
turns the golden red. The compile requirement is from the arc's record: a
sabotage that fails to compile proves nothing.

### Unit tests

- class resolution: outdoor, indoor and underground each selected for a
  representative biome, including the `!indoor` case and the unknown biome
  fallback to `default`
- underground never falls back to indoor or outdoor (silence beats wrong prose)
- `mild` selected below `StrongFeltThreshold`, `strong` at and above it
- `ValidatePool` accepts empty, rejects 1 through 5, accepts 6
- seasonal tables resolve the underground section

### Playtest gate

The arc ends every content slice with a playtest. The fixture problem here is
different from combat's: ambient emotes need **time in a zone**, not a fight
that survives rounds.

- stand in a cave zone with active weather and collect lines until the pool is
  seen to vary
- confirm no house prose appears underground
- confirm a surface indoor room (house or fort) still reads correctly
- `EmoteEveryRounds` and the two chance knobs can be turned up locally to
  shorten the run

---

## Out of scope

- **Weather driven mechanics** (wet conditions affecting items or players): a
  different arc entirely, owner 2026-08-31.
- **A separate felt threshold for underground.** Underground keeps
  `StrongFeltThreshold = 0.5`. Whether a storm should be fainter through stone
  than through walls is a feel question, filed for the playtest rather than
  guessed at now.
- **Promoting the classification maps to YAML.** Decided against; revisit only
  if builders need to add underground biomes without a code change.
- **A new `Underground` flag on `BiomeInfo`.** Rejected: it edits the shared
  room model to serve one narration consumer. If another system ever wants the
  concept, that is when it is promoted, and these maps are what get promoted.
- **Band unification.** `StrongFeltThreshold` stays per store, per the assembly
  rule, so M4 stays a parameter flip rather than a core edit.

---

## Open questions for the playtest

1. Should underground use a higher felt threshold than indoor, so deep stone
   muffles weather more than walls do?
2. Is depth 6 enough at the shipped emote cadence, or does a long session in
   one zone still feel looped?
3. `spiderweb` has no rooms. If it is ever used, does it want its own pools?
