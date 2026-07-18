// Package models defines domain types for the social service.
package models

import "time"

// Question represents a challenge question
type Question struct {
	ID           uint64    `db:"id"`
	Code         string    `db:"code"`
	Title        string    `db:"title"`
	Image        string    `db:"image"`
	CreatorCode  string    `db:"creator_code"`
	Prize        int32     `db:"prize"`
	Views        uint64    `db:"views"`
	Participants uint64    `db:"participants"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// Answer represents a possible answer to a question
type Answer struct {
	ID         uint64    `db:"id"`
	QuestionID uint64    `db:"question_id"`
	Title      string    `db:"title"`
	Image      string    `db:"image"`
	IsCorrect  bool      `db:"is_correct"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// UserQuestionAnswer represents a user's answer to a question
type UserQuestionAnswer struct {
	ID         uint64    `db:"id"`
	UserID     uint64    `db:"user_id"`
	QuestionID uint64    `db:"question_id"`
	AnswerID   uint64    `db:"answer_id"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// SystemVariable represents a system variable
type SystemVariable struct {
	ID        uint64    `db:"id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"`
	Value     float64   `db:"value"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// QuestionResource represents a question with its answers for API response
type QuestionResource struct {
	ID           uint64
	Title        string
	Image        string
	Prize        int32
	Participants uint64
	Views        uint64
	CreatorCode  string
	Answers      []AnswerResource
}

// AnswerResource represents an answer for API response
type AnswerResource struct {
	ID             uint64
	Title          string
	Image          string
	IsCorrect      bool
	VotePercentage int32
}

// Advertisement represents a challenge advertisement.
type Advertisement struct {
	Code            string
	Title           string
	Description     string
	InvestmentValue string
	EndsAt          string
	VideoURL        string
	ImageURL        string
	URL             string
	InvestmentAsset string
}

// TimingsData represents challenge timing configuration and statistics
type TimingsData struct {
	DisplayAdInterval       int32
	DisplayQuestionInterval int32
	DisplayAnswerInterval   int32
	Participants            int32
	CorrectAnswers          int32
	WrongAnswers            int32
}
