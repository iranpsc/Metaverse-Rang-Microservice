package service_test

import (
	"context"
	"testing"
	"time"

	"metarang/support-service/internal/models"
	"metarang/support-service/internal/service"
	"metarang/support-service/tests/internal/testutil"
)

func TestNoteService_CreateAndGetNotes(t *testing.T) {
	var stored []*models.Note
	repo := &testutil.MockNoteRepo{
		CreateFunc: func(ctx context.Context, note *models.Note) (*models.Note, error) {
			n := *note
			n.ID = 1
			stored = append(stored, &n)
			return &n, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			for _, n := range stored {
				if n.ID == noteID {
					return n, nil
				}
			}
			return nil, nil
		},
		GetByUserIDFunc: func(ctx context.Context, userID uint64) ([]*models.Note, error) {
			return stored, nil
		},
	}
	svc := service.NewNoteService(repo)
	n, err := svc.CreateNote(context.Background(), 10, "t", "c", []string{"http://a"})
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != 1 {
		t.Fatalf("id %d", n.ID)
	}
	list, err := svc.GetNotes(context.Background(), 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("list err=%v len=%d", err, len(list))
	}
}

func TestNoteService_GetNoteUnauthorized(t *testing.T) {
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return false, nil
		},
	}
	svc := service.NewNoteService(repo)
	_, err := svc.GetNote(context.Background(), 1, 2)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNoteService_GetNoteOK(t *testing.T) {
	note := &models.Note{ID: 1, UserID: 2, Title: "x", Content: "y", UpdatedAt: time.Now()}
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return noteID == 1 && userID == 2, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			return note, nil
		},
	}
	svc := service.NewNoteService(repo)
	got, err := svc.GetNote(context.Background(), 1, 2)
	if err != nil || got.Title != "x" {
		t.Fatalf("got %+v err=%v", got, err)
	}
}

func TestNoteService_UpdateNotOwned(t *testing.T) {
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return false, nil
		},
	}
	svc := service.NewNoteService(repo)
	_, err := svc.UpdateNote(context.Background(), 1, 2, "a", "b", nil, true)
	if err == nil {
		t.Fatal("expected unauthorized")
	}
}

func TestNoteService_UpdateDelete(t *testing.T) {
	note := &models.Note{ID: 1, UserID: 2, Title: "old", Content: "oldc", Attachments: []string{"u1"}, UpdatedAt: time.Now()}
	repo := &testutil.MockNoteRepo{
		CheckUserOwnershipFunc: func(ctx context.Context, noteID, userID uint64) (bool, error) {
			return true, nil
		},
		GetByIDFunc: func(ctx context.Context, noteID uint64) (*models.Note, error) {
			return note, nil
		},
		UpdateFunc: func(ctx context.Context, n *models.Note) error {
			*note = *n
			return nil
		},
		DeleteFunc: func(ctx context.Context, noteID uint64) error {
			return nil
		},
	}
	svc := service.NewNoteService(repo)
	got, err := svc.UpdateNote(context.Background(), 1, 2, "n", "nc", []string{"u2"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Attachments) != 1 || got.Attachments[0] != "u2" {
		t.Fatalf("attachments=%v", got.Attachments)
	}
	if err := svc.DeleteNote(context.Background(), 1, 2); err != nil {
		t.Fatal(err)
	}
}
