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

// Search must be index-backed on both halves: the tsvector for whole words and
// trigrams for the partial identifiers a search box sees while typing.
func TestSearchFilterUsesFullTextAndTrigrams(t *testing.T) {
	var builder strings.Builder
	var args []interface{}

	SearchFilter{Query: "github"}.ApplyTo(&builder, &args)

	clause := builder.String()
	if !strings.Contains(clause, "search_vector @@ websearch_to_tsquery") {
		t.Fatalf("expected a full-text predicate, got %q", clause)
	}
	if !strings.Contains(clause, "name ILIKE") {
		t.Fatalf("expected a trigram fallback on name, got %q", clause)
	}
	if len(args) != 2 || args[0] != "github" || args[1] != "%github%" {
		t.Fatalf("unexpected arguments: %#v", args)
	}
}

func TestSearchFilterIgnoresBlankQuery(t *testing.T) {
	var builder strings.Builder
	var args []interface{}

	SearchFilter{Query: "   "}.ApplyTo(&builder, &args)

	if builder.String() != "" || len(args) != 0 {
		t.Fatalf("a blank search must add no predicate, got %q / %#v", builder.String(), args)
	}
}

// The search term is always a bound parameter, never interpolated.
func TestSearchFilterBindsTheTerm(t *testing.T) {
	var builder strings.Builder
	var args []interface{}

	SearchFilter{Query: "'; DROP TABLE plugins--"}.ApplyTo(&builder, &args)

	if strings.Contains(builder.String(), "DROP TABLE") {
		t.Fatalf("search term was interpolated into the query: %q", builder.String())
	}
}

func TestRelevanceOrderRanksMatchesFirst(t *testing.T) {
	var args []interface{}

	order := RelevanceOrder("github", &args)

	if !strings.Contains(order, "ts_rank") {
		t.Fatalf("expected relevance ranking, got %q", order)
	}
	if len(args) != 1 || args[0] != "github" {
		t.Fatalf("unexpected arguments: %#v", args)
	}
}

func TestRelevanceOrderIsEmptyWithoutASearch(t *testing.T) {
	var args []interface{}

	if order := RelevanceOrder("", &args); order != "" {
		t.Fatalf("expected no ordering clause, got %q", order)
	}
}
