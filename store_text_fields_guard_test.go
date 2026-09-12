package main

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The Kind B stores own their text. Every buff, spell and quest-reward line is
// rendered through the store's Narrate door, so a site never reads a field
// and never decides for itself which audience a line is for. This guard keeps
// it that way: outside the owning package, no production file may name the
// fields. Quest ACTION fields (ActionDef.SendText / RoomText) are exempt:
// questengine.ExecuteAction tests them to dispatch, and the spellings are too
// common (user.SendText) to match safely.
var storeTextFieldOwners = []struct {
	owner   string
	pattern *regexp.Regexp
	what    string
}{
	{"internal/buffs", regexp.MustCompile(`\.(StartUserText|StartRoomText|TriggerUserText|TriggerRoomText|EndUserText|EndRoomText)\b`), "buff text fields"},
	{"internal/spells", regexp.MustCompile(`\.(CastUserText|CastRoomText|WaitUserText|WaitRoomText|MagicUserText|MagicRoomText)\b`), "spell text fields"},
	{"internal/quests", regexp.MustCompile(`Rewards\.(PlayerMessage|RoomMessage)\b`), "quest reward messages"},
}

func TestStoreTextFieldsAreReadOnlyByTheirStore(t *testing.T) {
	type hit struct {
		file string
		line int
		text string
	}
	for _, owner := range storeTextFieldOwners {
		var outside []hit
		inside := 0
		for _, root := range messagingSurfaceGoRoots {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				src, rerr := os.ReadFile(path)
				if rerr != nil {
					return rerr
				}
				rel := filepath.ToSlash(path)
				sc := bufio.NewScanner(bytes.NewReader(src))
				sc.Buffer(make([]byte, 1024*1024), 1024*1024)
				n := 0
				for sc.Scan() {
					n++
					if !owner.pattern.MatchString(sc.Text()) {
						continue
					}
					if strings.HasPrefix(rel, owner.owner+"/") {
						inside++
					} else {
						outside = append(outside, hit{rel, n, strings.TrimSpace(sc.Text())})
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walk %s: %v", root, err)
			}
		}
		// The pattern must be proven capable of matching, or an empty
		// "outside" list proves nothing.
		if inside == 0 {
			t.Errorf("%s: pattern matched nothing inside %s; the guard is blind, not the tree clean", owner.what, owner.owner)
		}
		sort.Slice(outside, func(i, j int) bool {
			if outside[i].file != outside[j].file {
				return outside[i].file < outside[j].file
			}
			return outside[i].line < outside[j].line
		})
		for _, h := range outside {
			t.Errorf("%s read outside %s: %s:%d: %s\n  render through the store's Narrate (or AuthoredStartLine) instead", owner.what, owner.owner, h.file, h.line, h.text)
		}
	}
}
