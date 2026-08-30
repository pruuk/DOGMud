# U11 filed findings: help copy, config and context.md

**Date:** 2026-08-30  
**Source:** U11 arc closer, `docs/superpowers/plans/2026-08-30-u11-arc-closer.md`

Filed, not fixed. Each item is real; each was out of scope for a closing
docs slice. Verify against source before acting -- this is a point-in-time
report.

---

## 1. Em dashes in help templates (154 files, 321 occurrences)

Found by inverting `u8ActionHelpPaths` into a whole-tree walk (U11 Task 7).
House style bans en/em dashes in player-facing copy, but that guard was
authored for the U8 action helpfiles. Applying it to all 454 templates is a
**policy decision, not a coverage fix**, and a mechanical substitution across
this much prose would damage sentences -- so the em-dash half of
`findForbiddenU8HelpDisclosures` stays scoped to `u8ActionHelpPaths` and the
numeric half runs tree-wide.

Owner decision 2026-08-30: fix the numbers now, file the dashes.

| Template | Count |
|---|---|
| `about` | 11 |
| `achievements` | 1 |
| `alchemy` | 4 |
| `alias` | 1 |
| `apex-predator` | 1 |
| `arena` | 2 |
| `assess` | 2 |
| `attack` | 8 |
| `bartering` | 3 |
| `blacksmithing` | 2 |
| `blood-boil` | 2 |
| `blood-frenzy` | 1 |
| `booming-lungs` | 2 |
| `cattail-down-cloak` | 1 |
| `chat` | 1 |
| `chrysalis-cocoon` | 1 |
| `chrysalis-haste` | 1 |
| `chrysalis-regeneration` | 1 |
| `chrysalis-setting` | 1 |
| `cloth-bandage` | 1 |
| `cloth-pants` | 1 |
| `colossus-form` | 2 |
| `commanding-presence` | 1 |
| `communion-of-flesh` | 2 |
| `conditions` | 2 |
| `conviction-armor` | 1 |
| `conviction-barrage` | 1 |
| `conviction-spike` | 2 |
| `conviction-surge` | 1 |
| `conviction-ward` | 1 |
| `cooking` | 1 |
| `copper-ring` | 1 |
| `core-discharge` | 4 |
| `core-drain` | 2 |
| `craft` | 9 |
| `death` | 1 |
| `dense-muscles` | 3 |
| `devtool` | 1 |
| `discorporation` | 1 |
| `dissonance-organ` | 1 |
| `dogmud` | 5 |
| `drink` | 2 |
| `drowned-veil` | 1 |
| `eat` | 1 |
| `empathic-shroud` | 1 |
| `energy-bread` | 1 |
| `engraved-bracelet` | 1 |
| `equipment` | 1 |
| `evil-eye` | 1 |
| `extra-arms` | 2 |
| `flee` | 1 |
| `forage` | 1 |
| `foraging` | 1 |
| `gc` | 1 |
| `gearup` | 1 |
| `gem-set-necklace` | 1 |
| `gold-signet-ring` | 1 |
| `grilled-meat` | 1 |
| `heal` | 2 |
| `hemorrhagic-burst` | 1 |
| `hemorrhagic-wave` | 1 |
| `herbal-tea` | 1 |
| `hollow-bones` | 2 |
| `hunter-eel-scale-vest` | 1 |
| `iron-buckler` | 1 |
| `iron-dagger` | 2 |
| `iron-short-sword` | 2 |
| `item-names` | 2 |
| `jewelcrafting` | 2 |
| `keen-senses` | 1 |
| `kinetic-backlash` | 1 |
| `kinetic-hurl` | 2 |
| `kinetic-shove` | 3 |
| `lake-iron-hook-spear` | 2 |
| `lake-tonic-of-steady-hand` | 1 |
| `leather-leggings` | 1 |
| `leather-satchel` | 1 |
| `leather-vest` | 1 |
| `linen-tunic` | 1 |
| `mail` | 1 |
| `map` | 13 |
| `masterwork-gem-pendant` | 1 |
| `masterwork-plate-helm` | 1 |
| `mend-wounds` | 1 |
| `mind-fog` | 2 |
| `mind-spike` | 2 |
| `mutation-catalyst` | 1 |
| `mutations` | 7 |
| `nerve-disruption` | 2 |
| `neural-stun` | 3 |
| `neural-toxin` | 2 |
| `newbie` | 1 |
| `oasis` | 5 |
| `ossified-frame` | 3 |
| `pet` | 1 |
| `petition` | 2 |
| `polished-stone-amulet` | 1 |
| `prehensile-tail` | 2 |
| `psychic-anchor` | 1 |
| `pyretic-surge` | 1 |
| `quicksilver-nerves` | 2 |
| `radiant-avatar` | 1 |
| `reach` | 1 |
| `reinforced-wooden-shield` | 1 |
| `remove` | 1 |
| `rending-claws` | 1 |
| `rep` | 1 |
| `repair-pulse` | 3 |
| `reply` | 1 |
| `report` | 1 |
| `resonant-larynx` | 2 |
| `rhetoric` | 5 |
| `search` | 4 |
| `second-sight` | 1 |
| `sensory-overload` | 2 |
| `sensory-veil` | 2 |
| `set-prompt` | 1 |
| `setdesc` | 1 |
| `silver-ring` | 1 |
| `simple-pendant` | 1 |
| `skill-attunement` | 1 |
| `skills` | 16 |
| `sparks` | 1 |
| `species` | 1 |
| `spellcasting` | 1 |
| `spells` | 7 |
| `spiracle-lungs` | 1 |
| `steel-buckler` | 1 |
| `steel-longsword` | 2 |
| `sticky-secretion` | 1 |
| `submission` | 6 |
| `suicide` | 1 |
| `surrender` | 4 |
| `synaptic-overload` | 1 |
| `tail` | 3 |
| `tailoring` | 2 |
| `tame` | 1 |
| `thick-hide` | 3 |
| `ticks` | 4 |
| `titan-growth` | 2 |
| `title` | 5 |
| `toxicity` | 2 |
| `track` | 4 |
| `trade` | 1 |
| `trail-rations` | 1 |
| `translucent-body` | 2 |
| `tremorsense` | 3 |
| `triggers` | 10 |
| `veil-rend` | 2 |
| `veiling-musk` | 1 |
| `vital-surge` | 1 |
| `weather` | 1 |
| `wool-cloak` | 1 |
| `zealous-conviction` | 2 |
