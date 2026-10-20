// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	daomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func TestManualRetryFrozenConfigRejectsCorruption(t *testing.T) {
	dao := daomocks.NewMockIExptItemRefDAO(gomock.NewController(t))
	dao.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(9), int64(7), int64(11)).Return(&model.ExptItemRef{ItemConfig: gptr.Of([]byte(`{`))}, nil)
	r := &ExptItemRefRepoImpl{dao: dao}
	_, err := r.GetByExptIDAndItemID(context.Background(), 9, 7, 11)
	require.Error(t, err, "corrupt frozen config must abort preparation, not turn into an absent target")
}
