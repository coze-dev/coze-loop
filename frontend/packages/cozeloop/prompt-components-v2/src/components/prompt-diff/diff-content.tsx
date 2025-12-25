// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-object-injection */
/* eslint-disable max-lines */
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { type ReactNode, useEffect, useMemo, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { isEqual } from 'lodash-es';
import classNames from 'classnames';
import {
  objSortedKeys,
  safeJsonParse,
  stringifyWithSortedKeys,
} from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { PageLoading } from '@cozeloop/components';
import {
  type PromptDetail,
  type Prompt,
  type ParamConfigValue,
} from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import {
  IconCozIllusEmpty,
  IconCozIllusEmptyDark,
} from '@coze-arch/coze-design/illustrations';
import {
  IconCozExpand,
  IconCozLongArrowUp,
  IconCozMinimize,
} from '@coze-arch/coze-design/icons';
import {
  Checkbox,
  Descriptions,
  Empty,
  Radio,
  Tag,
  Typography,
  Tooltip,
  Button,
  Modal,
} from '@coze-arch/coze-design';

import { convertSnippetsToMap, diffKeys } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { modelConfigLabelMap } from '@/consts';

import { PromptDiffEditor } from '../prompt-editor/diff';
import { usePromptDevProviderContext } from '../prompt-develop/components/prompt-provider';
import { DiffEditorLayout } from './diff-editor-layout';
import { DiffEditorContainer } from './diff-editor-container';

interface DiffContentProps {
  spaceID: string;
  preVersion?: string;
  currentInfo: Prompt;
  sameDesc?: string;
  onlineVersion?: string;
  currentVersionTitle?: string;
  diffWithEmptyPreInfo?: boolean;
  onlyOneTab?: boolean;
  showFullScreenBtn?: boolean;
}

const getDiffItem = (origin?: Int64, modified?: Int64) => {
  if (origin === modified) {
    return modified;
  }
  return (
    <div className="flex items-center gap-1">
      {origin ?? '-'}
      <IconCozLongArrowUp fontSize={12} className="rotate-90" />
      {modified}
    </div>
  );
};

const desc = I18n.t('prompt_no_change_info');

export function DiffContent({
  preVersion,
  currentInfo: oldCurrentInfo,
  sameDesc,
  currentVersionTitle,
  diffWithEmptyPreInfo,
  onlyOneTab,
  showFullScreenBtn,
}: DiffContentProps) {
  const { spaceID, modelInfo, hideSnippet } = usePromptDevProviderContext();
  const { setSnippetMap } = usePromptStore(
    useShallow(state => ({
      setSnippetMap: state.setSnippetMap,
    })),
  );

  const [diffType, setDiffType] = useState<string>('prompt');
  const [showFullScreen, setShowFullScreen] = useState(false);

  const [isOpenSub, setIsOpenSub] = useState(false);
  const [loadingSub, setLoadingSub] = useState(true);
  const [preInfo, setPreInfo] = useState<PromptDetail>();
  const [currentInfo, setCurrentInfo] = useState<PromptDetail | undefined>(
    oldCurrentInfo?.prompt_draft?.detail ||
      oldCurrentInfo?.prompt_commit?.detail,
  );

  const currentVersion = oldCurrentInfo?.prompt_draft?.draft_info
    ? ''
    : oldCurrentInfo?.prompt_commit?.commit_info?.version;
  const currentBaseVersion = (
    oldCurrentInfo?.prompt_draft?.draft_info ||
    oldCurrentInfo?.prompt_commit?.commit_info
  )?.base_version;

  const preInfoStr = useMemo(() => JSON.stringify(preInfo), [preInfo]);
  const currentInfoStr = useMemo(
    () => JSON.stringify(currentInfo),
    [currentInfo],
  );

  const preLibrarys =
    safeJsonParse<unknown[]>(preInfo?.ext_infos?.workflow ?? '[]') ?? [];
  const currentLibrarys =
    safeJsonParse<unknown[]>(currentInfo?.ext_infos?.workflow ?? '[]') ?? [];
  const allCozLibrarys = [...preLibrarys, ...currentLibrarys];

  const diffModelConfig = useMemo(() => {
    const array: { key: ReactNode; value: ReactNode }[] = [];
    const preModel = modelInfo?.list?.find(
      item => item.model_id === preInfo?.model_config?.model_id,
    );
    const currentModel = modelInfo?.list?.find(
      item => item.model_id === currentInfo?.model_config?.model_id,
    );
    if (preModel?.model_id !== currentModel?.model_id) {
      array.push({
        key: <Tag color="grey">{I18n.t('prompt_model_id')}</Tag>,
        value: getDiffItem(currentModel?.model_id, currentModel?.model_id),
      });
    }

    if (preModel?.name !== currentModel?.name) {
      array.push({
        key: <Tag color="grey">{I18n.t('model_name')}</Tag>,
        value: getDiffItem(preModel?.name, currentModel?.name),
      });
    }
    const keys = diffKeys(
      preInfo?.model_config || {},
      currentInfo?.model_config || {},
    ).filter(key => key !== 'model_id' && key !== 'param_config_values');

    keys.forEach(key => {
      array.push({
        key: <Tag color="grey">{modelConfigLabelMap[key]}</Tag>,
        value: getDiffItem(
          preInfo?.model_config?.[key],
          currentInfo?.model_config?.[key],
        ),
      });
    });

    const baseParamConfigValues = (preInfo?.model_config?.param_config_values ||
      []) as ParamConfigValue[];
    const currentParamConfigValues = (currentInfo?.model_config
      ?.param_config_values || []) as ParamConfigValue[];
    const baseKeys = baseParamConfigValues
      .map(item => item.name)
      .filter(key => key);
    const currentKeys = currentParamConfigValues
      .map(item => item.name)
      .filter(key => key);
    const keyArray = Array.from(
      new Set([...baseKeys, ...currentKeys]),
    ) as string[];

    keyArray.forEach(key => {
      const baseValue = baseParamConfigValues.find(item => item.name === key);
      const currentValue = currentParamConfigValues.find(
        item => item.name === key,
      );
      if (baseValue?.value?.value !== currentValue?.value?.value) {
        array.push({
          key: <Tag color="grey">{modelConfigLabelMap[key]}</Tag>,
          value: getDiffItem(
            baseValue?.value?.label || baseValue?.value?.value,
            currentValue?.value?.label || currentValue?.value?.value,
          ),
        });
      }
    });

    return array;
  }, [preInfoStr, currentInfoStr]);

  const extraDiffData = useMemo(() => {
    const array: { key: ReactNode; value: ReactNode }[] = [];

    if (!isEqual(preInfo?.tool_call_config, currentInfo?.tool_call_config)) {
      const preOldName = preInfo?.tool_call_config?.tool_choice;
      const currentOldName = currentInfo?.tool_call_config?.tool_choice;
      // TODO：指定方法运行对比
      array.push({
        key: <Tag color="grey">{I18n.t('function')}</Tag>,
        value: getDiffItem(preOldName, currentOldName),
      });
    }

    if (
      preInfo?.prompt_template?.template_type !==
      currentInfo?.prompt_template?.template_type
    ) {
      array.push({
        key: <Tag color="grey">{I18n.t('prompt_template_type')}</Tag>,
        value: getDiffItem(
          preInfo?.prompt_template?.template_type,
          currentInfo?.prompt_template?.template_type,
        ),
      });
    }
    return array;
  }, [preInfoStr, currentInfoStr]);

  const oldVariables = preInfo?.prompt_template?.variable_defs || [];
  const currentVariables = currentInfo?.prompt_template?.variable_defs || [];

  const diffVariables = useMemo(() => {
    const array: { key: ReactNode; value: ReactNode }[] = [];

    const addArray = currentVariables
      .filter(variable => !oldVariables.some(it => it.key === variable.key))
      .map(it => it.key);
    const deleteArray = oldVariables
      .filter(variable => !currentVariables.some(it => it.key === variable.key))
      .map(it => it.key);

    if (addArray.length) {
      const addText = addArray.join(',');
      array.push({
        key: (
          <Tag className="flex-shrink-0" color="grey">
            {I18n.t('add')}
          </Tag>
        ),

        value: (
          <Typography.Text
            ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
          >
            {addText}
          </Typography.Text>
        ),
      });
    }
    if (deleteArray.length) {
      const deleteText = deleteArray.join(',');
      array.push({
        key: (
          <Tag className="flex-shrink-0" color="grey">
            {I18n.t('delete')}
          </Tag>
        ),

        value: (
          <Typography.Text
            ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
          >
            {deleteText}
          </Typography.Text>
        ),
      });
    }

    return array;
  }, [oldVariables, currentVariables]);

  const isMetaDiff = !isEqual(
    preInfo?.prompt_template?.metadata || {},
    currentInfo?.prompt_template?.metadata || {},
  );

  const currentMessage =
    currentInfo?.prompt_template?.messages?.map(it => ({
      role: it.role,
      content: it.content,
      metadata: it.metadata,
      parts: it.parts,
    })) || [];
  const preMessage =
    preInfo?.prompt_template?.messages?.map(it => ({
      role: it.role,
      content: it.content,
      metadata: it.metadata,
      parts: it.parts,
    })) || [];

  const isMessageDiff = !isEqual(currentMessage, preMessage);

  const foreachMessageList = useMemo(
    () =>
      preMessage.length > currentMessage.length
        ? preInfo?.prompt_template?.messages
        : currentInfo?.prompt_template?.messages,
    [
      preInfo?.prompt_template?.messages,
      currentInfo?.prompt_template?.messages,
    ],
  );

  const isFcDiff = !isEqual(preInfo?.tools || [], currentInfo?.tools || []);

  const preVersionTitleDom = (
    <div className="flex items-center gap-2">
      {preInfo && preVersion === currentBaseVersion ? (
        <Tag className="mr-1">{I18n.t('prompt_source_version')}</Tag>
      ) : null}
      {preVersion || '-'}
    </div>
  );

  const currentVersionTitleDom = (
    <div className="flex items-center gap-2">
      {currentVersionTitle ? (
        <Tag color="brand">{currentVersionTitle}</Tag>
      ) : null}
      {currentVersion ?? I18n.t('draft_version')}
    </div>
  );

  const emptyDom = (
    <div className="w-full h-full flex justify-center items-center flex-1">
      <Empty
        image={<IconCozIllusEmpty className="w-40 h-40" />}
        darkModeImage={<IconCozIllusEmptyDark className="w-40 h-40" />}
        title={sameDesc || desc}
      />
    </div>
  );

  const promptTypeDiffCount = useMemo(() => {
    let count = 0;
    if (isMetaDiff) {
      count++;
    }
    if (isMessageDiff) {
      count++;
    }
    if (diffVariables.length) {
      count++;
    }
    return count;
  }, [isMetaDiff, isMessageDiff, diffVariables.length]);
  const promptDiffDom = (
    <>
      {isMetaDiff ? (
        <div className="w-full">
          <div className="text-sm mb-2 !font-semibold">Metadata</div>
          <DiffEditorContainer
            key={preVersion}
            preValue={stringifyWithSortedKeys(
              preInfo?.prompt_template?.metadata || {},
              null,
              2,
            )}
            currentValue={stringifyWithSortedKeys(
              currentInfo?.prompt_template?.metadata || {},
              null,
              2,
            )}
            preVersion={preVersionTitleDom}
            currentVersion={currentVersionTitleDom}
            diffEditorHeight={showFullScreen ? 400 : 300}
          />
        </div>
      ) : null}
      {isMessageDiff ? (
        <div className="w-full">
          <div className="text-sm mb-2 !font-semibold">Prompt Template</div>
          <DiffEditorLayout
            key={`${preVersion}-${currentVersion}-${isOpenSub}-messages`}
            preVersion={preVersionTitleDom}
            currentVersion={currentVersionTitleDom}
            diffEditorHeight="unset"
          >
            <div className="flex flex-col w-full">
              {foreachMessageList?.map((_, index) => {
                const preM = preInfo?.prompt_template?.messages?.[index] || {};
                const curM =
                  currentInfo?.prompt_template?.messages?.[index] || {};
                return (
                  <PromptDiffEditor
                    className={classNames(
                      'border !border-t-[var(--coz-stroke-primary)] !border-l-transparent !border-r-transparent !border-b-transparent !rounded-none',
                      {
                        '!border-t-0': index === 0,
                      },
                    )}
                    preMessage={preM}
                    message={curM}
                    disabled
                    snippetBtnHidden
                    cozeLibrarys={allCozLibrarys}
                    modalVariableEnable
                  />
                );
              })}
            </div>
          </DiffEditorLayout>
        </div>
      ) : null}
      {diffVariables.length ? (
        <div className="w-full">
          <div className="text-sm mb-2 !font-semibold">Input Variable</div>
          <div className="border border-solid coz-stroke-primary py-2.5 px-2 rounded-small bg-white overflow-hidden flex flex-col gap-3">
            {diffVariables.map(({ key, value }) => (
              <div className="flex items-center gap-4">
                {key}
                {value}
              </div>
            ))}
          </div>
        </div>
      ) : null}
      {!onlyOneTab && !promptTypeDiffCount ? emptyDom : null}
    </>
  );

  const configTypeDiffCount = useMemo(() => {
    let count = 0;
    if (isFcDiff) {
      count++;
    }
    if (diffModelConfig.length) {
      count++;
    }
    if (extraDiffData.length) {
      count++;
    }
    return count;
  }, [isFcDiff, diffModelConfig.length, extraDiffData.length]);

  const configDiffDom = (
    <>
      {diffModelConfig.length ? (
        <div className="w-full">
          <div className="text-sm mb-2 !font-semibold">
            {I18n.t('prompt_model_settings')}
          </div>
          <Descriptions
            className="border border-solid coz-stroke-primary pt-2.5 px-2 bg-white rounded-small"
            data={diffModelConfig}
          />
        </div>
      ) : null}
      {extraDiffData.length ? (
        <div className="w-full">
          <div className="text-sm mb-2 !font-semibold">
            {I18n.t('prompt_prompt_configuration')}
          </div>
          <Descriptions
            className="border border-solid coz-stroke-primary pt-2.5 px-2 bg-white rounded-small"
            data={extraDiffData}
          />
        </div>
      ) : null}
      {isFcDiff ? (
        <div className="w-full">
          <div className="text-sm mb-2 !font-semibold">
            {I18n.t('function')}
          </div>
          <DiffEditorContainer
            className="bg-white"
            key={preVersion}
            preValue={JSON.stringify(
              preInfo?.tools?.map(it => objSortedKeys(it)) || [],
              null,
              2,
            )}
            currentValue={JSON.stringify(
              currentInfo?.tools?.map(it => objSortedKeys(it)) || [],
              null,
              2,
            )}
            preVersion={preVersionTitleDom}
            currentVersion={currentVersionTitleDom}
            diffEditorHeight={showFullScreen ? 600 : 400}
          />
        </div>
      ) : null}
      {!configTypeDiffCount && !onlyOneTab ? emptyDom : null}
    </>
  );

  const detailDom = (
    <>
      <div
        className="border-0 border-b border-solid coz-stroke-primary h-[47px] flex items-center justify-between sticky top-0 z-10 px-4"
        style={{
          background: showFullScreen
            ? 'rgba(var(--coze-bg-2), var(--coze-bg-2-alpha))'
            : 'transparent',
        }}
      >
        {onlyOneTab ? (
          <Typography.Text strong>Prompt Diff</Typography.Text>
        ) : (
          <Radio.Group
            type="button"
            value={diffType}
            onChange={e => setDiffType(e.target.value)}
          >
            <Radio value="prompt" disabled={!promptTypeDiffCount}>
              {I18n.t('prompt_prompt_change_count', { promptTypeDiffCount })}
            </Radio>
            <Radio value="config" disabled={!configTypeDiffCount}>
              {I18n.t('prompt_config_change_count', { configTypeDiffCount })}
            </Radio>
          </Radio.Group>
        )}
        {showFullScreenBtn ? (
          <Tooltip
            content={
              showFullScreen
                ? I18n.t('prompt_exit_fullscreen')
                : I18n.t('evaluate_full_screen')
            }
          >
            <Button
              icon={
                showFullScreen ? (
                  <IconCozMinimize fontSize={12} />
                ) : (
                  <IconCozExpand fontSize={12} />
                )
              }
              color="secondary"
              onClick={() => {
                setShowFullScreen(!showFullScreen);
              }}
            />
          </Tooltip>
        ) : null}
      </div>
      <div className="flex-1 w-full h-full overflow-y-auto styled-scrollbar p-4 flex flex-col gap-4 relative">
        {isMessageDiff && diffType === 'prompt' && !hideSnippet ? (
          <div className="absolute top-4 right-4 z-10">
            <Checkbox
              checked={isOpenSub}
              onChange={v => setIsOpenSub(v?.target?.checked || false)}
            >
              {I18n.t('prompt_expand_nested_content')}
            </Checkbox>
          </div>
        ) : null}
        {onlyOneTab ? (
          <>
            {promptDiffDom}
            {configDiffDom}
          </>
        ) : diffType === 'prompt' ? (
          promptDiffDom
        ) : (
          configDiffDom
        )}
      </div>
    </>
  );

  useEffect(() => {
    if (oldCurrentInfo?.id) {
      setLoadingSub(true);
      const isDraft = Boolean(oldCurrentInfo?.prompt_draft);
      const currentPromptVersion =
        oldCurrentInfo?.prompt_commit?.commit_info?.version;
      StonePromptApi.GetPrompt({
        prompt_id: oldCurrentInfo?.id,
        workspace_id: spaceID,
        with_draft: isDraft,
        with_commit: true,
        commit_version: currentPromptVersion,
        expand_snippet: isOpenSub,
      })
        .then(res => {
          const currentDetal =
            res.prompt?.prompt_draft?.detail ||
            res.prompt?.prompt_commit?.detail;
          setCurrentInfo(currentDetal);
          setSnippetMap(map => ({
            ...map,
            ...convertSnippetsToMap(
              currentDetal?.prompt_template?.snippets || [],
            ),
          }));
          if (preVersion) {
            StonePromptApi.GetPrompt({
              prompt_id: oldCurrentInfo?.id,
              workspace_id: spaceID,
              with_commit: true,
              commit_version: preVersion,
              expand_snippet: isOpenSub,
            })
              .then(preRes => {
                setPreInfo(preRes.prompt?.prompt_commit?.detail);
                setSnippetMap(map => ({
                  ...map,
                  ...convertSnippetsToMap(
                    preRes.prompt?.prompt_commit?.detail?.prompt_template
                      ?.snippets || [],
                  ),
                }));
              })
              .catch(() => {
                setPreInfo(undefined);
              })
              .finally(() => {
                setLoadingSub(false);
              });
          } else {
            setLoadingSub(false);
          }
        })
        .catch(() => {
          setLoadingSub(false);
        });
    }
  }, [
    preVersion,
    oldCurrentInfo?.prompt_commit?.commit_info?.version,
    isOpenSub,
  ]);

  useEffect(() => {
    if (!onlyOneTab) {
      setDiffType('prompt');
    } else if (configTypeDiffCount) {
      setDiffType('config');
    } else {
      setDiffType('prompt');
    }
  }, [configTypeDiffCount, onlyOneTab]);

  if (loadingSub) {
    return (
      <div className="w-full h-full flex items-center justify-center">
        <PageLoading />
      </div>
    );
  }

  // Check if there are no differences to show
  if (
    (!diffModelConfig.length &&
      !diffVariables.length &&
      !isMessageDiff &&
      !isMetaDiff &&
      !extraDiffData.length &&
      !isFcDiff) ||
    (!preInfo && !diffWithEmptyPreInfo)
  ) {
    return emptyDom;
  }

  return (
    <>
      {detailDom}
      <Modal
        visible={showFullScreen}
        onCancel={() => setShowFullScreen(false)}
        fullScreen
        title={I18n.t('prompt_prompt_diff_change_info')}
        bodyStyle={{ paddingBottom: '32px' }}
        hasScroll={false}
      >
        <div className="px-2 overflow-y-auto" style={{ maxHeight: '100%' }}>
          {detailDom}
        </div>
      </Modal>
    </>
  );
}
