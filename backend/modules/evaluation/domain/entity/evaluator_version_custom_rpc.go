// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// EvaluatorAccessProtocol 评估器接入协议类型
type EvaluatorAccessProtocol = string

const (
	EvaluatorAccessProtocolRPC         EvaluatorAccessProtocol = "rpc"
	EvaluatorAccessProtocolRPCOld      EvaluatorAccessProtocol = "rpc_old"
	EvaluatorAccessProtocolFaasHTTP    EvaluatorAccessProtocol = "faas_http"
	EvaluatorAccessProtocolFaasHTTPOld EvaluatorAccessProtocol = "faas_http_old"
)

type EvaluatorHTTPMethod = string

const (
	EvaluatorHTTPMethodGet  EvaluatorHTTPMethod = "get"
	EvaluatorHTTPMethodPost EvaluatorHTTPMethod = "post"
)

type EvaluatorHTTPInfo struct {
	Method *EvaluatorHTTPMethod `json:"method,omitempty"`
	Path   *string              `json:"path,omitempty"`
}

type CustomRPCEvaluatorVersion struct {
	// standard EvaluatorVersion layer attributes
	ID            int64         `json:"id"`
	SpaceID       int64         `json:"space_id"`
	EvaluatorType EvaluatorType `json:"evaluator_type"`
	EvaluatorID   int64         `json:"evaluator_id"`
	Description   string        `json:"description"`
	Version       string        `json:"version"`
	BaseInfo      *BaseInfo     `json:"base_info"`

	// standard EvaluatorContent layer attributes
	InputSchemas  []*ArgsSchema `json:"input_schemas"`
	OutputSchemas []*ArgsSchema `json:"output_schemas"`

	// specific CustomRPCEvaluator layer attributes, refer to CustomRPCEvaluator DTO
	ProviderEvaluatorCode *string                 `json:"provider_evaluator_code"` // provider's evaluator identity code, e.g. provider A may name an evaluator as A001
	AccessProtocol        EvaluatorAccessProtocol `json:"access_protocol"`         // custom protocol
	ServiceName           *string                 `json:"service_name"`
	Cluster               *string                 `json:"cluster"`
	InvokeHTTPInfo        *EvaluatorHTTPInfo      `json:"invoke_http_info,omitempty"` // invoke http info
	Timeout               *int64                  `json:"timeout"`                    // timeout duration in milliseconds(ms)
	RateLimit             *RateLimit              `json:"rate_limit,omitempty"`
}

func (do *CustomRPCEvaluatorVersion) SetID(id int64) {
	do.ID = id
}

func (do *CustomRPCEvaluatorVersion) GetID() int64 {
	return do.ID
}

func (do *CustomRPCEvaluatorVersion) SetEvaluatorID(evaluatorID int64) {
	do.EvaluatorID = evaluatorID
}

func (do *CustomRPCEvaluatorVersion) GetEvaluatorID() int64 {
	return do.EvaluatorID
}

func (do *CustomRPCEvaluatorVersion) SetSpaceID(spaceID int64) {
	do.SpaceID = spaceID
}

func (do *CustomRPCEvaluatorVersion) GetSpaceID() int64 {
	return do.SpaceID
}

func (do *CustomRPCEvaluatorVersion) GetVersion() string {
	return do.Version
}

func (do *CustomRPCEvaluatorVersion) SetVersion(version string) {
	do.Version = version
}

func (do *CustomRPCEvaluatorVersion) SetDescription(description string) {
	do.Description = description
}

func (do *CustomRPCEvaluatorVersion) GetDescription() string {
	return do.Description
}

func (do *CustomRPCEvaluatorVersion) SetBaseInfo(baseInfo *BaseInfo) {
	do.BaseInfo = baseInfo
}

func (do *CustomRPCEvaluatorVersion) GetBaseInfo() *BaseInfo {
	return do.BaseInfo
}

func (do *CustomRPCEvaluatorVersion) ValidateInput(input *EvaluatorInputData) error {
	inputSchemaMap := make(map[string]*ArgsSchema)
	for _, argsSchema := range do.InputSchemas {
		inputSchemaMap[gptr.Indirect(argsSchema.Key)] = argsSchema
	}
	for fieldKey, content := range input.InputFields {
		if content == nil {
			continue
		}
		// no need to validate schema for fields not defined in input schemas
		if argsSchema, ok := inputSchemaMap[fieldKey]; ok {
			if !gslice.Contains(argsSchema.SupportContentTypes, gptr.Indirect(content.ContentType)) {
				return errorx.NewByCode(errno.ContentTypeNotSupportedCode, errorx.WithExtraMsg(fmt.Sprintf("content type %v not supported", gptr.Indirect(content.ContentType))))
			}
			if gptr.Indirect(content.ContentType) == ContentTypeText {
				valid, err := json.ValidateJSONSchema(gptr.Indirect(argsSchema.JsonSchema), gptr.Indirect(content.Text))
				if err != nil || !valid {
					return errorx.NewByCode(errno.ContentSchemaInvalidCode, errorx.WithExtraMsg(fmt.Sprintf("content %v does not validate with expected schema: %v", gptr.Indirect(content.Text), gptr.Indirect(argsSchema.JsonSchema))))
				}
			}
		}
	}
	return nil
}

func (do *CustomRPCEvaluatorVersion) ValidateBaseInfo() error {
	if do == nil {
		return errorx.NewByCode(errno.EvaluatorNotExistCode, errorx.WithExtraMsg("evaluator_version is nil"))
	}
	if do.ProviderEvaluatorCode == nil || lo.IsEmpty(*do.ProviderEvaluatorCode) {
		return errorx.NewByCode(errno.InvalidProviderEvaluatorCodeCode, errorx.WithExtraMsg("provider_evaluator_code is empty"))
	}
	if lo.IsEmpty(do.AccessProtocol) {
		return errorx.NewByCode(errno.InvalidAccessProtocolCode, errorx.WithExtraMsg("access_protocol is empty"))
	}
	if do.ServiceName == nil || lo.IsEmpty(*do.ServiceName) {
		return errorx.NewByCode(errno.InvalidServiceNameCode, errorx.WithExtraMsg("service_name is empty"))
	}

	return nil
}
