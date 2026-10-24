// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"bytes"
	"encoding/json"

	jsoniter "github.com/json-iterator/go"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/js_conv"
)

var nativePromptJSON = func() jsoniter.API {
	decoder := jsoniter.Config{DisallowUnknownFields: true}.Froze()
	decoder.RegisterExtension(js_conv.NewJSONIterExtension())
	return decoder
}()

func promptModelConfigFromNative(raw string) (*prompt.ModelConfig, error) {
	var envelope struct {
		Protocol    string          `json:"protocol"`
		ModelConfig json.RawMessage `json:"model_config"`
	}
	if err := nativePromptJSON.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid prompt_model_config envelope: "+err.Error()))
	}
	if envelope.Protocol != "oss_prompt" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("unsupported native protocol for OSS Prompt: "+envelope.Protocol))
	}
	config := bytes.TrimSpace(envelope.ModelConfig)
	if len(config) == 0 || config[0] != '{' {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("native model_config must be an object"))
	}
	modelConfig := new(prompt.ModelConfig)
	if err := nativePromptJSON.Unmarshal(config, modelConfig); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid OSS Prompt model_config: "+err.Error()))
	}
	return modelConfig, nil
}
