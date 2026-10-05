// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var errHookStorageCodec = errors.New("invalid protected hook data")

type StorageCodec struct{ protector hookcomponent.Protector }

var _ hookcomponent.StorageCodec = (*StorageCodec)(nil)

func NewStorageCodec(p hookcomponent.Protector) *StorageCodec { return &StorageCodec{protector: p} }

type configPayload struct {
	Version int                       `json:"version"`
	Purpose string                    `json:"purpose"`
	Owner   hookcomponent.ConfigOwner `json:"owner"`
	Config  *entity.LifecycleHookConf `json:"config"`
}
type stageProjection struct {
	Enabled     bool                   `json:"enabled"`
	Environment entity.HookEnvironment `json:"environment"`
}
type configProjection struct {
	Before *stageProjection `json:"before,omitempty"`
	After  *stageProjection `json:"after,omitempty"`
}
type configEnvelope struct {
	Version    int              `json:"version"`
	Purpose    string           `json:"purpose"`
	KeyID      string           `json:"key_id"`
	Hash       string           `json:"hash"`
	Cipher     []byte           `json:"cipher"`
	Projection configProjection `json:"projection"`
}

func (c *StorageCodec) EncodeConfig(ctx context.Context, key string, owner hookcomponent.ConfigOwner, conf *entity.LifecycleHookConf) ([]byte, error) {
	if conf == nil || (conf.Before == nil && conf.After == nil) {
		return nil, nil
	}
	if !validConfigOwner(owner) {
		return nil, errHookStorageCodec
	}
	normalized, err := entity.ResolveLifecycleHookConf(nil, conf)
	if err != nil {
		return nil, errHookStorageCodec
	}
	plain, err := json.Marshal(configPayload{Version: 1, Purpose: "configuration", Owner: owner, Config: normalized})
	if err != nil {
		return nil, errHookStorageCodec
	}
	encrypted, hash, err := c.protect(ctx, key, plain)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(configEnvelope{Version: 1, Purpose: "configuration", KeyID: key, Hash: hash, Cipher: encrypted, Projection: projectHookConfig(normalized)})
	if err != nil || len(encoded) > maxHookProtectedBytes {
		return nil, errHookStorageCodec
	}
	return encoded, nil
}

func (c *StorageCodec) DecodeConfig(ctx context.Context, owner hookcomponent.ConfigOwner, data []byte) (*entity.LifecycleHookConf, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, nil
	}
	if len(data) > maxHookProtectedBytes {
		return nil, errHookStorageCodec
	}
	var shape map[string]json.RawMessage
	if !validJSONObject(data) || json.Unmarshal(data, &shape) != nil {
		return nil, errHookStorageCodec
	}
	if len(shape) == 0 {
		return nil, nil
	}
	var envelope configEnvelope
	if !validConfigOwner(owner) || decodeHookStorageJSON(data, &envelope) != nil || envelope.Version != 1 || envelope.Purpose != "configuration" {
		return nil, errHookStorageCodec
	}
	plain, err := c.unprotect(ctx, envelope.KeyID, envelope.Cipher, envelope.Hash)
	if err != nil {
		return nil, err
	}
	var payload configPayload
	if decodeHookStorageJSON(plain, &payload) != nil || payload.Version != 1 || payload.Purpose != "configuration" || payload.Owner != owner || payload.Config == nil || (payload.Config.Before == nil && payload.Config.After == nil) {
		return nil, errHookStorageCodec
	}
	normalized, err := entity.ResolveLifecycleHookConf(nil, payload.Config)
	if err != nil || !reflect.DeepEqual(envelope.Projection, projectHookConfig(normalized)) {
		return nil, errHookStorageCodec
	}
	return normalized, nil
}

func validConfigOwner(o hookcomponent.ConfigOwner) bool {
	return o.WorkspaceID > 0 && o.ObjectID > 0 && (o.Kind == hookcomponent.ConfigOwnerExperiment || o.Kind == hookcomponent.ConfigOwnerTemplate) && hookReference(o.ExecutionScope, 128)
}
func projectHookConfig(conf *entity.LifecycleHookConf) configProjection {
	project := func(c *entity.HookConfig) *stageProjection {
		if c == nil {
			return nil
		}
		return &stageProjection{Enabled: *c.Enabled, Environment: *c.Environment}
	}
	return configProjection{Before: project(conf.Before), After: project(conf.After)}
}
func (c *StorageCodec) protect(ctx context.Context, key string, plain []byte) ([]byte, string, error) {
	if c == nil || missingHookDependency(c.protector) || !hookReference(key, 128) || len(plain) == 0 || len(plain) > maxHookPlaintextBytes {
		return nil, "", errHookStorageCodec
	}
	encrypted, err := c.protector.Protect(ctx, key, plain)
	if err != nil || len(encrypted) == 0 || len(encrypted) > maxHookProtectedBytes {
		return nil, "", errHookStorageCodec
	}
	return encrypted, hookContentHash(plain), nil
}
func (c *StorageCodec) unprotect(ctx context.Context, key string, encrypted []byte, hash string) ([]byte, error) {
	expected, err := hex.DecodeString(hash)
	if c == nil || missingHookDependency(c.protector) || !hookReference(key, 128) || len(encrypted) == 0 || len(encrypted) > maxHookProtectedBytes || err != nil || len(expected) != sha256.Size {
		return nil, errHookStorageCodec
	}
	plain, err := c.protector.Unprotect(ctx, key, encrypted)
	if err != nil || len(plain) == 0 || len(plain) > maxHookPlaintextBytes || hookContentHash(plain) != hash {
		return nil, errHookStorageCodec
	}
	return plain, nil
}
func hookContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func decodeHookStorageJSON(data []byte, out any) error {
	if !validJSONObject(data) {
		return errHookStorageCodec
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil {
		return errHookStorageCodec
	}
	return nil
}
