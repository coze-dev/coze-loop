// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { getTypeText } from '@cozeloop/evaluate-components';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { Space, Dropdown, Typography, Tag } from '@coze-arch/coze-design';

export function EvaluateSetColList({
  fieldSchemas,
}: {
  fieldSchemas?: FieldSchema[];
}) {
  if (fieldSchemas?.length) {
    return (
      <Space wrap>
        {fieldSchemas.map((item, index) => (
          <Dropdown
            key={index}
            render={
              <div className="w-[150px] overflow-hidden p-3 flex flex-col gap-2">
                <div className="flex items-center">
                  <Typography.Text className="flex-1 !text-[13px]">
                    {I18n.t('data_type')}
                  </Typography.Text>
                  <Typography.Text className="flex-1 !text-[13px] !font-bold">
                    {getTypeText(item)}
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
            <Tag key={item.key} color="primary">
              {item.name}
            </Tag>
          </Dropdown>
        ))}
      </Space>
    );
  }

  return <div className="text-sm coz-fg-primary font-normal">-</div>;
}
