// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

// This fixture exercises authenticated encryption at the existing IDKMS boundary;
// it does not certify the Commercial SDK, configured key or PPE deployment.
type snapshotDKMS struct {
	keys               map[string][]byte
	failure            error
	encrypts, decrypts int
}

func TestStorageCodecRejectsAuthenticatedInvalidPayload(t *testing.T) {
	ctx := context.Background()
	d := newSnapshotDKMS()
	codec := NewStorageCodec(NewDKMSProtector(d))
	s := codecSnapshot(t)
	in := s.Input()
	p, err := codec.EncodeSnapshot(ctx, "key-1", s)
	require.NoError(t, err)
	plaintext, err := d.Decrypt(ctx, p.KeyID, string(p.Cipher))
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		mutate func(map[string]json.RawMessage)
	}{
		{"version", func(m map[string]json.RawMessage) { m["version"] = json.RawMessage(`2`) }},
		{"purpose", func(m map[string]json.RawMessage) { m["purpose"] = json.RawMessage(`"configuration"`) }},
		{"unknown", func(m map[string]json.RawMessage) { m["unknown"] = json.RawMessage(`true`) }},
		{"nil context", func(m map[string]json.RawMessage) { m["context"] = json.RawMessage(`null`) }},
		{"empty context", func(m map[string]json.RawMessage) { m["context"] = json.RawMessage(`{}`) }},
		{"scope", func(m map[string]json.RawMessage) { m["execution_scope"] = json.RawMessage(`"prod"`) }},
		{"disabled run", func(m map[string]json.RawMessage) { m["config"] = json.RawMessage(`{"before":{"enabled":false}}`) }},
		{"object instead of parameter text", func(m map[string]json.RawMessage) {
			m["config"] = json.RawMessage(`{"before":{"enabled":true,"parameters_json":{"a":1}}}`)
		}},
		{"unknown stage field", func(m map[string]json.RawMessage) {
			m["config"] = json.RawMessage(`{"before":{"enabled":true,"secret_unknown":true}}`)
		}},
		{"missing online array", func(m map[string]json.RawMessage) {
			var c map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(m["context"], &c))
			delete(c, "eval_sets")
			m["context"], _ = json.Marshal(c)
		}},
		{"null online array", func(m map[string]json.RawMessage) {
			var c map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(m["context"], &c))
			c["eval_sets"] = json.RawMessage(`null`)
			m["context"], _ = json.Marshal(c)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var m map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(plaintext), &m))
			tc.mutate(m)
			plain, err := json.Marshal(m)
			require.NoError(t, err)
			encrypted, err := d.Encrypt(ctx, p.KeyID, string(plain))
			require.NoError(t, err)
			bad := p
			bad.Cipher = []byte(encrypted)
			sum := sha256.Sum256(plain)
			bad.Hash = hex.EncodeToString(sum[:])
			got, err := codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, bad)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
	// Both encrypted bytes and their real hash can be moved; purpose still binds the type.
	b, err := codec.EncodeConfig(ctx, "key-1", configOwner(), in.Config)
	require.NoError(t, err)
	var envelope configEnvelope
	require.NoError(t, json.Unmarshal(b, &envelope))
	_, err = codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, entity.HookProtectedSnapshot{Cipher: envelope.Cipher, KeyID: envelope.KeyID, Hash: envelope.Hash, ExecutionScope: in.ExecutionScope})
	require.Error(t, err)
	envelope.Cipher, envelope.Hash = p.Cipher, p.Hash
	b, err = json.Marshal(envelope)
	require.NoError(t, err)
	_, err = codec.DecodeConfig(ctx, configOwner(), b)
	require.Error(t, err)
}

func TestStorageCodecParameterAndUTF8Boundaries(t *testing.T) {
	codec := NewStorageCodec(NewDKMSProtector(newSnapshotDKMS()))
	ctx := context.Background()
	for _, size := range []int{16384, 16385} {
		conf := codecSnapshot(t).Input().Config
		conf.Before.ParametersJSON = gptr.Of(`{"x":"` + strings.Repeat("x", size-8) + `"}`)
		b, err := codec.EncodeConfig(ctx, "key-1", configOwner(), conf)
		if size == 16384 {
			require.NoError(t, err)
			out, err := codec.DecodeConfig(ctx, configOwner(), b)
			require.NoError(t, err)
			require.Equal(t, *conf.Before.ParametersJSON, *out.Before.ParametersJSON)
		} else {
			require.Error(t, err)
			require.Nil(t, b)
		}
	}
	for _, text := range []string{`{"x":true,"x":false}`, `{"a":[[[[[[[[1]]]]]]]]}`, `{"x":"` + string([]byte{0xff}) + `"}`} {
		conf := codecSnapshot(t).Input().Config
		conf.Before.ParametersJSON = &text
		_, err := codec.EncodeConfig(ctx, "key-1", configOwner(), conf)
		require.Error(t, err)
	}
	for _, text := range []*string{nil, gptr.Of(" \t\n"), gptr.Of(`{"empty":{},"false":false,"nil":null,"text":"😀中","exponent":1.2300e+20}`)} {
		conf := codecSnapshot(t).Input().Config
		conf.Before.ParametersJSON = text
		conf.Before.Retry = &entity.HookRetryConf{Enabled: gptr.Of(false)}
		b, err := codec.EncodeConfig(ctx, "key-1", configOwner(), conf)
		require.NoError(t, err)
		out, err := codec.DecodeConfig(ctx, configOwner(), b)
		require.NoError(t, err)
		require.False(t, *out.Before.Retry.Enabled)
		if text == nil || strings.TrimSpace(*text) == "" {
			require.Equal(t, "{}", *out.Before.ParametersJSON)
		} else {
			require.Equal(t, *text, *out.Before.ParametersJSON)
		}
	}
}

func TestStorageCodecFrozenRequestBodyLimit(t *testing.T) {
	codec := NewStorageCodec(NewDKMSProtector(newSnapshotDKMS()))
	in := codecSnapshot(t).Input()
	in.Context.Target = &spi.HookTargetRef{ID: gptr.Of("9"), Type: gptr.Of(strings.Repeat("x", 65536))}
	s, err := entity.NewHookRunSnapshot(in)
	if err == nil {
		_, err = codec.EncodeSnapshot(context.Background(), "key-1", s)
	}
	require.Error(t, err, "an irreducibly oversized request must fail before storing the Run")
}

func TestDKMSProtectorFailureAndContextBoundary(t *testing.T) {
	ctx := context.Background()
	d := newSnapshotDKMS()
	p := NewDKMSProtector(d)
	encrypted, err := p.Protect(ctx, "key-1", []byte("private=value"))
	require.NoError(t, err)
	plain, err := p.Unprotect(ctx, "key-1", encrypted)
	require.NoError(t, err)
	require.Equal(t, "private=value", string(plain))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = p.Protect(canceled, "key-1", []byte("private=value"))
	require.Error(t, err)
	var nilDKMS *snapshotDKMS
	for _, protector := range []*DKMSProtector{nil, NewDKMSProtector(nil), NewDKMSProtector(nilDKMS)} {
		_, err = protector.Protect(ctx, "key-1", []byte("private=value"))
		require.Error(t, err)
		_, err = protector.Unprotect(ctx, "key-1", encrypted)
		require.Error(t, err)
	}
	for _, key := range []string{"", " ", "key\n1", strings.Repeat("k", 129), "missing-key"} {
		_, err = p.Protect(ctx, key, []byte("private=value"))
		require.Error(t, err)
		require.NotContains(t, err.Error(), "secret-key-material")
		_, err = p.Unprotect(ctx, key, encrypted)
		require.Error(t, err)
	}
	_, err = p.Protect(ctx, "key-1", nil)
	require.Error(t, err)
	_, err = p.Unprotect(ctx, "key-1", nil)
	require.Error(t, err)
	_, err = p.Protect(ctx, "key-1", make([]byte, 256*1024+1))
	require.Error(t, err)
}

func newSnapshotDKMS() *snapshotDKMS {
	return &snapshotDKMS{keys: map[string][]byte{"key-1": []byte("0123456789abcdef0123456789abcdef"), "key-2": []byte("abcdef0123456789abcdef0123456789")}}
}
func (d *snapshotDKMS) aead(key string) (cipher.AEAD, error) {
	if d.failure != nil {
		return nil, d.failure
	}
	k, ok := d.keys[key]
	if !ok {
		return nil, errors.New("secret-key-material missing")
	}
	b, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}
func (d *snapshotDKMS) Encrypt(ctx context.Context, key, plain string) (string, error) {
	d.encrypts++
	if err := ctx.Err(); err != nil {
		return "", err
	}
	a, err := d.aead(key)
	if err != nil {
		return "", err
	}
	n := make([]byte, a.NonceSize())
	if _, err = rand.Read(n); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(a.Seal(n, n, []byte(plain), nil)), nil
}
func (d *snapshotDKMS) Decrypt(ctx context.Context, key, encrypted string) (string, error) {
	d.decrypts++
	if err := ctx.Err(); err != nil {
		return "", err
	}
	a, err := d.aead(key)
	if err != nil {
		return "", err
	}
	b, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil || len(b) < a.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}
	p, err := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], nil)
	return string(p), err
}
func codecSnapshot(t *testing.T) *entity.HookRunSnapshot {
	t.Helper()
	r := validRequest()
	r.Context.Experiment.Type = gptr.Of("online")
	r.Context.Initiator.Email = gptr.Of("private@example.com")
	s, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, ExecutionScope: "platform-ppe", CreatedAt: time.Date(2026, 9, 23, 1, 2, 3, 0, time.UTC), Context: r.Context,
		Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://hook.example/run?private=query")}, ParametersJSON: gptr.Of(`{"large":9007199254740993,"nested":{"list":[false,null,"秘密"]}}`)}, After: &entity.HookConfig{Enabled: gptr.Of(false)}}})
	require.NoError(t, err)
	return s
}
func configOwner() hookcomponent.ConfigOwner {
	return hookcomponent.ConfigOwner{WorkspaceID: 1, ObjectID: 2, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "platform-ppe"}
}

func TestStorageCodecSnapshotRoundTripAndRotation(t *testing.T) {
	ctx := context.Background()
	d := newSnapshotDKMS()
	codec := NewStorageCodec(NewDKMSProtector(d))
	s := codecSnapshot(t)
	in := s.Input()
	p, err := codec.EncodeSnapshot(ctx, "key-1", s)
	require.NoError(t, err)
	require.Len(t, p.Hash, 64)
	require.Equal(t, "key-1", p.KeyID)
	require.Equal(t, "platform-ppe", p.ExecutionScope)
	require.NotContains(t, string(p.Cipher), "private")
	require.NotContains(t, string(p.Cipher), "秘密")
	decoded, err := codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, p)
	require.NoError(t, err)
	require.Equal(t, in, decoded.Input())
	require.NotNil(t, decoded.Input().Context.EvalSets)
	p2, err := codec.EncodeSnapshot(ctx, "key-2", decoded)
	require.NoError(t, err)
	require.Equal(t, p.Hash, p2.Hash)
	require.NotEqual(t, p.Cipher, p2.Cipher)
	old, err := codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, p)
	require.NoError(t, err)
	require.Equal(t, in, old.Input())
	config, hash, err := codec.DecodePhase(ctx, in.Key, in.ExecutionScope, p, entity.HookPhaseBefore)
	require.NoError(t, err)
	require.Equal(t, p.Hash, hash)
	require.Equal(t, int32(180), *config.TimeoutSeconds)
	*config.TimeoutSeconds = 1
	config, _, err = codec.DecodePhase(ctx, in.Key, in.ExecutionScope, p, entity.HookPhaseBefore)
	require.NoError(t, err)
	require.Equal(t, int32(180), *config.TimeoutSeconds)
	_, _, err = codec.DecodePhase(ctx, in.Key, in.ExecutionScope, p, entity.HookPhaseAfter)
	require.Error(t, err)
	_, _, err = codec.DecodePhase(ctx, in.Key, in.ExecutionScope, p, entity.HookPhase("unknown"))
	require.Error(t, err)
	// Valid authenticated ciphertext cannot borrow another payload's hash.
	changed := in
	changed.Config.Before.ParametersJSON = gptr.Of(`{"changed":true}`)
	s2, err := entity.NewHookRunSnapshot(changed)
	require.NoError(t, err)
	p3, err := codec.EncodeSnapshot(ctx, "key-1", s2)
	require.NoError(t, err)
	p3.Hash = p.Hash
	_, err = codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, p3)
	require.Error(t, err)
}

func TestStorageCodecConfigAbsentNeverUsesDKMS(t *testing.T) {
	d := newSnapshotDKMS()
	d.failure = errors.New("dependency unavailable")
	codec := NewStorageCodec(NewDKMSProtector(d))
	ctx := context.Background()
	for _, raw := range [][]byte{nil, {}, []byte("  "), []byte("null"), []byte("{}"), []byte(" { } ")} {
		c, err := codec.DecodeConfig(ctx, configOwner(), raw)
		require.NoError(t, err)
		require.Nil(t, c)
	}
	for _, c := range []*entity.LifecycleHookConf{nil, {}} {
		b, err := codec.EncodeConfig(ctx, "", configOwner(), c)
		require.NoError(t, err)
		require.Nil(t, b)
	}
	require.Zero(t, d.encrypts)
	require.Zero(t, d.decrypts)
}

func TestStorageCodecDisabledConfigRetainsSensitiveFields(t *testing.T) {
	codec := NewStorageCodec(NewDKMSProtector(newSnapshotDKMS()))
	ctx := context.Background()
	conf := codecSnapshot(t).Input().Config
	*conf.Before.Enabled = false
	b, err := codec.EncodeConfig(ctx, "key-1", configOwner(), conf)
	require.NoError(t, err)
	for _, secret := range []string{"hook.example", "private=query", "9007199254740993", "parameters_json", "private@example.com", "秘密"} {
		require.NotContains(t, string(b), secret)
	}
	got, err := codec.DecodeConfig(ctx, configOwner(), b)
	require.NoError(t, err)
	require.Equal(t, conf, got)
	require.False(t, *got.Before.Enabled)
	var outer map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &outer))
	require.ElementsMatch(t, []string{"version", "purpose", "key_id", "hash", "cipher", "projection"}, mapKeys(outer))
	var projection map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(outer["projection"], &projection))
	require.Len(t, projection, 2)
	require.JSONEq(t, `{"enabled":false,"environment":"Prod"}`, string(projection["before"]))
}
func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestStorageCodecRejectsCrossOwnershipAndTamper(t *testing.T) {
	ctx := context.Background()
	codec := NewStorageCodec(NewDKMSProtector(newSnapshotDKMS()))
	s := codecSnapshot(t)
	in := s.Input()
	p, err := codec.EncodeSnapshot(ctx, "key-1", s)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		change func(*entity.HookRunKey, *string, *entity.HookProtectedSnapshot)
	}{
		{"run", func(k *entity.HookRunKey, _ *string, _ *entity.HookProtectedSnapshot) { k.RunID++ }},
		{"experiment", func(k *entity.HookRunKey, _ *string, _ *entity.HookProtectedSnapshot) { k.ExperimentID++ }},
		{"space", func(k *entity.HookRunKey, _ *string, _ *entity.HookProtectedSnapshot) { k.WorkspaceID++ }},
		{"scope", func(_ *entity.HookRunKey, s *string, p *entity.HookProtectedSnapshot) {
			*s = "prod"
			p.ExecutionScope = "prod"
		}},
		{"hash", func(_ *entity.HookRunKey, _ *string, p *entity.HookProtectedSnapshot) {
			p.Hash = strings.Repeat("0", 64)
		}},
		{"key", func(_ *entity.HookRunKey, _ *string, p *entity.HookProtectedSnapshot) { p.KeyID = "key-2" }},
		{"empty key", func(_ *entity.HookRunKey, _ *string, p *entity.HookProtectedSnapshot) { p.KeyID = "" }},
		{"cipher", func(_ *entity.HookRunKey, _ *string, p *entity.HookProtectedSnapshot) { p.Cipher[12] ^= 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key, scope, q := in.Key, in.ExecutionScope, p
			q.Cipher = append([]byte(nil), p.Cipher...)
			tc.change(&key, &scope, &q)
			got, err := codec.DecodeSnapshot(ctx, key, scope, q)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
	b, err := codec.EncodeConfig(ctx, "key-1", configOwner(), in.Config)
	require.NoError(t, err)
	for _, owner := range []hookcomponent.ConfigOwner{
		{WorkspaceID: 2, ObjectID: 2, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "platform-ppe"},
		{WorkspaceID: 1, ObjectID: 3, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "platform-ppe"},
		{WorkspaceID: 1, ObjectID: 2, Kind: hookcomponent.ConfigOwnerTemplate, ExecutionScope: "platform-ppe"},
		{WorkspaceID: 1, ObjectID: 2, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "prod"},
	} {
		got, err := codec.DecodeConfig(ctx, owner, b)
		require.Error(t, err)
		require.Nil(t, got)
	}
	var outer map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &outer))
	for _, tc := range []struct{ field, value string }{{"version", "2"}, {"purpose", `"snapshot"`}, {"hash", `"` + strings.Repeat("0", 64) + `"`}, {"projection", `{"before":{"enabled":true,"environment":"PPE"}}`}, {"unknown", "true"}} {
		copy := make(map[string]json.RawMessage)
		for k, v := range outer {
			copy[k] = v
		}
		copy[tc.field] = json.RawMessage(tc.value)
		bad, _ := json.Marshal(copy)
		got, err := codec.DecodeConfig(ctx, configOwner(), bad)
		require.Error(t, err)
		require.Nil(t, got)
	}
}

func TestStorageCodecRejectsUnknownNonemptyShapeAndLeaksNoDependencyText(t *testing.T) {
	ctx := context.Background()
	d := newSnapshotDKMS()
	codec := NewStorageCodec(NewDKMSProtector(d))
	for _, raw := range []string{`[]`, `""`, `false`, `{"before":{}}`, `{"version":1}`, `{"unknown":null}`, `{"version":1,"version":1}`} {
		got, err := codec.DecodeConfig(ctx, configOwner(), []byte(raw))
		require.Error(t, err, raw)
		require.Nil(t, got)
	}
	s := codecSnapshot(t)
	p, err := codec.EncodeSnapshot(ctx, "key-1", s)
	require.NoError(t, err)
	d.failure = errors.New("https://hook.example/?private=query secret-key-material private@example.com")
	for _, c := range []*StorageCodec{codec, NewStorageCodec(nil), NewStorageCodec(NewDKMSProtector(nil))} {
		got, err := c.DecodeSnapshot(ctx, s.Input().Key, s.Input().ExecutionScope, p)
		require.Error(t, err)
		require.Nil(t, got)
		for _, secret := range []string{"private", "https", "secret-key-material"} {
			require.NotContains(t, err.Error(), secret)
		}
		q, err := c.EncodeSnapshot(ctx, "key-1", s)
		require.Error(t, err)
		require.Empty(t, q.Cipher)
	}
	_, err = codec.EncodeSnapshot(ctx, "", s)
	require.Error(t, err)
}
