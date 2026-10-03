package aicompanion

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"gopkg.in/yaml.v3"
)

const summonsDir = `../../_datafiles/world/dogmud/mobs/summons`
const archetypesDir = `../../_datafiles/world/dogmud/behaviors/archetypes`

// Each of the six companions is meant to be a different person in a fight
// as well as in talk: a different scripted archetype on the mob template,
// a different set of moves offered to the model, and a different way of
// fighting described to it.
func TestEachCompanionFightsInTheirOwnWay(t *testing.T) {
	profiles, errs := loadProfiles()
	if len(errs) > 0 {
		t.Fatalf("profiles: %v", errs)
	}
	if len(profiles) != 6 {
		t.Fatalf("expected six companions, got %d", len(profiles))
	}
	archetypes := map[string]string{}
	approaches := map[string]string{}
	moveSets := map[string]string{}
	for id, p := range profiles {
		if strings.TrimSpace(p.Combat.Approach) == `` {
			t.Errorf("%s has no fighting approach", id)
		}
		if other, dup := approaches[p.Combat.Approach]; dup {
			t.Errorf("%s fights exactly as %s does", id, other)
		}
		approaches[p.Combat.Approach] = id
		key := strings.Join(movesFor(p), `,`)
		if other, dup := moveSets[key]; dup {
			t.Errorf("%s and %s are offered the same moves (%s)", id, other, key)
		}
		moveSets[key] = id

		matches, _ := filepath.Glob(filepath.Join(summonsDir, `*`))
		var tmpl *mobs.Mob
		for _, f := range matches {
			if strings.HasPrefix(filepath.Base(f), strconv.Itoa(p.MobId)+`-`) {
				raw, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				tmpl = &mobs.Mob{}
				if err := yaml.Unmarshal(raw, tmpl); err != nil {
					t.Fatalf("%s: %v", f, err)
				}
			}
		}
		if tmpl == nil {
			t.Fatalf("%s: no mob template %d", id, p.MobId)
		}
		a := tmpl.BehaviorArchetype
		if !strings.HasPrefix(a, `companion_`) {
			t.Errorf("%s fights with %q, not a companion archetype", id, a)
		}
		if _, err := os.Stat(filepath.Join(archetypesDir, a+`.yaml`)); err != nil {
			t.Errorf("%s: archetype %s missing: %v", id, a, err)
		}
		if other, dup := archetypes[a]; dup {
			t.Errorf("%s and %s share the archetype %s", id, other, a)
		}
		archetypes[a] = id
		if tmpl.AIProfile == `` {
			t.Errorf("%s: template has no aiprofile", id)
		}
		if _, ok := characters.ParseSurrenderPolicy(tmpl.SurrenderPolicy); tmpl.SurrenderPolicy != `` && !ok {
			t.Errorf("%s: bad surrender_policy %q", id, tmpl.SurrenderPolicy)
		}
		if _, ok := characters.ParseSubmissionPolicy(tmpl.SubmissionPolicy); !ok {
			t.Errorf("%s: bad submission_policy %q", id, tmpl.SubmissionPolicy)
		}
	}
}

// Isaura's kinetic hurl is her cheap working; without a category no
// scripted caster tree would ever reach for it.
func TestKineticHurlIsAHarmSpellTheTreesCanFind(t *testing.T) {
	raw, err := os.ReadFile(`../../_datafiles/world/dogmud/spells/kinetic-hurl.yaml`)
	if err != nil {
		t.Fatal(err)
	}
	sd := spells.SpellData{}
	if err := yaml.Unmarshal(raw, &sd); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range sd.Categories {
		found = found || c == `harm_single`
	}
	if !found {
		t.Fatal("kinetic-hurl needs the harm_single category")
	}
}

// styleSpells seeds a healer's and a battlemage's spells on her.
func styleSpells(t *testing.T, her *mobs.Mob) {
	t.Helper()
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{
		`heal`: {SpellId: `heal`, Name: `Mend Flesh`, EffectType: `heal`, Cost: 25,
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle},
		`mend-wounds`: {SpellId: `mend-wounds`, Name: `Mend Wounds`, EffectType: `heal`, Cost: 40,
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle},
		`conviction-ward`: {SpellId: `conviction-ward`, Name: `Conviction Ward`, EffectType: `shield`, Cost: 30,
			AttackType: combatvocab.AttackNone, DamageType: combatvocab.DamageNonHarm, Targeting: combatvocab.TargetSingle},
		`kinetic-hurl`: {SpellId: `kinetic-hurl`, Name: `Kinetic Hurl`, EffectType: `damage`, Cost: 15,
			AttackType: combatvocab.AttackSpell, DamageType: combatvocab.DamagePhysical, Targeting: combatvocab.TargetSingle},
	}))
	her.Character.SpellBook = map[string]int{`heal`: 1, `mend-wounds`: 1, `conviction-ward`: 1, `kinetic-hurl`: 1}
	her.Character.Conviction = 200
}

func queuedCasts(instanceId int) []string {
	var out []string
	for _, cmd := range events.DrainQueuedInputsForTest(instanceId) {
		out = append(out, cmd)
	}
	return out
}

// A healer mends her owner mid-fight once they are hurt past her line, with
// the strongest mending she can pay for, and wards them once as it opens.
// Nobody else does.
func TestHealerMendsAndWardsHerOwner(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	styleSpells(t, her)
	m, c, _ := strangerModule()
	c.profile.Combat.MendBelow = 60
	c.profile.Combat.WardOwner = true
	c.fight = &fightState{Stance: `hold_back`}
	owner.Character.HealthMax.Value = 100
	owner.Character.Health = 90

	if cmd := m.tendOwner(c, her, owner, room, 90); cmd != `cast conviction-ward @1` {
		t.Fatalf("first, a ward on her owner: %q", cmd)
	}
	if cmd := m.tendOwner(c, her, owner, room, 90); cmd != `` {
		t.Fatalf("only once a fight, and he is not hurt: %q", cmd)
	}
	owner.Character.Health = 40
	if cmd := m.tendOwner(c, her, owner, room, 40); cmd != `cast mend-wounds @1` {
		t.Fatalf("hurt past her line, her strongest mending on him: %q", cmd)
	}
	her.Character.Conviction = 30
	if cmd := m.tendOwner(c, her, owner, room, 40); cmd != `cast heal @1` {
		t.Fatalf("short of conviction, the cheaper one: %q", cmd)
	}
	her.Character.Cooldowns = characters.Cooldowns{`special-move`: 3}
	if cmd := m.tendOwner(c, her, owner, room, 40); cmd != `` {
		t.Fatalf("not while she cannot act yet: %q", cmd)
	}
	her.Character.Cooldowns = nil

	c.profile.Combat.MendBelow, c.profile.Combat.WardOwner = 0, false
	if cmd := m.tendOwner(c, her, owner, room, 10); cmd != `` {
		t.Fatalf("a companion who is no healer leaves it to the model: %q", cmd)
	}
}

// The reflex carries a healer's mending out with an ordinary cast command.
func TestReflexMendsTheOwner(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	styleSpells(t, her)
	m, c, _ := strangerModule()
	c.profile.Combat.MendBelow = 60
	c.fight = &fightState{Stance: `fight`}
	events.DrainQueuedInputsForTest(her.InstanceId)
	m.reflex(c, her, owner, room, 100, 90, 30, true)
	got := queuedCasts(her.InstanceId)
	if len(got) != 1 || got[0] != `cast mend-wounds @1` {
		t.Fatalf("expected her to mend him, queued %v", got)
	}
}

// The model can call for a spell in a fight. A harmful one lands only on
// what her owner could harm and never on him; a helping one goes where it
// was asked.
func TestModelCalledSpellsInAFight(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	styleSpells(t, her)
	wolf := harmMob(t, room, 302, `a grey wolf`)
	guard := harmMob(t, room, 300, `a caravan guard`)
	guard.PlayerAttackImmune = true
	m, c, _ := strangerModule()
	c.fight = &fightState{Stance: `fight`, Refs: map[string]int{`e1`: wolf.InstanceId, `e2`: guard.InstanceId}}
	hurl := spellRef(t, her, `kinetic-hurl`)
	mend := spellRef(t, her, `mend-wounds`)

	m.applyCombatProposal(c, CombatProposal{Spell: hurl, SpellAt: `owner`}, owner)
	if c.fight.Spell != nil {
		t.Fatal("a harmful spell is never aimed at her owner")
	}
	m.applyCombatProposal(c, CombatProposal{Spell: hurl, SpellAt: `e1`}, owner)
	if cmd, _ := fightCastCommand(c, her, owner, room, c.fight.Spell); cmd != `cast kinetic-hurl #302` {
		t.Fatalf("at the wolf: %q", cmd)
	}
	m.applyCombatProposal(c, CombatProposal{Spell: hurl, SpellAt: `e2`}, owner)
	if cmd, keep := fightCastCommand(c, her, owner, room, c.fight.Spell); cmd != `` || keep {
		t.Fatalf("never at a creature her owner could not fight: %q keep=%v", cmd, keep)
	}
	m.applyCombatProposal(c, CombatProposal{Spell: mend, SpellAt: `owner`}, owner)
	if cmd, _ := fightCastCommand(c, her, owner, room, c.fight.Spell); cmd != `cast mend-wounds @1` {
		t.Fatalf("a mending on her owner: %q", cmd)
	}
	m.applyCombatProposal(c, CombatProposal{Spell: mend}, owner)
	if cmd, _ := fightCastCommand(c, her, owner, room, c.fight.Spell); cmd != `cast mend-wounds` {
		t.Fatalf("a mending with nobody named is on herself: %q", cmd)
	}

	// Holding back is not joining in.
	c.fight.Stance = `hold_back`
	m.applyCombatProposal(c, CombatProposal{Spell: hurl, SpellAt: `e1`}, owner)
	if cmd, keep := fightCastCommand(c, her, owner, room, c.fight.Spell); cmd != `` || keep {
		t.Fatalf("no harm while holding back: %q", cmd)
	}

	// Busy: the spell waits for a better moment.
	c.fight.Stance = `fight`
	m.applyCombatProposal(c, CombatProposal{Spell: hurl, SpellAt: `e1`}, owner)
	her.Character.Cooldowns = characters.Cooldowns{`special-move`: 3}
	if cmd, keep := fightCastCommand(c, her, owner, room, c.fight.Spell); cmd != `` || !keep {
		t.Fatalf("it should wait out the cooldown: %q keep=%v", cmd, keep)
	}
}

// Only the moves that suit her are taken from the model.
func TestOnlyMovesThatSuitHer(t *testing.T) {
	owner, _, _, _ := harmWorld(t, configs.PVPDisabled)
	m, c, _ := strangerModule()
	c.profile.Combat.Moves = []string{`trip`}
	c.fight = &fightState{Stance: `fight`}
	m.applyCombatProposal(c, CombatProposal{Move: `bash`}, owner)
	if c.fight.Move != `` {
		t.Fatal("a healer is not sent to bash")
	}
	m.applyCombatProposal(c, CombatProposal{Move: `trip`}, owner)
	if c.fight.Move != `trip` {
		t.Fatal("but may trip")
	}
	c.profile.Combat.Moves = []string{}
	m.applyCombatProposal(c, CombatProposal{Move: `kick`}, owner)
	if c.fight.Move != `trip` {
		t.Fatal("someone with no moves at all is offered none")
	}
}

func hide(t *testing.T, mob *mobs.Mob) {
	t.Helper()
	mob.Character.Awareness = awareness.NewMachine()
	r := state.TransitionReason{Trigger: `test_setup`}
	if err := mob.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r); err != nil {
		t.Fatal(err)
	}
	mob.Character.Awareness.ResolveConcealment(true, r)
	if !mob.Character.IsHidden() {
		t.Fatal("fixture: she should be hidden")
	}
}

// From hiding, a skirmisher's first blow goes at whatever is on her owner,
// as an ordinary attack the engine turns into a surprise.
func TestSurpriseOpenerFromHiding(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	wolf := harmMob(t, room, 302, `a grey wolf`)
	wolf.Character.SetAggro(owner.UserId, 0, characters.DefaultAttack)
	m, c, _ := strangerModule()
	c.profile.Combat.Opener = `surprise`
	c.fight = &fightState{Stance: `fight`}
	enemies := map[int]string{wolf.InstanceId: wolf.Character.Name}
	events.DrainQueuedInputsForTest(her.InstanceId)

	if m.surpriseOpener(c, her, owner, room, 10, enemies) {
		t.Fatal("not hidden, no surprise")
	}
	hide(t, her)
	if !m.surpriseOpener(c, her, owner, room, 10, enemies) {
		t.Fatal("hidden, she strikes first")
	}
	got := events.DrainQueuedInputsForTest(her.InstanceId)
	if len(got) != 1 || got[0] != `attack #302` {
		t.Fatalf("queued %v", got)
	}
	c.profile.Combat.Opener = ``
	if m.surpriseOpener(c, her, owner, room, 10, enemies) {
		t.Fatal("a companion who does not fight that way does not")
	}
}

// Sneaking is a thief's and a scout's, and goes through the engine's own
// sneak.
func TestSneakIsForThoseWhoKnowHow(t *testing.T) {
	owner, _, _, her := harmWorld(t, configs.PVPDisabled)
	m, c, _ := strangerModule()
	sc := harmScene(0, 2)
	owners := []stimulus{{Kind: `heard`, FromOwner: true}}
	c.profile.Archetype.Skills = map[string]float64{`spellcasting`: 1}
	if out := m.performAction(c, her, owner, sc, ActionProposal{Verb: `sneak`}, owners, 0, 0); out.Issued {
		t.Fatalf("a healer does not slip into shadows: %+v", out)
	}
	if strings.Contains(capabilityWordsFor(Config{}, c.profile), `sneak`) {
		t.Fatal("nor is she told she can")
	}
	c.profile.Archetype.Skills = map[string]float64{`skullduggery`: 1}
	events.DrainQueuedInputsForTest(her.InstanceId)
	if out := m.performAction(c, her, owner, sc, ActionProposal{Verb: `sneak`}, owners, 0, 0); !out.Issued {
		t.Fatalf("a thief does: %+v", out)
	}
	if !strings.Contains(capabilityWordsFor(Config{}, c.profile), `sneak`) {
		t.Fatal("and is told so")
	}
}

// A helping spell with nobody named lands on herself, as the engine casts a
// mob's single-target help, rather than being refused.
func TestHelpingSpellWithNoTargetIsOnHerself(t *testing.T) {
	owner, _, _, her := harmWorld(t, configs.PVPDisabled)
	styleSpells(t, her)
	m, c, _ := strangerModule()
	mend := spellRef(t, her, `mend-wounds`)
	owners := []stimulus{{Kind: `heard`, FromOwner: true}}
	for _, to := range []string{``, `self`} {
		out := m.performAction(c, her, owner, harmScene(0, 2), ActionProposal{Verb: `cast`, Ref: mend, To: to}, owners, 0, 0)
		if !out.Issued {
			t.Fatalf("to=%q: %+v", to, out)
		}
	}
}

// The fighting paragraph of the prompt is written for each companion.
func TestCombatRuleIsWrittenForEachCompanion(t *testing.T) {
	profiles, _ := loadProfiles()
	rules := map[string]string{}
	for id, p := range profiles {
		rules[id] = combatRule(p, `Corvin`, len(p.StartingSpells) > 0)
	}
	if !strings.Contains(rules[`liesl`], `You mend Corvin yourself`) || !strings.Contains(rules[`liesl`], `ward Corvin`) {
		t.Errorf("Liesl's rule should say she mends and wards: %s", rules[`liesl`])
	}
	if !strings.Contains(rules[`tobin`], `by surprise`) {
		t.Errorf("Tobin's should speak of striking from hiding: %s", rules[`tobin`])
	}
	if !strings.Contains(rules[`isaura`], `no special moves`) || !strings.Contains(rules[`isaura`], `[m] ref`) {
		t.Errorf("Isaura's should offer spells and no moves: %s", rules[`isaura`])
	}
	if !strings.Contains(rules[`corvel`], `(taunt, bash, rally, warcry, kick)`) {
		t.Errorf("Corvel's should offer a shield man's moves: %s", rules[`corvel`])
	}
	if !strings.Contains(rules[`hal`], `(grapple, trip, kick)`) {
		t.Errorf("Hal's should offer a wrestler's moves: %s", rules[`hal`])
	}
	if strings.Contains(rules[`mara`], `[m] ref`) {
		t.Errorf("Mara knows no spells: %s", rules[`mara`])
	}
}

// An archer whose bow is not yet chambered fires at whoever she is
// fighting; firing chambers it, and her archetype's try_fire takes it from
// there. Without this, a bow handed over by her owner (they all arrive
// empty) was swung as a club for the whole fight.
func TestArcherLoosesTheFirstShot(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	wolf := harmMob(t, room, 302, `a grey wolf`)
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		9901: {ItemId: 9901, Name: `a short bow`, Type: items.Weapon, Subtype: items.Shooting, AmmoTag: `arrow`},
		9902: {ItemId: 9902, Name: `a bundle of arrows`, Type: items.Ammo, AmmoTag: `arrow`, Uses: 10},
	}))
	her.Character.Equipment.Weapon = items.New(9901)
	arrows := items.New(9902)
	arrows.Uses = 10
	her.Character.Items = []items.Item{arrows}
	her.Character.SetAggro(0, wolf.InstanceId, characters.DefaultAttack)
	m, c, _ := strangerModule()
	c.fight = &fightState{Stance: `fight`, Style: `ranged`}
	events.DrainQueuedInputsForTest(her.InstanceId)

	m.reflex(c, her, owner, room, 100, 90, 90, true)
	got := events.DrainQueuedInputsForTest(her.InstanceId)
	if len(got) != 1 || got[0] != `fire #302` {
		t.Fatalf("expected her first shot, queued %v", got)
	}

	her.Character.Equipment.Weapon.Loaded = true
	c.fight.LastReflex = 0
	m.reflex(c, her, owner, room, 200, 90, 90, true)
	if got := events.DrainQueuedInputsForTest(her.InstanceId); len(got) != 0 {
		t.Fatalf("a chambered bow is her archetype's to loose, not the reflex's: %v", got)
	}
}

// A healer goes into a fight holding back. If auto-assist already sent her
// at someone else's foe that round, she steps out again, and her owner's
// assist setting comes back when the fight is over.
func TestHoldingBackStepsOutOfAnAssist(t *testing.T) {
	owner, _, room, her := harmWorld(t, configs.PVPDisabled)
	wolf := harmMob(t, room, 302, `a grey wolf`)
	wolf.Character.SetAggro(owner.UserId, 0, characters.DefaultAttack)
	her.Character.SetAggro(0, wolf.InstanceId, characters.DefaultAttack)
	owner.Character.Companions = []characters.CompanionInfo{{InstanceId: her.InstanceId, AutoAssist: true, SourceType: characters.CompanionBonded}}
	m, c, _ := strangerModule()
	c.fight = &fightState{Stance: `hold_back`}
	if !her.Character.IsInCombat() {
		t.Fatal("fixture: assist has her fighting")
	}
	m.setHoldBack(c, owner, true)
	if her.Character.IsInCombat() {
		t.Fatal("holding back, she steps out of a fight that is not on her")
	}
	if owner.Character.GetCompanionByInstanceId(her.InstanceId).AutoAssist {
		t.Fatal("and does not get sent back in")
	}
	m.setHoldBack(c, owner, false)
	if !owner.Character.GetCompanionByInstanceId(her.InstanceId).AutoAssist {
		t.Fatal("afterwards her owner's setting is put back")
	}

	// One that is on her, she answers.
	wolf.Character.SetAggro(0, her.InstanceId, characters.DefaultAttack)
	her.Character.SetAggro(0, wolf.InstanceId, characters.DefaultAttack)
	c.fight = &fightState{Stance: `hold_back`}
	m.setHoldBack(c, owner, true)
	if !her.Character.IsInCombat() {
		t.Fatal("she still defends herself")
	}
}
