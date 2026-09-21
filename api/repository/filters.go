package repository

import (
	"fmt"
	"strings"
)

type Filter interface {
	ApplyTo(*strings.Builder, *[]interface{})
}

type CategoryFilter struct {
	Category string
}

func (f CategoryFilter) ApplyTo(builder *strings.Builder, args *[]interface{}) {
	if builder == nil || args == nil {
		return
	}
	category := strings.TrimSpace(f.Category)
	if category == "" {
		return
	}

	*args = append(*args, category)
	builder.WriteString(fmt.Sprintf(" AND category = $%d", len(*args)))
}

type SearchFilter struct {
	Query string
}

// ApplyTo matches the search term against the weighted tsvector and, as a
// fallback, against name and description as trigrams.
//
// Full-text search alone is not enough for a plugin registry: names are
// hyphenated identifiers like "provider-github" that tokenise poorly, and a
// search box sees partial words as they are typed. The trigram half covers
// that, and both halves are index-backed — the previous four-column
// "ILIKE '%term%'" could only ever be a sequential scan.
func (f SearchFilter) ApplyTo(builder *strings.Builder, args *[]interface{}) {
	if builder == nil || args == nil {
		return
	}
	query := strings.TrimSpace(f.Query)
	if query == "" {
		return
	}

	*args = append(*args, query)
	termIndex := len(*args)
	*args = append(*args, "%"+query+"%")
	likeIndex := len(*args)

	// websearch_to_tsquery never errors on user input — unbalanced quotes and
	// stray operators are treated as text rather than raising, which a search
	// box needs.
	builder.WriteString(fmt.Sprintf(
		" AND (search_vector @@ websearch_to_tsquery('simple', $%[1]d)"+
			" OR name ILIKE $%[2]d"+
			" OR description ILIKE $%[2]d"+
			" OR author ILIKE $%[2]d"+
			" OR repository ILIKE $%[2]d"+
			" OR EXISTS (SELECT 1 FROM plugin_aliases WHERE plugin_id = plugins.id AND alias ILIKE $%[2]d))",
		termIndex, likeIndex))
}

// RelevanceOrder returns the ORDER BY clause that ranks full-text matches
// first, appending the search term to args. It is used when the caller did not
// ask for an explicit sort: answering a search in alphabetical order buries the
// best match.
func RelevanceOrder(query string, args *[]interface{}) string {
	query = strings.TrimSpace(query)
	if query == "" || args == nil {
		return ""
	}
	*args = append(*args, query)
	return fmt.Sprintf(
		" ORDER BY ts_rank(search_vector, websearch_to_tsquery('simple', $%d)) DESC, name ASC",
		len(*args))
}

// AuthorFilter restricts results to plugins owned by a specific author (exact, case-insensitive).
type AuthorFilter struct {
	Author string
}

func (f AuthorFilter) ApplyTo(builder *strings.Builder, args *[]interface{}) {
	if builder == nil || args == nil {
		return
	}
	author := strings.TrimSpace(f.Author)
	if author == "" {
		return
	}
	*args = append(*args, author)
	builder.WriteString(fmt.Sprintf(" AND LOWER(author) = LOWER($%d)", len(*args)))
}

// StatusFilter restricts results to plugins with a specific status.
// When statuses is empty, no filter is applied (all statuses).
type StatusFilter struct {
	Statuses []string // e.g. ["active"] or ["pending", "rejected"]
}

func (f StatusFilter) ApplyTo(builder *strings.Builder, args *[]interface{}) {
	if builder == nil || args == nil || len(f.Statuses) == 0 {
		return
	}
	if len(f.Statuses) == 1 {
		*args = append(*args, f.Statuses[0])
		builder.WriteString(fmt.Sprintf(" AND status = $%d", len(*args)))
		return
	}
	placeholders := make([]string, len(f.Statuses))
	for i, s := range f.Statuses {
		*args = append(*args, s)
		placeholders[i] = fmt.Sprintf("$%d", len(*args))
	}
	builder.WriteString(fmt.Sprintf(" AND status IN (%s)", strings.Join(placeholders, ",")))
}

// NamespaceFilter restricts results to plugins belonging to a specific namespace (exact, case-insensitive).
type NamespaceFilter struct {
	Namespace string // e.g. "@semrel"
}

func (f NamespaceFilter) ApplyTo(builder *strings.Builder, args *[]interface{}) {
	if builder == nil || args == nil {
		return
	}
	ns := strings.TrimSpace(f.Namespace)
	if ns == "" {
		return
	}
	*args = append(*args, ns)
	builder.WriteString(fmt.Sprintf(" AND LOWER(namespace) = LOWER($%d)", len(*args)))
}

type SortFilter struct {
	Field     string
	Direction string
}

func (f SortFilter) ApplyTo(builder *strings.Builder, _ *[]interface{}) {
	if builder == nil {
		return
	}

	field := normalizeSortField(f.Field)
	if field == "" {
		field = "name"
	}

	direction := strings.ToUpper(strings.TrimSpace(f.Direction))
	if direction != "DESC" {
		direction = "ASC"
	}

	builder.WriteString(fmt.Sprintf(" ORDER BY %s %s", sortColumnExpr(field), direction))
}

// normalizeSortField maps a caller-supplied sort field onto the canonical
// logical name, or "" when the field is not sortable.
//
// It returns a logical name rather than a SQL fragment because both backends
// consume it: the file backend matched the result against plain field names and
// silently fell back to sorting by name when handed SQL.
func normalizeSortField(field string) string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "name":
		return "name"
	case "category":
		return "category"
	case "created_at", "createdat":
		return "created_at"
	case "updated_at", "updatedat":
		return "updated_at"
	case "downloads":
		return "downloads"
	case "views":
		return "views"
	default:
		return ""
	}
}

// sortColumnExpr renders a logical sort field as a SQL expression. Only fields
// that survived normalizeSortField reach this, so no caller input is
// interpolated into the query.
func sortColumnExpr(field string) string {
	switch field {
	case "downloads":
		return "COALESCE(downloads, 0)"
	case "views":
		return "COALESCE(views, 0)"
	default:
		return field
	}
}
