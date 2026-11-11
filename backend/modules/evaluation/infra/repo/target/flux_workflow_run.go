package target

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

func (r *fluxItemRunRepo) ScanPendingFluxCleanup(ctx context.Context, cursor int64, limit int, since, before time.Time) ([]*entity.FluxItemRun, int64, error) {
	if cursor < 0 || limit < 1 || limit > 500 || !before.After(since) {
		return nil, 0, errors.New("invalid Flux Workflow cleanup scan window or page")
	}
	lower, err := fluxRecordIDFloor(since)
	if err != nil {
		return nil, 0, err
	}
	upper, err := fluxRecordIDFloor(before)
	if err != nil || lower >= upper {
		return nil, 0, errors.New("invalid Flux Workflow cleanup scan ID range")
	}
	if cursor < lower {
		cursor = lower - 1
	}
	if cursor >= upper {
		return nil, 0, nil
	}
	var rows []model.TargetRecord
	err = r.db.NewSession(ctx).Clauses(dbresolver.Write).
		Select("id,space_id,status,deleted_at,ext").
		Where("id > ? AND id < ?", cursor, upper).
		Order("id ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	var pending []*entity.FluxItemRun
	for _, row := range rows {
		if row.DeletedAt.Valid || (row.Status != int32(entity.EvalTargetRunStatusSuccess) && row.Status != int32(entity.EvalTargetRunStatusFail)) {
			continue
		}
		run, err := decodeFluxItemRun(row.Ext)
		if err != nil {
			return nil, 0, fmt.Errorf("decode Workflow cleanup candidate id=%d: %w", row.ID, err)
		}
		if run == nil || run.CleanupState != "pending" || !run.ReportCompleted || run.DeliveryID == "" ||
			run.InvokeID != row.ID || run.SpaceID != row.SpaceID || run.ExperimentID <= 0 || run.BindingID == "" {
			continue
		}
		run.RecordStatus = row.Status
		pending = append(pending, run)
	}
	if len(rows) == limit {
		return pending, rows[len(rows)-1].ID, nil
	}
	return pending, 0, nil
}

// The record ID generator stores seconds and milliseconds in the high bits.
func fluxRecordIDFloor(t time.Time) (int64, error) {
	seconds := t.Unix()
	if seconds < 0 || seconds >= 1<<31 {
		return 0, errors.New("Flux Workflow cleanup scan time is outside the record ID range")
	}
	return seconds<<32 | (t.UnixMilli()%1000)<<22, nil
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
			if current.ExperimentSpaceID != 0 && next.ExperimentSpaceID != current.ExperimentSpaceID {
				return errors.New("Flux Workflow run experiment space changed")
			}
		}
		if version != expectedVersion {
			return nil
		}
		if current == nil && next.State != "reserved" {
			return errors.New("Flux Workflow run must be reserved before progressing")
		}
		if current != nil {
			if current.BindingID != "" && next.BindingID != current.BindingID {
				return errors.New("Flux Workflow run binding changed")
			}
			if current.QueryDigest != "" && next.QueryDigest != current.QueryDigest {
				return errors.New("Flux Workflow query changed")
			}
			if current.IdempotencyKey != "" && next.IdempotencyKey != current.IdempotencyKey {
				return errors.New("Flux Workflow idempotency key changed")
			}
			if current.DeliveryID != "" && next.DeliveryID != current.DeliveryID {
				return errors.New("Flux Workflow delivery changed")
			}
			if (current.State == "delivery_created" && next.State == "reserved") || (current.State == "finalized" && next.State != "finalized") {
				return errors.New("Flux Workflow run state regressed")
			}
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
