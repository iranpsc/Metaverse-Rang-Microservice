package models

import "time"

// Advertisement represents a challenge advertiser entry.
type Advertisement struct {
	Code            string `json:"code"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	InvestmentValue string `json:"investment_value"`
	EndsAt          string `json:"ends_at"`
	VideoURL        string `json:"video_url"`
	ImageURL        string `json:"image_url"`
}

// Question represents a challenge question
// Maps to Laravel: App\Models\Challenge\Question
type Question struct {
	ID           uint64    `json:"id" db:"id"`
	Text         string    `json:"text" db:"title"`
	Prize        string    `json:"prize" db:"prize"` // PSC amount as string
	Views        int32     `json:"views" db:"views"`
	Participants int32     `json:"participants" db:"participants"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Answer represents a possible answer to a question
// Maps to Laravel: App\Models\Challenge\Answer
type Answer struct {
	ID         uint64    `json:"id" db:"id"`
	QuestionID uint64    `json:"question_id" db:"question_id"`
	Text       string    `json:"text" db:"title"`
	IsCorrect  bool      `json:"is_correct" db:"is_correct"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// UserQuestionAnswer represents user's answers to questions
// Maps to Laravel: App\Models\Challenge\UserQuestionAnswer
type UserQuestionAnswer struct {
	ID         uint64    `json:"id" db:"id"`
	UserID     uint64    `json:"user_id" db:"user_id"`
	QuestionID uint64    `json:"question_id" db:"question_id"`
	AnswerID   uint64    `json:"answer_id" db:"answer_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}
