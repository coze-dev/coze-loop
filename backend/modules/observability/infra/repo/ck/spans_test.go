// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	ck_mock "github.com/coze-dev/coze-loop/backend/infra/ck/mocks"
	metrics_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

func TestBuildSql(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal("Failed to create mock")
	}
	defer func() {
		_ = sqlDB.Close()
	}()
	// 用mock DB替换GORM的DB
	db, err := gorm.Open(clickhouse.New(clickhouse.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	type testCase struct {
		filter      *loop_span.FilterFields
		expectedSql string
	}
	testCases := []testCase{
		{
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: "a",
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"1"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
						SubFilter: &loop_span.FilterFields{
							FilterFields: []*loop_span.FilterField{
								{
									FieldName:  "aa",
									FieldType:  loop_span.FieldTypeString,
									Values:     []string{"aaa"},
									QueryType:  ptr.Of(loop_span.QueryTypeEnumIn),
									QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
									SubFilter: &loop_span.FilterFields{
										FilterFields: []*loop_span.FilterField{
											{
												FieldName: "a",
												FieldType: loop_span.FieldTypeString,
												Values:    []string{"b"},
												QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
											},
										},
									},
								},
							},
						},
					},
					{
						FieldName:  "b",
						FieldType:  loop_span.FieldTypeString,
						Values:     []string{"b"},
						QueryType:  ptr.Of(loop_span.QueryTypeEnumNotIn),
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
						SubFilter: &loop_span.FilterFields{
							FilterFields: []*loop_span.FilterField{
								{
									FieldName: "c",
									FieldType: loop_span.FieldTypeString,
									Values:    []string{"c"},
									QueryType: ptr.Of(loop_span.QueryTypeEnumNotIn),
								},
								{
									FieldName: "c",
									FieldType: loop_span.FieldTypeString,
									Values:    []string{"d"},
									QueryType: ptr.Of(loop_span.QueryTypeEnumNotIn),
								},
								{
									FieldName: "c",
									FieldType: loop_span.FieldTypeString,
									Values:    []string{"e"},
									QueryType: ptr.Of(loop_span.QueryTypeEnumNotIn),
								},
							},
						},
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE ((tags_string['a'] IN ('1') AND (tags_string['aa'] IN ('aaa') OR tags_string['a'] = 'b')) AND (tags_string['b'] NOT IN ('b') OR (tags_string['c'] NOT IN ('c') AND tags_string['c'] NOT IN ('d') AND tags_string['c'] NOT IN ('e')))) AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: "custom_tag_string",
						FieldType: loop_span.FieldTypeString,
						Values:    []string{},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotExist),
					},
					{
						FieldName: "custom_tag_bool",
						FieldType: loop_span.FieldTypeBool,
						Values:    []string{},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotExist),
					},
					{
						FieldName: "custom_tag_double",
						FieldType: loop_span.FieldTypeDouble,
						Values:    []string{},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotExist),
					},
					{
						FieldName: "custom_tag_long",
						FieldType: loop_span.FieldTypeLong,
						Values:    []string{},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotExist),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE ((tags_string['custom_tag_string'] IS NULL OR tags_string['custom_tag_string'] = '') AND (tags_bool['custom_tag_bool'] IS NULL OR tags_bool['custom_tag_bool'] = 0) AND (tags_float['custom_tag_double'] IS NULL OR tags_float['custom_tag_double'] = 0) AND (tags_long['custom_tag_long'] IS NULL OR tags_long['custom_tag_long'] = 0)) AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: "custom_tag_long",
						FieldType: loop_span.FieldTypeLong,
						Values:    []string{"123", "-123"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE tags_long['custom_tag_long'] IN (123,-123) AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: "custom_tag_float64",
						FieldType: loop_span.FieldTypeDouble,
						Values:    []string{"123.999"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE tags_float['custom_tag_float64'] = 123.999 AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldDuration,
						FieldType: loop_span.FieldTypeLong,
						Values:    []string{"121"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumGte),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `duration` >= 121 AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: "custom_tag_bool",
						FieldType: loop_span.FieldTypeBool,
						Values:    []string{"true"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE tags_bool['custom_tag_bool'] = 1 AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"123"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `input` like '%123%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
	}
	for _, tc := range testCases {
		qDb, err := new(SpansCkDaoImpl).buildSingleSql(context.Background(), db, "observability_spans", &QueryParam{
			StartTime: 1,
			EndTime:   2,
			Filters:   tc.filter,
			Limit:     100,
		})
		assert.Nil(t, err)
		sql := qDb.ToSQL(func(tx *gorm.DB) *gorm.DB {
			return tx.Find([]*model.ObservabilitySpan{})
		})
		t.Log(sql)
		assert.Equal(t, tc.expectedSql, sql)
	}
}

func TestMetricSql(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal("Failed to create mock")
	}
	defer func() {
		_ = sqlDB.Close()
	}()
	// 用mock DB替换GORM的DB
	db, err := gorm.Open(clickhouse.New(clickhouse.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	type testCase struct {
		param       *GetMetricsParam
		expectedSql string
	}
	testCases := []testCase{
		{
			param: &GetMetricsParam{
				Tables: []string{"observability_spans"},
				Aggregations: []*metrics_entity.Dimension{
					{
						Alias:      "total_count",
						Expression: "count()",
					},
					{
						Alias:      "total_error_count",
						Expression: "countIf(status_code != 0)",
					},
				},
				Filters: &loop_span.FilterFields{
					QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
					FilterFields: []*loop_span.FilterField{
						{
							FieldName: loop_span.SpanFieldSpaceId,
							FieldType: loop_span.FieldTypeString,
							Values:    []string{"123"},
							QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
						},
					},
				},
				StartAt: 1,
				EndAt:   2,
			},
			expectedSql: "SELECT count() AS total_count, countIf(status_code != 0) AS total_error_count FROM (SELECT * FROM `observability_spans` WHERE `space_id` = '123' AND start_time >= 1 AND start_time <= 2 )  ",
		},
		{
			param: &GetMetricsParam{
				Tables: []string{"observability_spans"},
				Aggregations: []*metrics_entity.Dimension{
					{
						Alias:      "total_count",
						Expression: "count()",
					},
					{
						Alias:      "total_error_count",
						Expression: "countIf(status_code != 0)",
					},
				},
				Filters: &loop_span.FilterFields{
					QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
					FilterFields: []*loop_span.FilterField{
						{
							FieldName: loop_span.SpanFieldSpaceId,
							FieldType: loop_span.FieldTypeString,
							Values:    []string{"123"},
							QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
						},
					},
				},
				StartAt:     1,
				EndAt:       2,
				Granularity: metrics_entity.MetricGranularity1Min,
			},
			expectedSql: "SELECT toStartOfInterval(fromUnixTimestamp64Micro(start_time), INTERVAL 5 MINUTE) AS time_bucket, count() AS total_count, countIf(status_code != 0) AS total_error_count FROM (SELECT * FROM `observability_spans` WHERE `space_id` = '123' AND start_time >= 1 AND start_time <= 2 ) GROUP BY time_bucket ORDER BY time_bucket WITH FILL FROM toStartOfInterval(fromUnixTimestamp64Micro(1), INTERVAL 5 MINUTE) TO toStartOfInterval(fromUnixTimestamp64Micro(2), INTERVAL 5 MINUTE) STEP INTERVAL 5 MINUTE",
		},
		{
			param: &GetMetricsParam{
				Tables: []string{"observability_spans"},
				Aggregations: []*metrics_entity.Dimension{
					{
						Alias:      "total_count",
						Expression: "count()",
					},
					{
						Alias:      "total_error_count",
						Expression: "countIf(status_code != 0)",
					},
				},
				Filters: &loop_span.FilterFields{
					QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
					FilterFields: []*loop_span.FilterField{
						{
							FieldName: loop_span.SpanFieldSpaceId,
							FieldType: loop_span.FieldTypeString,
							Values:    []string{"123"},
							QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
						},
					},
				},
				GroupBys: []*metrics_entity.Dimension{
					{
						Alias:      "val",
						Expression: "tags_string['prompt_key']",
					},
				},
				StartAt:     1,
				EndAt:       2,
				Granularity: metrics_entity.MetricGranularity1Min,
			},
			expectedSql: "SELECT toStartOfInterval(fromUnixTimestamp64Micro(start_time), INTERVAL 5 MINUTE) AS time_bucket, count() AS total_count, countIf(status_code != 0) AS total_error_count, tags_string['prompt_key'] AS val FROM (SELECT * FROM `observability_spans` WHERE `space_id` = '123' AND start_time >= 1 AND start_time <= 2 ) GROUP BY time_bucket, tags_string['prompt_key'] ORDER BY time_bucket WITH FILL FROM toStartOfInterval(fromUnixTimestamp64Micro(1), INTERVAL 5 MINUTE) TO toStartOfInterval(fromUnixTimestamp64Micro(2), INTERVAL 5 MINUTE) STEP INTERVAL 5 MINUTE",
		},
	}
	ckProvider := ck_mock.NewMockProvider(gomock.NewController(t))
	ckProvider.EXPECT().NewSession(gomock.Any()).Return(db).AnyTimes()
	for _, tc := range testCases {
		impl := &SpansCkDaoImpl{
			db: ckProvider,
		}
		sql, err := impl.buildMetricsSql(context.Background(), tc.param)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, tc.expectedSql, sql)
	}
}
