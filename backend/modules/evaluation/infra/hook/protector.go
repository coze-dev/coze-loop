// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/coze-dev/coze-loop/backend/infra/dkms"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
)

const maxHookProtectedBytes = 16777215
const maxHookPlaintextBytes = 256 * 1024

var errHookProtection = errors.New("hook protection unavailable or invalid data")

type DKMSProtector struct{ client dkms.IDKMS }

var _ hookcomponent.Protector = (*DKMSProtector)(nil)

// keyID is the deployment's DKMS dataKey reference, not raw key material.
// IDKMS does not guarantee AEAD; commercial wiring must inject the Hook-specific GCM wrapper.
func NewDKMSProtector(client dkms.IDKMS) *DKMSProtector { return &DKMSProtector{client: client} }

func (p *DKMSProtector) Protect(ctx context.Context, keyID string, plain []byte) ([]byte, error) {
	if p == nil || missingHookDependency(p.client) || !hookReference(keyID, 128) || len(plain) == 0 || len(plain) > maxHookPlaintextBytes {
		return nil, errHookProtection
	}
	encrypted, err := p.client.Encrypt(ctx, keyID, string(plain))
	if err != nil || encrypted == "" || len(encrypted) > maxHookProtectedBytes {
		return nil, errHookProtection
	}
	return []byte(encrypted), nil
}
func (p *DKMSProtector) Unprotect(ctx context.Context, keyID string, encrypted []byte) ([]byte, error) {
	if p == nil || missingHookDependency(p.client) || !hookReference(keyID, 128) || len(encrypted) == 0 || len(encrypted) > maxHookProtectedBytes {
		return nil, errHookProtection
	}
	plain, err := p.client.Decrypt(ctx, keyID, string(encrypted))
	if err != nil || plain == "" || len(plain) > maxHookPlaintextBytes {
		return nil, errHookProtection
	}
	return []byte(plain), nil
}
func missingHookDependency(v any) bool {
	return v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}
func hookReference(s string, max int) bool {
	if s == "" || len(s) > max || strings.TrimSpace(s) != s {
		return false
	}
	for _, c := range s {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}
