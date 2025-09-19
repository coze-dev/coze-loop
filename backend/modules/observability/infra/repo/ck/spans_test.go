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
						FieldName: "manual_feedback_abc",
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"123"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
					},
				},
			},
			expectedSql: "SELECT * FROM `observability_spans` WHERE span_id in (SELECT span_id FROM `observability_annotations` WHERE (annotation_type = 'manual_feedback' AND key = 'abc' AND value_string IN ('123')) AND deleted_at = 0 AND start_time >= 1 AND start_time <= 2 SETTINGS final = 1) AND start_time >= 1 AND start_time <= 2 LIMIT 100",
		},
	}
	for _, tc := range testCases {
		qDb, err := new(SpansCkDaoImpl).buildSingleSql(context.Background(), &buildSqlParam{
			spanTable: "observability_spans",
			annoTable: "observability_annotations",
			queryParam: &QueryParam{
				StartTime: 1,
				EndTime:   2,
				Filters:   tc.filter,
				Limit:     100,
			},
			db: db,
		})
		assert.Nil(t, err)
		sql := qDb.ToSQL(func(tx *gorm.DB) *gorm.DB {
			return tx.Find([]*model.ObservabilitySpan{})
		})
		t.Log(sql)
		assert.Equal(t, tc.expectedSql, sql)
	}
}
