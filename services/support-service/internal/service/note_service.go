// Package service implements business logic for the support service.
package service

import (
	"context"
	"fmt"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/repository"
)

type NoteService interface {
	CreateNote(ctx context.Context, userID uint64, title, content string, attachments []string) (*models.Note, error)
	GetNotes(ctx context.Context, userID uint64) ([]*models.Note, error)
	GetNote(ctx context.Context, noteID, userID uint64) (*models.Note, error)
	UpdateNote(ctx context.Context, noteID, userID uint64, title, content string, attachments []string, replaceAttachments bool) (*models.Note, error)
	DeleteNote(ctx context.Context, noteID, userID uint64) error
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
		note.Attachments = attachments
	}

	err = s.noteRepo.Update(ctx, note)
	if err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	return s.noteRepo.GetByID(ctx, noteID)
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
