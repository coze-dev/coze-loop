// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"time"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/nodes"
	"github.com/nikolalohinski/gonja/v2/parser"
	"github.com/pkg/errors"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func init() {
	nilParser := func(p *parser.Parser, args *parser.Parser) (nodes.ControlStructure, error) {
		return nil, fmt.Errorf("invalid statement")
	}
	_ = gonja.DefaultEnvironment.ControlStructures.Replace("include", nilParser)
	_ = gonja.DefaultEnvironment.ControlStructures.Replace("extends", nilParser)
	_ = gonja.DefaultEnvironment.ControlStructures.Replace("import", nilParser)
	_ = gonja.DefaultEnvironment.ControlStructures.Replace("from", nilParser)

	gonja.DefaultEnvironment.Context.Set("range", safeRangeFunction)
}

func safeRangeFunction(_ *exec.Evaluator, params *exec.VarArgs) ([]int, error) {
	var (
		start = 0
		stop  = -1
		step  = 1
	)
	switch n := len(params.Args); n > 0 {
	case n == 1 && params.Args[0].IsInteger():
		stop = params.Args[0].Integer()
	case n == 2 && params.Args[0].IsInteger() && params.Args[1].IsInteger():
		start = params.Args[0].Integer()
		stop = params.Args[1].Integer()
	case n == 3 && params.Args[0].IsInteger() && params.Args[1].IsInteger() && params.Args[2].IsInteger():
		start = params.Args[0].Integer()
		stop = params.Args[1].Integer()
		step = params.Args[2].Integer()
	default:
		return nil, exec.ErrInvalidCall(errors.New("expected signature is [start, ]stop[, step] where all arguments are integers"))
	}

	if step == 0 {
		return nil, exec.ErrInvalidCall(errors.New("step must not be zero"))
	}

	count := 0
	if step > 0 && stop > start {
		count = (stop - start + step - 1) / step
	} else if step < 0 && start > stop {
		count = (start - stop - step - 1) / (-step)
	}

	if count > MaxRangeSize {
		return nil, exec.ErrInvalidCall(fmt.Errorf("range size %d exceeds maximum allowed %d", count, MaxRangeSize))
	}

	result := make([]int, 0, count)
	for i := start; (step > 0 && i < stop) || (step < 0 && i > stop); i += step {
		result = append(result, i)
	}
	return result, nil
}

func InterpolateJinja2(templateStr string, variables map[string]any) (string, error) {
	tpl, err := gonja.FromString(templateStr)
	if err != nil {
		return "", errorx.NewByCode(prompterr.TemplateParseErrorCode, errorx.WithExtraMsg(err.Error()))
	}

	data := exec.NewContext(variables)
	var out bytes.Buffer
	lw := &LimitedWriter{W: &out, N: MaxTemplateOutputSize}

	type result struct {
		err error
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{err: tpl.Execute(lw, data)}
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
