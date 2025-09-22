// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
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
		{
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"123"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `input` NOT like '%123%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
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

// TestQueryTypeEnumNotMatchSqlExceptionCases 测试 QueryTypeEnumNotMatch 的SQL构建异常流程
func TestQueryTypeEnumNotMatchSqlExceptionCases(t *testing.T) {
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
		name        string
		filter      *loop_span.FilterFields
		expectedSql string
		shouldError bool
	}

	testCases := []testCase{
		// 边界情况测试
		{
			name: "Empty values array should build valid SQL",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{}, // 空数组
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "",
			shouldError: true, // 空值应该返回错误
		},
		{
			name: "Multiple values should cause error",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"value1", "value2"}, // 多个值
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "",
			shouldError: true, // 多个值应该返回错误
		},
		{
			name: "Single value should work correctly",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"test_value"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `input` NOT like '%test_value%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
			shouldError: false,
		},
		{
			name: "Empty string value should work",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{""},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `input` NOT like '%%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
			shouldError: false,
		},
		// 特殊字符处理测试
		{
			name: "Special characters should be handled correctly",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"test'value"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `input` NOT like '%test''value%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
			shouldError: false,
		},
		{
			name: "SQL injection attempt should be escaped",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"'; DROP TABLE spans; --"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `input` NOT like '%''; DROP TABLE spans; --%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
			shouldError: false,
		},
		// 不同字段类型测试
		{
			name: "Custom string tag should work",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: "custom_tag",
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"tag_value"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE tags_string['custom_tag'] NOT like '%tag_value%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
			shouldError: false,
		},
		{
			name: "System field should work",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldSpanType,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"test_type"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `span_type` NOT like '%test_type%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
			shouldError: false,
		},
		// Unicode字符测试
		{
			name: "Unicode characters should work",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"测试数据"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE `input` NOT like '%测试数据%' AND start_time >= 1 AND start_time <= 2 LIMIT 100",
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			qDb, err := new(SpansCkDaoImpl).buildSingleSql(context.Background(), db, "observability_spans", &QueryParam{
				StartTime: 1,
				EndTime:   2,
				Filters:   tc.filter,
				Limit:     100,
			})

			if tc.shouldError {
				assert.Error(t, err, "Expected error for test case: %s", tc.name)
				return
			}

			assert.NoError(t, err, "Unexpected error for test case: %s", tc.name)
			sql := qDb.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Find([]*model.ObservabilitySpan{})
			})
			t.Logf("Test case: %s, Generated SQL: %s", tc.name, sql)
			assert.Equal(t, tc.expectedSql, sql, "SQL mismatch for test case: %s", tc.name)
		})
	}
}

// TestQueryTypeEnumNotMatchComplexScenarios 测试 QueryTypeEnumNotMatch 的复杂场景
func TestQueryTypeEnumNotMatchComplexScenarios(t *testing.T) {
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
		name        string
		filter      *loop_span.FilterFields
		expectedSql string
	}

	testCases := []testCase{
		{
			name: "NotMatch combined with other query types using AND",
			filter: &loop_span.FilterFields{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"error"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
					{
						FieldName: loop_span.SpanFieldSpanType,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"http_request"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE (`input` NOT like '%error%' AND `span_type` = 'http_request') AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			name: "NotMatch combined with other query types using OR",
			filter: &loop_span.FilterFields{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"success"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
					{
						FieldName: loop_span.SpanFieldStatusCode,
						FieldType: loop_span.FieldTypeLong,
						Values:    []string{"200"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE (`input` NOT like '%success%' OR `status_code` = 200) AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			name: "Multiple NotMatch conditions with AND",
			filter: &loop_span.FilterFields{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"error"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
					{
						FieldName: loop_span.SpanFieldOutput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"failed"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE (`input` NOT like '%error%' AND `output` NOT like '%failed%') AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
		{
			name: "NotMatch with nested SubFilter",
			filter: &loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldInput,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"test"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
					},
					{
						SubFilter: &loop_span.FilterFields{
							QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
							FilterFields: []*loop_span.FilterField{
								{
									FieldName: "custom_tag",
									FieldType: loop_span.FieldTypeString,
									Values:    []string{"debug"},
									QueryType: ptr.Of(loop_span.QueryTypeEnumNotMatch),
								},
								{
									FieldName: loop_span.SpanFieldStatusCode,
									FieldType: loop_span.FieldTypeLong,
									Values:    []string{"500"},
									QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
								},
							},
						},
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE (`input` NOT like '%test%' AND (tags_string['custom_tag'] NOT like '%debug%' OR `status_code` = 500)) AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			qDb, err := new(SpansCkDaoImpl).buildSingleSql(context.Background(), db, "observability_spans", &QueryParam{
				StartTime: 1,
				EndTime:   2,
				Filters:   tc.filter,
				Limit:     100,
			})
			assert.NoError(t, err, "Unexpected error for test case: %s", tc.name)
			sql := qDb.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Find([]*model.ObservabilitySpan{})
			})
			t.Logf("Test case: %s, Generated SQL: %s", tc.name, sql)
			assert.Equal(t, tc.expectedSql, sql, "SQL mismatch for test case: %s", tc.name)
		})
	}
}
