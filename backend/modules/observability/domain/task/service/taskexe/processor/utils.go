// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	dataset0 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

func getSession(ctx context.Context, task *task.Task) *common.Session {
	userIDStr := session.UserIDInCtxOrEmpty(ctx)
	if userIDStr == "" {
		userIDStr = task.GetBaseInfo().GetCreatedBy().GetUserID()
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		logs.CtxError(ctx, "[task-debug] AutoEvaluteProcessor OnChangeProcessor, ParseInt err:%v", err)
	}
	return &common.Session{
		UserID: gptr.Of(userID),
		//AppID:  gptr.Of(int32(717152)),
	}
}

func getCategory(taskType task.TaskType) entity.DatasetCategory {
	switch taskType {
	case task.TaskTypeAutoEval:
		return entity.DatasetCategory_Evaluation
	default:
		return entity.DatasetCategory_General
	}
}

// shouldTriggerBackfill 判断是否需要发送历史回溯MQ
func ShouldTriggerBackfill(taskDO *task.Task) bool {
	// 检查任务类型
	taskType := taskDO.GetTaskType()
	if taskType != task.TaskTypeAutoEval && taskType != task.TaskTypeAutoDataReflow {
		return false
	}

	// 检查回填时间配置
	rule := taskDO.GetRule()
	if rule == nil {
		return false
	}

	backfillTime := rule.GetBackfillEffectiveTime()
	if backfillTime == nil {
		return false
	}

	return backfillTime.GetStartAt() > 0 &&
		backfillTime.GetEndAt() > 0 &&
		backfillTime.GetStartAt() < backfillTime.GetEndAt()
}

func ShouldTriggerNewData(ctx context.Context, taskDO *task.Task) bool {
	// 检查任务类型
	taskType := taskDO.GetTaskType()
	if taskType != task.TaskTypeAutoEval && taskType != task.TaskTypeAutoDataReflow {
		return false
	}
	rule := taskDO.GetRule()
	if rule == nil {
		return false
	}

	effectiveTime := rule.GetEffectiveTime()
	if effectiveTime == nil {
		return false
	}
	logs.CtxInfo(ctx, "[auto_task] ShouldTriggerNewData, endAt:%d, startAt:%d", effectiveTime.GetEndAt(), effectiveTime.GetStartAt())

	return effectiveTime.GetEndAt() > 0 &&
		effectiveTime.GetStartAt() > 0 &&
		effectiveTime.GetStartAt() < effectiveTime.GetEndAt() &&
		time.Now().After(time.UnixMilli(effectiveTime.GetStartAt()))
}

func ToJSONString(ctx context.Context, obj interface{}) string {
	if obj == nil {
		return ""
	}
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		logs.CtxError(ctx, "JSON marshal error: %v", err)
		return ""
	}
	jsonStr := string(jsonData)
	return jsonStr
}

func getBasicEvaluationSetSchema(basicColumns []string) (*dataset0.DatasetSchema, []*expt.FieldMapping) {
	evaluationSetSchema := dataset0.NewDatasetSchema()
	var fromEvalSet []*expt.FieldMapping
	for _, column := range basicColumns {
		evaluationSetSchema.FieldSchemas = append(evaluationSetSchema.FieldSchemas, &dataset0.FieldSchema{
			Key:         gptr.Of(column),
			Name:        gptr.Of(column),
			Description: gptr.Of(column),
			ContentType: gptr.Of(common.ContentTypeText),
			TextSchema:  gptr.Of("{\"type\": \"string\"}"),
		})
		fromEvalSet = append(fromEvalSet, &expt.FieldMapping{
			FieldName:     gptr.Of(column),
			FromFieldName: gptr.Of(column),
		})
	}
	return evaluationSetSchema, fromEvalSet
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
// convertDatasetSchemaDTO2DO 转换数据集模式
func convertDatasetSchemaDTO2DO(schema *dataset0.DatasetSchema) entity.DatasetSchema {
	if schema == nil {
		return entity.DatasetSchema{}
	}

	result := entity.DatasetSchema{}

	if schema.IsSetFieldSchemas() {
		fieldSchemas := schema.GetFieldSchemas()
		result.FieldSchemas = make([]entity.FieldSchema, len(fieldSchemas))
		for i, fs := range fieldSchemas {
			key := fs.GetKey()
			if key == "" {
				key = fs.GetName()
			}
			name := fs.GetName()
			description := fs.GetDescription()
			textSchema := fs.GetTextSchema()
			result.FieldSchemas[i] = entity.FieldSchema{
				Key:         &key,
				Name:        name,
				Description: description,
				ContentType: convertContentTypeDTO2DO(fs.GetContentType()),
				TextSchema:  textSchema,
			}
		}
	}

	return result
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
// convertContentTypeDTO2DO 转换内容类型
func convertContentTypeDTO2DO(contentType common.ContentType) entity.ContentType {
	switch contentType {
	case common.ContentTypeText:
		return entity.ContentType_Text
	case common.ContentTypeImage:
		return entity.ContentType_Image
	case common.ContentTypeAudio:
		return entity.ContentType_Audio
	case common.ContentTypeMultiPart:
		return entity.ContentType_MultiPart
	default:
		return entity.ContentType_Text
	}
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
func buildItems(ctx context.Context, spans []*loop_span.Span, fieldMappings []*task.EvaluateFieldMapping,
	evaluationSetSchema string) (turns []*eval_set.Turn) {
	turns = make([]*eval_set.Turn, 0, len(spans))
	for _, span := range spans {
		fieldData := buildItem(ctx, span, fieldMappings, evaluationSetSchema)
		if len(fieldData) == 0 {
			continue
		}
		turns = append(turns, &eval_set.Turn{
			FieldDataList: fieldData,
		})
	}
	return turns
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
func buildItem(ctx context.Context, span *loop_span.Span, fieldMappings []*task.EvaluateFieldMapping,
	evaluationSetSchema string) []*eval_set.FieldData {
	var fieldDatas []*eval_set.FieldData
	fieldDatas = append(fieldDatas, &eval_set.FieldData{
		Key:  gptr.Of("trace_id"),
		Name: gptr.Of("trace_id"),
		Content: &common.Content{
			ContentType: gptr.Of(common.ContentTypeText),
			Text:        gptr.Of(span.TraceID),
		},
	})
	fieldDatas = append(fieldDatas, &eval_set.FieldData{
		Key:  gptr.Of("span_id"),
		Name: gptr.Of("span_id"),
		Content: &common.Content{
			ContentType: gptr.Of(common.ContentTypeText),
			Text:        gptr.Of(span.SpanID),
		},
	})
	for _, mapping := range fieldMappings {
		// 前端传入的是Name，评测集需要的是key，需要做一下mapping
		if mapping.EvalSetName == nil {
			logs.CtxInfo(ctx, "Evaluator field name is nil")
			continue
		}
		var evaluationSetSchemas []*eval_set.FieldSchema
		if evaluationSetSchema == "" {
			logs.CtxInfo(ctx, "Evaluation set schema is nil")
			continue
		}
		err := json.Unmarshal([]byte(evaluationSetSchema), &evaluationSetSchemas)
		if err != nil {
			logs.CtxInfo(ctx, "Unmarshal evaluation set schema failed, err:%v", err)
			continue
		}
		for _, fieldSchema := range evaluationSetSchemas {
			if fieldSchema.GetKey() == *mapping.EvalSetName {
				key := fieldSchema.GetKey()
				if key == "" {
					logs.CtxInfo(ctx, "Evaluator field key is empty, name:%v", *mapping.FieldSchema.Name)
					continue
				}
				value, err := span.ExtractByJsonpath(ctx, mapping.TraceFieldKey, mapping.TraceFieldJsonpath)
				if err != nil {
					logs.CtxInfo(ctx, "Extract field failed, err:%v", err)
					continue
				}
				content, err := GetContentInfo(ctx, fieldSchema.GetContentType(), value)
				if err != nil {
					logs.CtxInfo(ctx, "GetContentInfo failed, err:%v", err)
					return nil
				}
				fieldDatas = append(fieldDatas, &eval_set.FieldData{
					Key:     gptr.Of(key),
					Name:    gptr.Of(fieldSchema.GetName()),
					Content: content,
				})
			}
		}
	}
	return fieldDatas
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
func GetContentInfo(ctx context.Context, contentType common.ContentType, value string) (*common.Content, error) {
	var content *common.Content
	switch contentType {
	case common.ContentTypeMultiPart:
		var parts []tracespec.ModelMessagePart
		err := json.Unmarshal([]byte(value), &parts)
		if err != nil {
			logs.CtxInfo(ctx, "Unmarshal multi part failed, err:%v", err)
			return nil, err
		}
		var multiPart []*common.Content
		for _, part := range parts {
			// 本期仅支持回流图片的多模态数据，非ImageURL信息的，打包放进text
			switch part.Type {
			case tracespec.ModelMessagePartTypeImage:
				if part.ImageURL == nil {
					continue
				}
				multiPart = append(multiPart, &common.Content{
					ContentType: gptr.Of(common.ContentTypeImage),
					Image: &common.Image{
						Name: gptr.Of(part.ImageURL.Name),
						URL:  gptr.Of(part.ImageURL.URL),
					},
				})
			case tracespec.ModelMessagePartTypeText, tracespec.ModelMessagePartTypeFile:
				multiPart = append(multiPart, &common.Content{
					ContentType: gptr.Of(common.ContentTypeText),
					Text:        gptr.Of(part.Text),
				})
			default:
				logs.CtxWarn(ctx, "Unsupported part type: %s", part.Type)
				return nil, err
			}
		}
		content = &common.Content{
			ContentType: gptr.Of(common.ContentTypeMultiPart),
			MultiPart:   multiPart,
		}
	default:
		content = &common.Content{
			ContentType: gptr.Of(common.ContentTypeText),
			Text:        gptr.Of(value),
		}
	}
	return content, nil
}
