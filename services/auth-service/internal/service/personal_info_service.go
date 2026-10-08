package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

var (
	ErrInvalidOccupation     = errors.New("occupation must be at most 255 characters")
	ErrInvalidEducation      = errors.New("education must be at most 255 characters")
	ErrInvalidMemory         = errors.New("memory must be at most 20000 characters")
	ErrInvalidLovedCity      = errors.New("loved_city must be at most 255 characters")
	ErrInvalidLovedCountry   = errors.New("loved_country must be at most 255 characters")
	ErrInvalidLovedLanguage  = errors.New("loved_language must be at most 255 characters")
	ErrInvalidProblemSolving = errors.New("problem_solving must be at most 20000 characters")
	ErrInvalidPrediction     = errors.New("prediction must be at most 20000 characters")
	ErrInvalidAbout          = errors.New("about must be at most 10000 characters")
	ErrInvalidPassionKey     = errors.New("invalid passion key")
)

type PersonalInfoService interface {
	GetPersonalInfo(ctx context.Context, userID uint64) (*models.PersonalInfo, error)
	UpdatePersonalInfo(ctx context.Context, userID uint64, occupation, education, memory, lovedCity, lovedCountry, lovedLanguage, problemSolving, prediction, about string, passions map[string]bool) error
}

type personalInfoService struct {
	personalInfoRepo repository.PersonalInfoRepository
}

func NewPersonalInfoService(personalInfoRepo repository.PersonalInfoRepository) PersonalInfoService {
	return &personalInfoService{
		personalInfoRepo: personalInfoRepo,
	}
}

func (s *personalInfoService) GetPersonalInfo(ctx context.Context, userID uint64) (*models.PersonalInfo, error) {
	personalInfo, err := s.personalInfoRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get personal info: %w", err)
	}
	// Return nil if not found (handler will return empty array)
	return personalInfo, nil
}

func (s *personalInfoService) UpdatePersonalInfo(ctx context.Context, userID uint64, occupation, education, memory, lovedCity, lovedCountry, lovedLanguage, problemSolving, prediction, about string, passions map[string]bool) error {
	// Validate input
	if err := s.validatePersonalInfoInput(occupation, education, memory, lovedCity, lovedCountry, lovedLanguage, problemSolving, prediction, about, passions); err != nil {
		return err
	}

	// Get existing personal info
	existing, err := s.personalInfoRepo.FindByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check existing personal info: %w", err)
	}

	// Start with defaults or existing values
	var finalPassions map[string]bool
	if existing != nil && existing.Passions != nil {
		// Copy existing passions
		finalPassions = make(map[string]bool)
		for k, v := range existing.Passions {
			finalPassions[k] = v
		}
	} else {
		// Use defaults
		finalPassions = models.DefaultPassions()
	}

	// Merge new passions into existing/defaults
	if passions != nil {
		// Validate passion keys and update values
		validKeys := map[string]bool{
			"music":              true,
			"sport_health":       true,
			"art":                true,
			"language_culture":   true,
			"philosophy":         true,
			"animals_nature":     true,
			"aliens":             true,
			"food_cooking":       true,
			"travel_leature":     true,
			"manufacturing":      true,
			"science_technology": true,
			"space_time":         true,
			"history":            true,
			"politics_economy":   true,
		}
		for key, value := range passions {
			if !validKeys[key] {
				return ErrInvalidPassionKey
			}
			finalPassions[key] = value
		}
	}

	// Build PersonalInfo model
	personalInfo := &models.PersonalInfo{
		UserID:   userID,
		Passions: finalPassions,
	}

	// Handle nullable string fields - empty string becomes null (clears field)
	// Note: In proto3, we can't distinguish between "not provided" and "empty string"
	personalInfo.Occupation = sql.NullString{String: occupation, Valid: occupation != ""}
	personalInfo.Education = sql.NullString{String: education, Valid: education != ""}
	personalInfo.Memory = sql.NullString{String: memory, Valid: memory != ""}
	personalInfo.LovedCity = sql.NullString{String: lovedCity, Valid: lovedCity != ""}
	personalInfo.LovedCountry = sql.NullString{String: lovedCountry, Valid: lovedCountry != ""}
	personalInfo.LovedLanguage = sql.NullString{String: lovedLanguage, Valid: lovedLanguage != ""}
	personalInfo.ProblemSolving = sql.NullString{String: problemSolving, Valid: problemSolving != ""}
	personalInfo.Prediction = sql.NullString{String: prediction, Valid: prediction != ""}
	personalInfo.About = sql.NullString{String: about, Valid: about != ""}

	// Set ID if updating existing record
	if existing != nil {
		personalInfo.ID = existing.ID
	}

	// Upsert the record
	err = s.personalInfoRepo.Upsert(ctx, personalInfo)
	if err != nil {
		return fmt.Errorf("failed to upsert personal info: %w", err)
	}

	return nil
}

func (s *personalInfoService) validatePersonalInfoInput(occupation, education, memory, lovedCity, lovedCountry, lovedLanguage, problemSolving, prediction, about string, passions map[string]bool) error {
	// Validate string field lengths
	if len(occupation) > 255 {
		return ErrInvalidOccupation
	}
	if len(education) > 255 {
		return ErrInvalidEducation
	}
	if len(memory) > 20000 {
		return ErrInvalidMemory
	}
	if len(lovedCity) > 255 {
		return ErrInvalidLovedCity
	}
	if len(lovedCountry) > 255 {
		return ErrInvalidLovedCountry
	}
	if len(lovedLanguage) > 255 {
		return ErrInvalidLovedLanguage
	}
	if len(problemSolving) > 20000 {
		return ErrInvalidProblemSolving
	}
	if len(prediction) > 20000 {
		return ErrInvalidPrediction
	}
	if len(about) > 10000 {
		return ErrInvalidAbout
	}

	// Validate passion keys if provided
	if passions != nil {
		validKeys := map[string]bool{
			"music":              true,
			"sport_health":       true,
			"art":                true,
			"language_culture":   true,
			"philosophy":         true,
			"animals_nature":     true,
			"aliens":             true,
			"food_cooking":       true,
			"travel_leature":     true,
			"manufacturing":      true,
			"science_technology": true,
			"space_time":         true,
			"history":            true,
			"politics_economy":   true,
		}
		for key := range passions {
			if !validKeys[key] {
				return ErrInvalidPassionKey
			}
		}
	}

	return nil
}
