// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// EvaluatorTagDAO 定义 EvaluatorTag 的 Dao 接口
//
//go:generate mockgen -destination mocks/evaluator_tag_mock.go -package=mocks . EvaluatorTagDAO
type EvaluatorTagDAO interface {
	// BatchGetTagsBySourceIDsAndType 批量根据source_ids和tag_type筛选tag_key和tag_value
	BatchGetTagsBySourceIDsAndType(ctx context.Context, sourceIDs []int64, tagType int32, langType string, opts ...db.Option) ([]*model.EvaluatorTag, error)
	// GetSourceIDsByFilterConditions 根据筛选条件查询source_id列表，支持复杂的AND/OR逻辑和分页
	GetSourceIDsByFilterConditions(ctx context.Context, tagType int32, filterOption *entity.EvaluatorFilterOption, pageSize, pageNum int32, langType string, opts ...db.Option) ([]int64, int64, error)
	// AggregateTagValuesByType 根据 tag_type 聚合唯一的 tag_key、tag_value 组合
	AggregateTagValuesByType(ctx context.Context, tagType int32, langType string, opts ...db.Option) ([]*entity.AggregatedEvaluatorTag, error)
	// BatchCreateEvaluatorTags 批量创建评估器标签
	BatchCreateEvaluatorTags(ctx context.Context, evaluatorTags []*model.EvaluatorTag, opts ...db.Option) error
	// DeleteEvaluatorTagsByConditions 根据sourceID、tagType、tags条件删除标签
	DeleteEvaluatorTagsByConditions(ctx context.Context, sourceID int64, tagType int32, langType string, tags map[string][]string, opts ...db.Option) error
}

var (
	evaluatorTagDaoOnce      = sync.Once{}
	singletonEvaluatorTagDao EvaluatorTagDAO
)

// EvaluatorTagDAOImpl 实现 EvaluatorTagDAO 接口
type EvaluatorTagDAOImpl struct {
	provider db.Provider
}

func NewEvaluatorTagDAO(p db.Provider) EvaluatorTagDAO {
	evaluatorTagDaoOnce.Do(func() {
		singletonEvaluatorTagDao = &EvaluatorTagDAOImpl{
			provider: p,
		}
	})
	return singletonEvaluatorTagDao
}

// BatchGetTagsBySourceIDsAndType 批量根据source_ids和tag_type筛选tag_key和tag_value
func (dao *EvaluatorTagDAOImpl) BatchGetTagsBySourceIDsAndType(ctx context.Context, sourceIDs []int64, tagType int32, langType string, opts ...db.Option) ([]*model.EvaluatorTag, error) {
	if len(sourceIDs) == 0 {
		return []*model.EvaluatorTag{}, nil
	}

	dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)

	var tags []*model.EvaluatorTag
	query := dbsession.WithContext(ctx).
		Where("source_id IN (?) AND tag_type = ?", sourceIDs, tagType).
		Where("deleted_at IS NULL")
	if langType != "" {
		query = query.Where("lang_type = ?", langType)
	}
	err := query.
		Find(&tags).Error
	if err != nil {
		return nil, err
	}

	return tags, nil
}

// AggregateTagValuesByType 根据 tag_type 聚合唯一的 tag_key、tag_value 组合
func (dao *EvaluatorTagDAOImpl) AggregateTagValuesByType(ctx context.Context, tagType int32, langType string, opts ...db.Option) ([]*entity.AggregatedEvaluatorTag, error) {
	dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)

	query := dbsession.WithContext(ctx).
		Table(model.TableNameEvaluatorTag).
		Select("tag_key, tag_value").
		Where("tag_type = ?", tagType).
		Where("deleted_at IS NULL")
	if langType != "" {
		query = query.Where("lang_type = ?", langType)
	}

	var result []*entity.AggregatedEvaluatorTag
	err := query.
		Group("tag_key, tag_value").
		Find(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []*entity.AggregatedEvaluatorTag{}, nil
		}
		return nil, err
	}
	return result, nil
}

// BatchCreateEvaluatorTags 批量创建评估器标签
func (dao *EvaluatorTagDAOImpl) BatchCreateEvaluatorTags(ctx context.Context, evaluatorTags []*model.EvaluatorTag, opts ...db.Option) error {
	if len(evaluatorTags) == 0 {
		return nil
	}

	dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)
	return dbsession.WithContext(ctx).CreateInBatches(evaluatorTags, 100).Error
}

// DeleteEvaluatorTagsByConditions 根据sourceID、tagType、tags条件删除标签
func (dao *EvaluatorTagDAOImpl) DeleteEvaluatorTagsByConditions(ctx context.Context, sourceID int64, tagType int32, langType string, tags map[string][]string, opts ...db.Option) error {
	dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)

	// 基础查询条件
	query := dbsession.WithContext(ctx).
		Where("source_id = ? AND tag_type = ?", sourceID, tagType).
		Where("deleted_at IS NULL")
	if langType != "" {
		query = query.Where("lang_type = ?", langType)
	}

	// 如果有指定tags条件，则添加额外的删除条件
	if len(tags) > 0 {
		// 构建OR条件组，每个tag_key对应一个条件组
		var conditions []string
		var args []interface{}

		for tagKey, tagValues := range tags {
			if len(tagValues) == 0 {
				continue
			}
			// 对于每个tag_key，tag_value可以是多个值中的任意一个
			conditions = append(conditions, "(tag_key = ? AND tag_value IN (?))")
			args = append(args, tagKey, tagValues)
		}

		// 如果有标签条件，使用OR条件组合
		if len(conditions) > 0 {
			orCondition := "(" + strings.Join(conditions, " OR ") + ")"
			query = query.Where(orCondition, args...)
		}
	}

	return query.Delete(&model.EvaluatorTag{}).Error
}

// sourceIDInChunkSize 单次 IN 查询中 source_id 个数上限，避免包体过大
const sourceIDInChunkSize = 1000

// GetSourceIDsByFilterConditions 根据筛选条件查询 source_id 列表，支持 AND/OR 与分页。
// 实现上避免自连接/JOIN：拆成多次单表查询 + 内存集合运算，兼容不支持复杂 JOIN 的存储引擎。
func (dao *EvaluatorTagDAOImpl) GetSourceIDsByFilterConditions(ctx context.Context, tagType int32, filterOption *entity.EvaluatorFilterOption, pageSize, pageNum int32, langType string, opts ...db.Option) ([]int64, int64, error) {
	if filterOption == nil {
		filterOption = &entity.EvaluatorFilterOption{}
	}

	hasFilters := filterOption.Filters != nil && !evaluatorFiltersEmpty(filterOption.Filters)
	hasSearch := filterOption.SearchKeyword != nil && *filterOption.SearchKeyword != ""

	var ids []int64
	var err error
	if hasFilters {
		ids, err = dao.evalFiltersToSourceIDs(ctx, tagType, langType, filterOption.Filters, opts...)
	} else if hasSearch {
		// 仅有 Name 关键词搜索：直接按 Name LIKE 查 source_id，避免先全表 DISTINCT
		ids, err = dao.sourceIDsForNameLike(ctx, tagType, langType, *filterOption.SearchKeyword, nil, opts...)
		hasSearch = false
	} else {
		ids, err = dao.allDistinctSourceIDs(ctx, tagType, langType, opts...)
	}
	if err != nil {
		return nil, 0, err
	}

	if hasSearch {
		ids, err = dao.sourceIDsForNameLike(ctx, tagType, langType, *filterOption.SearchKeyword, ids, opts...)
		if err != nil {
			return nil, 0, err
		}
	}

	total := int64(len(ids))
	if total == 0 {
		return []int64{}, 0, nil
	}

	ids, err = dao.sortSourceIDsByNameTag(ctx, tagType, langType, ids, opts...)
	if err != nil {
		return nil, 0, err
	}

	logs.CtxInfo(ctx, "[GetSourceIDsByFilterConditions] matched source_id count=%d (no JOIN path)", total)

	var limit, offset int
	if pageSize > 0 && pageNum > 0 {
		limit = int(pageSize)
		offset = int((pageNum - 1) * pageSize)
		if offset >= len(ids) {
			return []int64{}, total, nil
		}
		end := offset + limit
		if end > len(ids) {
			end = len(ids)
		}
		return ids[offset:end], total, nil
	}

	return ids, total, nil
}

func evaluatorFiltersEmpty(f *entity.EvaluatorFilters) bool {
	if f == nil {
		return true
	}
	return len(f.FilterConditions) == 0 && len(f.SubFilters) == 0
}

func (dao *EvaluatorTagDAOImpl) allDistinctSourceIDs(ctx context.Context, tagType int32, langType string, opts ...db.Option) ([]int64, error) {
	dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)
	q := dbsession.WithContext(ctx).
		Table(model.TableNameEvaluatorTag).
		Distinct("source_id").
		Where("tag_type = ? AND deleted_at IS NULL", tagType)
	if langType != "" {
		q = q.Where("lang_type = ?", langType)
	}
	var ids []int64
	if err := q.Pluck("source_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// evalFiltersToSourceIDs 递归求满足 Filters 的 source_id（无 JOIN）
func (dao *EvaluatorTagDAOImpl) evalFiltersToSourceIDs(ctx context.Context, tagType int32, langType string, filters *entity.EvaluatorFilters, opts ...db.Option) ([]int64, error) {
	if filters == nil || evaluatorFiltersEmpty(filters) {
		return dao.allDistinctSourceIDs(ctx, tagType, langType, opts...)
	}

	isOr := filters.LogicOp != nil && *filters.LogicOp == entity.FilterLogicOp_Or
	if isOr {
		set := make(map[int64]struct{})
		for _, c := range filters.FilterConditions {
			if c == nil {
				continue
			}
			part, err := dao.querySourceIDsForCondition(ctx, tagType, langType, c, nil, opts...)
			if err != nil {
				return nil, err
			}
			for _, id := range part {
				set[id] = struct{}{}
			}
		}
		for _, sub := range filters.SubFilters {
			part, err := dao.evalFiltersToSourceIDs(ctx, tagType, langType, sub, opts...)
			if err != nil {
				return nil, err
			}
			for _, id := range part {
				set[id] = struct{}{}
			}
		}
		return mapKeysToSlice(set), nil
	}

	var s []int64
	for _, c := range filters.FilterConditions {
		if c == nil {
			continue
		}
		var part []int64
		var err error
		if s == nil {
			part, err = dao.querySourceIDsForCondition(ctx, tagType, langType, c, nil, opts...)
		} else {
			part, err = dao.querySourceIDsForCondition(ctx, tagType, langType, c, s, opts...)
		}
		if err != nil {
			return nil, err
		}
		s = part
		if len(s) == 0 {
			return s, nil
		}
	}
	for _, sub := range filters.SubFilters {
		subIDs, err := dao.evalFiltersToSourceIDs(ctx, tagType, langType, sub, opts...)
		if err != nil {
			return nil, err
		}
		if s == nil {
			s = subIDs
		} else {
			s = intersectInt64Slice(s, subIDs)
		}
		if len(s) == 0 {
			return s, nil
		}
	}
	if s == nil {
		return dao.allDistinctSourceIDs(ctx, tagType, langType, opts...)
	}
	return s, nil
}

func mapKeysToSlice(m map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out
}

func intersectInt64Slice(a, b []int64) []int64 {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	m := make(map[int64]struct{}, len(a))
	for _, id := range a {
		m[id] = struct{}{}
	}
	out := make([]int64, 0)
	for _, id := range b {
		if _, ok := m[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func chunkInt64Slice(ids []int64, chunk int) [][]int64 {
	if chunk <= 0 {
		chunk = sourceIDInChunkSize
	}
	var out [][]int64
	for i := 0; i < len(ids); i += chunk {
		j := i + chunk
		if j > len(ids) {
			j = len(ids)
		}
		out = append(out, ids[i:j])
	}
	return out
}

func (dao *EvaluatorTagDAOImpl) querySourceIDsForCondition(ctx context.Context, tagType int32, langType string, condition *entity.EvaluatorFilterCondition, restrictTo []int64, opts ...db.Option) ([]int64, error) {
	if condition == nil {
		return nil, nil
	}
	if restrictTo != nil {
		if len(restrictTo) == 0 {
			return nil, nil
		}
	}
	if restrictTo == nil || len(restrictTo) <= sourceIDInChunkSize {
		return dao.querySourceIDsForConditionOnce(ctx, tagType, langType, condition, restrictTo, opts...)
	}
	set := make(map[int64]struct{})
	for _, ch := range chunkInt64Slice(restrictTo, sourceIDInChunkSize) {
		part, err := dao.querySourceIDsForConditionOnce(ctx, tagType, langType, condition, ch, opts...)
		if err != nil {
			return nil, err
		}
		for _, id := range part {
			set[id] = struct{}{}
		}
	}
	return mapKeysToSlice(set), nil
}

func (dao *EvaluatorTagDAOImpl) querySourceIDsForConditionOnce(ctx context.Context, tagType int32, langType string, condition *entity.EvaluatorFilterCondition, restrictTo []int64, opts ...db.Option) ([]int64, error) {
	dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)
	q := dbsession.WithContext(ctx).
		Table(model.TableNameEvaluatorTag).
		Distinct("source_id").
		Where("tag_type = ? AND deleted_at IS NULL", tagType)
	if langType != "" {
		q = q.Where("lang_type = ?", langType)
	}
	condSQL, condArgs, err := dao.buildSingleCondition(condition)
	if err != nil {
		return nil, err
	}
	if condSQL != "" {
		q = q.Where(condSQL, condArgs...)
	}
	if restrictTo != nil {
		q = q.Where("source_id IN ?", restrictTo)
	}
	var ids []int64
	if err := q.Pluck("source_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (dao *EvaluatorTagDAOImpl) sourceIDsForNameLike(ctx context.Context, tagType int32, langType string, keyword string, restrictTo []int64, opts ...db.Option) ([]int64, error) {
	kw := "%" + keyword + "%"
	if restrictTo != nil {
		if len(restrictTo) == 0 {
			return nil, nil
		}
	}
	if restrictTo == nil || len(restrictTo) <= sourceIDInChunkSize {
		return dao.sourceIDsForNameLikeOnce(ctx, tagType, langType, kw, restrictTo, opts...)
	}
	set := make(map[int64]struct{})
	for _, ch := range chunkInt64Slice(restrictTo, sourceIDInChunkSize) {
		part, err := dao.sourceIDsForNameLikeOnce(ctx, tagType, langType, kw, ch, opts...)
		if err != nil {
			return nil, err
		}
		for _, id := range part {
			set[id] = struct{}{}
		}
	}
	return mapKeysToSlice(set), nil
}

func (dao *EvaluatorTagDAOImpl) sourceIDsForNameLikeOnce(ctx context.Context, tagType int32, langType string, keywordPattern string, restrictTo []int64, opts ...db.Option) ([]int64, error) {
	dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)
	q := dbsession.WithContext(ctx).
		Table(model.TableNameEvaluatorTag).
		Distinct("source_id").
		Where("tag_type = ? AND tag_key = ? AND tag_value LIKE ? AND deleted_at IS NULL", tagType, "Name", keywordPattern)
	if langType != "" {
		q = q.Where("lang_type = ?", langType)
	}
	if restrictTo != nil {
		q = q.Where("source_id IN ?", restrictTo)
	}
	var ids []int64
	if err := q.Pluck("source_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// sortSourceIDsByNameTag 按 Name 标签值升序排序；无 Name 行的 source 排在后面（与原 LEFT JOIN 语义一致）
func (dao *EvaluatorTagDAOImpl) sortSourceIDsByNameTag(ctx context.Context, tagType int32, langType string, ids []int64, opts ...db.Option) ([]int64, error) {
	if len(ids) <= 1 {
		return ids, nil
	}
	nameOf := make(map[int64]string, len(ids))
	for _, ch := range chunkInt64Slice(ids, sourceIDInChunkSize) {
		dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)
		q := dbsession.WithContext(ctx).
			Table(model.TableNameEvaluatorTag).
			Select("source_id", "tag_value").
			Where("source_id IN ? AND tag_type = ? AND tag_key = ? AND deleted_at IS NULL", ch, tagType, "Name")
		if langType != "" {
			q = q.Where("lang_type = ?", langType)
		}
		var rows []model.EvaluatorTag
		if err := q.Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			if prev, ok := nameOf[r.SourceID]; !ok || r.TagValue < prev {
				nameOf[r.SourceID] = r.TagValue
			}
		}
	}
	out := append([]int64(nil), ids...)
	sort.Slice(out, func(i, j int) bool {
		ni, oki := nameOf[out[i]]
		nj, okj := nameOf[out[j]]
		if oki != okj {
			return oki
		}
		if !oki {
			return false
		}
		return ni < nj
	})
	return out, nil
}

// buildFilterConditions 构建筛选条件的SQL和参数
// nolint:unused // 保留备用：复杂筛选条件的 SQL 生成
func (dao *EvaluatorTagDAOImpl) buildFilterConditions(filters *entity.EvaluatorFilters) (string, []interface{}, error) {
	if filters == nil {
		return "", nil, nil
	}

	var conditions []string
	var args []interface{}

	// 1) 本层条件
	if len(filters.FilterConditions) > 0 {
		for _, condition := range filters.FilterConditions {
			conditionSQL, conditionArgs, err := dao.buildSingleCondition(condition)
			if err != nil {
				return "", nil, err
			}
			if conditionSQL != "" {
				// 将每个原子条件独立包裹括号，便于与子条件并列组合
				conditions = append(conditions, "("+conditionSQL+")")
				args = append(args, conditionArgs...)
			}
		}
	}

	// 2) 递归子条件组
	if len(filters.SubFilters) > 0 {
		for _, sub := range filters.SubFilters {
			subSQL, subArgs, err := dao.buildFilterConditions(sub)
			if err != nil {
				return "", nil, err
			}
			if subSQL != "" {
				// 子条件整体也以括号包裹，与当前层条件并列
				conditions = append(conditions, "("+subSQL+")")
				args = append(args, subArgs...)
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil, nil
	}

	// 根据逻辑操作符组合条件：直接使用分隔符合并，不再整体再包一层括号
	if filters.LogicOp != nil && *filters.LogicOp == entity.FilterLogicOp_Or {
		return strings.Join(conditions, " OR "), args, nil
	}
	// 默认为 AND
	return strings.Join(conditions, " AND "), args, nil
}

// buildSingleCondition 构建单个筛选条件的SQL和参数
func (dao *EvaluatorTagDAOImpl) buildSingleCondition(condition *entity.EvaluatorFilterCondition) (string, []interface{}, error) {
	if condition == nil {
		return "", nil, nil
	}

	tagKey := string(condition.TagKey)
	operator := condition.Operator
	value := condition.Value

	switch operator {
	case entity.EvaluatorFilterOperatorType_Equal:
		return "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value = ?", []interface{}{tagKey, value}, nil

	case entity.EvaluatorFilterOperatorType_NotEqual:
		return "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value != ?", []interface{}{tagKey, value}, nil

	case entity.EvaluatorFilterOperatorType_In:
		// 将value按逗号分割
		values := strings.Split(value, ",")
		if len(values) == 0 {
			return "", nil, fmt.Errorf("IN operator requires non-empty values")
		}
		placeholders := strings.Repeat("?,", len(values))
		placeholders = placeholders[:len(placeholders)-1] // 移除最后的逗号
		return fmt.Sprintf("evaluator_tag.tag_key = ? AND evaluator_tag.tag_value IN (%s)", placeholders),
			append([]interface{}{tagKey}, convertToInterfaceSlice(values)...), nil

	case entity.EvaluatorFilterOperatorType_NotIn:
		// 将value按逗号分割
		values := strings.Split(value, ",")
		if len(values) == 0 {
			return "", nil, fmt.Errorf("NOT_IN operator requires non-empty values")
		}
		placeholders := strings.Repeat("?,", len(values))
		placeholders = placeholders[:len(placeholders)-1] // 移除最后的逗号
		return fmt.Sprintf("evaluator_tag.tag_key = ? AND evaluator_tag.tag_value NOT IN (%s)", placeholders),
			append([]interface{}{tagKey}, convertToInterfaceSlice(values)...), nil

	case entity.EvaluatorFilterOperatorType_Like:
		likeValue := "%" + value + "%"
		return "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value LIKE ?", []interface{}{tagKey, likeValue}, nil

	case entity.EvaluatorFilterOperatorType_IsNull:
		return "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value IS NULL", []interface{}{tagKey}, nil

	case entity.EvaluatorFilterOperatorType_IsNotNull:
		return "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value IS NOT NULL", []interface{}{tagKey}, nil

	default:
		return "", nil, fmt.Errorf("unsupported operator type: %v", operator)
	}
}

// convertToInterfaceSlice 将字符串切片转换为interface{}切片
func convertToInterfaceSlice(strs []string) []interface{} {
	result := make([]interface{}, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}
