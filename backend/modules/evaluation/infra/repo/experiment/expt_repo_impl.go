// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewExptRepo(exptDAO mysql.IExptDAO, exptEvaluatorRefDAO mysql.IExptEvaluatorRefDAO, idgen idgen.IIDGenerator) repo.IExperimentRepo {
	return &exptRepoImpl{exptDAO: exptDAO, exptEvaluatorRefDAO: exptEvaluatorRefDAO, idgen: idgen}
}

type exptRepoImpl struct {
	idgen               idgen.IIDGenerator
	exptDAO             mysql.IExptDAO
	exptEvaluatorRefDAO mysql.IExptEvaluatorRefDAO
}

func (e *exptRepoImpl) Create(ctx context.Context, expt *entity.Experiment, exptEvaluatorRefs []*entity.ExptEvaluatorRef) error {
	po, err := convert.NewExptConverter().DO2PO(expt)
	if err != nil {
		return err
	}

	if err := e.exptDAO.Create(ctx, po); err != nil {
		return err
	}

	ids, err := e.idgen.GenMultiIDs(ctx, len(exptEvaluatorRefs))
	if err != nil {
		return err
	}
	for i, ref := range exptEvaluatorRefs {
		ref.ID = ids[i]
	}

	exptEvaluatorRefPos := convert.NewExptEvaluatorRefConverter().DO2PO(exptEvaluatorRefs)
	err = e.exptEvaluatorRefDAO.Create(ctx, exptEvaluatorRefPos)
	if err != nil {
		return err
	}

	return nil
}

func (e *exptRepoImpl) Update(ctx context.Context, expt *entity.Experiment) error {
	po, err := convert.NewExptConverter().DO2PO(expt)
	if err != nil {
		return err
	}
	return e.exptDAO.Update(ctx, po)
}

func (e *exptRepoImpl) Delete(ctx context.Context, id, spaceID int64) error {
	return e.exptDAO.Delete(ctx, id)
}

func (e *exptRepoImpl) MDelete(ctx context.Context, ids []int64, spaceID int64) error {
	logs.CtxInfo(ctx, "batch delete experiments, id: %v", ids)
	return e.exptDAO.MDelete(ctx, ids)
}

func (e *exptRepoImpl) List(ctx context.Context, page, size int32, filter *entity.ExptListFilter, orders []*entity.OrderBy, spaceID int64) ([]*entity.Experiment, int64, error) {
	pos, cursor, err := e.exptDAO.List(ctx, page, size, filter, orders, spaceID)
	if err != nil {
		return nil, 0, err
	}

	exptIDs := slices.Transform(pos, func(e *model.Experiment, _ int) int64 {
		return e.ID
	})

	refs, err := e.exptEvaluatorRefDAO.MGetByExptID(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, 0, err
	}

	dos := make([]*entity.Experiment, 0, len(pos))
	for _, po := range pos {
		var eers []*model.ExptEvaluatorRef
		for _, ref := range refs {
			if ref.ExptID == po.ID {
				eers = append(eers, ref)
			}
		}
		do, err := convert.NewExptConverter().PO2DO(po, eers)
		if err != nil {
			return nil, 0, err
		}
		dos = append(dos, do)
	}

	return dos, cursor, err
}

func (e *exptRepoImpl) GetByID(ctx context.Context, id, spaceID int64) (*entity.Experiment, error) {
	got, err := e.MGetByID(ctx, []int64{id}, spaceID)
	if err != nil {
		return nil, err
	}
	if len(got) == 0 {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("experiment not found"))
	}
	return got[0], nil
}

func (e *exptRepoImpl) MGetByID(ctx context.Context, ids []int64, spaceID int64) ([]*entity.Experiment, error) {
	pos, err := e.exptDAO.MGetByID(ctx, ids)
	if err != nil {
		return nil, err
	}

	exptIDs := slices.Transform(pos, func(e *model.Experiment, _ int) int64 {
		return e.ID
	})

	refs, err := e.exptEvaluatorRefDAO.MGetByExptID(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}

	dos := make([]*entity.Experiment, 0, len(pos))
	for _, po := range pos {
		var eers []*model.ExptEvaluatorRef
		for _, ref := range refs {
			if ref.ExptID == po.ID {
				eers = append(eers, ref)
			}
		}
		do, err := convert.NewExptConverter().PO2DO(po, eers)
		if err != nil {
			return nil, err
		}
		dos = append(dos, do)
	}

	return dos, err
}

func (e *exptRepoImpl) MGetBasicByID(ctx context.Context, ids []int64) ([]*entity.Experiment, error) {
	pos, err := e.exptDAO.MGetByID(ctx, ids)
	if err != nil {
		return nil, err
	}

	res := make([]*entity.Experiment, 0, len(pos))
	for _, po := range pos {
		do, err := convert.NewExptConverter().PO2DO(po, nil)
		if err != nil {
			return nil, err
		}
		res = append(res, do)
	}

	return res, err
}

func (e *exptRepoImpl) GetByName(ctx context.Context, name string, spaceID int64) (*entity.Experiment, bool, error) {
	po, err := e.exptDAO.GetByName(ctx, name, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	do, err := convert.NewExptConverter().PO2DO(po, nil)
	if err != nil {
		return nil, false, err
	}

	return do, true, nil
}

func (e *exptRepoImpl) GetEvaluatorRefByExptIDs(ctx context.Context, exptIDs []int64, spaceID int64) ([]*entity.ExptEvaluatorRef, error) {
	pos, err := e.exptEvaluatorRefDAO.MGetByExptID(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}

	return convert.NewExptEvaluatorRefConverter().PO2DO(pos), nil
}
