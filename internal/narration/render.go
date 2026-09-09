package narration

import (
	"fmt"
	"sort"
	"strings"
)

// Selector names which variant group an event draws from.
//
// It is an OPAQUE STRING on purpose. Whether the groups are ordered (defence's
// weak, normal, heavy) or merely named (itemvoices' on_equip, casting's
// cast_started) is a property of the function that COMPUTES the selector, not
// of the pool it selects. Keeping that distinction outside the core is what
// leaves M4 a parameter flip instead of a core rewrite.
type Selector string

// Variants holds one candidate list per role. Any role may be empty.
//
// Every non-empty role must hold the SAME number of variants, because variant
// N of each role describes the same moment. ValidateVariants enforces that at
// load time; Render refuses to guess at render time.
type Variants struct {
	Actor         []string
	Actee         []string
	Observer      []string
	ActeeObserver []string
}

// Roles is one narrated event as each audience is told it.
//
// The names match messaging.Trio deliberately: rendering-side roles and
// delivery-side roles must not drift apart.
//
// ActeeObserver is observers where the ACTEE is, and is empty whenever the
// participants share a room. It exists for combat-messages' `separate` case,
// where a ranged attacker and defender are not co-located and there are
// genuinely two observer audiences.
type Roles struct {
	Actor         string
	Actee         string
	Observer      string
	ActeeObserver string
}

// roleLists returns the four pools in a fixed order for iteration.
func (v Variants) roleLists() [][]string {
	return [][]string{v.Actor, v.Actee, v.Observer, v.ActeeObserver}
}

// Len is the coordinated variant count, or 0 if the non-empty roles disagree
// or there are none.
//
// Zero means "cannot be coordinated", and every caller treats it as "render
// nothing". Silence is the right answer here rather than a best guess: index N
// cannot mean the same moment in a five-entry pool and a three-entry one, so
// rendering something would be inventing a pairing the author never wrote.
func (v Variants) Len() int {
	n := 0
	for _, pool := range v.roleLists() {
		if len(pool) == 0 {
			continue
		}
		if n == 0 {
			n = len(pool)
			continue
		}
		if len(pool) != n {
			return 0
		}
	}
	return n
}

// Render picks ONE index and applies it to every non-empty role, then
// substitutes tokens.
//
// ONE INDEX FOR ALL ROLES IS THE ENTIRE POINT. Picking per role gives each
// audience a coherent-looking line describing a DIFFERENT moment, which is the
// defect this core exists to make unrepresentable. It shipped twice in this
// codebase before the core existed: melee defence (PR #112) and taunt
// (PR #115), both found long after the fact.
//
// A nil picker means production behaviour, DefaultPicker.
//
// ⚠️ indexOverride does NOT skip the pick. pick(n) is called first and the
// draw is then discarded. This is bug-compatibility with
// items.DefenseOptions.RenderTriad, and it matters: DefaultPicker routes
// through util.Rand, which is GLOBAL engine randomness, so returning early
// would consume one fewer random number and shift every subsequent draw in the
// process.
func Render(v Variants, tokens map[string]string, pick Picker, indexOverride ...int) Roles {
	n := v.Len()
	if n == 0 {
		return Roles{}
	}
	if pick == nil {
		pick = DefaultPicker
	}

	index := pick(n)
	if len(indexOverride) > 0 {
		index = indexOverride[0] % n
		if index < 0 {
			index += n
		}
	}

	at := func(pool []string) string {
		if len(pool) == 0 {
			return ""
		}
		return substitute(pool[index], tokens)
	}

	return Roles{
		Actor:         at(v.Actor),
		Actee:         at(v.Actee),
		Observer:      at(v.Observer),
		ActeeObserver: at(v.ActeeObserver),
	}
}

// substitute replaces every token in one pass.
//
// One Replacer rather than sequential replacements, so a value that happens to
// contain a token spelling cannot be substituted again by a later pass. Names
// are player-supplied in principle, so that is the safe direction.
//
// The keys are sorted longest-first so the result never depends on Go's
// randomised map iteration order. Today's token spellings are brace-delimited
// and none is a prefix of another, so ordering is not currently observable;
// sorting means it stays that way if a future token breaks that property,
// rather than producing output that differs between runs.
func substitute(s string, tokens map[string]string) string {
	if len(tokens) == 0 {
		return s
	}
	keys := make([]string, 0, len(tokens))
	for k := range tokens {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	pairs := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		pairs = append(pairs, k, tokens[k])
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// Role identifies one audience of a narrated event, for stores that need to
// declare which roles they expect to be authored.
type Role uint8

const (
	RoleActor Role = iota
	RoleActee
	RoleObserver
	RoleActeeObserver
)

func (r Role) String() string {
	switch r {
	case RoleActor:
		return "actor"
	case RoleActee:
		return "actee"
	case RoleObserver:
		return "observer"
	case RoleActeeObserver:
		return "acteeObserver"
	}
	return "unknown"
}

// ValidateVariants checks that a pool set can be coordinated: every non-empty
// role holds the same count, that count is at least minVariants, and no
// variant is blank.
//
// The minimum is a PARAMETER because the stores genuinely differ. Defence
// demands 5 and ships 10 to 14; casting ships 3. Applying defence's number to
// casting would fail boot on shipped data without improving a single line of
// text. M4 is where these unify, if they should.
//
// 🔑 PASS `expected` IF YOUR STORE HAS MORE THAN ONE ROLE. Without it this
// function cannot tell a role that is deliberately absent (a buff has no actee)
// from one that went missing (a defence band lost its toroom pool), because
// both look like an empty slice. A three-role store that omits `expected` can
// therefore boot happily while narrating a real event to two audiences and
// SILENCE to the third, which is precisely the defect this whole package
// exists to prevent. Naming the roles you expect turns that into a boot
// failure.
//
// Call this at LOAD time. A store that only finds out at render time gets
// silence in play instead of a boot failure, which is how the taunt store
// shipped an 8/8/6 band that left the room with nothing to say.
func ValidateVariants(v Variants, minVariants int, expected ...Role) error {
	named := []struct {
		name string
		pool []string
	}{
		{"actor", v.Actor},
		{"actee", v.Actee},
		{"observer", v.Observer},
		{"acteeObserver", v.ActeeObserver},
	}

	if len(expected) > 0 {
		want := map[Role]bool{}
		for _, r := range expected {
			want[r] = true
		}
		for i, role := range named {
			present := len(role.pool) > 0
			if want[Role(i)] && !present {
				return fmt.Errorf("role %q is expected but holds no variants", role.name)
			}
			if !want[Role(i)] && present {
				return fmt.Errorf("role %q holds variants but is not expected by this store", role.name)
			}
		}
	}

	n := 0
	for _, role := range named {
		if len(role.pool) == 0 {
			continue
		}
		if n == 0 {
			n = len(role.pool)
		} else if len(role.pool) != n {
			return fmt.Errorf("role %q holds %d variants but a sibling role holds %d; variant N of each role must describe the SAME moment", role.name, len(role.pool), n)
		}
		for i, text := range role.pool {
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("role %q variant %d is empty", role.name, i)
			}
		}
	}

	if n == 0 {
		return fmt.Errorf("no role holds any variants")
	}
	if n < minVariants {
		return fmt.Errorf("every role holds %d variants, need at least %d", n, minVariants)
	}
	return nil
}
