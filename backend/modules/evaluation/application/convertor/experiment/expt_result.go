// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strconv"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	domain_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	evalsetconv "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	evaluatorconv "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	targetconv "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func ColumnEvalSetFieldsDO2DTOs(from []*entity.ColumnEvalSetField) []*domain_expt.ColumnEvalSetField {
	fields := make([]*domain_expt.ColumnEvalSetField, 0, len(from))
	for _, f := range from {
		fields = append(fields, ColumnEvalSetFieldsDO2DTO(f))
	}
	return fields
}

func ColumnEvalSetFieldsDO2DTO(from *entity.ColumnEvalSetField) *domain_expt.ColumnEvalSetField {
	contentType := common.ConvertContentTypeDO2DTO(from.ContentType)
	return &domain_expt.ColumnEvalSetField{
		Key:         from.Key,
		Name:        from.Name,
		Description: from.Description,
		ContentType: &contentType,
		TextSchema:  from.TextSchema,
	}
}

func ExptColumnEvaluatorsDO2DTOs(from []*entity.ExptColumnEvaluator) []*domain_expt.ExptColumnEvaluator {
	dtos := make([]*domain_expt.ExptColumnEvaluator, 0, len(from))
	for _, f := range from {
		dto := &domain_expt.ExptColumnEvaluator{
			ExperimentID:     f.ExptID,
			ColumnEvaluators: ColumnEvaluatorsDO2DTOs(f.ColumnEvaluators),
		}
		dtos = append(dtos, dto)
	}
	return dtos
}

func ColumnEvaluatorsDO2DTOs(from []*entity.ColumnEvaluator) []*domain_expt.ColumnEvaluator {
	evaluators := make([]*domain_expt.ColumnEvaluator, 0, len(from))
	for _, f := range from {
		evaluators = append(evaluators, ColumnEvaluatorsDO2DTO(f))
	}
	return evaluators
}

func ColumnEvaluatorsDO2DTO(from *entity.ColumnEvaluator) *domain_expt.ColumnEvaluator {
	return &domain_expt.ColumnEvaluator{
		EvaluatorVersionID: from.EvaluatorVersionID,
		EvaluatorID:        from.EvaluatorID,
		EvaluatorType:      evaluator.EvaluatorType(from.EvaluatorType),
		Name:               from.Name,
		Version:            from.Version,
		Description:        from.Description,
	}
}

func TagValueDO2DtO(tagValue *entity.TagValue) *tag.TagValue {
	return &tag.TagValue{
		TagValueID:   ptr.Of(tagValue.TagValueId),
		TagValueName: ptr.Of(tagValue.TagValueName),
		Status:       ptr.Of(tagValue.Status),
	}
}

func TagValueListDO2DTO(tagValues []*entity.TagValue) []*tag.TagValue {
	ret := make([]*tag.TagValue, 0, len(tagValues))
	for _, tagValue := range tagValues {
		ret = append(ret, TagValueDO2DtO(tagValue))
	}
	return ret
}

func ExptColumnAnnotationDO2DTOs(from []*entity.ExptColumnAnnotation) []*domain_expt.ExptColumnAnnotation {
	annotations := make([]*domain_expt.ExptColumnAnnotation, 0, len(from))
	for _, f := range from {
		dto := &domain_expt.ExptColumnAnnotation{
			ExperimentID:      f.ExptID,
			ColumnAnnotations: ColumnAnnotationDO2DTOs(f.ColumnAnnotations),
		}
		annotations = append(annotations, dto)
	}
	return annotations
}

func ColumnAnnotationDO2DTOs(from []*entity.ColumnAnnotation) []*domain_expt.ColumnAnnotation {
	annotations := make([]*domain_expt.ColumnAnnotation, 0, len(from))
	for _, f := range from {
		annotations = append(annotations, ColumnAnnotationDO2DTO(f))
	}
	return annotations
}

func ColumnAnnotationDO2DTO(from *entity.ColumnAnnotation) *domain_expt.ColumnAnnotation {
	columnAnnotation := &domain_expt.ColumnAnnotation{
		TagKeyID:    ptr.Of(from.TagKeyID),
		TagKeyName:  ptr.Of(from.TagName),
		Description: ptr.Of(from.Description),
		TagValues:   TagValueListDO2DTO(from.TagValues),
		ContentType: ptr.Of(tag.TagContentType(from.TagContentType)),
		Status:      ptr.Of(from.TagStatus),
	}

	if from.TagContentSpec != nil && from.TagContentSpec.ContinuousNumberSpec != nil {
		columnAnnotation.ContentSpec = &tag.TagContentSpec{
			ContinuousNumberSpec: &tag.ContinuousNumberSpec{
				MinValue:            from.TagContentSpec.ContinuousNumberSpec.MinValue,
				MinValueDescription: from.TagContentSpec.ContinuousNumberSpec.MinValueDescription,
				MaxValue:            from.TagContentSpec.ContinuousNumberSpec.MaxValue,
				MaxValueDescription: from.TagContentSpec.ContinuousNumberSpec.MaxValueDescription,
			},
		}
	}
	return columnAnnotation
}

func ItemResultsDO2DTOs(from []*entity.ItemResult) []*domain_expt.ItemResult_ {
	results := make([]*domain_expt.ItemResult_, 0, len(from))
	for _, f := range from {
		results = append(results, ItemResultsDO2DTO(f))
	}
	return results
}

func ItemResultsDO2DTO(from *entity.ItemResult) *domain_expt.ItemResult_ {
	return &domain_expt.ItemResult_{
		ItemID:      from.ItemID,
		TurnResults: TurnResultsDO2DTOs(from.TurnResults),
		SystemInfo:  ItemSystemInfoDO2DTO(from.SystemInfo),
		ItemIndex:   from.ItemIndex,
	}
}

func TurnResultsDO2DTOs(from []*entity.TurnResult) []*domain_expt.TurnResult_ {
	results := make([]*domain_expt.TurnResult_, 0, len(from))
	for _, f := range from {
		results = append(results, TurnResultsDO2DTO(f))
	}
	return results
}

func TurnResultsDO2DTO(from *entity.TurnResult) *domain_expt.TurnResult_ {
	return &domain_expt.TurnResult_{
		TurnID:            from.TurnID,
		ExperimentResults: ExperimentResultsDO2DTOs(from.ExperimentResults),
		TurnIndex:         from.TurnIndex,
	}
}

func ExperimentResultsDO2DTOs(from []*entity.ExperimentResult) []*domain_expt.ExperimentResult_ {
	results := make([]*domain_expt.ExperimentResult_, 0, len(from))
	for _, f := range from {
		results = append(results, ExperimentResultsDO2DTO(f))
	}
	return results
}

func ExperimentResultsDO2DTO(from *entity.ExperimentResult) *domain_expt.ExperimentResult_ {
	return &domain_expt.ExperimentResult_{
		ExperimentID: from.ExperimentID,
		Payload:      ExperimentTurnPayloadDO2DTO(from.Payload),
	}
}

func ExperimentTurnPayloadDO2DTO(from *entity.ExperimentTurnPayload) *domain_expt.ExperimentTurnPayload {
	return &domain_expt.ExperimentTurnPayload{
		TurnID:          from.TurnID,
		EvalSet:         TurnEvalSetDO2DTO(from.EvalSet),
		TargetOutput:    TurnTargetOutputDO2DTO(from.TargetOutput),
		EvaluatorOutput: TurnEvaluatorOutputDO2DTO(from.EvaluatorOutput),
		SystemInfo:      TurnSystemInfoDO2DTO(from.SystemInfo),
		AnnotateResult_: TurnAnnotationDO2DTO(from.AnnotateResult),
	}
}

func TurnAnnotationDO2DTO(from *entity.TurnAnnotateResult) *domain_expt.TurnAnnotateResult_ {
	if from == nil {
		return &domain_expt.TurnAnnotateResult_{}
	}

	annotateRecords := make(map[int64]*domain_expt.AnnotateRecord)
	for k, v := range from.AnnotateRecords {
		annotateRecords[k] = AnnotateRecordsDO2DTO(v)
	}

	return &domain_expt.TurnAnnotateResult_{
		AnnotateRecords: annotateRecords,
	}
}

func AnnotateRecordsDO2DTO(from *entity.AnnotateRecord) *domain_expt.AnnotateRecord {
	if from == nil || from.AnnotateData == nil {
		return &domain_expt.AnnotateRecord{}
	}

	return &domain_expt.AnnotateRecord{
		AnnotateRecordID:  ptr.Of(from.ID),
		TagKeyID:          ptr.Of(from.TagKeyID),
		Score:             ptr.Of(strconv.FormatFloat(ptr.From(from.AnnotateData.Score), 'f', -1, 64)),
		BooleanOption:     from.AnnotateData.BoolValue,
		PlainText:         from.AnnotateData.TextValue,
		CategoricalOption: from.AnnotateData.Option,
		TagContentType:    ptr.Of(tag.TagContentType(from.AnnotateData.TagContentType)),
		TagValueID:        ptr.Of(from.TagValueID),
	}
}

func TurnEvaluatorOutputDO2DTO(from *entity.TurnEvaluatorOutput) *domain_expt.TurnEvaluatorOutput {
	if from == nil {
		return &domain_expt.TurnEvaluatorOutput{}
	}
	evaluatorRecords := make(map[int64]*evaluator.EvaluatorRecord)
	for k, v := range from.EvaluatorRecords {
		evaluatorRecords[k] = evaluatorconv.ConvertEvaluatorRecordDO2DTO(v)
	}
	return &domain_expt.TurnEvaluatorOutput{
		EvaluatorRecords: evaluatorRecords,
	}
}

func TurnTargetOutputDO2DTO(from *entity.TurnTargetOutput) *domain_expt.TurnTargetOutput {
	if from == nil {
		return &domain_expt.TurnTargetOutput{}
	}
	return &domain_expt.TurnTargetOutput{
		EvalTargetRecord: targetconv.EvalTargetRecordDO2DTO(from.EvalTargetRecord),
	}
}

func TurnEvalSetDO2DTO(from *entity.TurnEvalSet) *domain_expt.TurnEvalSet {
	if from == nil {
		return &domain_expt.TurnEvalSet{}
	}
	return &domain_expt.TurnEvalSet{
		Turn: evalsetconv.TurnDO2DTO(from.Turn),
	}
}

func TurnSystemInfoDO2DTO(from *entity.TurnSystemInfo) *domain_expt.TurnSystemInfo {
	if from == nil {
		return &domain_expt.TurnSystemInfo{}
	}
	return &domain_expt.TurnSystemInfo{
		TurnRunState: domain_expt.TurnRunStatePtr(domain_expt.TurnRunState(from.TurnRunState)),
		LogID:        from.LogID,
		Error:        RunErrorDO2DTO(from.Error),
	}
}

func RunErrorDO2DTO(from *entity.RunError) *domain_expt.RunError {
	if from == nil {
		return nil
	}
	return &domain_expt.RunError{
		Code:    from.Code,
		Message: from.Message,
		Detail:  from.Detail,
	}
}

func ItemSystemInfoDO2DTO(from *entity.ItemSystemInfo) *domain_expt.ItemSystemInfo {
	if from == nil {
		return &domain_expt.ItemSystemInfo{}
	}
	return &domain_expt.ItemSystemInfo{
		RunState: domain_expt.ItemRunStatePtr(domain_expt.ItemRunState(from.RunState)),
		LogID:    from.LogID,
		Error:    RunErrorDO2DTO(from.Error),
	}
}

func CSVExportStatusDO2DTO(from entity.CSVExportStatus) domain_expt.CSVExportStatus {
	switch from {
	case entity.CSVExportStatus_Unknown:
		return domain_expt.CSVExportStatusUnknown
	case entity.CSVExportStatus_Running:
		return domain_expt.CSVExportStatusRunning
	case entity.CSVExportStatus_Success:
		return domain_expt.CSVExportStatusSuccess
	case entity.CSVExportStatus_Failed:
		return domain_expt.CSVExportStatusFailed
	default:
		return domain_expt.CSVExportStatusUnknown
	}
}

func ExportRecordDO2DTO(from *entity.ExptResultExportRecord) *domain_expt.ExptResultExportRecord {
	if from == nil {
		return nil
	}
	res := &domain_expt.ExptResultExportRecord{
		ExportID:        from.ID,
		WorkspaceID:     from.SpaceID,
		ExptID:          from.ExptID,
		CsvExportStatus: CSVExportStatusDO2DTO(from.CsvExportStatus),
		BaseInfo: &domain_common.BaseInfo{
			CreatedBy: &domain_common.UserInfo{
				UserID: ptr.Of(from.CreatedBy),
			},
		},
		URL:     from.URL,
		Expired: ptr.Of(from.Expired),
	}

	if from.StartAt != nil {
		res.StartTime = gptr.Of(from.StartAt.Unix())
	}
	if from.EndAt != nil {
		res.EndTime = gptr.Of(from.EndAt.Unix())
	}

	if len(from.ErrMsg) > 0 {
		errImpl, ok := errno.ParseErrImpl(errno.DeserializeErr([]byte(from.ErrMsg)))
		if ok {
			res.Error = &domain_expt.RunError{
				Detail: gptr.Of(errImpl.ErrMsg()),
			}
		}
	}

	return res
}
