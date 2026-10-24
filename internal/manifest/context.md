# Manifest Context

## Purpose

`internal/manifest` is the stable-key list format two committed files share:
`_datafiles/world/dogmud/dialogue/topic-keys.txt` (talk topic keys) and
`_datafiles/world/dogmud/quests/step-ids.txt` (quest step ids). Saved characters
hold these keys, so a key may leave content only through a `# retired` line or a
`# renamed` line (`old -> new`).

## Files

- **manifest.go**: `Manifest{Keys, Retired, Renamed}`, `Parse(name, data)` (name
  is the file name errors quote), `Manifest.Diff(content) (missing, unlisted)`.

## Callers

- `dialogue.ParseManifest`, `dialogue.HeardRenames` (topic keys).
- `quests.ParseStepManifest`, `quests.StepRenames` (step ids).
- Root guards `topic_manifest_test.go`, `step_manifest_test.go`.
