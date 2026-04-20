// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"text/template"
	"time"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func InterpolateGoTemplate(templateStr string, variables map[string]any) (string, error) {
	tpl, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		return "", errorx.NewByCode(prompterr.TemplateParseErrorCode, errorx.WithExtraMsg(err.Error()))
	}

	var out bytes.Buffer
	lw := &LimitedWriter{W: &out, N: MaxTemplateOutputSize}

	type result struct {
		err error
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{err: tpl.Execute(lw, variables)}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return "", errorx.NewByCode(prompterr.TemplateRenderErrorCode, errorx.WithExtraMsg(r.err.Error()))
		}
	case <-time.After(MaxTemplateTimeout):
		return "", errorx.NewByCode(prompterr.TemplateRenderErrorCode,
			errorx.WithExtraMsg("template rendering timeout"))
	}

	return out.String(), nil
}
