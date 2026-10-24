package items

// Viewer-aware accessors (owner ruling 2026-09-29). A bauble named on a
// player's own key while the server could not moderate it is finder-only:
// every viewer-agnostic accessor (GetSpec, Name, NameSimple, DisplayName,
// NameComplex, GetLongDescription, and so every template and GMCP payload
// built on them) shows the generic trinket, and these show its own text to
// its finder alone. Call them only where the output reaches that one
// viewer: the guard (internal/guards/bauble_finder_view_guard_test.go) lists
// every caller. For any other item each is its viewer-agnostic twin.

// GetSpecFor is GetSpec as viewerUserId sees it.
func (i *Item) GetSpecFor(viewerUserId int) ItemSpec {
	if i.Spec != nil || i.Bauble == `` {
		return i.GetSpec()
	}
	iSpec := GetItemSpec(i.ItemId)
	if iSpec == nil {
		iSpec = &ItemSpec{}
	}
	return baubleSpecFor(*iSpec, i.Bauble, viewerUserId)
}

// DisplayNameFor is DisplayName as viewerUserId sees it.
func (i *Item) DisplayNameFor(viewerUserId int) string {
	if i.Bauble == `` {
		return i.DisplayName()
	}
	return i.displayNameFrom(i.GetSpecFor(viewerUserId))
}

// NameFor is Name as viewerUserId sees it.
func (i *Item) NameFor(viewerUserId int) string {
	if i.Bauble == `` || i.ItemId < 1 {
		return i.Name()
	}
	return i.GetSpecFor(viewerUserId).Name
}

// LongDescriptionFor is GetLongDescription as viewerUserId sees it.
func (i *Item) LongDescriptionFor(viewerUserId int) string {
	if i.Bauble == `` {
		return i.GetLongDescription()
	}
	return i.longDescriptionFrom(i.GetSpecFor(viewerUserId))
}
