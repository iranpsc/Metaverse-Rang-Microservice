package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"metarang/features-service/internal/models"
)

type IsicCodeRepository struct {
	db *sql.DB
}

func NewIsicCodeRepository(db *sql.DB) *IsicCodeRepository {
	return &IsicCodeRepository{db: db}
}

func (r *IsicCodeRepository) FindPaginated(ctx context.Context, search string, limit, offset int) ([]models.IsicCode, error) {
	whereClause, args := isicCodeSearchClause(search)

	query := fmt.Sprintf(`
		SELECT id, name, code, verified
		FROM isic_codes
		%s
		ORDER BY id
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list isic codes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]models.IsicCode, 0, limit)
	for rows.Next() {
		item, err := scanIsicCode(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating isic codes: %w", err)
	}

	return items, nil
}

func (r *IsicCodeRepository) Count(ctx context.Context, search string) (int, error) {
	whereClause, args := isicCodeSearchClause(search)

	query := fmt.Sprintf(`SELECT COUNT(*) FROM isic_codes %s`, whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count isic codes: %w", err)
	}
	return total, nil
}

func isicCodeSearchClause(search string) (string, []interface{}) {
	search = strings.TrimSpace(search)
	if search == "" {
		return "", nil
	}

	pattern := "%" + search + "%"
	return "WHERE name LIKE ? OR CAST(code AS CHAR) LIKE ?", []interface{}{pattern, pattern}
}

func scanIsicCode(scanner interface {
	Scan(dest ...interface{}) error
}) (models.IsicCode, error) {
	var item models.IsicCode
	var code sql.NullInt64
	if err := scanner.Scan(&item.ID, &item.Name, &code, &item.Verified); err != nil {
		return models.IsicCode{}, fmt.Errorf("failed to scan isic code: %w", err)
	}
	if code.Valid {
		value := uint64(code.Int64)
		item.Code = &value
	}
	return item, nil
}
