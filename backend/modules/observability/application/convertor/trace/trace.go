// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	traced "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/trace"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
)

func AdvanceInfoDO2DTO(info *loop_span.TraceAdvanceInfo) *trace.TraceAdvanceInfo {
	return &trace.TraceAdvanceInfo{
		TraceID: info.TraceId,
		Tokens: &trace.TokenCost{
			Input:  info.InputCost,
			Output: info.OutputCost,
		},
	}
}

func BatchAdvanceInfoDO2DTO(infos []*loop_span.TraceAdvanceInfo) []*trace.TraceAdvanceInfo {
	ret := make([]*trace.TraceAdvanceInfo, len(infos))
	for i, info := range infos {
		ret[i] = AdvanceInfoDO2DTO(info)
	}
	return ret
}

func FileMetaDO2DTO() {
}

func AdvanceInfoDO2TraceDTO(info *loop_span.TraceAdvanceInfo) *traced.Trace {
	return &traced.Trace{
		TraceID: &info.TraceId,
		Tokens: &traced.TokenCost{
			InputToken:  info.InputCost,
			OutputToken: info.OutputCost,
		},
	}
}

func BatchAdvanceInfoDO2TraceDTO(infos []*loop_span.TraceAdvanceInfo) []*traced.Trace {
	ret := make([]*traced.Trace, len(infos))
	for i, info := range infos {
		ret[i] = AdvanceInfoDO2TraceDTO(info)
	}
	return ret
}

func ChatMessagesDO2DTO(messages []*service.ChatMessage) []*trace.ChatMessage {
	if messages == nil {
		return nil
	}
	ret := make([]*trace.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		ret = append(ret, &trace.ChatMessage{
			MessageType: msg.MessageType,
			Span:        SpanDO2DTO(msg.Span, nil, nil, nil, nil, false),
		})
	}
	return ret
}
