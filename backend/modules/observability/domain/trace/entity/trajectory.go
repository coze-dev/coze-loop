package entity

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/samber/lo"
)

type TrajectoryConfig struct {
	ID          int64
	WorkspaceID int64
	Filter      *loop_span.FilterFields
	CreatedAt   time.Time
	CreatedBy   string
	UpdatedAt   time.Time
	UpdatedBy   string
}

func (t *TrajectoryConfig) GetFilter() *loop_span.FilterFields {
	filters := &loop_span.FilterFields{
		QueryAndOr:   lo.ToPtr(loop_span.QueryAndOrEnumOr),
		FilterFields: make([]*loop_span.FilterField, 0),
	}

	filters.FilterFields = append(filters.FilterFields,
		&loop_span.FilterField{
			FieldName:  "parent_id",
			FieldType:  loop_span.FieldTypeString,
			Values:     []string{"", "0"},
			QueryType:  lo.ToPtr(loop_span.QueryTypeEnumIn),
			QueryAndOr: lo.ToPtr(loop_span.QueryAndOrEnumOr),
		},
	)

	if t.Filter != nil {
		filters.FilterFields = append(filters.FilterFields, &loop_span.FilterField{
			SubFilter: t.Filter,
		})
	}
	return filters
}
