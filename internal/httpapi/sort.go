package httpapi

import (
	"strings"

	"gorm.io/gorm"
)

func applySort(query *gorm.DB, field string, order string, allowed map[string]string, fallback string) *gorm.DB {
	return query.Order(sortClause(field, order, allowed, fallback))
}

func sortClause(field string, order string, allowed map[string]string, fallback string) string {
	column := allowed[strings.TrimSpace(field)]
	if column == "" {
		column = fallback
	}
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(order), "asc") {
		direction = "ASC"
	}
	return column + " " + direction
}
