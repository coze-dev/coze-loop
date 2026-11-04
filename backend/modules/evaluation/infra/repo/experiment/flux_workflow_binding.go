// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

type fluxWorkflowBindingRepo struct{ db db.Provider }

func NewFluxWorkflowBindingRepo(provider db.Provider) repo.IFluxWorkflowBindingRepo {
	return &fluxWorkflowBindingRepo{db: provider}
}

func (r *fluxWorkflowBindingRepo) GetFluxWorkflowBinding(ctx context.Context, spaceID, exptID int64) (*entity.FluxWorkflowBinding, error) {
	var row model.Experiment
	if err := r.db.NewSession(ctx).Clauses(dbresolver.Write).Select("eval_conf").Where("id = ? AND space_id = ?", exptID, spaceID).First(&row).Error; err != nil {
		return nil, err
	}
	conf, err := decodeFluxExperimentConf(row.EvalConf)
	if err != nil {
		return nil, err
	}
	return conf.FluxWorkflowBinding, nil
}

func (r *fluxWorkflowBindingRepo) ReserveFluxWorkflowBinding(ctx context.Context, spaceID, exptID int64, pending *entity.FluxWorkflowBinding) (*entity.FluxWorkflowBinding, error) {
	if pending == nil || pending.BindingID == "" || pending.State != "preparing" {
		return nil, errors.New("Flux Workflow pending binding must have identity and preparing state")
	}
	var result *entity.FluxWorkflowBinding
	err := r.db.NewSession(ctx).Transaction(func(tx *gorm.DB) error {
		row, conf, err := lockFluxExperiment(ctx, tx, spaceID, exptID)
		if err != nil {
			return err
		}
		if conf.FluxWorkflowBinding != nil {
			result = conf.FluxWorkflowBinding
			return nil
		}
		copy := *pending
		copy.SpaceID, copy.ExperimentID = spaceID, exptID
		conf.FluxWorkflowBinding = &copy
		if err := saveFluxExperimentConf(tx, row.ID, conf); err != nil {
			return err
		}
		result = &copy
		return nil
	})
	return result, err
}

func (r *fluxWorkflowBindingRepo) PutReadyFluxWorkflowBinding(ctx context.Context, spaceID, exptID int64, ready *entity.FluxWorkflowBinding) error {
	if ready == nil || ready.BindingID == "" || ready.State != "ready" {
		return errors.New("Flux Workflow ready binding must have identity and ready state")
	}
	return r.db.NewSession(ctx).Transaction(func(tx *gorm.DB) error {
		row, conf, err := lockFluxExperiment(ctx, tx, spaceID, exptID)
		if err != nil {
			return err
		}
		before := conf.FluxWorkflowBinding
		if before == nil || before.BindingID != ready.BindingID {
			return errors.New("Flux Workflow binding reservation changed")
		}
		if before.State == "ready" {
			if before.CopyDigest == ready.CopyDigest && before.EvalWorkflowID == ready.EvalWorkflowID && before.CopyTemplateID == ready.CopyTemplateID && before.CopyReleaseID == ready.CopyReleaseID {
				return nil
			}
			return errors.New("Flux Workflow binding already ready with another copy")
		}
		if before.State != "preparing" {
			return fmt.Errorf("Flux Workflow binding state %q cannot become ready", before.State)
		}
		copy := *ready
		copy.SpaceID, copy.ExperimentID = spaceID, exptID
		conf.FluxWorkflowBinding = &copy
		return saveFluxExperimentConf(tx, row.ID, conf)
	})
}

func lockFluxExperiment(ctx context.Context, tx *gorm.DB, spaceID, exptID int64) (*model.Experiment, *entity.EvaluationConfiguration, error) {
	var row model.Experiment
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "eval_conf").Where("id = ? AND space_id = ?", exptID, spaceID).First(&row).Error; err != nil {
		return nil, nil, err
	}
	conf, err := decodeFluxExperimentConf(row.EvalConf)
	return &row, conf, err
}

func decodeFluxExperimentConf(raw *[]byte) (*entity.EvaluationConfiguration, error) {
	conf := &entity.EvaluationConfiguration{}
	if raw != nil && len(*raw) != 0 {
		if err := json.Unmarshal(*raw, conf); err != nil {
			return nil, fmt.Errorf("decode experiment eval_conf: %w", err)
		}
	}
	return conf, nil
}

func saveFluxExperimentConf(tx *gorm.DB, exptID int64, conf *entity.EvaluationConfiguration) error {
	encoded, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	return tx.Model(&model.Experiment{}).Where("id = ?", exptID).UpdateColumn("eval_conf", encoded).Error
}
