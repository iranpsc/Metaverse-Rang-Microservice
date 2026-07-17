package models

import "time"

// Follow represents a follow relationship between two users
type Follow struct {
	ID          uint64    `db:"id"`
	FollowerID  uint64    `db:"follower_id"`
	FollowingID uint64    `db:"following_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// FollowResource represents a user in follow lists
type FollowResource struct {
	ID            uint64
	Name          string
	Code          string
	ProfilePhotos []string
	Level         string
	Online        bool
	Followed      bool
	Can           FollowPermissions
}

// FollowPermissions mirrors Laravel FollowResource "can" — actions the
// authenticated (viewer) user may take on the listed user.
type FollowPermissions struct {
	Follow         bool
	Unfollow       bool
	RemoveFollower bool
}
