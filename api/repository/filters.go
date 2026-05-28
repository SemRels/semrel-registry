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

func (f SearchFilter) ApplyTo(builder *strings.Builder, args *[]interface{}) {
	if builder == nil || args == nil {
		return
	}
	query := strings.TrimSpace(f.Query)
	if query == "" {
		return
	}

	*args = append(*args, "%"+query+"%")
	builder.WriteString(fmt.Sprintf(" AND (name ILIKE $%[1]d OR description ILIKE $%[1]d OR author ILIKE $%[1]d OR repository ILIKE $%[1]d)", len(*args)))
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

	builder.WriteString(fmt.Sprintf(" ORDER BY %s %s", field, direction))
}

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
	default:
		return ""
	}
}
