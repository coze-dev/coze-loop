// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type WebhookGlobalConf struct {
	Enable                  bool              `json:"enable" mapstructure:"enable"`
	Secret                  string            `json:"secret" mapstructure:"secret"`
	SpaceSecrets            map[string]string `json:"space_secrets" mapstructure:"space_secrets"`
	ResultURLTemplate       string            `json:"result_url_template" mapstructure:"result_url_template"`
	BitsCallbackURLTemplate string            `json:"bits_callback_url_template" mapstructure:"bits_callback_url_template"`
	DisabledSpaces          []int64           `json:"disabled_spaces" mapstructure:"disabled_spaces"`
}

func DefaultWebhookGlobalConf() *WebhookGlobalConf {
	return &WebhookGlobalConf{Enable: false}
}

func (c *WebhookGlobalConf) IsEnabled(spaceID int64) bool {
	if c == nil || !c.Enable {
		return false
	}
	for _, disabledSpaceID := range c.DisabledSpaces {
		if disabledSpaceID == spaceID {
			return false
		}
	}
	return true
}

func (c *WebhookGlobalConf) GetSigningSecret(spaceID int64) string {
	if c == nil {
		return ""
	}
	if c.SpaceSecrets != nil {
		keys := []string{
			strconv.FormatInt(spaceID, 10),
			fmt.Sprintf("space_%d", spaceID),
			"default",
		}
		for _, key := range keys {
			if secret := strings.TrimSpace(c.SpaceSecrets[key]); secret != "" {
				return secret
			}
		}
	}
	return strings.TrimSpace(c.Secret)
}

func (c *WebhookGlobalConf) BuildResultURL(spaceID, exptID int64) *string {
	if c == nil || strings.TrimSpace(c.ResultURLTemplate) == "" {
		return nil
	}
	result := strings.TrimSpace(c.ResultURLTemplate)
	result = strings.ReplaceAll(result, "{space_id}", strconv.FormatInt(spaceID, 10))
	result = strings.ReplaceAll(result, "{workspace_id}", strconv.FormatInt(spaceID, 10))
	result = strings.ReplaceAll(result, "{experiment_id}", strconv.FormatInt(exptID, 10))
	result = strings.ReplaceAll(result, "{expt_id}", strconv.FormatInt(exptID, 10))
	return &result
}

func (c *WebhookGlobalConf) BuildBitsCallbackURL(spaceID, exptID int64, sourceID string) *string {
	if c == nil || strings.TrimSpace(c.BitsCallbackURLTemplate) == "" {
		return nil
	}
	result := strings.TrimSpace(c.BitsCallbackURLTemplate)
	result = strings.ReplaceAll(result, "{space_id}", strconv.FormatInt(spaceID, 10))
	result = strings.ReplaceAll(result, "{workspace_id}", strconv.FormatInt(spaceID, 10))
	result = strings.ReplaceAll(result, "{experiment_id}", strconv.FormatInt(exptID, 10))
	result = strings.ReplaceAll(result, "{expt_id}", strconv.FormatInt(exptID, 10))
	result = strings.ReplaceAll(result, "{workflow_id}", sourceID)
	result = strings.ReplaceAll(result, "{source_id}", sourceID)
	return &result
}

type WebhookRetryConf struct {
	MaxRetries              int             `json:"max_retries" mapstructure:"max_retries"`
	RetryDelays             []time.Duration `json:"retry_delays" mapstructure:"retry_delays"`
	HTTPTimeout             time.Duration   `json:"http_timeout" mapstructure:"http_timeout"`
	NonRetryableStatusCodes []int           `json:"non_retryable_status_codes" mapstructure:"non_retryable_status_codes"`
	MessageTTL              time.Duration   `json:"message_ttl" mapstructure:"message_ttl"`
}

func DefaultWebhookRetryConf() *WebhookRetryConf {
	return &WebhookRetryConf{
		MaxRetries:              3,
		RetryDelays:             []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute},
		HTTPTimeout:             5 * time.Second,
		NonRetryableStatusCodes: []int{400, 401, 403, 404, 405, 410, 422},
		MessageTTL:              2 * time.Hour,
	}
}

type WebhookRateLimitConf struct {
	PerURLQPS           int `json:"per_url_qps" mapstructure:"per_url_qps"`
	GlobalConcurrency   int `json:"global_concurrency" mapstructure:"global_concurrency"`
	PerSpaceConcurrency int `json:"per_space_concurrency" mapstructure:"per_space_concurrency"`
}

func DefaultWebhookRateLimitConf() *WebhookRateLimitConf {
	return &WebhookRateLimitConf{
		PerURLQPS:         10,
		GlobalConcurrency: 50,
	}
}

type WebhookURLLimitConf struct {
	MaxURLsPerExperiment int `json:"max_urls_per_experiment" mapstructure:"max_urls_per_experiment"`
	MaxURLLength         int `json:"max_url_length" mapstructure:"max_url_length"`
}

func DefaultWebhookURLLimitConf() *WebhookURLLimitConf {
	return &WebhookURLLimitConf{
		MaxURLsPerExperiment: 10,
		MaxURLLength:         2048,
	}
}

type WebhookSecurityConf struct {
	BlockedCIDRs            []string `json:"blocked_cidrs" mapstructure:"blocked_cidrs"`
	AllowInternalForSources []string `json:"allow_internal_for_sources" mapstructure:"allow_internal_for_sources"`
	BlockedHosts            []string `json:"blocked_hosts" mapstructure:"blocked_hosts"`
}

func DefaultWebhookSecurityConf() *WebhookSecurityConf {
	return &WebhookSecurityConf{
		BlockedCIDRs: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"127.0.0.0/8",
			"169.254.0.0/16",
			"::1/128",
			"fe80::/10",
			"fc00::/7",
		},
		BlockedHosts: []string{
			"metadata.google.internal",
			"metadata.aws.internal",
		},
		AllowInternalForSources: []string{"bits_callback"},
	}
}
