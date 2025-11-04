// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
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

// GetSourceIDsByFilterConditions 根据筛选条件查询source_id列表，支持复杂的AND/OR逻辑和分页
func (dao *EvaluatorTagDAOImpl) GetSourceIDsByFilterConditions(ctx context.Context, tagType int32, filterOption *entity.EvaluatorFilterOption, pageSize, pageNum int32, langType string, opts ...db.Option) ([]int64, int64, error) {
	if filterOption == nil {
		return []int64{}, 0, nil
	}

    dbsession := dao.provider.NewSession(ctx, append(opts, db.Debug())...)

	// 基础查询条件
	query := dbsession.WithContext(ctx).Table("evaluator_tag").
		Select("evaluator_tag.source_id").
		Where("evaluator_tag.tag_type = ?", tagType).
		Where("evaluator_tag.deleted_at IS NULL")
	if langType != "" {
		query = query.Where("evaluator_tag.lang_type = ?", langType)
	}

	// 为了按 Name 的 tag_value 排序，左连接一份 Name 标签记录
	// 仅用于排序，不改变筛选逻辑
	nameJoinSQL := "LEFT JOIN evaluator_tag AS t_name ON t_name.source_id = evaluator_tag.source_id AND t_name.tag_type = ? AND t_name.tag_key = ? AND t_name.deleted_at IS NULL"
	nameJoinArgs := []interface{}{tagType, "Name"}
	if langType != "" {
		nameJoinSQL += " AND t_name.lang_type = ?"
		nameJoinArgs = append(nameJoinArgs, langType)
	}
	query = query.Joins(nameJoinSQL, nameJoinArgs...)

    // 处理搜索关键词（在所有非 Category 标签范围内 LIKE 匹配）
    if filterOption.SearchKeyword != nil && *filterOption.SearchKeyword != "" {
        keyword := "%" + *filterOption.SearchKeyword + "%"
        query = query.Where("evaluator_tag.tag_key <> ? AND evaluator_tag.tag_value LIKE ?", string(entity.EvaluatorTagKey_Category), keyword)
    }

    // 处理筛选条件（自连接实现 AND，WHERE 实现 OR）
    if filterOption.Filters != nil {
        joinSQLs, joinArgs, whereSQL, whereArgs, err := dao.buildSelfJoinAndWhere(filterOption.Filters, tagType, langType)
        if err != nil {
            return nil, 0, err
        }
        for i, js := range joinSQLs {
            query = query.Joins(js, joinArgs[i]...)
        }
        if whereSQL != "" {
            query = query.Where(whereSQL, whereArgs...)
        }
    }

	// 先查询总数
	var total int64
	countQuery := query.Session(&gorm.Session{})
	// 打印 COUNT SQL（完整）
	countSQL := countQuery.ToSQL(func(tx *gorm.DB) *gorm.DB {
		var tmp int64
		return tx.Distinct("evaluator_tag.source_id").Count(&tmp)
	})
	logs.CtxInfo(ctx, "[GetSourceIDsByFilterConditions] COUNT SQL: %s", countSQL)
	if err := countQuery.Distinct("evaluator_tag.source_id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页处理
	if pageSize > 0 && pageNum > 0 {
		offset := (pageNum - 1) * pageSize
		query = query.Limit(int(pageSize)).Offset(int(offset))
	}

	// 执行查询（按 Name 标签值排序；无 Name 的排在后面）
	var sourceIDs []int64
	// 打印 SELECT SQL（完整）
	selectSQL := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		var tmp []int64
		return tx.Distinct("evaluator_tag.source_id").Order("t_name.tag_value IS NULL, t_name.tag_value ASC").Pluck("evaluator_tag.source_id", &tmp)
	})
	logs.CtxInfo(ctx, "[GetSourceIDsByFilterConditions] SELECT SQL: %s", selectSQL)
	err := query.
		Distinct("evaluator_tag.source_id").
		Order("t_name.tag_value IS NULL, t_name.tag_value ASC").
		Pluck("evaluator_tag.source_id", &sourceIDs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []int64{}, total, nil
		}
		return nil, 0, err
	}

	return sourceIDs, total, nil
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

// buildSelfJoinAndWhere 基于自连接实现 AND，基于 WHERE 实现 OR
// 返回：
// - joinSQLs/joinArgs: 需要追加到 query.Joins 的 JOIN 片段（按顺序）
// - whereSQL/whereArgs: 需要追加到 query.Where 的 WHERE 片段
func (dao *EvaluatorTagDAOImpl) buildSelfJoinAndWhere(filters *entity.EvaluatorFilters, tagType int32, langType string) ([]string, [][]interface{}, string, []interface{}, error) {
    var joinSQLs []string
    var joinArgs [][]interface{}
    var whereParts []string
    var whereArgs []interface{}

    // 生成唯一别名
    aliasCounter := 0
    nextAlias := func() string {
        aliasCounter++
        return fmt.Sprintf("t_%d", aliasCounter)
    }

    var build func(f *entity.EvaluatorFilters, parentIsAnd bool) (string, []interface{}, error)
    build = func(f *entity.EvaluatorFilters, parentIsAnd bool) (string, []interface{}, error) {
        if f == nil {
            return "", nil, nil
        }

        isOr := f.LogicOp != nil && *f.LogicOp == entity.FilterLogicOp_Or

        // 当前层的原子条件
        var parts []string
        var args []interface{}

        if isOr {
            // OR: 不做自连接，直接在 WHERE 中拼接到 base（evaluator_tag）别名上
            for _, c := range f.FilterConditions {
                if c == nil {
                    continue
                }
                sqlFrag, sqlArgs, err := dao.buildSingleConditionWithAlias("evaluator_tag", c)
                if err != nil {
                    return "", nil, err
                }
                if sqlFrag != "" {
                    parts = append(parts, "("+sqlFrag+")")
                    args = append(args, sqlArgs...)
                }
            }
            // 子条件
            for _, sub := range f.SubFilters {
                subSQL, subArgs, err := build(sub, false)
                if err != nil {
                    return "", nil, err
                }
                if subSQL != "" {
                    parts = append(parts, "("+subSQL+")")
                    args = append(args, subArgs...)
                }
            }

            if len(parts) > 0 {
                return strings.Join(parts, " OR "), args, nil
            }
            return "", nil, nil
        }

        // AND: 自连接。每个原子条件产生一个 JOIN。
        for _, c := range f.FilterConditions {
            if c == nil {
                continue
            }
            alias := nextAlias()
            onFrag, onArgs, err := dao.buildJoinPredicate(alias, c)
            if err != nil {
                return "", nil, err
            }
            join := fmt.Sprintf("JOIN evaluator_tag AS %s ON %s.source_id = evaluator_tag.source_id AND %s.tag_type = ? AND %s.deleted_at IS NULL", alias, alias, alias, alias)
            jArgs := []interface{}{tagType}
            if langType != "" {
                join += fmt.Sprintf(" AND %s.lang_type = ?", alias)
                jArgs = append(jArgs, langType)
            }
            if onFrag != "" {
                join += " AND " + onFrag
                jArgs = append(jArgs, onArgs...)
            }
            joinSQLs = append(joinSQLs, join)
            joinArgs = append(joinArgs, jArgs)
        }

        // 子条件：如果子条件是 OR，会返回 where 片段；如果子条件是 AND，会追加更多 JOIN
        for _, sub := range f.SubFilters {
            subSQL, subArgs, err := build(sub, true)
            if err != nil {
                return "", nil, err
            }
            if subSQL != "" { // OR 分支产生的 where 片段
                parts = append(parts, "("+subSQL+")")
                args = append(args, subArgs...)
            }
        }

        if len(parts) > 0 {
            // AND 层对 where 片段用 AND 连接
            return strings.Join(parts, " AND "), args, nil
        }
        return "", nil, nil
    }

    whereSQL, whereArgsLocal, err := build(filters, false)
    if err != nil {
        return nil, nil, "", nil, err
    }
    if whereSQL != "" {
        whereParts = append(whereParts, whereSQL)
        whereArgs = append(whereArgs, whereArgsLocal...)
    }

    finalWhere := strings.Join(whereParts, " AND ")
    return joinSQLs, joinArgs, finalWhere, whereArgs, nil
}

// buildSingleConditionWithAlias 生成基于指定别名的条件子句
func (dao *EvaluatorTagDAOImpl) buildSingleConditionWithAlias(alias string, condition *entity.EvaluatorFilterCondition) (string, []interface{}, error) {
    if condition == nil {
        return "", nil, nil
    }
    tagKey := string(condition.TagKey)
    operator := condition.Operator
    value := condition.Value

    switch operator {
    case entity.EvaluatorFilterOperatorType_Equal:
        return fmt.Sprintf("%s.tag_key = ? AND %s.tag_value = ?", alias, alias), []interface{}{tagKey, value}, nil
    case entity.EvaluatorFilterOperatorType_NotEqual:
        return fmt.Sprintf("%s.tag_key = ? AND %s.tag_value != ?", alias, alias), []interface{}{tagKey, value}, nil
    case entity.EvaluatorFilterOperatorType_In:
        values := strings.Split(value, ",")
        if len(values) == 0 {
            return "", nil, fmt.Errorf("IN operator requires non-empty values")
        }
        placeholders := strings.Repeat("?,", len(values))
        placeholders = placeholders[:len(placeholders)-1]
        return fmt.Sprintf("%s.tag_key = ? AND %s.tag_value IN (%s)", alias, alias, placeholders), append([]interface{}{tagKey}, convertToInterfaceSlice(values)...), nil
    case entity.EvaluatorFilterOperatorType_NotIn:
        values := strings.Split(value, ",")
        if len(values) == 0 {
            return "", nil, fmt.Errorf("NOT_IN operator requires non-empty values")
        }
        placeholders := strings.Repeat("?,", len(values))
        placeholders = placeholders[:len(placeholders)-1]
        return fmt.Sprintf("%s.tag_key = ? AND %s.tag_value NOT IN (%s)", alias, alias, placeholders), append([]interface{}{tagKey}, convertToInterfaceSlice(values)...), nil
    case entity.EvaluatorFilterOperatorType_Like:
        likeValue := "%" + value + "%"
        return fmt.Sprintf("%s.tag_key = ? AND %s.tag_value LIKE ?", alias, alias), []interface{}{tagKey, likeValue}, nil
    case entity.EvaluatorFilterOperatorType_IsNull:
        return fmt.Sprintf("%s.tag_key = ? AND %s.tag_value IS NULL", alias, alias), []interface{}{tagKey}, nil
    case entity.EvaluatorFilterOperatorType_IsNotNull:
        return fmt.Sprintf("%s.tag_key = ? AND %s.tag_value IS NOT NULL", alias, alias), []interface{}{tagKey}, nil
    default:
        return "", nil, fmt.Errorf("unsupported operator type: %v", operator)
    }
}

// buildJoinPredicate AND 场景下的 JOIN 条件（别名版）
func (dao *EvaluatorTagDAOImpl) buildJoinPredicate(alias string, condition *entity.EvaluatorFilterCondition) (string, []interface{}, error) {
    return dao.buildSingleConditionWithAlias(alias, condition)
}

// convertToInterfaceSlice 将字符串切片转换为interface{}切片
func convertToInterfaceSlice(strs []string) []interface{} {
	result := make([]interface{}, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}
