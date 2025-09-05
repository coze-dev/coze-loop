// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import ReactJsonView from 'react-json-view';

import { isObject } from 'lodash-es';
import { formatTimestampToString } from '@cozeloop/toolkit';
import {
  IconCozCheckMarkCircleFillPalette,
  IconCozCrossCircleFill,
} from '@coze-arch/coze-design/icons';
import { Tag } from '@coze-arch/coze-design';

import { jsonFormat } from '@/utils/json';
import dayjs from '@/utils/dayjs';
import { formatNumberWithCommas } from '@/utils/basic';
import { type Span } from '@/typings/span';
import { QUERY_PROPERTY } from '@/consts/trace-attrs';
import { jsonViewerConfig } from '@/consts/json-view';
import { QUERY_PROPERTY_LABEL_MAP } from '@/consts/filter';
import { LatencyTag } from '@/components/time-tag';
import { TableHeaderText } from '@/components/table-header-text';
import { CustomTableTooltip } from '@/components/table-cell-text';

export const COLUMN_RECORD = {
  [QUERY_PROPERTY.Status]: {
    title: '',
    dataIndex: QUERY_PROPERTY.Status,
    width: 60,
    autoSizedDisabled: true,
    render: (_text, record) => {
      switch (record?.status) {
        case 'success':
          return (
            <Tag
              prefixIcon={
                <IconCozCheckMarkCircleFillPalette className="!w-3 !h-3" />
              }
              color="green"
              size="mini"
              className="flex items-center justify-center text-xs !w-5 !h-5 !rounded-[4px]"
            ></Tag>
          );
        case 'error':
          return (
            <Tag
              prefixIcon={<IconCozCrossCircleFill className="!w-3 !h-3" />}
              color="red"
              size="mini"
              className="flex items-center justify-center text-xs !w-5 !h-5 !rounded-[4px]"
            ></Tag>
          );
        default:
          return <div>{record?.status}</div>;
      }
    },
    disabled: true,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Status],
  },
  [QUERY_PROPERTY.TraceId]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.TraceId]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.TraceId,
    width: 158,
    disabled: true,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.TraceId],
    render: value => (
      <CustomTableTooltip enableCopy copyText={value}>
        {value || '-'}
      </CustomTableTooltip>
    ),
  },
  [QUERY_PROPERTY.Input]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Input]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.Input,
    width: 320,
    disabled: true,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Input],
    render: (_text, record: Span) => {
      const content = jsonFormat(record?.input ?? '');
      return (
        <div className="rounded-md bg-semi-info-light-default p-1 w-full overflow-hidden">
          <CustomTableTooltip
            opts={{
              position: 'bottom',
            }}
            content={
              isObject(content) ? (
                <div
                  onClick={e => e.stopPropagation()}
                  className="w-[400px] max-h-[320px] overflow-y-auto"
                >
                  <ReactJsonView src={content} {...jsonViewerConfig} />
                </div>
              ) : (
                record?.input
              )
            }
          >
            {record?.input || '-'}
          </CustomTableTooltip>
        </div>
      );
    },
  },

  [QUERY_PROPERTY.Output]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Output]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.Output,
    width: 320,
    render: (_, record: Span) => {
      const content = jsonFormat(record?.output ?? '');
      return (
        <div className="rounded-md bg-semi-success-light-default p-1 w-full overflow-hidden">
          <CustomTableTooltip
            opts={{
              position: 'bottom',
            }}
            content={
              isObject(content) ? (
                <div
                  onClick={e => e.stopPropagation()}
                  className="w-[400px] max-h-[320px] overflow-y-auto"
                >
                  <ReactJsonView src={content} {...jsonViewerConfig} />
                </div>
              ) : (
                record?.output
              )
            }
          >
            {record?.output || '-'}
          </CustomTableTooltip>
        </div>
      );
    },
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Output],
  },

  [QUERY_PROPERTY.Tokens]: {
    title: (
      <TableHeaderText align="right">
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Tokens]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.Tokens,
    width: 108,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Tokens],
    render: (_, record) => {
      let tokens = record.custom_tags?.tokens;

      if (record.spanType === 'root_span' && record.tokens) {
        tokens =
          Number(record.tokens?.input ?? 0) +
          Number(record.tokens?.output ?? 0);
      }
      return (
        <CustomTableTooltip textAlign="right">
          {tokens !== undefined ? formatNumberWithCommas(Number(tokens)) : '-'}
        </CustomTableTooltip>
      );
    },
  },

  [QUERY_PROPERTY.Latency]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Latency]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.Latency,
    width: 108,
    render: (_, record) => <LatencyTag latency={record.duration} />,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.Latency],
  },

  [QUERY_PROPERTY.LatencyFirst]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.LatencyFirst]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.LatencyFirst,
    width: 125,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.LatencyFirst],
    render: (_, record) => {
      const latencyFirst = record.custom_tags?.latency_first_resp;
      return (
        <LatencyTag latency={latencyFirst ? Number(latencyFirst) : undefined} />
      );
    },
  },
  [QUERY_PROPERTY.StartTime]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.StartTime]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.StartTime,
    width: 146,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.StartTime],
    render: (_, record) => (
      <CustomTableTooltip>
        {record.started_at
          ? dayjs(Number(record.started_at)).format('MM-DD HH:mm:ss')
          : '-'}
      </CustomTableTooltip>
    ),
  },

  [QUERY_PROPERTY.InputTokens]: {
    title: (
      <TableHeaderText align="right">
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.InputTokens]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.InputTokens,
    width: 110,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.InputTokens],
    render: (_, record) => {
      let tokens = record.custom_tags?.input_tokens;

      if (record.spanType === 'root_span' && record.tokens) {
        tokens = record.tokens.input;
      }
      return (
        <CustomTableTooltip textAlign="right">
          {tokens !== undefined ? formatNumberWithCommas(Number(tokens)) : '-'}
        </CustomTableTooltip>
      );
    },
  },
  [QUERY_PROPERTY.OutputTokens]: {
    title: (
      <TableHeaderText align="right">
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.OutputTokens]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.OutputTokens,
    width: 110,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.OutputTokens],
    render: (_, record) => {
      let tokens = record.custom_tags?.output_tokens;

      if (record.spanType === 'root_span' && record.tokens) {
        tokens = record.tokens.output;
      }
      return (
        <CustomTableTooltip textAlign="right">
          {tokens !== undefined ? formatNumberWithCommas(Number(tokens)) : '-'}
        </CustomTableTooltip>
      );
    },
  },
  [QUERY_PROPERTY.SpanId]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanId]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.SpanId,
    width: 120,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanId],
    render: (_text, record) => (
      <CustomTableTooltip copyText={record.span_id}>
        {record.span_id || '-'}
      </CustomTableTooltip>
    ),
  },
  [QUERY_PROPERTY.SpanType]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanType]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.SpanType,
    width: 120,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanType],
    render: (_, record) => (
      <CustomTableTooltip enableCopy copyText={record.span_type}>
        {record?.span_type || '-'}
      </CustomTableTooltip>
    ),
  },
  [QUERY_PROPERTY.SpanName]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanName]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.SpanName,
    width: 120,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.SpanName],
    render: (_, record) => (
      <CustomTableTooltip enableCopy copyText={record.span_name}>
        {record.span_name || '-'}
      </CustomTableTooltip>
    ),
  },
  [QUERY_PROPERTY.PromptKey]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.PromptKey]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.PromptKey,
    width: 110,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.PromptKey],
    render: (_, record) => (
      <CustomTableTooltip copyText={record.name}>
        {record.custom_tags?.prompt_key || '-'}
      </CustomTableTooltip>
    ),
  },

  [QUERY_PROPERTY.LogicDeleteDate]: {
    title: (
      <TableHeaderText>
        {QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.LogicDeleteDate]}
      </TableHeaderText>
    ),
    dataIndex: QUERY_PROPERTY.LogicDeleteDate,
    width: 176,
    checked: true,
    displayName: QUERY_PROPERTY_LABEL_MAP[QUERY_PROPERTY.LogicDeleteDate],
    render: (_, record: Span) => {
      const logicDeleteDate = record?.logic_delete_date;
      return (
        <CustomTableTooltip>
          {logicDeleteDate !== undefined
            ? formatTimestampToString(
                (Number(logicDeleteDate) / 1000).toFixed(0),
              )
            : '-'}
        </CustomTableTooltip>
      );
    },
  },
};
