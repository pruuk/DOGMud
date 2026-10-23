package behaviortree

// SelectorNode tries children in order, returns Success on first
// success, Failure if all fail. Like an OR gate.
type SelectorNode struct {
	Children []Node
}

func (n *SelectorNode) Evaluate(ctx *EvalContext) Result {
	for _, child := range n.Children {
		result := child.Evaluate(ctx)
		if result == Success || result == Running {
			return result
		}
	}
	return Failure
}

// SequenceNode runs children in order, returns Failure on first
// failure, Success if all succeed. Like an AND gate.
type SequenceNode struct {
	Children []Node
}

func (n *SequenceNode) Evaluate(ctx *EvalContext) Result {
	for _, child := range n.Children {
		result := child.Evaluate(ctx)
		if result == Failure {
			return Failure
		}
		if result == Running {
			return Running
		}
	}
	return Success
}

// EventFilterNode wraps a child and only evaluates it when the
// event type matches. Returns Failure on mismatch (skips branch).
// A give hand-back also skips, before any of it runs, a branch that does
// not name the item or could act beyond speech and return_item
// (give_handback.go).
type EventFilterNode struct {
	EventType string
	Child     Node
}

func (n *EventFilterNode) Evaluate(ctx *EvalContext) Result {
	if ctx.Event.EventType != n.EventType {
		return Failure
	}
	if ctx.Event.HandBack != nil && !handBackBranch(n.Child, ctx.Event.ItemId) {
		return Failure
	}
	return n.Child.Evaluate(ctx)
}
