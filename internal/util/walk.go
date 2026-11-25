package util

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// WalkDirTolerant is filepath.WalkDir for a tree that something else may be
// changing while the walk runs. Packages test in parallel, and tests create
// and remove temp folders under the repo (for example
// internal/<pkg>/_datafiles/...), so a guard that walks the repo can list an
// entry that is gone by the time it is visited (#953).
//
// An entry other than root that has vanished is skipped:
//   - an error the walk itself reports for it (a failed lstat or read of a
//     listed folder) is dropped when the error is fs.ErrNotExist or the path
//     no longer exists, and fn is not called with it;
//   - an error fn returns for it (say os.ReadFile on a file removed after it
//     was listed) is dropped when the path no longer exists.
//
// Every other error still reaches the caller, and a missing root is an error:
// a walk that silently scanned nothing would pass every guard built on it.
func WalkDirTolerant(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil && walkEntryVanished(root, path, err) {
			return nil
		}
		return walkCallbackResult(root, path, fn(path, d, err))
	})
}

// WalkTolerant is WalkDirTolerant for callers written against filepath.Walk
// and its os.FileInfo callback. The rules are the same.
func WalkTolerant(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil && walkEntryVanished(root, path, err) {
			return nil
		}
		return walkCallbackResult(root, path, fn(path, info, err))
	})
}

// walkEntryVanished reports whether err, raised by the walk for path, means
// path was removed after its folder was listed.
func walkEntryVanished(root, path string, err error) bool {
	if filepath.Clean(path) == filepath.Clean(root) {
		return false
	}
	return errors.Is(err, fs.ErrNotExist) || walkPathGone(path)
}

// walkCallbackResult drops an error the callback returned for a path that no
// longer exists, and passes every other result (SkipDir and SkipAll included)
// through unchanged.
func walkCallbackResult(root, path string, err error) error {
	if err == nil || err == filepath.SkipDir || err == filepath.SkipAll {
		return err
	}
	if filepath.Clean(path) != filepath.Clean(root) && walkPathGone(path) {
		return nil
	}
	return err
}

// walkPathGone reports whether path no longer exists.
func walkPathGone(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, fs.ErrNotExist)
}
