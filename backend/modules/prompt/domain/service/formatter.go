// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"

	loopentity "github.com/coze-dev/cozeloop-go/entity"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/trace"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/traceutil"
)

// PromptFormatter defines the interface for formatting prompts
type PromptFormatter interface {
	FormatPrompt(ctx context.Context, prompt *entity.Prompt, messages []*entity.Message, variableVals []*entity.VariableVal) (formattedMessages []*entity.Message, err error)
}

// DefaultPromptFormatter provides the default implementation of PromptFormatter
type DefaultPromptFormatter struct{}

// NewDefaultPromptFormatter creates a new instance of DefaultPromptFormatter
func NewDefaultPromptFormatter() PromptFormatter {
	return &DefaultPromptFormatter{}
}

// FormatPrompt implements the PromptFormatter interface
func (f *DefaultPromptFormatter) FormatPrompt(ctx context.Context, prompt *entity.Prompt, messages []*entity.Message, variableVals []*entity.VariableVal) (formattedMessages []*entity.Message, err error) {
	if parentSpan := looptracer.GetTracer().GetSpanFromContext(ctx); parentSpan != nil {
		var span looptracer.Span
		ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNamePromptTemplate, tracespec.VPromptTemplateSpanType, looptracer.WithSpanWorkspaceID(strconv.FormatInt(prompt.SpaceID, 10)))
		if span != nil {
			span.SetPrompt(ctx, loopentity.Prompt{PromptKey: prompt.PromptKey, Version: prompt.GetVersion()})
			span.SetInput(ctx, json.Jsonify(tracespec.PromptInput{
				Templates: trace.MessagesToSpanMessages(prompt.GetTemplateMessages(messages)),
				Arguments: trace.VariableValsToSpanPromptVariables(variableVals),
			}))
			defer func() {
				span.SetOutput(ctx, json.Jsonify(tracespec.PromptOutput{
					Prompts: trace.MessagesToSpanMessages(formattedMessages),
				}))
				if err != nil {
					span.SetStatusCode(ctx, int(traceutil.GetTraceStatusCode(err)))
					span.SetError(ctx, errors.New(errorx.ErrorWithoutStack(err)))
				}
				span.Finish(ctx)
			}()
		}
	}
	return prompt.FormatMessages(messages, variableVals)
}
