package util

import "strings"

// NounNumberVariants lists the other singular/plural spellings of a typed
// noun, in the order callers should try them (the exact spelling is the
// caller's first try): "-es" removed or added, "-ies" to "-y", then plain "-s"
// removed or added, then "-y" to "-ies". A word already ending in "s" never
// gains an "s" and a word ending in "ss" never loses one, so "mos" does not
// reach "moss". Only the end of the string changes, so a multi-word phrase
// varies in its last word. The input is used as given; lower-case it first.
//
// Room noun lookup (rooms.FindNoun) and quest trigger noun filters
// (questengine) both use this, so a noun a player can look at is a noun a
// quest trigger can see.
func NounNumberVariants(word string) []string {
	var out []string
	if strings.HasSuffix(word, "es") {
		out = append(out, strings.TrimSuffix(word, "es"))
	} else {
		out = append(out, word+"es")
	}
	if strings.HasSuffix(word, "ies") {
		out = append(out, strings.TrimSuffix(word, "ies")+"y")
	}
	if strings.HasSuffix(word, "s") {
		if !strings.HasSuffix(word, "ss") {
			out = append(out, strings.TrimSuffix(word, "s"))
		}
	} else {
		out = append(out, word+"s")
	}
	if strings.HasSuffix(word, "y") {
		out = append(out, strings.TrimSuffix(word, "y")+"ies")
	}
	return out
}

// NounHyphenFold treats a hyphen as a space. Room text highlights a noun as
// `nave-lamps` while its key is `nave lamps`, so noun matching (rooms.FindNoun,
// rooms.Room.NounNamedBy, quest trigger nouns) folds both the typed text and
// the keys through this before comparing.
func NounHyphenFold(s string) string {
	return strings.ReplaceAll(s, "-", " ")
}

// NounsDifferOnlyByNumber reports whether a and b are different spellings of
// one noun's number: b is one of a's NounNumberVariants or the reverse.
func NounsDifferOnlyByNumber(a, b string) bool {
	if a == "" || b == "" || a == b {
		return false
	}
	for _, v := range NounNumberVariants(a) {
		if v == b {
			return true
		}
	}
	for _, v := range NounNumberVariants(b) {
		if v == a {
			return true
		}
	}
	return false
}
