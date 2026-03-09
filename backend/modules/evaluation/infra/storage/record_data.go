// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"bytes"
	"context"
	"io"

	"github.com/bytedance/gg/gptr"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/utils"
)

const (
	// EvalRecordFieldKeyPrefix 大字段在 S3 的 key 前缀
	EvalRecordFieldKeyPrefix = `eval:record:field:`
)

// RecordDataStorage 评测记录大字段存储服务，按 Content 字段级存储
type RecordDataStorage struct {
	batchStorage fileserver.BatchObjectStorage // 可为 nil，nil 时退化为仅 RDS 存储
	configer     component.IConfiger
}

// NewRecordDataStorage 创建评测记录存储服务，batchStorage 为 nil 时大字段存储能力不生效
func NewRecordDataStorage(
	batchStorage fileserver.BatchObjectStorage,
	configer component.IConfiger,
) *RecordDataStorage {
	return &RecordDataStorage{
		batchStorage: batchStorage,
		configer:     configer,
	}
}

// SaveEvaluatorRecordData 遍历 Content，大字段上传 S3 并剪裁后放回，整结构存 MySQL
func (s *RecordDataStorage) SaveEvaluatorRecordData(ctx context.Context, record *entity.EvaluatorRecord) error {
	if record == nil || s.batchStorage == nil {
		return nil
	}
	fieldMaxSize := s.getFieldMaxSize(ctx)
	if fieldMaxSize <= 0 {
		return nil
	}
	if record.EvaluatorInputData != nil {
		if err := s.processInputData(ctx, record.EvaluatorInputData, fieldMaxSize); err != nil {
			return errors.WithMessage(err, "process evaluator input data")
		}
	}
	if record.EvaluatorOutputData != nil {
		// EvaluatorOutputData 无 Content 字段，仅 EvaluatorResult.Reasoning 等，暂不处理
		_ = record.EvaluatorOutputData
	}
	return nil
}

// LoadEvaluatorRecordData 从 S3 加载评估器记录中被省略的大字段完整内容
func (s *RecordDataStorage) LoadEvaluatorRecordData(ctx context.Context, record *entity.EvaluatorRecord) error {
	if s.batchStorage == nil || record == nil {
		return nil
	}
	if record.EvaluatorInputData != nil {
		if err := s.loadOmittedContent(ctx, record.EvaluatorInputData); err != nil {
			return errors.WithMessage(err, "load evaluator input omitted content")
		}
	}
	return nil
}

// SaveEvalTargetRecordData 遍历 Content，大字段上传 S3 并剪裁后放回，整结构存 MySQL
func (s *RecordDataStorage) SaveEvalTargetRecordData(ctx context.Context, record *entity.EvalTargetRecord) error {
	if record == nil || s.batchStorage == nil {
		return nil
	}
	fieldMaxSize := s.getFieldMaxSize(ctx)
	if fieldMaxSize <= 0 {
		return nil
	}
	if record.EvalTargetInputData != nil {
		if err := s.processEvalTargetInputData(ctx, record.EvalTargetInputData, fieldMaxSize); err != nil {
			return errors.WithMessage(err, "process eval target input data")
		}
	}
	if record.EvalTargetOutputData != nil {
		if err := s.processEvalTargetOutputData(ctx, record.EvalTargetOutputData, fieldMaxSize); err != nil {
			return errors.WithMessage(err, "process eval target output data")
		}
	}
	return nil
}

// LoadEvalTargetOutputFields 从 S3 加载 output 中指定字段的大对象完整内容
func (s *RecordDataStorage) LoadEvalTargetOutputFields(ctx context.Context, record *entity.EvalTargetRecord, fieldKeys []string) error {
	if s.batchStorage == nil || record == nil || len(fieldKeys) == 0 {
		return nil
	}
	if record.EvalTargetOutputData == nil || record.EvalTargetOutputData.OutputFields == nil {
		return nil
	}
	keySet := make(map[string]struct{}, len(fieldKeys))
	for _, k := range fieldKeys {
		keySet[k] = struct{}{}
	}
	for k, c := range record.EvalTargetOutputData.OutputFields {
		if _, ok := keySet[k]; !ok {
			continue
		}
		if err := s.loadContentFromS3(ctx, c); err != nil {
			return errors.WithMessagef(err, "load output field %s", k)
		}
	}
	return nil
}

// LoadEvalTargetRecordData 从 S3 加载评测对象记录中被省略的大字段完整内容
func (s *RecordDataStorage) LoadEvalTargetRecordData(ctx context.Context, record *entity.EvalTargetRecord) error {
	if s.batchStorage == nil || record == nil {
		return nil
	}
	if record.EvalTargetInputData != nil {
		if err := s.loadOmittedContentFromInputData(ctx, record.EvalTargetInputData); err != nil {
			return errors.WithMessage(err, "load eval target input omitted content")
		}
	}
	if record.EvalTargetOutputData != nil {
		if err := s.loadOmittedContentFromOutputData(ctx, record.EvalTargetOutputData); err != nil {
			return errors.WithMessage(err, "load eval target output omitted content")
		}
	}
	return nil
}

func (s *RecordDataStorage) getFieldMaxSize(ctx context.Context) int64 {
	cfg := s.configer.GetEvaluationRecordStorage(ctx)
	if cfg == nil || len(cfg.Providers) == 0 {
		return 0
	}
	for _, p := range cfg.Providers {
		if p.Provider == "RDS" && p.MaxSize > 0 {
			return p.MaxSize
		}
	}
	return 204800 // 默认 200KB
}

func (s *RecordDataStorage) processInputData(ctx context.Context, input *entity.EvaluatorInputData, fieldMaxSize int64) error {
	if input == nil {
		return nil
	}
	// input_fields、evaluate_dataset_fields、evaluate_target_output_fields 中的大字段需写入 TOS
	if input.InputFields != nil {
		for _, c := range input.InputFields {
			if err := s.processContent(ctx, c, fieldMaxSize); err != nil {
				return err
			}
		}
	}
	if input.EvaluateDatasetFields != nil {
		for _, c := range input.EvaluateDatasetFields {
			if err := s.processContent(ctx, c, fieldMaxSize); err != nil {
				return err
			}
		}
	}
	if input.EvaluateTargetOutputFields != nil {
		for _, c := range input.EvaluateTargetOutputFields {
			if err := s.processContent(ctx, c, fieldMaxSize); err != nil {
				return err
			}
		}
	}
	if input.HistoryMessages != nil {
		for _, m := range input.HistoryMessages {
			if m != nil && m.Content != nil {
				if err := s.processContent(ctx, m.Content, fieldMaxSize); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *RecordDataStorage) processEvalTargetInputData(ctx context.Context, input *entity.EvalTargetInputData, fieldMaxSize int64) error {
	for _, c := range input.InputFields {
		if err := s.processContent(ctx, c, fieldMaxSize); err != nil {
			return err
		}
	}
	for _, m := range input.HistoryMessages {
		if m != nil && m.Content != nil {
			if err := s.processContent(ctx, m.Content, fieldMaxSize); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *RecordDataStorage) processEvalTargetOutputData(ctx context.Context, output *entity.EvalTargetOutputData, fieldMaxSize int64) error {
	for _, c := range output.OutputFields {
		if err := s.processContent(ctx, c, fieldMaxSize); err != nil {
			return err
		}
	}
	return nil
}

// processContent 递归处理 Content，大 Text 上传 S3 并剪裁后放回
func (s *RecordDataStorage) processContent(ctx context.Context, content *entity.Content, fieldMaxSize int64) error {
	if content == nil {
		return nil
	}
	// 处理 MultiPart 递归
	for _, part := range content.MultiPart {
		if err := s.processContent(ctx, part, fieldMaxSize); err != nil {
			return err
		}
	}
	// 仅对 Text 类型且超长时处理
	if content.Text == nil || content.ContentType == nil || *content.ContentType != entity.ContentTypeText {
		return nil
	}
	text := *content.Text
	if int64(len(text)) <= fieldMaxSize {
		return nil
	}
	// 上传完整内容到 S3
	key := EvalRecordFieldKeyPrefix + uuid.New().String()
	if err := s.batchStorage.Upload(ctx, key, bytes.NewReader([]byte(text))); err != nil {
		return errors.WithMessagef(err, "upload field to S3, key=%s", key)
	}
	// 剪裁后放回 Text，设置 ContentOmitted、FullContent
	preview := utils.TruncateJsonPreviewToSize([]byte(text), fieldMaxSize)
	content.Text = gptr.Of(string(preview))
	content.ContentOmitted = gptr.Of(true)
	content.FullContent = &entity.ObjectStorage{
		Provider: entity.StorageProviderPtr(entity.StorageProvider_S3),
		URI:      gptr.Of(key),
	}
	content.FullContentBytes = gptr.Of(int32(len(text)))
	return nil
}

func (s *RecordDataStorage) loadOmittedContent(ctx context.Context, input *entity.EvaluatorInputData) error {
	if input == nil {
		return nil
	}
	if input.InputFields != nil {
		for _, c := range input.InputFields {
			if err := s.loadContentFromS3(ctx, c); err != nil {
				return err
			}
		}
	}
	if input.EvaluateDatasetFields != nil {
		for _, c := range input.EvaluateDatasetFields {
			if err := s.loadContentFromS3(ctx, c); err != nil {
				return err
			}
		}
	}
	if input.EvaluateTargetOutputFields != nil {
		for _, c := range input.EvaluateTargetOutputFields {
			if err := s.loadContentFromS3(ctx, c); err != nil {
				return err
			}
		}
	}
	if input.HistoryMessages != nil {
		for _, m := range input.HistoryMessages {
			if m != nil && m.Content != nil {
				if err := s.loadContentFromS3(ctx, m.Content); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *RecordDataStorage) loadOmittedContentFromInputData(ctx context.Context, input *entity.EvalTargetInputData) error {
	for _, c := range input.InputFields {
		if err := s.loadContentFromS3(ctx, c); err != nil {
			return err
		}
	}
	for _, m := range input.HistoryMessages {
		if m != nil && m.Content != nil {
			if err := s.loadContentFromS3(ctx, m.Content); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *RecordDataStorage) loadOmittedContentFromOutputData(ctx context.Context, output *entity.EvalTargetOutputData) error {
	for _, c := range output.OutputFields {
		if err := s.loadContentFromS3(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// loadContentFromS3 递归加载 Content 中被省略的完整内容
func (s *RecordDataStorage) loadContentFromS3(ctx context.Context, content *entity.Content) error {
	if content == nil {
		return nil
	}
	for _, part := range content.MultiPart {
		if err := s.loadContentFromS3(ctx, part); err != nil {
			return err
		}
	}
	if !content.IsContentOmitted() || content.FullContent == nil || content.FullContent.URI == nil {
		return nil
	}
	key := *content.FullContent.URI
	reader, err := s.batchStorage.Read(ctx, key)
	if err != nil {
		return errors.WithMessagef(err, "read field from S3, key=%s", key)
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	if err != nil {
		return errors.WithMessagef(err, "read field body, key=%s", key)
	}
	content.Text = gptr.Of(string(data))
	content.ContentOmitted = gptr.Of(false)
	content.FullContent = nil
	content.FullContentBytes = nil
	return nil
}
