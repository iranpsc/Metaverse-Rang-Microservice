package service_test

import (
	"context"
	"strings"
	"testing"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/service"
	"metarang/support-service/tests/internal/testutil"
)

func TestNoteService_CreateNote_StoresUpToFiveAttachments(t *testing.T) {
	var stored *models.Note
	created := 0
	repo := &testutil.MockNoteRepo{
		CreateFunc: func(ctx context.Context, note *models.Note) (*models.Note, error) {
			created++
			n := *note
			n.ID = 8
			n.Attachments = append([]string{}, note.Attachments...)
			stored = &n
			return &n, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			return stored, nil
		},
	}
	svc := service.NewNoteService(repo)
	atts := []string{"1.pdf", "2.docx", "3.jpg", "4.jpeg", "5.png"}
	got, err := svc.CreateNote(context.Background(), 9, "t", "c", atts)
	if err != nil || len(got.Attachments) != 5 || got.Attachments[4] != "5.png" {
		t.Fatalf("got=%v err=%v", got, err)
	}

	_, err = svc.CreateNote(context.Background(), 9, "t", "c", append(atts, "6.pdf"))
	if err == nil || !strings.Contains(err.Error(), "more than 5") {
		t.Fatalf("err=%v", err)
	}
	if created != 1 {
		t.Fatalf("create calls=%d", created)
	}
}

func TestNoteService_UpdateNote_ReplacesListAndDropsRemovedLinks(t *testing.T) {
	note := &models.Note{ID: 3, UserID: 9, Title: "t", Content: "c", Attachments: []string{"1.pdf", "2.docx", "3.jpg"}}
	updated := 0
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return true, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			cp := *note
			cp.Attachments = append([]string{}, note.Attachments...)
			return &cp, nil
		},
		UpdateFunc: func(ctx context.Context, n *models.Note) error {
			updated++
			note.Attachments = append([]string{}, n.Attachments...)
			note.Title = n.Title
			note.Content = n.Content
			return nil
		},
	}
	svc := service.NewNoteService(repo)
	got, err := svc.UpdateNote(context.Background(), 3, 9, "t2", "c2", []string{"1.pdf", "4.jpeg"}, true)
	if err != nil || len(got.Attachments) != 2 || got.Attachments[0] != "1.pdf" || got.Attachments[1] != "4.jpeg" {
		t.Fatalf("got=%v err=%v", got, err)
	}

	_, err = svc.UpdateNote(context.Background(), 3, 9, "t2", "c2", []string{"a", "b", "c", "d", "e", "f"}, true)
	if err == nil || !strings.Contains(err.Error(), "more than 5") {
		t.Fatalf("err=%v", err)
	}
	if updated != 1 || len(note.Attachments) != 2 || note.Attachments[1] != "4.jpeg" {
		t.Fatalf("updated=%d attachments=%v", updated, note.Attachments)
	}

	got, err = svc.UpdateNote(context.Background(), 3, 9, "kept", "c2", nil, true)
	if err != nil || len(got.Attachments) != 2 || got.Title != "kept" {
		t.Fatalf("keep got=%v err=%v", got, err)
	}

	got, err = svc.UpdateNote(context.Background(), 3, 9, "cleared", "c2", []string{}, true)
	if err != nil || len(got.Attachments) != 0 {
		t.Fatalf("clear got=%v err=%v", got, err)
	}
}
