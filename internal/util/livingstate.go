package util

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The living-state persistence contract (roadmap chunk 2.1).
//
// "Living state" is the world's accumulated, unreproducible data: mob instance
// progression, shop economies, guilds, bans, petitions, room instances. It is
// categorically different from authored content, which can always be reloaded
// from the repository. Losing living state is not recoverable by rebooting.
//
// The contract has three rules.
//
//  1. WRITE ATOMICALLY AND DURABLY. Go through SafeSave (or Save, which
//     defaults to it). It writes a sibling temp file, flushes it to disk, then
//     renames over the target. A rename within a directory is atomic, so a
//     reader sees either the whole old file or the whole new one, never a
//     half-written mixture.
//
//  2. NEVER CONFLATE ABSENT WITH CORRUPT. Read through ReadLivingState, which
//     reports ErrStateAbsent and ErrStateCorrupt as distinct errors. This is
//     the rule every store was breaking: absent legitimately means "first run,
//     seed defaults", so treating a corrupt file as absent silently reseeds and
//     destroys the real data (review findings 5, 6, 7, 15).
//
//  3. ON CORRUPTION, QUARANTINE THEN CONTINUE. Call QuarantineCorrupt to move
//     the bad file aside, log at ERROR so it is visible in production, and
//     start that entity from defaults. Nothing is deleted, the file can be
//     inspected or hand-repaired later, and one bad byte on the droplet does
//     not take the game offline. (Policy chosen 2026-08-10; the alternative,
//     refusing to boot, was rejected because it turns a single corrupt file
//     into an outage.)
//
// A fourth rule has no helper because it is an ordering discipline, not a
// function: PERSIST BEFORE PUBLISHING. Build the new state as a value, write
// it, and only mutate the in-memory registry once the write returned nil.
// Guild joins, bans and unbans currently mutate memory first and discard the
// save error, so memory and disk diverge and an unban silently returns after
// a restart (finding 7).

var (
	// ErrStateAbsent means the file does not exist. For living state this is
	// normal and expected on first run: the caller should seed defaults.
	ErrStateAbsent = errors.New("living state absent")

	// ErrStateCorrupt means the file exists but its contents could not be
	// obtained or understood. The caller must NOT treat this as absent. It
	// should quarantine the file, log at ERROR, and then seed defaults.
	//
	// Callers that unmarshal should also wrap their own decode failures in
	// this, so "unreadable" and "unparseable" reach the same handling.
	ErrStateCorrupt = errors.New("living state corrupt")
)

// ReadLivingState reads a living-state file, distinguishing absent from
// corrupt.
//
// Returns the file contents on success; ErrStateAbsent when the file does not
// exist; ErrStateCorrupt when it exists but cannot be read. Both errors wrap
// the underlying cause, so errors.Is works and the original is still printable.
func ReadLivingState(path string) ([]byte, error) {
	path = filepath.FromSlash(path)

	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrStateAbsent, path)
	}
	return nil, fmt.Errorf("%w: %s: %v", ErrStateCorrupt, path, err)
}

// QuarantineCorrupt moves a corrupt living-state file aside and returns the
// path it was moved to.
//
// It never deletes: the bytes are preserved for inspection or hand repair. The
// quarantine name keeps the original filename so the two are obviously related,
// and carries a timestamp so repeated corruption of the same file cannot
// overwrite the earlier evidence.
//
// After a successful call the original path reads as ErrStateAbsent, which is
// exactly what lets the caller fall through to its normal seed-defaults path.
func QuarantineCorrupt(path string) (string, error) {
	path = filepath.FromSlash(path)

	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("quarantine %s: %w", path, err)
	}

	// Second granularity is not enough: a load loop can hit two corrupt reads
	// of the same path inside one second, and the second must not clobber the
	// first. Nanoseconds plus a collision loop makes that safe.
	base := fmt.Sprintf("%s.corrupt-%s", path, time.Now().UTC().Format("20060102T150405.000000000Z"))
	dest := base
	for i := 1; ; i++ {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			break
		}
		dest = fmt.Sprintf("%s.%d", base, i)
	}

	if err := os.Rename(path, dest); err != nil {
		return "", fmt.Errorf("quarantine %s: %w", path, err)
	}
	return dest, nil
}
