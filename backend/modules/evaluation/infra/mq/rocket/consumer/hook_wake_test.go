// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type hookWakeRecorder struct {
	events []entity.HookWakeEvent
	err    error
}

func (w *hookWakeRecorder) Wake(_ context.Context, event entity.HookWakeEvent) error {
	w.events = append(w.events, event)
	return w.err
}

func hookWakeMessage() *mq.MessageExt {
	return &mq.MessageExt{Message: mq.Message{Tag: "lifecycle_hook_wake", Body: []byte(`{"workspace_id":1,"experiment_id":2,"run_id":3,"operation_id":"hook_42","execution_scope":"ppe_hooks"}`)}}
}

func TestHookWakeConsumerDuplicateAndUnorderedHints(t *testing.T) {
	w := &hookWakeRecorder{}
	c, err := NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", nil }, w)
	require.NoError(t, err)
	// No claims, attempts or HTTP dependency exists at this boundary; all hints
	// reach the worker, which must decide eligibility from current database state.
	for _, id := range []string{"hook_42", "hook_41", "hook_42"} {
		msg := hookWakeMessage()
		msg.Body = []byte(strings.Replace(string(msg.Body), "hook_42", id, 1))
		require.NoError(t, c.HandleMessage(context.Background(), msg))
	}
	require.Equal(t, []entity.HookWakeEvent{
		{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "hook_42", ExecutionScope: "ppe_hooks"},
		{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "hook_41", ExecutionScope: "ppe_hooks"},
		{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "hook_42", ExecutionScope: "ppe_hooks"},
	}, w.events)
}

func TestHookWakeConsumerChecksCurrentTrustedScope(t *testing.T) {
	scope := "ppe_hooks"
	w := &hookWakeRecorder{}
	c, err := NewHookWakeConsumer(func(context.Context) (string, error) { return scope, nil }, w)
	require.NoError(t, err)
	require.NoError(t, c.HandleMessage(context.Background(), hookWakeMessage()))
	for _, value := range []string{"prod", "boe_hooks", "ppe_other", "", "ppe hooks", strings.Repeat("s", 129)} {
		scope = value
		msg := hookWakeMessage()
		msg.Properties = map[string]string{"execution_scope": "ppe_hooks", "x-tt-env": "ppe_hooks"}
		err = c.HandleMessage(context.Background(), msg)
		require.Error(t, err)
	}
	require.Len(t, w.events, 1)
}

func TestHookWakeConsumerInvalidMessageNeverWakes(t *testing.T) {
	for name, mutate := range map[string]func(*mq.MessageExt){
		"old_tag":     func(m *mq.MessageExt) { m.Tag = "webhook_retry" },
		"missing_tag": func(m *mq.MessageExt) { m.Tag = "" },
		"old_payload": func(m *mq.MessageExt) { m.Body = []byte(`{"ExptID":1,"Payload":"secret@example.com"}`) },
		"malformed":   func(m *mq.MessageExt) { m.Body = []byte(`secret@example.com`) },
		"missing_id": func(m *mq.MessageExt) {
			m.Body = []byte(strings.Replace(string(m.Body), `"run_id":3`, `"run_id":0`, 1))
		},
		"sensitive_extra": func(m *mq.MessageExt) {
			m.Body = []byte(strings.TrimSuffix(string(m.Body), "}") + `,"email":"secret@example.com"}`)
		},
		"oversize": func(m *mq.MessageExt) { m.Body = []byte(strings.Repeat("x", 1025)) },
	} {
		t.Run(name, func(t *testing.T) {
			w := &hookWakeRecorder{}
			c, err := NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", nil }, w)
			require.NoError(t, err)
			msg := hookWakeMessage()
			mutate(msg)
			err = c.HandleMessage(context.Background(), msg)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret@example.com")
			require.Empty(t, w.events)
		})
	}
	w := &hookWakeRecorder{}
	c, err := NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", nil }, w)
	require.NoError(t, err)
	require.Error(t, c.HandleMessage(context.Background(), nil))
	require.Empty(t, w.events)
}

func TestHookWakeConsumerFailuresAreVisibleAndRedacted(t *testing.T) {
	w := &hookWakeRecorder{}
	c, err := NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", errors.New("secret@example.com") }, w)
	require.NoError(t, err)
	err = c.HandleMessage(context.Background(), hookWakeMessage())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret@example.com")
	require.Nil(t, errors.Unwrap(err))
	require.Empty(t, w.events)
	w.err = errors.New("secret@example.com")
	c, err = NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", nil }, w)
	require.NoError(t, err)
	err = c.HandleMessage(context.Background(), hookWakeMessage())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret@example.com")
	require.Nil(t, errors.Unwrap(err))
	require.Len(t, w.events, 1)
}

func TestHookWakeConsumerCanceledContextNeverWakes(t *testing.T) {
	w := &hookWakeRecorder{}
	c, err := NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", nil }, w)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, c.HandleMessage(ctx, hookWakeMessage()), context.Canceled)
	require.Empty(t, w.events)
}

func TestHookWakeConsumerConstructionRejectsMissingDependency(t *testing.T) {
	c, err := NewHookWakeConsumer(nil, &hookWakeRecorder{})
	require.Error(t, err)
	require.Nil(t, c)
	c, err = NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", nil }, nil)
	require.Error(t, err)
	require.Nil(t, c)
}

type hookWakeFunc func(context.Context, entity.HookWakeEvent) error

func (f hookWakeFunc) Wake(ctx context.Context, event entity.HookWakeEvent) error {
	return f(ctx, event)
}

func TestHookWakeConsumerRejectsTypedNilHandler(t *testing.T) {
	for name, dependency := range map[string]hook.WakeHandler{
		"pointer":  (*hookWakeRecorder)(nil),
		"function": hookWakeFunc(nil),
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", nil }, dependency)
			require.Error(t, err)
			require.Nil(t, c)
		})
	}
}

type hookWakeValueHandler struct{ *hookWakeRecorder }

func TestHookWakeConsumerAcceptsValueAndFunctionHandlers(t *testing.T) {
	for _, name := range []string{"value", "function"} {
		t.Run(name, func(t *testing.T) {
			recorder := &hookWakeRecorder{}
			var dependency hook.WakeHandler = hookWakeValueHandler{recorder}
			if name == "function" {
				dependency = hookWakeFunc(recorder.Wake)
			}
			c, err := NewHookWakeConsumer(func(context.Context) (string, error) { return "ppe_hooks", nil }, dependency)
			require.NoError(t, err)
			require.NoError(t, c.HandleMessage(context.Background(), hookWakeMessage()))
			require.Equal(t, []entity.HookWakeEvent{{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "hook_42", ExecutionScope: "ppe_hooks"}}, recorder.events)
		})
	}
}
