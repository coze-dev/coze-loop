// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

// Jinja2TemplateService Jinja2模板服务
type Jinja2TemplateService struct{}

// NewJinja2TemplateService 创建新的Jinja2模板服务实例
func NewJinja2TemplateService() *Jinja2TemplateService {
	return &Jinja2TemplateService{}
}

// ValidateTemplate 验证Jinja2模板语法
func (s *Jinja2TemplateService) ValidateTemplate(ctx context.Context, req *openapi.ValidateTemplateRequest) (*openapi.ValidateTemplateResponse, error) {
	// 验证模板类型
	if req.TemplateType != "jinja2" {
		return &openapi.ValidateTemplateResponse{
			Code: 400,
			Msg:  "Only jinja2 template type is supported for validation",
			Data: &openapi.ValidateTemplateData{
				IsValid:     false,
				ErrorMessage: "Unsupported template type",
			},
		}, nil
	}

	// 创建Jinja2引擎进行语法验证
	engine := entity.NewJinja2Engine()

	// 使用空变量进行语法验证
	_, err := engine.Execute(req.Template, map[string]interface{}{})

	response := &openapi.ValidateTemplateResponse{
		Code: 200,
		Msg:  "Success",
		Data: &openapi.ValidateTemplateData{
			IsValid: err == nil,
		},
	}

	if err != nil {
		response.Data.ErrorMessage = err.Error()
	}

	return response, nil
}

// PreviewTemplate 预览Jinja2模板渲染结果
func (s *Jinja2TemplateService) PreviewTemplate(ctx context.Context, req *openapi.PreviewTemplateRequest) (*openapi.PreviewTemplateResponse, error) {
	// 验证模板类型
	if req.TemplateType != "jinja2" {
		return &openapi.PreviewTemplateResponse{
			Code: 400,
			Msg:  "Only jinja2 template type is supported for preview",
			Data: &openapi.PreviewTemplateData{
				Result: "Error: Unsupported template type",
			},
		}, nil
	}

	// 创建Jinja2引擎
	engine := entity.NewJinja2Engine()

	// 构建变量映射
	variables := make(map[string]interface{})
	for key, value := range req.Variables {
		variables[key] = value
	}

	// 执行模板渲染
	result, err := engine.Execute(req.Template, variables)
	if err != nil {
		return &openapi.PreviewTemplateResponse{
			Code: 400,
			Msg:  "Template execution failed",
			Data: &openapi.PreviewTemplateData{
				Result: "Error: " + err.Error(),
			},
		}, nil
	}

	return &openapi.PreviewTemplateResponse{
		Code: 200,
		Msg:  "Success",
		Data: &openapi.PreviewTemplateData{
			Result: result,
		},
	}, nil
}
