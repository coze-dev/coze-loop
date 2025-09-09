// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-params */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-magic-numbers */
import { useEffect, useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { Resizable } from 're-resizable';
import classNames from 'classnames';
import { useSize } from 'ahooks';
import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { DevLayout } from '@cozeloop/prompt-components';
import {
  IconCozColumnCollapse,
  IconCozColumnExpand,
  IconCozInfoCircle,
  IconCozSideCollapse,
  IconCozSideExpand,
} from '@coze-arch/coze-design/icons';
import { Divider, IconButton, Space, Tooltip } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { useBasicStore } from '@/store/use-basic-store';

import { VariablesCard } from '../variables-card';
import { ToolsCard } from '../tools-card';
import { PromptEditorCard } from '../prompt-editor-card';
import { ModelConfigCard } from '../model-config-card';
import { ExecuteArea } from '../execute-area';

export function NormalArea() {
  const { versionChangeVisible } = useBasicStore(
    useShallow(state => ({ versionChangeVisible: state.versionChangeVisible })),
  );
  const { variables, promptInfo } = usePromptStore(
    useShallow(state => ({
      variables: state.variables,
      tools: state.tools,
      promptInfo: state.promptInfo,
    })),
  );
  const editContainerRef = useRef(null);
  const windowSize = useSize(document.body);
  const isSmallWindowOpenVersion =
    windowSize?.width && windowSize.width < 1600 && versionChangeVisible;
  const size = useSize(editContainerRef.current);
  const minWidth = size?.width ? size.width - 350 : '50%';
  const [configAreaVisible, setConfigAreaVisible] = useState(
    Boolean(localStorage.getItem('configAreaVisible') !== 'false'),
  );

  const [configExecuteVisible, setConfigExecuteVisible] = useState(
    Boolean(localStorage.getItem('configExecuteVisible') !== 'false'),
  );

  const [arrangeWidth, setArrangeWidth] = useState<Int64>('65%');
  const [promptEditorWidth, setPromptEditorWidth] = useState<Int64>('65%');

  const [effectChange, setEffectChange] = useState(false);

  useEffect(() => {
    setEffectChange(true);
    if (configAreaVisible && configExecuteVisible) {
      setArrangeWidth('65%');
      setPromptEditorWidth('65%');
    } else if (configAreaVisible && !configExecuteVisible) {
      setArrangeWidth('100%');
      setPromptEditorWidth('65%');
    } else if (!configAreaVisible && configExecuteVisible) {
      setArrangeWidth('50%');
      setPromptEditorWidth('100%');
    } else if (!configAreaVisible && !configExecuteVisible) {
      setArrangeWidth('100%');
      setPromptEditorWidth('100%');
    }
    setTimeout(() => {
      setEffectChange(false);
    }, 400);
    localStorage.setItem('configAreaVisible', `${configAreaVisible}`);
    localStorage.setItem('configExecuteVisible', `${configExecuteVisible}`);
  }, [configAreaVisible, configExecuteVisible]);

  return (
    <div className="flex flex-1 overflow-hidden w-full">
      <Resizable
        size={{
          width: arrangeWidth,
          height: '100%',
        }}
        minWidth="715px"
        maxWidth={
          isSmallWindowOpenVersion || !configExecuteVisible ? '100%' : '65%'
        }
        enable={{
          right:
            isSmallWindowOpenVersion || !configExecuteVisible ? false : true,
        }}
        handleStyles={{
          right: {
            width: '4px',
            right: '-2px',
          },
        }}
        handleComponent={{
          right: isSmallWindowOpenVersion ? (
            <div />
          ) : (
            <div className="w-[2px] h-full border-0 border-solid border-brand-9 hover:border-l-2"></div>
          ),
        }}
        className={classNames('flex flex-col', {
          '!w-full': isSmallWindowOpenVersion,
          'transition-all': effectChange,
        })}
        onResizeStop={(_e, _dir, _ref, d) => {
          setArrangeWidth(w => `calc(${w} + ${d.width}px)`);
        }}
      >
        <DevLayout
          title="编排"
          actionBtns={
            <Space spacing="tight">
              <Tooltip
                theme="dark"
                content={
                  configAreaVisible
                    ? '收起模型配置与变量区'
                    : '展开模型配置与变量区'
                }
              >
                <IconButton
                  size="mini"
                  color="primary"
                  onClick={() => {
                    sendEvent(EVENT_NAMES.cozeloop_pe_column_collapse, {
                      prompt_id: `${promptInfo?.id || 'playground'}`,
                      type: configAreaVisible ? 1 : 0,
                    });
                    setConfigAreaVisible(v => !v);
                  }}
                  icon={
                    configAreaVisible ? (
                      <IconCozColumnCollapse />
                    ) : (
                      <IconCozColumnExpand />
                    )
                  }
                />
              </Tooltip>
              {configExecuteVisible ? null : (
                <Tooltip theme="dark" content="展开预览与调试">
                  <IconButton
                    size="mini"
                    color="primary"
                    onClick={() => {
                      sendEvent(EVENT_NAMES.cozeloop_pe_column_collapse, {
                        prompt_id: `${promptInfo?.id || 'playground'}`,
                        type: 4,
                      });
                      setConfigExecuteVisible(true);
                    }}
                    icon={<IconCozSideExpand />}
                  />
                </Tooltip>
              )}
            </Space>
          }
        >
          <div className="flex-1 flex overflow-hidden" ref={editContainerRef}>
            <Resizable
              size={{
                width: promptEditorWidth,
                height: '100%',
              }}
              minWidth="50%"
              maxWidth={configAreaVisible ? minWidth : '100%'}
              enable={{ right: configAreaVisible }}
              handleComponent={{
                right: !configAreaVisible ? (
                  <div />
                ) : (
                  <div className="w-[5px] h-full ml-[3px] border-0 border-solid border-brand-9 hover:border-l-2"></div>
                ),
              }}
              onResizeStop={(_e, _dir, _ref, d) => {
                setPromptEditorWidth(w => `calc(${w} + ${d.width}px)`);
              }}
              className={classNames(
                'p-6 w-full overflow-y-auto overflow-x-hidden bg-[#fcfcff]',
                {
                  'transition-all': effectChange,
                },
              )}
            >
              <PromptEditorCard />
            </Resizable>

            <div
              className={classNames(
                ' p-6 pr-[18px] box-border border-0 border-l border-solid flex flex-col gap-3 overflow-y-auto overflow-x-hidden bg-[#fcfcff] flex-1 flex-shrink-0 min-w-[350px] styled-scrollbar',
                {
                  '!hidden': !configAreaVisible,
                },
              )}
            >
              <ModelConfigCard />
              <Divider />
              <VariablesCard defaultVisible key={variables?.length} />
              <Divider />
              <ToolsCard defaultVisible />
            </div>
          </div>
        </DevLayout>
      </Resizable>

      <DevLayout
        className={classNames(
          'transition-all flex flex-col flex-1 flex-shrink-0 border-0 border-l border-solid ',
          {
            '!hidden': isSmallWindowOpenVersion || !configExecuteVisible,
          },
        )}
        style={{ minWidth: '35%' }}
        title={
          <div className="flex items-center gap-2">
            <span>预览与调试</span>
            {promptInfo?.id ? null : (
              <Tooltip
                theme="dark"
                content="历史图片消息1天后过期，可查看 Trace 或前往 Prompt 开发页面调试获取图片信息"
              >
                <IconCozInfoCircle className="cursor-pointer" />
              </Tooltip>
            )}
          </div>
        }
        actionBtns={
          <Tooltip theme="dark" content="收起预览与调试">
            <IconButton
              size="mini"
              color="primary"
              onClick={() => {
                sendEvent(EVENT_NAMES.cozeloop_pe_column_collapse, {
                  prompt_id: `${promptInfo?.id || 'playground'}`,
                  type: 3,
                });
                setConfigExecuteVisible(false);
              }}
              icon={<IconCozSideCollapse />}
            />
          </Tooltip>
        }
      >
        <ExecuteArea />
      </DevLayout>
    </div>
  );
}
