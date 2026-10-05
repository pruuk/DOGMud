# Spec Template for a New System

Copy the template into a GitHub issue or a draft pull request and fill it in
before you start building. One page is the target; a spec that needs more
than two pages is usually two systems. The owner replies with yes, no, or
changes. See [CONTRIBUTING](../../.github/CONTRIBUTING.md), section 1.

A worked example follows the template.

## The template

```markdown
# <System name>

## What the player sees
Two to five sentences: what a player does, what they get, why it is fun.
List the new commands.

## Out of scope
What this spec deliberately does not do, so review does not ask for it.

## Systems it touches
Packages and files it changes, and any hook it needs in shared engine code
(internal/rooms, internal/actions, internal/usercommands, world.go).

## What it reuses
For each job the system does, the existing mechanism it uses
(see HOUSE_RULES.md, section 1). If it needs a new mechanism, say why the
existing one does not fit.

## What it saves
New state that must survive a restart, where it lives, and whether it is
living state or an instance override. "Nothing new" is a fine answer.

## Gold and items
Every new source of gold or items, and the sink or limit that balances it.
How a player with a second account could abuse it, and what stops them.

## Balance knobs
Each new tuning number: knob name, what it does, proposed shipped value.

## Outside APIs and player data
Does it call a model or any outside service? What player data would leave
the server? "None" is the answer we hope for.

## How we will know it works
The tests you will write, and what a playtest should try.

## Open questions
Anything you want the owner to decide.
```

## Worked example: fishing

This example shows the level of detail we want. It is an illustration, not an
approved design.

> # Fishing
>
> ## What the player sees
> A player standing by water with a fishing pole types `fish`. After a short
> wait they land a fish, an old boot, or nothing. Better poles, bait and the
> right time of day catch better fish. A new cooking recipe turns a raw catch
> into the smoked river fish that already exists (item 40125), and fish sell
> to fishmongers. New command: `fish`.
>
> ## Out of scope
> Boats, deep-sea fishing, fishing contests, fish farming.
>
> ## Systems it touches
> A new `actions.Fish` beside `actions.Forage` in `internal/actions`, a
> `fish` user command, a `fish` mob command for NPC fishers, and a yield table
> beside the forage tables in `internal/forager`. No hooks in shared engine
> code.
>
> ## What it reuses
> - **The forage path.** `actions.Fish` takes the same `Actor` that
>   `actions.Forage` does, so players and NPC fishers share one code path, and
>   it rolls through the same attempt structure (`forager.ForageAttempt`) with
>   the actor's search score.
> - **Where it works.** The room biome, read through `room.GetBiome()`.
>   Fishing works in the `water`, `river` and `shore` biomes, which already
>   exist.
> - **Pacing.** `char.TryCooldown("fish", ...)`, as forage does.
> - **Sight.** The score is scaled by `messaging.SightMult`, so fishing in the
>   dark is harder, like foraging.
> - **Narration.** `messaging.SendTrio`, so watchers see the catch and
>   players who cannot see the fisher do not learn their name.
> - **Selling.** Fish are ordinary items with vendor categories, so the
>   shops' existing buy rules (`shops.EvaluateBuyRules`) price them; no new
>   price formula.
>
> ## What it saves
> Nothing new. Fish are carried items and are saved with the player.
>
> ## Gold and items
> - Source: fish, from a yield table per biome.
> - Sinks: bait is used up on every cast and bought from fishmongers; poles
>   wear out.
> - Abuse: a player could fish all day and flood the fish market. The
>   cooldown limits the rate, and a shop stops buying once it holds its
>   overstock cap. An alt changes nothing, since each character
>   pays its own cooldown and bait.
>
> ## Balance knobs
> | Knob | What it does | Proposed |
> |---|---|---|
> | `FishCooldownRounds` | Rounds between casts | 8 |
> | `FishBaitBonusMult` | Catch-quality multiplier with bait | 1.25 |
> | `FishDawnDuskBonusMult` | Catch multiplier at dawn and dusk | 1.15 |
>
> ## Outside APIs and player data
> None.
>
> ## How we will know it works
> - Unit tests: a catch in each water biome; no catch in a dry biome; the
>   cooldown blocks a second cast; bait is consumed; a mob actor fishes
>   through the same path.
> - The test that "no catch without water" passes is shown to fail when the
>   biome check is removed.
> - Playtest: fish at a river by day and by night, sell the catch, check the
>   fishmonger stops buying at its overstock cap.
>
> ## Open questions
> - Should NPC fishers sell their catch to the fishmonger, like foragers sell
>   herbs?
> - Is there a fishing skill, or is it a Perception and Search activity like
>   foraging?
