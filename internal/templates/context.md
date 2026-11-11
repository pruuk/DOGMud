# Template System Context

## Overview

The `internal/templates` package provides a comprehensive template processing system for the GoMud game engine. It handles text templating with ANSI color support, markdown processing, screen reader compatibility, caching, and multi-source file system integration for dynamic content generation.

## Key Components

### Core Files
- **templates.go**: Complete template processing and management system
- **templatesfunctions.go**: Template function library and utilities
- **templatesfunctions_test.go**: Comprehensive unit tests for template functions
- **layout.go**: Template layout and structure management
- **layout_test.go**: Layout system unit tests
- **name_description.go**: Name and description template utilities
- **testing_support.go**: `SetFSForTest`, a test-only helper (not a `_test.go`
  file, so it ships in the binary but is only ever called from tests, mirroring
  `configs.SetConfigForTest`)

### Key Structures

#### AnsiFlag
```go
type AnsiFlag uint8

const (
    AnsiTagsDefault AnsiFlag = iota // Do not parse tags
    AnsiTagsParse                   // Parse ansi tags before returning
    AnsiTagsStrip                   // Strip out all ansi tags
    AnsiTagsMono                    // Parse tags but strip color info
    AnsiTagsNone    = AnsiTagsDefault
)
```
Controls ANSI tag processing behavior for different output contexts.

#### templateConfig
```go
type templateConfig struct {
    ScreenReader bool     // Screen reader friendly templates
    AnsiFlags    AnsiFlag // ANSI processing configuration
}
```
Per-user template configuration for accessibility and display preferences.

#### cacheEntry
```go
type cacheEntry struct {
    tpl           *template.Template
    ansiPreparsed bool
    modified      time.Time
}
```
Cached template with metadata for efficient reuse and cache invalidation.

### Global State
- **templateCache**: `map[string]cacheEntry` - Compiled template cache
- **templateConfigCache**: `sync.Map` (key: int userId → value: templateConfig) — per-user configuration cache; concurrency-safe, accessed via the `getTemplateConfig(userId)` helper
- **fileSystems**: `[]fs.ReadFileFS` - Registered file systems for template loading
- **forceAnsiFlags**: Global ANSI flag override
- **ansiLock**: Read-write mutex for thread-safe ANSI processing

## Core Functions

### Template Processing
- **Process(fname string, data any, receivingUserId ...int) (string, error)**: Main template processing function
  - Loads and compiles templates with caching
  - Applies user-specific configuration (screen reader, ANSI preferences)
  - Processes markdown content with dividers
  - Handles ANSI tag parsing based on user settings
  - Supports template inheritance and layout systems

### File System Management
- **RegisterFS(f fs.ReadFileFS)**: Registers additional file systems
  - Enables plugin-based template overrides
  - Supports multiple template sources with priority ordering
  - Used for modular template systems and customization

- **Exists(name string) bool**: Checks template existence across file systems
  - Searches registered file systems first (plugins)
  - Falls back to core data files
  - Enables conditional template loading

- **SetFSForTest(t *testing.T, filesystems ...fs.ReadFileFS)** (`testing_support.go`):
  use this in tests instead of `RegisterFS`. `RegisterFS` only appends and has
  no matching unregister, so a call from a test leaks that filesystem into
  every later test sharing the same test binary process — and because
  `readFile`'s zero-registrations behavior is "vacuously succeed with empty
  content" rather than "fail", that leak silently changes what unrelated
  tests render. `SetFSForTest` replaces `fileSystems` for the calling test
  only and restores it via `t.Cleanup`. It is **not safe with parallel tests**
  (`t.Parallel()`): `fileSystems` is an unlocked package-level var, so two
  tests racing `SetFSForTest` step on each other's registrations and
  cleanups.

### Configuration Management
- **SetAnsiFlag(flag AnsiFlag)**: Sets global ANSI processing override
- **ClearTemplateConfigCache(userId int)**: Clears user-specific template cache
  - Used when user preferences change
  - Forces reloading of user-specific template configuration

### Mitigation on the status sheet (#465)
- **effectiveMitigation(char, channel)** (`templatesfunctions.go`): a
  character's mitigation on `"physical"`, `"magical"` or `"conviction"`, bent
  through that channel's curve by `combat.EffectiveMitigationFor`, the value
  combat applies. An unknown channel name fails the render. The DOGMud
  `character/status` template passes it to **defenseQuality** (#479), which
  returns "impregnable" from 85% effective and otherwise defers to
  **mitigationQuality**. `mitigationQuality` maps a fraction to a word band
  ("none" to "fortified", "fortified" from 70%). `descriptions/identify`
  still passes one item's own points to `mitigationQuality`, raw, so
  `identify` bands a single item's raw points (top band "fortified") while
  `status` bands the wearer's effective value: the same words on different
  scales, plus a status-only top band. Both are top-level functions in
  `templatesfunctions.go`, registered in the func map.

### Markdown Integration
- **processMarkdown(in string) string**: Processes markdown content
  - Converts markdown to ANSI-formatted text
  - Adds decorative dividers for visual separation
  - Integrates with markdown package for rich text formatting

## Template Features

### ANSI Color Support
- **Dynamic Processing**: ANSI tags processed based on user preferences
- **Screen Reader Mode**: Strips colors for accessibility
- **Mono Mode**: Preserves formatting without colors
- **Full Color**: Complete ANSI color and formatting support

### Caching System
- **Template Compilation**: Compiled templates cached for performance
- **Cache Invalidation**: File modification time tracking for automatic updates
- **User Configuration**: Per-user configuration caching
- **Thread Safety**: Concurrent access protection with mutexes

### Accessibility Features
- **Screen Reader Support**: Alternative templates for screen reader users
- **ANSI Stripping**: Removes color codes for text-only output
- **Alternative Layouts**: Screen reader optimized template variants
- **Configurable Output**: User-controllable output formatting

### Narrow forms and the reader's width (`forms.go`)
- `Process` renders for the reader's width, `users.UserRecord.WrapWidth()`
  (`FullWidth`, 80, with no reader). The width is read on every call, never
  cached, so a resized screen or a new `set linewidth` takes effect at once.
- The `screenwidth` template function returns that width, capped at
  `FullWidth`, so a template can fit a rule or a wrapped paragraph to its
  reader: `{{ repeat "*" (sub screenwidth 1) }}`,
  `{{ splitstring .X (sub screenwidth 9) "  " }}`. `min` is there for caps:
  `(min 67 (sub screenwidth 12))`.
- `wraptext text width prefix` wraps with `messaging.WrapAnsi`, the game's
  one wrapper, and starts each later line with prefix. It breaks only at
  spaces and reopens a colour tag on each line, where `splitstring`
  (`util.SplitString`) can leave an opening bracket at the end of a line
  ("Missing Person (" / "following)"). Narrow forms use it.
- `visiblewidth s` is the columns s takes, colour tags not counted, so a form
  can test whether a coloured value (a `vitalQuality` word, a `NameComplex`)
  fits beside its label.
- `centerlines text width` centres text on a screen width columns wide:
  whole when it fits with 2 columns clear each side, else one piece to a
  line when every piece fits with 1 column clear, else wrapped with
  `messaging.WrapAnsi` with 2 clear. A piece ends at a sentence end or at a
  run of two or more spaces (`respace`, which steps over colour tags, so a
  per-letter colour pattern such as the wildfire's flame reads by its
  letters); wrapped, a run of spaces is one space. Lines are padded on the
  left only. The room view's alerts use it between two `screenwidth` rules.
- `wrapspans text width prefix` wraps as `wraptext` does but keeps each
  colour span (a name, an exit, a count), with the text glued to it
  ("Name..."), on one line unless that unit is wider than width. A span
  inside another is a unit too (plan 3b-3), so an item named inside a
  coloured mail NOTE stays whole while the NOTE wraps. The room view's
  tracking line, the mail forms and the banners use it.
- `trimspace s` drops the padding around s. A 30 form uses it on a
  localized label (`Inbox.Sent: 'Sent:    '`) to follow it with one space,
  so the padding rule holds in every language without new strings.
- The banners `character/questup`, `skillup` and `statup` (plan 3b-3) have
  no forms: their star rules are `screenwidth` wide (79 at 80 and wider, as
  a desktop always saw them) and their text wraps inside the rules at
  indent 1 with `wrapspans`, so one layout fits every width.
- `mail/message` (plan 3b-3) is the 80 layout, its message and NOTE lines
  wrapped to 80 under their value; its `.w42` and `.w30` forms continue
  that prose 2 past the label and keep the date-time
  (`users.Message.DateUnits`), the sender and an attached item whole.
- `wrapbreak text width prefix` wraps as `wraptext` does but first breaks a
  word longer than width into width-wide pieces. Signs and runes use it,
  since players write them. `visibleRunes` and `rebuild` are the shared
  walk over a string's visible runes behind `respace`, `wrapspans` and
  `wrapbreak`.
- The character screens' forms (`character/status`, `inventory`, `skills`,
  `conditions`, `cooldowns`, `quests`, `storage`, `title`, each `.w42` and
  `.w30`) are drawn to their own width, not to `screenwidth`: a 42 form drawn
  to a 35-column reader would fit and hide the 30 form. Shared layout
  pieces live in `{{ define }}` blocks inside each form file. So are the
  looking-at-things forms (plan 3b-2): `character/description`,
  `description-corpse`, `inventory-look`, `pet`, and `descriptions/biome`,
  `identify`, `insidecontainer`, `track`.
- The room view's own templates (`descriptions/room`, `room-title`, `exits`,
  `fixtures`, `rune`, `sign`) have no forms: `rooms.GetDetails` wraps the
  room's prose to the reader's own width, so a form drawn to 42 would not
  fit a 60-column reader's prose. They draw to `screenwidth` instead, and
  wrap a line only when it is wider than the screen, so a line that fits
  keeps the bytes it always had. An exit and its tag (`northeast (locked)`)
  and the title's PvP tag are units, never split while they fit a line.
- `descriptions/biome` draws its box as two 80-wide rules at every width
  (owner-approved mock, 2026-11-10), with its lighting wrapped under its
  value.
- A `.template` may have narrow forms beside it, named for the width they are
  drawn to: `tables/generic.w40.template`, `tables/generic.w30.template`. Only
  widths in `FormWidths` (42, 40, 30) are read. When the base output's widest
  line (`WidestLine`) is wider than the reader, the forms are rendered widest
  first and the first that fits is used, or the narrowest one found. A base
  that fits is always used. Which forms a template has is found once and
  remembered (`formsOf`); a form file added while the server runs is seen
  after a restart.
- A screen-reader file still comes first and has no forms. A `.md` file has
  no forms.
- `ProcessForWidth(fname, data, screenReader, width)` (forms.go) renders for
  every reader who shares that screen-reader setting and width, picking the
  form by width alone (`formFor`): the base at `FullWidth` and wider, else
  the widest form drawn to no more than width, or the narrowest. It measures
  nothing, so a broadcast renders once per distinct (screen reader, width)
  among its readers (spec ruling 11, plan 3b-3; the auction broadcasts use
  it). `Process` and `ProcessForWidth` share `processPicked`.
- `TemplateTable.RowDetail` gives a row's other cells as `Header: value`
  pairs for the two-line table form.
- `ProcessAtWidth` (testing_support.go) renders at a given width for guards.
- `internal/guards/template_width_guard_test.go` knows every template and why
  it is safe at 30 columns, and renders the ones that must fit at 30, 42, 80.

### Multi-Source Loading
- **Plugin Templates**: Templates from plugin file systems
- **Core Templates**: Built-in game templates
- **Priority System**: Plugin templates override core templates
- **Fallback Mechanism**: Graceful fallback to available templates

## Gotcha: the helpfile guards cover 27 of 454 templates

`u8ActionHelpPaths` (`u8_help_test.go:19-53`) is a **hand-maintained
allowlist**, currently **27** entries. There are **454** files in
`_datafiles/world/dogmud/templates/help/`. Three tests iterate only that
allowlist:

- `TestU8ActionAdmissionHelpTemplatesProcess` — the template exists and parses
- `TestU8ActionAdmissionHelpStatesExactPolicyWithoutTuning` — required phrases
  are present and no raw or worded tuning numbers leak
- `TestU8ActionHelpCrossReferencesResolve` — `help X` cross-references resolve

So **427 help templates get none of those three checks.** The allowlist has to
be edited by hand for every new topic, and anything not added is invisible to
the guards — which is the same failure mode that let `stow` go unreferenced
(see `docs/audits/HELPFILE_AUDIT_2026-08-03.md`). U10d hit it directly: both
help files it edited that sat outside the list were exactly where its surviving
copy defects were found, and `help/ambush` had to be added by hand as the 27th
entry.

**The fix is to invert it:** walk every `help/*.template` and keep a shrinking,
commented *exception* list instead of a growing allowlist. Assigned to **U11**'s
help-registry cleanup; see the "U11 inbox from U10d" section of
`docs/roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`.

Note the guards also depend on `_datafiles/world/dogmud/keywords.yaml`: an
unregistered helpfile never appears in the topic index, however well it parses.

## Dependencies

### Internal Dependencies
- `internal/colorpatterns`: For color pattern application in templates
- `internal/configs`: For accessing configuration file paths
- `internal/fileloader`: For template file loading operations
- `internal/markdown`: For markdown processing and formatting
- `internal/mudlog`: For logging template operations
- `internal/users`: For user preference and configuration access
- `internal/util`: For utility functions and file path operations

### External Dependencies
- `github.com/GoMudEngine/ansitags`: For ANSI tag parsing and processing
- `github.com/mattn/go-runewidth`: For accurate text width calculations
- `gopkg.in/yaml.v2`: For YAML template configuration parsing
- Standard library: `bytes`, `fmt`, `io/fs`, `os`, `strconv`, `strings`, `sync`, `text/template`, `time`

## Usage Patterns

### Basic Template Processing
```go
// Process template with data
result, err := templates.Process("character_sheet", characterData, userId)
if err != nil {
    // Handle template processing error
}

// Process without user-specific settings
result, err := templates.Process("global_message", data)
```

### Plugin Template Registration
```go
// Register plugin file system for templates
templates.RegisterFS(pluginFileSystem)

// Check if template exists
if templates.Exists("custom_template") {
    // Use custom template
}
```

### User Configuration
```go
// Clear user template cache when preferences change
templates.ClearTemplateConfigCache(userId)

// Set global ANSI behavior
templates.SetAnsiFlag(templates.AnsiTagsStrip)
```

## Integration Points

### User Interface
- **Dynamic Content**: Generate dynamic game content from templates
- **User Customization**: Per-user template rendering preferences
- **Accessibility**: Screen reader compatible output generation
- **Rich Formatting**: ANSI color and formatting for enhanced display

### Plugin System
- **Template Overrides**: Plugins can override core templates
- **Custom Templates**: Plugins can add new template types
- **Asset Integration**: Templates can reference plugin assets
- **Modular Design**: Independent template systems per plugin

### Web Interface
- **HTML Generation**: Templates for web interface content
- **Dynamic Pages**: Server-side template rendering for web
- **Asset Integration**: Template-based asset serving
- **Responsive Design**: Template-based responsive layouts

### Game Systems
- **Character Sheets**: Dynamic character information display
- **Room Descriptions**: Rich room and environment descriptions
- **Item Information**: Detailed item descriptions and stats
- **Help System**: Formatted help and documentation content

## Performance Considerations

### Caching Strategy
- **Template Compilation**: Expensive compilation cached for reuse
- **File System Caching**: Reduce file system access through caching
- **User Configuration**: Cache user preferences to avoid repeated lookups
- **Memory Management**: Efficient memory usage for large template sets

### Concurrent Access
- **Thread Safety**: Mutex protection for concurrent template access
- **Read-Write Locks**: Optimized locking for read-heavy workloads
- **Cache Contention**: Minimize lock contention in high-traffic scenarios
- **Scalable Design**: Architecture supports high concurrent usage

### File System Optimization
- **Multi-Source Priority**: Efficient search across multiple file systems
- **Existence Checking**: Fast template existence validation
- **Lazy Loading**: Templates loaded only when needed
- **Modification Tracking**: Efficient cache invalidation based on file changes
