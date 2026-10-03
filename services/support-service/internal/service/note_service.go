// Package service implements business logic for the support service.
package service

import (
	"context"
	"fmt"
	"strings"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/repository"
)

const maxNoteAttachments = 5

type NoteService interface {
	CreateNote(ctx context.Context, userID uint64, title, content string, attachments []string) (*models.Note, error)
	GetNotes(ctx context.Context, userID uint64) ([]*models.Note, error)
	GetNote(ctx context.Context, noteID, userID uint64) (*models.Note, error)
	UpdateNote(ctx context.Context, noteID, userID uint64, title, content string, attachments []string, replaceAttachments bool) (*models.Note, error)
	DeleteNote(ctx context.Context, noteID, userID uint64) error
	AddNoteAttachments(ctx context.Context, noteID, userID uint64, attachments []string) (*models.Note, error)
	DeleteNoteAttachment(ctx context.Context, noteID, userID uint64, attachment string) (*models.Note, error)
}

type noteService struct {
	noteRepo repository.NoteRepository
}

func NewNoteService(noteRepo repository.NoteRepository) NoteService {
	return &noteService{
		noteRepo: noteRepo,
	}
}

func (s *noteService) CreateNote(ctx context.Context, userID uint64, title, content string, attachments []string) (*models.Note, error) {
	if err := validateNoteAttachmentLimit(attachments); err != nil {
		return nil, err
	}

	note := &models.Note{
		Title:       title,
		Content:     content,
		Attachments: attachments,
		UserID:      userID,
	}

	created, err := s.noteRepo.Create(ctx, note)
	if err != nil {
		return nil, err
	}

	return s.noteRepo.GetByID(ctx, created.ID)
}

func (s *noteService) GetNotes(ctx context.Context, userID uint64) ([]*models.Note, error) {
	return s.noteRepo.GetByUserID(ctx, userID)
}

func (s *noteService) GetNote(ctx context.Context, noteID, userID uint64) (*models.Note, error) {
	owned, err := s.noteRepo.CheckUserOwnership(ctx, noteID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check ownership: %w", err)
	}
	if !owned {
		return nil, fmt.Errorf("unauthorized: you don't have permission to view this note")
	}

	return s.noteRepo.GetByID(ctx, noteID)
}

func (s *noteService) UpdateNote(ctx context.Context, noteID, userID uint64, title, content string, attachments []string, replaceAttachments bool) (*models.Note, error) {
	owned, err := s.noteRepo.CheckUserOwnership(ctx, noteID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check ownership: %w", err)
	}
	if !owned {
		return nil, fmt.Errorf("unauthorized: you don't have permission to update this note")
	}

	note, err := s.noteRepo.GetByID(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}
	if note == nil {
		return nil, fmt.Errorf("note not found")
	}

	note.Title = title
	note.Content = content
	if replaceAttachments {
		if err := validateNoteAttachmentLimit(attachments); err != nil {
			return nil, err
		}
		note.Attachments = attachments
	}

	err = s.noteRepo.Update(ctx, note)
	if err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	return s.noteRepo.GetByID(ctx, noteID)
}

func (s *noteService) AddNoteAttachments(ctx context.Context, noteID, userID uint64, attachments []string) (*models.Note, error) {
	note, err := s.ownedNote(ctx, noteID, userID, "update")
	if err != nil {
		return nil, err
	}

	merged := append([]string{}, note.Attachments...)
	seen := make(map[string]struct{}, len(merged))
	for _, existing := range merged {
		seen[existing] = struct{}{}
	}
	for _, attachment := range attachments {
		attachment = strings.TrimSpace(attachment)
		if attachment == "" {
			continue
		}
		if _, ok := seen[attachment]; ok {
			continue
		}
		merged = append(merged, attachment)
		seen[attachment] = struct{}{}
	}
	if err := validateNoteAttachmentLimit(merged); err != nil {
		return nil, err
	}

	note.Attachments = merged
	if err := s.noteRepo.Update(ctx, note); err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}
	return s.noteRepo.GetByID(ctx, noteID)
}

func (s *noteService) DeleteNoteAttachment(ctx context.Context, noteID, userID uint64, attachment string) (*models.Note, error) {
	attachment = strings.TrimSpace(attachment)
	if attachment == "" {
		return nil, fmt.Errorf("attachment is required")
	}

	note, err := s.ownedNote(ctx, noteID, userID, "update")
	if err != nil {
		return nil, err
	}

	next := make([]string, 0, len(note.Attachments))
	found := false
	for _, existing := range note.Attachments {
		if !found && existing == attachment {
			found = true
			continue
		}
		next = append(next, existing)
	}
	if !found {
		return nil, fmt.Errorf("attachment not found")
	}

	note.Attachments = next
	if err := s.noteRepo.Update(ctx, note); err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}
	return s.noteRepo.GetByID(ctx, noteID)
}

func validateNoteAttachmentLimit(attachments []string) error {
	if len(attachments) > maxNoteAttachments {
		return fmt.Errorf("attachments must not have more than 5 items")
	}
	return nil
}

func (s *noteService) ownedNote(ctx context.Context, noteID, userID uint64, action string) (*models.Note, error) {
	owned, err := s.noteRepo.CheckUserOwnership(ctx, noteID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check ownership: %w", err)
	}
	if !owned {
		return nil, fmt.Errorf("unauthorized: you don't have permission to %s this note", action)
	}

	note, err := s.noteRepo.GetByID(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}
	if note == nil {
		return nil, fmt.Errorf("note not found")
	}
	return note, nil
}

func (s *noteService) DeleteNote(ctx context.Context, noteID, userID uint64) error {
	owned, err := s.noteRepo.CheckUserOwnership(ctx, noteID, userID)
	if err != nil {
		return fmt.Errorf("failed to check ownership: %w", err)
	}
	if !owned {
		return fmt.Errorf("unauthorized: you don't have permission to delete this note")
	}

	return s.noteRepo.Delete(ctx, noteID)
}
