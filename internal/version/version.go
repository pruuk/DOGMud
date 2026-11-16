package version

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	Older = -1
	Newer = 1
	Equal = 0
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf(`%d.%d.%d`, v.Major, v.Minor, v.Patch)
}

func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return Older
		}
		return Newer
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return Older
		}
		return Newer
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return Older
		}
		return Newer
	}
	return Equal
}

func (v Version) IsNewerThan(other Version) bool {
	return v.Compare(other) == Newer
}

func (v Version) IsOlderThan(other Version) bool {
	return v.Compare(other) == Older
}

func (v Version) IsEqualTo(other Version) bool {
	return v.Compare(other) == Equal
}

func New(major int, minor int, patch int) Version {
	return Version{major, minor, patch}
}

func Parse(v string) (Version, error) {
	// lowercase it all for predicatability
	s := strings.ToLower(v)

	// Remove leading "v" if present
	s = strings.TrimPrefix(s, "v")

	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, fmt.Errorf("invalid version format: %s", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %w", err)
	}

	patch := 0
	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch version: %w", err)
		}
	}

	if major == 0 && minor == 0 && patch == 0 {
		return Version{}, fmt.Errorf("invalid version: %s", v)
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}
