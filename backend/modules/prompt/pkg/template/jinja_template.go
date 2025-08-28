// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/nodes"
	"github.com/nikolalohinski/gonja/v2/parser"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func init() {
	// 安全初始化 gonja v2，禁用危险的控制结构
	nilParser := func(p *parser.Parser, args *parser.Parser) (nodes.ControlStructure, error) {
		return nil, fmt.Errorf("invalid statement")
	}
	_ = gonja.DefaultEnvironment.ControlStructures.Replace("include", nilParser)
	_ = gonja.DefaultEnvironment.ControlStructures.Replace("extends", nilParser)
	_ = gonja.DefaultEnvironment.ControlStructures.Replace("import", nilParser)
	_ = gonja.DefaultEnvironment.ControlStructures.Replace("from", nilParser)
}

func InterpolateJinja2(templateStr string, variables map[string]any) (string, error) {
	// 解析模板
	tpl, err := gonja.FromString(templateStr)
	if err != nil {
		return "", errorx.NewByCode(prompterr.TemplateParseErrorCode, errorx.WithExtraMsg(err.Error()))
	}

	// 创建执行上下文
	data := exec.NewContext(variables)
	var out bytes.Buffer

	// 执行模板渲染
	err = tpl.Execute(&out, data)
	if err != nil {
		return "", errorx.NewByCode(prompterr.TemplateRenderErrorCode, errorx.WithExtraMsg(err.Error()))
	}

	return out.String(), nil
}
