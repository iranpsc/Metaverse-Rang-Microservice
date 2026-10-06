package service_test

import (
	"context"
	"metarang/auth-service/internal/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

// MockSearchRepository is a mock implementation of SearchRepository
type MockSearchRepository struct {
	mock.Mock
}

func (m *MockSearchRepository) SearchUsers(ctx context.Context, searchTerm string) ([]*repository.SearchUserResult, error) {
	args := m.Called(ctx, searchTerm)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.SearchUserResult), args.Error(1)
}

func (m *MockSearchRepository) SearchIsicCodes(ctx context.Context, searchTerm string) ([]*repository.IsicCodeResult, error) {
	args := m.Called(ctx, searchTerm)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.IsicCodeResult), args.Error(1)
}

func TestSearchService_SearchUsers(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		searchTerm    string
		repoResults   []*repository.SearchUserResult
		repoError     error
		wantResults   int
		wantName      string
		wantCode      string
		wantFollowers int32
		wantLevel     *string
		wantPhoto     *string
		wantError     bool
	}{
		{
			name:       "successful search with KYC verified user",
			searchTerm: "john",
			repoResults: []*repository.SearchUserResult{
				{
					User: &models.User{
						ID:   1,
						Name: "john",
						Code: "usr001",
					},
					KYC: &models.KYC{
						Fname:  "John",
						Lname:  "Smith",
						Status: 1, // Verified
					},
					ProfilePhotos: []*models.Image{
						{URL: "http://example.com/photo1.jpg"},
					},
					Followers:   5,
					LatestLevel: &repository.UserLevel{Name: "Level 3"},
				},
			},
			wantResults:   1,
			wantName:      "John Smith", // Should use KYC name
			wantCode:      "USR001",     // Should be uppercased
			wantFollowers: 5,
			wantLevel:     stringPtr("Level 3"),
			wantPhoto:     stringPtr("http://example.com/photo1.jpg"),
			wantError:     false,
		},
		{
			name:       "successful search with non-verified user",
			searchTerm: "jane",
			repoResults: []*repository.SearchUserResult{
				{
					User: &models.User{
						ID:   2,
						Name: "jane doe",
						Code: "usr002",
					},
					KYC: &models.KYC{
						Fname:  "Jane",
						Lname:  "Doe",
						Status: 0, // Not verified
					},
					Followers: 0,
				},
			},
			wantResults:   1,
			wantName:      "jane doe", // Should use user name
			wantCode:      "USR002",
			wantFollowers: 0,
			wantError:     false,
		},
		{
			name:        "empty search term",
			searchTerm:  "",
			wantResults: 0,
			wantError:   false,
		},
		{
			name:       "repository error",
			searchTerm: "error",
			repoError:  assert.AnError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSearchRepository)
			if tt.searchTerm != "" {
				mockRepo.On("SearchUsers", ctx, tt.searchTerm).Return(tt.repoResults, tt.repoError)
			}

			svc := service.NewSearchService(mockRepo)
			results, err := svc.SearchUsers(ctx, tt.searchTerm)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, results)
			} else {
				require.NoError(t, err)
				assert.Len(t, results, tt.wantResults)

				if tt.wantResults > 0 && len(results) > 0 {
					assert.Equal(t, tt.wantName, results[0].Name)
					assert.Equal(t, tt.wantCode, results[0].Code)
					assert.Equal(t, tt.wantFollowers, results[0].Followers)
					if tt.wantLevel != nil {
						assert.Equal(t, *tt.wantLevel, *results[0].Level)
					}
					if tt.wantPhoto != nil {
						assert.Equal(t, *tt.wantPhoto, *results[0].Photo)
					}
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSearchService_SearchIsicCodes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		searchTerm  string
		repoResults []*repository.IsicCodeResult
		repoError   error
		wantResults int
		wantError   bool
	}{
		{
			name:       "successful search",
			searchTerm: "manufacturing",
			repoResults: []*repository.IsicCodeResult{
				{ID: 1, Name: "Manufacture of textiles", Code: 1311},
				{ID: 2, Name: "Manufacture of beverages", Code: 1104},
			},
			wantResults: 2,
			wantError:   false,
		},
		{
			name:        "empty search term",
			searchTerm:  "",
			wantResults: 0,
			wantError:   false,
		},
		{
			name:       "repository error",
			searchTerm: "error",
			repoError:  assert.AnError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSearchRepository)
			if tt.searchTerm != "" {
				mockRepo.On("SearchIsicCodes", ctx, tt.searchTerm).Return(tt.repoResults, tt.repoError)
			}

			svc := service.NewSearchService(mockRepo)
			results, err := svc.SearchIsicCodes(ctx, tt.searchTerm)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, results)
			} else {
				require.NoError(t, err)
				assert.Len(t, results, tt.wantResults)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
