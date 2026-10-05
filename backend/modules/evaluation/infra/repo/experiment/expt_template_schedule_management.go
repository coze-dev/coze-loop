// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	hook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	mysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type exptTemplateScheduleStore struct{ *hookConfigRepo }

func NewExptTemplateScheduleStore(p db.Provider, codec hook.StorageCodec) repo.IExptTemplateScheduleStore {
	return &exptTemplateScheduleStore{&hookConfigRepo{provider: p, codec: codec}}
}
func templateScheduleRevision(po *model.ExptTemplate, refs []*model.ExptTemplateEvaluatorRef) string {
	raw, _ := json.Marshal([]any{po, po.LifecycleHookConf, po.ScheduleRunBinding, refs})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func templateScheduleRows(tx *gorm.DB, key entity.ExptTemplateScheduleBindingKey) (*model.ExptTemplate, []*model.ExptTemplateEvaluatorRef, error) {
	var po model.ExptTemplate
	if err := tx.Where("id=? AND space_id=?", key.TemplateID, key.SpaceID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&po).Error; err != nil {
		return nil, nil, err
	}
	var refs []*model.ExptTemplateEvaluatorRef
	err := tx.Where("space_id=? AND expt_template_id=?", key.SpaceID, key.TemplateID).Order("id").Find(&refs).Error
	return &po, refs, err
}
func templateScheduleOwner(key entity.ExptTemplateScheduleBindingKey) hook.ConfigOwner {
	return hook.ConfigOwner{WorkspaceID: key.SpaceID, ObjectID: key.TemplateID, ExecutionScope: key.ExecutionScope, Kind: hook.ConfigOwnerTemplate}
}
func (r *exptTemplateScheduleStore) Read(ctx context.Context, key entity.ExptTemplateScheduleBindingKey) (*repo.ExptTemplateScheduleState, error) {
	if r == nil || r.provider == nil || r.codec == nil || key.Validate() != nil {
		return nil, entity.ErrHookConfigStorage
	}
	var po *model.ExptTemplate
	var refs []*model.ExptTemplateEvaluatorRef
	var binding *entity.ExptTemplateScheduleBinding
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		var err error
		po, refs, err = templateScheduleRows(tx, key)
		if err != nil {
			return err
		}
		binding, _, err = readScheduleBinding(tx, key, false)
		return err
	}, db.WithMaster())
	if err != nil {
		return nil, err
	}
	template, err := convert.NewExptTemplateConverter().PO2DO(po, refs)
	if err != nil {
		return nil, err
	}
	var raw []byte
	if po.LifecycleHookConf != nil {
		raw = *po.LifecycleHookConf
	}
	conf, err := r.codec.DecodeConfig(ctx, templateScheduleOwner(key), raw)
	if err != nil {
		return nil, entity.ErrHookConfigStorage
	}
	return &repo.ExptTemplateScheduleState{Template: template, Config: &entity.HookConfigRecord{Config: conf, Revision: hookConfigRevision(raw)}, Binding: binding, Revision: templateScheduleRevision(po, refs)}, nil
}
func (r *exptTemplateScheduleStore) Write(ctx context.Context, in repo.ExptTemplateScheduleWrite) error {
	if r == nil || r.provider == nil || r.codec == nil || in.Key.Validate() != nil || in.Binding != nil && in.Disable {
		return entity.ErrHookConfigStorage
	}
	if in.Template != nil && (in.Template.GetID() != in.Key.TemplateID || in.Template.GetSpaceID() != in.Key.SpaceID) {
		return entity.ErrHookStoreConflict
	}
	if in.Create && (in.Template == nil || in.ExpectedRevision != "" || in.Disable) {
		return entity.ErrHookStoreConflict
	}
	var current *repo.ExptTemplateScheduleState
	var err error
	if !in.Create {
		current, err = r.Read(ctx, in.Key)
		if err != nil {
			return err
		}
		if current.Revision != in.ExpectedRevision {
			return entity.ErrHookStoreConflict
		}
	}
	explicit := in.Hook.Config != nil && (in.Hook.Config.Before != nil || in.Hook.Config.After != nil)
	var encoded []byte
	if explicit {
		var old *entity.LifecycleHookConf
		if current != nil {
			if current.Config.Revision != in.Hook.ExpectedRevision {
				return entity.ErrHookStoreConflict
			}
			old = current.Config.Config
		}
		conf, err := entity.ResolveLifecycleHookConf(old, in.Hook.Config)
		if err != nil {
			return err
		}
		encoded, err = r.codec.EncodeConfig(ctx, in.Hook.KeyID, templateScheduleOwner(in.Key), conf)
		if err != nil {
			return entity.ErrHookConfigStorage
		}
	}
	ids := make([]int64, len(in.Refs))
	seen := map[int64]bool{}
	for i, ref := range in.Refs {
		if ref == nil || ref.ID <= 0 || seen[ref.ID] || ref.SpaceID != in.Key.SpaceID || ref.ExptTemplateID != in.Key.TemplateID {
			return entity.ErrHookConfigStorage
		}
		seen[ref.ID] = true
		ids[i] = ref.ID
	}
	if in.Binding != nil {
		if in.Binding.Validate() != nil || !scheduleBindingMatches(in.Binding, in.Key) || !in.Binding.Enabled || in.Binding.JobID != "" {
			return entity.ErrExptTemplateScheduleBindingInvalid
		}
	}
	for field := range in.Fields {
		switch field {
		case "name", "description", "expt_type", "visibility", "cron_activate", "expt_info", "updated_at", "updated_by":
		default:
			return entity.ErrHookStoreConflict
		}
	}
	return r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		bound := hookManagementTxProvider{r.provider, tx}
		if in.Create {
			po, err := convert.NewExptTemplateConverter().DO2PO(in.Template)
			if err != nil {
				return err
			}
			po.ScheduleRunBinding = nil
			if explicit {
				po.LifecycleHookConf = &encoded
			} else {
				po.LifecycleHookConf = nil
			}
			if err := tx.Create(po).Error; err != nil {
				return err
			}
			if len(in.Refs) > 0 {
				if err := tx.Create(convert.NewExptTemplateEvaluatorRefConverter().DO2PO(in.Refs)).Error; err != nil {
					return err
				}
			}
		} else {
			po, refs, err := templateScheduleRows(tx, in.Key)
			if err != nil {
				return err
			}
			if templateScheduleRevision(po, refs) != in.ExpectedRevision {
				return entity.ErrHookStoreConflict
			}
			if in.Template != nil {
				legacy := NewExptTemplateRepo(mysql.NewExptTemplateDAO(bound), mysql.NewExptTemplateEvaluatorRefDAO(bound), &hookManagementReservedIDs{ids: ids})
				if err := legacy.UpdateWithRefs(ctx, in.Template, in.Refs); err != nil {
					return err
				}
				cron := in.Template.ExptInfo != nil && in.Template.ExptInfo.CronActivate
				if err := tx.Model(&model.ExptTemplate{}).Where("id=? AND space_id=?", in.Key.TemplateID, in.Key.SpaceID).UpdateColumn("cron_activate", cron).Error; err != nil {
					return err
				}
			} else if len(in.Fields) > 0 {
				if err := tx.Model(&model.ExptTemplate{}).Where("id=? AND space_id=?", in.Key.TemplateID, in.Key.SpaceID).UpdateColumns(in.Fields).Error; err != nil {
					return err
				}
			}
			if explicit {
				if err := tx.Model(&model.ExptTemplate{}).Where("id=? AND space_id=?", in.Key.TemplateID, in.Key.SpaceID).UpdateColumn("lifecycle_hook_conf", encoded).Error; err != nil {
					return err
				}
			}
		}
		bindings := NewExptTemplateScheduleBindingRepo(r.provider)
		version := int64(0)
		if current != nil && current.Binding != nil {
			version = current.Binding.Version
		}
		if in.Binding != nil {
			_, err = bindings.SaveCAS(ctx, in.Key, version, in.Binding, db.WithTransaction(tx))
		} else if in.Disable && current != nil && current.Binding != nil {
			_, err = bindings.DisableCAS(ctx, in.Key, current.Binding.BindingID, version, db.WithTransaction(tx))
		}
		if errors.Is(err, entity.ErrExptTemplateScheduleBindingConflict) {
			return entity.ErrHookStoreConflict
		}
		return err
	}, db.WithMaster())
}
