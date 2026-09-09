# Messaging M3.1 + M3.2: Store Core Extraction, Defence, itemvoices, casting

Design for the next slice of the messaging unification arc
([`2026-08-31-messaging-unification-design.md`](2026-08-31-messaging-unification-design.md)),
covering items 1 and 2 of that spec's M3 order.

Preceded by a standalone taunt bug-fix PR, specified in its own section below.

## Facts verified against source, 2026-09-09

Read from master `65447be86`. Nothing in this document is recalled or inferred.

| Fact | Evidence |
|---|---|
| `internal/narration` imports `internal/util` and nothing else | `narration/picker.go:6` |
| Five store-owning packages already import `narration` | `combat/taunt_messages.go`, `grapplemessaging/render.go`, `items/attack_messages.go`, `items/defensive_messages.go`, `spells/casting_messages.go` |
| **`internal/items` imports `internal/buffs`; buffs does not import items** | `items/itemspec.go`. This disqualifies `items` as the core's home, because M3 item 5 migrates buffs |
| `spells`, `grapplemessaging`, `conversations`, `buffs` import `items` zero times | four store owners are outside items' reach |
| Defence validator demands at least 5 variants, equal lengths across the three roles, no empty strings | `items/defensive_messages.go:67` to `:105`, minimum at `:83` |
| Defence coordinates one index across all three roles | `items/defensive_messages.go:133` `RenderTriad` |
| Defence banding is split and deliberately unresolved | `GetDefenseMessage:171` bands on `zScore`; `RenderDefenseMessage:107` bands on crit plus margin |
| `dodge.yaml` ships 10 variants per role in the weak band, 14 in normal and heavy, 114 lines total | `_datafiles/world/dogmud/defense-messages/dodge.yaml` |
| **`itemvoices` has no picker seam**, calling `util.Rand` directly | `itemvoices/itemvoices.go:73` |
| **`itemvoices` has no golden.** `testdata/stores/` holds six, none of them itemvoices | `narration/testdata/stores/` |
| itemvoices ships 2 voice files, zero tokens, and has exactly one caller | `hooks/pinnacle_tick.go:517` |
| **casting is not in `fileloader`**: bare `os.ReadFile` under a `sync.Once` | `spells/casting_messages.go:32` to `:35` |
| casting has a hardcoded Go fallback that substitutes silently | `spells/casting_messages.go:51` `defaultCastingMessages` |
| casting ships pools of 3, 3, 3 and 4 | `_datafiles/world/dogmud/casting-messages.yaml` |
| Defence and combat-messages **panic** on a validation error | `items/itemspec.go:788`, `:795` |
| **Taunt draws its three viewpoints from three independent picks** | `usercommands/taunt.go:191` to `:193` |
| Taunt pools are authored index-paired | `taunt-messages/rhetoric.yaml`: index 0 is "cut deep" on both sides, index 1 "biting remark" on both |
| Taunt's validator checks required keys only, with no length check | `combat/taunt_messages.go:39` to `:47` |
| **Taunt `hit` and `miss` ship 8 / 8 / 6**, so indices 6 and 7 have no room line. `critical` and `fumble` are 6 / 6 / 6 | `taunt-messages/rhetoric.yaml` |
| **The mob side of taunt never touches the store**, hand-rolling its own literals | `mobcommands/taunt.go:60`, `:62`, `:83`, `:85` |
| `combat-messages` splits observers in two for the `separate` case | `items/attack_messages.go:40` `SeparateMessages{ToAttacker, ToDefender, ToAttackerRoom, ToDefenderRoom}` |
| Skill tiers are a pool **union**, not a selection | `items/attack_messages.go:113` `GetForSkillLevelWith` unions beginner, expert and master by skill band |
| Grapple filters the pool before picking | `grapplemessaging/render.go:36` `PickTemplate(pool, cooldowns, keyPrefix, picker)` |
| buffs, spells and crafting hold **single strings**, not pools | `buffs/buffspec.go:111` to `:115`, `spells/spells.go:66` to `:68`, `crafting/crafting.go:46` to `:47` |
| `messaging.Trio` has exactly three role fields, and an AST guard requires every literal to name all three | `messaging/trio.go`; guard at `messaging_surface_guard_test.go:1545` |
| `.gitattributes` pins `*.go`, `go.mod` and `go.sum` to LF, and **covers no golden** | `.gitattributes` |

### One claim corrected during fact gathering

An earlier draft of this design said defence "ships 114 variants for dodge",
contrasted against casting's 3. 114 is the file's total line count across three
bands and three roles. The real figure is 10 to 14 variants per band per role.
The contrast with casting still holds and is still the argument for the M6
content item, but it is 10 to 14 against 3, not 114 against 3.

## Why the core cannot live in `internal/items`

M2 established that the import graph decides where a seam can live, and that
both obvious homes were impossible until grepped. The same is true here, with
the answer inverted.

`internal/items` looks like the natural home: it already owns the defence store,
the combat-messages store, and the shared token and intensity types. But
`items/itemspec.go` imports `internal/buffs`, so buffs can never import items,
and M3 item 5 migrates the buffs store. `spells`, `grapplemessaging` and
`conversations` do not import items either.

`internal/narration` imports `internal/util` and nothing else. No store's
package can create a cycle with it, and five of the nine store owners already
import it. It is the only home that works for all nine.

## The finding that reshapes M3's order

The arc spec lists nine stores as one kind of thing. Read against source they
are two kinds, and the split does not follow the spec's ordering.

**Kind A, pooled:** defence, itemvoices, casting, taunt, grapple,
combat-messages. These hold `[]string` pools and have a real selection problem.

**Kind B, single string:** buffs, spells, quests, crafting. `StartUserText` is a
`string`, not a pool. These have **no selection problem at all**. Their problem
is that the role set is incomplete, which is exactly what the arc spec means
when it says "the Actee slot appears here".

A core designed only against Kind A will not serve Kind B. The unifying concept
therefore has to be **the role set plus a selector**, with pooling as an
optional property, rather than a pool renderer that roles are bolted onto.

## The core

In `internal/narration`, alongside the existing `Picker`:

```go
// Selector names which variant group an event draws from. It is an opaque
// string on purpose: whether the groups are ORDERED (defence's weak, normal,
// heavy) or merely NAMED (itemvoices' on_equip, casting's cast_started) is a
// property of the function that computes the selector, not of the pool. That
// function is the parameter M4 flips.
type Selector string

// Variants holds one candidate list per role. Any role may be empty.
type Variants struct{ Actor, Actee, Observer, ActeeObserver []string }

// Roles is one event as its audiences are told it.
type Roles struct{ Actor, Actee, Observer, ActeeObserver string }

// Render picks ONE index and applies it to every non-empty role, then
// substitutes tokens. One index across all roles is the whole point: it is
// what makes the room see the same event the participants saw.
func Render(v Variants, tok map[string]string, pick Picker) Roles

// ValidateVariants checks equal lengths across non-empty roles, no empty
// strings, and at least minVariants entries. The minimum is a parameter
// because the stores genuinely differ, and M4 is where that unifies.
func ValidateVariants(v Variants, minVariants int) error
```

### The assembly rule

**The store assembles the pools and computes the selector. The core picks one
index, applies it across roles, and substitutes tokens.**

The core never learns about bands, skill tiers or cooldowns. This is not
minimalism for its own sake; it is what makes two of the four corners in this
design disappear rather than be worked around:

- **Skill tiers are a union, not a choice.** `GetForSkillLevelWith` grows the
  pool as skill rises. Folding tier into `Selector` would convert a union into a
  selection, a behaviour change. Under the assembly rule the store hands over an
  already unioned `[]string` and the core is none the wiser.
- **Grapple filters before picking.** `PickTemplate` drops recently used
  templates. Same resolution: the store filters, then assembles.

It also keeps `bandFromZScore` and its siblings as per-store functions, which is
precisely the parameter the arc defines M4 as flipping. A core that owned
banding would make M4 "edit the core" instead of "flip a parameter".

### The fourth role, and why it exists now

```go
Actor         // the one acting
Actee         // the one acted upon
Observer      // observers where the ACTOR is
ActeeObserver // observers where the ACTEE is, when the actee is elsewhere
```

`combat-messages` splits observers in two for its `separate` case, because in
ranged combat the attacker and the defender are not in the same room. A
three-field `Roles` cannot express that.

In the `together` case `ActeeObserver` is empty, because both participants share
a room and `Observer` already reaches everyone. That empty case is the existing
idiom: `Actee` is already empty for events that have no actee. Nothing in this
slice populates the fourth field.

**`messaging.Trio` keeps three fields this slice.** Adding a fourth would
rewrite the role literals in all 25 files M2 migrated, because its AST guard
requires every literal to name every role, and it would do so for a slot nothing
can populate until `combat-messages` migrates at item 8. This is not an
inconsistency being filed as a follow-up: nothing becomes inconsistent until a
`separate` capable store exists, and the prerequisite is recorded here so item 8
does not rediscover it.

## Per-store migration

```go
// defence. Three roles, banded, pooled. The aliasing is ONE line, and it is
// the line most worth guarding: a swap here inverts every defence message in
// the game.
opts := defenseMessages[dt].Options[band]
narration.Render(narration.Variants{
    Actor:    opts.Together.ToAttacker,  // the attacker ACTS
    Actee:    opts.Together.ToDefender,  // the defender is ACTED UPON
    Observer: opts.Together.ToRoom,
}, tokens, pick)

// itemvoices. One role, pooled, selector is the event. No tokens exist.
narration.Render(narration.Variants{Actor: v.Lines[event]}, nil, pick)

// casting. One role, pooled, selector is the category.
narration.Render(narration.Variants{Actor: cm.pool(category)}, tok, pick)
```

Both existing public surfaces keep their signatures. `RenderDefenseMessage` and
`GetDefenseMessage(...).RenderTriad(...)` continue to return
`DefenseMessageTriad`, built from a `narration.Roles`. `GetCastMessage` and
`VoiceSpec.Line` keep returning a `string`. No caller outside these three
packages changes, and the roughly 175 references to `items.TokenName`,
`items.ItemMessage` and `items.Intensity` are untouched.

## The net

This is Task 0, and it is built before any migration, for the reason M2 proved:
**a net must target what the refactor makes easier to get wrong, not what it
makes visible.** What this refactor makes easier to get wrong is role
misassignment, because three separately named pools become four adjacent fields
of one literal that differ only by role.

1. **Fix the two M2 follow-ups that make `go test ./...` unreliable.** The root
   guards walk from the repo root and skip no dot-directories, so an agent
   worktree under `.claude/worktrees/` presents a second copy of `internal/` and
   reds three tests with nothing wrong in the real tree
   (`contest_floor_guard_test.go:163`, `durable_write_guard_test.go:100`,
   `:180`). Separately, `.gitattributes` does not cover `*.golden`, so
   `defense_messages.golden` is checked out CRLF and
   `TestSnapshotStores/defense_messages` fails on Windows while passing in CI.
   Both are pre-existing and owner-approved, and this slice cannot be trusted
   green until they are fixed.
2. **Build itemvoices' picker seam and golden from pre-migration code.** It has
   neither today. Written after the migration, a golden proves nothing; written
   before, it is the baseline. This is the third slice running where the
   previous slice's net does not reach the next slice's target, and it should
   now be assumed rather than discovered.
3. **Migrate, and require all three goldens to come out byte-identical.**

### The byte-identical rule, and the trap inside it

`defense_messages.golden` keys its rows by the **authored** role name and prints
the rendered text:

```
block|weak|todefender => ...You barely block <ansi fg="user">Attacker</ansi>'s attack!...
```

So a swapped alias would put the attacker's sentence in the `todefender` row and
the diff would show it. **That protection holds only while the migration keeps
emitting the authored key names.** Re-recording the golden under the new role
vocabulary would bake a swap into the new baseline invisibly, which is the same
staleness trap that stalled M2's registry keys when a line was reworded.

**The goldens are not re-recorded in this slice.** Any diff in any of the three
means the refactor changed behaviour and the migration is wrong.

## Casting's validator

The arc spec says casting "gains the validator it has never had". Three facts
make that less simple than it sounds: casting ships pools of 3, 3, 3 and 4 where
defence demands at least 5; casting is not in `fileloader` so there is no
`Validate()` hook to implement; and casting's Go fallback silently substitutes
unauthored text, which a boot-failing validator contradicts.

Rulings:

- **Minimum of 3.** The validator's job is catching a regression, a pool
  emptied or a key renamed, not setting a content quality bar. A minimum of 5
  would not improve a single line of text; it would fail boot until someone
  authored eight, pulling authored content and the SOP's playtest gate into a
  refactor slice.
- **Boot-fail on invalid data**, by panic, matching `items/itemspec.go:788` and
  `:795`. The whole slice then has one failure philosophy instead of two.
- **`defaultCastingMessages` is deleted.** Once a bad file panics, the fallback
  cannot run for a malformed file, and for a missing one it silently serves text
  that exists in no data file. That is the same shape as the project's standing
  rule that balance numbers never come from a Go default. Boot-fail and a silent
  fallback cannot both be the policy.
- Casting has only one role, so `ValidateVariants`' equal-length check is
  vacuous for it. The minimum is the entirety of casting's validation, which is
  why the number was worth deciding rather than defaulting.

**Filed to M6, not fixed here:** casting's pool depth. `cast_started` fires on
every cast and has 3 variants, so a caster sees a repeat every third spell,
against 10 to 14 per pool for defence. That is authored content and belongs in
the content pass.

## The taunt bug, as a standalone PR before this slice

`usercommands/taunt.go:191` to `:193` calls `GetTauntMessage` three times, once
per perspective, each with an independent pick. The pools are authored
index-paired: index 0 is "cut deep" on both the attacker and defender side,
index 1 "biting remark" on both. So the three viewpoints of one taunt describe
three different moments.

This is the melee defence triad bug (PR #112) in a store nobody has touched, and
it is the same shape M1 found seven times: a code path that has the mechanism
right and the narration uncoordinated.

In scope for the standalone PR:

- One coordinated index across the three perspectives.
- The equal-length validator taunt lacks, which defence has had all along and
  which would have caught the data gap below.
- The data gap: `hit` and `miss` ship 8 / 8 / 6, so a coordinated index of 6 or
  7 has no room line. **Fixed in the data**, by authoring two more `toroom`
  variants for each of those bands, rather than tolerated by a modulo that would
  silently pair the wrong room line with the participants' line.

Out of scope, deferred to M3 item 3 where taunt migrates onto the core:
**`mobcommands/taunt.go` never reads the store at all**, hand-rolling its own
`fmt.Sprintf` literals at `:60`, `:62`, `:83` and `:85`. One event, two
narration sources, which is the M2 shape in a place M2 never looked. Routing the
mob side through the authored pools changes what mobs say, so it is authored
content under the SOP and carries the playtest gate.

Authoring four new room lines is itself player-facing content, so **the taunt PR
carries the playtest gate**. The M3 slice proper does not: it changes no
player-facing text, and the byte-identical goldens are the proof of that.

## Risks

- **A swap that the golden cannot see.** The rule above holds for defence, whose
  golden prints all three roles. itemvoices and casting are single-role, so a
  role swap is not expressible for them. The risk is concentrated in exactly one
  line of defence's migration, and it is the line the golden covers.
- **The assembly rule pushes work into stores rather than removing it.** True,
  and accepted: the work being pushed out is per-store by nature (banding, tier
  unions, cooldowns), and pulling it in would make M4 a rewrite.
- **Deleting `defaultCastingMessages` turns a missing file into a dead server.**
  Accepted by the owner, on the grounds that silently serving unauthored text is
  the worse failure and the harder one to notice.
- **`ActeeObserver` ships unused.** It is dead until item 8. The alternative,
  reshaping `Roles` later, would touch every migrated store at the point where
  the most stores exist.

## How we know it worked

- `defense_messages.golden`, `casting_messages.golden` and the new itemvoices
  golden are **byte-identical** before and after the migration.
- `go test ./...` is green **and trustworthy**, which requires net step 1 first.
- A deliberate sabotage of defence's aliasing line, swapping `Actor` and
  `Actee`, turns `defense_messages.golden` red. Per the standing rule, the null
  probe is proven capable of failing before green is trusted, and the sabotage
  is verified to compile.
- Casting boots with valid data, panics with a pool emptied, and no longer has a
  Go fallback for any caller to reach.
- Taunt, on its own PR, narrates one coordinated moment to all three
  perspectives, and no band has a role list shorter than another.
