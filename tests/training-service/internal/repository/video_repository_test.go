package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/training-service/internal/repository"
)

func newSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func videoColumns() []string {
	return []string{"id", "video_sub_category_id", "title", "slug", "description", "fileName", "creator_code", "image", "created_at", "updated_at"}
}

func addVideoRow(rows *sqlmock.Rows, id uint64) *sqlmock.Rows {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	return rows.AddRow(id, uint64(2), "title", "slug-"+string(rune('a'+id)), "desc", "f.mp4", "c1", "img.jpg", now, now)
}

func TestVideoRepository_GetVideos_NoFilter(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(int32(1)))
	mock.ExpectQuery("SELECT v.id, v.video_sub_category_id").WithArgs(int32(18), int32(0)).
		WillReturnRows(addVideoRow(sqlmock.NewRows(videoColumns()), 1))

	r := repository.NewVideoRepository(db)
	list, total, err := r.GetVideos(context.Background(), 1, 18, nil, nil)
	if err != nil || total != 1 || len(list) != 1 || list[0].Title != "title" {
		t.Fatalf("err=%v total=%d n=%d", err, total, len(list))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVideoRepository_GetVideos_CategoryAndSubCategoryFilters(t *testing.T) {
	db, mock := newSQLMock(t)
	cat, sub := uint64(7), uint64(9)
	mock.ExpectQuery("SELECT COUNT").WithArgs(cat, sub).WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(int32(2)))
	mock.ExpectQuery("SELECT v.id, v.video_sub_category_id").WithArgs(cat, sub, int32(10), int32(10)).
		WillReturnRows(addVideoRow(sqlmock.NewRows(videoColumns()), 2))

	r := repository.NewVideoRepository(db)
	list, total, err := r.GetVideos(context.Background(), 2, 10, &cat, &sub)
	if err != nil || total != 2 || len(list) != 1 {
		t.Fatalf("err=%v total=%d n=%d", err, total, len(list))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVideoRepository_GetVideos_CountErrorAndQueryError(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery("SELECT COUNT").WillReturnError(sql.ErrConnDone)
	r := repository.NewVideoRepository(db)
	_, _, err := r.GetVideos(context.Background(), 1, 18, nil, nil)
	if err == nil {
		t.Fatal("expected count error")
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(int32(1)))
	mock.ExpectQuery("SELECT v.id").WillReturnError(sql.ErrConnDone)
	r = repository.NewVideoRepository(db)
	_, _, err = r.GetVideos(context.Background(), 1, 18, nil, nil)
	if err == nil {
		t.Fatal("expected query error")
	}
}

func TestVideoRepository_GetVideoByFileName_FoundAndNotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery("SELECT id, video_sub_category_id").WithArgs("%clip%").
		WillReturnRows(addVideoRow(sqlmock.NewRows(videoColumns()), 3))
	r := repository.NewVideoRepository(db)
	v, err := r.GetVideoByFileName(context.Background(), "clip")
	if err != nil || v == nil || v.ID != 3 {
		t.Fatalf("v=%+v err=%v", v, err)
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT id, video_sub_category_id").WithArgs("%missing%").WillReturnError(sql.ErrNoRows)
	r = repository.NewVideoRepository(db)
	v, err = r.GetVideoByFileName(context.Background(), "missing")
	if err != nil || v != nil {
		t.Fatalf("expected nil, got %+v err=%v", v, err)
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT id, video_sub_category_id").WillReturnError(sql.ErrConnDone)
	r = repository.NewVideoRepository(db)
	_, err = r.GetVideoByFileName(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVideoRepository_GetVideoBySlug_FoundAndNotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery("SELECT id, video_sub_category_id").WithArgs("intro").
		WillReturnRows(addVideoRow(sqlmock.NewRows(videoColumns()), 4))
	r := repository.NewVideoRepository(db)
	v, err := r.GetVideoBySlug(context.Background(), "intro")
	if err != nil || v == nil || v.ID != 4 {
		t.Fatalf("v=%+v err=%v", v, err)
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT id, video_sub_category_id").WithArgs("missing").WillReturnError(sql.ErrNoRows)
	r = repository.NewVideoRepository(db)
	v, err = r.GetVideoBySlug(context.Background(), "missing")
	if err != nil || v != nil {
		t.Fatalf("expected nil, got %+v err=%v", v, err)
	}
}

func TestVideoRepository_GetVideoBySlug_ScanError(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery("SELECT id, video_sub_category_id").WithArgs("bad").WillReturnError(sql.ErrConnDone)
	r := repository.NewVideoRepository(db)
	_, err := r.GetVideoBySlug(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVideoRepository_SearchVideos(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery("utf8mb4_general_ci").WithArgs("%go%").WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(int32(1)))
	mock.ExpectQuery("utf8mb4_general_ci").WithArgs("%go%", int32(18), int32(0)).
		WillReturnRows(addVideoRow(sqlmock.NewRows(videoColumns()), 1))
	r := repository.NewVideoRepository(db)
	list, total, err := r.SearchVideos(context.Background(), "go", 1, 18)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("err=%v total=%d n=%d", err, total, len(list))
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("utf8mb4_general_ci").WithArgs("%عباس%").WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(int32(1)))
	mock.ExpectQuery("utf8mb4_general_ci").WithArgs("%عباس%", int32(18), int32(0)).
		WillReturnRows(addVideoRow(sqlmock.NewRows(videoColumns()), 5))
	r = repository.NewVideoRepository(db)
	list, total, err = r.SearchVideos(context.Background(), "  عباس  ", 1, 18)
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != 5 {
		t.Fatalf("persian search err=%v total=%d list=%v", err, total, list)
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT COUNT").WillReturnError(sql.ErrConnDone)
	r = repository.NewVideoRepository(db)
	_, _, err = r.SearchVideos(context.Background(), "x", 1, 18)
	if err == nil {
		t.Fatal("expected count error")
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(int32(1)))
	mock.ExpectQuery("SELECT id, video_sub_category_id").WillReturnError(sql.ErrConnDone)
	r = repository.NewVideoRepository(db)
	_, _, err = r.SearchVideos(context.Background(), "x", 1, 18)
	if err == nil {
		t.Fatal("expected search error")
	}
}

func TestVideoRepository_GetVideoStats(t *testing.T) {
	db, mock := newSQLMock(t)
	for _, n := range []int32{10, 3, 1, 4} {
		mock.ExpectQuery("SELECT COUNT").WithArgs(uint64(8)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(n))
	}
	r := repository.NewVideoRepository(db)
	st, err := r.GetVideoStats(context.Background(), 8)
	if err != nil || st.ViewsCount != 10 || st.LikesCount != 3 || st.DislikesCount != 1 || st.CommentsCount != 4 {
		t.Fatalf("stats=%+v err=%v", st, err)
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT COUNT").WillReturnError(sql.ErrConnDone)
	r = repository.NewVideoRepository(db)
	_, err = r.GetVideoStats(context.Background(), 8)
	if err == nil {
		t.Fatal("expected views error")
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT COUNT").WithArgs(uint64(8)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int32(1)))
	mock.ExpectQuery("SELECT COUNT").WithArgs(uint64(8)).WillReturnError(sql.ErrConnDone)
	r = repository.NewVideoRepository(db)
	if _, err := r.GetVideoStats(context.Background(), 8); err == nil {
		t.Fatal("expected likes error")
	}
}

func TestVideoRepository_GetUserInteraction(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery("SELECT liked").WithArgs(uint64(1), uint64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"liked"}).AddRow(true))
	r := repository.NewVideoRepository(db)
	got, err := r.GetUserInteraction(context.Background(), 1, 2)
	if err != nil || got == nil || !*got {
		t.Fatalf("got=%v err=%v", got, err)
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT liked").WillReturnError(sql.ErrNoRows)
	r = repository.NewVideoRepository(db)
	got, err = r.GetUserInteraction(context.Background(), 1, 2)
	if err != nil || got != nil {
		t.Fatalf("expected nil interaction, got %v err=%v", got, err)
	}

	db, mock = newSQLMock(t)
	mock.ExpectQuery("SELECT liked").WillReturnError(sql.ErrConnDone)
	r = repository.NewVideoRepository(db)
	_, err = r.GetUserInteraction(context.Background(), 1, 2)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVideoRepository_IncrementViewAndAddInteraction_Success(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectExec("INSERT INTO views").WillReturnResult(sqlmock.NewResult(1, 1))
	r := repository.NewVideoRepository(db)
	if err := r.IncrementView(context.Background(), 1, "ip"); err != nil {
		t.Fatal(err)
	}

	db, mock = newSQLMock(t)
	mock.ExpectExec("INSERT INTO interactions").WillReturnResult(sqlmock.NewResult(1, 1))
	r = repository.NewVideoRepository(db)
	if err := r.AddInteraction(context.Background(), 1, 2, true, "ip"); err != nil {
		t.Fatal(err)
	}
}

func TestVideoRepository_IncrementViewAndAddInteraction_Errors(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectExec("INSERT INTO views").WillReturnError(sql.ErrConnDone)
	r := repository.NewVideoRepository(db)
	if err := r.IncrementView(context.Background(), 1, "ip"); err == nil {
		t.Fatal("expected increment error")
	}

	db, mock = newSQLMock(t)
	mock.ExpectExec("INSERT INTO interactions").WillReturnError(sql.ErrConnDone)
	r = repository.NewVideoRepository(db)
	if err := r.AddInteraction(context.Background(), 1, 2, false, "ip"); err == nil {
		t.Fatal("expected interaction error")
	}
}
