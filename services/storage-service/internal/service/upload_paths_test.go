package service

import (
	"path/filepath"
	"testing"
)

func TestNormalizeUploadSubdir(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "profile API path", input: "/uploads/profile", want: "profile"},
		{name: "profile without leading slash", input: "uploads/profile", want: "profile"},
		{name: "kyc path", input: "/uploads/kyc", want: "kyc"},
		{name: "subdir only", input: "profile", want: "profile"},
		{name: "empty", input: "", want: ""},
		{name: "uploads root", input: "/uploads", want: ""},
		{name: "path traversal", input: "../../etc", wantErr: true},
		{name: "nested path", input: "profile/nested", wantErr: true},
		{name: "disallowed subdir", input: "/uploads/evil", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeUploadSubdir(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeUploadSubdir(%q) expected error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeUploadSubdir(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeUploadSubdir(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveChunkLocalPath(t *testing.T) {
	base := filepath.Join("data", "uploads")
	got, err := resolveChunkLocalPath(base, filepath.Join("profile", "abc.jpg"), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "profile", "abc.jpg")
	if got != want {
		t.Fatalf("custom upload local path = %q, want %q", got, want)
	}

	if _, err := resolveChunkLocalPath(base, filepath.Join("..", "escape.jpg"), true); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}

	defaultPath := filepath.Join("uploads", "image-jpeg", "2024-01-01", "abc.jpg")
	if got, err := resolveChunkLocalPath(base, defaultPath, false); err != nil || got != defaultPath {
		t.Fatalf("default upload local path = %q, want %q (err=%v)", got, defaultPath, err)
	}
}

func TestResolveChunkPublicDir(t *testing.T) {
	if got := resolveChunkPublicDir("", "profile", true); got != "/uploads/profile/" {
		t.Fatalf("custom public dir = %q, want /uploads/profile/", got)
	}

	relative := filepath.Join("uploads", "image-jpeg", "2024-01-01", "abc.jpg")
	if got := resolveChunkPublicDir(relative, "", false); got != "uploads/image-jpeg/2024-01-01/" {
		t.Fatalf("default public dir = %q, want uploads/image-jpeg/2024-01-01/", got)
	}
}

func TestSanitizeUploadID(t *testing.T) {
	if err := sanitizeUploadID("valid-upload-id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := sanitizeUploadID("../bad"); err == nil {
		t.Fatal("expected traversal upload_id to be rejected")
	}
}
