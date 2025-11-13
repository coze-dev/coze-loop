// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"text/template"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func InterpolateGoTemplate(templateStr string, variables map[string]any) (string, error) {
	// 解析模板
	tpl, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		return "", errorx.NewByCode(prompterr.TemplateParseErrorCode, errorx.WithExtraMsg(err.Error()))
	}

	// 执行模板渲染
	var out bytes.Buffer
	err = tpl.Execute(&out, variables)
	if err != nil {
		return "", errorx.NewByCode(prompterr.TemplateRenderErrorCode, errorx.WithExtraMsg(err.Error()))
	}

	return out.String(), nil
}
