# DOGMud documentation

Everything that isn't code. Start with [`world.md`](world.md) if you want the
setting, or [`schemas/`](schemas/) if you want to author content.

## Start here

| Path | What's in it |
|------|--------------|
| [`world.md`](world.md) | The world-design document — lore, factions, zones, species |
| [`PATCH_NOTES.md`](PATCH_NOTES.md) | Dated shipping log of every change |
| [`PRE_DEPLOY_PLAYTEST_CRIBSHEET.md`](PRE_DEPLOY_PLAYTEST_CRIBSHEET.md) | The owner's manual checklist before deploying the unified-resolution arc; the Elemental Queen fight is mandatory |
| [`U11_OWNER_PUNCHLIST.md`](U11_OWNER_PUNCHLIST.md) | Short companion to the crib sheet: only what U11 changed and did NOT verify, plus the decisions the owner has to make. Point-in-time, 2026-08-30 |
| [`PATH_TO_1.0.md`](PATH_TO_1.0.md) | Remaining work before the 1.0 tag |

## Reference

| Path | What's in it |
|------|--------------|
| [`schemas/`](schemas/) | YAML schema references (room, mob, item, spell, buff, dialogue, schedule, patrol) |
| [`architecture/`](architecture/) | System-level architecture notes and deliberate divergences from upstream |
| [`economy/`](economy/) | Living-economy design and tuning |
| [`balance/`](balance/) | Combat and progression tuning |
| [`worldbuilding/`](worldbuilding/) | Zone expansion plan, coordinate map, settlement canon, world atlas |

## Guides

| Path | What's in it |
|------|--------------|
| [`guides/CONTENT_GENERATION_GUIDE.md`](guides/CONTENT_GENERATION_GUIDE.md) | How the content-generation workflow fits together |
| [`guides/github_guide.md`](guides/github_guide.md) | Branch strategy and Git workflow |
| [`guides/DEPLOYMENT_GUIDE.md`](guides/DEPLOYMENT_GUIDE.md) | Deploying to the production droplet |
| [`guides/ADVERTISING_LISTINGS.md`](guides/ADVERTISING_LISTINGS.md) | Copy used for MUD-listing sites |
| [`guides/TESTING_GUIDE.md`](guides/TESTING_GUIDE.md) | Reproducible race-enabled test baseline, local and CI |

## Audits & findings

Point-in-time reports. Each is true as of its date and is **not** kept current —
verify against the code before acting on one.

| Path | What's in it |
|------|--------------|
| [`audits/`](audits/) | Tech-debt and test-coverage audits, the code-smell review queue, upstream cherry-pick triage, playtest findings |
| [`audits/META_REVIEW_AGENT_WORK_2026-08-08.md`](audits/META_REVIEW_AGENT_WORK_2026-08-08.md) | Adversarial meta-review grading the Cursor and Jules agent work after 2026-08-04 |
| [`audits/ADVERSARIAL_REVIEW.md`](audits/ADVERSARIAL_REVIEW.md) | Jules' architectural teardown: global lock, autosave I/O, boot wiring, progression math |
| [`audits/ADVERSARIAL_CODE_REVIEW_2026-08-07.md`](audits/ADVERSARIAL_CODE_REVIEW_2026-08-07.md) | Fresh-eyes review of server, commands, persistence, loaders, CI and client: 9 high, 18 medium, 5 low findings |
| [`audits/2026-08-19-progression-firing-audit.md`](audits/2026-08-19-progression-firing-audit.md) | Every progression call site with its firing condition. Ten different conventions found (the plan estimated seven); input to U10b |
| [`audits/2026-08-30-kinetic-backlash-reflect.md`](audits/2026-08-30-kinetic-backlash-reflect.md) | Where "An unseen force slams into you" comes from: buff 109 via the Kinetic Backlash mutation. Two defects, both open: the reflect has no death guard so a corpse hits back, and the copy names no source |
| [`audits/2026-08-30-shield-spells-converge-at-the-cap.md`](audits/2026-08-30-shield-spells-converge-at-the-cap.md) | Why Chrysalis Cocoon and Conviction Ward feel identical: shield magnitude is an unbounded percentage feeding a 75% cap, so every shield saturates past ~Wil 100 / skill 25. For the spell scaling arc |
| [`perf/`](perf/) | Performance baselines and profiling notes |

Two of these are **live scripts**, not point-in-time reports — run them rather
than reading an old result:

| Path | What it does |
|------|--------------|
| [`../tools/context_md_audit.py`](../tools/context_md_audit.py) | Finds `context.md` files documenting symbols the package no longer defines. Coverage is 100%; accuracy is what rots. Triage the output, it has known false positives |
| [`../tools/balance/`](../tools/balance/) | Combat balance models behind roadmap 5.11 — skill leverage, real player-vs-mob matchups, and the `SkillWeight`/statpool tuning matrix |
| [`../tools/fold_mob_training_to_base.py`](../tools/fold_mob_training_to_base.py) | One-shot U10b-0 Phase A migration that folded authored mob `training:` into `base:` across 599 templates, kept as the record of the arithmetic. `--verify <report.csv>` re-checks it against the files on disk |

## History

| Path | What's in it |
|------|--------------|
| [`roadmaps/`](roadmaps/) | Long-form roadmaps: development plan, combat-state, mob-aliveness |
| [`roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md`](roadmaps/ADVERSARIAL_REVIEW_REMEDIATION_ROADMAP.md) | 37-finding remediation roadmap from the 2026-08-07 adversarial reviews. Note: its source doc `ADVERSARIAL_CODE_REVIEW_2026-08-07.md` is currently untracked in the repo root |
| [`roadmaps/UNIFIED_RESOLUTION_ROADMAP.md`](roadmaps/UNIFIED_RESOLUTION_ROADMAP.md) | **CLOSED.** Stages U0–U12 collapsing 34 scattered opposed-roll resolution sites onto one contest core, plus the cost and harm model. Refactor-first, one behaviour flip at U6. Shipped to production 2026-08-30; kept as the arc's merge-evidence record |
| [`superpowers/specs/2026-08-31-toxicity-tolerance-and-visibility-design.md`](superpowers/specs/2026-08-31-toxicity-tolerance-and-visibility-design.md) | Toxicity redesign: tolerance earned from the alchemy skill rather than a flat ceiling, nine untagged legacy potions backfilled, and two feedback bands added below the first penalty band so pressure is visible before it bites |
| [`superpowers/plans/2026-08-31-toxicity-tolerance-and-visibility.md`](superpowers/plans/2026-08-31-toxicity-tolerance-and-visibility.md) | Implementation plan for the toxicity spec above, in 8 tasks. Corrects four of the spec's facts: Meirok's vitality is read as `ValueAdj` (150 with gear, not the save's `base: 104`), which retunes `ToxicityAlchemyScale` to 2.5; `status` and `help toxicity` already work; and the new ceiling would have made Chrysalis Catalyst and Mutagen Brew undrinkable, so the heavy tier is rescaled |
| [`superpowers/specs/2026-08-31-messaging-unification-design.md`](superpowers/specs/2026-08-31-messaging-unification-design.md) | Design for the messaging unification arc: 14 narration stores, 6 role conventions, 2 token engines and 3 send helpers collapsed onto one Actor/Actee/Observer core. Same refactor-first shape as the resolution arc, guarded by a snapshot harness that freezes today's flavor |
| [`superpowers/audits/2026-08-31-messaging-surface-sweep.md`](superpowers/audits/2026-08-31-messaging-surface-sweep.md) | M0 of the messaging arc: a mechanically derived inventory of every text-bearing key spelling and every pipeline bypass, locked in CI by `messaging_surface_guard_test.go` |
| [`perf/droplet-screenshots/`](perf/droplet-screenshots/) | DigitalOcean Insights captures per prod deploy, with an index explaining what each shows |
| [`superpowers/`](superpowers/) | Per-feature specs and implementation plans, live and completed |
| [`archive/`](archive/) | Retired documents and old bug screenshots |
| [`upstream/`](upstream/) | Upstream-facing material in both directions: artifacts inherited from the GoMud engine, and briefs for changes we want to send out to third-party tools |
| [`upstream/archify-sublabel-pr-brief.md`](upstream/archify-sublabel-pr-brief.md) | Ready-to-hand-off brief for a PR to `tt-a1i/archify`: four of its five renderers never measure `sublabel`/`tag` against the node box, so diagram text overflows while validation still passes |
| [`images/`](images/) | Screenshots used by the top-level README |

Per-package developer notes live beside the code, as `context.md` in each
`internal/` and `modules/` package — see the convention in the repo-root
`CLAUDE.md`.
