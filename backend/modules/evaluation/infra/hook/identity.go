// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IdentityProvider struct {
	users   rpc.IUserProvider
	timeout time.Duration
}

var _ hookcomponent.IdentityProvider = (*IdentityProvider)(nil)

func NewIdentityProvider(users rpc.IUserProvider, timeout time.Duration) (*IdentityProvider, error) {
	if timeout < 0 || timeout > 500*time.Millisecond {
		return nil, errors.New("invalid hook identity timeout")
	}
	if timeout == 0 {
		timeout = 500 * time.Millisecond
	}
	if identityNilProvider(users) {
		users = nil
	}
	return &IdentityProvider{users: users, timeout: timeout}, nil
}

func (p *IdentityProvider) ResolveInitiator(ctx context.Context, userID string) (*spi.HookInitiator, error) {
	if !validIdentityUserID(userID) || ctx == nil {
		return nil, errors.New("HOOK_IDENTITY_INVALID")
	}
	identityType := "fornax_user"
	initiator := &spi.HookInitiator{UserID: &userID, IdentityType: &identityType}
	if p == nil || p.users == nil || ctx.Err() != nil {
		return initiator, nil
	}
	// The existing RPC provider must honor cancellation; no background enrichment
	// or later snapshot mutation is scheduled here.
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	users, err := p.users.MGetUserInfo(ctx, []string{userID})
	if err != nil || ctx.Err() != nil {
		return initiator, nil
	}
	var matched *entity.UserInfo
	for _, user := range users {
		if user == nil || user.UserID == nil || *user.UserID != userID {
			continue
		}
		if matched != nil {
			return initiator, nil
		}
		matched = user
	}
	if matched != nil {
		initiator.Email = identityOptionalText(matched.Email, 320)
		initiator.Name = identityOptionalText(matched.Name, 256)
	}
	return initiator, nil
}

func validIdentityUserID(id string) bool {
	if len(id) == 0 || len(id) > 128 || id == "0" || !utf8.ValidString(id) {
		return false
	}
	for _, c := range id {
		if unicode.IsSpace(c) || unicode.IsControl(c) {
			return false
		}
	}
	return true
}

func identityOptionalText(value *string, maxBytes int) *string {
	if value == nil || strings.TrimSpace(*value) == "" || !utf8.ValidString(*value) || len(*value) > maxBytes {
		return nil
	}
	copy := *value
	return &copy
}

func identityNilProvider(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}
