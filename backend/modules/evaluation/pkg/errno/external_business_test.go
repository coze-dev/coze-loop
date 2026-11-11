// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package errno

import (
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
)

func TestExternalBusinessCodeRegistration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    int32
		number  int32
		message string
		stable  bool
	}{
		{"service account", ExtBizSAAuthorizationCode, 601207102, "external business service-account authorization is required", true},
		{"evaluation access", ExtBizEvaluationAccessCode, 601207103, "external business evaluation resources or capabilities are unavailable", true},
		{"analysis template", ExtBizAnalysisTemplateCode, 601207104, "external business analysis template is unavailable", true},
		{"check failure", ExtBizAccessCheckFailedCode, 601207701, "external business access check failed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.number, tc.code)
			err := errorx.NewByCode(tc.code)
			status, ok := errorx.FromStatusError(err)
			require.True(t, ok)
			require.Equal(t, tc.number, status.Code())
			wire, ok := kerrors.FromBizStatusError(err)
			require.True(t, ok)
			affect := "1"
			if tc.stable {
				affect = "0"
			}
			require.Equal(t, affect, wire.BizExtra()["biz_err_affect_stability"])
			require.Contains(t, err.Error(), tc.message)
		})
	}
}
