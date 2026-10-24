# Dialogue Context

## Purpose

`internal/dialogue` is the NPC conversation data model and its rules: it loads a
mob's dialogue YAML (ambient greetings, the opener `talk` says first, and a menu
of keyed topics), decides which topics a player may see, applies a chosen
topic's effects, and validates a proposed world of dialogue files. The `talk`
flow, `give` resolution and the roster marker that call it live in
`internal/conversation`.

It deliberately does **not** import `characters` or `users`. Everything it needs
to know about the player arrives through `PlayerState`, a struct of callbacks.
That keeps the package testable and keeps the import graph acyclic.

Quest arc #375 phase 1 replaced the old keyword patterns and stateful tree
(`Match`, `MatchWithFallbackInfo`, `TreeAdvance`, `Greet`, the old
`ValidateDialogueFile` validator and its registry wiring) with topics. Those
are deleted, and a file still carrying `patterns:`, `tree:` or `memory:` no
longer decodes under the strict loader.

## Files

- **types.go**: `PlayerState`, `QuestFlagSet`, `RepBump`, `Mood`, `Greeting`,
  `DialogueFile`.
- **topic.go**: `Gates`, `Effects`, `Topic`, `Opener`, `OpenerVariant`, the
  kind constants `KindInfo` / `KindOffer` / `KindHandIn`, and `HeardKey`.
- **topics.go**: the `DialogueFile` methods `FindTopic`, `Eligible`,
  `TopicMarker`, `Marker`, `OpenerFor`, `Breadcrumbs`, `Menu`, `ItemTopic`,
  `Apply`.
- **engine.go**: the shared gate and effect helpers `checkQuestGate`,
  `itemExcludedBlocks`, `tryCharge`, `applyPaidEffects`, `applyQuestEffects`.
- **loader.go**: `Load(mobId, zone)` (strict decode, lazy cache) and
  zone-name sanitising.
- **save.go**: `SaveDialogueFile`, `CreateNewDialogueFile` (writes an opener
  skeleton), `DeleteDialogueFile`. No caller outside the tests while the
  builder's dialogue editor is off; the phase 6 topic builder is their
  intended caller.
- **memory.go**: per-(mob instance, player) memory, now only the greet-once
  record.
- **mood.go**: per-mob-instance mood state.
- **greetings.go**: ambient arrival greetings and greet-once tracking.
- **world.go**: `World`, `QuestInfo`, `Problem`, `ValidateWorld`: the topic
  validators over a proposed world (#375 spec 5). `World.BehaviorTopics` holds
  each mob's per-mob tree `topic_is` keys (checked both ways);
  `World.ArchetypeTopics` holds its declared archetype's keys, which also
  serve a `behavior: true` topic but are never a problem on a mob with no
  such topic, since an archetype is shared.
- **world_registry.go**: `LoadAllFiles` (strict decode of every shipped
  file) and `RegistryWorld(behaviorTopics)`, the world from the loaded
  registries. The boot door and the CI sweep
  `TestSmoke_ShippedDialoguePassesTheValidators` both run
  `validateShippedDialogue` in `main_dialogue.go`, which also fills
  `ArchetypeTopics` and adds the file-placement rules only the loaded mob
  templates can judge (dialogue folder equals the mob's zone folder, behavior
  trees read at the runtime's path, unparsable trees reported).
- **manifest.go**: `Manifest`, `ParseManifest`, `Manifest.Diff`,
  `HeardRenames`: the committed topic-key manifest,
  `dialogue/topic-keys.txt`. `topic_manifest_test.go` (internal/guards) holds it
  to content, and `users.LoadUser` passes `HeardRenames()` to
  `Character.RenameHeardTopics` after `MigrateQuestFlags`.
- **validation.go**: `CollectFlagReferences` over topics and opener variants,
  for `questengine.ValidateAllFlags`.
- **walk.go**: `WalkAllFiles`, used by the quest editor's reference guard.
- **testing.go**: `InstallForTest`, for tests in other packages.

## Data model

```go
type DialogueFile struct {
    MobId       int
    Zone        string
    DefaultMood string
    Greetings   []Greeting
    Opener      *Opener   // optional
    Topics      []Topic
}
```

Topics and opener variants (`topic.go`) share one `Gates` struct:
`questRequired`, `questExcluded`, `questFlagRequired`, `questFlagExcluded`,
`requiresItem` (taken when the effects apply), `itemExcluded` (hidden while
carried), `carriesItem` (shown only while the player carries the item anywhere,
`PlayerState.CarriesItem` falling back to `HasItem`, never taken: a reusable
key such as the Threshold Keeper's disc; `ItemTopic` treats it as wanting the
item, so `give` plays the topic and keeps the item), `goldRequired` and
`masterworkRequired`. `Gates.pass` checks them all; a nil `PlayerState` skips
them, and `carriesItem` with no item callback fails closed. `ValidateWorld`
checks every item id they name. `itemExcluded` keeps a quest giver's handout
to one item while the quest is open (2026-10-17).

A topic's `Effects` are `grantsQuest`, `givesItem`, `setsQuestFlag`,
`bumpsRep`, `givesGold`, `chargesGold` and `moodChange`. Opener variants carry
gates only.

```go
func checkQuestGate(questRequired, questExcluded []string, requiresItem int,
    flagRequired, flagExcluded map[string]string, masterworkRequired, goldRequired int,
    ps *PlayerState) bool
func applyQuestEffects(grantsQuest string, requiresItem, givesItem int,
    flagSet *QuestFlagSet, bumpsRep []RepBump, givesGold int, ps *PlayerState) bool
```

`applyQuestEffects` attempts `GiveItem` **before every other effect** and
returns false, applying nothing else, when delivery fails (2026-08-03
soft-lock fix): no quest grant, no `requiresItem` removal, no flags/rep/gold,
and `Apply` does not record the topic as heard, so it plays again once the
player makes room. `PlayerState.GiveItem` is `func(int) bool`; the production
callback in `conversation.PlayerState` returns `StoreItem`'s verdict and
returns false (with an error log) for an invalid item id. A nil `GiveItem`
counts as delivered (skip-checks contract).

`goldRequired` (2026-10-05) hides a topic from a player carrying less than
that much gold (`PlayerState.HasGold`; nil fails it closed). It takes nothing
by itself. `chargesGold` is the first effect `Apply` takes (`tryCharge`,
backed by `PlayerState.ChargeGold`, all or nothing; nil fails closed). A
failed charge makes `Apply` report false with nothing applied. If the paid
topic's effects then abort (an undeliverable item), `applyPaidEffects` gives
the gold back. `ValidateWorld` refuses a `chargesGold` without a
`goldRequired` of at least the same price. Do not sell a quest by charging in
its `quest_granted` trigger: that runs later, outside the step that checked
the gold. `quest_charge_gold_gate_guard_test.go` pins the rule for shipped
content.

Moods: `friendly`, `neutral`, `hostile`, `afraid`, `grateful`. A greeting can
restrict itself to a subset via `moods:`; a topic's `moodChange` shifts the
mood when chosen (the caller does it with `ShiftMood`, since `Apply` has no mob
instance). Mood never gates a topic.

## Public API

```go
func Load(mobId int, zone string) *DialogueFile
func WalkAllFiles(fn func(mobId int, zone string, df *DialogueFile))
func LoadAllFiles() (files []*DialogueFile, problems []Problem)
func RegistryWorld(behaviorTopics map[int][]string) (World, []Problem)
func ValidateWorld(w World) []Problem

func (df *DialogueFile) FindTopic(key string) *Topic
func (df *DialogueFile) Eligible(t *Topic, ps *PlayerState) bool
func (df *DialogueFile) TopicMarker(t *Topic, ps *PlayerState) string
func (df *DialogueFile) Marker(ps *PlayerState) string
func (df *DialogueFile) OpenerFor(ps *PlayerState) (text, hints string)
func (df *DialogueFile) Breadcrumbs(ps *PlayerState) []string
func (df *DialogueFile) Menu(npcName string, ps *PlayerState) menu.Menu
func (df *DialogueFile) ItemTopic(itemId int, ps *PlayerState) (t *Topic, wanted bool)
func (df *DialogueFile) Apply(t *Topic, ps *PlayerState) bool
func HeardKey(mobId int, key string) string

func PickGreeting(gs []Greeting, currentMood Mood) (string, bool)
func HasGreeted(mobInstanceId, userId int) bool
func MarkGreeted(mobInstanceId, userId int)

func GetMood(mobInstanceId int, defaultMood string) Mood
func SetMood(mobInstanceId int, mood Mood)
func ShiftMood(mobInstanceId int, target, defaultMood string)

func GetMemory(mobInstanceId, userId int) *PlayerMemory
func SweepMemories() int
func ForgetMobInstance(mobInstanceId int) int

func CollectFlagReferences(df *DialogueFile) (refs, sets []FlagRef)
func ParseManifest(data []byte) (Manifest, error)
func HeardRenames() map[string]string

func SaveDialogueFile(df DialogueFile) error
func CreateNewDialogueFile(mobId int, zone string) error
func DeleteDialogueFile(mobId int, zone string) error
func InstallForTest(df *DialogueFile) func()
```

## Per-instance memory

```go
type PlayerMemory struct {
    LastVisitRound uint64
    Greeted        bool
}
```

Keyed by `mobInstanceId<<32 | uint32(userId)` in a package-level map. **This is
in-process only**: never written to disk, so every restart resets who has been
greeted. What a player has heard is saved on the character (`HeardTopics`),
never here. `MarkGreeted` stamps `LastVisitRound`, so `SweepMemories` ages an
idle pair out (after `memorySweepIdleRounds`) and the NPC greets afresh.

`hooks.SweepDialogueMemory` runs the sweep every 500 turns. Call
`ForgetMobInstance` when a mob despawns: instance ids are reused, and a new
mob must not inherit a stranger's greeting.

## Gotchas

- **A `nil` `*PlayerState` disables all gating.** Accepted for backward
  compatibility with non-user contexts; every requirement, exclusion, flag
  check and masterwork gate is skipped. Never pass nil from a production
  path: `conversation.PlayerState` always returns a populated one.
- **Every callback field is optional and individually nil-checked.** A missing
  *interrogative* callback fails the gate **closed** (no `HasQuest` means a
  `questRequired` topic is hidden, because absent information is not proof the
  player qualifies). `questExcluded` is the deliberate exception: without
  `HasQuest` we cannot confirm the excluding token, so the topic stays visible
  rather than silently vanishing. A nil `HasHeard` fails `heardRequired`
  closed.
- **The filename must be `<mobId>.yaml`, in the mob's zone folder.** The boot
  door refuses a mismatch (`decodeFile`, `dialogueFolderProblems`).
- **Decoding is strict.** An unknown key, an old-format field or a bare scalar
  where a list is expected fails the file; the boot door and the CI sweep
  report it rather than leaving the NPC mute.
- **Semicolons in spoken text are refused** (`ValidateWorld`): `;` is the
  command separator and truncates a mob's `say`. Hints go out through
  SendText and are exempt.
- **Voice rules are enforced by review, not by code:** `text` is the NPC
  speaking (first person); `hints` is narrator text addressed to the player
  (never a third-person self-reference).
- **Every `grantsQuest` needs the granted token and the quest's end token in
  `questExcluded`**, or the offer repeats or a finished quest is re-offered.
  `ValidateWorld` refuses either omission.

## Dependencies

`util` (round count), `mudlog`, `configs`, `menu` (menu types and
`LintLabels`), `quests` and `items` (for `RegistryWorld` only), plus the YAML
loader. Quest effects are applied through `PlayerState` callbacks, so there is
no compile-time dependency on the quest engine.

## Consumers

`internal/conversation` (talk, give, roster marker, companion asker),
`internal/usercommands` (`go.go` arrival greetings), `internal/questengine`
(`CollectFlagReferences`), `internal/hooks` (memory sweep), `main_dialogue.go`
(boot door), `modules/gmcp` (quest reference guard, `hasDialogue`),
`modules/aicompanion`, and the dev tool `cmd/dialoguecheck`. The one-time
converter `cmd/dialogueconvert` did its job and was deleted in plan Task 24
(git history keeps it). The admin web dialogue editor is off (every
`Build.Dialogue.*` message is refused in `modules/gmcp`) until the topic
builder of quest arc phase 6.
