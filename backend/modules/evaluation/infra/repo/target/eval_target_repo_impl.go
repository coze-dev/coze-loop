// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"context"
	"sync"

	"github.com/coze-dev/coze-loop/backend/pkg/logs"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

var (
	evalTargetRepoOnce      = sync.Once{}
	singletonEvalTargetRepo repo.IEvalTargetRepo
)

type EvalTargetRepoImpl struct {
	evalTargetDao        mysql.EvalTargetDAO
	evalTargetVersionDao mysql.EvalTargetVersionDAO
	evalTargetRecordDao  mysql.EvalTargetRecordDAO

	idgen      idgen.IIDGenerator
	dbProvider db.Provider
	lwt        platestwrite.ILatestWriteTracker
}

func NewEvalTargetRepo(idgen idgen.IIDGenerator, provider db.Provider, evalTargetDao mysql.EvalTargetDAO, evalTargetVersionDao mysql.EvalTargetVersionDAO, evalTargetRecordDao mysql.EvalTargetRecordDAO, lwt platestwrite.ILatestWriteTracker) repo.IEvalTargetRepo {
	evalTargetRepoOnce.Do(func() {
		singletonEvalTargetRepo = &EvalTargetRepoImpl{
			evalTargetDao:        evalTargetDao,
			evalTargetVersionDao: evalTargetVersionDao,
			evalTargetRecordDao:  evalTargetRecordDao,
			idgen:                idgen,
			dbProvider:           provider,
			lwt:                  lwt,
		}
	})
	return singletonEvalTargetRepo
}

func (e *EvalTargetRepoImpl) CreateEvalTarget(ctx context.Context, do *entity.EvalTarget) (id, versionID int64, err error) {
	if do == nil {
		return 0, 0, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	if do.EvalTargetVersion == nil {
		return 0, 0, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	// 生成主键ID
	genIDs, err := e.idgen.GenMultiIDs(ctx, 2)
	if err != nil {
		return 0, 0, err
	}
	id = genIDs[0]
	versionID = genIDs[1]
	err = e.dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opt := db.WithTransaction(tx)
		// 检查是否创建过这个对象
		target, errGet := e.evalTargetDao.GetEvalTargetBySourceID(ctx, do.SpaceID, do.SourceTargetID, int32(do.EvalTargetType), opt, db.WithSelectForUpdate())
		if errGet != nil {
			return errGet
		}
		// 如果没有创建过，则创建
		if target == nil {
			do.ID = id
			errCreate := e.evalTargetDao.CreateEvalTarget(ctx, convertor.EvalTargetDO2PO(do), opt)
			if errCreate != nil {
				return errCreate
			}
		} else {
			id = target.ID
		}
		// 检查这个对象的版本是否创建过
		version, errGetVersion := e.evalTargetVersionDao.GetEvalTargetVersionByTarget(ctx, do.SpaceID, id, do.EvalTargetVersion.SourceTargetVersion, opt, db.WithSelectForUpdate())
		if errGetVersion != nil {
			return errGetVersion
		}
		// 如果版本没有创建过，则创建
		if version == nil {
			do.EvalTargetVersion.ID = versionID
			do.EvalTargetVersion.TargetID = id
			po, errTOPO := convertor.EvalTargetVersionDO2PO(do.EvalTargetVersion)
			if errTOPO != nil {
				return errTOPO
			}
			errCV := e.evalTargetVersionDao.CreateEvalTargetVersion(ctx, po, opt)
			if errCV != nil {
				return errCV
			}
		} else {
			versionID = version.ID
		}
		return nil
	})
	if err != nil {
		return 0, 0, errorx.WrapByCode(err, errno.CommonRPCErrorCode)
	}

	e.lwt.SetWriteFlag(ctx, platestwrite.ResourceTypeTarget, do.ID)
	e.lwt.SetWriteFlag(ctx, platestwrite.ResourceTypeTargetVersion, versionID)

	return id, versionID, nil
}

func (e *EvalTargetRepoImpl) GetEvalTarget(ctx context.Context, targetID int64) (do *entity.EvalTarget, err error) {
	var opts []db.Option
	if e.lwt.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeTarget, targetID) {
		opts = append(opts, db.WithMaster())
	}
	target, err := e.evalTargetDao.GetEvalTarget(ctx, targetID, opts...)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, nil
	}
	do = convertor.EvalTargetPO2DO(target)
	return do, nil
}

func (e *EvalTargetRepoImpl) GetEvalTargetVersion(ctx context.Context, spaceID, versionID int64) (targetDO *entity.EvalTarget, err error) {
	var versionOpts []db.Option
	if e.lwt.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeTargetVersion, versionID) {
		versionOpts = append(versionOpts, db.WithMaster())
		logs.CtxInfo(ctx, "GetEvalTargetVersion CheckWriteFlagByID true")
	} else {
		logs.CtxInfo(ctx, "GetEvalTargetVersion CheckWriteFlagByID false")
	}
	versionPO, err := e.evalTargetVersionDao.GetEvalTargetVersion(ctx, spaceID, versionID, versionOpts...)
	if err != nil {
		return nil, err
	}
	if versionPO == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	var opts []db.Option
	if e.lwt.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeTarget, versionPO.TargetID) {
		opts = append(opts, db.WithMaster())
	}
	targetPO, err := e.evalTargetDao.GetEvalTarget(ctx, versionPO.TargetID, opts...)
	if err != nil {
		return nil, err
	}
	if targetPO == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	targetDO = convertor.EvalTargetPO2DO(targetPO)
	versionDO := convertor.EvalTargetVersionPO2DO(versionPO, targetDO.EvalTargetType)
	targetDO.EvalTargetVersion = versionDO

	return targetDO, nil
}

func (e *EvalTargetRepoImpl) BatchGetEvalTargetBySource(ctx context.Context, param *repo.BatchGetEvalTargetBySourceParam) (dos []*entity.EvalTarget, err error) {
	targets, err := e.evalTargetDao.BatchGetEvalTargetBySource(ctx, param.SpaceID, param.SourceTargetID, int32(param.TargetType))
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}
	return convertor.EvalTargetPO2DOs(targets), nil
}

func (e *EvalTargetRepoImpl) BatchGetEvalTargetVersion(ctx context.Context, spaceID int64, versionIDs []int64) (dos []*entity.EvalTarget, err error) {
	versions, err := e.evalTargetVersionDao.BatchGetEvalTargetVersion(ctx, spaceID, versionIDs)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, nil
	}
	targetIDs := make([]int64, 0)
	for _, version := range versions {
		targetIDs = append(targetIDs, version.TargetID)
	}
	targets, err := e.evalTargetDao.BatchGetEvalTarget(ctx, spaceID, targetIDs)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}
	targetMap := make(map[int64]*model.Target)
	for _, target := range targets {
		targetMap[target.ID] = target
	}
	dos = make([]*entity.EvalTarget, 0)
	for _, version := range versions {
		target, ok := targetMap[version.TargetID]
		if !ok || target == nil {
			continue
		}
		targetDO := convertor.EvalTargetPO2DO(target)
		versionDO := convertor.EvalTargetVersionPO2DO(version, targetDO.EvalTargetType)
		targetDO.EvalTargetVersion = versionDO
		dos = append(dos, targetDO)
	}
	return dos, nil
}

func (e *EvalTargetRepoImpl) CreateEvalTargetRecord(ctx context.Context, record *entity.EvalTargetRecord) (int64, error) {
	po, err := convertor.EvalTargetRecordDO2PO(record)
	if err != nil {
		return 0, errorx.WrapByCode(err, errno.CommonInternalErrorCode)
	}

	id, err := e.evalTargetRecordDao.Create(ctx, po)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (e *EvalTargetRepoImpl) GetEvalTargetRecordByIDAndSpaceID(ctx context.Context, spaceID int64, recordID int64) (*entity.EvalTargetRecord, error) {
	recordPO, err := e.evalTargetRecordDao.GetByIDAndSpaceID(ctx, recordID, spaceID)
	if err != nil {
		return nil, err
	}
	do, err := convertor.EvalTargetRecordPO2DO(recordPO)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.CommonInternalErrorCode)
	}
	return do, nil
}

func (e *EvalTargetRepoImpl) ListEvalTargetRecordByIDsAndSpaceID(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.EvalTargetRecord, error) {
	recordPOList, err := e.evalTargetRecordDao.ListByIDsAndSpaceID(ctx, recordIDs, spaceID)
	if err != nil {
		return nil, err
	}
	res := make([]*entity.EvalTargetRecord, 0)
	if len(recordPOList) == 0 {
		return res, nil
	}
	for _, recordPO := range recordPOList {
		do, err := convertor.EvalTargetRecordPO2DO(recordPO)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.CommonInternalErrorCode)
		}
		res = append(res, do)
	}

	return res, nil
}
