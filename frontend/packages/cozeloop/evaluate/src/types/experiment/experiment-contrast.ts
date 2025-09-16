// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type ColumnAnnotation,
  type ColumnEvaluator,
  type EvaluatorRecord,
  type AnnotateRecord,
} from '@cozeloop/api-schema/evaluation';

interface ColumnMap {
  evaluator: ColumnEvaluator;
  annotation: ColumnAnnotation;
}
export interface ColumnInfo<
  T extends keyof ColumnMap = 'evaluator' | 'annotation',
> {
  type: T;
  key: string;
  name: string;
  /** 评估器表头信息 | 人工标注标签表头信息 */
  data: ColumnMap[T];
}

interface ColumnRecordMap {
  evaluator: EvaluatorRecord;
  annotation: AnnotateRecord;
}

export interface ColumnRecord<
  T extends keyof ColumnRecordMap = 'evaluator' | 'annotation',
> {
  type: T;
  columnInfo: ColumnInfo<T>;
  data?: ColumnRecordMap[T];
}
