# U11 owner punchlist

**Date:** 2026-08-30
**Branch:** `feature/u11-arc-closer` (18 commits, not yet merged)
**Companion to:** [`PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`](PRE_DEPLOY_PLAYTEST_CRIBSHEET.md)
**Findings detail:** [`audits/2026-08-30-u11-filed-findings.md`](audits/2026-08-30-u11-filed-findings.md)

Everything U11 changed that **was not verified**, plus the decisions only you can
make. Verified items are deliberately absent; they are listed at the bottom so
you do not re-check them.

Two AI playtests ran (veteran `881a07a678dcbe39`, newbie `9828eedc9fdaf7db`).

> ⚠️ **Read this before trusting either run.** Both goals files were given
> `start_room: 5227` (the Drill Yard), so **both agents played starter-area
> content** — the veteran included, whose own profile home is 5455 Kilnreach
> Works. A veteran fighting newbie mobs proves very little about a veteran.
> That is a fixture mistake in how the runs were set up, not a property of the
> game, and it is why several rows below are UNVERIFIED rather than failed.

---

## A. Blocked by the known behaviour gap

These need a mob that can actually knock a player down. Per your note, this is
already owned by the scheduled **behaviour unification arc**, so nothing here is
a new bug — but U11's headline change cannot be judged from the player's seat
until it lands.

Verified independently: `grep -rln "specialmovechance\|movepreferences"
_datafiles/world/dogmud/mobs/` returns **10 files, 8 of them under
`mobs/test*/`**. The two live ones are in `labyrinth_of_low_tunnels`. Zero of
the 70 Pothole Coulee mobs set either.

| # | Check | Why it matters |
|---|---|---|
| A1 | **Get knocked down and fail to rise.** Wait; do not type `stand`. | The whole point of the slice, from the receiving end. Never observed. |
| A2 | **`stand` while held down.** Must work instantly, every time, at the usual cost. | It is the designed escape hatch. If it is ever refused while merely prone, that is serious. |
| A3 | **Nobody is pinned forever.** Count consecutive failed rises against something strong. | Tuning. See D1. |
| A4 | **Free stand still free:** be knocked down with nothing attacking you. | Must succeed on its own. A regression here would be badly felt. |
| A5 | **Progression from scrambling.** Watch for `SKILL ADVANCEMENT` after failed stands. | Unarmed combat should now earn from this; it never did before. Banners read `SKILL ADVANCEMENT` / `STATISTIC INCREASED`. |

**Fastest route if you want these today:** the two `labyrinth_of_low_tunnels`
warren mobs (73, 75) are the only live things that set the field. Otherwise a
purpose-built fixture.

---

## B. Changed AFTER the playtests ran — nobody has seen these in game

| # | Check | What changed |
|---|---|---|
| B1 | **`help` index: no heading holds a single entry.** | `discord` moved from `Integration` to `Communication`. The newbie run flagged `Integration` as a stranded one-entry heading; this fix came after. |
| B2 | **`help defense` reads cleanly, no orphaned fragments.** | I split four over-80-column lines and left `on dodge` stranded on its own line. Re-flowed after the run. |
| B3 | **Companion assist announces ONCE.** Let something attack your companion. | The runs saw `prepares to fight X.` in two consecutive rounds. Fixed with a per-round claim; not re-playtested. |
| B4 | **`set` points at `help set-wimpy`, and it opens.** | The pointer was deleted, then restored, after the run. |

---

## C. Cannot be verified on this machine

| # | Item | Why |
|---|---|---|
| C1 | **`go test -race`** never ran. | Needs cgo; `gcc` is not installed here. One reviewer traced the concurrency question by hand and found no new race, but it is not machine-verified. Worth one run wherever you have a toolchain. |
| C2 | **Logout releases engagement.** | New Presence observer. No playtest reached link-death or the AFK timeout. Matters because a `UserId` is stable across sessions, so a leftover entry resolves to the *new* character on relogin. |
| C3 | **`character new` can respawn after dying.** | Pre-existing bug fixed in passing: a fresh alt had a zero identity all session, and `die.go:71` skips respawn for that. Make an alt, die, confirm you come back. |
| C4 | **`character view` / `character hire`.** | Both dropped `MobInstanceId` and `IsMob`; now go through `mobs.AdoptCharacter`. A hired alt should behave as a mob (training caps apply) and be attackable normally. |

---

## D. Decisions only you can make

| # | Decision | Context |
|---|---|---|
| D1 | **Is repeated knockdown a lock?** | The newbie saw **five consecutive failed stands** because the free `! SWEEP!` re-fires every round. Working as designed. Feel judgement. |
| D2 | **Five balance knobs are silently inert at zero.** `StealCooldown` (60), `StealHiddenBonus` (25), `ShadowCooldown` (5), `SneakFailCooldown` (3), `PackScatterRounds` (2). | Absent from `config.yaml`, and their validators cannot repair a zero. Three are skullduggery economy. **Filed, not fixed** — adding the keys is a live balance change and wants its own slice + playtest. |
| D3 | **154 help templates contain em dashes.** | House style bans them in player copy, but that guard was written for the U8 action helpfiles. Applying it tree-wide is a policy call, and a mechanical pass over that much prose would damage sentences. |
| D4 | **`help health` is a bare column of twelve percentage ranges.** | U11 exempted it as a "resource-bar legend". The newbie's read suggests that was too generous for a help *page*. Keep the exemption, or give the page prose? |
| D5 | **`bury` is filed under `character` and a newcomer could not find it.** | U11 restored `character` because that is what the main `keywords.yaml` says. The finding is that the placement is unintuitive, not that the restore was wrong. |

---

## E. Filed defects, not caused by U11 — confirm you want them queued

None of these block the merge. Detail and evidence in the findings audit.

- `help submission` prints raw HTML entities: `set submission &lt;mercy|...&gt;`
- **`kick` produced no output at all** — echoed, then silence, verified as the
  only command that round
- `consider` calls the deliberately harmless Straw Effigy "severely outmatched"
- The Drill Yard's three NPCs are all incapable of hitting back, so a combat
  goals file bound to room 5227 starts where nothing can test defence
- `look <unknown-noun>` replies "Look at what???" as if given no argument
- `Your fists flies wide but somehow lands...` (grammar, and self-contradictory)
- `set charset` promises a conversion it does not perform
- Ragged short-line wrapping across the help corpus — likely one wrapper bug
  measuring invisible markup, rather than 454 files to fix by hand
- `tools/playtest/engine-profile.yaml` is stale: says spawn is Sanctum Basin
  (it is Pothole Coulee) and "ten skills" (the index lists 21)

---

## F. Already verified — do NOT spend time re-checking

- **Contested prone recovery works from the attacker's side.** Observed
  repeatedly, both outcomes verbatim, in both runs: `attempts to stand, but
  slips and falls in the chaos of battle` then `clambers to their feet in a
  rushed panic`. That line had not printed for any player since U10.
- Actor and room text correctly differ: *"You attempt to stand, but slip back
  down..."* vs *"...slips and falls..."*.
- **24 help topics opened, zero broken links**, including 13 reachable only by
  following a page link.
- `help time`, `help dual-wield`, `help wimpy` all resolve (all three were
  wrongly deleted mid-slice and restored).
- All combat helpfiles pass with all 10 cross-references resolving.
- `help stand` documents both the new contest and the guaranteed paid escape.
- `help prone`, `help armor`, `help quell`, `help defy` explain mechanics with
  no numbers.
- Isolated boot: exit 124, zero panics, one `Server Ready`, cartcheck clean.
- Full `internal/...` suite, `gofmt`, and `go build` all clean.

---

## G. Housekeeping

- **Two commits (`f5dd61c44`, `385bf8f58`) contain a review agent's mutation
  experiment** swept up by `git add -A`. HEAD is correct and the merge result is
  unaffected; it only matters if someone bisects through them. Left deliberately
  rather than risk rewriting 18 commits.
- The knockdown content gap is folded into the **behaviour unification arc**,
  not filed separately, per your note that these gaps are known.
