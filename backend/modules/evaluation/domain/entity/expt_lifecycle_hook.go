// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type HookAccessProtocol string
type HookEnvironment string
type HookFailurePolicy string

const (
	HookAccessProtocolHTTP    HookAccessProtocol = "http"
	HookAccessProtocolRPC     HookAccessProtocol = "rpc"
	HookEnvironmentProd       HookEnvironment    = "Prod"
	HookEnvironmentPPE        HookEnvironment    = "PPE"
	HookEnvironmentBOE        HookEnvironment    = "BOE"
	HookFailurePolicyBlock    HookFailurePolicy  = "block"
	HookFailurePolicyContinue HookFailurePolicy  = "continue"
)

type HookHTTPInfo struct {
	Method *string `json:"method,omitempty"`
	URL    *string `json:"url,omitempty"`
}

type HookRetryConf struct {
	Enabled    *bool  `json:"enabled,omitempty"`
	MaxRetries *int32 `json:"max_retries,omitempty"`
}

type HookConfig struct {
	Enabled        *bool               `json:"enabled,omitempty"`
	AccessProtocol *HookAccessProtocol `json:"access_protocol,omitempty"`
	InvokeHTTPInfo *HookHTTPInfo       `json:"invoke_http_info,omitempty"`
	Environment    *HookEnvironment    `json:"environment,omitempty"`
	Lane           *string             `json:"lane,omitempty"`
	ParametersJSON *string             `json:"parameters_json,omitempty"`
	TimeoutSeconds *int32              `json:"timeout_seconds,omitempty"`
	Retry          *HookRetryConf      `json:"retry,omitempty"`
	OnFailure      *HookFailurePolicy  `json:"on_failure,omitempty"`
}

type LifecycleHookConf struct {
	Before *HookConfig `json:"before,omitempty"`
	After  *HookConfig `json:"after,omitempty"`
}

// EffectiveMaxRetries counts extra deliveries, not the initial attempt.
func (r *HookRetryConf) EffectiveMaxRetries() int32 {
	if r != nil {
		if r.Enabled != nil && !*r.Enabled {
			return 0
		}
		if r.MaxRetries != nil {
			return *r.MaxRetries
		}
	}
	return 1
}
