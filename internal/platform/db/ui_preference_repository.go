package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrUIPreferenceConflict = errors.New("UI preference conflict")

type UIPreference struct {
	ExperienceMode string
	Version        int64
	UpdatedAt      *time.Time
}

type UIPreferenceRepository struct{ database *sql.DB }

func NewUIPreferenceRepository(database *sql.DB) (*UIPreferenceRepository, error) {
	if database == nil {
		return nil, errors.New("UI preference database is required")
	}
	return &UIPreferenceRepository{database: database}, nil
}

func (r *UIPreferenceRepository) Get(ctx context.Context, tenantID, subjectID string) (UIPreference, error) {
	var preference UIPreference
	var updatedAt time.Time
	err := r.database.QueryRowContext(ctx, `SELECT experience_mode,version,updated_at FROM operator_ui_preferences WHERE tenant_id=$1 AND subject_id=$2`, tenantID, subjectID).Scan(&preference.ExperienceMode, &preference.Version, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UIPreference{ExperienceMode: "simple"}, nil
	}
	if err != nil {
		return UIPreference{}, fmt.Errorf("read UI preference: %w", err)
	}
	updatedAt = updatedAt.UTC()
	preference.UpdatedAt = &updatedAt
	return preference, nil
}

func (r *UIPreferenceRepository) Update(ctx context.Context, tenantID, subjectID, mode string, expectedVersion int64) (UIPreference, error) {
	var preference UIPreference
	var updatedAt time.Time
	var err error
	if expectedVersion == 0 {
		err = r.database.QueryRowContext(ctx, `INSERT INTO operator_ui_preferences(tenant_id,subject_id,experience_mode,version,updated_at) VALUES($1,$2,$3,1,now()) ON CONFLICT (tenant_id,subject_id) DO NOTHING RETURNING experience_mode,version,updated_at`, tenantID, subjectID, mode).Scan(&preference.ExperienceMode, &preference.Version, &updatedAt)
	} else {
		err = r.database.QueryRowContext(ctx, `UPDATE operator_ui_preferences SET experience_mode=$3,version=version+1,updated_at=now() WHERE tenant_id=$1 AND subject_id=$2 AND version=$4 RETURNING experience_mode,version,updated_at`, tenantID, subjectID, mode, expectedVersion).Scan(&preference.ExperienceMode, &preference.Version, &updatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return UIPreference{}, ErrUIPreferenceConflict
	}
	if err != nil {
		return UIPreference{}, fmt.Errorf("update UI preference: %w", err)
	}
	updatedAt = updatedAt.UTC()
	preference.UpdatedAt = &updatedAt
	return preference, nil
}
