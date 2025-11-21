// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	timeutil "github.com/coze-dev/coze-loop/backend/pkg/time"
)

func ToJSONString(ctx context.Context, obj interface{}) string {
	if obj == nil {
		return ""
	}
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		logs.CtxError(ctx, "JSON marshal error: %v", err)
		return ""
	}
	jsonStr := string(jsonData)
	return jsonStr
}

func (h *TraceHubServiceImpl) getTenants(ctx context.Context, platform loop_span.PlatformType) ([]string, error) {
	return h.tenantProvider.GetTenantsByPlatformType(ctx, platform)
}

func processSpecificFilter(f *loop_span.FilterField) error {
	switch f.FieldName {
	case loop_span.SpanFieldStatus:
		if err := processStatusFilter(f); err != nil {
			return err
		}
	case loop_span.SpanFieldDuration,
		loop_span.SpanFieldLatencyFirstResp,
		loop_span.SpanFieldStartTimeFirstResp,
		loop_span.SpanFieldStartTimeFirstTokenResp,
		loop_span.SpanFieldLatencyFirstTokenResp,
		loop_span.SpanFieldReasoningDuration:
		if err := processLatencyFilter(f); err != nil {
			return err
		}
	}
	return nil
}
func processStatusFilter(f *loop_span.FilterField) error {
	if f.QueryType == nil || *f.QueryType != loop_span.QueryTypeEnumIn {
		return fmt.Errorf("status filter should use in operator")
	}
	f.FieldName = loop_span.SpanFieldStatusCode
	f.FieldType = loop_span.FieldTypeLong
	checkSuccess, checkError := false, false
	for _, val := range f.Values {
		switch val {
		case loop_span.SpanStatusSuccess:
			checkSuccess = true
		case loop_span.SpanStatusError:
			checkError = true
		default:
			return fmt.Errorf("invalid status code field value")
		}
	}
	if checkSuccess && checkError {
		f.QueryType = ptr.Of(loop_span.QueryTypeEnumAlwaysTrue)
		f.Values = nil
	} else if checkSuccess {
		f.Values = []string{"0"}
	} else if checkError {
		f.QueryType = ptr.Of(loop_span.QueryTypeEnumNotIn)
		f.Values = []string{"0"}
	} else {
		return fmt.Errorf("invalid status code query")
	}
	return nil
}

// ms -> us
func processLatencyFilter(f *loop_span.FilterField) error {
	if f.FieldType != loop_span.FieldTypeLong {
		return fmt.Errorf("latency field type should be long ")
	}
	micros := make([]string, 0)
	for _, val := range f.Values {
		integer, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("fail to parse long value %s, %v", val, err)
		}
		integer = timeutil.MillSec2MicroSec(integer)
		micros = append(micros, strconv.FormatInt(integer, 10))
	}
	f.Values = micros
	return nil
}
