package service_test

import (
	"context"
	"errors"
	"testing"

	"metargb/calendar-service/internal/models"
	"metargb/calendar-service/internal/service"
	"metargb/calendar-service/internal/testutil"
)

func TestGetEvents_EventTypeDefault(t *testing.T) {
	var gotType string
	m := &testutil.MockCalendarRepo{}
	m.GetEventsFunc = func(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
		gotType = eventType
		return []*models.Calendar{{ID: 1}}, false, nil
	}
	svc := service.NewCalendarService(m)
	_, _, err := svc.GetEvents(context.Background(), "event", "", "", 0, 1, 10)
	if err != nil || gotType != "event" {
		t.Fatalf("gotType=%q err=%v", gotType, err)
	}
}

func TestGetEvents_VersionType(t *testing.T) {
	var gotType string
	m := &testutil.MockCalendarRepo{}
	m.GetEventsFunc = func(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
		gotType = eventType
		return nil, false, nil
	}
	svc := service.NewCalendarService(m)
	_, _, err := svc.GetEvents(context.Background(), "version", "", "", 0, 1, 10)
	if err != nil || gotType != "version" {
		t.Fatalf("gotType=%q err=%v", gotType, err)
	}
}

func TestGetEvents_WithDate(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetEventsFunc = func(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
		if date != "1403/01/15" {
			t.Fatalf("unexpected date %q", date)
		}
		return []*models.Calendar{{ID: 1}}, false, nil
	}
	svc := service.NewCalendarService(m)
	ev, hasMore, err := svc.GetEvents(context.Background(), "event", "", "1403/01/15", 0, 1, 10)
	if err != nil || hasMore || len(ev) != 1 {
		t.Fatal(err, hasMore, len(ev))
	}
}

func TestGetEvent_Found(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetEventByIDFunc = func(ctx context.Context, id uint64) (*models.Calendar, error) {
		return &models.Calendar{ID: id, Title: "E"}, nil
	}
	svc := service.NewCalendarService(m)
	ev, err := svc.GetEvent(context.Background(), 42, 0)
	if err != nil || ev.ID != 42 || ev.Title != "E" {
		t.Fatal(err, ev)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetEventByIDFunc = func(ctx context.Context, id uint64) (*models.Calendar, error) {
		return nil, nil
	}
	svc := service.NewCalendarService(m)
	_, err := svc.GetEvent(context.Background(), 99, 0)
	if !errors.Is(err, service.ErrEventNotFound) {
		t.Fatalf("want ErrEventNotFound got %v", err)
	}
}

func TestGetEvent_RepoError(t *testing.T) {
	repoErr := errors.New("db down")
	m := &testutil.MockCalendarRepo{}
	m.GetEventByIDFunc = func(ctx context.Context, id uint64) (*models.Calendar, error) {
		return nil, repoErr
	}
	svc := service.NewCalendarService(m)
	_, err := svc.GetEvent(context.Background(), 1, 0)
	if err == nil || errors.Is(err, service.ErrEventNotFound) {
		t.Fatal("expected wrapped repo error")
	}
}

func TestFilterByDateRange_OK(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.FilterByDateRangeFunc = func(ctx context.Context, startDate, endDate string) ([]*models.Calendar, error) {
		return []*models.Calendar{{ID: 1, Title: "A"}}, nil
	}
	svc := service.NewCalendarService(m)
	list, err := svc.FilterByDateRange(context.Background(), "1403/01/01", "1403/01/10")
	if err != nil || len(list) != 1 {
		t.Fatal(err, list)
	}
}

func TestGetLatestVersionTitle(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetLatestVersionTitleFunc = func(ctx context.Context) (string, error) {
		return "v1.0", nil
	}
	svc := service.NewCalendarService(m)
	v, err := svc.GetLatestVersionTitle(context.Background())
	if err != nil || v != "v1.0" {
		t.Fatal(err, v)
	}
}

func TestGetEventStats(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetEventStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{ViewsCount: 3, LikesCount: 2, DislikesCount: 1}, nil
	}
	svc := service.NewCalendarService(m)
	st, err := svc.GetEventStats(context.Background(), 10)
	if err != nil || st.ViewsCount != 3 || st.LikesCount != 2 || st.DislikesCount != 1 {
		t.Fatal(err, st)
	}
}

func TestGetUserInteraction_NoInteraction(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetUserInteractionFunc = func(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
		return nil, nil
	}
	svc := service.NewCalendarService(m)
	in, err := svc.GetUserInteraction(context.Background(), 1, 2)
	if err != nil || in != nil {
		t.Fatal(err, in)
	}
}

func TestGetUserInteraction_Liked(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetUserInteractionFunc = func(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
		return &models.Interaction{Liked: true}, nil
	}
	svc := service.NewCalendarService(m)
	in, err := svc.GetUserInteraction(context.Background(), 1, 2)
	if err != nil || !in.Liked {
		t.Fatal(err, in)
	}
}

func TestAddInteraction_Like(t *testing.T) {
	var got int32
	m := &testutil.MockCalendarRepo{}
	m.AddInteractionFunc = func(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error {
		got = liked
		return nil
	}
	svc := service.NewCalendarService(m)
	if err := svc.AddInteraction(context.Background(), 1, 2, 1, "127.0.0.1"); err != nil || got != 1 {
		t.Fatal(err, got)
	}
}

func TestAddInteraction_Dislike(t *testing.T) {
	var got int32
	m := &testutil.MockCalendarRepo{}
	m.AddInteractionFunc = func(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error {
		got = liked
		return nil
	}
	svc := service.NewCalendarService(m)
	if err := svc.AddInteraction(context.Background(), 1, 2, 0, "127.0.0.1"); err != nil || got != 0 {
		t.Fatal(err, got)
	}
}

func TestAddInteraction_Remove(t *testing.T) {
	var got int32
	m := &testutil.MockCalendarRepo{}
	m.AddInteractionFunc = func(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error {
		got = liked
		return nil
	}
	svc := service.NewCalendarService(m)
	if err := svc.AddInteraction(context.Background(), 1, 2, -1, "127.0.0.1"); err != nil || got != -1 {
		t.Fatal(err, got)
	}
}

func TestAddInteraction_InvalidValue(t *testing.T) {
	svc := service.NewCalendarService(&testutil.MockCalendarRepo{})
	err := svc.AddInteraction(context.Background(), 1, 2, 2, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIncrementView(t *testing.T) {
	var gotEvent uint64
	var gotIP string
	m := &testutil.MockCalendarRepo{}
	m.IncrementViewFunc = func(ctx context.Context, eventID uint64, ipAddress string) error {
		gotEvent = eventID
		gotIP = ipAddress
		return nil
	}
	svc := service.NewCalendarService(m)
	if err := svc.IncrementView(context.Background(), 7, "10.0.0.1"); err != nil || gotEvent != 7 || gotIP != "10.0.0.1" {
		t.Fatal(err, gotEvent, gotIP)
	}
}
