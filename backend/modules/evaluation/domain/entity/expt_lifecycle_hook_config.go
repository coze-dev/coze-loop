// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ResolveLifecycleHookConf replaces explicit stages and inherits absent stages.
// The result owns all its pointers; neither input is changed, including on error.
func ResolveLifecycleHookConf(base, override *LifecycleHookConf) (*LifecycleHookConf, error) {
	if base == nil && override == nil {
		return nil, nil
	}
	selected := LifecycleHookConf{}
	if base != nil {
		selected = *base
	}
	if override != nil {
		if override.Before != nil {
			selected.Before = override.Before
		}
		if override.After != nil {
			selected.After = override.After
		}
	}
	before, err := normalizeHookConfig(selected.Before, true)
	if err != nil {
		return nil, invalidParam("lifecycle_hook_conf.before: " + err.Error())
	}
	after, err := normalizeHookConfig(selected.After, false)
	if err != nil {
		return nil, invalidParam("lifecycle_hook_conf.after: " + err.Error())
	}
	return &LifecycleHookConf{Before: before, After: after}, nil
}

func normalizeHookConfig(input *HookConfig, before bool) (*HookConfig, error) {
	if input == nil {
		return nil, nil
	}
	c := &HookConfig{
		Enabled:        hookValueOrDefault(input.Enabled, false),
		AccessProtocol: hookValueOrDefault(input.AccessProtocol, HookAccessProtocolHTTP),
		Environment:    hookValueOrDefault(input.Environment, HookEnvironmentProd),
		Lane:           copyHookValue(input.Lane), ParametersJSON: hookValueOrDefault(input.ParametersJSON, "{}"),
		TimeoutSeconds: hookValueOrDefault(input.TimeoutSeconds, int32(180)),
		OnFailure:      copyHookValue(input.OnFailure),
	}
	if *c.AccessProtocol != HookAccessProtocolHTTP {
		return nil, errors.New("access_protocol must be http; rpc is reserved")
	}
	switch *c.Environment {
	case HookEnvironmentProd, HookEnvironmentPPE, HookEnvironmentBOE:
	default:
		return nil, errors.New("environment must be Prod, PPE or BOE")
	}
	if c.Lane != nil && !utf8.ValidString(*c.Lane) {
		return nil, errors.New("lane must be UTF-8")
	}
	if *c.Environment == HookEnvironmentProd {
		if c.Lane != nil && *c.Lane != "" {
			return nil, errors.New("Prod must not specify lane")
		}
	} else if *c.Enabled && (c.Lane == nil || strings.TrimSpace(*c.Lane) == "") {
		return nil, errors.New("enabled PPE/BOE hook requires lane")
	}
	httpInfo := HookHTTPInfo{}
	if input.InvokeHTTPInfo != nil {
		httpInfo = *input.InvokeHTTPInfo
	}
	c.InvokeHTTPInfo = &HookHTTPInfo{Method: hookValueOrDefault(httpInfo.Method, "post"), URL: copyHookValue(httpInfo.URL)}
	if *c.InvokeHTTPInfo.Method != "post" {
		return nil, errors.New("invoke_http_info.method must be post")
	}
	if httpInfo.URL == nil || *httpInfo.URL == "" {
		if *c.Enabled {
			return nil, errors.New("enabled hook requires invoke_http_info.url")
		}
	} else if err := validateHookURL(*httpInfo.URL); err != nil {
		return nil, err
	}
	if *c.TimeoutSeconds < 0 || *c.TimeoutSeconds > 1200 {
		return nil, errors.New("timeout_seconds must be between 0 and 1200")
	}
	if *c.TimeoutSeconds == 0 {
		*c.TimeoutSeconds = 180
	}
	retry := HookRetryConf{}
	if input.Retry != nil {
		retry = *input.Retry
	}
	c.Retry = &HookRetryConf{Enabled: hookValueOrDefault(retry.Enabled, true), MaxRetries: hookValueOrDefault(retry.MaxRetries, int32(1))}
	if *c.Retry.MaxRetries < 1 || *c.Retry.MaxRetries > 10 {
		return nil, errors.New("retry.max_retries must be between 1 and 10")
	}
	if before {
		c.OnFailure = hookValueOrDefault(input.OnFailure, HookFailurePolicyBlock)
		if *c.OnFailure != HookFailurePolicyBlock && *c.OnFailure != HookFailurePolicyContinue {
			return nil, errors.New("on_failure must be block or continue")
		}
	} else if c.OnFailure != nil && *c.OnFailure != "" {
		return nil, errors.New("after does not accept on_failure")
	}
	parameters, err := normalizeHookParameters(*c.ParametersJSON)
	if err != nil {
		return nil, err
	}
	*c.ParametersJSON = parameters
	return c, nil
}

// Endpoint admission and DNS/IP safety are enforced by the transport, not here.
func validateHookURL(raw string) error {
	invalid := errors.New("invoke_http_info.url must be an HTTPS URL of at most 2048 bytes without userinfo or fragment")
	if len(raw) > 2048 || !utf8.ValidString(raw) || strings.Contains(raw, "#") {
		return invalid
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return invalid
	}
	if strings.HasSuffix(u.Host, ":") {
		return invalid
	}
	if strings.HasPrefix(u.Host, "[") {
		address, err := netip.ParseAddr(u.Hostname())
		if err != nil || !address.Is6() {
			return invalid
		}
	} else if strings.Contains(u.Hostname(), ":") {
		return invalid
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return invalid
		}
	}
	return nil
}

func copyHookValue[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func hookValueOrDefault[T any](value *T, fallback T) *T {
	if value == nil {
		return &fallback
	}
	return copyHookValue(value)
}
