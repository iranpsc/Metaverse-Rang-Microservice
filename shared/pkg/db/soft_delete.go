package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// SoftDeleteQuery helps build queries that respect soft deletes
type SoftDeleteQuery struct {
	baseQuery    string
	tableName    string
	deleteColumn string
	params       []interface{}
	whereClause  []string
}

// NewSoftDeleteQuery creates a new soft delete query builder
func NewSoftDeleteQuery(baseQuery, tableName string) *SoftDeleteQuery {
	return &SoftDeleteQuery{
		baseQuery:    baseQuery,
		tableName:    tableName,
		deleteColumn: "deleted_at",
		whereClause:  []string{},
		params:       []interface{}{},
	}
}

// WithDeleteColumn sets custom soft delete column name (default: deleted_at)
func (q *SoftDeleteQuery) WithDeleteColumn(column string) *SoftDeleteQuery {
	q.deleteColumn = column
	return q
}

// Where adds a WHERE condition
func (q *SoftDeleteQuery) Where(condition string, args ...interface{}) *SoftDeleteQuery {
	q.whereClause = append(q.whereClause, condition)
	q.params = append(q.params, args...)
	return q
}

// Build builds the final query with soft delete filter
func (q *SoftDeleteQuery) Build() (string, []interface{}) {
	// Add soft delete filter
	softDeleteFilter := fmt.Sprintf("%s.%s IS NULL", q.tableName, q.deleteColumn)
	q.whereClause = append(q.whereClause, softDeleteFilter)

	// Build WHERE clause
	whereSQL := ""
	if len(q.whereClause) > 0 {
		whereSQL = " WHERE " + strings.Join(q.whereClause, " AND ")
	}

	finalQuery := q.baseQuery + whereSQL
	return finalQuery, q.params
}

// QueryRows executes the query and returns rows
func (q *SoftDeleteQuery) QueryRows(db *sql.DB) (*sql.Rows, error) {
	query, params := q.Build()
	return db.Query(query, params...)
}

// QueryRow executes the query and returns a single row
func (q *SoftDeleteQuery) QueryRow(db *sql.DB) *sql.Row {
	query, params := q.Build()
	return db.QueryRow(query, params...)
}

// WithTrashed includes soft deleted records
func WithTrashed(query string) string {
	// Returns query as-is, allowing soft deleted records
	return query
}

// OnlyTrashed returns only soft deleted records
func OnlyTrashed(query, tableName, deleteColumn string) string {
	if strings.Contains(strings.ToUpper(query), "WHERE") {
		return fmt.Sprintf("%s AND %s.%s IS NOT NULL", query, tableName, deleteColumn)
	}
	return fmt.Sprintf("%s WHERE %s.%s IS NOT NULL", query, tableName, deleteColumn)
}
