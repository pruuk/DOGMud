package util

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// walkFixture builds root/a.txt, root/b/ (holding inner.txt), root/c.txt and
// root/d.txt. A walk visits them in that lexical order, so a callback on
// a.txt can remove b/ or c.txt after the walk has listed root and before it
// visits them: the exact race #953 saw, made deterministic.
func walkFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"a.txt", filepath.Join("b", "inner.txt"), "c.txt", "d.txt"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func relNames(t *testing.T, root string, paths []string) []string {
	t.Helper()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
}

func equalNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A folder removed between the listing of its parent and its own read is
// skipped, and the walk goes on to the entries after it.
func TestWalkDirTolerant_SkipsAFolderRemovedMidWalk(t *testing.T) {
	root := walkFixture(t)
	var read []string
	err := WalkDirTolerant(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "a.txt" {
			if rerr := os.RemoveAll(filepath.Join(root, "b")); rerr != nil {
				t.Fatal(rerr)
			}
		}
		if _, rerr := os.ReadFile(path); rerr != nil {
			return rerr
		}
		read = append(read, path)
		return nil
	})
	if err != nil {
		t.Fatalf("a folder that vanished mid-walk failed the walk: %v", err)
	}
	if got, want := relNames(t, root, read), []string{"a.txt", "c.txt", "d.txt"}; !equalNames(got, want) {
		t.Fatalf("read %v, want %v", got, want)
	}
}

// A file removed after it was listed makes the callback's own read fail;
// that error is dropped because the file is gone, and d.txt is still read.
func TestWalkDirTolerant_SkipsAFileRemovedBeforeTheCallbackReadsIt(t *testing.T) {
	root := walkFixture(t)
	var read []string
	err := WalkDirTolerant(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "a.txt" {
			if rerr := os.Remove(filepath.Join(root, "c.txt")); rerr != nil {
				t.Fatal(rerr)
			}
		}
		if _, rerr := os.ReadFile(path); rerr != nil {
			return rerr
		}
		read = append(read, path)
		return nil
	})
	if err != nil {
		t.Fatalf("a file that vanished mid-walk failed the walk: %v", err)
	}
	if got, want := relNames(t, root, read), []string{"a.txt", "b/inner.txt", "d.txt"}; !equalNames(got, want) {
		t.Fatalf("read %v, want %v", got, want)
	}
}

// filepath.Walk lstats every listed name, so a removed file reaches the
// callback as an error with a nil FileInfo.
func TestWalkTolerant_SkipsAFileRemovedMidWalk(t *testing.T) {
	root := walkFixture(t)
	var read []string
	err := WalkTolerant(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == "a.txt" {
			if rerr := os.Remove(filepath.Join(root, "c.txt")); rerr != nil {
				t.Fatal(rerr)
			}
			if rerr := os.RemoveAll(filepath.Join(root, "b")); rerr != nil {
				t.Fatal(rerr)
			}
		}
		read = append(read, path)
		return nil
	})
	if err != nil {
		t.Fatalf("entries that vanished mid-walk failed the walk: %v", err)
	}
	if got, want := relNames(t, root, read), []string{"a.txt", "d.txt"}; !equalNames(got, want) {
		t.Fatalf("read %v, want %v", got, want)
	}
}

// Real errors still fail the walk: a callback error for a path that exists,
// a not-exist error the callback raised about some other file, and a root
// that does not exist.
func TestWalkDirTolerant_StillReportsRealErrors(t *testing.T) {
	root := walkFixture(t)
	boom := errors.New("boom")
	err := WalkDirTolerant(root, func(path string, d fs.DirEntry, err error) error {
		if filepath.Base(path) == "c.txt" {
			return boom
		}
		return err
	})
	if !errors.Is(err, boom) {
		t.Fatalf("a callback error on a live path was lost: got %v", err)
	}

	err = WalkDirTolerant(root, func(path string, d fs.DirEntry, err error) error {
		if filepath.Base(path) == "c.txt" {
			_, rerr := os.ReadFile(filepath.Join(root, "no-such-file.txt"))
			return rerr
		}
		return err
	})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a not-exist error about another file was swallowed: got %v", err)
	}

	missing := filepath.Join(root, "no-such-root")
	if err := WalkDirTolerant(missing, func(path string, d fs.DirEntry, err error) error { return err }); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("WalkDirTolerant on a missing root: got %v, want a not-exist error", err)
	}
	if err := WalkTolerant(missing, func(path string, info os.FileInfo, err error) error { return err }); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("WalkTolerant on a missing root: got %v, want a not-exist error", err)
	}
}
