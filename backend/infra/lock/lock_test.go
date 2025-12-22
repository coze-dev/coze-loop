package lock

import (
	"context"
	"testing"
	"time"

	redisMocks "github.com/coze-dev/coze-loop/backend/infra/redis/mocks"
	"github.com/redis/go-redis/v9"
	"go.uber.org/mock/gomock"
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

