// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookProjectionSchemaUnicodeCodeRoundTrip(t *testing.T) {
	for _, code := range []string{"LEGACY_ASCII", "资源未就绪", strings.Repeat("😀", 32)} {
		t.Run(code[:min(len(code), 12)], func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			_, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.NoError(t, err)
			err = f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=? AND space_id=? AND expt_id=?", in.Before.OperationID, f.space, f.expt).UpdateColumns(map[string]any{"error_code": code, "error_message": "请稍后重试😀"}).Error
			require.NoError(t, err)
			var row model.ExptLifecycleHookRun
			require.NoError(t, f.sql.Where("operation_id=? AND space_id=? AND expt_id=?", in.Before.OperationID, f.space, f.expt).First(&row).Error)
			require.Equal(t, code, gptr.Indirect(row.ErrorCode))
			require.Equal(t, "请稍后重试😀", gptr.Indirect(row.ErrorMessage))
		})
	}
}
