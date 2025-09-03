// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"sync"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/js_conv"
)

type IRuntimeParam interface {
	GetJSONDemo() string
	GetJSONValue() string
	ParseFromJSON(val string) (IRuntimeParam, error)
}

var (
	promptRuntimeParamDemoOnce sync.Once
	promptRuntimeParamDemo     string
)

func NewPromptRuntimeParam(modelConfig *ModelConfig) IRuntimeParam {
	return &PromptRuntimeParam{ModelConfig: modelConfig}
}

type PromptRuntimeParam struct {
	ModelConfig *ModelConfig `json:"model_config" jsonschema:"description:ModelConfig"`
}

func (p *PromptRuntimeParam) ParseFromJSON(val string) (IRuntimeParam, error) {
	ppp := &PromptRuntimeParam{}
	if err := js_conv.GetUnmarshaler()([]byte(val), ppp); err != nil {
		return nil, errorx.Wrapf(err, "PromptRuntimeParam json unmarshal fail")
	}
	return ppp, nil
}

func (p *PromptRuntimeParam) GetJSONDemo() string {
	promptRuntimeParamDemoOnce.Do(func() {
		bytes, _ := js_conv.GetMarshaler()(&PromptRuntimeParam{
			ModelConfig: &ModelConfig{
				MaxTokens:   gptr.Of(int32(0)),
				Temperature: gptr.Of(float64(0)),
				TopP:        gptr.Of(float64(0)),
				JSONExt:     gptr.Of("{}"),
			},
		})
		promptRuntimeParamDemo = string(bytes)
	})
	return promptRuntimeParamDemo
}

func (p *PromptRuntimeParam) GetJSONValue() string {
	bytes, _ := js_conv.GetMarshaler()(p)
	return conv.UnsafeBytesToString(bytes)
}

func NewDummyRuntimeParam() *DummyRuntimeParam {
	return &DummyRuntimeParam{}
}

type DummyRuntimeParam struct{}

func (d *DummyRuntimeParam) ParseFromJSON(val string) (IRuntimeParam, error) {
	return &DummyRuntimeParam{}, nil
}

func (d *DummyRuntimeParam) GetJSONDemo() string {
	return "{}"
}

func (d *DummyRuntimeParam) GetJSONValue() string {
	return "{}"
}
