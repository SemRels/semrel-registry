package service

import (
	"fmt"
	"strconv"
	"strings"
)

// Semantic-version handling for the `compatibility.semrelCore` ranges plugins
// declare.
//
// This is implemented here rather than pulled in as a dependency on purpose:
// the accepted grammar is small and already pinned by the published JSON
// schema, and a registry whose job is supply-chain integrity should not add a
// module to compare three integers.

// Version is a parsed semantic version. Build metadata is accepted and ignored,
// as the specification requires.
type Version struct {
	Major, Minor, Patch int
	Prerelease          string
}

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	return s
}

// ParseVersion reads "1.2.3", "v1.2.3", "1.2.3-rc.1" or "1.2.3+build.5".
func ParseVersion(raw string) (Version, error) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "v")
	if text == "" {
		return Version{}, fmt.Errorf("version is empty")
	}

	// Build metadata never affects precedence.
	if idx := strings.IndexByte(text, '+'); idx >= 0 {
		text = text[:idx]
	}

	core, prerelease, _ := strings.Cut(text, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("version %q must have the form major.minor.patch", raw)
	}

	numbers := make([]int, 3)
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("version %q has a non-numeric component %q", raw, part)
		}
		numbers[i] = n
	}

	return Version{Major: numbers[0], Minor: numbers[1], Patch: numbers[2], Prerelease: prerelease}, nil
}

// CompareVersions returns -1, 0 or 1. Prerelease precedence follows semver:
// a version with a prerelease ranks below the same version without one, and
// prerelease identifiers compare numerically when both are numeric.
func CompareVersions(a, b Version) int {
	for _, pair := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}

	switch {
	case a.Prerelease == "" && b.Prerelease == "":
		return 0
	case a.Prerelease == "":
		return 1 // a release outranks a prerelease of the same version
	case b.Prerelease == "":
		return -1
	}
	return comparePrerelease(a.Prerelease, b.Prerelease)
}

func comparePrerelease(a, b string) int {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] == right[i] {
			continue
		}
		leftNum, leftErr := strconv.Atoi(left[i])
		rightNum, rightErr := strconv.Atoi(right[i])

		switch {
		case leftErr == nil && rightErr == nil:
			if leftNum < rightNum {
				return -1
			}
			return 1
		case leftErr == nil:
			return -1 // numeric identifiers rank below alphanumeric ones
		case rightErr == nil:
			return 1
		case left[i] < right[i]:
			return -1
		default:
			return 1
		}
	}

	switch {
	case len(left) == len(right):
		return 0
	case len(left) < len(right):
		return -1
	default:
		return 1
	}
}

// constraint is one comparison within a range.
type constraint struct {
	operator string
	version  Version
}

func (c constraint) allows(v Version) bool {
	cmp := CompareVersions(v, c.version)
	switch c.operator {
	case "", "=", "==":
		return cmp == 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	}
	return false
}

// ParseRange reads a space-separated range such as ">=0.25.0 <1.0.0".
//
// Terms are combined with AND, matching the published schema. The `~` and `^`
// shorthands expand into a pair of bounds:
//
//	~1.2.3 → >=1.2.3 <1.3.0   (patch-level changes)
//	^1.2.3 → >=1.2.3 <2.0.0   (compatible changes)
//	^0.2.3 → >=0.2.3 <0.3.0   (0.x: the minor is the breaking axis)
func ParseRange(raw string) ([]constraint, error) {
	var constraints []constraint

	for _, term := range strings.Fields(raw) {
		operator, rest := splitOperator(term)
		version, err := ParseVersion(rest)
		if err != nil {
			return nil, fmt.Errorf("range %q: %w", raw, err)
		}

		switch operator {
		case "~":
			constraints = append(constraints,
				constraint{">=", version},
				constraint{"<", Version{Major: version.Major, Minor: version.Minor + 1}})
		case "^":
			upper := Version{Major: version.Major + 1}
			if version.Major == 0 {
				upper = Version{Minor: version.Minor + 1}
			}
			constraints = append(constraints, constraint{">=", version}, constraint{"<", upper})
		case "", "=", "==", ">", ">=", "<", "<=":
			constraints = append(constraints, constraint{operator, version})
		default:
			return nil, fmt.Errorf("range %q: unsupported operator %q", raw, operator)
		}
	}

	return constraints, nil
}

func splitOperator(term string) (operator, version string) {
	i := 0
	for i < len(term) && strings.ContainsRune("<>=~^", rune(term[i])) {
		i++
	}
	return term[:i], strings.TrimSpace(term[i:])
}

// SatisfiesRange reports whether version falls inside the declared range.
//
// An empty range means "unconstrained": plugins published before compatibility
// metadata existed carry none, and the README promises they stay valid. Treating
// that as "incompatible" would hide most of the catalogue behind the filter.
func SatisfiesRange(version, rangeExpr string) (bool, error) {
	if strings.TrimSpace(rangeExpr) == "" {
		return true, nil
	}

	parsed, err := ParseVersion(version)
	if err != nil {
		return false, err
	}
	constraints, err := ParseRange(rangeExpr)
	if err != nil {
		return false, err
	}

	for _, c := range constraints {
		if !c.allows(parsed) {
			return false, nil
		}
	}
	return true, nil
}
