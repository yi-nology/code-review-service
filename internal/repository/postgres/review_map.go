package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"gorm.io/datatypes"
	gormLib "gorm.io/gorm"
)

type ReviewMapRepo struct {
	db *gormLib.DB
}

func NewReviewMapRepo(db *gormLib.DB) *ReviewMapRepo {
	return &ReviewMapRepo{db: db}
}

// Create creates a new review map
func (r *ReviewMapRepo) Create(ctx context.Context, m *models.ReviewMap) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// GetByMapID returns a review map by its map ID
func (r *ReviewMapRepo) GetByMapID(ctx context.Context, mapID string) (*models.ReviewMap, error) {
	var m models.ReviewMap
	err := r.db.WithContext(ctx).Where("map_id = ?", mapID).First(&m).Error
	if err != nil {
		if err == gormLib.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// GetByReviewID returns all review maps for a review
func (r *ReviewMapRepo) GetByReviewID(ctx context.Context, reviewID string) ([]models.ReviewMap, error) {
	var maps []models.ReviewMap
	err := r.db.WithContext(ctx).
		Where("review_id = ?", reviewID).
		Order("created_at DESC").
		Find(&maps).Error
	return maps, err
}

// GetBySessionID returns the review map for a session
func (r *ReviewMapRepo) GetBySessionID(ctx context.Context, sessionID string) (*models.ReviewMap, error) {
	var m models.ReviewMap
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&m).Error
	if err != nil {
		if err == gormLib.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// Update updates a review map
func (r *ReviewMapRepo) Update(ctx context.Context, m *models.ReviewMap) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// Delete deletes a review map
func (r *ReviewMapRepo) Delete(ctx context.Context, mapID string) error {
	return r.db.WithContext(ctx).Where("map_id = ?", mapID).Delete(&models.ReviewMap{}).Error
}

// DeleteByReviewID deletes all review maps for a review
func (r *ReviewMapRepo) DeleteByReviewID(ctx context.Context, reviewID string) error {
	return r.db.WithContext(ctx).Where("review_id = ?", reviewID).Delete(&models.ReviewMap{}).Error
}

// UpdateFileReviewed updates the reviewed status of a specific file in a review map
func (r *ReviewMapRepo) UpdateFileReviewed(ctx context.Context, mapID, filePath string, reviewed bool) error {
	var m models.ReviewMap
	if err := r.db.WithContext(ctx).Where("map_id = ?", mapID).First(&m).Error; err != nil {
		return err
	}

	var sections []models.SectionData
	if err := json.Unmarshal(m.Sections, &sections); err != nil {
		return err
	}

	updated := false
	for i, section := range sections {
		for j, file := range section.Files {
			if file.Path == filePath {
				sections[i].Files[j].Reviewed = reviewed
				updated = true
				break
			}
		}
		if updated {
			// Update section progress
			totalFiles := len(sections[i].Files)
			reviewedCount := 0
			for _, f := range sections[i].Files {
				if f.Reviewed {
					reviewedCount++
				}
			}
			if totalFiles > 0 {
				sections[i].Progress = float64(reviewedCount) / float64(totalFiles) * 100
			}
			break
		}
	}

	if !updated {
		return fmt.Errorf("file not found: %s", filePath)
	}

	sectionsJSON, err := json.Marshal(sections)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Model(&m).Update("sections", datatypes.JSON(sectionsJSON)).Error
}

// UpdateSectionFilesReviewed updates the reviewed status of all files in a section
func (r *ReviewMapRepo) UpdateSectionFilesReviewed(ctx context.Context, mapID, sectionID string, reviewed bool) error {
	var m models.ReviewMap
	if err := r.db.WithContext(ctx).Where("map_id = ?", mapID).First(&m).Error; err != nil {
		return err
	}

	var sections []models.SectionData
	if err := json.Unmarshal(m.Sections, &sections); err != nil {
		return err
	}

	updated := false
	for i, section := range sections {
		if section.ID == sectionID {
			for j := range section.Files {
				sections[i].Files[j].Reviewed = reviewed
			}
			if reviewed {
				sections[i].Progress = 100
			} else {
				sections[i].Progress = 0
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("section not found: %s", sectionID)
	}

	sectionsJSON, err := json.Marshal(sections)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Model(&m).Update("sections", datatypes.JSON(sectionsJSON)).Error
}
