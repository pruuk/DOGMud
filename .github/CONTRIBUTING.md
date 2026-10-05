# Contributing to DOGMud

Thank you for building on Delusions of Grandeur. This page covers how work
gets from your fork into `pruuk/DOGMud`. The rules for what the code and text
must look like live in [`docs/guides/HOUSE_RULES.md`](../docs/guides/HOUSE_RULES.md).
Read both before you start a new system.

DOGMud is a fork of [GoMud](https://github.com/GoMudEngine/GoMud). Open
DOGMud pull requests against `pruuk/DOGMud`, never against
`GoMudEngine/GoMud`. The `gh` CLI defaults to the upstream parent, so pass
`--repo pruuk/DOGMud` on every `gh pr` command.

## The short version

1. Talk about a new system before you build it (a one-page spec).
2. One system per pull request.
3. The pull request is green before you open it.
4. Anything that calls an outside API ships switched off.
5. Run your own review first and attach the notes.

## 1. Spec before code for a new system

A new system is anything that adds a package, a player command family, a
persistent store, a new kind of room or item behaviour, or a call to an
outside service. Before writing it, open a GitHub issue or a draft PR holding
a one-page spec. [`docs/guides/SPEC_TEMPLATE.md`](../docs/guides/SPEC_TEMPLATE.md)
has a template to copy and a worked example. It covers:

- What the player sees and does, in a few sentences.
- What existing systems it touches, and which existing mechanisms it reuses
  (see "Reuse before you build" in the house rules).
- What it persists, and where.
- Any new gold or item sources and sinks.
- Any outside API use, and what player data it would send.

The owner answers with yes, no, or changes. This is quick, and it is much
cheaper than reviewing 15,000 lines built in a direction the game is not
taking. Bug fixes, content additions inside an existing system, and small
improvements do not need a spec.

## 2. One system per pull request

- A pull request carries one system or one coherent change. Housing, rifts
  and crafting are three pull requests, even when one builds on another;
  stack them and say which one comes first.
- Aim for a diff a reviewer can hold in their head. As a guide, keep Go
  changes under roughly 5,000 lines per pull request. Generated content
  (blank unit rooms, item YAML) does not count toward that, but say how it was
  generated and include the generator.
- Rebase onto current `master` before opening, and keep the branch rebased
  while it is in review.
- A system that needs a hook in shared engine code (`internal/rooms`,
  `internal/actions`, `internal/usercommands`, `world.go`) should land that
  hook in its own small pull request first, or call it out at the top of the
  description.

## 3. Green before you open

Run the full local gate from the repo root and paste the result into the
pull request description:

```
gofmt -l internal/ modules/ *.go        # must print nothing
go vet ./...
go build ./...
go test ./... -count=1                  # every package, including the repo root
golangci-lint run --new-from-merge-base=origin/master   # must report 0 issues
```

Then boot the server once and confirm it reaches the login prompt with no
panic. CI runs the same checks (`.github/workflows/validate.yml` and
`run-tests.yml`); a pull request with red checks is not ready for review.

The repo root holds about 34 guard tests (`*_guard_test.go`) that enforce
project rules, such as: every item holder is reachable by the bauble sweep,
every narration site has a viewpoint verdict, shops refuse to trade in the
dark. `internal/templates` checks that help text shows no raw tuning numbers.
When one fails, it names the rule and the fix. Fix the
code or register the new site as its message says; do not delete or loosen
the guard.

## 4. Outside APIs ship switched off

DOGMud can call a language model through `internal/apiframework`. Every
feature that does, or that sends any player text or data anywhere, follows
these rules:

- Off by default, in both the Go default and the shipped
  `_datafiles/config.yaml`. The server operator turns it on.
- Players opt in. Any consent box a player sees starts unticked, and consent
  given for one feature never carries over to a new one. Keys saved before a
  feature existed are not opted in to it.
- Model output is untrusted input. It never reaches `mob.Command`,
  `user.Command`, a file path, or a template without passing the existing
  allowlist for player-key text (`baubles.CheckPlayerKeyText`). A reply
  that comes through a player's own key is controlled by that player.
- Text one player can influence never becomes world text other players see
  (room descriptions, cached details, saved rooms) unless it passes the
  same checks and is attributable.
- Spend is bounded and survives a restart.

## 5. Review your own work first

Before opening, review the diff adversarially, by hand or with your own
tools, against the house rules: look for gold or item duplication, permission
bypasses, input that reaches commands or files, places where you rebuilt
something the engine already has, and hardcoded balance numbers. Put the
notes, and what you fixed, in the description. Our review then checks yours
rather than starting from nothing, and your pull request moves faster.

## Writing the pull request

The template asks for a description, the changes, and a checklist. Also
include:

- Which spec or issue it implements.
- New config keys with their shipped values.
- New commands players can type.
- Anything you are unsure of. Saying so is welcome.

## If you work with Claude Code

This repo ships the project instructions in `CLAUDE.md` and detailed
procedures in `.claude/skills/` (balance config, persistence, player copy,
content authoring, tests, shipping and more). They load automatically when
you work in a checkout of this repo, and they are the same rules our reviewers
apply. Keep your fork current so you get the latest ones.

## Reviews

Review comments say whether they block: a comment prefixed `SUGGESTION:` or
`CONSIDER:` does not. Reviews are direct and respectful in both directions.

## Getting help

Ask in the pull request or an issue, or reach the owner directly.
