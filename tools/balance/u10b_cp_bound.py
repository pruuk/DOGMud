"""Casting is CP-bound over a full hour, not cooldown-bound.

waitrounds caps the BURST rate; conviction regen caps the SUSTAINED rate, and
over an hour the sustained rate is what matters.
"""

ROUNDS_PER_HOUR = 3600 / 4          # 900, RoundSeconds = 4
CP_REGEN_PCT = 0.02                 # PlayerConvictionRegenPct
REGEN_EVERY_ROUNDS = 3              # regen tick cadence

# mid profile: ConvictionMax 450 (Cha 110 x 3 + Wil 115 x 1 + 5)
CP_MAX = 450

cp_per_tick = CP_MAX * CP_REGEN_PCT
cp_per_round = cp_per_tick / REGEN_EVERY_ROUNDS
cp_per_hour = cp_per_round * ROUNDS_PER_HOUR

print("Conviction budget for a mid character (CP_MAX = %d)" % CP_MAX)
print("  %.1f CP per tick, %.2f CP per round, %.0f CP per hour" % (cp_per_tick, cp_per_round, cp_per_hour))
print("  (plus the %d you start the hour with)" % CP_MAX)
print()

print("%-22s %-7s %-12s %-12s %s" % ("spell family", "cost", "casts/hr", "burst cap", "binding limit"))
for name, cost, wait in [
    ("spellcasting (cheap)", 20, 1),
    ("spellcasting (typical)", 40, 1),
    ("spellcasting (sparks)", 75, 1),
    ("manifestation (raise)", 30, 4),
    ("manifestation (conjure)", 45, 4),
    ("manifestation (charm)", 120, 3),
]:
    sustained = (cp_per_hour + CP_MAX) / cost
    burst = ROUNDS_PER_HOUR / wait
    binding = "CP" if sustained < burst else "cast time"
    print("%-22s %-7d %-12.0f %-12.0f %s" % (name, cost, sustained, burst, binding))

print()
print("So over a real hour BOTH are CP-bound, by a wide margin.")
print()
print("Realistic uses/hour, at 10%% engagement for the combat-embedded part:")
print("  spellcasting  : a caster spends most of their CP casting.")
print("                  typical 40 CP -> %.0f casts/hr if CP goes only to spells"
      % ((cp_per_hour + CP_MAX) / 40))
print("  manifestation : casts (CP-bound) + assess (free, 6-round cd, corpse-bound)")
print("                  %.0f casts/hr + ~15 assess/hr" % ((cp_per_hour + CP_MAX) / 45))
print()
print("NOTE: a fielded companion RESERVES conviction permanently, shrinking the")
print("pool it regenerates from, so a manifestor running pets sustains fewer")
print("casts than this. Treated as a haircut below.")
