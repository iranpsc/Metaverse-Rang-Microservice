// Package handler implements gRPC handlers for calendar events.
package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"metarang/calendar-service/internal/models"
	"metarang/calendar-service/internal/service"
	calendarpb "metarang/shared/pb/calendar"
	commonpb "metarang/shared/pb/common"
	"metarang/shared/pkg/jalali"
)

type CalendarHandler struct {
	calendarpb.UnimplementedCalendarServiceServer
	service service.CalendarServiceInterface
}

func RegisterCalendarHandler(grpcServer *grpc.Server, svc service.CalendarServiceInterface) {
	handler := &CalendarHandler{service: svc}
	calendarpb.RegisterCalendarServiceServer(grpcServer, handler)
}

func (h *CalendarHandler) GetEvents(ctx context.Context, req *calendarpb.GetEventsRequest) (*calendarpb.EventsResponse, error) {
	page := int32(1)
	perPage := int32(10)
	hasDateFilter := req.Date != ""

	if !hasDateFilter && req.Pagination != nil {
		if req.Pagination.Page > 0 {
			page = req.Pagination.Page
		}
		if req.Pagination.PerPage > 0 {
			perPage = req.Pagination.PerPage
		}
	}

	events, hasMore, err := h.service.GetEvents(ctx, req.Type, req.Search, req.Date, req.UserId, page, perPage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get events: %v", err)
	}

	response := &calendarpb.EventsResponse{
		Events: make([]*calendarpb.EventResponse, 0, len(events)),
	}

	if !hasDateFilter {
		response.HasMore = hasMore
		response.Pagination = &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
		}
	}

	for _, event := range events {
		stats, _ := h.service.GetEventStats(ctx, event.ID)

		var userInteraction *calendarpb.UserInteraction
		if req.UserId > 0 {
			interaction, _ := h.service.GetUserInteraction(ctx, event.ID, req.UserId)
			if interaction != nil {
				userInteraction = buildUserInteraction(interaction)
			}
		}

		response.Events = append(response.Events, buildEventResponse(event, stats, userInteraction, true))
	}

	return response, nil
}

func (h *CalendarHandler) GetEvent(ctx context.Context, req *calendarpb.GetEventRequest) (*calendarpb.EventResponse, error) {
	event, err := h.service.GetEvent(ctx, req.EventId, req.UserId)
	if err != nil {
		if errors.Is(err, service.ErrEventNotFound) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to get event: %v", err)
	}

	ipAddress := clientIPFromContext(ctx)
	_ = h.service.IncrementView(ctx, event.ID, ipAddress)

	return h.buildEventResponseForID(ctx, req.EventId, req.UserId, true)
}

func (h *CalendarHandler) FilterByDateRange(ctx context.Context, req *calendarpb.FilterByDateRangeRequest) (*calendarpb.SimplifiedEventsResponse, error) {
	events, err := h.service.FilterByDateRange(ctx, req.StartDate, req.EndDate)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to filter events: %v", err)
	}

	response := &calendarpb.SimplifiedEventsResponse{
		Events: make([]*calendarpb.SimplifiedEventResponse, 0, len(events)),
	}

	for _, event := range events {
		simplified := &calendarpb.SimplifiedEventResponse{
			Id:       event.ID,
			Title:    event.Title,
			StartsAt: jalali.CarbonToJalali(event.StartsAt),
			Color:    event.Color,
		}
		if event.EndsAt != nil {
			simplified.EndsAt = jalali.CarbonToJalali(*event.EndsAt)
		}
		response.Events = append(response.Events, simplified)
	}

	return response, nil
}

func (h *CalendarHandler) GetLatestVersion(ctx context.Context, req *calendarpb.GetLatestVersionRequest) (*calendarpb.LatestVersionResponse, error) {
	versionTitle, err := h.service.GetLatestVersionTitle(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest version: %v", err)
	}

	return &calendarpb.LatestVersionResponse{
		VersionTitle: versionTitle,
	}, nil
}

func (h *CalendarHandler) AddInteraction(ctx context.Context, req *calendarpb.AddInteractionRequest) (*calendarpb.EventResponse, error) {
	if req.Liked < -1 || req.Liked > 1 {
		return nil, status.Errorf(codes.InvalidArgument, "liked value must be -1, 0, or 1")
	}

	ipAddress := clientIPFromContext(ctx)

	if err := h.service.AddInteraction(ctx, req.EventId, req.UserId, req.Liked, ipAddress); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add interaction: %v", err)
	}

	return h.buildEventResponseForID(ctx, req.EventId, req.UserId, false)
}

func (h *CalendarHandler) buildEventResponseForID(ctx context.Context, eventID, userID uint64, includeViews bool) (*calendarpb.EventResponse, error) {
	event, err := h.service.GetEvent(ctx, eventID, userID)
	if err != nil {
		if errors.Is(err, service.ErrEventNotFound) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to get event: %v", err)
	}

	var stats *models.CalendarStats
	if includeViews {
		stats, _ = h.service.GetEventStats(ctx, eventID)
	} else {
		stats, _ = h.service.GetInteractionStats(ctx, eventID)
	}

	var userInteraction *calendarpb.UserInteraction
	if userID > 0 {
		interaction, _ := h.service.GetUserInteraction(ctx, eventID, userID)
		if interaction != nil {
			userInteraction = buildUserInteraction(interaction)
		}
	}

	return buildEventResponse(event, stats, userInteraction, includeViews), nil
}

func buildUserInteraction(interaction *models.Interaction) *calendarpb.UserInteraction {
	return &calendarpb.UserInteraction{
		HasLiked:    interaction.Liked,
		HasDisliked: !interaction.Liked,
	}
}

func calendarIsVersion(event *models.Calendar) bool {
	if event == nil {
		return false
	}
	if event.IsVersion {
		return true
	}
	return event.VersionTitle != nil && *event.VersionTitle != ""
}

func buildEventResponse(event *models.Calendar, stats *models.CalendarStats, userInteraction *calendarpb.UserInteraction, includeViews bool) *calendarpb.EventResponse {
	isVersion := calendarIsVersion(event)

	response := &calendarpb.EventResponse{
		Id:          event.ID,
		Title:       event.Title,
		Description: event.Content,
		StartsAt:    jalali.CarbonToJalaliDateTime(event.StartsAt),
		IsVersion:   isVersion,
	}

	if !isVersion {
		if event.EndsAt != nil {
			response.EndsAt = jalali.CarbonToJalaliDateTime(*event.EndsAt)
		}

		if stats != nil {
			if includeViews {
				response.Views = stats.ViewsCount
			}
			response.Likes = stats.LikesCount
			response.Dislikes = stats.DislikesCount
		}

		if event.BtnName != nil {
			response.BtnName = *event.BtnName
		}
		if event.BtnLink != nil {
			response.BtnLink = *event.BtnLink
		}
		response.Color = event.Color
		if event.Image != nil {
			response.Image = *event.Image
		}

		response.UserInteraction = userInteraction
	} else if event.VersionTitle != nil && *event.VersionTitle != "" {
		response.VersionTitle = *event.VersionTitle
	}

	return response
}

func clientIPFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ips := md.Get("x-forwarded-for"); len(ips) > 0 {
			return ips[0]
		}
		if ips := md.Get("x-real-ip"); len(ips) > 0 {
			return ips[0]
		}
	}
	return "unknown"
}
