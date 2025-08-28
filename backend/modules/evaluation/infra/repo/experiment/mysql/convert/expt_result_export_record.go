// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
)

func ExptResultExportRecordDOToPO(do *entity.ExptResultExportRecord) *model.ExptResultExportRecord {
	if do == nil {
		return nil
	}
	po := &model.ExptResultExportRecord{
		ID:              do.ID,
		SpaceID:         do.SpaceID,
		ExptID:          do.ExptID,
		CsvExportStatus: int32(do.CsvExportStatus),
		FilePath:        do.FilePath,
		StartAt:         do.StartAt,
		EndAt:           do.EndAt,
		CreatedBy:       do.CreatedBy,
		ErrMsg:          gptr.Of(conv.UnsafeStringToBytes(do.ErrMsg)),
	}

	return po
}

func ExptResultExportRecordPOToDO(po *model.ExptResultExportRecord) *entity.ExptResultExportRecord {
	if po == nil {
		return nil
	}
	do := &entity.ExptResultExportRecord{
		ID:              po.ID,
		SpaceID:         po.SpaceID,
		ExptID:          po.ExptID,
		CsvExportStatus: entity.CSVExportStatus(po.CsvExportStatus),
		FilePath:        po.FilePath,
		CreatedBy:       po.CreatedBy,
		StartAt:         po.StartAt,
		EndAt:           po.EndAt,
		ErrMsg:          conv.UnsafeBytesToString(gptr.Indirect(po.ErrMsg)),
	}

	return do
}
