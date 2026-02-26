// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package lock

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/mock/gomock"

	redisMocks "github.com/coze-dev/coze-loop/backend/infra/redis/mocks"
)

// helper to build a redis.Cmd with given value and error
func newIntCmdResult(val int64, err error) *redis.Cmd {
	cmd := redis.NewCmd(context.Background())
	if err != nil {
		cmd.SetErr(err)
		return cmd
	}
	cmd.SetVal(val)
	return cmd
}

// helper to build a redis.Cmd for Eval script result (val is typically []interface{}{locked, holder})
func newEvalCmdResult(val interface{}, err error) *redis.Cmd {
	cmd := redis.NewCmd(context.Background())
	if err != nil {
		cmd.SetErr(err)
		return cmd
	}
	cmd.SetVal(val)
	return cmd
}

func TestRedisLocker_BackoffLockWithValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cases := []struct {
		name      string
		expiresIn time.Duration
		maxWait   time.Duration
		setupMock func(m *redisMocks.MockPersistentCmdable)
		wantOk    bool
		wantVal   string
		wantErr   bool
	}{
		{
			name:      "ttl_too_short_returns_error",
			expiresIn: 100 * time.Millisecond,
			maxWait:   time.Second,
			setupMock: func(m *redisMocks.MockPersistentCmdable) {},
			wantOk:    false,
			wantVal:   "",
			wantErr:   true,
		},
		{
			name:      "success_locked_on_first_try",
			expiresIn: 2 * time.Second,
			maxWait:   time.Second,
			setupMock: func(m *redisMocks.MockPersistentCmdable) {
				m.EXPECT().
					Eval(gomock.Any(), setNXWithGetScript, []string{"key1"}, "val1", int64(2000)).
					Return(newEvalCmdResult([]interface{}{int64(1), "val1"}, nil)).
					Times(1)
			},
			wantOk:  true,
			wantVal: "val1",
			wantErr: false,
		},
		{
			name:      "locked_by_others_returns_holder_string",
			expiresIn: 2 * time.Second,
			maxWait:   100 * time.Millisecond,
			setupMock: func(m *redisMocks.MockPersistentCmdable) {
				m.EXPECT().
					Eval(gomock.Any(), setNXWithGetScript, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(newEvalCmdResult([]interface{}{int64(0), "other_holder"}, nil)).
					AnyTimes()
			},
			wantOk:  false,
			wantVal: "other_holder",
			wantErr: false,
		},
		{
			name:      "locked_by_others_returns_holder_bytes",
			expiresIn: 2 * time.Second,
			maxWait:   100 * time.Millisecond,
			setupMock: func(m *redisMocks.MockPersistentCmdable) {
				m.EXPECT().
					Eval(gomock.Any(), setNXWithGetScript, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(newEvalCmdResult([]interface{}{int64(0), []byte("byte_holder")}, nil)).
					AnyTimes()
			},
			wantOk:  false,
			wantVal: "byte_holder",
			wantErr: false,
		},
		{
			name:      "redis_error_returns_error",
			expiresIn: 2 * time.Second,
			maxWait:   100 * time.Millisecond,
			setupMock: func(m *redisMocks.MockPersistentCmdable) {
				m.EXPECT().
					Eval(gomock.Any(), setNXWithGetScript, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(newEvalCmdResult(nil, context.DeadlineExceeded)).
					AnyTimes()
			},
			wantOk:  false,
			wantVal: "",
			wantErr: true,
		},
		{
			name:      "unexpected_script_result_type_returns_error",
			expiresIn: 2 * time.Second,
			maxWait:   100 * time.Millisecond,
			setupMock: func(m *redisMocks.MockPersistentCmdable) {
				m.EXPECT().
					Eval(gomock.Any(), setNXWithGetScript, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(newEvalCmdResult(interface{}("not_a_slice"), nil)).
					AnyTimes()
			},
			wantOk:  false,
			wantVal: "",
			wantErr: true,
		},
		{
			name:      "unexpected_script_result_length_returns_error",
			expiresIn: 2 * time.Second,
			maxWait:   100 * time.Millisecond,
			setupMock: func(m *redisMocks.MockPersistentCmdable) {
				m.EXPECT().
					Eval(gomock.Any(), setNXWithGetScript, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(newEvalCmdResult([]interface{}{int64(0)}, nil)).
					AnyTimes()
			},
			wantOk:  false,
			wantVal: "",
			wantErr: true,
		},
		{
			name:      "unexpected_holder_type_returns_error",
			expiresIn: 2 * time.Second,
			maxWait:   100 * time.Millisecond,
			setupMock: func(m *redisMocks.MockPersistentCmdable) {
				m.EXPECT().
					Eval(gomock.Any(), setNXWithGetScript, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(newEvalCmdResult([]interface{}{int64(0), 12345}, nil)).
					AnyTimes()
			},
			wantOk:  false,
			wantVal: "",
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockRedis := redisMocks.NewMockPersistentCmdable(ctrl)
			c.setupMock(mockRedis)
			locker := &redisLocker{c: mockRedis, holder: "holder"}
			ok, val, err := locker.BackoffLockWithValue(context.Background(), "key1", "val1", c.expiresIn, c.maxWait)
			if c.wantErr != (err != nil) {
				t.Fatalf("err: got %v, wantErr %v", err, c.wantErr)
			}
			if ok != c.wantOk {
				t.Errorf("ok: got %v, want %v", ok, c.wantOk)
			}
			if val != c.wantVal {
				t.Errorf("val: got %q, want %q", val, c.wantVal)
			}
		})
	}
}

func TestRedisLocker_renewLock_ContextDoneUnlocksAndReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRedis := redisMocks.NewMockPersistentCmdable(ctrl)
	key := "test-key"

	// Unlock should be called once when context is done.
	mockRedis.
		EXPECT().
		Eval(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(newIntCmdResult(1, nil)).
		Times(1)

	locker := &redisLocker{
		c:      mockRedis,
		holder: "holder",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		locker.renewLock(ctx, key, time.Second, 5*time.Second)
		close(done)
	}()

	// cancel context shortly after starting renewLock
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("renewLock did not return after context cancel")
	}
}

func TestRedisLocker_renewLock_MaxHoldUnlocksAndReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRedis := redisMocks.NewMockPersistentCmdable(ctrl)
	key := "test-key"

	// Unlock should be called once when maxHold is reached.
	mockRedis.
		EXPECT().
		Eval(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(newIntCmdResult(1, nil)).
		Times(1)

	locker := &redisLocker{
		c:      mockRedis,
		holder: "holder",
	}

	ctx := context.Background()
	maxHold := 50 * time.Millisecond

	done := make(chan struct{})
	go func() {
		locker.renewLock(ctx, key, time.Second, maxHold)
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("renewLock did not return after maxHold")
	}
}

func TestRedisLocker_renewLock_ExpireLockLostReturns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRedis := redisMocks.NewMockPersistentCmdable(ctrl)
	key := "test-key"

	// Expect one Eval call from ExpireLockIn; simulate "lock lost" (return 0, nil).
	mockRedis.
		EXPECT().
		Eval(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(newIntCmdResult(0, nil)).
		Times(1)

	locker := &redisLocker{
		c:      mockRedis,
		holder: "holder",
	}

	ctx := context.Background()
	ttl := time.Second
	maxHold := 5 * time.Second

	done := make(chan struct{})
	go func() {
		locker.renewLock(ctx, key, ttl, maxHold)
		close(done)
	}()

	// wait for at most ~2 seconds for ticker + renew path
	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatalf("renewLock did not return after ExpireLockIn reported lock lost")
	}
}
