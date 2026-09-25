package models

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// NullableDateTime is sql.NullTime that also accepts VARCHAR datetime strings.
// buy_feature_requests.requested_grace_period is varchar(191) in the Laravel schema,
// so go-sql-driver/mysql returns []byte rather than time.Time even with parseTime=true.
type NullableDateTime struct {
	sql.NullTime
}

var nullableDateTimeLayouts = []string{
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05",
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02",
}

// Scan implements sql.Scanner.
func (n *NullableDateTime) Scan(value any) error {
	if n == nil {
		return fmt.Errorf("NullableDateTime: Scan on nil pointer")
	}
	if value == nil {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		n.Time, n.Valid = v, true
		return nil
	case []byte:
		return n.parse(string(v))
	case string:
		return n.parse(v)
	default:
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type *NullableDateTime", value)
	}
}

func (n *NullableDateTime) parse(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || s == "0000-00-00" || s == "0000-00-00 00:00:00" {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}
	for _, layout := range nullableDateTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			n.Time, n.Valid = t, true
			return nil
		}
		if t, err := time.Parse(layout, s); err == nil {
			n.Time, n.Valid = t, true
			return nil
		}
	}
	return fmt.Errorf("cannot parse datetime %q into NullableDateTime", s)
}

// Value implements driver.Valuer.
func (n NullableDateTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time, nil
}
