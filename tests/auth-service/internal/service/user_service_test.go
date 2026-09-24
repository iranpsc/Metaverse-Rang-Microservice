package service_test

import (
	"context"
	"errors"
	"testing"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
)

type fakeSettingsRepoForUser struct {
	settings *models.Settings
}

func (f *fakeSettingsRepoForUser) FindByUserID(context.Context, uint64) (*models.Settings, error) {
	return f.settings, nil
}
func (f *fakeSettingsRepoForUser) FindByID(context.Context, uint64) (*models.Settings, error) {
	return f.settings, nil
}
func (f *fakeSettingsRepoForUser) Update(context.Context, *models.Settings) error { return nil }
func (f *fakeSettingsRepoForUser) Create(context.Context, *models.Settings) error { return nil }

var _ repository.SettingsRepository = (*fakeSettingsRepoForUser)(nil)

func TestUserService_Core(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepository(map[uint64]*models.User{
		1: {ID: 1, Name: "Ada", Email: "a@x.com", Code: "hm-1", Score: 50},
	})
	users.updateFunc = func(_ context.Context, u *models.User) error {
		users.users[u.ID] = u
		return nil
	}
	users.listUsersFunc = func(context.Context, string, string, int32, int32) ([]*repository.UserWithRelations, int32, error) {
		u := users.users[1]
		name := "KYC Name"
		return []*repository.UserWithRelations{{User: u, KYCName: &name}}, 1, nil
	}
	users.getUsersLevelsForListFunc = func(context.Context, []uint64) (map[uint64]*repository.UserListLevels, error) {
		return map[uint64]*repository.UserListLevels{
			1: {Current: &repository.UserListLevel{ID: 2, Name: "L", Slug: "l", Score: 10}},
		}, nil
	}
	users.getUserLatestLevelFunc = func(context.Context, uint64) (*repository.UserLevel, error) {
		return &repository.UserLevel{ID: 2, Name: "L", Slug: "l", Score: 10}, nil
	}
	users.getLevelsBelowScoreFunc = func(context.Context, int32) ([]*repository.UserLevel, error) {
		return []*repository.UserLevel{{ID: 1, Name: "P", Slug: "p", Score: 1}}, nil
	}
	users.getNextLevelScoreFunc = func(context.Context, int32) (int32, error) { return 100, nil }
	users.getFollowersCountFunc = func(context.Context, uint64) (int32, error) { return 3, nil }
	users.getFollowingCountFunc = func(context.Context, uint64) (int32, error) { return 2, nil }
	users.getAllProfilePhotoURLsFunc = func(context.Context, uint64) ([]string, error) {
		return []string{"/a.jpg"}, nil
	}
	users.getFeatureCountsFunc = func(context.Context, uint64) (int32, int32, int32, error) {
		return 1, 2, 3, nil
	}

	settings := &fakeSettingsRepoForUser{
		settings: &models.Settings{Privacy: map[string]int{
			"name": 1, "registered_at": 1, "followers_count": 1, "following_count": 1,
		}},
	}
	svc := service.NewUserServiceWithDependencies(users, nil, settings, nil)

	t.Run("get and update", func(t *testing.T) {
		u, err := svc.GetUser(ctx, 1)
		if err != nil || u.Name != "Ada" {
			t.Fatalf("%v %v", u, err)
		}
		_, err = svc.GetUser(ctx, 99)
		if err == nil {
			t.Fatal("expected not found")
		}
		u, err = svc.UpdateProfile(ctx, 1, "Bob", "b@x.com", "0912")
		if err != nil || u.Name != "Bob" || !u.Phone.Valid {
			t.Fatalf("%+v err=%v", u, err)
		}
		_, err = svc.UpdateProfile(ctx, 1, "Bob", "evil@example.com\nCc: attacker@evil.com", "0912")
		if !errors.Is(err, service.ErrInvalidEmailFormat) {
			t.Fatalf("expected invalid email format, got %v", err)
		}
	})

	t.Run("list levels profile features", func(t *testing.T) {
		items, total, limit, err := svc.ListUsers(ctx, "", "", 0)
		if err != nil || total != 1 || limit != 20 || len(items) != 1 || items[0].Name != "KYC Name" {
			t.Fatalf("items=%+v total=%d limit=%d err=%v", items, total, limit, err)
		}
		levels, err := svc.GetUserLevels(ctx, 1)
		if err != nil || levels.LatestLevel == nil || len(levels.PreviousLevels) != 1 {
			t.Fatalf("%+v err=%v", levels, err)
		}
		viewer := uint64(2)
		profile, err := svc.GetUserProfile(ctx, 1, &viewer)
		if err != nil || profile.FollowersCount == nil || *profile.FollowersCount != 3 {
			t.Fatalf("%+v err=%v", profile, err)
		}
		fc, err := svc.GetUserFeaturesCount(ctx, 1)
		if err != nil || fc.MaskoniFeaturesCount != 1 {
			t.Fatalf("%+v err=%v", fc, err)
		}
	})
}
