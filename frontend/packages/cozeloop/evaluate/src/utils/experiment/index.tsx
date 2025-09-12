// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { get } from 'lodash-es';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  DatasetItem,
  getFieldColumnConfig,
} from '@cozeloop/evaluate-components';
import {
  EvalTargetType,
  type Content,
  type Experiment,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { Tag, Tooltip, type ColumnProps } from '@coze-arch/coze-design';

import { ActualOutputWithTrace } from '@/components/experiment';

export async function wait(time: number) {
  return new Promise(resolve => {
    setTimeout(() => {
      resolve(true);
    }, time);
  });
}

export function CellContentRender({
  content,
  fieldSchema,
  expand,
  displayFormat,
  className,
}: {
  content: Content | undefined;
  fieldSchema?: FieldSchema;
  expand?: boolean;
  displayFormat?: boolean;
  className?: string;
}) {
  if (!content) {
    return '-';
  }
  return (
    <DatasetItem
      fieldSchema={fieldSchema}
      fieldContent={content}
      expand={expand}
      displayFormat={displayFormat}
      className={classNames(
        'overflow-hidden',
        displayFormat ? '!border-0 !p-0' : '',
        className,
      )}
    />
  );
}

/** 数据集列配置 */
export function getDatasetColumns(
  datasetFields: FieldSchema[],
  options?: { prefix?: string; expand?: boolean },
) {
  const { prefix = '', expand = false } = options ?? {};
  const columns: ColumnProps[] = datasetFields.map(field => {
    const column = getFieldColumnConfig({ field, prefix, expand });
    return column;
  });
  return columns;
}

/** 实际输出列配置 */
export function getActualOutputColumn(params?: {
  expand?: boolean;
  column?: ColumnProps;
  /** traceID的在表格行数据record对象上的字段路径 */
  traceIdPath?: string;
  experiment: Experiment | undefined;
}) {
  const {
    expand = false,
    column = {},
    traceIdPath = '',
    experiment,
  } = params ?? {};
  const newColumn: ColumnProps = {
    title: (
      <div className="flex items-center gap-1">
        <div>actual_output</div>
        <Tooltip
          theme="dark"
          content={I18n.t('evaluation_object_actual_output')}
        >
          <Tag color="primary" className="text-[12px] font-semibold">
            {I18n.t('actual_output')}
          </Tag>
          {/* <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" /> */}
        </Tooltip>
      </div>
    ),
    displayName: 'actual_output',
    dataIndex: 'actualOutput',
    key: 'actualOutput',
    width: 280,
    render(val: Content, record) {
      return (
        <ActualOutputWithTrace
          expand={expand}
          content={val}
          enableTrace={Boolean(traceIdPath)}
          traceID={get(record, traceIdPath)}
          startTime={experiment?.start_time}
          endTime={experiment?.end_time}
        />
      );
    },
    ...column,
  };
  return newColumn;
}

/** 评测对象是否是Trace */
export function isTraceTargetExpr(experiment: Experiment | undefined) {
  return experiment?.eval_target?.eval_target_type === EvalTargetType.Trace;
}
