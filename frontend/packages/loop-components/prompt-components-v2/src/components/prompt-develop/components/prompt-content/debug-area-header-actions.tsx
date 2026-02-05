// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useShallow } from 'zustand/react/shallow';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import {
  IconCozChatPlus,
  IconCozHistory,
  IconCozSideCollapse,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  IconButton,
  Select,
  Space,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import {
  getButtonDisabledFromConfig,
  getButtonHiddenFromConfig,
} from '@/utils/base';
import { useBasicStore } from '@/store/use-basic-store';
import { EVENT_NAMES, MessageListRoundType } from '@/consts';

import { usePromptDevProviderContext } from '../prompt-provider';

export function DebugAreaHeaderActions({
  promptInfo,
  addMessage,
  setConfigExecuteVisible,
}: {
  promptInfo?: Prompt;
  configExecuteVisible: boolean;
  addMessage: () => void;
  setConfigExecuteVisible: (visible: boolean) => void;
}) {
  const { debugAreaConfig, sendEvent, buttonConfig } =
    usePromptDevProviderContext();

  const { roundType, setRoundType } = useBasicStore(
    useShallow(state => ({
      roundType: state.roundType,
      setRoundType: state.setRoundType,
    })),
  );
  return (
    <>
      <Space spacing={8}>
        {debugAreaConfig?.canSignleRound ? (
          <Select
            size="small"
            value={roundType}
            style={{ height: 24, minHeight: 24, padding: '1px 0 1px 4px' }}
            renderSelectedItem={item => (
              <Typography.Text size="small">{item.label}</Typography.Text>
            )}
            onChange={v => setRoundType(v as MessageListRoundType)}
          >
            <Select.Option value={MessageListRoundType.Multi}>
              {I18n.t('prompt_multi_turn_conversation')}
            </Select.Option>
            <Select.Option value={MessageListRoundType.Single}>
              {I18n.t('prompt_single_turn_conversation')}
            </Select.Option>
          </Select>
        ) : null}

        {debugAreaConfig?.canAddMessage ? (
          <Tooltip theme="dark" content={I18n.t('prompt_new_message')}>
            <IconButton
              size="mini"
              icon={<IconCozChatPlus fontSize={14} />}
              color="secondary"
              onClick={addMessage}
            />
          </Tooltip>
        ) : null}

        {getButtonHiddenFromConfig(buttonConfig?.traceHistoryButton) ? null : (
          <Button
            size="mini"
            icon={<IconCozHistory fontSize={14} />}
            color="secondary"
            onClick={() => {
              if (buttonConfig?.traceHistoryButton?.onClick) {
                buttonConfig?.traceHistoryButton?.onClick?.({
                  prompt: promptInfo,
                });
              }
            }}
            disabled={getButtonDisabledFromConfig(
              buttonConfig?.traceHistoryButton,
            )}
            data-btm="d84335"
            data-btm-title={I18n.t('debug_history')}
          >
            {I18n.t('debug_history')}
          </Button>
        )}

        <Tooltip theme="dark" content={I18n.t('collapse_preview_and_debug')}>
          <IconButton
            size="mini"
            color="secondary"
            onClick={() => {
              sendEvent?.(EVENT_NAMES.cozeloop_pe_column_collapse, {
                prompt_id: `${promptInfo?.id || 'playground'}`,
                type: 3,
              });
              setConfigExecuteVisible(false);
            }}
            icon={<IconCozSideCollapse fontSize={14} />}
          />
        </Tooltip>
      </Space>
    </>
  );
}
