package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/calendar-service/internal/repository"
)

const calendarMorphType = "App\\Models\\Calendar"

func TestGetEventByID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	st := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	en := time.Date(2024, 6, 2, 12, 0, 0, 0, time.UTC)
	slug := "s"
	vt := "v"
	rows := sqlmock.NewRows([]string{
		"id", "slug", "title", "content", "color", "writer", "is_version", "version_title",
		"btn_name", "btn_link", "image", "starts_at", "ends_at", "created_at", "updated_at",
	}).AddRow(uint64(1), slug, "T", "body", "#fff", "w", int64(0), vt, nil, nil, nil, st, en, st, st)

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, slug, title, content, color, writer, is_version, version_title, btn_name, btn_link, image, starts_at, ends_at, created_at, updated_at FROM calendars WHERE id = ?",
	)).WithArgs(uint64(42)).WillReturnRows(rows)

	r := repository.NewCalendarRepository(db)
	ev, err := r.GetEventByID(context.Background(), 42)
	if err != nil || ev == nil || ev.ID != 1 || ev.Title != "T" {
		t.Fatal(err, ev)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetEventByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, slug, title, content, color, writer, is_version, version_title, btn_name, btn_link, image, starts_at, ends_at, created_at, updated_at FROM calendars WHERE id = ?",
	)).WithArgs(uint64(999)).WillReturnError(sql.ErrNoRows)

	r := repository.NewCalendarRepository(db)
	ev, err := r.GetEventByID(context.Background(), 999)
	if err != nil || ev != nil {
		t.Fatal(err, ev)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetEvents_Pagination(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	st := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "slug", "title", "content", "color", "writer", "is_version", "version_title",
		"btn_name", "btn_link", "image", "starts_at", "ends_at", "created_at", "updated_at",
	}).AddRow(uint64(1), nil, "E", "c", "#000", "w", int64(0), nil, nil, nil, nil, st, nil, st, st)

	mock.ExpectQuery("ORDER BY starts_at DESC").
		WithArgs(int32(11), int64(0)).
		WillReturnRows(rows)

	r := repository.NewCalendarRepository(db)
	list, hasMore, err := r.GetEvents(context.Background(), "event", "", "", 0, 1, 10)
	if err != nil || hasMore || len(list) != 1 {
		t.Fatal(err, hasMore, list)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetEvents_WithDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	st := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "slug", "title", "content", "color", "writer", "is_version", "version_title",
		"btn_name", "btn_link", "image", "starts_at", "ends_at", "created_at", "updated_at",
	}).AddRow(uint64(1), nil, "E", "c", "#000", "w", int64(0), nil, nil, nil, nil, st, nil, st, st)

	mock.ExpectQuery("DATE\\(starts_at\\).*ORDER BY created_at DESC").
		WillReturnRows(rows)

	r := repository.NewCalendarRepository(db)
	_, _, err = r.GetEvents(context.Background(), "event", "", "1403/07/01", 0, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetLatestVersionTitle_NoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT version_title FROM calendars WHERE is_version = 1 ORDER BY starts_at DESC LIMIT 1",
	)).WillReturnError(sql.ErrNoRows)

	r := repository.NewCalendarRepository(db)
	v, err := r.GetLatestVersionTitle(context.Background())
	if err != nil || v != "" {
		t.Fatal(err, v)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetEventStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM views WHERE viewable_type = ? AND viewable_id = ?")).
		WithArgs(calendarMorphType, uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(5)))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM interactions WHERE likeable_type = ? AND likeable_id = ? AND liked = 1")).
		WithArgs(calendarMorphType, uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(2)))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM interactions WHERE likeable_type = ? AND likeable_id = ? AND liked = 0")).
		WithArgs(calendarMorphType, uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := repository.NewCalendarRepository(db)
	st, err := r.GetEventStats(context.Background(), 7)
	if err != nil || st.ViewsCount != 5 || st.LikesCount != 2 || st.DislikesCount != 1 {
		t.Fatal(err, st)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetUserInteraction_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tm := time.Now()
	row := sqlmock.NewRows([]string{
		"id", "likeable_type", "likeable_id", "user_id", "liked", "ip_address", "created_at", "updated_at",
	}).AddRow(uint64(1), "App\\Models\\Calendar", uint64(10), uint64(2), true, "127.0.0.1", tm, tm)

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, likeable_type, likeable_id, user_id, liked, ip_address, created_at, updated_at FROM interactions WHERE likeable_type = ? AND likeable_id = ? AND user_id = ?",
	)).WithArgs(calendarMorphType, uint64(10), uint64(2)).WillReturnRows(row)

	r := repository.NewCalendarRepository(db)
	in, err := r.GetUserInteraction(context.Background(), 10, 2)
	if err != nil || in == nil || !in.Liked {
		t.Fatal(err, in)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAddInteraction_Like(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO interactions").
		WithArgs(calendarMorphType, uint64(10), uint64(2), true, "127.0.0.1", true, "127.0.0.1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := repository.NewCalendarRepository(db)
	err = r.AddInteraction(context.Background(), 10, 2, 1, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAddInteraction_Remove(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM interactions WHERE likeable_type = ? AND likeable_id = ? AND user_id = ?",
	)).WithArgs(calendarMorphType, uint64(10), uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))

	r := repository.NewCalendarRepository(db)
	err = r.AddInteraction(context.Background(), 10, 2, -1, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIncrementView_InsertsRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	morph := "App\\Models\\Calendar"
	mock.ExpectExec(regexp.QuoteMeta(
		"INSERT INTO views (viewable_type, viewable_id, ip_address, created_at, updated_at)",
	)).WithArgs(morph, uint64(5), "192.168.1.1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := repository.NewCalendarRepository(db)
	err = r.IncrementView(context.Background(), 5, "192.168.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFilterByDateRange_OverlapLogic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	st := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "slug", "title", "content", "color", "writer", "is_version", "version_title",
		"btn_name", "btn_link", "image", "starts_at", "ends_at", "created_at", "updated_at",
	}).AddRow(uint64(1), nil, "E", "c", "#000", "w", int64(0), nil, nil, nil, nil, st, nil, st, st)

	mock.ExpectQuery("DATE\\(starts_at\\) BETWEEN.*OR.*DATE\\(ends_at\\) BETWEEN.*OR").
		WillReturnRows(rows)

	r := repository.NewCalendarRepository(db)
	list, err := r.FilterByDateRange(context.Background(), "1403/01/01", "1403/01/10")
	if err != nil || len(list) != 1 {
		t.Fatal(err, list)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetEventStats_ViewQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM views WHERE viewable_type = ? AND viewable_id = ?")).
		WithArgs(calendarMorphType, uint64(1)).
		WillReturnError(errors.New("db error"))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM interactions WHERE likeable_type = ? AND likeable_id = ? AND liked = 1")).
		WithArgs(calendarMorphType, uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM interactions WHERE likeable_type = ? AND likeable_id = ? AND liked = 0")).
		WithArgs(calendarMorphType, uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := repository.NewCalendarRepository(db)
	st, err := r.GetEventStats(context.Background(), 1)
	if err != nil || st.ViewsCount != 0 {
		t.Fatal(err, st)
	}
}

func TestGetInteractionStats_OK(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM interactions WHERE likeable_type = ? AND likeable_id = ? AND liked = 1")).
		WithArgs(calendarMorphType, uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(3)))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM interactions WHERE likeable_type = ? AND likeable_id = ? AND liked = 0")).
		WithArgs(calendarMorphType, uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := repository.NewCalendarRepository(db)
	st, err := r.GetInteractionStats(context.Background(), 7)
	if err != nil || st.LikesCount != 3 || st.DislikesCount != 1 {
		t.Fatal(err, st)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetLatestVersionTitle_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT version_title FROM calendars WHERE is_version = 1 ORDER BY starts_at DESC LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"version_title"}).AddRow("V1.2.3"))

	r := repository.NewCalendarRepository(db)
	title, err := r.GetLatestVersionTitle(context.Background())
	if err != nil || title != "V1.2.3" {
		t.Fatal(err, title)
	}
}

func TestGetLatestVersionTitle_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT version_title FROM calendars WHERE is_version = 1 ORDER BY starts_at DESC LIMIT 1")).
		WillReturnError(errors.New("db error"))

	r := repository.NewCalendarRepository(db)
	_, err = r.GetLatestVersionTitle(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
