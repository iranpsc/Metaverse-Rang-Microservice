package handler_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	calendarpb "metargb/shared/pb/calendar"
	commonpb "metargb/shared/pb/common"

	"metargb/calendar-service/internal/handler"
	"metargb/calendar-service/internal/models"
	"metargb/calendar-service/internal/service"
	"metargb/calendar-service/internal/testutil"
)

func TestGetEvents_DefaultPagination(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetEventsFunc = func(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
		return []*models.Calendar{{ID: 1, Title: "A", Content: "c", StartsAt: time.Now(), Color: "#fff"}}, false, nil
	}
	m.GetEventStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{}, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetEvents(context.Background(), &calendarpb.GetEventsRequest{
		Pagination: &commonpb.PaginationRequest{Page: 1, PerPage: 10},
	})
	if err != nil || resp.Pagination == nil || resp.Pagination.CurrentPage != 1 || resp.Pagination.PerPage != 10 {
		t.Fatalf("err=%v pagination=%+v", err, resp.Pagination)
	}
}

func TestGetEvents_WithDateNoPagination(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetEventsFunc = func(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
		return []*models.Calendar{{ID: 1, Title: "A", Content: "c", StartsAt: time.Now(), Color: "#fff"}}, false, nil
	}
	m.GetEventStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{}, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetEvents(context.Background(), &calendarpb.GetEventsRequest{
		Date:       "1403/07/01",
		Pagination: &commonpb.PaginationRequest{Page: 1, PerPage: 10},
	})
	if err != nil || resp.Pagination != nil {
		t.Fatalf("err=%v pagination=%+v", err, resp.Pagination)
	}
}

func TestGetEvents_WithUserInteraction(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetEventsFunc = func(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
		return []*models.Calendar{{ID: 1, Title: "A", Content: "c", StartsAt: time.Now(), Color: "#fff"}}, false, nil
	}
	m.GetEventStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{}, nil
	}
	m.GetUserInteractionFunc = func(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
		return &models.Interaction{Liked: true}, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetEvents(context.Background(), &calendarpb.GetEventsRequest{
		UserId:     99,
		Pagination: &commonpb.PaginationRequest{Page: 1, PerPage: 10},
	})
	if err != nil || len(resp.Events) != 1 || resp.Events[0].UserInteraction == nil {
		t.Fatal(err, resp)
	}
	ui := resp.Events[0].UserInteraction
	if !ui.HasLiked || ui.HasDisliked {
		t.Fatalf("user_interaction=%+v", ui)
	}
}

func TestGetEvents_VersionType(t *testing.T) {
	vt := "v2"
	m := &testutil.MockCalendarRepo{}
	m.GetEventsFunc = func(ctx context.Context, eventType, search, date string, userID uint64, page, perPage int32) ([]*models.Calendar, bool, error) {
		if eventType != "version" {
			t.Fatalf("type=%s", eventType)
		}
		return []*models.Calendar{{
			ID:           1,
			Title:        "Ver",
			Content:      "c",
			IsVersion:    true,
			VersionTitle: &vt,
			StartsAt:     time.Now(),
			Color:        "",
		}}, false, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetEvents(context.Background(), &calendarpb.GetEventsRequest{
		Type:       "version",
		Pagination: &commonpb.PaginationRequest{Page: 1, PerPage: 10},
	})
	if err != nil || len(resp.Events) != 1 || resp.Events[0].VersionTitle != "v2" {
		t.Fatal(err, resp)
	}
}

func TestGetEvent_Found(t *testing.T) {
	st := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	m := &testutil.MockCalendarRepo{}
	m.GetEventByIDFunc = func(ctx context.Context, id uint64) (*models.Calendar, error) {
		return &models.Calendar{ID: id, Title: "T", Content: "body", StartsAt: st, Color: "#000"}, nil
	}
	m.IncrementViewFunc = func(ctx context.Context, eventID uint64, ipAddress string) error { return nil }
	m.GetEventStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{ViewsCount: 1}, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetEvent(context.Background(), &calendarpb.GetEventRequest{EventId: 5})
	if err != nil || resp.Id != 5 || resp.Title != "T" || resp.Description != "body" {
		t.Fatal(err, resp)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetEventByIDFunc = func(ctx context.Context, id uint64) (*models.Calendar, error) {
		return nil, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	_, err := client.GetEvent(context.Background(), &calendarpb.GetEventRequest{EventId: 404})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Fatalf("got %v", err)
	}
}

func TestGetEvent_HasLiked(t *testing.T) {
	st := time.Now()
	m := &testutil.MockCalendarRepo{}
	m.GetEventByIDFunc = func(ctx context.Context, id uint64) (*models.Calendar, error) {
		return &models.Calendar{ID: id, Title: "T", Content: "c", StartsAt: st, Color: "#fff"}, nil
	}
	m.IncrementViewFunc = func(ctx context.Context, eventID uint64, ipAddress string) error { return nil }
	m.GetEventStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{}, nil
	}
	m.GetUserInteractionFunc = func(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
		return &models.Interaction{Liked: true}, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetEvent(context.Background(), &calendarpb.GetEventRequest{EventId: 1, UserId: 10})
	if err != nil || resp.UserInteraction == nil || !resp.UserInteraction.HasLiked || resp.UserInteraction.HasDisliked {
		t.Fatal(err, resp)
	}
}

func TestGetEvent_HasDisliked(t *testing.T) {
	st := time.Now()
	m := &testutil.MockCalendarRepo{}
	m.GetEventByIDFunc = func(ctx context.Context, id uint64) (*models.Calendar, error) {
		return &models.Calendar{ID: id, Title: "T", Content: "c", StartsAt: st, Color: "#fff"}, nil
	}
	m.IncrementViewFunc = func(ctx context.Context, eventID uint64, ipAddress string) error { return nil }
	m.GetEventStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{}, nil
	}
	m.GetUserInteractionFunc = func(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
		return &models.Interaction{Liked: false}, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetEvent(context.Background(), &calendarpb.GetEventRequest{EventId: 1, UserId: 10})
	if err != nil || resp.UserInteraction == nil || resp.UserInteraction.HasLiked || !resp.UserInteraction.HasDisliked {
		t.Fatal(err, resp)
	}
}

func TestFilterByDateRange_OK(t *testing.T) {
	st := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	en := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
	m := &testutil.MockCalendarRepo{}
	m.FilterByDateRangeFunc = func(ctx context.Context, startDate, endDate string) ([]*models.Calendar, error) {
		return []*models.Calendar{{ID: 9, Title: "X", StartsAt: st, EndsAt: &en, Color: "#abc"}}, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.FilterByDateRange(context.Background(), &calendarpb.FilterByDateRangeRequest{
		StartDate: "1402/10/11",
		EndDate:   "1402/10/15",
	})
	if err != nil || len(resp.Events) != 1 || resp.Events[0].Id != 9 || resp.Events[0].Title != "X" {
		t.Fatal(err, resp)
	}
}

func TestGetLatestVersion_Present(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetLatestVersionTitleFunc = func(ctx context.Context) (string, error) {
		return "v3", nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetLatestVersion(context.Background(), &calendarpb.GetLatestVersionRequest{})
	if err != nil || resp.VersionTitle != "v3" {
		t.Fatal(err, resp)
	}
}

func TestGetLatestVersion_Empty(t *testing.T) {
	m := &testutil.MockCalendarRepo{}
	m.GetLatestVersionTitleFunc = func(ctx context.Context) (string, error) {
		return "", nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.GetLatestVersion(context.Background(), &calendarpb.GetLatestVersionRequest{})
	if err != nil || resp.VersionTitle != "" {
		t.Fatal(err, resp)
	}
}

func TestAddInteraction_InvalidLiked(t *testing.T) {
	svc := service.NewCalendarService(&testutil.MockCalendarRepo{})
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	_, err := client.AddInteraction(context.Background(), &calendarpb.AddInteractionRequest{
		EventId: 1, UserId: 2, Liked: 2,
	})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
}

func setupAddInteractionMock() *testutil.MockCalendarRepo {
	st := time.Now()
	m := &testutil.MockCalendarRepo{}
	m.GetEventByIDFunc = func(ctx context.Context, id uint64) (*models.Calendar, error) {
		return &models.Calendar{ID: id, Title: "E", Content: "c", StartsAt: st, Color: "#111"}, nil
	}
	m.IncrementViewFunc = func(ctx context.Context, eventID uint64, ipAddress string) error { return nil }
	m.GetEventStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{LikesCount: 2}, nil
	}
	m.GetInteractionStatsFunc = func(ctx context.Context, eventID uint64) (*models.CalendarStats, error) {
		return &models.CalendarStats{LikesCount: 2}, nil
	}
	m.GetUserInteractionFunc = func(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
		return &models.Interaction{Liked: true}, nil
	}
	return m
}

func TestAddInteraction_Like(t *testing.T) {
	m := setupAddInteractionMock()
	var added bool
	m.AddInteractionFunc = func(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error {
		if liked == 1 {
			added = true
		}
		return nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.AddInteraction(context.Background(), &calendarpb.AddInteractionRequest{
		EventId: 1, UserId: 2, Liked: 1,
	})
	if err != nil || !added || resp == nil || resp.Likes != 2 {
		t.Fatal(err, added, resp)
	}
}

func TestAddInteraction_Remove(t *testing.T) {
	m := setupAddInteractionMock()
	var removed bool
	m.AddInteractionFunc = func(ctx context.Context, eventID, userID uint64, liked int32, ipAddress string) error {
		if liked == -1 {
			removed = true
		}
		return nil
	}
	m.GetUserInteractionFunc = func(ctx context.Context, eventID, userID uint64) (*models.Interaction, error) {
		return nil, nil
	}
	svc := service.NewCalendarService(m)
	conn, cleanup := testutil.DialBufConn(func(s *grpc.Server) {
		handler.RegisterCalendarHandler(s, svc)
	})
	defer cleanup()
	client := calendarpb.NewCalendarServiceClient(conn)
	resp, err := client.AddInteraction(context.Background(), &calendarpb.AddInteractionRequest{
		EventId: 1, UserId: 2, Liked: -1,
	})
	if err != nil || !removed || resp == nil {
		t.Fatal(err, removed, resp)
	}
}
