---
name: dogmud-writing-tests
description: Use when writing, fixing, or debugging a Go test in DOGMud, especially one that is flaky or passing for the wrong reason. Covers the fact that test binaries load Go config defaults rather than the shipped config.yaml, the shared-binary CWD trap, the 2.3% attack fumble rate that makes combat assertions flaky, and the rule that a null probe must be proven capable of failing before a green run means anything.
---

This skill covers writing, fixing, and debugging Go tests in DOGMud: the
discipline for trusting a passing test, the ways a test binary's environment
differs from the shipped game, known sources of combat-test flakiness, stat
and progression fixture setup, and how to confirm an effect actually
happened rather than trusting that an attempt was made.

## A null probe must be proven capable of failing

Read this section first: it is the discipline that catches everything else
below, and it is the most-skipped rule on this project.

A verification that cannot fail proves nothing when it passes. Before
trusting a test (a new test, or an existing test after a fix), sabotage the
code it is supposed to catch, rerun it, and confirm two things: the test
goes red, and the failure output names the specific behavior or line under
test, not something unrelated. Only then restore the code and confirm green
again. This is not ceremony around the real work, it is the only way to know
the assertion is wired to what you think it is wired to.

When two branches you are comparing are byte-identical (there is nothing to
diff structurally), sabotage BY LINE NUMBER: pick the exact line the fix
touches, break only that line, and confirm the failure names that line, not
a neighboring one.

The project has recorded three false passes from skipping this discipline
(noted in the project memory index's current-status log, not a single topic
file). [[reference-test-binary-config-defaults-differ-from-shipped]]
documents one concrete case of the same failure mode from a different angle: a new
`TestCalcSwingCount_FistSwingsAtFistSkill` draft passed against unfixed code
because the test binary's `SkillWeight` default (2.0) happened to compress
two different skill levels to the same swing count. Confirming red is not
ceremony there either; a config default is one of the quietest ways a test
passes for the wrong reason.

## Test binaries never load config.yaml

[[reference-test-binary-config-defaults-differ-from-shipped]]: a test binary
never reads `_datafiles/config.yaml`. Every balance knob arrives at its Go
default instead, and several knobs differ sharply from what ships. The
recorded example is `Balance.SkillWeight`: Go default 2.0 against a shipped
5.0, a 2.5x gap on the skill term of the swing-count formula.

Any assertion whose subject is a value `config.yaml` can move must pin that
knob explicitly in the test (the source names a `pinSkillWeightForTest(t,
5.0)` helper) rather than relying on the ambient default. Do not assume a
test that passes tells you anything about the number the game actually
ships with; go read `config.yaml` for that number and pin it if the test
depends on it.

## CWD is not the package dir

Per the project memory index: a test binary's working directory is not
reliably the package directory. DOGMud builds one shared test binary, so a
relative file path passes or fails depending on which order tests run in,
not on which package they live in. Anchor any file path a test needs on
`runtime.Caller(0)` (the test file's own path) rather than assuming the
process CWD is the package under test.

## Known flake sources

[[project-flaky-tests-blocking-ci]] and the project memory index both record
that roughly 2.3% of attacks fumble and always miss. The fumble check runs
against the attacker's own roll distribution, so it fires at that rate
regardless of how outclassed the defender is, and a fumble always misses
regardless of stats. This is documented as deliberate, self-relative
behavior that a chunk explicitly declined to change, not a bug to route
around by altering the mechanic. A naive combat assertion like "one loaded
shot must land" will flake at roughly that rate; the recorded fix pattern is
a bounded retry (fire, and if it missed, reload and fire again up to a small
cap) rather than tuning the underlying roll.

## Fixture setup

Per the project memory index: when a test needs to set a character's stat
directly, set `.Base` and then call `Recalculate()`; do not assume writing
a derived value is enough.

Progression banners in test output read `SKILL ADVANCEMENT` for a skill
and `STATISTIC INCREASED` for a stat. Grepping for `SKILL INCREASED`
undercounts and will make a passing test look like it failed to fire, when
the real string was never matched.

## Verify the thing actually happened

Two recorded traps share the same shape: an attempt is not proof of an
effect. Confirm the effect, not the attempt.

[[feedback_verify_test_server_bound_its_port]]: a local test server that
fails to bind does not crash, it stays alive and every connection silently
lands on whatever process already owned that port, usually the developer's
own running server. A "clean" result from a test server can be measuring
the wrong process entirely. Pick ports the real server does not use, and
grep the boot log for the actual "Starting http server port=..." line (and
for `bind:` errors) before trusting any measurement taken against it.

[[feedback_verify_served_not_just_written]]: a clean boot proves the server
started, not that a web change reached the browser. A boot-test worktree is
populated from `git diff`, which only carries tracked files, so a brand-new
file that was never `git add`ed silently never arrives and the boot log
stays spotless anyway. After a web change, fetch the rendered page from a
running instance and check both that the new asset is referenced in the
HTML and that its own URL returns 200, rather than trusting that the file
exists on disk.

## Silent no-ops

Two recorded traps produce no error anywhere, so a test that does not
specifically probe for them will pass while the real feature is dead.

[[feedback_yaml_unexported_field_tag_noop]]: a `yaml:` tag on an unexported
Go struct field compiles cleanly and unmarshals to nothing. Go's
reflection-based unmarshallers cannot set unexported fields and skip them
without diagnostics. The recorded incident: lowercasing a tagged field
during a refactor silently severed `hostile:` for every mob YAML in the
game for two months, undetected, because nothing failed. Guard any
load-bearing YAML field with a round-trip unit test: literal YAML in,
assert the post-`Validate()` effect out, not just that parsing did not
error.

[[feedback_validator_conditional_checks]]: a YAML validator that only checks
field X when field Y is set leaves the "Y unset" branch free to pass a zero
default that crashes downstream. The recorded incident: a buff validator
only enforced `RoundInterval >= 1` inside an `if TriggerRate != ""` block,
so a flag-only buff with no `triggerrate:` passed validation at
`RoundInterval = 0` and divided by zero the first time it was actually
ticked in combat. A test of the validator alone is not enough; also test
(or add a defensive guard at) the consumer that divides, multiplies, or
otherwise depends on the field the validator only conditionally checks.

## The guide

`docs/guides/TESTING_GUIDE.md` (235 lines) is accurate for what it covers
and does not overlap with anything above. It is the authoritative source
for running the suite: the Docker Linux-container race baseline (`docker
compose -f compose.test.yml run --build --rm test`), how that compares to
Ubuntu PR CI, failure-category triage (daemon down, image build failure,
generation failure, timeout, race report), the reproducibility boundary
(Go version pinned, base image is not), and the full `playtestrun` /
`playtestenv` harness surface (single-agent, multi-agent scenario, synthetic
profiles, lease/renew/reap, opt-in Docker integration tests). None of that
is restated here. This skill covers a different problem: what makes an
individual Go test correct, or flaky, once you are writing or debugging one,
which the guide does not address at all. Nothing in the guide is stale or
contradicted by the sources folded above.

## Sources

Folded: [[reference-test-binary-config-defaults-differ-from-shipped]] ·
[[project-flaky-tests-blocking-ci]] ·
[[feedback_verify_test_server_bound_its_port]] ·
[[feedback_verify_served_not_just_written]] ·
[[feedback_yaml_unexported_field_tag_noop]] ·
[[feedback_validator_conditional_checks]] · the project memory index's
current-status and feedback sections (null-probe discipline, CWD/
`runtime.Caller`, stat `.Base`/`Recalculate()`, and the two progression
banner strings, none of which have a separate topic file).
