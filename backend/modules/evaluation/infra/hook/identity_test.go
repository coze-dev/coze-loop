// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/cloudwego/kitex/client/callopt"
	userentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/user"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/foundation"
	"github.com/stretchr/testify/require"
)

type identityUsersFunc func(context.Context, []string) ([]*entity.UserInfo, error)

func (f identityUsersFunc) MGetUserInfo(ctx context.Context, ids []string) ([]*entity.UserInfo, error) {
	return f(ctx, ids)
}

type identityFoundationClient struct {
	userservice.Client
	profile *userentity.UserInfoDetail
}

func (c *identityFoundationClient) MGetUserInfo(_ context.Context, req *user.MGetUserInfoRequest, _ ...callopt.Option) (*user.MGetUserInfoResponse, error) {
	return &user.MGetUserInfoResponse{UserInfos: []*userentity.UserInfoDetail{{UserID: gptr.Of("41"), Email: gptr.Of("wrong@example.com")}, c.profile}}, nil
}

func TestHookIdentityFoundationAdapterAndFrozenSnapshot(t *testing.T) {
	client := &identityFoundationClient{profile: &userentity.UserInfoDetail{UserID: gptr.Of("42"), Email: gptr.Of("initial@example.com"), NickName: gptr.Of("initial")}}
	p, err := NewIdentityProvider(foundation.NewUserRPCProvider(client), 0)
	require.NoError(t, err)
	initiator, err := p.ResolveInitiator(context.Background(), "42")
	require.NoError(t, err)
	input := codecSnapshot(t).Input()
	input.Context.Initiator = initiator
	frozen, err := entity.NewHookRunSnapshot(input)
	require.NoError(t, err)
	*client.profile.Email, *client.profile.NickName = "later@example.com", "later"
	*initiator.UserID = "43"
	later, err := p.ResolveInitiator(context.Background(), "42")
	require.NoError(t, err)
	require.Equal(t, "later@example.com", later.GetEmail())
	got := frozen.Input().Context.Initiator
	require.Equal(t, "42", got.GetUserID())
	require.Equal(t, "initial@example.com", got.GetEmail())
	require.Equal(t, "initial", got.GetName())
}

func TestHookIdentityUsesOnlyExactIDAndOwnsProfile(t *testing.T) {
	profile := &entity.UserInfo{UserID: gptr.Of("42"), Email: gptr.Of("same@example.com"), Name: gptr.Of("名字")}
	provider, err := NewIdentityProvider(identityUsersFunc(func(ctx context.Context, ids []string) ([]*entity.UserInfo, error) {
		require.Equal(t, []string{"42"}, ids)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 500*time.Millisecond)
		return []*entity.UserInfo{nil, {UserID: gptr.Of("41"), Email: gptr.Of("other@example.com")}, profile}, nil
	}), 0)
	require.NoError(t, err)
	got, err := provider.ResolveInitiator(context.Background(), "42")
	require.NoError(t, err)
	require.Equal(t, "42", got.GetUserID())
	require.Equal(t, "fornax_user", got.GetIdentityType())
	require.Equal(t, "same@example.com", got.GetEmail())
	require.Equal(t, "名字", got.GetName())
	*profile.UserID, *profile.Email, *profile.Name = "41", "changed", "changed"
	require.Equal(t, "42", got.GetUserID())
	require.Equal(t, "same@example.com", got.GetEmail())
	require.Equal(t, "名字", got.GetName())
}

func TestHookIdentityOptionalProfileFailures(t *testing.T) {
	for _, tt := range []struct {
		name                string
		users               []*entity.UserInfo
		err                 error
		wantEmail, wantName *string
	}{
		{name: "lookup error", err: errors.New("sensitive upstream failure"), users: []*entity.UserInfo{{UserID: gptr.Of("42"), Email: gptr.Of("ignore@example.com")}}},
		{name: "missing"},
		{name: "wrong user", users: []*entity.UserInfo{{UserID: gptr.Of("41"), Email: gptr.Of("wrong@example.com")}}},
		{name: "missing returned id", users: []*entity.UserInfo{{Email: gptr.Of("wrong@example.com")}}},
		{name: "ambiguous same id", users: []*entity.UserInfo{{UserID: gptr.Of("42"), Name: gptr.Of("one")}, {UserID: gptr.Of("42"), Name: gptr.Of("two")}}},
		{name: "email overlong", users: []*entity.UserInfo{{UserID: gptr.Of("42"), Email: gptr.Of(strings.Repeat("a", 321)), Name: gptr.Of("keep")}}, wantName: gptr.Of("keep")},
		{name: "name overlong bytes", users: []*entity.UserInfo{{UserID: gptr.Of("42"), Email: gptr.Of("keep@example.com"), Name: gptr.Of(strings.Repeat("名", 86))}}, wantEmail: gptr.Of("keep@example.com")},
		{name: "invalid utf8", users: []*entity.UserInfo{{UserID: gptr.Of("42"), Email: gptr.Of("\xff"), Name: gptr.Of("\xff")}}},
		{name: "empty optional", users: []*entity.UserInfo{{UserID: gptr.Of("42"), Email: gptr.Of(" "), Name: gptr.Of("")}}},
		{name: "byte boundaries", users: []*entity.UserInfo{{UserID: gptr.Of("42"), Email: gptr.Of(strings.Repeat("a", 320)), Name: gptr.Of(strings.Repeat("n", 256))}}, wantEmail: gptr.Of(strings.Repeat("a", 320)), wantName: gptr.Of(strings.Repeat("n", 256))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewIdentityProvider(identityUsersFunc(func(context.Context, []string) ([]*entity.UserInfo, error) { return tt.users, tt.err }), 0)
			require.NoError(t, err)
			got, err := p.ResolveInitiator(context.Background(), "42")
			require.NoError(t, err)
			require.Equal(t, "42", got.GetUserID())
			require.Equal(t, "fornax_user", got.GetIdentityType())
			require.Equal(t, tt.wantEmail, got.Email)
			require.Equal(t, tt.wantName, got.Name)
		})
	}
}

func TestHookIdentityRejectsInvalidIDWithoutLookup(t *testing.T) {
	p, err := NewIdentityProvider(identityUsersFunc(func(context.Context, []string) ([]*entity.UserInfo, error) {
		t.Fatal("invalid ID reached user service")
		return nil, nil
	}), 0)
	require.NoError(t, err)
	for _, id := range []string{"", "0", " 42", "42 ", "user\nname", "user\x00name", strings.Repeat("1", 129), "\xff"} {
		got, err := p.ResolveInitiator(context.Background(), id)
		require.EqualError(t, err, "HOOK_IDENTITY_INVALID")
		require.Nil(t, got)
	}
}

func TestHookIdentityPreservesOpaqueUserID(t *testing.T) {
	for _, id := range []string{"user1", "trusted-user", "9223372036854775808", strings.Repeat("u", 128)} {
		p, err := NewIdentityProvider(identityUsersFunc(func(_ context.Context, ids []string) ([]*entity.UserInfo, error) {
			require.Equal(t, []string{id}, ids)
			return []*entity.UserInfo{{UserID: gptr.Of(id), Email: gptr.Of("same@example.com")}}, nil
		}), 0)
		require.NoError(t, err)
		got, err := p.ResolveInitiator(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, id, got.GetUserID())
		require.Equal(t, "same@example.com", got.GetEmail())
	}
}

func TestHookIdentityBudgetAndUnavailableProvider(t *testing.T) {
	for _, parentBudget := range []time.Duration{20 * time.Millisecond, time.Second} {
		t.Run(parentBudget.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), parentBudget)
			defer cancel()
			p, err := NewIdentityProvider(identityUsersFunc(func(ctx context.Context, _ []string) ([]*entity.UserInfo, error) {
				<-ctx.Done()
				return []*entity.UserInfo{{UserID: gptr.Of("42"), Email: gptr.Of("late@example.com")}}, nil
			}), 50*time.Millisecond)
			require.NoError(t, err)
			start := time.Now()
			got, err := p.ResolveInitiator(ctx, "42")
			require.NoError(t, err)
			require.Less(t, time.Since(start), 300*time.Millisecond)
			require.Nil(t, got.Email)
			require.Equal(t, "42", got.GetUserID())
		})
	}
	for _, timeout := range []time.Duration{-1, 501 * time.Millisecond} {
		_, err := NewIdentityProvider(nil, timeout)
		require.Error(t, err)
	}
	var users identityUsersFunc
	p, err := NewIdentityProvider(users, 0)
	require.NoError(t, err)
	got, err := p.ResolveInitiator(context.Background(), "42")
	require.NoError(t, err)
	require.Nil(t, got.Email)
	require.Nil(t, got.Name)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p, err = NewIdentityProvider(identityUsersFunc(func(context.Context, []string) ([]*entity.UserInfo, error) {
		t.Fatal("canceled lookup")
		return nil, nil
	}), 0)
	require.NoError(t, err)
	got, err = p.ResolveInitiator(ctx, "42")
	require.NoError(t, err)
	require.Equal(t, "42", got.GetUserID())
}
