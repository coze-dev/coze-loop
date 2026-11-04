package target

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

const fluxItemRunExtKey = "flux_workflow_run"

type fluxItemRunRepo struct{ db db.Provider }

func NewFluxItemRunRepo(provider db.Provider) repo.IFluxItemRunRepo {
	return &fluxItemRunRepo{db: provider}
}

func (r *fluxItemRunRepo) GetFluxItemRun(ctx context.Context, spaceID, invokeID int64) (*entity.FluxItemRun, error) {
	var row model.TargetRecord
	err := r.db.NewSession(ctx).Clauses(dbresolver.Write).Select("ext").Where("id = ? AND space_id = ? AND deleted_at IS NULL", invokeID, spaceID).Take(&row).Error
	if err != nil {
		return nil, err
	}
	return decodeFluxItemRun(row.Ext)
}

func (r *fluxItemRunRepo) CompareAndSwapFluxItemRun(ctx context.Context, spaceID, invokeID, expectedVersion int64, next *entity.FluxItemRun) (bool, *entity.FluxItemRun, error) {
	if next == nil || expectedVersion < 0 || next.SpaceID != spaceID || next.InvokeID != invokeID || next.State == "" {
		return false, nil, errors.New("invalid Flux Workflow run identity or state")
	}
	var current *entity.FluxItemRun
	updated := false
	err := r.db.NewSession(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.TargetRecord
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "ext").Where("id = ? AND space_id = ? AND deleted_at IS NULL", invokeID, spaceID).Take(&row).Error; err != nil {
			return err
		}
		ext, err := decodeFluxItemRunExt(row.Ext)
		if err != nil {
			return err
		}
		current, err = fluxItemRunFromExt(ext)
		if err != nil {
			return err
		}
		version := int64(0)
		if current != nil {
			version = current.Version
			if next.ExperimentID != current.ExperimentID {
				return errors.New("Flux Workflow run experiment changed")
			}
		}
		if version != expectedVersion {
			return nil
		}
		copy := *next
		copy.Version = version + 1
		encoded, err := json.Marshal(&copy)
		if err != nil {
			return err
		}
		ext[fluxItemRunExtKey] = string(encoded)
		extJSON, err := json.Marshal(ext)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.TargetRecord{}).Where("id = ? AND space_id = ?", invokeID, spaceID).UpdateColumn("ext", datatypes.JSON(extJSON)).Error; err != nil {
			return err
		}
		updated, current = true, &copy
		return nil
	})
	return updated, current, err
}

func decodeFluxItemRun(raw datatypes.JSON) (*entity.FluxItemRun, error) {
	ext, err := decodeFluxItemRunExt(raw)
	if err != nil {
		return nil, err
	}
	return fluxItemRunFromExt(ext)
}

func decodeFluxItemRunExt(raw datatypes.JSON) (map[string]string, error) {
	ext := make(map[string]string)
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &ext); err != nil {
			return nil, fmt.Errorf("decode target record ext: %w", err)
		}
	}
	return ext, nil
}

func fluxItemRunFromExt(ext map[string]string) (*entity.FluxItemRun, error) {
	raw := ext[fluxItemRunExtKey]
	if raw == "" {
		return nil, nil
	}
	var run entity.FluxItemRun
	if err := json.Unmarshal([]byte(raw), &run); err != nil {
		return nil, fmt.Errorf("decode Flux Workflow run: %w", err)
	}
	return &run, nil
}
