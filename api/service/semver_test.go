package service

import "testing"

func TestParseVersionAcceptsTheFormsPluginsPublish(t *testing.T) {
	cases := map[string]Version{
		"1.2.3":           {Major: 1, Minor: 2, Patch: 3},
		"v1.2.3":          {Major: 1, Minor: 2, Patch: 3},
		"0.25.0":          {Minor: 25},
		"1.2.3-rc.1":      {Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"},
		"1.2.3+build.5":   {Major: 1, Minor: 2, Patch: 3},
		"1.2.3-rc.1+meta": {Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"},
		"  1.2.3  ":       {Major: 1, Minor: 2, Patch: 3},
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			got, err := ParseVersion(raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestParseVersionRejectsMalformedInput(t *testing.T) {
	for _, raw := range []string{"", "1", "1.2", "1.2.3.4", "a.b.c", "1.-2.3", "latest"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseVersion(raw); err == nil {
				t.Fatalf("expected %q to be rejected", raw)
			}
		})
	}
}

// Precedence rules straight from the specification.
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.0.0-rc.1", "1.0.0", -1}, // a prerelease ranks below the release
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1}, // numeric identifiers compare as numbers
		{"1.0.0-rc", "1.0.0-rc.1", -1},    // fewer identifiers rank lower
		{"1.0.0+build", "1.0.0", 0},       // build metadata is ignored
	}
	for _, c := range cases {
		t.Run(c.a+" vs "+c.b, func(t *testing.T) {
			a, _ := ParseVersion(c.a)
			b, _ := ParseVersion(c.b)
			if got := CompareVersions(a, b); got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestSatisfiesRange(t *testing.T) {
	cases := []struct {
		version, rangeExpr string
		want               bool
	}{
		// The example from the README.
		{"0.25.0", ">=0.25.0 <1.0.0", true},
		{"0.24.9", ">=0.25.0 <1.0.0", false},
		{"1.0.0", ">=0.25.0 <1.0.0", false},
		{"0.99.0", ">=0.25.0 <1.0.0", true},

		// Exact and simple comparisons.
		{"1.2.3", "1.2.3", true},
		{"1.2.4", "1.2.3", false},
		{"1.2.4", ">1.2.3", true},
		{"1.2.3", ">=1.2.3", true},
		{"1.2.2", "<1.2.3", true},

		// Tilde: patch-level changes only.
		{"1.2.9", "~1.2.3", true},
		{"1.3.0", "~1.2.3", false},
		{"1.2.2", "~1.2.3", false},

		// Caret: compatible changes, with 0.x treating minor as breaking.
		{"1.9.0", "^1.2.3", true},
		{"2.0.0", "^1.2.3", false},
		{"0.2.9", "^0.2.3", true},
		{"0.3.0", "^0.2.3", false},

		// No declared range means unconstrained: plugins published before the
		// metadata existed must not vanish behind the filter.
		{"1.2.3", "", true},
		{"1.2.3", "   ", true},
	}

	for _, c := range cases {
		t.Run(c.version+" in "+c.rangeExpr, func(t *testing.T) {
			got, err := SatisfiesRange(c.version, c.rangeExpr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestSatisfiesRangeReportsBadInput(t *testing.T) {
	if _, err := SatisfiesRange("not-a-version", ">=1.0.0"); err == nil {
		t.Fatal("expected an invalid version to be reported")
	}
	if _, err := SatisfiesRange("1.0.0", ">=oops"); err == nil {
		t.Fatal("expected an invalid range to be reported")
	}
	if _, err := SatisfiesRange("1.0.0", "!=1.0.0"); err == nil {
		t.Fatal("expected an unsupported operator to be reported")
	}
}
