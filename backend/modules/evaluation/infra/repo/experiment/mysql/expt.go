// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

//go:generate mockgen -destination=mocks/expt.go -package=mocks . IExptDAO
type IExptDAO interface {
	Create(ctx context.Context, expt *model.Experiment) error

	Update(ctx context.Context, expt *model.Experiment) error

	UpdateFields(ctx context.Context, id int64, ufields map[string]any) error

	Delete(ctx context.Context, id int64) error

	MDelete(ctx context.Context, ids []int64) error

	List(ctx context.Context, page, size int32, filter *entity.ExptListFilter, orders []*entity.OrderBy, spaceID int64) ([]*model.Experiment, int64, error)

	GetByName(ctx context.Context, name string, spaceID int64) (*model.Experiment, error)

	GetByID(ctx context.Context, id int64) (*model.Experiment, error)

	MGetByID(ctx context.Context, ids []int64) ([]*model.Experiment, error)

	// GetIDsByGroupKey 取该分组下的实验 ID + 总数。page/pageSize 均 >0 才分页，否则返回全量（total = 返回条数）。
	GetIDsByGroupKey(ctx context.Context, spaceID int64, groupKey string, page, pageSize int32) ([]int64, int64, error)

	ExistGroupKey(ctx context.Context, groupKey string, spaceID int64) (bool, error)

	// ScanSchedulerQueue 在指定 scheduler_scope 内跨空间扫描中心调度候选实验。
	//
	// 与 List 的关键差别：不带 space_id 条件 —— 中心调度在 Scope 内按全局优先级排序，若按空间
	// 分别扫描，低优空间的实验会先于高优空间被处理，全局优先级语义即失效。
	// 但 scheduler_scope 必须带：它是调度所有权边界（线上/各 PPE 泳道共库）。
	// 走 idx_scheduler_queue，keyset 分页保证翻页不重不漏。
	ScanSchedulerQueue(ctx context.Context, param *entity.SchedulerQueueScanParam) ([]*model.Experiment, error)
}

func NewExptDAO(db db.Provider) IExptDAO {
	return &exptDAOImpl{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

const defaultLimit = 20

type exptDAOImpl struct {
	db    db.Provider
	query *query.Query
}

func (d *exptDAOImpl) UpdateFields(ctx context.Context, id int64, ufields map[string]any) error {
	q := query.Use(d.db.NewSession(ctx)).Experiment
	_, err := q.WithContext(ctx).
		Where(q.ID.Eq(id)).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "update expt fail, expt_id: %v, ufields: %v", id, ufields)
	}
	return nil
}

func (d *exptDAOImpl) Create(ctx context.Context, expt *model.Experiment) error {
	if err := d.db.NewSession(ctx).Create(expt).Error; err != nil {
		return errorx.Wrapf(err, "create expt fail, model: %v", json.Jsonify(expt))
	}
	return nil
}

// schedulingFrozenColumns 是创建时一次性冻结、此后任何 Update 都不得改写的调度列。
//
// 为什么必须显式 Omit：本方法用 struct 做 Updates，GORM 只跳过**零值**字段。
// 而 DO2PO 会把未设置的调度字段 Normalize 成非零值（mode ""→"legacy"、priority 0→1，
// 见 convert/expt.go），于是任何"只带 ID + 一两个业务字段"的部分更新（全仓 8 处，如
// LogRun 写 latest_run_id、ScheduleStart 改 status）都会顺手把 enforce 实验改回 legacy、
// 把申报的优先级重置为 1。
//
// 后果是静默且严重的：mode 变回 legacy 后中心调度再也扫不到它（扫描条件是
// scheduler_mode='enforce'），而旧 daemon 的抑制判断读到 legacy 会**恢复自主派发** ——
// 同一个 run 出现两个派发驱动、绕过全局额度账本，正是设计上明令禁止的情形。
// 且 scope 是零值会被跳过，最终留下 mode=legacy + scope 非空 的不可能组合。
//
// scheduler_mode / scheduler_scope 的唯一合法写入点是 Create（见 expt_manage_impl.go 的冻结逻辑）。
// priority_level 多一个：UpdateRunConf 允许运行中改优先级，它走 UpdateFields 的显式列名 map，
// 只能改到写进 map 的那一列 —— 这正是本 Omit 要防的"部分更新顺手重置"在那条路径上不成立的原因。
var schedulingFrozenColumns = []string{"priority_level", "scheduler_mode", "scheduler_scope"}

func (d *exptDAOImpl) Update(ctx context.Context, expt *model.Experiment) error {
	if err := d.db.NewSession(ctx).Model(&model.Experiment{}).Where("id = ?", expt.ID).
		Omit(schedulingFrozenColumns...).
		Updates(expt).Error; err != nil {
		return errorx.Wrapf(err, "update expt fail, expt_id: %v, updated: %v", expt.ID, json.Jsonify(expt))
	}
	return nil
}

func (d *exptDAOImpl) Delete(ctx context.Context, id int64) error {
	if err := d.db.NewSession(ctx).Delete(&model.Experiment{}, id).Error; err != nil {
		return errorx.Wrapf(err, "delete expt fail, expt_id: %v", id)
	}
	return nil
}

func (d *exptDAOImpl) MDelete(ctx context.Context, ids []int64) error {
	if err := d.db.NewSession(ctx).Delete(slices.Transform(ids, func(e int64, _ int) *model.Experiment {
		return &model.Experiment{ID: e}
	})).Error; err != nil {
		return errorx.Wrapf(err, "delete expts fail, expt_ids: %v", ids)
	}
	return nil
}

func (d *exptDAOImpl) List(ctx context.Context, page, size int32, filter *entity.ExptListFilter, orders []*entity.OrderBy, spaceID int64) ([]*model.Experiment, int64, error) {
	var (
		experiments []*model.Experiment
		db          = d.db.NewSession(ctx)
		count       int64
	)

	if d.filterNeedJoin(filter) {
		db = db.Model(&model.Experiment{}).
			Joins("INNER JOIN expt_evaluator_ref ON experiment.id = expt_evaluator_ref.expt_id").
			Where("experiment.space_id = ?", spaceID)
		db = db.Where("experiment.visibility <> ?", int32(entity.Visibility_Hidden))
	} else {
		db = db.Model(&model.Experiment{}).Where("space_id = ?", spaceID)
		db = db.Where("visibility <> ?", int32(entity.Visibility_Hidden))
	}

	conds, ok := d.toConditions(filter, orders, spaceID)
	if !ok {
		return experiments, 0, nil
	}

	for _, cond := range conds {
		db = cond(db)
	}

	db.Group("experiment.id")
	db.Count(&count)

	if page > 0 && size > 0 {
		db = db.Limit(int(size)).Offset(int((page - 1) * size))
	} else {
		db = db.Limit(defaultLimit)
	}

	if err := db.Find(&experiments).Error; err != nil {
		return nil, 0, errorx.Wrapf(err, "pull expt fail, space_id: %v, page: %v, size: %v, filter: %v", spaceID, page, size, json.Jsonify(filter))
	}

	return experiments, count, nil
}

func (d *exptDAOImpl) filterNeedJoin(f *entity.ExptListFilter) bool {
	if f != nil {
		if f.Includes != nil && len(f.Includes.EvaluatorIDs) > 0 {
			return true
		}
		if f.Excludes != nil && len(f.Excludes.EvaluatorIDs) > 0 {
			return true
		}
	}
	return false
}

func (d *exptDAOImpl) toConditions(f *entity.ExptListFilter, orders []*entity.OrderBy, spaceID int64) ([]func(tx *gorm.DB) *gorm.DB, bool) {
	if f == nil && len(orders) == 0 {
		return nil, true
	}

	if f != nil && !f.Includes.IsValid() {
		return nil, false
	}

	var (
		exptPrefix = ""
		eefPrefix  = model.TableNameExptEvaluatorRef + "."
		conditions []func(tx *gorm.DB) *gorm.DB
	)

	if d.filterNeedJoin(f) {
		exptPrefix = model.TableNameExperiment + "."
	}

	condFn := func(comparator, scopeComparator string, ffields *entity.ExptFilterFields) []func(tx *gorm.DB) *gorm.DB {
		var conds []func(tx *gorm.DB) *gorm.DB

		if ffields == nil {
			return conds
		}

		if ffields != nil && len(ffields.CreatedBy) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%screated_by %s (?)", exptPrefix, scopeComparator), ffields.CreatedBy)
			})
		}
		if ffields != nil && len(ffields.UpdatedBy) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%supdated_by %s (?)", exptPrefix, scopeComparator), ffields.UpdatedBy)
			})
		}
		if ffields != nil && len(ffields.TargetIDs) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%starget_id %s (?)", exptPrefix, scopeComparator), ffields.TargetIDs)
			})
		}
		// EvalSetID / EvalSetVersionID 语义升级为「实验包含该 set/version」:
		// 老实验 = experiment 列匹配; 新实验(MultiSetConfig) = expt_item_ref 倒排 (id IN 子查询)。
		// Include(IN): 列命中 OR 倒排命中; Exclude(NOT IN): 列不命中 AND 倒排不命中。
		// 用子查询而非 JOIN: expt_item_ref 一实验数千行, JOIN+GROUP BY fan-out 不可控; 子查询走
		// idx_space_eval_set_version_expt 前缀, 也避免把倒排集合物化成超大 IN-list。
		isInclude := scopeComparator == "IN"
		if ffields != nil && len(ffields.EvalSetIDs) > 0 {
			ids := ffields.EvalSetIDs
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				sub := db.Session(&gorm.Session{NewDB: true}).Table(model.TableNameExptItemRef).
					Select("expt_id").
					Where("space_id = ? AND eval_set_id IN (?) AND deleted_at IS NULL", spaceID, ids)
				if isInclude {
					return db.Where(
						fmt.Sprintf("(%[1]seval_set_id IN (?) OR %[1]sid IN (?))", exptPrefix),
						ids, sub)
				}
				return db.Where(fmt.Sprintf("%[1]seval_set_id NOT IN (?) AND %[1]sid NOT IN (?)", exptPrefix), ids, sub)
			})
		}
		if ffields != nil && len(ffields.EvalSetVersionIDs) > 0 {
			verIDs := ffields.EvalSetVersionIDs
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				sub := db.Session(&gorm.Session{NewDB: true}).Table(model.TableNameExptItemRef).
					Select("expt_id").
					Where("space_id = ? AND eval_set_version_id IN (?) AND deleted_at IS NULL", spaceID, verIDs)
				if isInclude {
					return db.Where(
						fmt.Sprintf("(%[1]seval_set_version_id IN (?) OR %[1]sid IN (?))", exptPrefix),
						verIDs, sub)
				}
				return db.Where(fmt.Sprintf("%[1]seval_set_version_id NOT IN (?) AND %[1]sid NOT IN (?)", exptPrefix), verIDs, sub)
			})
		}
		if ffields != nil && len(ffields.Status) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sstatus %s (?)", exptPrefix, scopeComparator), ffields.Status)
			})
		}
		if ffields != nil && len(ffields.TargetType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%starget_type %s (?)", exptPrefix, scopeComparator), ffields.TargetType)
			})
		}
		if ffields != nil && len(ffields.EvaluatorIDs) > 0 {
			conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sevaluator_id %s (?)", eefPrefix, scopeComparator), ffields.EvaluatorIDs)
			})
		}
		if ffields != nil && len(ffields.SourceID) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%ssource_id %s (?)", exptPrefix, scopeComparator), ffields.SourceID)
			})
		}
		if ffields != nil && len(ffields.ExptType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sexpt_type %s (?)", exptPrefix, scopeComparator), ffields.ExptType)
			})
		}
		if ffields != nil && len(ffields.ExptTemplateIDs) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sexpt_template_id %s (?)", exptPrefix, scopeComparator), ffields.ExptTemplateIDs)
			})
		}
		if ffields != nil && len(ffields.SourceType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%ssource_type %s (?)", exptPrefix, scopeComparator), ffields.SourceType)
			})
		}
		if ffields != nil && len(ffields.TriggerType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%strigger_type %s (?)", exptPrefix, scopeComparator), ffields.TriggerType)
			})
		}

		return conds
	}

	if f != nil && len(f.FuzzyName) > 0 {
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			return db.Where(exptPrefix+"name like ?", "%"+f.FuzzyName+"%")
		})
	}

	// eval_set_source_type 顶层筛选 (与 FuzzyName 同级, 非 Includes/Excludes):
	// 调用方未传 EvalSetSourceTypes → 默认排除 MultiSetConfig(2), 旧数据 NULL (item-centric 改造前) 一并保留;
	// 显式传则严格按白名单 IN。
	if f != nil {
		if len(f.EvalSetSourceTypes) > 0 {
			srcTypes := f.EvalSetSourceTypes
			conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
				return db.Where(exptPrefix+"eval_set_source_type IN (?)", srcTypes)
			})
		} else {
			conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
				col := exptPrefix + "eval_set_source_type"
				return db.Where(col+" <> ? OR "+col+" IS NULL", int64(entity.ExptEvalSetSourceType_MultiSetConfig))
			})
		}
	}

	if f != nil {
		conditions = append(conditions, condFn("=", "IN", f.Includes)...)
		conditions = append(conditions, condFn("!=", "NOT IN", f.Excludes)...)
	}

	ordered := false
	for _, orderBy := range orders {
		column := sortFieldToColumn[gptr.Indirect(orderBy.Field)]
		if len(column) == 0 {
			continue
		}

		ordered = true
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			sort := consts.SortDesc
			if gptr.Indirect(orderBy.IsAsc) {
				sort = consts.SortAsc
			}
			return db.Order(exptPrefix + column + " " + sort)
		})
	}

	if !ordered {
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			return db.Order(exptPrefix + "start_at desc")
		})
	}

	return conditions, true
}

var sortFieldToColumn = map[string]string{
	"start_time": "start_at",
	"end_time":   "end_at",
}

func (d *exptDAOImpl) GetByName(ctx context.Context, name string, spaceID int64) (*model.Experiment, error) {
	expt := d.query.Experiment
	found, err := expt.WithContext(ctx).
		Where(expt.SpaceID.Eq(spaceID)).
		Where(expt.Name.Eq(name)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, errorx.WrapByCode(err, errno.CommonMySqlErrorCode)
	}
	return found, nil
}

func (d *exptDAOImpl) GetByID(ctx context.Context, id int64) (*model.Experiment, error) {
	expt := d.query.Experiment
	q := expt.WithContext(ctx)
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}

	experiment, err := q.Where(
		expt.ID.Eq(id),
	).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.WrapByCode(err, errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("experiment %d not found", id)))
		}
		return nil, errorx.Wrapf(err, "mysql get expt fail, expt_ids: %v", id)
	}
	return experiment, nil
}

func (d *exptDAOImpl) MGetByID(ctx context.Context, ids []int64) ([]*model.Experiment, error) {
	expt := d.query.Experiment
	q := expt.WithContext(ctx)
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}

	experiments, err := q.Where(
		expt.ID.In(ids...),
	).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "mysql mget expt fail, expt_ids: %v", ids)
	}

	experimentMap := make(map[int64]*model.Experiment)
	for _, experiment := range experiments {
		experimentMap[experiment.ID] = experiment
	}

	sortedExperiments := make([]*model.Experiment, 0, len(ids))
	for _, id := range ids {
		if experiment, exists := experimentMap[id]; exists {
			sortedExperiments = append(sortedExperiments, experiment)
		}
	}

	return sortedExperiments, nil
}

// GetIDsByGroupKey 按 group key 取该空间下的实验 ID 列表。
//
// 分页语义（关键，勿改成 defaultLimit 兜底）：
//   - page > 0 且 pageSize > 0 → 启用分页：先在同一个查询上 Count() 得全量 total，再 Limit/Offset 取当页。
//   - 否则（含零值、负数）→ 不分页，返回全量，total = len(ids)，且**不发** count 查询。
//     内部面调用方（前端/内部服务）不感知分页、不传即零值，必须拿到全量，绝不能被隐式截断。
//
// 排序：无条件 ORDER BY id ASC —— 分页与否顺序一致，保证翻页不重不漏。
func (d *exptDAOImpl) GetIDsByGroupKey(ctx context.Context, spaceID int64, groupKey string, page, pageSize int32) ([]int64, int64, error) {
	expt := d.query.Experiment
	q := expt.WithContext(ctx)
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}

	// count 与 pluck 复用同一个 q（含 WriteDB 分支），避免主从不一致导致 total 与列表对不上。
	q = q.Where(
		expt.SpaceID.Eq(spaceID),
		expt.ExperimentGroupKey.Eq(groupKey),
		expt.DeletedAt.IsNull(),
	).Order(expt.ID)

	paginated := page > 0 && pageSize > 0

	var total int64
	if paginated {
		cnt, err := q.Count()
		if err != nil {
			return nil, 0, errorx.Wrapf(err, "mysql count experiment ids by group key fail, space_id: %v, group_key: %v", spaceID, groupKey)
		}
		total = cnt
		q = q.Limit(int(pageSize)).Offset(int((page - 1) * pageSize))
	}

	var ids []int64
	if err := q.Pluck(expt.ID, &ids); err != nil {
		return nil, 0, errorx.Wrapf(err, "mysql get experiment ids by group key fail, space_id: %v, group_key: %v, page: %v, page_size: %v", spaceID, groupKey, page, pageSize)
	}
	if !paginated {
		total = int64(len(ids))
	}
	return ids, total, nil
}

// ExistGroupKey 判断 group key 是否已被“其它空间”占用（跨空间隔离）。
// 同一空间内允许多个实验共享同一 group key，故排除 spaceID 本身；命中其它空间即视为冲突。
func (d *exptDAOImpl) ExistGroupKey(ctx context.Context, groupKey string, spaceID int64) (bool, error) {
	expt := d.query.Experiment
	q := expt.WithContext(ctx)
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}

	cnt, err := q.Where(
		expt.ExperimentGroupKey.Eq(groupKey),
		expt.SpaceID.Neq(spaceID),
		expt.DeletedAt.IsNull(),
	).Limit(1).Count()
	if err != nil {
		return false, errorx.Wrapf(err, "mysql exist experiment group key fail, group_key: %v", groupKey)
	}
	return cnt > 0, nil
}

// ScanSchedulerQueue 在指定 scheduler_scope 内跨空间扫描中心调度候选实验，走 idx_scheduler_queue。
//
// 用裸 gorm 而非 gen DSL：keyset 的三元组比较是一段带括号 OR 的复合条件，
// gen 的链式 API 表达它需要嵌套多层 Or(...)，可读性远差于一条 SQL 片段。
//
// FORCE INDEX 的取舍：status IN (...) 是 range 条件，MySQL 可能因此放弃用索引满足 ORDER BY 而
// 走 filesort；灰度期数据量小可接受，故此处不 FORCE，留给 EXPLAIN 实测后再决定 ——
// 过早 FORCE INDEX 会在数据分布变化后反而选到更差的计划。
func (d *exptDAOImpl) ScanSchedulerQueue(ctx context.Context, param *entity.SchedulerQueueScanParam) ([]*model.Experiment, error) {
	if param == nil {
		return nil, nil
	}
	limit := int(param.Limit)
	if limit <= 0 {
		limit = defaultLimit
	}

	// Scope 为空时拒绝查询而非退化成扫全表。
	//
	// 这是"泳道不得调度线上实验"的物理闸门：一旦这里放行空 Scope，PPE 实例就会扫出
	// 线上的 enforce 实验、为它们预占额度并派发 item（item 走泳道 topic、结果写回共享库），
	// 而线上侧完全无感知。宁可这一拍报错（可见），也不能静默越界（不可见）。
	if strings.TrimSpace(param.SchedulerScope) == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode,
			errorx.WithExtraMsg("empty scheduler_scope for scheduler queue scan; refusing to scan across all scopes"))
	}

	tx := d.db.NewSession(ctx).Model(&model.Experiment{}).
		Where("scheduler_mode = ?", param.DispatchMode).
		Where("scheduler_scope = ?", param.SchedulerScope).
		Where("deleted_at IS NULL").
		// latest_run_id > 0 排除"只 Create 尚未 Run"的实验：它们没有 run 可供派发 item，
		// 扫进来只会让每拍白跑一遍。
		Where("latest_run_id > 0")

	if len(param.Statuses) > 0 {
		tx = tx.Where("status IN ?", param.Statuses)
	}

	if c := param.Cursor; c != nil {
		// keyset：严格小于游标（按 priority DESC, created_at ASC, id ASC 的字典序）
		tx = tx.Where(
			"(priority_level < ?) OR (priority_level = ? AND created_at > FROM_UNIXTIME(?)) OR (priority_level = ? AND created_at = FROM_UNIXTIME(?) AND id > ?)",
			c.PriorityLevel,
			c.PriorityLevel, c.CreatedAtUnix,
			c.PriorityLevel, c.CreatedAtUnix, c.ExptID,
		)
	}

	var pos []*model.Experiment
	if err := tx.
		Order("priority_level DESC, created_at ASC, id ASC").
		Limit(limit).
		Find(&pos).Error; err != nil {
		return nil, errorx.Wrapf(err, "mysql scan scheduler queue fail, mode: %v, scope: %v, statuses: %v",
			param.DispatchMode, param.SchedulerScope, param.Statuses)
	}
	return pos, nil
}
