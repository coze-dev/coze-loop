// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/bytedance/gg/gmap"
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/vfs"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type importHandler struct {
	job          *entity.IOJob
	fieldMapping map[string][]string
	ds           *DatasetWithSchema
	fsUnion      vfs.IUnionFS
	svc          IDatasetAPI
	repo         repo.IDatasetAPI

	currentUnit *importUnit // 当前在处理的单元，其内容实时更新
}

type importUnit struct {
	status       entity.JobStatus
	preProcessed int64
	errors       map[entity.ItemErrorType]*entity.ItemErrorGroup
	progresses   map[string]*entity.DatasetIOJobProgress
	filename     string // 更换文件时更新

	// 以下字段，每次保存后会清空。
	startedAt *time.Time
	total     *int64 // 文件总行数，扫到最后一行时更新
	processed int64
	added     int64
	items     []*IndexedItem
}

type importWorkspace struct {
	source   *entity.DatasetIOFile
	progress map[string]*entity.DatasetIOJobProgress // 上次 checkpoint 中各文件的处理进度
	fs       vfs.ROFileSystem                        // can read dir && files
	dir      string
	files    []string
	cursor   int // files 读取的游标
}

func (s *DatasetServiceImpl) newImportHandler(job *entity.IOJob, ds *DatasetWithSchema) *importHandler {
	groupBySource := gslice.GroupBy(job.FieldMappings, func(field *entity.FieldMapping) string {
		return field.Source
	})
	mapping := gmap.Map(groupBySource, func(source string, fms []*entity.FieldMapping) (string, []string) {
		return source, gslice.Uniq(gslice.Map(fms, func(fm *entity.FieldMapping) string { return fm.Target }))
	})
	return &importHandler{
		job:          job,
		fieldMapping: mapping,
		ds:           ds,
		fsUnion:      s.fsUnion,
		svc:          s,
		repo:         s.repo,
		currentUnit:  newImportUnit(job),
	}
}

func newImportUnit(job *entity.IOJob) *importUnit {
	var preProcessed *int64
	var subProgresses []*entity.DatasetIOJobProgress
	if job.Progress != nil {
		preProcessed = job.Progress.Processed
		subProgresses = job.Progress.SubProgresses
	}
	unit := &importUnit{
		status:       gptr.Indirect(job.Status),
		preProcessed: gptr.Indirect(preProcessed),
	}
	unit.errors = gslice.ToMap(job.Errors, func(item *entity.ItemErrorGroup) (entity.ItemErrorType, *entity.ItemErrorGroup) {
		return gptr.Indirect(item.Type), item
	})
	unit.progresses = gslice.ToMap(subProgresses, func(prog *entity.DatasetIOJobProgress) (string, *entity.DatasetIOJobProgress) {
		return gptr.Indirect(prog.Name), prog
	})
	return unit
}

func newImportWorkspace(ctx context.Context, job *entity.IOJob, fs vfs.IUnionFS) (*importWorkspace, error) {
	// 处理源 file
	var source *entity.DatasetIOFile
	if job.Source != nil {
		source = job.Source.File
	}
	var subProgresses []*entity.DatasetIOJobProgress
	if job.Progress != nil {
		subProgresses = job.Progress.SubProgresses
	}
	if source == nil {
		return nil, errors.New(`source file is nil`)
	}
	rfs, err := fs.GetROFileSystem(source.Provider)
	if err != nil {
		return nil, err
	}

	w := &importWorkspace{
		source: source,
		fs:     rfs,
		files:  []string{source.Path},
	}
	w.progress = gslice.ToMap(subProgresses, func(t *entity.DatasetIOJobProgress) (string, *entity.DatasetIOJobProgress) {
		return gptr.Indirect(t.Name), t
	})

	// todo: 支持压缩文件 & 目录导入
	return w, nil
}

func (h *importHandler) Handle(ctx context.Context) error {
	// todo: add operation lock
	return h.run(ctx)
}

func (h *importHandler) run(ctx context.Context) error {
	w, err := newImportWorkspace(ctx, h.job, h.fsUnion)
	if err != nil {
		return err
	}

	started, err := h.startJob(ctx)
	if err != nil {
		return err
	}
	if !started {
		return nil
	}

	for {
		if entity.IsJobTerminal(h.currentUnit.status) {
			return nil
		}

		fr, ok, err := h.nextFile(ctx, w)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := h.importFile(ctx, w, fr); err != nil {
			return err
		}
		if h.currentUnit.status == entity.JobStatus_Completed && len(h.currentUnit.errors) != 0 {
			// 若任务完成时包含错误，需要扫描剩下的文件内容更新总行数
			h.scanFileWithoutSave(ctx, w, fr)
		}
	}
}

func (h *importHandler) startJob(ctx context.Context) (bool, error) {
	if h.currentUnit.status == entity.JobStatus_Running {
		return true, nil
	}

	job := h.job
	h.currentUnit.startedAt = gptr.Of(time.Now())
	h.currentUnit.status = entity.JobStatus_Running // 内存中开始任务，暂不更新 DB
	var overwrite bool
	if job.Option != nil {
		overwrite = gptr.Indirect(job.Option.OverwriteDataset)
	}
	if !overwrite {
		return true, nil
	}
	// 清空
	if err := h.svc.ClearDataset(ctx, h.ds); err != nil {
		logs.CtxError(ctx, "clear dataset failed, job_id=%d, dataset_id=%d, err=%v", job.ID, h.ds.ID, err)

		if err := h.endJobWithError(ctx, err.Error()); err != nil {
			return false, err // 不重试，避免多次清空数据集
		}
		return false, err
	}

	return true, nil
}

func (h *importHandler) nextFile(ctx context.Context, w *importWorkspace) (fr *vfs.FileReader, ok bool, err error) {
	fr, ok, err = w.nextFile(ctx)
	if err != nil {
		h.currentUnit.onInternalErr(ctx, err, err.Error())
		if err := h.saveCurrentUnit(ctx); err != nil {
			return nil, false, err
		}
	}
	return
}

func (h *importHandler) importFile(ctx context.Context, w *importWorkspace, fr *vfs.FileReader) (err error) {
	const bulkSize = 100

	unit := h.currentUnit
	unit.filename = fr.GetName()
	lastCursor := fr.GetCursor()

	for {
		kv, err := fr.Next()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			unit.onInternalErr(ctx, err, `read next item from file failed`)
			break
		}

		unit.items = append(unit.items, &IndexedItem{
			Item:  h.kv2Item(kv),
			Index: int(fr.GetCursor()),
		})
		if len(unit.items) < bulkSize {
			continue
		}

		unit.processed = fr.GetCursor() - lastCursor
		lastCursor = fr.GetCursor()
		if err := h.saveCurrentUnit(ctx); err != nil {
			return err
		}
		if unit.status != entity.JobStatus_Running {
			break
		}
	}

	unit.processed = fr.GetCursor() - lastCursor
	if w.noMoreFile() { // 最后一个文件。
		unit.status = entity.JobStatus_Completed
		unit.total = gptr.Of(unit.processed + unit.preProcessed)
	}
	return h.saveCurrentUnit(ctx)
}

func (h *importHandler) kv2Item(kv map[string]any) *entity.Item {
	item := &entity.Item{
		Data: make([]*entity.FieldData, 0, len(h.fieldMapping)),
	}
	for k, v := range kv {
		names, ok := h.fieldMapping[k]
		if !ok {
			continue
		}
		for _, name := range names {
			item.Data = append(item.Data, &entity.FieldData{
				Name:    name,
				Content: fmt.Sprintf("%v", v), // todo: 非 string json marshal?
			})
		}
	}
	return item
}

func (h *importHandler) saveCurrentUnit(ctx context.Context) error {
	unit := h.currentUnit
	if unit.isEmpty() {
		logs.CtxInfo(ctx, "nothing to save")
		return nil
	}
	defer unit.onFlush()

	if err := ctx.Err(); err != nil {
		logs.CtxWarn(ctx, "save current unit got context error, job_id=%d, err=%v", h.job.ID, err)
		return errno.NewRetryableErr(err)
	}

	// 保存 items
	SanitizeInputItem(h.ds, gslice.Map(unit.items, func(i *IndexedItem) *entity.Item { return i.Item })...)
	good, bad := ValidateIndexedItems(h.ds, unit.items)
	unit.onBadItems(ctx, bad...)
	added, err := h.svc.BatchCreateItems(ctx, h.ds, good, &MAddItemOpt{PartialAdd: true})
	if err != nil {
		// todo: 记录失败的范围
		unit.onInternalErr(ctx, err, `batch create items failed`)
		// continue on error
	}

	unit.added = int64(len(added))
	if err == nil && len(added) < len(good) {
		logs.CtxInfo(ctx, "batch create items got dataset full, job will be ended, job_id=%d, dataset_id=%d, added=%d, good=%d", h.job.ID, h.ds.ID, len(added), len(good))
		unit.onDatasetFull()
	}

	// 保存 job 进度
	delta := unit.toDeltaDatasetIOJob()
	if err := h.repo.UpdateIOJob(ctx, h.job.ID, delta); err != nil {
		return errno.NewRetryableErr(err)
	}

	logs.CtxInfo(ctx, "import unit saved, processed_item=%d, added=%d, pre_processed=%d", unit.processed, len(added), unit.preProcessed)
	return nil
}

func (h *importHandler) endJobWithError(ctx context.Context, errMsg string) error {
	delta := &repo.DeltaDatasetIOJob{
		Status: gptr.Of(entity.JobStatus_Failed),
		Errors: []*entity.ItemErrorGroup{
			{
				Type:    gptr.Of(entity.ItemErrorType_InternalError),
				Summary: gptr.Of(errMsg),
			},
		},
	}
	return h.repo.UpdateIOJob(ctx, h.job.ID, delta)
}

// scanFileWithoutSave 仅更新文件的总行数，不写入 item。
func (h *importHandler) scanFileWithoutSave(ctx context.Context, w *importWorkspace, fr *vfs.FileReader) {
	// 在以下场景中，任务流转至完成，但文件并未被完全处理，可能会导致总行数与实际不一致
	// 1. 数据集已满
	// 2. 包含审核不通过的内容
	unit := h.currentUnit
	for {
		unit.filename = fr.GetName()
		lastCursor := fr.GetCursor()
		for {
			_, err := fr.Next()
			if err != nil && errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				logs.CtxWarn(ctx, "get next failed, err=%v", err)
				break
			}
		}

		unit.processed += fr.GetCursor() - lastCursor
		if prog, ok := unit.progresses[unit.filename]; ok {
			prog.Total = gptr.Of(fr.GetCursor())
			prog.Processed = gptr.Of(fr.GetCursor())
		}
		if w.noMoreFile() { // 最后一个文件
			break
		}
		// next file reader
		nextFR, ok, err := h.nextFile(ctx, w)
		if err != nil {
			logs.CtxWarn(ctx, "get file reader failed, err=%v", err)
			return
		}
		if !ok {
			return
		}
		fr = nextFR
	}
	delta := &repo.DeltaDatasetIOJob{
		Total:          gptr.Of(unit.processed + unit.preProcessed),
		DeltaProcessed: unit.processed,
		SubProgresses:  gmap.Values(unit.progresses),
	}
	if err := h.repo.UpdateIOJob(ctx, h.job.ID, delta); err != nil {
		logs.CtxWarn(ctx, "update io_job failed, err=%v", err)
	}
}

func (w *importWorkspace) nextFile(ctx context.Context) (lr *vfs.FileReader, ok bool, err error) {
	if w.cursor >= len(w.files) {
		return nil, false, nil
	}

	name := w.files[w.cursor]
	proc, ok := w.progress[name]
	if ok && gptr.Indirect(proc.Total) > 0 && gptr.Indirect(proc.Total) <= gptr.Indirect(proc.Processed) { // 已处理完成.
		w.cursor += 1
		return w.nextFile(ctx)
	}

	filename := filepath.Join(w.dir, name)
	r, err := w.fs.ReadFile(ctx, filename)
	if err != nil {
		err = errors.WithMessagef(err, "filename=%s", filename)
		return nil, false, err
	}
	info, err := w.fs.Stat(ctx, filename)
	if err != nil {
		err = errors.WithMessagef(err, "filename=%s", filename)
		return nil, false, err
	}
	var fm *entity.FileFormat
	if w.source != nil {
		fm = w.source.Format
	}
	lr, err = vfs.NewFileReader(name, r, info, gptr.Indirect(fm))
	if err != nil {
		return nil, false, err
	}
	w.cursor += 1
	if proc != nil && gptr.Indirect(proc.Processed) > 0 {
		logs.CtxInfo(ctx, "resume reading cursor from line %d, file=%s", gptr.Indirect(proc.Processed), name)
		if err := lr.SeekToOffset(gptr.Indirect(proc.Processed)); err != nil {
			_ = r.Close()
			return nil, true, err
		}
	}

	return lr, true, nil
}

func (w *importWorkspace) noMoreFile() bool {
	return w.cursor >= len(w.files)
}

func (u *importUnit) onInternalErr(ctx context.Context, err error, msg string) {
	logs.CtxWarn(ctx, "got internal error '%s', file=%s, err=%v", msg, u.filename, err)
	u.appendError(&entity.ItemErrorGroup{
		Type:    gptr.Of(entity.ItemErrorType_InternalError),
		Details: []*entity.ItemErrorDetail{{Message: gptr.Of(msg)}},
	})
}

func (u *importUnit) onDatasetFull() {
	u.appendError(&entity.ItemErrorGroup{
		Type:       gptr.Of(entity.ItemErrorType_ExceedDatasetCapacity),
		Details:    []*entity.ItemErrorDetail{{Message: gptr.Of("exceed dataset capacity")}},
		ErrorCount: gptr.Of(int32(-1)),
	})
	u.status = entity.JobStatus_Completed // 超限视作完成，不记作失败
}

func (u *importUnit) onIllegalContentErr(ctx context.Context, err error, msg string) {
	logs.CtxWarn(ctx, "contain illegal content '%s', file=%s, err=%v", msg, u.filename, err)
	u.appendError(&entity.ItemErrorGroup{
		Type:    gptr.Of(entity.ItemErrorType_IllegalContent),
		Details: []*entity.ItemErrorDetail{{Message: gptr.Of(msg)}},
	})
	u.status = entity.JobStatus_Completed // 审核不通过视为完成，不记作失败
}

func (u *importUnit) onBadItems(ctx context.Context, errors ...*entity.ItemErrorGroup) {
	for _, e := range errors {
		details := gslice.Map(e.Details, func(d *entity.ItemErrorDetail) string { return entity.ItemErrorDetailToString(d) })
		logs.CtxInfo(ctx, "got %d invalid items, file=%s, error_type=%v, details=%v", e.ErrorCount, u.filename, e.Type, details)
		u.appendError(e)
	}
}

func (u *importUnit) appendError(eg *entity.ItemErrorGroup) {
	if u.errors == nil {
		u.errors = make(map[entity.ItemErrorType]*entity.ItemErrorGroup)
	}
	pre, ok := u.errors[gptr.Indirect(eg.Type)]
	if !ok {
		pre = &entity.ItemErrorGroup{Type: eg.Type}
		u.errors[gptr.Indirect(eg.Type)] = pre
	}

	pre.ErrorCount = gptr.Of(gptr.Indirect(pre.ErrorCount) + gptr.Indirect(eg.ErrorCount))
	pre.Details = append(pre.Details, eg.Details...)
	if len(pre.Details) > 10 {
		pre.Details = pre.Details[:10]
	}
}

func (u *importUnit) onFlush() {
	u.progresses[u.filename] = u.mergeProgress()
	u.preProcessed += u.processed
	u.processed = 0
	u.added = 0
	u.items = u.items[:0]
	u.startedAt = nil
	u.total = nil
}

func (u *importUnit) isEmpty() bool {
	switch {
	case u.processed > 0:
	case u.added > 0:
	case len(u.items) > 0:
	case len(u.errors) > 0:
	case u.status != entity.JobStatus_Running:
	case u.startedAt != nil:
	case u.total != nil:
	default:
		return true
	}
	return false
}

func (u *importUnit) mergeProgress() *entity.DatasetIOJobProgress {
	prog := &entity.DatasetIOJobProgress{
		Name:      gptr.Of(u.filename),
		Processed: gptr.Of(u.processed),
		Added:     gptr.Of(u.added),
		Total:     u.total,
	}
	if pre, ok := u.progresses[u.filename]; ok {
		prog.Processed = gptr.Of(gptr.Indirect(pre.Processed) + u.processed)
		prog.Added = gptr.Of(gptr.Indirect(pre.Added) + u.added)
	}
	return prog
}

func (u *importUnit) toDeltaDatasetIOJob() *repo.DeltaDatasetIOJob {
	prog := u.mergeProgress()
	progs := gslice.Filter(gmap.Values(u.progresses), func(p *entity.DatasetIOJobProgress) bool {
		return gptr.Indirect(p.Name) != u.filename
	})
	progs = append(progs, prog)

	delta := &repo.DeltaDatasetIOJob{
		Status:         gptr.Of(u.status),
		PreProcessed:   gptr.Of(u.preProcessed),
		SubProgresses:  progs,
		DeltaProcessed: u.processed,
		DeltaAdded:     u.added,
		Errors:         gmap.Values(u.errors),
		StartedAt:      u.startedAt,
	}
	if entity.IsJobTerminal(u.status) {
		delta.EndedAt = gptr.Of(time.Now())
		totals := gslice.Map(progs, func(fp *entity.DatasetIOJobProgress) int64 { return gptr.Indirect(fp.Total) })
		delta.Total = gptr.Of(gslice.Sum(totals))
	}
	return delta
}
