---
name: dogmud-playtesting
description: Use when running the DOGMud playtest harness, writing a goals file, or triaging playtest findings. Covers the external harness location and how to restore it when missing, that local runs need an ephemeral goals file and a --checkout, that a combat fixture must survive several rounds or the run comes back partial, that reports are gitignored so findings must be extracted to memory, and the rule never to kill the user's running server.
---

This skill covers running the GoMud playtest harness against DOGMud: which
servers you are allowed to stop, where the harness lives and how to restore
it, how to invoke a run locally versus against prod, what makes a run
actually productive, the harness's own limits, what a playtest is for, and
what to do with the results before the report file is gone.

## Which servers you may stop

Read this first. The two rules below can sound like they conflict; getting
either one wrong is expensive.

**Kill the test servers you started, after each chunk.** Once a chunk of
work that booted a local server for smoke-testing or verification is done,
kill the process before considering the chunk finished. A lingering test
process holds ports and file handles and causes spurious failures on the
next run, including a server a background agent run started and never
cleaned up itself. [[feedback_kill_test_servers]]

**Never blanket-kill by process name or port sweep.** The user runs their
own server on this machine. It is not yours to stop, restart, or start a
second one against. A recorded incident established this the hard way: a
name-based kill of every `GoMud.exe` process took down the user's own live
session mid-test, because the assumption about which of two processes was
"stale" was backwards. The dated narrative stays in the file; the
operational rule from it does not. [[feedback_never_blanket_kill_the_local_server]]

The distinction, stated precisely: kill by PID, identified as yours, never
by name or port sweep that could also catch a server you did not start.
Before killing anything, check `netstat -ano` for the PID that owns the
port in question, and read each candidate server's log for `bind:` errors.
The process that owns the port is the live one; the one logging bind
errors is the redundant one. If a server is already running and the user
might be connected, do not start a second one; ask, or reuse the running
instance.

`feedback_kill_test_servers`'s own example cleanup commands
(`Stop-Process -Name dogmud* -Force`, `taskkill /F /IM dogmud-test.exe`)
are themselves name-based blanket kills. Treat them as superseded by the
PID-based rule above; do not run them literally.

## The harness is external

`/playtest local` has two halves, and only one of them lives in this repo.
The environment (`cmd/playtestrun` + `internal/playtestenv`) is
self-contained Docker and needs no external checkout. The agent adapter,
`mudagent`, is external: it lives in the public repo
`GoMudEngine/GoMud-Module-Playtest-Harness`, expected at
`$GOMUD_HARNESS_DIR`, default `../gomud-playtest-harness`.

It was permanently deleted from this machine during a C-drive cleanup and
restored 2026-08-13. Before concluding the harness is broken, check that it
is actually present: `ls ../gomud-playtest-harness` and
`ls tools/playtest/.run/*/bridge/events.jsonl` (an empty `events.jsonl`
means a run never had an agent attached, not that the harness is missing).
If it is genuinely gone, restore it:

```bash
cd C:/Users/"Calabe Davis"/workspace
git clone --depth 1 https://github.com/GoMudEngine/GoMud-Module-Playtest-Harness.git gomud-playtest-harness
cd gomud-playtest-harness && go build -o mudagent.exe ./cmd/mudagent
```

Cloning this repo is fine even under the project's hard "do not touch
`GoMudEngine/GoMud`" rule: it is a different, public repo, and read-only
cloning of it is exactly what the project's own playtest command
documentation tells you to do. [[reference-playtest-harness-restore]]

## Local versus prod invocation

Lifted verbatim from CLAUDE.md's "AI Testing" section.

## AI Testing
Run autonomous AI testers via the **GoMud playtest harness** (`mudagent` +
`/playtest` driver) against an ephemeral local env (`playtestrun` /
`playtestenv`) or production. The old `/test-mud` + `tools/mud_bridge.py` +
`tools/ai_player.py` stack was retired 2026-06-08 (archived under
`tools/_archive/testing-pre-harness/`).

**Local (0.3c+)** always starts a disposable Docker checkout via `playtestrun`.
It requires `--checkout` and a goals file with `ephemeral:`. It does **not**
use `targets.yaml` for endpoint/creds. Example adversarial SOP:

```text
/playtest local --checkout <abs> bug-finder 2026-08-03-prepush-sweep.yaml
```

Other local examples (goals must already include `ephemeral:`):

- `/playtest local --checkout <abs> feature-tester corpse-looting.yaml`
- `/playtest local --checkout <abs> feel-tester newbie-naive.yaml`

**Prod** is unchanged: `/playtest prod bug-finder` (uses `targets.yaml`; no
`playtestrun`).

Usage: `/playtest <local|prod> …`. See `.claude/commands/playtest.md` and
`internal/playtestrun/context.md` (Human invocation). The driver spawns
`mudagent` (`GOMUD_HARNESS_DIR`, default `../gomud-playtest-harness`) over
`output`/`gmcp`/`status`/`beacon` JSON events. Local mudagent bridge files
live under `tools/playtest/.run/<run_id>/bridge/`.

Overlay (DOGMud-specific): `tools/playtest/`
- `engine-profile.yaml`: DOGMud commands/world/mechanics
- `targets.yaml`: **prod** (and legacy) creds only; not used for local
  endpoint. **Gitignored since 2026-08-08**: an audit found live prod
  credentials committed to the public repo. Copy `targets.example.yaml` to
  `targets.yaml` locally; never commit it, never paste its contents anywhere
- `personalities/` (bug-finder, feature-tester, feel-tester)
- `goals/` (session objectives; local needs `ephemeral:`), `profiles/`,
  `report-templates/`, `reports/` (gitignored)

The vendored `playtest` server module (`modules/playtest/`) emits per-round
`Playtest.Round` GMCP **beacons** (`hp/sp/cp + max`, room) when enabled via
`Modules.playtest.*`.

**Multi-agent (0.3d+):** shared ephemeral env via `playtestrun scenario` /
`/playtest-scenario --checkout <abs> <scenario.yaml>`. Concurrent mudagents,
per-actor bridges under `.run/<run_id>/actors/<id>/bridge/`, file blackboard
(no ptorch). Use multiple single-agent `playtestrun run`s when agents do not
need a shared world. See `internal/playtestrun/context.md` and
`.claude/commands/playtest-scenario.md`.

## Making a run productive

A combat fixture must be able to survive several rounds, or the run comes
back with a partial result. The most reliable one on record is Sable, in
Rift Chamber 5000: `ask sable arena|oasis <gold>`, where the gold amount is
the difficulty dial. [[project-u12c-2-playtest-findings]]

Thornwall's merchants run on NPC schedules and genuinely go to bed, so a
shopping-flow playtest can be blocked entirely by game time of day; the
symptom (a closed shop, a sleeping NPC one room away) reads like a content
bug unless the full path has actually been walked. Two ways around it:
shout or make noise near a sleeping merchant to wake them (a documented
sleep-wake trigger), or seed the run's `ephemeral.profile` with an
already-stocked character instead of `creation_flow: true`, which hands a
fresh character 250g and an empty pack, the right fixture for onboarding
and the wrong one for anything that needs to buy goods.
[[reference-thornwall-shops-sleep-plan-playtests-for-daytime]]

To drain stamina deliberately, for example to test resource-depletion
behavior, encumber the character with item 12. Crafting needs a station;
use the `crafter` profile rather than trying to craft from an arbitrary
starting room. (Per MEMORY.md.)

## Harness limits

The AI port caps input at `Network.AICommandsPerRound` (shipped at 3)
commands per round. MEMORY.md's index describes the overflow as silently
dropped after being echoed back. The source file behind that fact
describes the AI-rate-limit drop differently, as visible (a "Command
dropped -- AI rate limit" line), and reserves "silent" for a separate,
already-fixed bug where multiple command lines arriving in one TCP segment
used to be concatenated into a single garbled command. Both descriptions
are recorded here rather than adjudicated. Either way, pace batched sends
at roughly one command per append, a couple of seconds apart.
[[reference-multiline-input-concatenated]]

`playtestrun stop` exits 0 but does not actually tear down the container.
Confirm the container is gone and, if not, remove it directly with
`docker rm -f`. [[project-u7b-recheck-findings]]

LAN access to the local server needs only a Windows Firewall inbound rule;
the game config itself needs no changes for this (it already binds all
interfaces except the deliberately loopback-only admin port).
[[reference_lan_access_local_server]]

Verifying an ANSI color change must happen over the human telnet port
(33333), not the AI port. The AI port and the mudagent harness strip color
codes, so a harness-driven run cannot confirm color rendering; capture raw
bytes from port 33333 instead. [[reference_verify_ansi_colors_via_telnet_port]]

## What a playtest is for

Any plan or task that authors player-facing content (rooms, mobs, items,
quests, dialogue, tutorials, onboarding, room prose) must end with an
in-game adversarial playtest-harness review before the work is handed to
the user to playtest. This is a required final task on every content plan,
not an optional extra. [[feedback_content_adversarial_playtest_gate_sop]]

Boot-clean and "YAML parses" verify the system, never the experience.
Content defects (instructions buried in room `description:` prose,
confusing or double-rendered prompts, broken or mis-ordered gates,
dead-ends, awkward pacing, wrong NPC voice) are invisible to a boot test
and to code reasoning. They surface only when something plays the content
as a confused human would; verify the actual player experience in a
client, not just that the server booted. [[feedback_verify_human_experience_not_just_boot]]

For onboarding and newbie content specifically, run the verification as a
true naive new player: spawn a fresh character and give the feel-tester
minimal, coaching-free instructions, no route hints, no command lists, no
spoilers, and no admin commands or teleports. A boring or lost session is
still a valid, valuable result. [[feedback_naive_newbie_playtest]]

AI playtesters (bug-finder, feature-tester, feel-tester roles) are
observation-only. They report findings; they never edit code, run git
commands, or start servers, even for an "obvious" fix. A rogue in-session
fix skips the project's spec, review, and pipeline entirely, and tends to
be incomplete besides. [[feedback_testers_observe_only]]

For newbie-area difficulty and balance specifically, defer tuning to a
single pass after the whole area is built, rather than agonizing over
per-spoke difficulty while building. Verify mechanics thoroughly as you go
(quests complete, rewards and triggers fire, clean boot); flag tuning
suspicions as notes for the batch pass instead of blocking on them.
[[feedback_defer_tuning_to_post_build_playtest]]

## After a run

Playtest reports are gitignored. Extract findings to memory after each
run, or they are lost once the report file is cleaned up or the working
tree moves on. [[project-playtest-findings-not-yet-fixed]]
[[project_playtest_findings_2026_08_08]]

## Sources

- CLAUDE.md, "AI Testing" (source lines 953-1002): lifted verbatim in
  "Local versus prod invocation," with three em dashes normalized to
  colons per this skill's own punctuation rule.
- [[feedback_kill_test_servers]]: rule folded (kill the test servers you
  started, after each chunk); its 2026-05-06 incident narrative and its
  own name-based cleanup commands are not restated as guidance here.
- [[feedback_never_blanket_kill_the_local_server]]: rule folded (never
  blanket-kill by process name; kill by PID, identified via the owning
  port and bind-error logs); its 2026-07-30 dated narrative stays in the
  file.
- [[reference-playtest-harness-restore]]: rule folded (the harness is
  external, was deleted once, check before assuming it is broken, restore
  steps); the file's architecture notes and its three session-cost traps
  are not restated here.
- [[reference-thornwall-shops-sleep-plan-playtests-for-daytime]]: rule
  folded (merchants sleep on schedule and can block a shopping playtest,
  with two workarounds); the file's separate "do not re-report the
  missing apothecary" correction narrative is not restated here.
- [[reference-multiline-input-concatenated]]: cited, not folded. Its
  narrative (the 2026-08-14 fix, PR #40) stays in the file; only its
  distinction between the fixed silent-concatenation bug and the
  separate, still-live AI-rate-limit drop is used above, to flag a
  disagreement with MEMORY.md's index wording rather than resolve it.
- [[project-u7b-recheck-findings]]: rule folded (`playtestrun stop`
  leaves the container running).
- [[reference_lan_access_local_server]]: rule folded (LAN access needs a
  firewall rule, not a config change).
- [[reference_verify_ansi_colors_via_telnet_port]]: rule folded (verify
  ANSI color over the human telnet port 33333, not the AI port).
- [[feedback_content_adversarial_playtest_gate_sop]]: rule folded
  (content plans require a final adversarial in-game playtest gate).
- [[feedback_verify_human_experience_not_just_boot]]: rule folded
  (boot-clean proves the system, not the experience); its 2026-07-17
  dated narrative stays in the file.
- [[feedback_naive_newbie_playtest]]: rule folded (coaching-free naive
  newbie method for onboarding content).
- [[feedback_testers_observe_only]]: rule folded (AI testers never edit
  code); its 2026-04-30 dated narrative stays in the file.
- [[feedback_defer_tuning_to_post_build_playtest]]: rule folded (defer
  newbie-area balance tuning to one post-build pass).
- [[project-playtest-findings-not-yet-fixed]] and
  [[project_playtest_findings_2026_08_08]]: rule folded (playtest reports
  are gitignored, so findings must be extracted to memory).
- MEMORY.md index: source for the combat-fixture note (Sable, Rift
  Chamber 5000), the encumber-for-stamina note (item 12), the
  crafting-needs-a-station note, and the AI-port-cap wording flagged
  above as disagreeing with its own underlying source file.
