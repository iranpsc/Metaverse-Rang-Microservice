package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	calendarpb "metargb/shared/pb/calendar"
)

func TestBuildCalendarEventMap_VersionLaravelShape(t *testing.T) {
	event := &calendarpb.EventResponse{
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
	}

	out := buildCalendarEventMap(event, true, "version")

	require.Equal(t, uint64(717), out["id"])
	assert.Equal(t, "V1.1.32", out["version_title"])
	_, hasViews := out["views"]
	assert.False(t, hasViews, "version entries must not expose event-only fields")
	_, hasLikes := out["likes"]
	assert.False(t, hasLikes)
}

func TestBuildCalendarEventMap_VersionFromTitleWhenFlagMissing(t *testing.T) {
	event := &calendarpb.EventResponse{
		Id:           717,
		Title:        "Next.js migration",
		Description:  "<p>changelog</p>",
		StartsAt:     "1405/02/02 00:00",
		VersionTitle: "V1.1.32",
		Views:        2,
		Likes:        1,
	}

	out := buildCalendarEventMap(event, true, "event")

	assert.Equal(t, "V1.1.32", out["version_title"])
	_, hasViews := out["views"]
	assert.False(t, hasViews)
}

func TestBuildCalendarEventMap_EventShape(t *testing.T) {
	event := &calendarpb.EventResponse{
		Id:          710,
		Title:       "Event",
		Description: "desc",
		StartsAt:    "1405/05/19 09:00",
		EndsAt:      "1405/07/01 09:00",
		Views:       4,
		Likes:       1,
		Dislikes:    0,
		Color:       "#ff00ff",
	}

	out := buildCalendarEventMap(event, true, "event")

	assert.Equal(t, int32(4), out["views"])
	assert.Equal(t, "1405/07/01 09:00", out["ends_at"])
	_, hasVersionTitle := out["version_title"]
	assert.False(t, hasVersionTitle)
}
