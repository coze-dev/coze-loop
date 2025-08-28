// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptAnnotateServiceImpl struct {
	txDB                     db.Provider
	repo                     repo.IExptAnnotateRepo
	exptRepo                 repo.IExperimentRepo
	exptTurnResultRepo       repo.IExptTurnResultRepo
	exptPublisher            events.ExptEventPublisher
	evaluationSetItemService EvaluationSetItemService
	exptResultService        ExptResultService
	exptTurnResultFilterRepo repo.IExptTurnResultFilterRepo
	exptAggrResultRepo       repo.IExptAggrResultRepo
}

func NewExptAnnotateService(txDB db.Provider, repo repo.IExptAnnotateRepo, exptTurnResultRepo repo.IExptTurnResultRepo, exptPublisher events.ExptEventPublisher, evaluationSetItemService EvaluationSetItemService, exptRepo repo.IExperimentRepo, exptResultService ExptResultService, exptTurnResultFilterRepo repo.IExptTurnResultFilterRepo, exptAggrResultRepo repo.IExptAggrResultRepo) IExptAnnotateService {
	return &ExptAnnotateServiceImpl{
		repo:                     repo,
		txDB:                     txDB,
		exptTurnResultRepo:       exptTurnResultRepo,
		exptPublisher:            exptPublisher,
		evaluationSetItemService: evaluationSetItemService,
		exptRepo:                 exptRepo,
		exptResultService:        exptResultService,
		exptTurnResultFilterRepo: exptTurnResultFilterRepo,
		exptAggrResultRepo:       exptAggrResultRepo,
	}
}

func (e ExptAnnotateServiceImpl) CreateExptTurnResultTagRefs(ctx context.Context, refs []*entity.ExptTurnResultTagRef) error {
	if len(refs) == 0 || refs[0] == nil {
		return nil
	}

	ref := refs[0]
	expt, err := e.exptRepo.GetByID(ctx, ref.ExptID, ref.SpaceID)
	if err != nil {
		return err
	}

	_, total, _, err := e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
		SpaceID:         ref.SpaceID,
		EvaluationSetID: expt.EvalSetID,
		VersionID:       ptr.Of(expt.EvalSetVersionID),
		PageNumber:      ptr.Of(int32(1)),
		PageSize:        ptr.Of(int32(1)),
	})
	if err != nil {
		return err
	}
	if total == nil {
		return fmt.Errorf("evaluation set items total is nil")
	}

	// 支持多轮后需要修改
	totalCnt := ptr.From(total)

	for _, r := range refs {
		r.TotalCnt = int32(totalCnt)
	}

	err = e.repo.CreateExptTurnResultTagRefs(ctx, refs)
	if err != nil {
		return err
	}

	//existMappings, err := e.exptTurnResultFilterRepo.GetExptTurnResultFilterKeyMappings(ctx, ref.SpaceID, ref.ExptID)
	//if err != nil {
	//	return err
	//}
	//// 根据已有的mapping数量计算ToKey
	//var annotateMappingCount int
	//for _, m := range existMappings {
	//	if m.FieldType == entity.FieldTypeManualAnnotation {
	//		annotateMappingCount++
	//	}
	//}

	exptTurnResultFilterKeyMappings := make([]*entity.ExptTurnResultFilterKeyMapping, 0)
	for _, r := range refs {
		exptTurnResultFilterKeyMappings = append(exptTurnResultFilterKeyMappings, &entity.ExptTurnResultFilterKeyMapping{
			SpaceID:   r.SpaceID,
			ExptID:    r.ExptID,
			FromField: strconv.FormatInt(r.TagKeyID, 10),
			ToKey:     strconv.FormatInt(r.TagKeyID, 10),
			FieldType: entity.FieldTypeManualAnnotation,
		})
	}

	if err := e.exptResultService.InsertExptTurnResultFilterKeyMappings(ctx, exptTurnResultFilterKeyMappings); err != nil {
		return err
	}

	return nil
}

func (e ExptAnnotateServiceImpl) DeleteExptTurnResultTagRef(ctx context.Context, exptID int64, spaceID int64, tagKeyID int64) error {
	mapping := &entity.ExptTurnResultFilterKeyMapping{
		SpaceID:   spaceID,
		ExptID:    exptID,
		FromField: strconv.FormatInt(tagKeyID, 10),
		FieldType: entity.FieldTypeManualAnnotation,
	}

	err := e.txDB.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx)}

		err := e.repo.DeleteExptTurnResultTagRef(ctx, exptID, spaceID, tagKeyID, opts...)
		if err != nil {
			return err
		}

		err = e.repo.DeleteTurnAnnotateRecordRef(ctx, exptID, spaceID, tagKeyID, opts...)
		if err != nil {
			return err
		}

		exptAggrResult := &entity.ExptAggrResult{
			SpaceID:      spaceID,
			ExperimentID: exptID,
			FieldType:    int32(entity.FieldType_Annotation),
			FieldKey:     strconv.FormatInt(tagKeyID, 10),
		}
		err = e.exptAggrResultRepo.DeleteExptAggrResult(ctx, exptAggrResult)
		if err != nil {
			return err
		}

		err = e.exptTurnResultFilterRepo.DeleteExptTurnResultFilterKeyMapping(ctx, mapping, opts...)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (e ExptAnnotateServiceImpl) GetExptTurnResultTagRefs(ctx context.Context, exptID, spaceID int64) ([]*entity.ExptTurnResultTagRef, error) {
	refs, err := e.repo.GetExptTurnResultTagRefs(ctx, exptID, spaceID)
	if err != nil {
		return nil, err
	}

	return refs, nil
}

func (e ExptAnnotateServiceImpl) SaveAnnotateRecord(ctx context.Context, exptID int64, itemID int64, turnID int64, record *entity.AnnotateRecord) error {
	turnResult, err := e.exptTurnResultRepo.Get(ctx, record.SpaceID, exptID, itemID, turnID)
	if err != nil {
		return err
	}

	turnResultID := turnResult.ID

	err = e.txDB.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx)}
		err = e.repo.SaveAnnotateRecord(ctx, turnResultID, record, opts...)
		if err != nil {
			return err
		}

		// calculate aggregate result
		_, _, err = e.repo.UpdateCompleteCount(ctx, exptID, record.SpaceID, record.TagKeyID, opts...)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	ctx = contexts.WithCtxWriteDB(ctx)
	tagRef, err := e.repo.GetTagRefByTagKeyID(ctx, exptID, record.SpaceID, record.TagKeyID)
	if err != nil {
		return err
	}

	if tagRef.CompleteCnt == tagRef.TotalCnt {
		event := &entity.AggrCalculateEvent{
			SpaceID:       record.SpaceID,
			ExperimentID:  exptID,
			CalculateMode: entity.CreateAnnotationFields,
			SpecificFieldInfo: &entity.SpecificFieldInfo{
				FieldKey:  strconv.FormatInt(record.TagKeyID, 10),
				FieldType: entity.FieldType_Annotation,
			},
		}
		err = e.exptPublisher.PublishExptAggrCalculateEvent(ctx, []*entity.AggrCalculateEvent{event}, gptr.Of(time.Second*3))
		if err != nil {
			return err
		}
	}

	err = e.exptResultService.UpsertExptTurnResultFilter(ctx, record.SpaceID, exptID, []int64{itemID})
	if err != nil {
		logs.CtxError(ctx, "UpsertExptTurnResultFilter fail, err: %v", err)
	}
	err = e.exptPublisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{
		ExperimentID: exptID,
		SpaceID:      record.SpaceID,
		ItemID:       []int64{itemID},
		RetryTimes:   ptr.Of(int32(0)),
		FilterType:   ptr.Of(entity.UpsertExptTurnResultFilterTypeCheck),
	}, ptr.Of(10*time.Second))
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "SaveAnnotateRecord UpsertExptTurnResultFilter done, expt_id: %v, item_ids: %v", exptID, []int64{itemID})

	return nil
}

func (e ExptAnnotateServiceImpl) UpdateAnnotateRecord(ctx context.Context, itemID int64, turnID int64, record *entity.AnnotateRecord) error {
	tagRef, err := e.repo.GetTagRefByTagKeyID(ctx, record.ExperimentID, record.SpaceID, record.TagKeyID)
	if err != nil {
		return err
	}

	err = e.repo.UpdateAnnotateRecord(ctx, record)
	if err != nil {
		return err
	}

	err = e.exptResultService.UpsertExptTurnResultFilter(ctx, record.SpaceID, record.ExperimentID, []int64{itemID})
	if err != nil {
		logs.CtxError(ctx, "UpsertExptTurnResultFilter fail, err: %v", err)
	}
	err = e.exptPublisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{
		ExperimentID: record.ExperimentID,
		SpaceID:      record.SpaceID,
		ItemID:       []int64{itemID},
		RetryTimes:   ptr.Of(int32(0)),
		FilterType:   ptr.Of(entity.UpsertExptTurnResultFilterTypeCheck),
	}, ptr.Of(10*time.Second))
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "UpdateAnnotateRecord UpsertExptTurnResultFilter done, expt_id: %v, item_ids: %v", record.ExperimentID, []int64{itemID})

	if tagRef.TotalCnt == tagRef.CompleteCnt {
		event := &entity.AggrCalculateEvent{
			SpaceID:       record.SpaceID,
			ExperimentID:  record.ExperimentID,
			CalculateMode: entity.UpdateAnnotationFields,
			SpecificFieldInfo: &entity.SpecificFieldInfo{
				FieldKey:  strconv.FormatInt(record.TagKeyID, 10),
				FieldType: entity.FieldType_Annotation,
			},
		}
		err = e.exptPublisher.PublishExptAggrCalculateEvent(ctx, []*entity.AggrCalculateEvent{event}, gptr.Of(time.Second*3))
		if err != nil {
			return err
		}
	}

	return nil
}

func (e ExptAnnotateServiceImpl) GetAnnotateRecordsByIDs(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.AnnotateRecord, error) {
	records, err := e.repo.GetAnnotateRecordsByIDs(ctx, spaceID, recordIDs)
	if err != nil {
		return nil, err
	}

	return records, nil
}
