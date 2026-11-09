// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const RetryRuntimeParamExtKey = "retry_runtime_param"

var retryBinaryVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var retryCUASHA256Pattern = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)

// Only artifact selection may change for a retry; model and task settings stay frozen.
func ParseRetryRuntimeOverride(ext map[string]string) (map[string]string, error) {
	raw, exists := ext[RetryRuntimeParamExtKey]
	if !exists {
		return nil, nil
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil || len(values) == 0 {
		return nil, fmt.Errorf("retry_runtime_param must be a non-empty string object")
	}
	for key, value := range values {
		if key != "binary_version" && key != "cua_package_url" && key != "cua_package_sha256" {
			return nil, fmt.Errorf("unsupported retry runtime field %q", key)
		}
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return nil, fmt.Errorf("retry runtime field %q must be non-empty without surrounding whitespace", key)
		}
	}
	if version, ok := values["binary_version"]; ok && (!retryBinaryVersionPattern.MatchString(version) || strings.Contains(version, "..")) {
		return nil, fmt.Errorf("invalid retry binary_version")
	}
	packageURL, hasURL := values["cua_package_url"]
	sha, hasSHA := values["cua_package_sha256"]
	if hasURL != hasSHA {
		return nil, fmt.Errorf("retry cua_package_url and cua_package_sha256 must be provided together")
	}
	if hasURL {
		parsed, err := url.Parse(packageURL)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
			return nil, fmt.Errorf("retry cua_package_url must be an absolute HTTP(S) URL without user info")
		}
		if !retryCUASHA256Pattern.MatchString(sha) {
			return nil, fmt.Errorf("retry cua_package_sha256 must contain 64 hexadecimal characters")
		}
	}
	return values, nil
}

func MergeRetryRuntimeOverride(runtimeParam string, eventExt map[string]string) (string, error) {
	override, err := ParseRetryRuntimeOverride(eventExt)
	if err != nil || len(override) == 0 {
		return runtimeParam, err
	}
	values := map[string]json.RawMessage{}
	if runtimeParam != "" {
		if err := json.Unmarshal([]byte(runtimeParam), &values); err != nil {
			return "", fmt.Errorf("original runtime_param must be an object for a version override")
		}
	}
	if values == nil {
		values = map[string]json.RawMessage{}
	}
	for key, value := range override {
		values[key], _ = json.Marshal(value)
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err
}
