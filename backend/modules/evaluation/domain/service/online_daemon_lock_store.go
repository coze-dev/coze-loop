// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import "sync"

// OnlineDaemonLockCancelStore 在线实验心跳锁 cancel 存储，Run/Invoke 抢锁成功后 Store，ExptEnd 结束时 LoadAndDelete 并调用释放
type OnlineDaemonLockCancelStore interface {
	Store(key string, cancel func())
	LoadAndDelete(key string) (func(), bool)
}

type onlineDaemonLockCancelStore struct {
	m sync.Map
}

func NewOnlineDaemonLockCancelStore() OnlineDaemonLockCancelStore {
	return &onlineDaemonLockCancelStore{}
}

func (s *onlineDaemonLockCancelStore) Store(key string, cancel func()) {
	s.m.Store(key, cancel)
}

func (s *onlineDaemonLockCancelStore) LoadAndDelete(key string) (func(), bool) {
	v, ok := s.m.LoadAndDelete(key)
	if !ok {
		return nil, false
	}
	cancel, ok := v.(func())
	return cancel, ok
}
