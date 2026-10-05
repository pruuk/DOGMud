# Description

Provide a description of this Pull Request. Link the spec or issue it
implements.

## Changes

Provide a bullet point list of noteworthy changes in this Pull Request:

- Added `x`, `y`, `z`.
- Changed `x`, `y`, `z`, from `this` to `that`
- Modified `settingX` - `valueOld` => `valueNew`

## Checklist

See [CONTRIBUTING](CONTRIBUTING.md) and the
[house rules](../docs/guides/HOUSE_RULES.md).

- [ ] One system or one coherent change, rebased on current `master`
- [ ] `gofmt`, `go vet`, `go build`, `go test ./...` and `golangci-lint` all clean (paste the result below)
- [ ] The server boots to the login prompt
- [ ] New balance numbers are config knobs, with shipped values listed below
- [ ] No raw numbers in player-facing text; text wraps at 80 columns
- [ ] Any outside API use is off by default and opt-in for players
- [ ] New packages have a `context.md`; new docs are in `docs/README.md`
- [ ] My own adversarial review notes are below

## Gate output

## New config keys and commands

## Self-review notes
