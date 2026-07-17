package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/social-service/internal/models"
)

type ChallengeRepository interface {
	GetRandomUnansweredQuestion(ctx context.Context, userID uint64) (*models.Question, error)
	GetQuestionByID(ctx context.Context, questionID uint64) (*models.Question, error)
	GetAnswersByQuestionID(ctx context.Context, questionID uint64) ([]*models.Answer, error)
	IncrementQuestionViews(ctx context.Context, questionID uint64) error
	IncrementQuestionParticipants(ctx context.Context, questionID uint64) error
	CreateUserAnswer(ctx context.Context, userID, questionID, answerID uint64) error
	HasUserAnswered(ctx context.Context, userID, questionID uint64) (bool, error)
	GetUserAnswerCount(ctx context.Context, userID uint64, isCorrect bool) (int32, error)
	GetTotalParticipantsCount(ctx context.Context) (int32, error)
	GetSystemVariable(ctx context.Context, slug string) (float64, error)
	GetAnswerVoteCount(ctx context.Context, answerID uint64) (int32, error)
	GetQuestionTotalAnswers(ctx context.Context, questionID uint64) (int32, error)
}

type challengeRepository struct {
	db *sql.DB
}

func NewChallengeRepository(db *sql.DB) ChallengeRepository {
	return &challengeRepository{db: db}
}

func (r *challengeRepository) GetRandomUnansweredQuestion(ctx context.Context, userID uint64) (*models.Question, error) {
	// A question is no longer eligible after the user's first answer,
	// regardless of whether that answer was correct.
	query := `
		SELECT q.id, q.code, q.title, q.image, q.creator_code, q.prize, q.views, q.participants, q.created_at, q.updated_at
		FROM questions q
		WHERE NOT EXISTS (
			SELECT 1 FROM user_question_answers uqa
			WHERE uqa.user_id = ? AND uqa.question_id = q.id
		)
		ORDER BY RAND()
		LIMIT 1
	`
	question := &models.Question{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&question.ID, &question.Code, &question.Title, &question.Image,
		&question.CreatorCode, &question.Prize, &question.Views,
		&question.Participants, &question.CreatedAt, &question.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get random question: %w", err)
	}
	return question, nil
}

func (r *challengeRepository) GetQuestionByID(ctx context.Context, questionID uint64) (*models.Question, error) {
	query := `
		SELECT id, code, title, image, creator_code, prize, views, participants, created_at, updated_at
		FROM questions
		WHERE id = ?
	`
	question := &models.Question{}
	err := r.db.QueryRowContext(ctx, query, questionID).Scan(
		&question.ID, &question.Code, &question.Title, &question.Image,
		&question.CreatorCode, &question.Prize, &question.Views,
		&question.Participants, &question.CreatedAt, &question.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get question: %w", err)
	}
	return question, nil
}

func (r *challengeRepository) GetAnswersByQuestionID(ctx context.Context, questionID uint64) ([]*models.Answer, error) {
	query := `
		SELECT id, question_id, title, image, is_correct, created_at, updated_at
		FROM answers
		WHERE question_id = ?
		ORDER BY id
	`
	rows, err := r.db.QueryContext(ctx, query, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get answers: %w", err)
	}
	defer rows.Close()

	var answers []*models.Answer
	for rows.Next() {
		answer := &models.Answer{}
		if err := rows.Scan(
			&answer.ID, &answer.QuestionID, &answer.Title, &answer.Image,
			&answer.IsCorrect, &answer.CreatedAt, &answer.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan answer: %w", err)
		}
		answers = append(answers, answer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating answers: %w", err)
	}
	return answers, nil
}

func (r *challengeRepository) IncrementQuestionViews(ctx context.Context, questionID uint64) error {
	query := `UPDATE questions SET views = views + 1 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, questionID)
	if err != nil {
		return fmt.Errorf("failed to increment views: %w", err)
	}
	return nil
}

func (r *challengeRepository) IncrementQuestionParticipants(ctx context.Context, questionID uint64) error {
	query := `UPDATE questions SET participants = participants + 1 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, questionID)
	if err != nil {
		return fmt.Errorf("failed to increment participants: %w", err)
	}
	return nil
}

func (r *challengeRepository) CreateUserAnswer(ctx context.Context, userID, questionID, answerID uint64) error {
	query := `
		INSERT INTO user_question_answers (user_id, question_id, answer_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`
	_, err := r.db.ExecContext(ctx, query, userID, questionID, answerID)
	if err != nil {
		return fmt.Errorf("failed to create user answer: %w", err)
	}
	return nil
}

func (r *challengeRepository) HasUserAnswered(ctx context.Context, userID, questionID uint64) (bool, error) {
	query := `
		SELECT COUNT(*) FROM user_question_answers
		WHERE user_id = ? AND question_id = ?
	`
	var count int
	err := r.db.QueryRowContext(ctx, query, userID, questionID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check previous answer: %w", err)
	}
	return count > 0, nil
}

func (r *challengeRepository) GetUserAnswerCount(ctx context.Context, userID uint64, isCorrect bool) (int32, error) {
	query := `
		SELECT COUNT(*) FROM user_question_answers uqa
		INNER JOIN answers a ON uqa.answer_id = a.id
		WHERE uqa.user_id = ? AND a.is_correct = ?
	`
	var count int
	err := r.db.QueryRowContext(ctx, query, userID, isCorrect).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get answer count: %w", err)
	}
	return int32(count), nil
}

func (r *challengeRepository) GetTotalParticipantsCount(ctx context.Context) (int32, error) {
	query := `SELECT COUNT(DISTINCT user_id) FROM user_question_answers`
	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get total participants: %w", err)
	}
	return int32(count), nil
}

func (r *challengeRepository) GetSystemVariable(ctx context.Context, slug string) (float64, error) {
	query := `SELECT value FROM system_variables WHERE slug = ?`
	var value float64
	err := r.db.QueryRowContext(ctx, query, slug).Scan(&value)
	if err == sql.ErrNoRows {
		// Return default value of 15 if not found
		return 15.0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get system variable: %w", err)
	}
	return value, nil
}

func (r *challengeRepository) GetAnswerVoteCount(ctx context.Context, answerID uint64) (int32, error) {
	query := `SELECT COUNT(*) FROM user_question_answers WHERE answer_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, answerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get vote count: %w", err)
	}
	return int32(count), nil
}

func (r *challengeRepository) GetQuestionTotalAnswers(ctx context.Context, questionID uint64) (int32, error) {
	query := `SELECT COUNT(*) FROM user_question_answers WHERE question_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, questionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get total answers: %w", err)
	}
	return int32(count), nil
}
