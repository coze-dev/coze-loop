// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestSharedResourceInfoConverters(t *testing.T) {
	assert.Nil(t, SharedResourceInfoDTO2DO(nil))
	assert.Nil(t, SharedResourceInfoDO2DTO(nil))

	dto := &commondto.SharedResourceInfo{
		IsShared:      gptr.Of(true),
		SourceSpaceID: gptr.Of(int64(100)),
		AccessLevel:   gptr.Of(entity.SharedAccessLevelReadable),
		VersionPolicy: gptr.Of(entity.SharedVersionPolicySpecified),
	}
	domain := SharedResourceInfoDTO2DO(dto)
	require.NotNil(t, domain)
	assert.True(t, domain.IsShared)
	assert.Equal(t, int64(100), domain.SourceSpaceID)
	assert.Equal(t, entity.SharedAccessLevelReadable, domain.AccessLevel)
	assert.Equal(t, entity.SharedVersionPolicySpecified, domain.VersionPolicy)

	converted := SharedResourceInfoDO2DTO(domain)
	require.NotNil(t, converted)
	assert.True(t, converted.GetIsShared())
	assert.Equal(t, int64(100), converted.GetSourceSpaceID())
	assert.Equal(t, entity.SharedAccessLevelReadable, converted.GetAccessLevel())
	assert.Equal(t, entity.SharedVersionPolicySpecified, converted.GetVersionPolicy())
}
