// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type ExptAggrResultRepoImpl struct {
	exptAggrResultDAO mysql.ExptAggrResultDAO
	idgenerator       idgen.IIDGenerator
}

func NewExptAggrResultRepo(exptAggrResultDAO mysql.ExptAggrResultDAO, idgenerator idgen.IIDGenerator) repo.IExptAggrResultRepo {
	return &ExptAggrResultRepoImpl{
		exptAggrResultDAO: exptAggrResultDAO,
		idgenerator:       idgenerator,
	}
}

func (r *ExptAggrResultRepoImpl) GetExptAggrResult(ctx context.Context, experimentID int64, fieldType int32, fieldKey string) (*entity.ExptAggrResult, error) {
	exptAggrResultPO, err := r.exptAggrResultDAO.GetExptAggrResult(ctx, experimentID, fieldType, fieldKey)
	if err != nil {
		return nil, err
	}

	exptAggrResultDO := convert.ExptAggrResultPOToDO(exptAggrResultPO)
	return exptAggrResultDO, nil
}

func (r *ExptAggrResultRepoImpl) GetExptAggrResultByExperimentID(ctx context.Context, experimentID int64) ([]*entity.ExptAggrResult, error) {
	exptAggrResultPOs, err := r.exptAggrResultDAO.GetExptAggrResultByExperimentID(ctx, experimentID)
	if err != nil {
		return nil, err
	}
	exptAggrResultDOs := make([]*entity.ExptAggrResult, 0)
	for _, exptAggrResult := range exptAggrResultPOs {
		exptAggrResultDO := convert.ExptAggrResultPOToDO(exptAggrResult)
		exptAggrResultDOs = append(exptAggrResultDOs, exptAggrResultDO)
	}
	return exptAggrResultDOs, nil
}

func (r *ExptAggrResultRepoImpl) BatchGetExptAggrResultByExperimentIDs(ctx context.Context, experimentIDs []int64) ([]*entity.ExptAggrResult, error) {
	exptAggrResultPOs, err := r.exptAggrResultDAO.BatchGetExptAggrResultByExperimentIDs(ctx, experimentIDs)
	if err != nil {
		return nil, err
	}

	exptAggrResultDOs := make([]*entity.ExptAggrResult, 0)
	for _, exptAggrResult := range exptAggrResultPOs {
		exptAggrResultDO := convert.ExptAggrResultPOToDO(exptAggrResult)
		exptAggrResultDOs = append(exptAggrResultDOs, exptAggrResultDO)
	}
	return exptAggrResultDOs, nil
}

func (r *ExptAggrResultRepoImpl) CreateExptAggrResult(ctx context.Context, exptAggrResult *entity.ExptAggrResult) error {
	id, err := r.idgenerator.GenID(ctx)
	if err != nil {
		return err
	}
	exptAggrResult.ID = id

	exptAggrResultPO := convert.ExptAggrResultDOToPO(exptAggrResult)

	return r.exptAggrResultDAO.CreateExptAggrResult(ctx, exptAggrResultPO)
}

func (r *ExptAggrResultRepoImpl) BatchCreateExptAggrResult(ctx context.Context, exptAggrResults []*entity.ExptAggrResult) error {
	ids, err := r.idgenerator.GenMultiIDs(ctx, len(exptAggrResults))
	if err != nil {
		return err
	}
	for index, exptAggrResult := range exptAggrResults {
		exptAggrResult.ID = ids[index]
	}

	exptAggrResultsPO := make([]*model.ExptAggrResult, 0)
	for _, exptAggrResult := range exptAggrResults {
		exptAggrResultPO := convert.ExptAggrResultDOToPO(exptAggrResult)
		exptAggrResultsPO = append(exptAggrResultsPO, exptAggrResultPO)
	}

	return r.exptAggrResultDAO.BatchCreateExptAggrResult(ctx, exptAggrResultsPO)
}

func (r *ExptAggrResultRepoImpl) UpdateExptAggrResultByVersion(ctx context.Context, exptAggrResult *entity.ExptAggrResult, taskVersion int64) error {
	exptAggrResultPO := convert.ExptAggrResultDOToPO(exptAggrResult)
	return r.exptAggrResultDAO.UpdateExptAggrResultByVersion(ctx, exptAggrResultPO, taskVersion)
}

// UpdateAndGetLatestVersion 返回更新后的version, clause.Returning 需要开启conf.WithReturning = true.
func (r *ExptAggrResultRepoImpl) UpdateAndGetLatestVersion(ctx context.Context, experimentID int64, fieldType int32, fieldKey string) (int64, error) {
	return r.exptAggrResultDAO.UpdateAndGetLatestVersion(ctx, experimentID, fieldType, fieldKey)
}

func (r *ExptAggrResultRepoImpl) DeleteExptAggrResult(ctx context.Context, exptAggrResult *entity.ExptAggrResult, opts ...db.Option) error {
	exptAggrResultPO := convert.ExptAggrResultDOToPO(exptAggrResult)
	return r.exptAggrResultDAO.DeleteExptAggrResult(ctx, exptAggrResultPO, opts...)
}
