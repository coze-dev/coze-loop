// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */

import ReactJsonView from 'react-json-view';

import { SpanStatus } from '@cozeloop/api-schema/observation';
import { Tag } from '@coze-arch/coze-design';

import {
  formatNumberWithCommas,
  formatTimestampToString,
} from '@/shared/utils/format';
import { dayJsTimeZone } from '@/shared/utils/dayjs';
import { LatencyTag } from '@/shared/ui/time-tag';
import { TableHeaderText } from '@/shared/ui/table-header-text';
import { CustomTableTooltip } from '@/shared/ui/table-cell-text';
import { useLocale } from '@/i18n';
import { jsonFormat } from '@/features/trace-list/utils/json';
import { type Span, type ConvertSpan } from '@/features/trace-list/types/span';
import { BUILD_IN_COLUMN, type ColumnItem } from '@/features/trace-list/types';
import { useCustomComponents } from '@/features/trace-list/hooks/use-custom-components';
import { QUERY_PROPERTY } from '@/features/trace-list/constants/trace-attrs';
import { jsonViewerConfig } from '@/features/trace-list/constants/json-view';
import { QUERY_PROPERTY_LABEL_MAP } from '@/features/trace-list/constants/filter';
import { useConfigContext } from '@/config-provider';

interface StatusProps {
  record: Span;
}
const Status: React.FC<StatusProps> = ({ record }) => {
  const { StatusSuccessIcon, StatusErrorIcon } = useCustomComponents();
  switch (record?.status) {
    case SpanStatus.Success:
      return (
        <Tag
          prefixIcon={<StatusSuccessIcon className="!w-3 !h-3" />}
          color="green"
          size="mini"
          className="flex items-center justify-center text-xs !w-5 !h-5 !rounded-[4px]"
        />
      );
    case SpanStatus.Error:
      return (
        <Tag
          prefixIcon={<StatusErrorIcon className="!w-3 !h-3" />}
          color="red"
          size="mini"
          className="flex items-center justify-center text-xs !w-5 !h-5 !rounded-[4px]"
        />
      );
    default:
      return <div>{record?.status}</div>;
  }
};

export const useTableCols = () => {
  const { t } = useLocale();

  const StatusItem: ColumnItem = {
    value: BUILD_IN_COLUMN.Status,
    key: BUILD_IN_COLUMN.Status,
    title: '',
    dataIndex: QUERY_PROPERTY.Status,
    width: 60,
    autoSizedDisabled: true,
    render: (_: string, record: Span) => <Status record={record} />,
    disabled: true,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Status]),
  };

  const TraceId = (value: string, _record: Span) => (
    <CustomTableTooltip enableCopy copyText={value}>
      {value || '-'}
    </CustomTableTooltip>
  );

  const TraceIdItem: ColumnItem = {
    value: BUILD_IN_COLUMN.TraceId,
    key: BUILD_IN_COLUMN.TraceId,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.TraceId])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.TraceId,
    width: 158,
    disabled: true,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.TraceId]),
    render: TraceId,
  };

  const Input = (_text: string, record: Span) => {
    if (record.system_tags?.is_encryption_data === 'true') {
      return (
        <div className="w-full h-full flex items-center">
          <span>******</span>
        </div>
      );
    }
    const content = jsonFormat(record?.input ?? '');
    const inputStr =
      typeof record.input === 'string' ? record.input : (record.input ?? '-');
    return (
      <div className="rounded-md bg-semi-info-light-default p-1 w-full overflow-hidden">
        <CustomTableTooltip
          opts={{
            position: 'bottom',
          }}
          content={
            typeof content === 'object' ? (
              <div
                onClick={e => e.stopPropagation()}
                className="w-[400px] max-h-[320px] overflow-y-auto"
              >
                <ReactJsonView src={content as object} {...jsonViewerConfig} />
              </div>
            ) : (
              <span>{inputStr}</span>
            )
          }
        >
          {inputStr}
        </CustomTableTooltip>
      </div>
    );
  };

  const InputItem: ColumnItem = {
    value: BUILD_IN_COLUMN.Input,
    key: BUILD_IN_COLUMN.Input,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Input])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.Input,
    width: 320,
    disabled: true,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Input]),
    render: Input,
  };

  const Output = (_: string, record: Span) => {
    if (record.system_tags?.is_encryption_data === 'true') {
      return (
        <div className="w-full h-full flex items-center">
          <span>******</span>
        </div>
      );
    }
    const content = jsonFormat(record?.output ?? '');
    const outputStr =
      typeof record.output === 'string'
        ? record.output
        : (record.output ?? '-');
    return (
      <div className="rounded-md bg-semi-success-light-default p-1 w-full overflow-hidden">
        <CustomTableTooltip
          opts={{
            position: 'bottom',
          }}
          content={
            typeof content === 'object' ? (
              <div
                onClick={e => e.stopPropagation()}
                className="w-[400px] max-h-[320px] overflow-y-auto"
              >
                <ReactJsonView src={content as object} {...jsonViewerConfig} />
              </div>
            ) : (
              <span>{outputStr}</span>
            )
          }
        >
          {outputStr}
        </CustomTableTooltip>
      </div>
    );
  };

  const OutputItem: ColumnItem = {
    value: BUILD_IN_COLUMN.Output,
    key: BUILD_IN_COLUMN.Output,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Output])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.Output,
    width: 320,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Output]),
    render: Output,
  };

  const Tokens = (_: string, record: any) => {
    let tokens = record.custom_tags?.tokens;
    if (record.tokens) {
      tokens =
        Number(record.tokens?.input ?? 0) + Number(record.tokens?.output ?? 0);
    }

    return (
      <CustomTableTooltip textAlign="right">
        {tokens !== undefined ? formatNumberWithCommas(Number(tokens)) : '-'}
      </CustomTableTooltip>
    );
  };

  const TokensItem: ColumnItem = {
    value: BUILD_IN_COLUMN.Tokens,
    key: BUILD_IN_COLUMN.Tokens,
    title: (
      <TableHeaderText align="right">
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Tokens])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.Tokens,
    width: 108,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Tokens]),
    render: Tokens,
  };

  const LatencyItem: ColumnItem = {
    value: BUILD_IN_COLUMN.Latency,
    key: BUILD_IN_COLUMN.Latency,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Latency])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.Latency,
    width: 108,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Latency]),
    render: (_: string, record: Span) => (
      <LatencyTag latency={record.duration} />
    ),
  };

  const LatencyFirst = (_: string, record: Span) => {
    const latencyFirst = record.custom_tags?.latency_first_resp;
    return (
      <LatencyTag latency={latencyFirst ? Number(latencyFirst) : undefined} />
    );
  };

  const LatencyFirstItem: ColumnItem = {
    value: BUILD_IN_COLUMN.LatencyFirst,
    key: BUILD_IN_COLUMN.LatencyFirst,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.LatencyFirst])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.LatencyFirst,
    width: 150,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.LatencyFirst]),
    render: LatencyFirst,
  };

  const StartTimeCell = ({ record }: { record: Span }) => {
    const configContext = useConfigContext();
    return (
      <CustomTableTooltip>
        {record.started_at
          ? dayJsTimeZone(
              Number(record.started_at),
              configContext.timeZone,
            ).format('MM-DD HH:mm:ss')
          : '-'}
      </CustomTableTooltip>
    );
  };

  const StartTime = (_: string, record: Span) => (
    <StartTimeCell record={record} />
  );

  const StartTimeItem: ColumnItem = {
    value: BUILD_IN_COLUMN.StartTime,
    key: BUILD_IN_COLUMN.StartTime,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.StartTime])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.StartTime,
    width: 146,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.StartTime]),
    render: StartTime,
  };

  const InputTokens = (_: string, record: ConvertSpan) => {
    let tokens = record.custom_tags?.input_tokens;

    if (record.tokens) {
      tokens = record.tokens.input;
    }
    return (
      <CustomTableTooltip textAlign="right">
        {tokens !== undefined ? formatNumberWithCommas(Number(tokens)) : '-'}
      </CustomTableTooltip>
    );
  };

  const InputTokensItem: ColumnItem = {
    value: BUILD_IN_COLUMN.InputTokens,
    key: BUILD_IN_COLUMN.InputTokens,
    title: (
      <TableHeaderText align="right">
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.InputTokens])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.InputTokens,
    width: 120,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.InputTokens]),
    render: InputTokens,
  };

  const OutputTokens = (_: string, record: ConvertSpan) => {
    let tokens = record.custom_tags?.output_tokens;

    if (record.tokens) {
      tokens = record.tokens.output;
    }
    return (
      <CustomTableTooltip textAlign="right">
        {tokens !== undefined ? formatNumberWithCommas(Number(tokens)) : '-'}
      </CustomTableTooltip>
    );
  };

  const OutputTokensItem: ColumnItem = {
    value: BUILD_IN_COLUMN.OutputTokens,
    key: BUILD_IN_COLUMN.OutputTokens,
    title: (
      <TableHeaderText align="right">
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.OutputTokens])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.OutputTokens,
    width: 136,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.OutputTokens]),
    render: OutputTokens,
  };

  const SpanId = (_text: string, record: Span) => (
    <CustomTableTooltip copyText={record.span_id}>
      {record.span_id || '-'}
    </CustomTableTooltip>
  );

  const SpanIdItem: ColumnItem = {
    value: BUILD_IN_COLUMN.SpanId,
    key: BUILD_IN_COLUMN.SpanId,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanId])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.SpanId,
    width: 120,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanId]),
    render: SpanId,
  };

  const SpanType = (_: string, record: Span) => (
    <CustomTableTooltip enableCopy copyText={record.span_type}>
      {record?.span_type || '-'}
    </CustomTableTooltip>
  );

  const SpanTypeItem: ColumnItem = {
    value: BUILD_IN_COLUMN.SpanType,
    key: BUILD_IN_COLUMN.SpanType,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanType])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.SpanType,
    width: 158,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanType]),
    render: SpanType,
  };

  const SpanName = (_: string, record: Span) => (
    <CustomTableTooltip enableCopy copyText={record.span_name}>
      {record.span_name || '-'}
    </CustomTableTooltip>
  );

  const SpanNameItem: ColumnItem = {
    value: BUILD_IN_COLUMN.SpanName,
    key: BUILD_IN_COLUMN.SpanName,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanName])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.SpanName,
    width: 126,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanName]),
    render: SpanName,
  };

  const PromptKey = (_: string, record: Span) => (
    <CustomTableTooltip>
      {record.custom_tags?.prompt_key || '-'}
    </CustomTableTooltip>
  );

  const PromptKeyItem: ColumnItem = {
    value: BUILD_IN_COLUMN.PromptKey,
    key: BUILD_IN_COLUMN.PromptKey,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.PromptKey])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.PromptKey,
    width: 110,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.PromptKey]),
    render: PromptKey,
  };

  const LogicDeleteDate = (_: string, record: Span) => {
    const logicDeleteDate = record?.logic_delete_date;
    return (
      <CustomTableTooltip>
        {logicDeleteDate !== undefined
          ? formatTimestampToString((Number(logicDeleteDate) / 1000).toFixed(0))
          : '-'}
      </CustomTableTooltip>
    );
  };

  const LogicDeleteDateItem: ColumnItem = {
    value: BUILD_IN_COLUMN.LogicDeleteDate,
    key: BUILD_IN_COLUMN.LogicDeleteDate,
    title: (
      <TableHeaderText>
        {t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.LogicDeleteDate])}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.LogicDeleteDate,
    width: 176,
    checked: true,
    displayName: t(QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.LogicDeleteDate]),
    render: LogicDeleteDate,
  };

  return {
    [BUILD_IN_COLUMN.Status]: StatusItem,
    [BUILD_IN_COLUMN.TraceId]: TraceIdItem,
    [BUILD_IN_COLUMN.Input]: InputItem,
    [BUILD_IN_COLUMN.Output]: OutputItem,
    [BUILD_IN_COLUMN.Tokens]: TokensItem,
    [BUILD_IN_COLUMN.InputTokens]: InputTokensItem,
    [BUILD_IN_COLUMN.OutputTokens]: OutputTokensItem,
    [BUILD_IN_COLUMN.Latency]: LatencyItem,
    [BUILD_IN_COLUMN.LatencyFirst]: LatencyFirstItem,
    [BUILD_IN_COLUMN.SpanId]: SpanIdItem,
    [BUILD_IN_COLUMN.SpanName]: SpanNameItem,
    [BUILD_IN_COLUMN.SpanType]: SpanTypeItem,
    [BUILD_IN_COLUMN.PromptKey]: PromptKeyItem,
    [BUILD_IN_COLUMN.LogicDeleteDate]: LogicDeleteDateItem,
    [BUILD_IN_COLUMN.StartTime]: StartTimeItem,
  };
};
