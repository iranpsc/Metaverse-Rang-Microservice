package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	calendarpb "metargb/shared/pb/calendar"

	"metargb/grpc-gateway/internal/handler"
	"metargb/grpc-gateway/internal/testutil"
)

func newCalendarHandler(t *testing.T, calendar *testutil.MockCalendarService) *handler.CalendarHandler {
	t.Helper()
	conn, cleanup := testutil.DialCalendarConn(calendar)
	t.Cleanup(cleanup)
	return handler.NewCalendarHandler(conn, conn)
}

func TestGetEvents_VersionLaravelShape(t *testing.T) {
	calendar := &testutil.MockCalendarService{}
	calendar.GetEventsFunc = func(_ context.Context, req *calendarpb.GetEventsRequest) (*calendarpb.EventsResponse, error) {
		if req.Type != "version" {
			t.Fatalf("type=%s", req.Type)
		}
		return &calendarpb.EventsResponse{
			Events: []*calendarpb.EventResponse{
				{
					Id:           717,
					Title:        "Next.js migration",
					Description:  "<p>changelog</p>",
					StartsAt:     "1405/02/02 00:00",
					VersionTitle: "V1.1.32",
					IsVersion:    true,
					Views:        4,
					Likes:        1,
					Dislikes:     0,
					Color:        "#ff00ff",
				},
			},
		}, nil
	}
	h := newCalendarHandler(t, calendar)

	req := httptest.NewRequest(http.MethodGet, "/api/calendar?type=version", nil)
	w := httptest.NewRecorder()
	h.GetEvents(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].([]interface{})
	event := data[0].(map[string]interface{})

	require.EqualValues(t, 717, event["id"])
	assert.Equal(t, "V1.1.32", event["version_title"])
	_, hasViews := event["views"]
	assert.False(t, hasViews, "version entries must not expose event-only fields")
	_, hasLikes := event["likes"]
	assert.False(t, hasLikes)
}

func TestGetEvent_VersionFromTitleWhenFlagMissing(t *testing.T) {
	calendar := &testutil.MockCalendarService{}
	calendar.GetEventFunc = func(_ context.Context, _ *calendarpb.GetEventRequest) (*calendarpb.EventResponse, error) {
		return &calendarpb.EventResponse{
			Id:           717,
			Title:        "Next.js migration",
			Description:  "<p>changelog</p>",
			StartsAt:     "1405/02/02 00:00",
			VersionTitle: "V1.1.32",
			Views:        2,
			Likes:        1,
		}, nil
	}
	h := newCalendarHandler(t, calendar)

	req := httptest.NewRequest(http.MethodGet, "/api/calendar/717", nil)
	w := httptest.NewRecorder()
	h.GetEvent(w, req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	event := body["data"].(map[string]interface{})

	assert.Equal(t, "V1.1.32", event["version_title"])
	_, hasViews := event["views"]
	assert.False(t, hasViews)
}

func TestGetEvent_EventShape(t *testing.T) {
	calendar := &testutil.MockCalendarService{}
	calendar.GetEventFunc = func(_ context.Context, _ *calendarpb.GetEventRequest) (*calendarpb.EventResponse, error) {
		return &calendarpb.EventResponse{
			Id:          710,
			Title:       "Event",
			Description: "desc",
			StartsAt:    "1405/05/19 09:00",
			EndsAt:      "1405/07/01 09:00",
			Views:       4,
			Likes:       1,
			Dislikes:    0,
			Color:       "#ff00ff",
		}, nil
	}
	h := newCalendarHandler(t, calendar)

	req := httptest.NewRequest(http.MethodGet, "/api/calendar/710", nil)
	w := httptest.NewRecorder()
	h.GetEvent(w, req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	event := body["data"].(map[string]interface{})

	assert.EqualValues(t, 4, event["views"])
	assert.Equal(t, "1405/07/01 09:00", event["ends_at"])
	_, hasVersionTitle := event["version_title"]
	assert.False(t, hasVersionTitle)
}
