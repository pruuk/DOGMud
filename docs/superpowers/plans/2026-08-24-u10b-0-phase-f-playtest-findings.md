# U10b-0 arc-close playtest — findings

Reports under `tools/playtest/reports/` are gitignored, so the findings are
extracted here.

**Run:** `/playtest local --checkout <repo> bug-finder
2026-08-24-u10b-0-arc-close.yaml`
**Env:** ephemeral Docker, run_id `b5e0c73decbc066a`, commit `ab7b6ff08`, dirty
(untracked `.agents/`, `.codex/`).
**Character:** Tessic, fresh, veteran start (skipped tutorials to reach combat
inside the wall clock).
**Outcome:** partial — the progression, banner and assess goals were driven to
a conclusion; the companion logout/login goal and the non-combat-skill pacing
goal were not reached before the budget ran out.

---

## PASS — the arc's core behaviour works

- **Progression banners fire, quickly, and read correctly.** Both
  `SKILL ADVANCEMENT` and `STATISTIC INCREASED` appeared within roughly four
  rounds of a first fight. The stat banner named `strength`; the skill banner
  named `manifestation` and showed the tier change `novice → apprentice`. **No
  raw numbers, ranks or percentages anywhere in either.**
- **Early progress feels fast**, which is what the retune intended. A brand new
  character improved twice inside one fight.
- **`assess` cost disclosure is in words**, per the no-hard-numbers rule:
  "a heavy share of your conviction", "nearly all you can set aside".
- **`assess` trains manifestation**, and the advancement fired on the
  assessment itself.

## BUG — `help skills` was wrong in three ways *(fixed, `2d5fcf411`)*

It listed **`first-aid`**, which exists nowhere in `internal/skills`, and
omitted **`manifestation`** and **`salvage`**, which both exist and both appear
in the `skills` command's own output. `CLAUDE.md` carried the same error one
level up, claiming **10** skills where the game has **16**.

## BUG — `assess` advertises raising; the obvious verb does not exist *(fixed, `2d5fcf411`)*

`assess <corpse>` prints "Raising a skeleton would set aside a heavy share of
your conviction while it serves." Typing `raise skeleton` returns **"Raise not
recognized. Type help for commands."** `help assess` then spends four paragraphs
on raising, "your ritual" and what each form costs, and **never names the verb**.

Raising is a spell (`cast raise-skeleton corpse`); the six forms live in
`_datafiles/world/dogmud/spells/raise-*.yaml`. The help now says so.

## OBSERVATION — retired-model framing in player text *(fixed, `2d5fcf411`)*

`help assess` read "the more corpses you study, the better you become", and
`help skills` "All skills progress through use" with nothing further. Both echo
the use-count model U10b-0 retired. Not false enough to mislead badly, but it is
exactly the framing the arc replaced, so both now state what governs the pace.

## CONCERN — no `help progression` topic *(not fixed)*

`help progression` returns "No help found". How characters improve is the single
most-changed system in this arc and has no help topic of its own. `help skills`
and `help assess` now each carry a sentence, but a player looking for the rule
directly finds nothing. Worth a topic; out of scope for a phase that is meant to
close the arc.

## OBSERVATION — known issues re-confirmed, not re-filed

- **"You attack the darkness!"** on attacking a name whose owner just died.
  Already filed as `project-attack-the-darkness-messaging`.
- **Combat message repetition** — "You throw everything you have into a wild
  swing at Old Fen!" three times in one round. Already filed as
  `project-combat-message-sprawl-and-quell-defy-gap`.
- **`playtestrun stop` exits 0 and leaves the container running.** Re-confirmed
  again; torn down with `docker rm -f
  dogmud-playtest-b5e0c73decbc066a-server-1`.

## NOT COVERED

- **Companion across a logout/login boundary.** This is the branch the migration
  audit flagged as highest-risk and the wall clock did not reach it. The
  arithmetic is verified (Phase F Task 1) but the live round trip is not. **This
  belongs in the pre-deploy playtest**, where the owner already has to fight the
  Elemental Queen with Meirok, who owns two legacy golems.
- Non-combat skill pacing (forage, search, a craft) against combat pacing.
