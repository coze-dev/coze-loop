// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strconv"
	"time"

	annodto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/annotation"
	commdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/samber/lo"
)

const (
	CozeChatFeedbackAnnotationKey        = "chat_feedback"
	CozeChatFeedbackAnnotationValLike    = "like"
	CozeChatFeedbackAnnotationValDislike = "dislike"
)

func AnnotationDTO2DO(a *annodto.Annotation) (*loop_span.Annotation, error) {
	if a == nil {
		return nil, fmt.Errorf("annotation is nil")
	}
	ret := &loop_span.Annotation{
		ID:             ptr.From(a.ID),
		SpanID:         ptr.From(a.SpanID),
		TraceID:        ptr.From(a.TraceID),
		StartTime:      time.UnixMilli(ptr.From(a.StartTime)),
		WorkspaceID:    ptr.From(a.WorkspaceID),
		AnnotationType: loop_span.AnnotationType(ptr.From(a.Type)),
		Key:            ptr.From(a.Key),
		Status:         loop_span.AnnotationStatus(ptr.From(a.Status)),
		Reasoning:      ptr.From(a.Reasoning),
	}
	valueType := loop_span.AnnotationValueType(ptr.From(a.ValueType))
	switch valueType {
	case loop_span.AnnotationValueTypeLong:
		i, err := strconv.ParseInt(ptr.From(a.Value), 10, 64)
		if err != nil {
			return nil, err
		}
		ret.Value = loop_span.NewLongValue(i)
	case loop_span.AnnotationValueTypeDouble:
		f, err := strconv.ParseFloat(ptr.From(a.Value), 64)
		if err != nil {
			return nil, err
		}
		ret.Value = loop_span.NewDoubleValue(f)
	case loop_span.AnnotationValueTypeString:
		ret.Value = loop_span.NewStringValue(ptr.From(a.Value))
	case loop_span.AnnotationValueTypeBool:
		b, err := strconv.ParseBool(ptr.From(a.Value))
		if err != nil {
			return nil, err
		}
		ret.Value = loop_span.NewBoolValue(b)
	}
	return ret, nil
}

func AnnotationDO2DTO(
	a *loop_span.Annotation,
	userMap map[string]*common.UserInfo,
	evalMap map[int64]*rpc.Evaluator,
	tagMap map[int64]*rpc.TagInfo,
) *annodto.Annotation {
	ret := &annodto.Annotation{
		ID:          ptr.Of(a.ID),
		SpanID:      ptr.Of(a.SpanID),
		TraceID:     ptr.Of(a.TraceID),
		WorkspaceID: ptr.Of(a.WorkspaceID),
		StartTime:   ptr.Of(a.StartTime.UnixMilli()),
		Type:        ptr.Of(annodto.AnnotationType(a.AnnotationType)),
		Key:         ptr.Of(a.Key),
		ValueType:   ptr.Of(annodto.ValueType(a.Value.ValueType)),
		Status:      ptr.Of(string(a.Status)),
		Reasoning:   ptr.Of(a.Reasoning),
	}
	switch a.Value.ValueType {
	case loop_span.AnnotationValueTypeLong:
		ret.Value = ptr.Of(strconv.FormatInt(a.Value.LongValue, 10))
	case loop_span.AnnotationValueTypeString:
		ret.Value = ptr.Of(a.Value.StringValue)
	case loop_span.AnnotationValueTypeBool:
		ret.Value = ptr.Of(strconv.FormatBool(a.Value.BoolValue))
	case loop_span.AnnotationValueTypeDouble:
		ret.Value = ptr.Of(strconv.FormatFloat(a.Value.FloatValue, 'f', -1, 64))
	}
	// user info
	ret.BaseInfo = &commdto.BaseInfo{
		CreatedAt: ptr.Of(a.CreatedAt.UnixMilli()),
		UpdatedAt: ptr.Of(a.UpdatedAt.UnixMilli()),
	}
	if userInfo, ok := userMap[a.CreatedBy]; ok {
		ret.BaseInfo.CreatedBy = UserInfoDO2DTO(userInfo)
	}
	if userInfo, ok := userMap[a.UpdatedBy]; ok {
		ret.BaseInfo.UpdatedBy = UserInfoDO2DTO(userInfo)
	}
	// auto eval info
	if a.AnnotationType == loop_span.AnnotationTypeAutoEvaluate {
		meta := a.GetAutoEvaluateMetadata()
		if meta != nil {
			ret.AutoEvaluate = annodto.NewAutoEvaluate()
			ret.AutoEvaluate.EvaluatorVersionID = meta.EvaluatorVersionID
			ret.AutoEvaluate.TaskID = strconv.FormatInt(meta.TaskID, 10)
			ret.AutoEvaluate.RecordID = meta.EvaluatorRecordID
			ret.AutoEvaluate.EvaluatorResult_ = annodto.NewEvaluatorResult_()
			ret.AutoEvaluate.EvaluatorResult_.Score = ptr.Of(a.Value.FloatValue)
			ret.AutoEvaluate.EvaluatorResult_.Reasoning = ptr.Of(a.Reasoning)
			if len(a.Corrections) > 0 {
				manualCorrections := lo.Filter(a.Corrections, func(item loop_span.AnnotationCorrection, index int) bool {
					return item.Type == loop_span.AnnotationCorrectionTypeManual
				})
				if len(manualCorrections) > 0 {
					manualCorrection := manualCorrections[len(manualCorrections)-1]
					ret.AutoEvaluate.EvaluatorResult_.Correction = annodto.NewCorrection()
					ret.AutoEvaluate.EvaluatorResult_.Correction.Score = ptr.Of(manualCorrection.Value.FloatValue)
					ret.AutoEvaluate.EvaluatorResult_.Correction.Explain = ptr.Of(manualCorrection.Reasoning)
				}
			}
			if evalInfo, ok := evalMap[meta.EvaluatorVersionID]; ok {
				ret.AutoEvaluate.EvaluatorName = evalInfo.EvaluatorName
				ret.AutoEvaluate.EvaluatorVersion = evalInfo.EvaluatorVersion
			}
		}
	}
	// tag info
	if a.AnnotationType == loop_span.AnnotationTypeManualFeedback {
		ret.ManualFeedback = annodto.NewManualFeedback()
		keyId, _ := strconv.ParseInt(a.Key, 10, 64)
		ret.ManualFeedback.TagKeyID = keyId
		if tagInfo, ok := tagMap[keyId]; ok {
			ret.ManualFeedback.TagKeyName = tagInfo.TagKeyName
			switch tagInfo.TagContentType {
			case rpc.TagContentTypeCategorical, rpc.TagContentTypeBoolean:
				ret.ManualFeedback.TagValueID = ptr.Of(a.Value.LongValue)
				if tagVal := tagInfo.GetTagValue(a.Value.LongValue); tagVal != nil {
					ret.ManualFeedback.TagValue = ptr.Of(tagVal.TagValueName)
				}
			case rpc.TagContentTypeContinuousNumber:
				ret.ManualFeedback.TagValue = ptr.Of(strconv.FormatFloat(a.Value.FloatValue, 'f', -1, 64))
			case rpc.TagContentTypeFreeText:
				ret.ManualFeedback.TagValue = ptr.Of(a.Value.StringValue)
			}
		}
	}
	if a.AnnotationType == loop_span.AnnotationTypeCozeFeedback {
		if a.Key == CozeChatFeedbackAnnotationKey {
			ret.Key = ptr.Of("消息反馈")
		}
		switch a.Value.StringValue {
		case CozeChatFeedbackAnnotationValLike:
			ret.Value = ptr.Of("赞")
		case CozeChatFeedbackAnnotationValDislike:
			ret.Value = ptr.Of("踩")
		}
	}
	return ret
}

func AnnotationListDO2DTO(
	annotations loop_span.AnnotationList,
	userMap map[string]*common.UserInfo,
	evalMap map[int64]*rpc.Evaluator,
	tagMap map[int64]*rpc.TagInfo,
) []*annodto.Annotation {
	ret := make([]*annodto.Annotation, 0)
	for _, a := range annotations {
		switch a.AnnotationType {
		case loop_span.AnnotationTypeAutoEvaluate:
			fallthrough
		case loop_span.AnnotationTypeManualFeedback:
			fallthrough
		case loop_span.AnnotationTypeCozeFeedback:
			ret = append(ret, AnnotationDO2DTO(a, userMap, evalMap, tagMap))
		default:
			continue
		}
	}
	return ret
}
