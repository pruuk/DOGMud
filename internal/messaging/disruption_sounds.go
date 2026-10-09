package messaging

// The lines a reader who sees nothing hears when a spell being woven is
// disrupted (#242, owner ruling R4: disruptions are heard; the quiet weave and
// the focus shift are sight-only). Every disruption path shares them, mob and
// player caster alike, so a blind reader hears one kind of event one way.
const (
	// SoundChantBreaksOff is a broken concentration or an interrupted cast.
	SoundChantBreaksOff = `Someone's chant breaks off.`
	// SoundSpellSputtersOut is a spell that fizzles or falters.
	SoundSpellSputtersOut = `A half-formed spell sputters out.`
)
