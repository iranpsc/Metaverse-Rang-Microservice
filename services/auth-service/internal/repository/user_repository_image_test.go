package repository

import "testing"

func TestUserRepository_formatImageURL(t *testing.T) {
	tests := []struct {
		name          string
		adminPanelURL string
		input         string
		want          string
	}{
		{
			name:          "empty input",
			adminPanelURL: "http://admin.example.com",
			input:         "",
			want:          "",
		},
		{
			name:          "full URL unchanged",
			adminPanelURL: "http://admin.example.com",
			input:         "https://cdn.example.com/level.png",
			want:          "https://cdn.example.com/level.png",
		},
		{
			name:          "relative path with admin panel",
			adminPanelURL: "http://admin.example.com",
			input:         "levels/1.png",
			want:          "http://admin.example.com/uploads/levels/1.png",
		},
		{
			name:          "uploads path without admin panel",
			adminPanelURL: "",
			input:         "levels/1.png",
			want:          "/uploads/levels/1.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &userRepository{adminPanelURL: tt.adminPanelURL}
			got := r.formatImageURL(tt.input)
			if got != tt.want {
				t.Errorf("formatImageURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
