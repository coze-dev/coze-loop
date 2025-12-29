// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

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
		refsMap[ref.TemplateID] = append(refsMap[ref.TemplateID], ref)
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

	// 删除旧的评估器引用
	if err := e.templateEvaluatorRefDAO.DeleteByTemplateID(ctx, template.ID); err != nil {
		return err
	}

	// 创建新的评估器引用
	if len(refs) > 0 {
		ids, err := e.idgen.GenMultiIDs(ctx, len(refs))
		if err != nil {
			return err
		}
		for i, ref := range refs {
			ref.ID = ids[i]
		}

		refPos := convert.NewExptTemplateEvaluatorRefConverter().DO2PO(refs)
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
		refsMap[ref.TemplateID] = append(refsMap[ref.TemplateID], ref)
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
