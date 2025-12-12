/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-params */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-magic-numbers */
import { useCallback, useEffect, useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { Resizable } from 're-resizable';
import { nanoid } from 'nanoid';
import classNames from 'classnames';
import { useSize } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { Role } from '@cozeloop/api-schema/prompt';
import {
  IconCozColumnCollapse,
  IconCozInfoCircle,
  IconCozSideExpand,
} from '@coze-arch/coze-design/icons';
import {
  Divider,
  IconButton,
  Space,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import {
  type DebugMessage,
  usePromptMockDataStore,
} from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { EVENT_NAMES } from '@/consts';
import { PromptDevLayout } from '@/components/prompt-dev-layout';

import { VariablesCard } from '../variables-card';
import { ToolsCard } from '../tools-card';
import { usePromptDevProviderContext } from '../prompt-provider';
import { PromptEditorCard } from '../prompt-editor-card';
import { ModelConfigCard } from '../model-config-card';
import { ExecuteArea } from '../execute-area';
import { DebugAreaHeaderActions } from './debug-area-header-actions';

export function NormalArea() {
  const {
    sendEvent,
    isPlayground,
    extraTabs = [],
  } = usePromptDevProviderContext();
  const { versionChangeVisible } = useBasicStore(
    useShallow(state => ({ versionChangeVisible: state.versionChangeVisible })),
  );

  const { promptInfo } = usePromptStore(
    useShallow(state => ({
      variables: state.variables,
      tools: state.tools,
      promptInfo: state.promptInfo,
    })),
  );

  const { historicMessage = [], setHistoricMessage } = usePromptMockDataStore(
    useShallow(state => ({
      historicMessage: state.historicMessage,
      setHistoricMessage: state.setHistoricMessage,
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

  const showTabs = extraTabs.length > 0 && !isPlayground;

  const addMessage = useCallback(() => {
    const { length } = historicMessage;
    const id = nanoid();
    const chat: DebugMessage = {
      id,
      isEdit: true,
      content: '',
    };
    if (length) {
      const laseItem = historicMessage[length - 1];
      if (laseItem?.role === Role.User) {
        setHistoricMessage(list => [
          ...list,
          {
            ...chat,
            role: Role.Assistant,
          },
        ]);
      } else {
        setHistoricMessage(list => [
          ...list,
          {
            ...chat,
            role: Role.User,
          },
        ]);
      }
    } else {
      setHistoricMessage(list => [
        ...list,
        {
          ...chat,
          role: Role.User,
        },
      ]);
    }
    sendEvent?.(EVENT_NAMES.prompt_insert_mock_msg, {
      prompt_key: promptInfo?.prompt_key || 'playground',
    });
  }, [historicMessage.length]);

  useEffect(() => {
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
        minWidth="800px"
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
        })}
        onResizeStop={(_e, _dir, _ref, d) => {
          setArrangeWidth(w => `calc(${w} + ${d.width}px)`);
        }}
      >
        <div className="flex-1 flex overflow-hidden" ref={editContainerRef}>
          <Resizable
            size={{
              width: promptEditorWidth,
              height: '100%',
            }}
            minWidth="400px"
            maxWidth={configAreaVisible ? minWidth : '100%'}
            enable={{ right: configAreaVisible }}
            handleComponent={{
              right: configAreaVisible ? (
                <div className="w-[5px] h-full ml-[3px] border-0 border-solid border-brand-9 hover:border-l-2"></div>
              ) : (
                <div />
              ),
            }}
            onResizeStop={(_e, _dir, _ref, d) => {
              setPromptEditorWidth(w => `calc(${w} + ${d.width}px)`);
            }}
            className={classNames(
              'pb-6 w-full overflow-hidden bg-[#fcfcff] border-0 border-t border-solid',
              {
                '!border-t-0': showTabs,
              },
            )}
          >
            <PromptEditorCard
              configAreaVisible={configAreaVisible}
              setConfigAreaVisible={setConfigAreaVisible}
              configExecuteVisible={configExecuteVisible}
              setConfigExecuteVisible={setConfigExecuteVisible}
            />
          </Resizable>
          <PromptDevLayout
            data-btm="c30437"
            className="box-border border-0 border-l border-solid flex flex-col gap-1 overflow-hidden bg-[#fcfcff] flex-shrink-0 min-w-[350px] flex-1"
            wrapperClassName={classNames('!px-3', {
              '!border-t-0': showTabs,
            })}
            title={I18n.t('prompt_common_configuration')}
            actionBtns={
              <Space spacing="tight">
                <Tooltip
                  theme="dark"
                  content={I18n.t('collapse_model_and_var_area')}
                >
                  <IconButton
                    size="mini"
                    color="secondary"
                    onClick={() => {
                      sendEvent?.(EVENT_NAMES.cozeloop_pe_column_collapse, {
                        prompt_id: `${promptInfo?.id || 'playground'}`,
                        type: configAreaVisible ? 1 : 0,
                      });
                      setConfigAreaVisible(v => !v);
                    }}
                    icon={<IconCozColumnCollapse fontSize={14} />}
                  />
                </Tooltip>
                {configExecuteVisible ? null : (
                  <Tooltip
                    theme="dark"
                    content={I18n.t('expand_preview_and_debug')}
                  >
                    <IconButton
                      size="mini"
                      color="secondary"
                      onClick={() => {
                        sendEvent?.(EVENT_NAMES.cozeloop_pe_column_collapse, {
                          prompt_id: `${promptInfo?.id || 'playground'}`,
                          type: 4,
                        });
                        setConfigExecuteVisible(true);
                      }}
                      icon={<IconCozSideExpand fontSize={14} />}
                    />
                  </Tooltip>
                )}
              </Space>
            }
          >
            <div
              className={classNames(
                'px-3 pb-6 pr-[6px] styled-scrollbar flex flex-col gap-4',
                {
                  '!hidden': !configAreaVisible,
                },
              )}
            >
              <ModelConfigCard />
              <Divider
                style={{
                  margin: '0 -12px',
                  width: 'calc(100% + 24px)',
                }}
              />

              <VariablesCard defaultVisible />
              <Divider
                style={{
                  margin: '0 -12px',
                  width: 'calc(100% + 24px)',
                }}
              />

              <ToolsCard defaultVisible />
            </div>
          </PromptDevLayout>
        </div>
      </Resizable>

      <PromptDevLayout
        data-btm="c37617"
        className={classNames(
          'transition-all flex flex-col flex-1 gap-3 flex-shrink-0 border-0 border-l border-solid bg-[#fff]',
          {
            '!hidden': isSmallWindowOpenVersion || !configExecuteVisible,
          },
        )}
        wrapperClassName={showTabs ? '!border-t-0' : ''}
        style={{ minWidth: '35%' }}
        title={
          <div className="flex items-center gap-2 font-semibold">
            <Typography.Title heading={6}>
              {I18n.t('preview_and_debug')}
            </Typography.Title>
            {!isPlayground ? null : (
              <Tooltip
                theme="dark"
                content={I18n.t('prompt_historical_image_message_expires_1day')}
              >
                <IconCozInfoCircle className="cursor-pointer" />
              </Tooltip>
            )}
          </div>
        }
        actionBtns={
          <DebugAreaHeaderActions
            promptInfo={promptInfo}
            addMessage={addMessage}
            configExecuteVisible={configExecuteVisible}
            setConfigExecuteVisible={setConfigExecuteVisible}
          />
        }
      >
        <ExecuteArea />
      </PromptDevLayout>
    </div>
  );
}
