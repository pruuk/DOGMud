package behaviortree

// A give hand-back (#375 R8, review M1): a player gave an NPC an item one of
// its talk topics wants while no such topic is eligible, so the item never
// left the player. internal/conversation still offers the give to the mob's
// tree as player_give, marked with EventContext.HandBack, so an authored
// refusal ("See Drunn first, then come back.") plays in place of the generic
// hand-back line. Only a branch written for that item answers: one with an
// item_matches condition naming it. A catch-all give branch ("I have no use
// for this.") is for ordinary gifts and would misread an item the NPC asks
// for. A hand-back credits nothing: a branch that could act beyond speech
// and return_item is skipped whole, before any of it speaks, and any other
// action fails without acting.

// GiveHandBack marks a player_give whose item is still with the player.
// Returned reports that the tree's return_item ran, so the caller does not
// print a second hand-back line.
type GiveHandBack struct {
	Returned bool
}

// handBackActions are the actions a hand-back may run: speech, and
// return_item, which only says the item came back.
var handBackActions = map[string]bool{
	"say":            true,
	"respond":        true,
	"emote":          true,
	"mob_say":        true,
	"mob_emote":      true,
	"speak":          true,
	"send_user_text": true,
	"return_item":    true,
}

// handBackSafe reports whether every action under n is a hand-back action.
// Conditions and composites are safe; a node of an unknown kind is not.
func handBackSafe(n Node) bool {
	switch v := n.(type) {
	case *ActionNode:
		return handBackActions[v.Name]
	case *ConditionNode:
		return true
	case *SelectorNode:
		return allHandBackSafe(v.Children)
	case *SequenceNode:
		return allHandBackSafe(v.Children)
	case *EventFilterNode:
		return handBackSafe(v.Child)
	case *CooldownDecorator:
		return handBackSafe(v.Child)
	case *RepeatDecorator:
		return handBackSafe(v.Child)
	case *InvertDecorator:
		return handBackSafe(v.Child)
	case *RandomDecorator:
		return handBackSafe(v.Child)
	case *DelayDecorator:
		return handBackSafe(v.Child)
	}
	return false
}

// handBackBranch reports whether a give branch may answer a hand-back of
// itemId: it is hand-back safe and names the item in an item_matches
// condition.
func handBackBranch(n Node, itemId int) bool {
	return handBackSafe(n) && namesItem(n, itemId)
}

// namesItem reports whether an item_matches condition under n names itemId.
func namesItem(n Node, itemId int) bool {
	switch v := n.(type) {
	case *ConditionNode:
		return v.Name == "item_matches" && getIntParam(v.Params, "item_id") == itemId
	case *SelectorNode:
		return anyNamesItem(v.Children, itemId)
	case *SequenceNode:
		return anyNamesItem(v.Children, itemId)
	case *EventFilterNode:
		return namesItem(v.Child, itemId)
	case *CooldownDecorator:
		return namesItem(v.Child, itemId)
	case *RepeatDecorator:
		return namesItem(v.Child, itemId)
	case *InvertDecorator:
		return namesItem(v.Child, itemId)
	case *RandomDecorator:
		return namesItem(v.Child, itemId)
	case *DelayDecorator:
		return namesItem(v.Child, itemId)
	}
	return false
}

func anyNamesItem(nodes []Node, itemId int) bool {
	for _, c := range nodes {
		if namesItem(c, itemId) {
			return true
		}
	}
	return false
}

func allHandBackSafe(nodes []Node) bool {
	for _, c := range nodes {
		if !handBackSafe(c) {
			return false
		}
	}
	return true
}
