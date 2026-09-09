# Item Voices Context

## Purpose

`internal/itemvoices` gives an item a personality: a named pool of lines keyed
by event, so a sentient or storied item can speak when it is wielded, when it
strikes, when its owner is hurt, and so on.

It is a small authored-data package — a registry of `VoiceSpec` files and one
lookup that returns a random line for an event.

## API

```go
type VoiceSpec struct { /* VoiceId + event → lines */ }

func LoadDataFiles()
func GetVoice(id string) *VoiceSpec
func AllVoiceIds() []string
func SeedVoicesForTest(m map[string]*VoiceSpec) func()

func (v *VoiceSpec) Id() string
func (v *VoiceSpec) Filepath() string
func (v *VoiceSpec) Validate() error
func (v *VoiceSpec) Line(event string) string
func (v *VoiceSpec) LineWith(pick narration.Picker, event string) string
```

An item references a voice by id; `Line(event)` picks one of the authored lines
for that event. `Line` delegates to `LineWith(nil, event)`, and a nil picker
means production behaviour, so the two never diverge.

`LineWith` exists so this store can be snapshotted. Until 2026-09-09 `Line`
called `util.Rand` directly, which made its output impossible to pin and left
itemvoices as the one message store with no golden.

## Gotchas

- **`Line` returns an empty string for an unknown event or an empty pool.** The
  caller must treat empty as "say nothing" rather than sending a blank line.
- **`GetVoice` returns nil for an unknown id** — a typo in an item YAML gives a
  silent, voiceless item rather than a load error.
- **Voices are shared, not per-instance.** Two copies of the same item draw
  from the same pool and have no memory of what the other said.
- **Lines are player-facing text**: 80-column wrapping and no raw numbers apply.
- **This is the single-role degenerate case of the narration core.** `LineWith`
  renders through `narration.Render` with only the `Actor` role populated: one
  role, no band, no tokens. It uses the same seam as the coordinated defence
  triad, which is the evidence that roles and bands are genuinely optional
  there rather than something a single-role store works around.

## Dependencies

`util`, `mudlog`, `narration`, `items`, and the fileloader.

## Consumers

`internal/items` and the combat/equip messaging paths.
