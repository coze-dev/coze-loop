// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cs from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type FieldSchema,
  type EvaluationSet,
} from '@cozeloop/api-schema/evaluation';
import {
  Dropdown,
  OverflowList,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { getColumnType } from '../dataset-item/util';
import { dataTypeMap } from '../dataset-item/type';

export const ColumnNameListTag = ({ set }: { set: EvaluationSet }) => {
  const version = set.evaluation_set_version;
  const renderOverflow = items =>
    items.length ? (
      <Dropdown
        render={
          <Dropdown.Menu className=" w-[120px] !p-4 max-h-[300px] overflow-y-auto">
            <div
              className="flex justify-between pb-[6px]"
              style={{
                borderBottom: '1px solid var(--semi-color-border)',
              }}
            >
              <Typography.Text>{I18n.t('column')}</Typography.Text>
              <Typography.Text className="!font-medium">
                {items.length}
              </Typography.Text>
            </div>
            <div className="flex w-full overflow-hidden flex-col gap-2 pt-[8px]">
              {items.map(item => renderItem(item, '!w-fit !max-w-full'))}
            </div>
          </Dropdown.Menu>
        }
      >
        <Tag
          className="!rounded-[10px] w-[26px]"
          color="primary"
          onClick={e => {
            e.stopPropagation();
          }}
        >
          +{items.length}
        </Tag>
      </Dropdown>
    ) : null;
  const renderItem = (item: FieldSchema, className?: string) => (
    <Dropdown
      render={
        <div className="max-w-[200px] min-w-[150px] overflow-hidden p-3 flex flex-col gap-2">
          <div className="flex items-center">
            <Typography.Text className="flex-1 !text-[13px]">
              {I18n.t('data_type')}
            </Typography.Text>
            <Typography.Text className="flex-1 !text-[13px] !font-bold">
              {dataTypeMap[getColumnType(item)]}
            </Typography.Text>
          </div>
          <div className="flex items-center ">
            <Typography.Text className="flex-1 !text-[13px]">
              {I18n.t('description')}
            </Typography.Text>
            <Typography.Text
              className="flex-1 !text-[13px] !font-bold"
              ellipsis={{
                showTooltip: {
                  opts: {
                    theme: 'dark',
                  },
                },
              }}
            >
              {item.description || '-'}
            </Typography.Text>
          </div>
        </div>
      }
    >
      <Tag
        key={item.key}
        color="primary"
        className={cs(className)}
        style={{ marginRight: 8, flex: '0 0 auto' }}
      >
        <Typography.Text
          style={{ color: 'inherit', fontSize: 'inherit' }}
          ellipsis={{
            showTooltip: {
              opts: { theme: 'dark' },
            },
          }}
          className="!w-full"
        >
          {item.name}
        </Typography.Text>
      </Tag>
    </Dropdown>
  );
  return (
    <div
      onClick={e => {
        e.stopPropagation();
      }}
    >
      <OverflowList
        items={version?.evaluation_set_schema?.field_schemas ?? []}
        overflowRenderer={renderOverflow}
        visibleItemRenderer={item => renderItem(item)}
      />
    </div>
  );
};
