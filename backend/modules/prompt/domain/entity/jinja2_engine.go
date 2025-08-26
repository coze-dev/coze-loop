// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
)

// Jinja2Engine Jinja2模板引擎
type Jinja2Engine struct {
	templateSet *pongo2.TemplateSet
	timeout     time.Duration
}

// NewJinja2Engine 创建新的Jinja2引擎实例
func NewJinja2Engine() *Jinja2Engine {
	templateSet := pongo2.NewSet("coze-loop", pongo2.MustNewLocalFileSystemLoader(""))

	// 注册安全过滤器
	pongo2.RegisterFilter("safe_upper", filterSafeUpper)
	pongo2.RegisterFilter("safe_lower", filterSafeLower)
	pongo2.RegisterFilter("truncate", filterTruncate)
	pongo2.RegisterFilter("default", filterDefault)

	// 注册高级过滤器
	pongo2.RegisterFilter("strip", filterStrip)
	pongo2.RegisterFilter("split", filterSplit)
	pongo2.RegisterFilter("join", filterJoin)
	pongo2.RegisterFilter("replace", filterReplace)
	pongo2.RegisterFilter("abs", filterAbs)
	pongo2.RegisterFilter("round", filterRound)
	pongo2.RegisterFilter("max", filterMax)
	pongo2.RegisterFilter("min", filterMin)
	pongo2.RegisterFilter("strftime", filterStrftime)
	pongo2.RegisterFilter("bool", filterBool)
	pongo2.RegisterFilter("not", filterNot)

	return &Jinja2Engine{
		templateSet: templateSet,
		timeout:     30 * time.Second,
	}
}

// Execute 执行Jinja2模板
func (j *Jinja2Engine) Execute(templateStr string, variables map[string]interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), j.timeout)
	defer cancel()

	// 创建模板
	template, err := j.templateSet.FromString(templateStr)
	if err != nil {
		return "", errorx.NewByCode(prompterr.CommonInvalidParamCode).WithMessage("invalid jinja2 template: " + err.Error())
	}

	// 创建安全上下文
	safeContext := j.createSafeContext(variables)

	// 执行模板
	result, err := template.ExecuteWithContext(ctx, safeContext)
	if err != nil {
		return "", errorx.NewByCode(prompterr.CommonInvalidParamCode).WithMessage("template execution failed: " + err.Error())
	}

	return result, nil
}

// createSafeContext 创建安全的模板执行上下文
func (j *Jinja2Engine) createSafeContext(variables map[string]interface{}) pongo2.Context {
	safeContext := pongo2.Context{}

	// 只允许安全的变量类型
	for key, value := range variables {
		if j.isSafeValue(value) {
			safeContext[key] = value
		}
	}

	// 添加内置函数
	safeContext["now"] = time.Now
	safeContext["len"] = func(v interface{}) int {
		if s, ok := v.(string); ok {
			return len(s)
		}
		if arr, ok := v.([]interface{}); ok {
			return len(arr)
		}
		if arr, ok := v.([]string); ok {
			return len(arr)
		}
		return 0
	}

	return safeContext
}

// isSafeValue 检查值是否安全
func (j *Jinja2Engine) isSafeValue(value interface{}) bool {
	switch value.(type) {
	case string, int, int32, int64, float32, float64, bool:
		return true
	case []string, []int, []interface{}:
		return true
	case map[string]interface{}, map[string]string:
		return true
	case time.Time:
		return true
	default:
		return false
	}
}

// 基础安全过滤器实现
func filterSafeUpper(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.ToUpper(in.String())), nil
}

func filterSafeLower(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.ToLower(in.String())), nil
}

func filterTruncate(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	length := param.Integer()
	str := in.String()
	if len(str) > length {
		return pongo2.AsValue(str[:length] + "..."), nil
	}
	return pongo2.AsValue(str), nil
}

func filterDefault(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if in.IsNil() || in.String() == "" {
		return param, nil
	}
	return in, nil
}

// 高级过滤器实现
func filterStrip(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.TrimSpace(in.String())), nil
}

func filterSplit(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	delimiter := param.String()
	if delimiter == "" {
		delimiter = " "
	}
	return pongo2.AsValue(strings.Split(in.String(), delimiter)), nil
}

func filterJoin(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	delimiter := param.String()
	if slice, ok := in.Interface().([]string); ok {
		return pongo2.AsValue(strings.Join(slice, delimiter)), nil
	}
	return in, nil
}

func filterReplace(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	// 期望参数格式: "old,new"
	parts := strings.Split(param.String(), ",")
	if len(parts) != 2 {
		return in, nil
	}
	return pongo2.AsValue(strings.ReplaceAll(in.String(), parts[0], parts[1])), nil
}

func filterAbs(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(math.Abs(in.Float())), nil
}

func filterRound(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	precision := param.Integer()
	multiplier := math.Pow(10, float64(precision))
	return pongo2.AsValue(math.Round(in.Float()*multiplier)/multiplier), nil
}

func filterMax(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(math.Max(in.Float(), param.Float())), nil
}

func filterMin(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(math.Min(in.Float(), param.Float())), nil
}

func filterStrftime(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	format := param.String()
	if t, ok := in.Interface().(time.Time); ok {
		return pongo2.AsValue(t.Format(format)), nil
	}
	return in, nil
}

func filterBool(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(in.Bool()), nil
}

func filterNot(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(!in.Bool()), nil
}
