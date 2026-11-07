---
name: dogmud-playtesting
description: Use when running the DOGMud playtest harness, writing a goals file, or triaging playtest findings. Covers the external harness location and how to restore it when missing, that local runs need an ephemeral goals file and a --checkout, that a combat fixture must survive several rounds or the run comes back partial, that reports are gitignored so findings must be filed as GitHub issues, never killing the user's server, and fixture, overlay and seat lessons.
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
cleaned up itself.

**Never blanket-kill by process name or port sweep.** The user runs their
own server on this machine. It is not yours to stop, restart, or start a
second one against. A recorded incident established this the hard way: a
name-based kill of every `GoMud.exe` process took down the user's own live
session mid-test, because the assumption about which of two processes was
"stale" was backwards (2026-07-30). The global rule is in
`~/.claude/CLAUDE.md` and `guard.py` (H4) enforces it.

The distinction, stated precisely: kill by PID, identified as yours, never
by name or port sweep that could also catch a server you did not start.
Before killing anything, check `netstat -ano` for the PID that owns the
port in question, and read each candidate server's log for `bind:` errors.
The process that owns the port is the live one; the one logging bind
errors is the redundant one. If a server is already running and the user
might be connected, do not start a second one; ask, or reuse the running
instance.

Never clean up with `Stop-Process -Name dogmud*` or
`taskkill /F /IM dogmud-test.exe`: both are name-based blanket kills.

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
The Rift Chamber (room 5000): `talk sable`, choose the Arena or the Planar
Oasis, then answer the stake prompt; the gold staked is the difficulty
dial. `ask sable ...` no longer works (`ask` only chats with your own
bonded companion). For a single round of combat, admin `mob spawn 205`
(Steppe Wolf) in room 5000. [[project-u12c-2-playtest-findings]]

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

**Your own visit can hide a boot bug.** A room's spawns fire on the first
player visit unless the room is in `main.go`'s `systemNPCAnchorRooms`, so an
admin `goto` or `look` into a room makes a missing NPC appear and the
symptom "goes away". Before calling something a timing artifact, ask what
you did that could have made it disappear, and check it on a clean boot or
on prod. (2026-05-26: Tova never spawned on prod because her anchor still
named her old room.)

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

`playtestrun stop` tears the run down itself (Supervisor.Stop, then a sweep
of containers labelled for that run id) and exits 0 only after confirming no
container remains; if one survives it exits non-zero and names it. It no
longer depends on the `run` process still being alive (#288). If it exits
non-zero, remove what it names with `docker rm -f <id>`. The flags are
`playtestrun stop --checkout <abs> --run <run_id>`.

Docker's address pool runs out when playtest networks leak. The failure
surfaces only as `build_failure` or "exited with status 1"; a manual
`docker network create x` shows the real cause. A normal stop removes the
run's network (compose `down`), but a container you `docker rm -f` by hand
leaves its network behind. Run networks carry the labels
`dogmud.playtest.managed=true` and `dogmud.playtest.run-id=<run_id>`
(`docker network ls --filter label=dogmud.playtest.run-id=<run_id>`);
remove only your own run's. Overlapping heavy Docker work (a `-race` test,
another boot) has also failed a `compose up` with no server log, so poll the
run's sidecar (`playtestrun status`) rather than waiting on a monitor that
may never fire. To wait for readiness, wait on the `run_id` JSON in stdout or
"environment failed" in stderr; a Docker build line contains the word
"error" in a package name, so a bare error grep fires early. The image
build takes about 90 to 100 seconds.

The playtest container publishes only the AI port (55555), on loopback
(`internal/playtestenv/compose.playtest.yml`): there is no web port and no
human telnet port from the host. Every materialized profile account is
flagged `isai`, and joining on the AI port sets it again
(`internal/hooks/PlayerSpawn_HandleJoin.go`), so harness characters earn no
achievements (`UserRecord.AchievementsExcluded`, which also excludes
admins). To test achievements, `quit`, set `isai: false` in the save inside
the container, and log in on 33333 through `docker exec <c> nc localhost
33333` (worked 11-03). The human port gets no `Playtest.Round` beacon, so
sample GMCP vitals there; a sampled "worst round" is an upper bound.

LAN access to the local server needs only a Windows Firewall inbound rule;
the game config itself needs no changes for this (it already binds all
interfaces except the deliberately loopback-only admin port).
[[reference_lan_access_local_server]]

Verifying an ANSI color change must happen over the human telnet port
(33333), not the AI port. The AI port and the mudagent harness strip color
codes, so a harness-driven run cannot confirm color rendering; capture raw
bytes from port 33333 instead. [[reference_verify_ansi_colors_via_telnet_port]]

## Fixtures, overlays and seats

Standing lessons from the per-run memories listed under Sources. Facts
marked "observed" were seen in a run and not re-derived from code.

### Profiles (`tools/playtest/profiles/`)

- A profile fixes the character name; materialization makes only the
  username unique. Two seats on one profile share a name (`mid` is
  "Midroad Scout", `early` is "Early Strider"), so room lines are ambiguous
  and a seat hears the other's actions as a replay of its own. Give seats
  different profiles when that matters.
- `mid`, `m2-actor`, `m2-witness` and `slice-a-infrared` all wield the same
  Iron Longsword (10006). A weapon-name test needs a different weapon from
  `grant_items` or `equip`.
- `veteran` has finished quests 1, 2, 3, 10, 11, 12, 14, 15, 17 and 30 and is
  part way through 4 (its `questprogress`). Tokens cannot move it back, so
  grep a profile's `questprogress` before picking it and use `mid` or
  `early` for those quests.
- `veteran` brings two flesh golems, Rocky and Fleshy, which fight on their
  own and can kill a boss before the seat acts. `dismiss rocky` and
  `dismiss fleshy` to measure the player alone. Its potions carry old
  `crafted_round` stamps and were spoiled when used (observed); grant fresh
  ones.
- As yardsticks: `veteran` is high-end and `mid` is far below early endgame,
  so neither stands for an early-endgame player (observed, #667). `mid` is
  effectively unarmoured, a good "unarmoured reference player" (observed).
  `veteran` overpowers most single-mob fixtures, even with its golems gone.
- `early` has no spellbook at all; grant every spell a seat needs with
  `grant_spells` (Glow is `chrysalis-glow`).
- `admin` wields a Practice Sword (10002). Its high Perception keeps thieves
  from hiding and barely separates targeted from plain search odds, so test
  stealth or search tuning on a low-Perception profile.
- `playtestrun scenario` refuses the `admin` profile
  (`internal/playtestrun/scenario.go`). For an operator, seed a user profile
  and flip its save before that player logs in:
  `MSYS_NO_PATHCONV=1 docker exec <c> sh -c "sed -i 's/^role: user/role: admin/' /app/_datafiles/world/dogmud/users/<id>.yaml"`.
  A single `playtestrun run` may use `admin`, which is the easy way to get
  `server set night`.

### Overlays (`internal/playtestprofiles/overlay.go`)

They apply in this order: `start_room`, `set_mutations`, `grant_spells`,
`grant_skills`, `grant_items`, `equip`, `set_quest_tokens`,
`set_quest_flags`, `set_gold`. Any failure aborts materialization, so the
environment dies at boot with the overlay's error.

- `set_quest_tokens` goes through `GiveQuestToken`. A quest with no progress
  accepts only `N-start` (or `N-end` for a one-step quest), then later steps
  in order, so `["30-end"]` alone kills the env ("could not grant quest
  token"). A token for a quest the character already holds or finished is
  skipped silently, which is why it cannot rewind a profile. Set only the
  walked quest's own tokens; prerequisite quests gate offers, not tokens. A
  full chain plus `start_room` puts a seat on the last step of a long quest
  in one line.
- Tokens grant no items. If a step's hand-in needs the item the offer would
  have handed over, add it with `grant_items`. Write the goal for the step
  the last token leaves the seat ON, not the step after it.
- Token fixtures skip the cross-quest gates a real player walks, so a seat
  can be offered a quest no real player would see at that point. Do not file
  that as a bug without checking a real route.
- `set_quest_flags` is checked by `quests.ValidateFlag`: seeding a flag the
  quest no longer declares kills the env. Check the quest's `flags:` when a
  scenario carries an old-save seat.
- `grant_items` goes through `StoreItem`, which refuses anything past twice
  carry capacity. Build an over-full pack in game (`item spawn` then `get`).
- `grant_skills` writes `Character.Skills`, which `GetSkillLevel` reads
  directly, so a skill-gated seat needs nothing else.
- `set_mutations` is an authored set (no prerequisite or budget check) and
  runs first, so extra-arm slots exist before `equip`.

### Admin commands that help a playtest

There is no `goto` and no `tell`; `say` does not reach the next room, so
seats in different rooms coordinate by clock time, not chat.

- `teleport <room>`; `teleport <name> <room>` moves another player. A
  relogin starts at the last saved room, not the profile's `start_room`.
- `mob spawn <id>`, `item spawn <id>` (the item lands on the floor), and
  `mob heal` to keep a fragile fixture alive between rounds. An admin
  `mob spawn` gives the mob's authored statpool, not the scaled one a paid
  instance entry gives (observed with 9561 and 9562).
- `zap <target>` sets HP and Conviction to 1: a reliable falter or
  finishing fixture.
- `setcondition <id>` or `setcondition <target> <id>`; `command <mob> ...`.
  Both reach only targets in the admin's own room. `command <mob> cast`
  did nothing in a run (observed); a Bandit Caster (285) casts on its own.
- `locate` needs the full name or a wildcard (`locate *scout*`).
- `questtoken N-step` follows the same order rule as `set_quest_tokens`:
  `N-end` on a fresh character does nothing until `N-start` is in.
  `questtoken flag <key> [value]` reads or sets a quest flag.
- `earnmutation [player]` runs one earned-mutation event through the code a
  fight runs; it can deepen the same mutation twice in a row, so check
  `mutations` after each.
- `server set day` and `server set night` jump the clock forward but cannot
  pick a date. To land on a calendar date, set `Timing.RoundsPerDay` in the
  run's throwaway config (shipped 900); `server set Timing.RoundsPerDay`
  live makes the clock jump unpredictably. `server set GamePlay.PVP enabled`
  turns on PvP.
- `skillset <skill> <n>` changes a skill. A cursed item needs `cursed: true`
  in the item YAML inside the container and `reload items`; Spellcasting 4
  or more overrides a curse on remove (`actions.CursedHolds`), so drop the
  admin's Spellcasting first.
- To edit a save, `quit` in game and wait for the disconnect first: the
  server keeps a logged-in player in memory and overwrites the edit.

### Mob and world fixtures

- `hostile: false` does not mean peaceful: read `behavior_archetype`
  (an `ambusher` strikes from the dark). Noncombat archetypes take a fright
  reaction and hold no grudge, so they cannot test one.
- Theft: the Thornwall Highwayman (mob 90, `thief`, rooms 441 and 445) with
  a `veteran` seat at `start_room: 441`, golems dismissed and Glow cast.
  Mob thieves take most of a purse silently (owner ruling, it stays), so
  money seats lose gold and then call a quest blocked: brief them to avoid
  the Outskirts and Underdocks until they have paid, and trace `gold_carry`
  in `Char.Worth` in `events.jsonl`. A failed `steal` on a townsperson has
  escalated to lethal combat and arrest (observed).
- A mob's schedule segment REPLACES its YAML `idlecommands`
  (`NewRound_IdleMobs_schedule.go`), so a scheduled NPC may never run an
  idle command its YAML lists. For ward or caster AI, spawn a Goblin Shaman
  (219) or Bandit Caster (285), both `defensive_caster`.
- Mob idle behaviour runs only while a player is near, so an empty-room
  baseline proves nothing. Mob skill rolls show in `docker logs` (a scout's
  search, for instance); read the logs, not only the seats' screens.
- A spawn room starts its respawn clock when it is next prepared after the
  mob is gone (a round with a player in it, or a player walking into it or
  next door), not at the death (`Room.Prepare`, `HandleRespawns`). A mob
  that died away from its spawn room, or a room nobody stands in, comes back
  later than its YAML rate suggests.
- Small beasts (Ironwind Steppe cat and rat, mob 202 and 200) die in one or
  two rounds even to `mid` bare-handed: use `early`, or `mob heal` each
  round. Hostile mobs near some start rooms (3002, the 3101 area) attack on
  login, the witness included; a seat in combat must `flee` to leave
  (observed).
- Some bosses attack on arrival (The Sentinel, 9552): walk and fight in one
  step, or the seat is dead before its script starts. Auto-attack results
  say little about a boss; script spells, potions and full HP before
  calling a fight too hard.
- Crit against armour: The Sentinel blocks with a shield and takes almost
  no crits; use The Pass-Apex (9541). A harmful spell such as Mind Fog on a
  player makes them retaliate the moment the cast starts.
- Arrest: attack a constable, then `mob spawn 335`. An identified crime
  brings an arrest about 14 game hours later and a hold of about 2
  (observed), so plan crime cases after any timed watches.
- Hidden spawn: Pale Lurker (225) ambushes once found.
- Bloom Wafer (40108) is `drink wafer`; toxicity refuses further doses for
  minutes, so give a Bloom seat few wafers or its own long run.
- The search cooldown blocks an immediate second search; a goal that tests
  a repeat search must wait.

### Light and sight fixtures

- Cave Mouth (4121) is pitch dark; Lake Path Bend (4120) next to it is dim
  in the morning. Outdoor rooms at night may be moonlit, not dark, so the
  Training Yard (201) is no pitch-dark room.
- The Sunstone (20099) is the reliable source of a shapes band; a torch
  lights a room fully.
- Night Sight is condition 128, Heat Sight 129 (lasts about 3 minutes),
  Blinded 3 (three rounds, so re-apply about 2 seconds before the event).
  Night Vision and Heat Sight cannot be cancelled (observed).
- Some rooms show shapes even in daylight (the Eastern Highlands, Pothole
  Coulee's wash and grove): without a worn Hooded Lantern (20097) a seat
  cannot target by name and fights at a large penalty. Have seats
  `equip lantern` first. An admin start at dawn shows NPCs as "a figure"
  and refuses `attack <name>` until daylight.
- `kick` cannot target a shape.
- Light notices print on the actor's NEXT command (observed): to time a
  light change to the round, send a command every round; to test a
  combined cause, send that actor nothing between the two changes.

### Measuring combat

- Combat debug lines in `docker logs`: `DistDamage` (`dmgMean` per swing),
  `CritDamage` (`rawDmg`, `critMult`, `critMean`, `mitigatedDmg`) and
  `SpellDamage` (`rawDmg`, `effMitig`, `rolled`, `defended`, `evaded`,
  `isCrit`, and the `dmg` applied). Compare first rounds of fresh spawns at
  the same swing count.
- Round beacons in `events.jsonl` can give a cast line and the next round's
  warning the same round number; timestamp the stream to place windup
  lines (observed).
- Report the commit the playtestrun supervisor built, not HEAD: the
  checkout can move during a run.

### Seats and subagents

- One subagent per seat worked cleanly; a single helper client works better
  still. The controller starts every mudagent, records each PID, and gives
  seats one script that takes a seat id, a wait and commands, appends to
  that seat's `commands.txt` and prints new events since its own offset.
  Seats share the session scratchpad, so any per-seat file is named after
  the actor id or seats overwrite each other's scripts and drive each
  other's bridges.
- For a one-question live check, drive one seat from a short scratchpad
  script rather than spending an agent.
- A subagent seat is never fully naive: it starts with recent commit
  subjects and the memory index in its context. Weigh early interest
  accordingly; failing to find a thread is still strong evidence.
- Seat goals that say "find X" without a start beside X mostly measure the
  walk. Start the seat beside the NPC when the walk is not the test, and
  check the goal's command assumption against the code (for example,
  `throw` takes a grenade and has no target).
- Seats misreport. Before filing or passing anything, check each claim
  against raw `actors/<id>/bridge/events.jsonl`: a seat's own grep filter
  hides lines, its helper can lose a reply, and the logout meditation
  ("sits down and begins to meditate") has been reported as a death. Read
  topic labels from the dialogue YAML, not the seat's paraphrase.
- A non-menu command closes an open talk menu and runs as a command
  (`internal/menu`), so a number sent after the menu closed reads as "not
  recognized".
- Crafting seats should send nothing until a craft finishes (#696,
  unconfirmed).
- The bridge is `tail -f commands.txt | mudagent > events.jsonl`. mudagent
  takes `--target --user --password`; the credentials are in the run's
  `control/creds.json` under `players[]`. Create empty `commands.txt` and
  `events.jsonl` first.
- `quit` starts a five-round logout meditation that any action cancels,
  and mudagent exits when it ends. To log back in, restart mudagent with
  `>>` on `events.jsonl` and `tail -n 0 -f` on `commands.txt`, or old
  commands replay. A seat can do this itself in the background.
- After every run, seats leave `tail.exe` behind. Find them with
  `Get-CimInstance Win32_Process -Filter "Name='tail.exe'"`, match the
  command line to this run's folder, and stop those by PID. Other runs'
  tails on the box are not yours; leave them.
- Reading events from Python on Windows: `python -I` ignores
  `PYTHONIOENCODING`, so call `sys.stdout.reconfigure(encoding='utf-8')`.
  The prompt starts with a sun or moon glyph (`[☀ HP:` or `[☾ HP:`), so a
  prompt filter must allow a glyph before `HP:`, and a filter that drops
  every line starting with `[` hides more than prompts.
- In Git Bash, `docker exec <c> sed ... /app/...` has its path rewritten;
  use `MSYS_NO_PATHCONV=1 docker exec <c> sh -c '...'`.

### Web client screenshots

The container publishes only the AI port, so for real-client PNGs boot a
local server from a detached worktree with `Playtest.ProfilesManifest:
<scratch manifest>` in the `CONFIG_PATH` override. It materializes the
seeded characters at boot and writes `creds.json` beside the manifest (the
server also rewrites the override file at boot, which is harmless in
scratch). Then run `node tools/webshot/live-shot.js <shots.json>`; it answers
the "Kick them?" prompt a second login gets and closes its own browser. The
web map draws only rooms the character has visited, so walk the area
before a screenshot that needs it.

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

Playtest reports are gitignored. Extract findings after each run, or they
are lost once the report file is cleaned up or the working tree moves on.
[[project-playtest-findings-not-yet-fixed]]
[[project_playtest_findings_2026_08_08]]

**Findings go to GitHub issues, not memory** (owner, 2026-10-05). For each
defect, check it against current master, then search open issues first
(`gh issue list --repo pruuk/DOGMud --search "<words>"`) and add to an
existing one rather than filing a duplicate. File new ones with
`gh issue create --repo pruuk/DOGMud`, labels `bug` and `from-playtest` plus
an `area:*` label; group trivial copy nits from one run into a single
checklist issue. Use `owner-decision` for anything that needs a ruling.

The repo is public. A defect live on master that lets a player duplicate
gold or items, create value from nothing, or bypass a permission does NOT go
in a public issue: tell the owner and use a private security advisory.

A run's harness lessons (traps, fixture and profile facts) go into this
skill once verified against master, and the run gets one line in the
`reference-playtest-harness-lessons` memory with the issue numbers filed;
never a new line in the MEMORY.md index (owner, 2026-11-06).

## Sources

- CLAUDE.md, "AI Testing" (source lines 953-1002): lifted verbatim in
  "Local versus prod invocation," with three em dashes normalized to
  colons per this skill's own punctuation rule.
- The kill-test-servers and never-blanket-kill memories were folded here
  and retired 2026-10-15 once kill-by-PID went global.
- The timing-artifact memory (2026-05-26, Tova's stale anchor room) was
  folded into "Making a run productive" and retired 2026-10-15.
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
  leaves the container running), since superseded by #288 (stop tears the
  run down and confirms).
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
  above as disagreeing with its own underlying source file. The Sable note
  was corrected 2026-11-06: `ask sable` is gone, Sable uses a talk menu.
- Per-run harness lessons, indexed in the `reference-playtest-harness-lessons`
  memory, folded 2026-11-06 into "Fixtures, overlays and seats" and
  "Harness limits" after checking each against master (`overlay.go`,
  `materialize.go`, `scenario.go`, `cmd/playtestrun`, `sweep.go`, the
  profiles, the admin commands). Container-left-up lessons are dropped
  (fixed by #288). Folded: [[project-375-p7-tier5-playtest-harness-lessons-2026-11-04]],
  [[project-667-fight-measurement-lessons-2026-11-04]],
  [[project-667-playtest-harness-lessons-2026-11-03]],
  [[project-375-p7-tier23-playtest-harness-lessons-2026-11-02]],
  [[project-375-p7-tier1-playtest-harness-lessons-2026-11-02]],
  [[project-375-p7-close-playtest-harness-lessons-2026-11-01]],
  [[project-375-phase4-slice3-playtest-harness-lessons-2026-10-28]],
  [[project-375-phase4-slice1-playtest-harness-lessons-2026-10-28]],
  [[project-375-phase3b2-playtest-harness-lessons-2026-10-27]],
  [[project-375-phase3a-playtest-harness-lessons-2026-10-26]],
  [[project-680-playtest-harness-lessons-2026-10-26]],
  [[project-375-phase2a-playtest-harness-lessons-2026-10-24]],
  [[project-541-slice2-playtest-harness-lessons-2026-10-19]],
  [[project-480-playtest-harness-lessons-2026-10-10]],
  [[project-465-playtest-harness-lessons-2026-10-10]],
  [[project-m6-slice1-playtest-harness-lessons-2026-10-10]],
  [[project-sight-wrapup-playtest-harness-lessons-2026-10-09]],
  [[project-sight-followups-playtest-harness-lessons-2026-10-09]] (its
  "each profile's longsword has its own name" is false on master and was
  not folded), [[project-sight-closeout-playtest-harness-lessons-2026-10-08]],
  [[project-lighting-plan6-playtest-harness-lessons-2026-10-07]],
  [[project-sight-gates-5b-playtest-findings-2026-09-30]] (harness gaps
  only; its infravision-not-active-at-login gap is fixed in the profile),
  [[project-sight-gates-5a-playtest-findings-2026-09-29]] (curse fixture
  and leaked networks only), [[project-shadow-follow-playtest-findings-2026-09-30]]
  (harness lessons only), [[project-bauble-corpus-playtest-findings-2026-09-29]]
  (overlapping Docker work only).
- Cited only, no standing harness lesson to fold:
  [[project-m6-slice2-lessons-2026-10-12]] (copy-sweep, YAML and lint
  lessons belong to other skills; its dark-fixture note is folded above as
  "veteran overpowers single-mob fixtures"),
  [[project-lighting-5d-playtest-findings-2026-10-05]] (findings; its
  harness notes duplicate ones folded above),
  [[project-movement-4b-playtest-findings-2026-09-29]] (findings; AI
  companion errands need a model key the harness lacks),
  [[project-bauble-shelf-playtest-findings-2026-09-30]] (findings only).
