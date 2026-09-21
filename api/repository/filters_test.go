package repository

import (
	"strings"
	"testing"
)

// The ORDER BY clause is built by string concatenation, so anything reaching it
// must come from the fixed set of known columns rather than from the request.
func TestSortFilterRejectsUnknownFields(t *testing.T) {
	for name, field := range map[string]string{
		"sql injection": "name; DROP TABLE plugins--",
		"subquery":      "(SELECT 1)",
		"unknown":       "author",
	} {
		t.Run(name, func(t *testing.T) {
			var builder strings.Builder
			var args []interface{}
			SortFilter{Field: field, Direction: "DESC"}.ApplyTo(&builder, &args)

			if got := builder.String(); got != " ORDER BY name DESC" {
				t.Fatalf("unsafe field reached the query: %q", got)
			}
		})
	}
}

func TestSortFilterMapsPopularityColumns(t *testing.T) {
	for field, want := range map[string]string{
		"downloads":  " ORDER BY COALESCE(downloads, 0) DESC",
		"views":      " ORDER BY COALESCE(views, 0) DESC",
		"name":       " ORDER BY name DESC",
		"created_at": " ORDER BY created_at DESC",
	} {
		t.Run(field, func(t *testing.T) {
			var builder strings.Builder
			var args []interface{}
			SortFilter{Field: field, Direction: "desc"}.ApplyTo(&builder, &args)

			if got := builder.String(); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
