/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { useRef } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWhenDisabled } from '@cozeloop/components';
import {
  IconCozArrowDown,
  IconCozColumnExpand,
  IconCozEdit,
  IconCozSideExpand,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  IconButton,
  Tooltip,
  Typography,
  Image,
} from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { EVENT_NAMES } from '@/consts';
import { type ModelConfigWithName } from '@/components/model-config-editor/utils';
import { PopoverModelConfigEditor } from '@/components/model-config-editor/popover-model-config-editor';

import { usePromptDevProviderContext } from '../prompt-provider';
import { TemplateSelect } from './template-select';

interface EditorCardHeaderActionsProps {
  disabled?: boolean;
  configAreaVisible?: boolean;
  configExecuteVisible?: boolean;
  inDiffEditor?: boolean;
  setInDiffEditor?: (inDiffEditor: boolean) => void;
  setConfigAreaVisible?: (visible: boolean) => void;
  setConfigExecuteVisible?: (visible: boolean) => void;
}

export function EditorCardHeaderActions({
  disabled,
  inDiffEditor,
  setInDiffEditor,
  configAreaVisible,
  setConfigAreaVisible,
  configExecuteVisible,
  setConfigExecuteVisible,
}: EditorCardHeaderActionsProps) {
  const lastConfigAreaVisible = useRef(configAreaVisible);
  const { promptInfo, modelConfig, setModelConfig, setCurrentModel } =
    usePromptStore(
      useShallow(state => ({
        promptInfo: state.promptInfo,
        modelConfig: state.modelConfig,
        setModelConfig: state.setModelConfig,
        setCurrentModel: state.setCurrentModel,
      })),
    );
  const { compareConfig } = usePromptMockDataStore(
    useShallow(state => ({
      compareConfig: state.compareConfig,
    })),
  );

  const { renderTemplateType, sendEvent, modelInfo, canDiffEdit } =
    usePromptDevProviderContext();

  const isFirstVersion = !promptInfo?.prompt_basic?.latest_version;

  return (
    <div className="flex items-center gap-2">
      {compareConfig?.groups?.length ? null : (
        <>
          {!canDiffEdit ? null : inDiffEditor ? (
            <Button
              size="mini"
              onClick={() => {
                setInDiffEditor?.(false);
                setConfigAreaVisible?.(lastConfigAreaVisible.current ?? true);
              }}
              data-btm="d27366"
              data-btm-title={I18n.t('prompt_exit_diff')}
            >
              {I18n.t('prompt_exit_diff')}
            </Button>
          ) : (
            <TooltipWhenDisabled
              disabled={isFirstVersion}
              content={I18n.t('prompt_no_submitted_versions_no_compare')}
              theme="dark"
            >
              <Button
                color="secondary"
                className="!border border-solid coz-stroke-primary !font-medium"
                size="mini"
                icon={<IconCozEdit />}
                disabled={disabled || isFirstVersion}
                onClick={() => {
                  lastConfigAreaVisible.current = configAreaVisible;
                  setInDiffEditor?.(true);
                  setConfigAreaVisible?.(false);
                }}
                data-btm="d37080"
                data-btm-title={I18n.t('prompt_enter_diff')}
              >
                Diff
              </Button>
            </TooltipWhenDisabled>
          )}
          {renderTemplateType?.({
            prompt: promptInfo,
            streaming: disabled,
          }) ?? <TemplateSelect streaming={disabled} />}
        </>
      )}
      {configAreaVisible ? null : (
        <>
          <PopoverModelConfigEditor
            value={modelConfig as ModelConfigWithName}
            onChange={config => {
              setModelConfig({ ...config });
            }}
            disabled={disabled}
            renderDisplayContent={model => {
              console.info(999, model);
              return (
                <Button
                  size="mini"
                  color="secondary"
                  className="!border border-solid coz-stroke-primary !w-[160px]"
                  icon={
                    <Image
                      src={(model as { icon?: string })?.icon || ''}
                      width={16}
                      height={16}
                    />
                  }
                >
                  <Typography.Text
                    className="!font-medium w-[115px] !max-w-[115px] text-center"
                    size="small"
                    ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
                  >
                    {model?.name}
                  </Typography.Text>
                  <IconCozArrowDown className="ml-1" />
                </Button>
              );
            }}
            models={modelInfo?.list || []}
            onModelChange={setCurrentModel}
            modelSelectProps={{
              className: 'w-full',
              loading: modelInfo?.loading,
            }}
          />

          <Tooltip theme="dark" content={I18n.t('expand_model_and_var_area')}>
            <IconButton
              size="mini"
              color="secondary"
              onClick={() => {
                sendEvent?.(EVENT_NAMES.cozeloop_pe_column_collapse, {
                  prompt_id: `${promptInfo?.id || 'playground'}`,
                  type: configAreaVisible ? 1 : 0,
                });
                lastConfigAreaVisible.current = true;
                setConfigAreaVisible?.(true);
              }}
              icon={<IconCozColumnExpand fontSize={14} />}
            />
          </Tooltip>
        </>
      )}
      {configAreaVisible || configExecuteVisible ? null : (
        <Tooltip theme="dark" content={I18n.t('expand_preview_and_debug')}>
          <IconButton
            size="mini"
            color="secondary"
            onClick={() => {
              sendEvent?.(EVENT_NAMES.cozeloop_pe_column_collapse, {
                prompt_id: `${promptInfo?.id || 'playground'}`,
                type: 4,
              });
              setConfigExecuteVisible?.(true);
            }}
            icon={<IconCozSideExpand fontSize={14} />}
          />
        </Tooltip>
      )}
    </div>
  );
}
