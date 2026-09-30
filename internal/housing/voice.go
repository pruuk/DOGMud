package housing

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A landlord's voice. Everything a landlord says through the say command
// comes from here: defaultLines holds the lines every landlord says unless
// his building's `voice:` map overrides them, one entry per key. A line may
// use {placeholders}: the building's own words are always available
// ({proprietor}, {Proprietor}, {vouched_by}, {Vouched_by}, {standing_hint},
// {name}, {Name}, {door}), and each key adds its own (defaultLines shows
// which). Validate rejects an unknown key, a placeholder the key does not
// provide, and a semicolon (it ends a spoken command).
var defaultLines = map[string]string{
	// What the landlord says when asked about the terms (DescribeTerms).
	`terms.owner`:        `You've got a room already. {Door}'s behind me, hand on the plate, same as always.`,
	`terms.extension`:    `If you want it bigger, an extension deed is {price} gold for you. Goes up every time, that's {proprietor}'s rule. A redecorating voucher is {voucher}. Type list.`,
	`terms.no_extension`: `No extensions for you just now. {reason}. A redecorating voucher is {voucher}, if you're bored of the walls. Type list.`,
	`terms.guest_key`:    `Want to let a friend in? A guest key's {price}. Give it to them, they use it on the door, done. Type house to see who's got in.`,
	`terms.storage`:      `Somewhere to keep your things? A container deed's {container}, a strongbox deed's {strongbox}. You name it, it turns up. Anyone you let in can use a container. A strongbox opens for you alone.`,
	`terms.furnish`:      `A bed's {bed}, and you'll rest twice as fast in it. A crafting station's {station}, whichever kind you like. One of each to a room, and anyone you let in can use them. Type list.`,
	`terms.full`:         `Every room's let. Nothing I can do. Try again another day.`,
	`terms.pitch`:        `{Tier}, {price} gold, paid once. Four walls, a window, and a door that opens for you and nobody else. Extensions and redecorating come after, for lodgers. It's all on the list.`,
	`terms.not_vouched`:  `But {proprietor} only lets to people {vouched_by} can vouch for, and nobody's vouched for you. {standing_hint}`,
	`terms.no_gold`:      `You'd want the gold first. The bank counts. Type buy home when you've got it.`,
	`terms.ready`:        `{Vouched_by} speaks well enough of you. Type buy home and I'll get the stamp out.`,
	`terms.open`:         `Nobody needs to vouch for you here. Type buy home when you're ready.`,

	// Buying a home (Purchase).
	`home.unknown_building`: `I've no rooms to let just now.`,
	`home.unknown_tier`:     `I don't let that kind of room.`,
	`home.already_owner`:    `You've already got a room. The {door}'s behind me. It knows you. Go on.`,
	`home.not_vouched`:      `{Proprietor} only lets to people {vouched_by} will vouch for, and nobody's vouched for you. {standing_hint} I'll still be here. I'm always here.`,
	`home.no_gold`:          `It's {price} gold for {tier}, and you haven't got it, not even counting the bank. Come back when you have.`,
	`home.held_owner`:       `Your name's in the ledger already, but the page is in a state. {Proprietor}'s clerk is going over it. Nothing I can let you till it's straight.`,
	`home.frozen`:           `The ledger's in a state. {Proprietor}'s clerk is going over it, and I'm to let nothing till it's done. Come back another day.`,
	`home.full`:             `Every room's let. Nothing I can do. Try another day.`,
	`home.welcome`:          `Stamped. Welcome to {name}. Try not to set anything on fire.`,

	// Anything whose record could not be written; nothing was taken.
	`ledger_error`: `The ledger's in a state. I'll not take your coin till it's straight. Come back later.`,

	// Extension deeds (buyExtension).
	`deed.no_home`:   `Extensions are for lodgers. You'd want a home here first. It's on the list.`,
	`deed.frozen`:    `The ledger's in a state. {Proprietor}'s clerk is going over it, and I'm to sell no rooms till it's done.`,
	`deed.unused`:    `You've a deed already, made out to you and not used. Use that one first. One room at a time.`,
	`deed.reissued`:  `Lost your deed? The ledger says you paid for one and never used it. Here's a fresh copy, no charge. Whichever you use first counts, and the other's waste paper after.`,
	`deed.max_rooms`: `The company won't let one lodger have more of the building than you've got. Rules. Not mine.`,
	`deed.no_gold`:   `An extension runs you {price} gold, the bank counting. You're short.`,
	`deed.no_units`:  `No rooms left to add on. Every empty room's promised to somebody's deed already. Try again when somebody moves out.`,
	`deed.too_heavy`: `You're carrying too much to take a sheet of paper. Impressive. Put something down.`,
	`deed.sold`:      `{price} gold. Your name's on it, so don't bother selling it on. Stand in your lodging and type use deed. It'll ask you which way the room goes.`,

	// Redecorating vouchers (buyRedecorate).
	`voucher.no_home`:   `A voucher's no use without a room to spend it on. Get a home first.`,
	`voucher.no_gold`:   `It's {price} gold for a voucher. You haven't got it.`,
	`voucher.too_heavy`: `You're carrying too much to take a slip of paper. Put something down.`,
	`voucher.sold`:      `{price} gold. Use it in whichever room you want to look different, then write what you'd like it to look like. One room, one go.`,

	// Guest keys (buyGuestKey).
	`key.no_home`:   `A key to what? You'd want a home here first.`,
	`key.max`:       `Your lock already knows as many palms as {proprietor} allows. Revoke somebody first. Type house guests.`,
	`key.no_gold`:   `It's {price} gold for a guest key. You haven't got it.`,
	`key.too_heavy`: `You're carrying too much to take a key. Put something down.`,
	`key.sold`:      `{price} gold. Give it to whoever you want let in. They use it at the door, once, and the lock learns them. Type house guests to see who's in, and house revoke to throw them out.`,

	// Container and strongbox deeds (buyContainerDeed). {what} is
	// "container" or "strongbox".
	`storage.no_home`:        `A {what}'s no use without a room to put it in. Get a home first.`,
	`storage.full`:           `Every room you've got is full of furniture already. The company won't allow more to a room. Fire hazard, they say. Get another room.`,
	`storage.carrying`:       `You're carrying deeds enough for every free corner you've got. Use those first.`,
	`storage.no_gold`:        `It's {price} gold for a {what}. You haven't got it.`,
	`storage.too_heavy`:      `You're carrying too much to take a slip of paper. Put something down.`,
	`storage.container_sold`: `{price} gold. Stand where you want it and use the deed. Give it a one-word name when it asks. Mug, chest, whatever. Anyone you let in can use it.`,
	`storage.strongbox_sold`: `{price} gold. Stand where you want it and use the deed. Give it a one-word name when it asks. Only you'll get it open.`,

	// Beds and crafting stations ({what} is "bed" or "crafting station").
	`furnish.no_home`:      `A {what}'s no use without a room to put it in. Get a home first.`,
	`furnish.full`:         `Every room you've got has a {what} already. One to a room. Get another room.`,
	`furnish.carrying`:     `You're carrying a deed for a {what} in every room that hasn't got one. Use those first.`,
	`furnish.no_gold`:      `It's {price} gold for a {what}. You haven't got it.`,
	`furnish.too_heavy`:    `You're carrying too much to take a slip of paper. Put something down.`,
	`furnish.bed_sold`:     `{price} gold. Stand in the room you want it in and use the deed. The porters do the rest. Sleep in it and you'll rest twice as fast as on the floor, and so will anyone you let in.`,
	`furnish.station_sold`: `{price} gold. Stand in the room you want it in and use the deed, and tell the fitters which station you want. One to a room. Anyone you let in can work at it.`,

	// The note beside an available home on the list.
	`list.home_note`: `Four walls and a door that knows you`,

	// An ask that names nothing he sells (behaviortree buy_housing).
	`ask.buy_what`: `Buy what? Type list and I'll show you what {proprietor} sells. Then buy home, buy deed, and so on.`,
}

// linePlaceholders are the placeholders each key provides beyond the
// building's own words; Validate checks a voice override against them.
var linePlaceholders = map[string][]string{
	`terms.extension`: {`price`, `voucher`}, `terms.no_extension`: {`reason`, `voucher`},
	`terms.guest_key`: {`price`}, `terms.storage`: {`container`, `strongbox`},
	`terms.pitch`: {`Tier`, `tier`, `price`}, `home.no_gold`: {`price`, `tier`},
	`deed.no_gold`: {`price`}, `deed.sold`: {`price`}, `voucher.no_gold`: {`price`},
	`voucher.sold`: {`price`}, `key.no_gold`: {`price`}, `key.sold`: {`price`},
	`storage.no_home`: {`what`}, `storage.no_gold`: {`price`, `what`},
	`storage.container_sold`: {`price`}, `storage.strongbox_sold`: {`price`},
	`terms.furnish`: {`bed`, `station`}, `furnish.no_home`: {`what`}, `furnish.full`: {`what`},
	`furnish.carrying`: {`what`}, `furnish.no_gold`: {`price`, `what`},
	`furnish.bed_sold`: {`price`}, `furnish.station_sold`: {`price`},
}

var buildingPlaceholders = []string{`proprietor`, `Proprietor`, `vouched_by`, `Vouched_by`,
	`standing_hint`, `name`, `Name`, `door`, `Door`}

var placeholderRe = regexp.MustCompile(`\{([A-Za-z_]+)\}`)

// Line is what this building's landlord says for key, with its placeholders
// filled from vars (pairs of name, value) and the building's own words.
func (b Building) Line(key string, vars ...any) string {
	text, ok := b.Voice[key]
	if !ok {
		text = defaultLines[key]
	}
	values := map[string]string{
		`proprietor`: b.Proprietor, `Proprietor`: capitalise(b.Proprietor),
		`vouched_by`: b.VouchedBy, `Vouched_by`: capitalise(b.VouchedBy),
		`standing_hint`: b.StandingHint, `name`: b.Name, `Name`: capitalise(b.Name),
		`door`: b.DoorExit, `Door`: capitalise(b.DoorExit),
	}
	for i := 0; i+1 < len(vars); i += 2 {
		values[fmt.Sprint(vars[i])] = fmt.Sprint(vars[i+1])
	}
	return placeholderRe.ReplaceAllStringFunc(text, func(m string) string {
		if v, ok := values[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

// validateVoice checks a building's voice overrides.
func (b Building) validateVoice() error {
	keys := make([]string, 0, len(b.Voice))
	for k := range b.Voice {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		text := b.Voice[key]
		if _, known := defaultLines[key]; !known {
			return fmt.Errorf(`housing building %s: voice.%s is not a line a landlord says`, b.BuildingId, key)
		}
		if strings.TrimSpace(text) == `` {
			return fmt.Errorf(`housing building %s: voice.%s is empty`, b.BuildingId, key)
		}
		if strings.Contains(text, `;`) {
			return fmt.Errorf(`housing building %s: voice.%s must not contain ';' (it ends a spoken command)`, b.BuildingId, key)
		}
		allowed := map[string]bool{}
		for _, p := range buildingPlaceholders {
			allowed[p] = true
		}
		for _, p := range linePlaceholders[key] {
			allowed[p] = true
		}
		for _, m := range placeholderRe.FindAllStringSubmatch(text, -1) {
			if !allowed[m[1]] {
				return fmt.Errorf(`housing building %s: voice.%s uses {%s}, which that line does not provide`, b.BuildingId, key, m[1])
			}
		}
	}
	return nil
}
