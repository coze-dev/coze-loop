// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { TemplateType } from '@cozeloop/api-schema/prompt';
import {
  IconCozArrowDown,
  IconCozQuestionMarkCircle,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Modal,
  Popover,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';

interface TemplateSelectProps {
  streaming?: boolean;
}

export function TemplateSelect({ streaming }: TemplateSelectProps) {
  const [dropVisible, setDropVisible] = useState(false);

  const { templateType, setTemplateType, messageList, setMessageList } =
    usePromptStore(
      useShallow(state => ({
        templateType: state.templateType,
        setTemplateType: state.setTemplateType,
        messageList: state.messageList,
        setMessageList: state.setMessageList,
      })),
    );

  const templateTypeChange = (type: TemplateType) => {
    if (templateType === type) {
      return;
    }

    setDropVisible(false);
    Modal.confirm({
      title: '更换模板引擎',
      content: '可能会导致已有变量渲染失败，请谨慎操作。',
      onOk: () => {
        setTemplateType(type);
        setMessageList([...(messageList || [])]);
      },
      okText: '确认',
      okButtonColor: 'yellow',
      cancelText: '取消',
    });
  };

  return (
    <div className="flex items-center gap-3">
      <Popover
        trigger="custom"
        visible={dropVisible}
        content={
          <div className="px-4 pt-3 pb-4 w-[350px]">
            <Typography.Text strong className="mb-3 block">
              选择模板
            </Typography.Text>

            <div className="flex flex-col gap-2">
              <div
                className={classNames(
                  '!h-fit !px-3 !pt-1.5 !pb-3 border border-solid coz-stroke-primary rounded-lg cursor-pointer hover:bg-[#969fff26]',
                  {
                    'coz-stroke-hglt': templateType === TemplateType.Normal,
                    'bg-[#969fff26]': templateType === TemplateType.Normal,
                  },
                )}
                onClick={() => {
                  templateTypeChange(TemplateType.Normal);
                }}
              >
                <div className="flex flex-col items-start">
                  <Typography.Text strong style={{ lineHeight: '32px' }}>
                    Normal 模板引擎
                  </Typography.Text>
                  <Typography.Text
                    size="small"
                    className="!text-[13px] !leading-[20px] !coz-fg-secondary"
                  >
                    双大括号 {'{{}}'} 识别变量
                  </Typography.Text>
                </div>
              </div>

              <div
                className={classNames(
                  'items-start !h-fit !px-3 !pt-1.5 !pb-3 border border-solid coz-stroke-primary rounded-lg cursor-pointer hover:bg-[#969fff26]',
                  {
                    'coz-stroke-hglt': templateType === TemplateType.Jinja2,
                    'bg-[#969fff26]': templateType === TemplateType.Jinja2,
                  },
                )}
                onClick={() => {
                  templateTypeChange(TemplateType.Jinja2);
                }}
              >
                <div className="flex flex-col items-start">
                  <Typography.Text strong style={{ lineHeight: '32px' }}>
                    Jinja2 模板引擎
                  </Typography.Text>
                  <Typography.Text
                    size="small"
                    className="!text-[13px] !leading-[20px] !coz-fg-secondary flex items-center gap-1"
                  >
                    手动添加和删除变量，支持复杂逻辑
                    <Tooltip
                      content={
                        <>
                          查看
                          <a
                            href="https://loop.coze.cn/open/docs/cozeloop/create-prompt#51f641db"
                            target="_blank"
                            style={{
                              color: '#AAA6FF',
                              textDecoration: 'none',
                            }}
                          >
                            用户手册
                          </a>
                        </>
                      }
                      stopPropagation
                      theme="dark"
                    >
                      <IconCozQuestionMarkCircle />
                    </Tooltip>
                  </Typography.Text>
                </div>
              </div>
            </div>
          </div>
        }
        position="topLeft"
        onClickOutSide={() => setDropVisible(false)}
      >
        <Button
          icon={<IconCozArrowDown />}
          iconPosition="right"
          color="secondary"
          className="!border border-solid coz-stroke-primary"
          onClick={() => !streaming && setDropVisible(true)}
          disabled={streaming}
          size="small"
        >
          <Typography.Text>
            {templateType === TemplateType.Jinja2 ? 'Jinja2' : 'Normal'}
          </Typography.Text>
        </Button>
      </Popover>
    </div>
  );
}
