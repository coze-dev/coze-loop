// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

func NewExptTemplateRepo(
	templateDAO mysql.IExptTemplateDAO,
	templateEvaluatorRefDAO mysql.IExptTemplateEvaluatorRefDAO,
	idgen idgen.IIDGenerator,
) repo.IExptTemplateRepo {
	return &exptTemplateRepoImpl{
		templateDAO:              templateDAO,
		templateEvaluatorRefDAO: templateEvaluatorRefDAO,
		idgen:                    idgen,
	}
}

type exptTemplateRepoImpl struct {
	idgen                    idgen.IIDGenerator
	templateDAO              mysql.IExptTemplateDAO
	templateEvaluatorRefDAO   mysql.IExptTemplateEvaluatorRefDAO
}

func (e *exptTemplateRepoImpl) Create(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error {
	po, err := convert.NewExptTemplateConverter().DO2PO(template)
	if err != nil {
		return err
	}

	if err := e.templateDAO.Create(ctx, po); err != nil {
		return err
	}

	// 生成评估器引用的ID
	if len(refs) > 0 {
		ids, err := e.idgen.GenMultiIDs(ctx, len(refs))
		if err != nil {
			return err
		}
		for i, ref := range refs {
			ref.ID = ids[i]
		}

		refPos := convert.NewExptTemplateEvaluatorRefConverter().DO2PO(refs)
		err = e.templateEvaluatorRefDAO.Create(ctx, refPos)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *exptTemplateRepoImpl) GetByID(ctx context.Context, id, spaceID int64) (*entity.ExptTemplate, error) {
	po, err := e.templateDAO.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if po == nil {
		return nil, nil
	}
	if po.SpaceID != spaceID {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("template not found or access denied"))
	}

	refs, err := e.templateEvaluatorRefDAO.GetByTemplateIDs(ctx, []int64{id})
	if err != nil {
		return nil, err
	}

	return convert.NewExptTemplateConverter().PO2DO(po, refs)
}

func (e *exptTemplateRepoImpl) GetByName(ctx context.Context, name string, spaceID int64) (*entity.ExptTemplate, bool, error) {
	po, err := e.templateDAO.GetByName(ctx, name, spaceID)
	if err != nil {
		return nil, false, err
	}
	if po == nil {
		return nil, false, nil
	}

	refs, err := e.templateEvaluatorRefDAO.GetByTemplateIDs(ctx, []int64{po.ID})
	if err != nil {
		return nil, false, err
	}

	do, err := convert.NewExptTemplateConverter().PO2DO(po, refs)
	if err != nil {
		return nil, false, err
	}

	return do, true, nil
}

func (e *exptTemplateRepoImpl) MGetByID(ctx context.Context, ids []int64, spaceID int64) ([]*entity.ExptTemplate, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	pos, err := e.templateDAO.MGetByID(ctx, ids)
	if err != nil {
		return nil, err
	}

	// 过滤spaceID
	filteredPos := make([]*model.ExptTemplate, 0, len(pos))
	templateIDs := make([]int64, 0, len(pos))
	for _, po := range pos {
		if po.SpaceID == spaceID {
			filteredPos = append(filteredPos, po)
			templateIDs = append(templateIDs, po.ID)
		}
	}

	if len(templateIDs) == 0 {
		return nil, nil
	}

	refs, err := e.templateEvaluatorRefDAO.GetByTemplateIDs(ctx, templateIDs)
	if err != nil {
		return nil, err
	}

	// 构建refs映射
	refsMap := make(map[int64][]*model.ExptTemplateEvaluatorRef)
	for _, ref := range refs {
		refsMap[ref.ExptTemplateID] = append(refsMap[ref.ExptTemplateID], ref)
	}

	results := make([]*entity.ExptTemplate, 0, len(filteredPos))
	for _, po := range filteredPos {
		do, err := convert.NewExptTemplateConverter().PO2DO(po, refsMap[po.ID])
		if err != nil {
			return nil, err
		}
		results = append(results, do)
	}

	return results, nil
}

func (e *exptTemplateRepoImpl) Update(ctx context.Context, template *entity.ExptTemplate) error {
	po, err := convert.NewExptTemplateConverter().DO2PO(template)
	if err != nil {
		return err
	}

	return e.templateDAO.Update(ctx, po)
}

func (e *exptTemplateRepoImpl) UpdateFields(ctx context.Context, templateID int64, ufields map[string]any) error {
	return e.templateDAO.UpdateFields(ctx, templateID, ufields)
}

func (e *exptTemplateRepoImpl) UpdateWithRefs(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error {
	// 更新模板基本信息
	po, err := convert.NewExptTemplateConverter().DO2PO(template)
	if err != nil {
		return err
	}

	if err := e.templateDAO.Update(ctx, po); err != nil {
		return err
	}

	// 获取现有的评估器引用（包括已软删除的）
	existingRefs, err := e.templateEvaluatorRefDAO.GetByTemplateIDsIncludeDeleted(ctx, []int64{template.GetID()})
	if err != nil {
		return err
	}

	// 构建现有引用的映射：key = evaluator_id#evaluator_version_id, value = ref
	existingMap := make(map[string]*model.ExptTemplateEvaluatorRef)
	for _, ref := range existingRefs {
		key := fmt.Sprintf("%d#%d", ref.EvaluatorID, ref.EvaluatorVersionID)
		existingMap[key] = ref
	}

	// 构建新引用的映射：key = evaluator_id#evaluator_version_id
	newMap := make(map[string]bool)
	for _, ref := range refs {
		key := fmt.Sprintf("%d#%d", ref.EvaluatorID, ref.EvaluatorVersionID)
		newMap[key] = true
	}

	// 计算差集
	var toCreate []*entity.ExptTemplateEvaluatorRef
	var toRestore []int64
	var toSoftDelete []int64

	for _, ref := range refs {
		key := fmt.Sprintf("%d#%d", ref.EvaluatorID, ref.EvaluatorVersionID)
		existingRef, exists := existingMap[key]
		if !exists {
			// 需要新增
			toCreate = append(toCreate, ref)
		} else if existingRef.DeletedAt.Valid {
			// 需要恢复（之前被软删除，现在又需要了）
			toRestore = append(toRestore, existingRef.ID)
		}
		// 如果已存在且未删除，则保持不变
	}

	// 找出需要软删除的（现有的但不在新的列表中）
	for key, existingRef := range existingMap {
		if !newMap[key] && !existingRef.DeletedAt.Valid {
			// 需要软删除（存在但不在新列表中）
			toSoftDelete = append(toSoftDelete, existingRef.ID)
		}
	}

	// 执行恢复
	if len(toRestore) > 0 {
		if err := e.templateEvaluatorRefDAO.RestoreByIDs(ctx, toRestore); err != nil {
			return err
		}
	}

	// 执行软删除
	if len(toSoftDelete) > 0 {
		if err := e.templateEvaluatorRefDAO.SoftDeleteByIDs(ctx, toSoftDelete); err != nil {
			return err
		}
	}

	// 创建新的评估器引用
	if len(toCreate) > 0 {
		ids, err := e.idgen.GenMultiIDs(ctx, len(toCreate))
		if err != nil {
			return err
		}
		for i, ref := range toCreate {
			ref.ID = ids[i]
		}

		refPos := convert.NewExptTemplateEvaluatorRefConverter().DO2PO(toCreate)
		if err := e.templateEvaluatorRefDAO.Create(ctx, refPos); err != nil {
			return err
		}
	}

	return nil
}

func (e *exptTemplateRepoImpl) Delete(ctx context.Context, id, spaceID int64) error {
	// 验证spaceID
	po, err := e.templateDAO.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if po == nil {
		return errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	if po.SpaceID != spaceID {
		return errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("template not found or access denied"))
	}

	return e.templateDAO.Delete(ctx, id)
}

func (e *exptTemplateRepoImpl) List(ctx context.Context, page, size int32, filter *entity.ExptTemplateListFilter, orders []*entity.OrderBy, spaceID int64) ([]*entity.ExptTemplate, int64, error) {
	pos, count, err := e.templateDAO.List(ctx, page, size, filter, orders, spaceID)
	if err != nil {
		return nil, 0, err
	}

	if len(pos) == 0 {
		return nil, count, nil
	}

	templateIDs := slices.Transform(pos, func(t *model.ExptTemplate, _ int) int64 {
		return t.ID
	})

	refs, err := e.templateEvaluatorRefDAO.GetByTemplateIDs(ctx, templateIDs)
	if err != nil {
		return nil, 0, err
	}

	// 构建refs映射
	refsMap := make(map[int64][]*model.ExptTemplateEvaluatorRef)
	for _, ref := range refs {
		refsMap[ref.ExptTemplateID] = append(refsMap[ref.ExptTemplateID], ref)
	}

	results := make([]*entity.ExptTemplate, 0, len(pos))
	for _, po := range pos {
		do, err := convert.NewExptTemplateConverter().PO2DO(po, refsMap[po.ID])
		if err != nil {
			return nil, 0, err
		}
		results = append(results, do)
	}

	return results, count, nil
}
