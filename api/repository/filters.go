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
