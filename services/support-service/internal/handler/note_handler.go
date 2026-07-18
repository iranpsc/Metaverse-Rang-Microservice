// Package handler implements gRPC handlers for the support service.
package handler

import (
	"context"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/service"
	"metarang/support-service/internal/utils"

	pbCommon "metarang/shared/pb/common"
	pb "metarang/shared/pb/support"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NoteHandler struct {
	pb.UnimplementedNoteServiceServer
	noteService service.NoteService
}

func NewNoteHandler(noteService service.NoteService) *NoteHandler {
	return &NoteHandler{
		noteService: noteService,
	}
}

func RegisterNoteHandler(grpcServer *grpc.Server, noteService service.NoteService) {
	handler := NewNoteHandler(noteService)
	pb.RegisterNoteServiceServer(grpcServer, handler)
}

func handlerLocale(ctx context.Context) string {
	_ = ctx
	return "en"
}

func (h *NoteHandler) CreateNote(ctx context.Context, req *pb.CreateNoteRequest) (*pb.NoteResponse, error) {
	locale := handlerLocale(ctx)
	validationErrors := mergeValidationErrors(
		validateRequired("user_id", req.UserId, locale),
		validateRequired("title", req.Title, locale),
		validateRequired("content", req.Content, locale),
		validateMaxLen("title", req.Title, 130, locale),
		validateMaxLen("content", req.Content, 2000, locale),
	)
	if len(req.Attachments) > 5 {
		validationErrors = mergeValidationErrors(validationErrors, map[string]string{
			"attachments": "The attachments field must not have more than 5 items",
		})
	}
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	atts := req.Attachments
	if atts == nil {
		atts = nil
	}

	note, err := h.noteService.CreateNote(ctx, req.UserId, req.Title, req.Content, atts)
	if err != nil {
		return nil, MapServiceError(err)
	}

	return convertNoteToProto(note), nil
}

func (h *NoteHandler) GetNotes(ctx context.Context, req *pb.GetNotesRequest) (*pb.NotesResponse, error) {
	locale := handlerLocale(ctx)
	validationErrors := validateRequired("user_id", req.UserId, locale)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	notes, err := h.noteService.GetNotes(ctx, req.UserId)
	if err != nil {
		return nil, MapServiceError(err)
	}

	response := &pb.NotesResponse{
		Notes: make([]*pb.NoteResponse, len(notes)),
	}

	for i, note := range notes {
		response.Notes[i] = convertNoteToProto(note)
	}

	return response, nil
}

func (h *NoteHandler) GetNote(ctx context.Context, req *pb.GetNoteRequest) (*pb.NoteResponse, error) {
	locale := handlerLocale(ctx)
	validationErrors := mergeValidationErrors(
		validateRequired("note_id", req.NoteId, locale),
		validateRequired("user_id", req.UserId, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	note, err := h.noteService.GetNote(ctx, req.NoteId, req.UserId)
	if err != nil {
		return nil, MapServiceError(err)
	}

	if note == nil {
		return nil, status.Error(codes.NotFound, "note not found")
	}

	return convertNoteToProto(note), nil
}

func (h *NoteHandler) UpdateNote(ctx context.Context, req *pb.UpdateNoteRequest) (*pb.NoteResponse, error) {
	locale := handlerLocale(ctx)
	validationErrors := mergeValidationErrors(
		validateRequired("note_id", req.NoteId, locale),
		validateRequired("user_id", req.UserId, locale),
		validateRequired("title", req.Title, locale),
		validateRequired("content", req.Content, locale),
		validateMaxLen("title", req.Title, 130, locale),
		validateMaxLen("content", req.Content, 2000, locale),
	)
	if len(req.Attachments) > 5 {
		validationErrors = mergeValidationErrors(validationErrors, map[string]string{
			"attachments": "The attachments field must not have more than 5 items",
		})
	}
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	atts := req.Attachments
	note, err := h.noteService.UpdateNote(ctx, req.NoteId, req.UserId, req.Title, req.Content, atts, true)
	if err != nil {
		return nil, MapServiceError(err)
	}

	return convertNoteToProto(note), nil
}

func (h *NoteHandler) DeleteNote(ctx context.Context, req *pb.DeleteNoteRequest) (*pbCommon.Empty, error) {
	locale := handlerLocale(ctx)
	validationErrors := mergeValidationErrors(
		validateRequired("note_id", req.NoteId, locale),
		validateRequired("user_id", req.UserId, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	err := h.noteService.DeleteNote(ctx, req.NoteId, req.UserId)
	if err != nil {
		return nil, MapServiceError(err)
	}

	return &pbCommon.Empty{}, nil
}

func convertNoteToProto(note *models.Note) *pb.NoteResponse {
	if note == nil {
		return nil
	}
	atts := note.Attachments
	if atts == nil {
		atts = []string{}
	}
	return &pb.NoteResponse{
		Id:          note.ID,
		Title:       note.Title,
		Content:     note.Content,
		Attachments: atts,
		Date:        utils.FormatJalaliDate(note.UpdatedAt),
		Time:        utils.FormatJalaliTime(note.UpdatedAt),
	}
}
