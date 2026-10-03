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

func TestNoteService_AddNoteAttachments_AppendsAndSkipsDuplicates(t *testing.T) {
	note := &models.Note{ID: 3, UserID: 9, Title: "t", Content: "c", Attachments: []string{"http://a.pdf"}}
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return noteID == 3 && userID == 9, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			return note, nil
		},
		UpdateFunc: func(ctx context.Context, n *models.Note) error {
			note.Attachments = append([]string{}, n.Attachments...)
			return nil
		},
	}
	svc := service.NewNoteService(repo)
	got, err := svc.AddNoteAttachments(context.Background(), 3, 9, []string{"http://a.pdf", " http://b.docx ", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Attachments) != 2 || got.Attachments[0] != "http://a.pdf" || got.Attachments[1] != "http://b.docx" {
		t.Fatalf("attachments=%v", got.Attachments)
	}
}

func TestNoteService_AddNoteAttachments_RejectsMoreThanFive(t *testing.T) {
	note := &models.Note{ID: 3, UserID: 9, Attachments: []string{"1.pdf", "2.jpg", "3.jpeg", "4.docx"}}
	updated := false
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return true, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			return note, nil
		},
		UpdateFunc: func(ctx context.Context, n *models.Note) error {
			updated = true
			return nil
		},
	}
	svc := service.NewNoteService(repo)
	_, err := svc.AddNoteAttachments(context.Background(), 3, 9, []string{"5.png", "6.pdf"})
	if err == nil || !strings.Contains(err.Error(), "more than 5") {
		t.Fatalf("err=%v", err)
	}
	if updated {
		t.Fatal("update must not run when the note would exceed 5 attachments")
	}
}

func TestNoteService_AddNoteAttachments_Unauthorized(t *testing.T) {
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return false, nil
		},
	}
	svc := service.NewNoteService(repo)
	_, err := svc.AddNoteAttachments(context.Background(), 3, 9, []string{"http://a.pdf"})
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("err=%v", err)
	}
}

func TestNoteService_DeleteNoteAttachment(t *testing.T) {
	note := &models.Note{ID: 3, UserID: 9, Attachments: []string{"http://a.pdf", "http://b.docx"}}
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return true, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			return note, nil
		},
		UpdateFunc: func(ctx context.Context, n *models.Note) error {
			note.Attachments = append([]string{}, n.Attachments...)
			return nil
		},
	}
	svc := service.NewNoteService(repo)
	got, err := svc.DeleteNoteAttachment(context.Background(), 3, 9, "http://a.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Attachments) != 1 || got.Attachments[0] != "http://b.docx" {
		t.Fatalf("attachments=%v", got.Attachments)
	}

	_, err = svc.DeleteNoteAttachment(context.Background(), 3, 9, "missing.pdf")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err=%v", err)
	}

	_, err = svc.DeleteNoteAttachment(context.Background(), 3, 9, "  ")
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("err=%v", err)
	}
}
