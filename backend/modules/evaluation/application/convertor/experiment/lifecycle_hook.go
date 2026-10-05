// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func copyHookValue[T any](p *T) *T {
	if p == nil {
		return nil
	}
	return gptr.Of(*p)
}

// LifecycleHookConfDTO2DO preserves omission; ResolveLifecycleHookConf owns validation/defaults.
func LifecycleHookConfDTO2DO(c *domain.LifecycleHookConf) *entity.LifecycleHookConf {
	if c == nil {
		return nil
	}
	return &entity.LifecycleHookConf{Before: hookConfigDTO2DO(c.Before), After: hookConfigDTO2DO(c.After)}
}

func hookConfigDTO2DO(c *domain.HookConfig) *entity.HookConfig {
	if c == nil {
		return nil
	}
	r := &entity.HookConfig{Enabled: copyHookValue(c.Enabled), Lane: copyHookValue(c.Lane), ParametersJSON: copyHookValue(c.ParametersJSON), TimeoutSeconds: copyHookValue(c.TimeoutSeconds)}
	if c.AccessProtocol != nil {
		r.AccessProtocol = gptr.Of(entity.HookAccessProtocol(*c.AccessProtocol))
	}
	if c.Environment != nil {
		r.Environment = gptr.Of(entity.HookEnvironment(*c.Environment))
	}
	if c.OnFailure != nil {
		r.OnFailure = gptr.Of(entity.HookFailurePolicy(*c.OnFailure))
	}
	if c.InvokeHTTPInfo != nil {
		r.InvokeHTTPInfo = &entity.HookHTTPInfo{Method: copyHookValue(c.InvokeHTTPInfo.Method), URL: copyHookValue(c.InvokeHTTPInfo.URL)}
	}
	if c.Retry != nil {
		r.Retry = &entity.HookRetryConf{Enabled: copyHookValue(c.Retry.Enabled), MaxRetries: copyHookValue(c.Retry.MaxRetries)}
	}
	return r
}

func LifecycleHookConfDO2DTO(c *entity.LifecycleHookConf) *domain.LifecycleHookConf {
	if c == nil {
		return nil
	}
	return &domain.LifecycleHookConf{Before: hookConfigDO2DTO(c.Before), After: hookConfigDO2DTO(c.After)}
}

func hookConfigDO2DTO(c *entity.HookConfig) *domain.HookConfig {
	if c == nil {
		return nil
	}
	r := &domain.HookConfig{Enabled: copyHookValue(c.Enabled), Lane: copyHookValue(c.Lane), ParametersJSON: copyHookValue(c.ParametersJSON), TimeoutSeconds: copyHookValue(c.TimeoutSeconds)}
	if c.AccessProtocol != nil {
		r.AccessProtocol = gptr.Of(domain.HookAccessProtocol(*c.AccessProtocol))
	}
	if c.Environment != nil {
		r.Environment = gptr.Of(domain.HookEnvironment(*c.Environment))
	}
	if c.OnFailure != nil {
		r.OnFailure = gptr.Of(domain.HookFailurePolicy(*c.OnFailure))
	}
	if c.InvokeHTTPInfo != nil {
		r.InvokeHTTPInfo = &domain.HookHTTPInfo{Method: copyHookValue(c.InvokeHTTPInfo.Method), URL: copyHookValue(c.InvokeHTTPInfo.URL)}
	}
	if c.Retry != nil {
		r.Retry = &domain.HookRetryConf{Enabled: copyHookValue(c.Retry.Enabled), MaxRetries: copyHookValue(c.Retry.MaxRetries)}
	}
	return r
}

func OpenAPILifecycleHookConfDTO2Domain(c *openapi.LifecycleHookConf) *domain.LifecycleHookConf {
	if c == nil {
		return nil
	}
	return &domain.LifecycleHookConf{Before: openAPIHookConfig(c.Before), After: openAPIHookConfig(c.After)}
}

func openAPIHookConfig(c *openapi.HookConfig) *domain.HookConfig {
	if c == nil {
		return nil
	}
	r := &domain.HookConfig{Enabled: copyHookValue(c.Enabled), Lane: copyHookValue(c.Lane), ParametersJSON: copyHookValue(c.ParametersJSON), TimeoutSeconds: copyHookValue(c.TimeoutSeconds)}
	if c.AccessProtocol != nil {
		r.AccessProtocol = gptr.Of(domain.HookAccessProtocol(*c.AccessProtocol))
	}
	if c.Environment != nil {
		r.Environment = gptr.Of(domain.HookEnvironment(*c.Environment))
	}
	if c.OnFailure != nil {
		r.OnFailure = gptr.Of(domain.HookFailurePolicy(*c.OnFailure))
	}
	if c.InvokeHTTPInfo != nil {
		r.InvokeHTTPInfo = &domain.HookHTTPInfo{Method: copyHookValue(c.InvokeHTTPInfo.Method), URL: copyHookValue(c.InvokeHTTPInfo.URL)}
	}
	if c.Retry != nil {
		r.Retry = &domain.HookRetryConf{Enabled: copyHookValue(c.Retry.Enabled), MaxRetries: copyHookValue(c.Retry.MaxRetries)}
	}
	return r
}

func LifecycleHookConfDomain2OpenAPI(c *domain.LifecycleHookConf) *openapi.LifecycleHookConf {
	if c == nil {
		return nil
	}
	return &openapi.LifecycleHookConf{Before: domainHookConfigToOpenAPI(c.Before), After: domainHookConfigToOpenAPI(c.After)}
}

func domainHookConfigToOpenAPI(c *domain.HookConfig) *openapi.HookConfig {
	if c == nil {
		return nil
	}
	r := &openapi.HookConfig{Enabled: copyHookValue(c.Enabled), Lane: copyHookValue(c.Lane), ParametersJSON: copyHookValue(c.ParametersJSON), TimeoutSeconds: copyHookValue(c.TimeoutSeconds)}
	if c.AccessProtocol != nil {
		r.AccessProtocol = gptr.Of(openapi.HookAccessProtocol(*c.AccessProtocol))
	}
	if c.Environment != nil {
		r.Environment = gptr.Of(openapi.HookEnvironment(*c.Environment))
	}
	if c.OnFailure != nil {
		r.OnFailure = gptr.Of(openapi.HookFailurePolicy(*c.OnFailure))
	}
	if c.InvokeHTTPInfo != nil {
		r.InvokeHTTPInfo = &openapi.HookHTTPInfo{Method: copyHookValue(c.InvokeHTTPInfo.Method), URL: copyHookValue(c.InvokeHTTPInfo.URL)}
	}
	if c.Retry != nil {
		r.Retry = &openapi.HookRetryConf{Enabled: copyHookValue(c.Retry.Enabled), MaxRetries: copyHookValue(c.Retry.MaxRetries)}
	}
	return r
}

func LifecycleHookSummaryDO2DTO(s *entity.LifecycleHookRunSummary) *domain.LifecycleHookRunSummary {
	if s == nil {
		return nil
	}
	return &domain.LifecycleHookRunSummary{RunID: gptr.Of(strconv.FormatInt(s.RunID, 10)), Before: hookSummaryDO2DTO(s.Before), After: hookSummaryDO2DTO(s.After)}
}

func hookSummaryDO2DTO(s entity.HookRunSummary) *domain.HookRunSummary {
	r := &domain.HookRunSummary{Status: gptr.Of(domain.HookOperationStatus(s.Status)), OperationID: gptr.Of(s.OperationID), Attempt: gptr.Of(s.Attempt)}
	if s.UpdatedAt != nil {
		r.UpdatedAt = gptr.Of(s.UpdatedAt.UTC().Format(time.RFC3339Nano))
	}
	if s.Status == entity.HookOperationSucceeded {
		r.Response = copyHookResponse(s.Response)
	}
	if s.Status == entity.HookOperationFailed {
		r.Error = copyHookError(s.Error)
	}
	return r
}

func lifecycleHookSummaryDomain2OpenAPI(s *domain.LifecycleHookRunSummary) *openapi.LifecycleHookRunSummary {
	if s == nil {
		return nil
	}
	return &openapi.LifecycleHookRunSummary{RunID: copyHookValue(s.RunID), Before: domainHookSummaryToOpenAPI(s.Before), After: domainHookSummaryToOpenAPI(s.After)}
}

func domainHookSummaryToOpenAPI(s *domain.HookRunSummary) *openapi.HookRunSummary {
	if s == nil {
		return nil
	}
	r := &openapi.HookRunSummary{OperationID: copyHookValue(s.OperationID), Attempt: copyHookValue(s.Attempt), UpdatedAt: copyHookValue(s.UpdatedAt)}
	if s.Status != nil {
		r.Status = gptr.Of(openapi.HookOperationStatus(*s.Status))
	}
	if s.GetStatus() == "succeeded" {
		r.Response = copyHookResponse(s.Response)
	}
	if s.GetStatus() == "failed" {
		r.Error = copyHookError(s.Error)
	}
	return r
}

func copyHookResponse(s *spi.InvokeExperimentHookResponse) *spi.InvokeExperimentHookResponse {
	if s == nil {
		return nil
	}
	r := &spi.InvokeExperimentHookResponse{Status: copyHookValue(s.Status), Error: copyHookError(s.Error)}
	if s.Result_ != nil {
		r.Result_ = make(map[string]string, len(s.Result_))
		for k, v := range s.Result_ {
			r.Result_[k] = v
		}
	}
	return r
}

func copyHookError(s *spi.HookError) *spi.HookError {
	if s == nil {
		return nil
	}
	return &spi.HookError{Code: copyHookValue(s.Code), Message: copyHookValue(s.Message), Retryable: copyHookValue(s.Retryable)}
}
