// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package looptracer

import (
	"context"
	"time"

	"github.com/coze-dev/cozeloop-go"
	"github.com/coze-dev/cozeloop-go/entity"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

var _ Span = (*SpanImpl)(nil)

type SpanImpl struct {
	LoopSpan cozeloop.Span
}

func (s SpanImpl) GetSpanID() string {
	return s.LoopSpan.GetSpanID()
}

func (s SpanImpl) GetTraceID() string {
	return s.LoopSpan.GetTraceID()
}

func (s SpanImpl) GetBaggage() map[string]string {
	return s.LoopSpan.GetBaggage()
}

func (s SpanImpl) SetInput(ctx context.Context, input interface{}) {
	s.LoopSpan.SetInput(ctx, input)
}

func (s SpanImpl) SetOutput(ctx context.Context, output interface{}) {
	s.LoopSpan.SetOutput(ctx, output)
}

func (s SpanImpl) SetError(ctx context.Context, err error) {
	s.LoopSpan.SetError(ctx, err)
}

func (s SpanImpl) SetStatusCode(ctx context.Context, code int) {
	s.LoopSpan.SetStatusCode(ctx, code)
}

func (s SpanImpl) SetUserID(ctx context.Context, userID string) {
	s.LoopSpan.SetUserID(ctx, userID)
}

func (s SpanImpl) SetUserIDBaggage(ctx context.Context, userID string) {
	s.LoopSpan.SetUserIDBaggage(ctx, userID)
}

func (s SpanImpl) SetMessageID(ctx context.Context, messageID string) {
	s.LoopSpan.SetMessageID(ctx, messageID)
}

func (s SpanImpl) SetMessageIDBaggage(ctx context.Context, messageID string) {
	s.LoopSpan.SetMessageIDBaggage(ctx, messageID)
}

func (s SpanImpl) SetThreadID(ctx context.Context, threadID string) {
	s.LoopSpan.SetThreadID(ctx, threadID)
}

func (s SpanImpl) SetThreadIDBaggage(ctx context.Context, threadID string) {
	s.LoopSpan.SetThreadIDBaggage(ctx, threadID)
}

func (s SpanImpl) SetPrompt(ctx context.Context, prompt entity.Prompt) {
	s.LoopSpan.SetPrompt(ctx, prompt)
}

func (s SpanImpl) SetModelProvider(ctx context.Context, modelProvider string) {
	s.LoopSpan.SetModelProvider(ctx, modelProvider)
}

func (s SpanImpl) SetModelName(ctx context.Context, modelName string) {
	s.LoopSpan.SetModelName(ctx, modelName)
}

func (s SpanImpl) SetModelCallOptions(ctx context.Context, callOptions interface{}) {
	s.LoopSpan.SetModelCallOptions(ctx, callOptions)
}

func (s SpanImpl) SetInputTokens(ctx context.Context, inputTokens int) {
	s.LoopSpan.SetInputTokens(ctx, inputTokens)
}

func (s SpanImpl) SetOutputTokens(ctx context.Context, outputTokens int) {
	s.LoopSpan.SetOutputTokens(ctx, outputTokens)
}

func (s SpanImpl) SetStartTimeFirstResp(ctx context.Context, startTimeFirstResp int64) {
	s.LoopSpan.SetStartTimeFirstResp(ctx, startTimeFirstResp)
}

func (s SpanImpl) SetRuntime(ctx context.Context, runtime tracespec.Runtime) {
	s.LoopSpan.SetRuntime(ctx, runtime)
}

func (s SpanImpl) SetTags(ctx context.Context, tagKVs map[string]interface{}) {
	s.LoopSpan.SetTags(ctx, tagKVs)
}

func (s SpanImpl) SetBaggage(ctx context.Context, baggageItems map[string]string) {
	s.LoopSpan.SetBaggage(ctx, baggageItems)
}

func (s SpanImpl) Finish(ctx context.Context) {
	s.LoopSpan.Finish(ctx)
}

func (s SpanImpl) GetStartTime() time.Time {
	return s.LoopSpan.GetStartTime()
}

func (s SpanImpl) ToHeader() (map[string]string, error) {
	return s.LoopSpan.ToHeader()
}

func (s SpanImpl) SetCallType(callType string) {
	s.LoopSpan.SetBaggage(context.Background(), map[string]string{
		tracespec.CallType: callType,
	})
}

func (s SpanImpl) SetServiceName(ctx context.Context, serviceName string) {
	s.LoopSpan.SetServiceName(ctx, serviceName)
}

func (s SpanImpl) SetLogID(ctx context.Context, logID string) {
	s.LoopSpan.SetLogID(ctx, logID)
}

func (s SpanImpl) SetFinishTime(finishTime time.Time) {
	s.LoopSpan.SetFinishTime(finishTime)
}

func (s SpanImpl) SetSystemTags(ctx context.Context, systemTags map[string]interface{}) {
	s.LoopSpan.SetSystemTags(ctx, systemTags)
}

func (s SpanImpl) SetDeploymentEnv(ctx context.Context, deploymentEnv string) {
	s.LoopSpan.SetDeploymentEnv(ctx, deploymentEnv)
}

var _ Span = (*noopSpan)(nil)

type noopSpan struct{}

func (n noopSpan) GetSpanID() string {
	return ""
}

func (n noopSpan) GetTraceID() string {
	return ""
}

func (n noopSpan) GetBaggage() map[string]string {
	return nil
}

func (n noopSpan) SetInput(ctx context.Context, input interface{}) {
}

func (n noopSpan) SetOutput(ctx context.Context, output interface{}) {
}

func (n noopSpan) SetError(ctx context.Context, err error) {
}

func (n noopSpan) SetStatusCode(ctx context.Context, code int) {
}

func (n noopSpan) SetUserID(ctx context.Context, userID string) {
}

func (n noopSpan) SetUserIDBaggage(ctx context.Context, userID string) {
}

func (n noopSpan) SetMessageID(ctx context.Context, messageID string) {
}

func (n noopSpan) SetMessageIDBaggage(ctx context.Context, messageID string) {
}

func (n noopSpan) SetThreadID(ctx context.Context, threadID string) {
}

func (n noopSpan) SetThreadIDBaggage(ctx context.Context, threadID string) {
}

func (n noopSpan) SetPrompt(ctx context.Context, prompt entity.Prompt) {
}

func (n noopSpan) SetModelProvider(ctx context.Context, modelProvider string) {
}

func (n noopSpan) SetModelName(ctx context.Context, modelName string) {
}

func (n noopSpan) SetModelCallOptions(ctx context.Context, callOptions interface{}) {
}

func (n noopSpan) SetInputTokens(ctx context.Context, inputTokens int) {
}

func (n noopSpan) SetOutputTokens(ctx context.Context, outputTokens int) {
}

func (n noopSpan) SetStartTimeFirstResp(ctx context.Context, startTimeFirstResp int64) {
}

func (n noopSpan) SetRuntime(ctx context.Context, runtime tracespec.Runtime) {
}

func (n noopSpan) SetTags(ctx context.Context, tagKVs map[string]interface{}) {
}

func (n noopSpan) SetBaggage(ctx context.Context, baggageItems map[string]string) {
}

func (n noopSpan) Finish(ctx context.Context) {
}

func (n noopSpan) GetStartTime() time.Time {
	return time.Time{}
}

func (n noopSpan) ToHeader() (map[string]string, error) {
	return map[string]string{}, nil
}

func (n noopSpan) SetCallType(callType string) {
}

func (n noopSpan) SetServiceName(ctx context.Context, serviceName string) {
}

func (n noopSpan) SetLogID(ctx context.Context, logID string) {
}

func (n noopSpan) SetFinishTime(finishTime time.Time) {
}

func (n noopSpan) SetSystemTags(ctx context.Context, systemTags map[string]interface{}) {
}

func (n noopSpan) SetDeploymentEnv(ctx context.Context, deploymentEnv string) {
}
