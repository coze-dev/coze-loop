// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"fmt"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func AnnotateRecordDOToPO(do *entity.AnnotateRecord) (*model.AnnotateRecord, error) {
	if do == nil {
		return nil, nil
	}

	if do.AnnotateData == nil {
		return nil, fmt.Errorf("annotate data is nil")
	}

	po := &model.AnnotateRecord{
		ID:           do.ID,
		SpaceID:      do.SpaceID,
		TagKeyID:     do.TagKeyID,
		ExperimentID: do.ExperimentID,
		TagValueID:   do.TagValueID,
	}

	switch do.AnnotateData.TagContentType {
	case entity.TagContentTypeContinuousNumber:
		po.Score = gptr.Indirect(do.AnnotateData.Score)
	case entity.TagContentTypeCategorical:
		po.TextValue = gptr.Indirect(do.AnnotateData.Option)
	case entity.TagContentTypeFreeText:
		po.TextValue = gptr.Indirect(do.AnnotateData.TextValue)
	case entity.TagContentTypeBoolean:
		po.TextValue = gptr.Indirect(do.AnnotateData.BoolValue)
	}

	annotateDataBytes, err := json.Marshal(do.AnnotateData)
	if err != nil {
		return nil, err
	}
	po.AnnotateData = annotateDataBytes

	return po, nil
}

func AnnotateRecordPOToDO(po *model.AnnotateRecord) (*entity.AnnotateRecord, error) {
	annotateData := &entity.AnnotateData{}
	err := json.Unmarshal(po.AnnotateData, annotateData)
	if err != nil {
		return nil, err
	}

	do := &entity.AnnotateRecord{
		ID:           po.ID,
		SpaceID:      po.SpaceID,
		TagKeyID:     po.TagKeyID,
		ExperimentID: po.ExperimentID,
		AnnotateData: annotateData,
		TagValueID:   po.TagValueID,
	}

	return do, nil
}

func ExptTurnAnnotateRecordRefDOToPO(do *entity.ExptTurnAnnotateRecordRef) *model.ExptTurnAnnotateRecordRef {
	po := &model.ExptTurnAnnotateRecordRef{
		ID:               do.ID,
		SpaceID:          do.SpaceID,
		ExptTurnResultID: do.ExptTurnResultID,
		TagKeyID:         do.TagKeyID,
		AnnotateRecordID: do.AnnotateRecordID,
		ExptID:           do.ExptID,
	}
	return po
}

func ExptTurnAnnotateRecordRefPOToDO(po *model.ExptTurnAnnotateRecordRef) *entity.ExptTurnAnnotateRecordRef {
	do := &entity.ExptTurnAnnotateRecordRef{
		ID:               po.ID,
		SpaceID:          po.SpaceID,
		ExptTurnResultID: po.ExptTurnResultID,
		TagKeyID:         po.TagKeyID,
		AnnotateRecordID: po.AnnotateRecordID,
		ExptID:           po.ExptID,
	}
	return do
}

func ExptTurnResultTagRefDOToPO(do *entity.ExptTurnResultTagRef) *model.ExptTurnResultTagRef {
	po := &model.ExptTurnResultTagRef{
		ID:          do.ID,
		SpaceID:     do.SpaceID,
		TagKeyID:    do.TagKeyID,
		ExptID:      do.ExptID,
		TotalCnt:    do.TotalCnt,
		CompleteCnt: do.CompleteCnt,
	}
	return po
}

func ExptTurnResultTagRefPOToDO(po *model.ExptTurnResultTagRef) *entity.ExptTurnResultTagRef {
	do := &entity.ExptTurnResultTagRef{
		ID:          po.ID,
		SpaceID:     po.SpaceID,
		TagKeyID:    po.TagKeyID,
		ExptID:      po.ExptID,
		TotalCnt:    po.TotalCnt,
		CompleteCnt: po.CompleteCnt,
	}
	return do
}
