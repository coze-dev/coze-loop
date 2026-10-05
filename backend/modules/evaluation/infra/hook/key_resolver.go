// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type KeyResolver struct {
	config  *RuntimeConfigProvider
	secrets hookcomponent.SigningSecretProvider
}

var _ hookcomponent.KeyResolver = (*KeyResolver)(nil)

func NewKeyResolver(config *RuntimeConfigProvider, secrets hookcomponent.SigningSecretProvider) *KeyResolver {
	if identityNilProvider(secrets) {
		secrets = nil
	}
	return &KeyResolver{config: config, secrets: secrets}
}

func (r *KeyResolver) Resolve(ctx context.Context, binding entity.HookKeyBinding) (entity.HookSigningKey, error) {
	empty := entity.HookSigningKey{}
	unavailable := errors.New("hook signing key unavailable")
	if r == nil || r.config == nil || r.secrets == nil || ctx == nil || ctx.Err() != nil || strings.Contains(binding.URL, "#") {
		return empty, unavailable
	}
	u, err := url.Parse(binding.URL)
	if err != nil {
		return empty, unavailable
	}
	target := EndpointTarget{WorkspaceID: binding.WorkspaceID, URL: u, Environment: string(binding.Environment), Lane: binding.Lane}
	snapshot, err := r.config.read(ctx)
	if err != nil {
		return empty, unavailable
	}
	policy, err := MatchEndpoint(target, snapshot.policies)
	if err != nil {
		return empty, unavailable
	}
	fingerprint, err := endpointPolicyFingerprint(policy)
	if err != nil {
		return empty, unavailable
	}
	key, ok := snapshot.keys[runtimeBinding(policy)]
	if !ok {
		return empty, unavailable
	}
	secret, err := r.secrets.ReadSigningSecret(ctx, key.KeyRef)
	if err != nil || ctx.Err() != nil || len(secret) == 0 {
		return empty, unavailable
	}
	// A secret read may block while operators revoke or rotate its binding.
	current, err := r.config.read(ctx)
	if err != nil {
		return empty, unavailable
	}
	currentPolicy, err := MatchEndpoint(target, current.policies)
	if err != nil || current.keys[runtimeBinding(currentPolicy)] != key || ctx.Err() != nil {
		return empty, unavailable
	}
	currentFingerprint, err := endpointPolicyFingerprint(currentPolicy)
	if err != nil || currentFingerprint != fingerprint {
		return empty, unavailable
	}
	return entity.HookSigningKey{KeyID: key.KeyID, Secret: append([]byte(nil), secret...), PolicyFingerprint: fingerprint}, nil
}

func endpointPolicyFingerprint(policy EndpointPolicy) (string, error) {
	policy, err := normalizeEndpointPolicy(policy)
	if err != nil {
		return "", err
	}
	slices.SortFunc(policy.PrivateCIDRs, func(a, b netip.Prefix) int { return strings.Compare(a.String(), b.String()) })
	policy.PrivateCIDRs = slices.Compact(policy.PrivateCIDRs)
	if len(policy.PrivateCIDRs) == 0 {
		policy.PrivateCIDRs = nil
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
