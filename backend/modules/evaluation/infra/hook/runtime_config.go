// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/netip"
	"reflect"
	"strconv"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

type RuntimeConfigProvider struct {
	loader          conf.IConfigLoader
	workerInstalled bool
}

type runtimeConfigInput struct {
	StorageKeyID                string                  `json:"storage_key_id"`
	AdmissionEnabled            *bool                   `json:"admission_enabled"`
	WorkspaceAllowlist          json.RawMessage         `json:"workspace_allowlist"`
	WorkerEnabled               *bool                   `json:"worker_enabled"`
	MQWakeEnabled               *bool                   `json:"mq_wake_enabled"`
	WorkerConcurrency           *int32                  `json:"worker_concurrency"`
	WorkspaceConcurrency        *int32                  `json:"workspace_concurrency"`
	ScanIntervalSeconds         *int32                  `json:"scan_interval_seconds"`
	ScanBatchSize               *int32                  `json:"scan_batch_size"`
	LeaseSeconds                *int32                  `json:"lease_seconds"`
	RenewSeconds                *int32                  `json:"renew_seconds"`
	IdentityEnrichmentTimeoutMS *int32                  `json:"identity_enrichment_timeout_ms"`
	RetentionDays               *int32                  `json:"retention_days"`
	EndpointPolicy              []*runtimeEndpoint      `json:"endpoint_policy"`
	SigningKeyRefs              []*runtimeSigningKeyRef `json:"signing_key_refs"`
}

type runtimeEndpointBinding struct {
	WorkspaceID runtimeWorkspaceID `json:"workspace_id"`
	Host        string             `json:"host"`
	Port        uint16             `json:"port"`
	Environment string             `json:"environment"`
	Lane        string             `json:"lane,omitempty"`
}

type runtimeEndpoint struct {
	runtimeEndpointBinding
	PrivateCIDRs []string `json:"private_cidrs"`
}

type runtimeSigningKeyRef struct {
	runtimeEndpointBinding
	KeyID  string `json:"key_id"`
	KeyRef string `json:"key_ref"`
}

type runtimeConfigSnapshot struct {
	storageKeyID string
	runtime      entity.HookRuntimeConfig
	policies     []EndpointPolicy
	keys         map[runtimeEndpointBinding]runtimeSigningKeyRef
}

var errHookRuntimeConfig = errors.New("hook runtime configuration unavailable")

var _ hookcomponent.RuntimeConfigProvider = (*RuntimeConfigProvider)(nil)

func NewRuntimeConfigProvider(loader conf.IConfigLoader, workerInstalled bool) *RuntimeConfigProvider {
	if identityNilProvider(loader) {
		loader = nil
	}
	return &RuntimeConfigProvider{loader: loader, workerInstalled: workerInstalled}
}

func (p *RuntimeConfigProvider) GetRuntimeConfig(ctx context.Context) (entity.HookRuntimeConfig, error) {
	snapshot, err := p.read(ctx)
	if err != nil {
		return entity.HookRuntimeConfig{}, err
	}
	return snapshot.runtime, nil
}

// The current write key is independent of the key in an existing envelope.
func (p *RuntimeConfigProvider) StorageKeyID(ctx context.Context) (string, error) {
	snapshot, err := p.read(ctx)
	if err != nil || snapshot.storageKeyID == "" {
		return "", errHookRuntimeConfig
	}
	return snapshot.storageKeyID, nil
}

func (p *RuntimeConfigProvider) EndpointPolicies(ctx context.Context) ([]EndpointPolicy, error) {
	snapshot, err := p.read(ctx)
	if err != nil || len(snapshot.policies) == 0 {
		return nil, errHookRuntimeConfig
	}
	return snapshot.policies, nil
}

// Existing Runs keep their frozen targets after admission is closed or revoked.
func (p *RuntimeConfigProvider) ResolveEndpointPolicy(ctx context.Context, target EndpointTarget) (EndpointPolicy, error) {
	snapshot, err := p.read(ctx)
	if err != nil {
		return EndpointPolicy{}, err
	}
	return snapshot.resolveEndpointPolicy(target)
}

func (s runtimeConfigSnapshot) resolveEndpointPolicy(target EndpointTarget) (EndpointPolicy, error) {
	if !s.runtime.WorkspaceAllowlistConfigured {
		return MatchEndpoint(target, s.policies)
	}
	host, port, err := normalizedEndpointURL(target.URL)
	if err != nil {
		return EndpointPolicy{}, err
	}
	policy, err := normalizeEndpointPolicy(EndpointPolicy{WorkspaceID: target.WorkspaceID, Host: host, Port: port,
		Environment: target.Environment, Lane: target.Lane, AllowPrivateIPs: true})
	if err != nil {
		return EndpointPolicy{}, err
	}
	if ip, err := netip.ParseAddr(policy.Host); err == nil {
		if err := ValidateEndpointIPs(policy, []netip.Addr{ip}); err != nil {
			return EndpointPolicy{}, err
		}
	}
	return policy, nil
}

func (p *RuntimeConfigProvider) read(ctx context.Context) (runtimeConfigSnapshot, error) {
	var empty runtimeConfigSnapshot
	if p == nil || p.loader == nil || ctx == nil || ctx.Err() != nil {
		return empty, errHookRuntimeConfig
	}
	// Get bypasses the Commercial UnmarshalKey cache. A failed/missing read is
	// deliberately not replaced by a last-known-good security configuration.
	value := p.loader.Get(ctx, "lifecycle_hook")
	var raw []byte
	switch v := value.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	case json.RawMessage:
		raw = v
	case map[string]any:
		var err error
		raw, err = json.Marshal(v)
		if err != nil || unsafeRuntimeMapNumber(reflect.ValueOf(v)) {
			return empty, errHookRuntimeConfig
		}
	default:
		return empty, errHookRuntimeConfig
	}
	if ctx.Err() != nil || !validJSONObject(raw) {
		return empty, errHookRuntimeConfig
	}
	var input runtimeConfigInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		return empty, errHookRuntimeConfig
	}
	if input.StorageKeyID != "" && !hookReference(input.StorageKeyID, 128) {
		return empty, errHookRuntimeConfig
	}
	snapshot := runtimeConfigSnapshot{runtime: entity.HookRuntimeConfig{
		AdmissionEnabled:  input.AdmissionEnabled != nil && *input.AdmissionEnabled,
		WorkerEnabled:     p.workerInstalled && (input.WorkerEnabled == nil || *input.WorkerEnabled),
		MQWakeEnabled:     input.MQWakeEnabled == nil || *input.MQWakeEnabled,
		WorkerConcurrency: runtimeValue(input.WorkerConcurrency, 8), WorkspaceConcurrency: runtimeValue(input.WorkspaceConcurrency, 2),
		ScanIntervalSeconds: runtimeValue(input.ScanIntervalSeconds, 10), ScanBatchSize: runtimeValue(input.ScanBatchSize, 100),
		LeaseSeconds: runtimeValue(input.LeaseSeconds, entity.HookDefaultLeaseSeconds), RenewSeconds: runtimeValue(input.RenewSeconds, entity.HookDefaultRenewSeconds),
		IdentityEnrichmentTimeoutMS: runtimeValue(input.IdentityEnrichmentTimeoutMS, 500), RetentionDays: runtimeValue(input.RetentionDays, 30),
	}, storageKeyID: input.StorageKeyID, keys: make(map[runtimeEndpointBinding]runtimeSigningKeyRef)}
	if input.WorkspaceAllowlist != nil {
		var ids []runtimeWorkspaceID
		if json.Unmarshal(input.WorkspaceAllowlist, &ids) != nil || ids == nil {
			return empty, errHookRuntimeConfig
		}
		snapshot.runtime.WorkspaceAllowlistConfigured = true
		snapshot.runtime.WorkspaceAllowlist = make([]int64, 0, len(ids))
		seen := make(map[runtimeWorkspaceID]bool, len(ids))
		for _, id := range ids {
			if seen[id] {
				return empty, errHookRuntimeConfig
			}
			seen[id] = true
			snapshot.runtime.WorkspaceAllowlist = append(snapshot.runtime.WorkspaceAllowlist, int64(id))
		}
	}
	cfg := snapshot.runtime
	if entity.ValidateHookWorkerConfig(cfg) != nil || entity.ValidateHookLeaseTiming(cfg.LeaseSeconds, cfg.RenewSeconds) != nil ||
		cfg.IdentityEnrichmentTimeoutMS <= 0 || cfg.IdentityEnrichmentTimeoutMS > 500 || cfg.RetentionDays <= 0 {
		return empty, errHookRuntimeConfig
	}
	allowed := make(map[runtimeEndpointBinding]bool)
	for _, entry := range input.EndpointPolicy {
		if entry == nil {
			return empty, errHookRuntimeConfig
		}
		policy := entry.runtimeEndpointBinding.policy()
		for _, rawPrefix := range entry.PrivateCIDRs {
			prefix, err := netip.ParsePrefix(rawPrefix)
			if err != nil {
				return empty, errHookRuntimeConfig
			}
			policy.PrivateCIDRs = append(policy.PrivateCIDRs, prefix)
		}
		policy, err := normalizeEndpointPolicy(policy)
		if err != nil {
			return empty, errHookRuntimeConfig
		}
		if ip, err := netip.ParseAddr(policy.Host); err == nil && ValidateEndpointIPs(policy, []netip.Addr{ip}) != nil {
			return empty, errHookRuntimeConfig
		}
		binding := runtimeBinding(policy)
		if allowed[binding] {
			return empty, errHookRuntimeConfig
		}
		allowed[binding] = true
		snapshot.policies = append(snapshot.policies, policy)
	}
	for _, entry := range input.SigningKeyRefs {
		if entry == nil || !signatureToken(entry.KeyID) || len(entry.KeyID) > 128 || !signatureToken(entry.KeyRef) || len(entry.KeyRef) > 128 {
			return empty, errHookRuntimeConfig
		}
		policy, err := normalizeEndpointPolicy(entry.runtimeEndpointBinding.policy())
		if err != nil {
			return empty, errHookRuntimeConfig
		}
		binding := runtimeBinding(policy)
		if !allowed[binding] {
			return empty, errHookRuntimeConfig
		}
		if _, exists := snapshot.keys[binding]; exists {
			return empty, errHookRuntimeConfig
		}
		key := *entry
		key.runtimeEndpointBinding = binding
		snapshot.keys[binding] = key
	}
	if len(allowed) != len(snapshot.keys) || (cfg.AdmissionEnabled && !cfg.WorkspaceAllowlistConfigured && len(allowed) == 0) || ctx.Err() != nil {
		return empty, errHookRuntimeConfig
	}
	return snapshot, nil
}

func runtimeValue(value *int32, fallback int32) int32 {
	if value != nil {
		return *value
	}
	return fallback
}

func (b runtimeEndpointBinding) policy() EndpointPolicy {
	return EndpointPolicy{WorkspaceID: int64(b.WorkspaceID), Host: b.Host, Port: b.Port, Environment: b.Environment, Lane: b.Lane}
}

func runtimeBinding(p EndpointPolicy) runtimeEndpointBinding {
	return runtimeEndpointBinding{WorkspaceID: runtimeWorkspaceID(p.WorkspaceID), Host: p.Host, Port: p.Port, Environment: p.Environment, Lane: p.Lane}
}

// Operator configuration accepts exact integer tokens or decimal strings; this
// does not change the management API or SPI workspace_id representation.
type runtimeWorkspaceID int64

func (id *runtimeWorkspaceID) UnmarshalJSON(raw []byte) error {
	text := string(raw)
	if len(raw) > 0 && raw[0] == '"' {
		if json.Unmarshal(raw, &text) != nil {
			return errHookRuntimeConfig
		}
	}
	if len(text) == 0 {
		return errHookRuntimeConfig
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return errHookRuntimeConfig
		}
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n <= 0 {
		return errHookRuntimeConfig
	}
	*id = runtimeWorkspaceID(n)
	return nil
}

// File JSON may already be rounded by Viper. Never turn an unsafe float into an
// apparently exact integer; use quoted workspace IDs in operator JSON files.
func unsafeRuntimeMapNumber(value reflect.Value) bool {
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return false
		}
		return unsafeRuntimeMapNumber(value.Elem())
	}
	switch value.Kind() {
	case reflect.Float32, reflect.Float64:
		n := value.Float()
		limit := float64(1<<53 - 1)
		if value.Kind() == reflect.Float32 {
			limit = 1<<24 - 1
		}
		return math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > limit || math.Trunc(n) != n
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			if unsafeRuntimeMapNumber(iterator.Value()) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if unsafeRuntimeMapNumber(value.Index(i)) {
				return true
			}
		}
	}
	return false
}
