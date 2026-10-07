# Gossip Context

## Purpose

`internal/gossip` is the gossip template store: pools of lines a gossiping NPC
can say about a recent world event or a known fact, loaded from
`DataFiles/gossip_templates.yaml`. It owns the text, its validation, and the
random pick. It does not decide WHICH key to use or WHEN to gossip; that
choice stays in `internal/hooks` (`buildGossipLine`, `renderFactGossip`),
which builds a key from an event's type, significance and distance
(`"EventType-Significance[-Local|-Distant]"`) or a fact's id, tag or the
`fact-default` fallback, then asks this package for the pool at that key.

## API

```go
func Load()
func Validate(templates map[string][]string) error
func Pool(key string) []string
func Keys() []string
func Render(pool []string, token, value string) string
func LowerLeadingArticle(s string) string

// test helpers, in test_helpers.go
func SeedForTest(m map[string][]string) func()
func RenderWithForTest(pool []string, token, value string, pick narration.Picker) string
```

## The rules

- **`Load` panics on a bad file; a missing file is an empty store.** A read
  error other than "does not exist", a parse error, or a failed `Validate`
  panics, the same as a bad recipe or spell file. Before this store existed,
  a bad `gossip_templates.yaml` was only logged and gossip went silently
  empty for the life of the process; that failure mode is gone.
- **`Render` draws exactly once, through `narration.DefaultPicker`.** That
  picker routes through `util.Rand`, global engine randomness, so drawing
  more or fewer times than the pre-store code (`util.Rand(len(pool))`, one
  draw) would shift every later random roll in the process. `renderWith` is
  the testable core with an explicit picker, used by the draw-count test and
  by `RenderWithForTest`.
- **Each token appears at most once per line.** `Validate` refuses a pool
  with no lines, a blank line, or a line using `{desc}` or `{description}`
  more than once. The token rule is what keeps `Render`'s single substitution
  matching the pre-store code's `strings.Replace(..., 1)` byte for byte: with
  no repeats, replacing once and replacing every occurrence produce the same
  line.
- **A rumour dropped mid-sentence loses its leading article's capital.**
  When the token follows a lead-in ("I heard {desc}", "Traders brought news
  -- {desc}", anything not ending in `.`, `!`, `?`, `:` or a quote), a value
  starting "A ", "An " or "The " is substituted with that word lowercased
  (`LowerLeadingArticle`), so it reads "I heard a caravan" (#430). No other
  word is touched, since it may be a proper noun. The lines are adjusted
  before the pick, so the one-draw rule above still holds. The hooks-side
  "I heard that ..." fallback uses `LowerLeadingArticle` too.

## Consumers

`internal/hooks`: `buildGossipLine` (world-event gossip) and
`renderFactGossip` (known-fact gossip) are the only callers of `Pool` and
`Render`. `internal/gossip/gossip.go` is registered as a Kind A caller in
`narration_render_callers_guard_test.go`, and `internal/narration`'s
`gossip.golden` freezes every key's rendered lines.

## Dependencies

`configs`, `mudlog`, `narration`, `yaml.v2`, plus stdlib.

## Gotchas

- **A key with no lines never renders.** `Pool` returns nil for an absent
  key, and `Render` on a nil or empty pool returns `""`; the hooks side must
  treat that as "say nothing," not as an error.
- **`gossip.Validate` cannot see whether a key is ever reachable.** It only
  checks the shape of the lines under a key, not whether any
  `worldevents.WorldEventType` maps to that key's prefix in
  `eventTypeKey` (`internal/hooks/MobIdle_HandleIdleMobs.go`). A well-formed
  key can still be dead content; see messaging M6 content ledger row 23.
