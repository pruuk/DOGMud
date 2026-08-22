"""Vitality cannot be solved by the generic pace model.

Its OnStatUse faucet does not exist -- nothing in production calls it -- so
"match the old OnStatUse pace" is undefined. Its two real faucets are the regen
tick (live) and taking a physical crit (dormant until Phase D sets
ObservedCritProgressionBonus).
"""

import math

BASE = 0.12
RATE = 2.25
D_BELOW = 3.0
NEW_SOFT = 50.0
REGEN_BASE = 0.01     # RegenProgressionBase
REGEN_CURVE = 3.0     # RegenProgressionCurve
CRIT_BONUS = 0.5      # ObservedCritProgressionBonus, once set


def curve(rank):
    if rank <= 0:
        return BASE
    return BASE * math.exp(-D_BELOW * min(rank, NEW_SOFT) / NEW_SOFT) if rank <= NEW_SOFT else \
        BASE * math.exp(-D_BELOW) * math.exp(-2.0 * (rank / NEW_SOFT - 1.0))


def damper(T):
    """regenDamperFactor: curve(rank)/BASE. Exactly 1.0 at rank 0."""
    return curve(T) / BASE


def regen_chance(T, depletion, m):
    """Chance per regen tick at a given pool depletion (0..1)."""
    return REGEN_BASE * (depletion ** REGEN_CURVE) * m * damper(T)


def crit_chance(T, m):
    """Chance per physical crit RECEIVED, once the knob is set."""
    return min(1.0, curve(T) * RATE * m * CRIT_BONUS)


print("VITALITY: the two real faucets, at multiplier 4.50 (shipped)")
print()
print("Regen tick. Damper is NEW in Phase C -- before the re-key it was pinned")
print("at exactly 1.0 because nothing ever moved vitality's rank.")
print("%-10s %-9s %-13s %-13s %s" % ("Training", "damper", "at 100% hurt", "at 50% hurt", "at 25% hurt"))
for T in (0, 5, 14, 25, 40):
    print("%-10d %-9.3f %-13.4f %-13.5f %.5f" % (
        T, damper(T),
        regen_chance(T, 1.0, 4.5), regen_chance(T, 0.5, 4.5), regen_chance(T, 0.25, 4.5)))

print()
print("Crit-toughen, per physical crit RECEIVED (dormant today, Phase D enables)")
print("%-10s %-12s %s" % ("Training", "m=4.50", "m=1.00"))
for T in (0, 5, 14, 25, 40):
    print("%-10d %-12.4f %.4f" % (T, crit_chance(T, 4.5), crit_chance(T, 1.0)))

print()
print("What the damper alone already did to vitality in Phase C:")
for T in (14, 25, 40):
    print("  Training %-3d regen chance is %.2fx what it was pre-re-key"
          % (T, damper(T)))

print()
print("Per-round estimate for a character fighting at half health,")
print("regen tick every 3 rounds, crits received ~5%% of incoming attacks:")
for T in (0, 14, 25):
    per_tick = regen_chance(T, 0.5, 4.5)
    per_round_regen = per_tick / 3.0
    per_round_crit = 0.05 * crit_chance(T, 4.5)
    print("  Training %-3d regen %.5f/round + crit %.5f/round = %.5f  (~1 gain per %.0f rounds)"
          % (T, per_round_regen, per_round_crit, per_round_regen + per_round_crit,
             1.0 / (per_round_regen + per_round_crit)))
