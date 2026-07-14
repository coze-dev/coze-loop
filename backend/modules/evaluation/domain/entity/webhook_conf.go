// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// Webhook configuration structs (T1.2). Commercial reads them via IConfiger.
// Default* factories are the single source of truth for fallback values and
// are consumed by commercial `exptConfiger.Get*` when the config key is
// missing / invalid.

// WebhookGlobalConf gates the whole webhook subsystem.
type WebhookGlobalConf struct {
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	DryRun  bool `mapstructure:"dry_run" json:"dry_run"`
	// DisableConsumer skips subscribing the webhook_delivery consumer at boot.
	// Ops flips this on when the RocketMQ topic has not been provisioned yet
	// so the pod passes readiness while retries are held. Defaults to false.
	DisableConsumer bool `mapstructure:"disable_consumer" json:"disable_consumer"`
}

func DefaultWebhookGlobalConf() *WebhookGlobalConf {
	return &WebhookGlobalConf{Enabled: true, DryRun: false, DisableConsumer: false}
}

// WebhookRetryConf controls send timeout and retry backoff cadence.
type WebhookRetryConf struct {
	BackoffSec       []int `mapstructure:"backoff_sec" json:"backoff_sec"`
	MaxAttempts      int   `mapstructure:"max_attempts" json:"max_attempts"`
	RequestTimeoutMS int   `mapstructure:"request_timeout_ms" json:"request_timeout_ms"`
}

func DefaultWebhookRetryConf() *WebhookRetryConf {
	return &WebhookRetryConf{
		BackoffSec:       []int{60, 300, 1800},
		MaxAttempts:      4,
		RequestTimeoutMS: 5000,
	}
}

// WebhookRateLimitConf caps per-space delivery burst.
type WebhookRateLimitConf struct {
	PerSpacePerMin int `mapstructure:"per_space_per_min" json:"per_space_per_min"`
}

func DefaultWebhookRateLimitConf() *WebhookRateLimitConf {
	return &WebhookRateLimitConf{PerSpacePerMin: 300}
}

// WebhookURLLimitConf enforces URL count + scheme whitelist at IDL validation.
type WebhookURLLimitConf struct {
	MaxURLsPerExperiment int      `mapstructure:"max_urls_per_experiment" json:"max_urls_per_experiment"`
	URLSchemeAllowlist   []string `mapstructure:"url_scheme_allowlist" json:"url_scheme_allowlist"`
}

func DefaultWebhookURLLimitConf() *WebhookURLLimitConf {
	return &WebhookURLLimitConf{
		MaxURLsPerExperiment: 10,
		URLSchemeAllowlist:   []string{"https"},
	}
}

// WebhookSecurityConf pins signature header names + algorithm.
type WebhookSecurityConf struct {
	SignatureHeader  string `mapstructure:"signature_header" json:"signature_header"`
	TimestampHeader  string `mapstructure:"timestamp_header" json:"timestamp_header"`
	DeliveryIDHeader string `mapstructure:"delivery_id_header" json:"delivery_id_header"`
	Algorithm        string `mapstructure:"algorithm" json:"algorithm"`
}

func DefaultWebhookSecurityConf() *WebhookSecurityConf {
	return &WebhookSecurityConf{
		SignatureHeader:  "X-Fornax-Signature",
		TimestampHeader:  "X-Fornax-Timestamp",
		DeliveryIDHeader: "X-Fornax-Delivery-Id",
		Algorithm:        "sha256",
	}
}
