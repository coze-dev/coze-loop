// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useMemo } from 'react';

import classNames from 'classnames';
import { TypographyText, TagGroup } from '@cozeloop/shared-components';
import { type EvaluatorTemplate } from '@cozeloop/api-schema/evaluation';
import { IconCozFireFill } from '@coze-arch/coze-design/icons';
import { type TagProps } from '@coze-arch/coze-design';

import { getEvaluatorTagList } from './utils';

// const getDefaultOutputSchemas = (): ArgsSchema[] => [
//   {
//     key: 'score',
//     json_schema: JSON.stringify({ type: 'number' }),
//   },
//   {
//     key: 'reason',
//     json_schema: JSON.stringify({ type: 'string' }),
//   },
// ];

export function EvaluatorTemplateCard({
  evaluatorTemplate,
  className,
  style,
  getCardHeaderActions,
  onClick,
}: {
  evaluatorTemplate: EvaluatorTemplate;
  className?: string;
  style?: React.CSSProperties;
  getCardHeaderActions?: (template: EvaluatorTemplate) => React.ReactNode;
  onClick?: (evaluatorTemplate: EvaluatorTemplate) => void;
}) {
  const { name, description, popularity, tags } = evaluatorTemplate ?? {};

  const { tagList } = useMemo(() => getEvaluatorTagList(tags), [tags]);

  return (
    <div
      style={style}
      className={classNames(
        'flex flex-col gap-[10px] group rounded-lg border border-solid border-[var(--coz-stroke-primary)] p-4 bg-white overflow-hidden cursor-pointer hover:shadow-[0_4px_12px_0_rgba(0,0,0,0.08),_0_8px_24px_0_rgba(0,0,0,0.04)]',
        className,
      )}
      onClick={e => {
        const selectedText = window.getSelection()?.toString() || '';
        // 点击时如果有选中文本，则不触发点击事件
        if (selectedText) {
          return;
        }
        onClick?.(evaluatorTemplate);
      }}
    >
      <div className="flex items-center">
        <TypographyText className="!text-[16px] !font-medium !coz-fg-primary mr-2">
          {name}
        </TypographyText>
        {popularity !== undefined ? (
          <div className="coz-fg-dim flex items-center gap-1">
            <IconCozFireFill className="coz-fg-dim" />
            {popularity > 10000
              ? '10k+'
              : popularity > 1000
                ? `${(popularity / 1000).toFixed(1)}k`
                : popularity}
          </div>
        ) : null}
        {getCardHeaderActions ? (
          <div
            className="group-hover:visible invisible flex items-center gap-[6px] ml-auto"
            onClick={e => {
              e.stopPropagation();
            }}
          >
            {getCardHeaderActions?.(evaluatorTemplate)}
          </div>
        ) : null}
      </div>
      {/* 卡片描述 */}
      <div className="coz-fg-secondary text-sm h-10">
        <TypographyText ellipsis={{ rows: 2 }}>
          {description ?? '-'}
        </TypographyText>
      </div>

      <TagGroup tagList={tagList as TagProps[]} showPopover={true} />

      {/* 输入输出部分 */}
      {/* <div>
        <div className="mb-2 flex">
          <span
            className="w-10 text-xs coz-fg-dim"
            style={{ lineHeight: '20px' }}
          >
            {I18n.t('input')}
          </span>
          <div className="flex flex-wrap gap-2">
            {input_schemas?.map(schema => (
              <Tag key={schema.key} size="small" color="primary">
                {schema.key}
              </Tag>
            )) || '-'}
          </div>
        </div>

        <div className="flex">
          <span
            className="w-10 text-xs coz-fg-dim"
            style={{ lineHeight: '20px' }}
          >
            {I18n.t('output')}
          </span>
          <div className="flex flex-wrap gap-2">
            {output_schemas?.map(schema => (
              <Tag
                key={schema.key}
                size="small"
                color="primary"
                onClick={e => e.stopPropagation()}
              >
                {schema.key}
              </Tag>
            )) || '-'}
          </div>
        </div>
      </div> */}

      {/* 底部信息 */}
      {/* <EvaluatorTemplateInfo
        className="mt-auto"
        providerClassName="ml-auto"
        template={evaluatorTemplate}
      /> */}
    </div>
  );
}
