package convertor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
)

func TestUserInfoDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	result := UserInfoDO2DTO(nil)
	assert.Nil(t, result)
}

func TestUserInfoDO2DTO_FullFields(t *testing.T) {
	t.Parallel()
	do := &rpc.UserInfo{
		UserID:    "user-1",
		UserName:  "Demo User",
		NickName:  "Demo",
		AvatarURL: "https://example.com/avatar.png",
		Email:     "demo@example.com",
		Mobile:    "1234567890",
	}
	result := UserInfoDO2DTO(do)
	assert.NotNil(t, result)
	assert.Equal(t, "user-1", result.GetUserID())
	assert.Equal(t, "Demo User", result.GetName())
	assert.Equal(t, "Demo", result.GetNickName())
	assert.Equal(t, "https://example.com/avatar.png", result.GetAvatarURL())
	assert.Equal(t, "demo@example.com", result.GetEmail())
	assert.Equal(t, "1234567890", result.GetMobile())
}

func TestUserInfoDO2DTO_EmptyFields(t *testing.T) {
	t.Parallel()
	do := &rpc.UserInfo{}
	result := UserInfoDO2DTO(do)
	assert.NotNil(t, result)
	assert.Equal(t, "", result.GetUserID())
	assert.Equal(t, "", result.GetName())
}

func TestBatchUserInfoDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	result := BatchUserInfoDO2DTO(nil)
	assert.Nil(t, result)
}

func TestBatchUserInfoDO2DTO_EmptySlice(t *testing.T) {
	t.Parallel()
	result := BatchUserInfoDO2DTO([]*rpc.UserInfo{})
	assert.Nil(t, result)
}

func TestBatchUserInfoDO2DTO_AllNil(t *testing.T) {
	t.Parallel()
	result := BatchUserInfoDO2DTO([]*rpc.UserInfo{nil, nil})
	assert.Nil(t, result)
}

func TestBatchUserInfoDO2DTO_MixedNilAndValid(t *testing.T) {
	t.Parallel()
	dos := []*rpc.UserInfo{
		nil,
		{UserID: "user-1", UserName: "Demo User 1"},
		nil,
		{UserID: "user-2", UserName: "Demo User 2"},
	}
	result := BatchUserInfoDO2DTO(dos)
	assert.Len(t, result, 2)
	assert.Equal(t, "user-1", result[0].GetUserID())
	assert.Equal(t, "user-2", result[1].GetUserID())
}
