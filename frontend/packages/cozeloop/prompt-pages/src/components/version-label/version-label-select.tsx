// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useState } from 'react';

import { debounce } from 'lodash-es';
import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { BaseSearchSelect } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { StonePromptApi } from '@cozeloop/api-schema';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Input,
  withField,
  Form,
  type CommonFieldProps,
  Checkbox,
  Highlight,
  Tag,
  Tooltip,
  Modal,
  Space,
  Toast,
} from '@coze-arch/coze-design';

import { useVersionLabelList } from '@/hooks/use-version-label-list';

export interface LabelWithPromptVersion {
  key: string;
  promptVersion?: string;
}

interface Props {
  promptID: string;
  value?: LabelWithPromptVersion[];
  onChange?: (value: LabelWithPromptVersion[]) => void;
}

const MAX_SELECT_COUNT = 20;

export function VersionLabelSelect(props: Props) {
  const [showCreate, setShowCreate] = useState(false);
  const { spaceID } = useSpace();
  const [createInputVal, setCreateInputVal] = useState('');

  const [createErrorInfo, setCreateErrorInfo] = useState('');

  const [filterKey, setFilterKey] = useState('');

  const service = useVersionLabelList({
    promptID: props.promptID,
    filterKey,
  });

  const createLabel = useRequest(
    async (name: string) => {
      await StonePromptApi.CreateLabel({
        workspace_id: spaceID,
        label: { key: name },
      });
      return { key: name };
    },
    {
      manual: true,
      onSuccess: () => {
        setCreateInputVal('');
        if (filterKey) {
          setFilterKey('');
        } else {
          // filterKey 清空时会触发reload
          service.reload();
        }

        Toast.success('创建版本标识成功');
      },
    },
  );

  const validateCreateInput = (v: string) => {
    let err = '';
    if (!v) {
      err = '请输入版本标识';
    } else if (!/^[a-z0-9_]+$/.test(v)) {
      err = '标识只允许小写字母、数字和下划线';
    } else if (service.data?.list?.find(item => item.value === v)) {
      err = '标识已存在';
    } else if (v.length > 50) {
      err = '标识长度不能超过50个字符';
    }
    setCreateErrorInfo(err);
  };

  const validateCreateInputAsync = async (v: string) => {
    const res = await StonePromptApi.BatchGetLabel({
      workspace_id: spaceID,
      label_keys: [v],
    });

    const hasError = !!res.labels?.length;
    if (hasError) {
      setCreateErrorInfo('标识已存在');
    }
    return hasError;
  };

  const resetCreateState = () => {
    setShowCreate(false);
    setCreateInputVal('');
    setCreateErrorInfo('');
  };

  const handleSearch = (inputValue: string) => {
    setFilterKey(inputValue);
  };

  const renderOptionItem = renderProps => {
    const {
      disabled,
      selected,
      label,
      focused,
      className,
      style,
      onMouseEnter,
      onClick,
      promptVersion,
    } = renderProps;
    const optionCls = classNames({
      ['semi-select-option']: true,
      ['semi-select-option-focused']: focused,
      ['semi-select-option-disabled']: disabled,
      ['semi-select-option-selected']: selected,
      className,
    });
    const searchWords = [filterKey];
    return (
      <div
        style={style}
        className={optionCls}
        onClick={() => onClick()}
        onMouseEnter={e => onMouseEnter()}
      >
        <Checkbox checked={selected} className="mr-2" />
        <div className="flex-1 pr-2">
          <Highlight
            sourceString={label}
            searchWords={searchWords}
            highlightStyle={{
              color: '#5147FF',
              backgroundColor: 'inherit',
              fontWeight: 400,
            }}
          />
        </div>
        {promptVersion ? (
          <Tooltip
            theme="dark"
            content={`当前标识已存在于${promptVersion}版本`}
          >
            <Tag color="grey" size="mini">
              {promptVersion}
            </Tag>
          </Tooltip>
        ) : null}
      </div>
    );
  };

  const handleCreateLabel = async () => {
    const hasError = await validateCreateInputAsync(createInputVal);
    if (!hasError) {
      createLabel.run(createInputVal);
    }
  };
  return (
    <BaseSearchSelect
      filter
      remote
      multiple
      className="w-full"
      showRefreshBtn
      showClear
      maxTagCount={100}
      showRestTagsPopover
      ellipsisTrigger
      onClickRefresh={() => service.reload()}
      loading={service.loading}
      loadOptionByIds={ids =>
        Promise.resolve(
          ((ids as string[]) || []).map(id => ({
            label: id,
            value: id,
          })),
        )
      }
      optionList={service.data?.list}
      renderOptionItem={renderOptionItem}
      onSearch={debounce(handleSearch, 500)}
      onListScroll={e => {
        const { currentTarget: target } = e;
        // 距离底部 20px 时加载更多
        if (
          target.scrollTop + target.clientHeight + 20 >=
          target.scrollHeight
        ) {
          service.loadMore();
        }
      }}
      onDropdownVisibleChange={v => {
        if (!v) {
          resetCreateState();
        }
      }}
      outerBottomSlot={
        <div>
          <Divider margin={4} />
          {showCreate ? (
            <div className="p-1">
              <div
                className="flex flex-col h-8"
                onBlur={() => resetCreateState()}
              >
                <Input
                  autoFocus
                  suffix={
                    <Button
                      size="small"
                      icon={<IconCozPlus />}
                      disabled={!!createErrorInfo || !createInputVal}
                      className="!w-6 !h-6 relative left-[4px]"
                      loading={createLabel.loading}
                      onClick={handleCreateLabel}
                    />
                  }
                  validateStatus={createErrorInfo ? 'error' : 'default'}
                  value={createInputVal}
                  onChange={v => {
                    setCreateInputVal(v);
                    validateCreateInput(v);
                  }}
                />
              </div>
              {createErrorInfo ? (
                <Form.ErrorMessage
                  className="text-[12px]"
                  error={createErrorInfo}
                />
              ) : null}
            </div>
          ) : (
            <div
              onClick={() => setShowCreate(true)}
              className="flex items-center h-8 coz-fg-hglt cursor-pointer px-3"
            >
              <IconCozPlus className="mr-2" />
              创建自定义标识
            </div>
          )}
        </div>
      }
      max={MAX_SELECT_COUNT}
      placeholder="请输入版本标识"
      value={props.value?.map(item => item.key)}
      onExceed={() => {
        Toast.info(`最多选择${MAX_SELECT_COUNT}个标识`);
      }}
      onChange={v => {
        props.onChange?.(
          (v as string[]).map(item => ({
            key: item,
            promptVersion: service.data?.versionMap?.[item] || '',
          })),
        );
      }}
    />
  );
}

export const FormVersionLabelSelect: React.FC<CommonFieldProps & Props> =
  withField(VersionLabelSelect);

/**
 * 检测标签是否被其他版本使用
 * @param labels
 */
export const checkLabelDuplicate = async (
  labels: LabelWithPromptVersion[] = [],
  currentVersion?: string,
) => {
  const labelsWithPrompt = (labels || []).filter(item =>
    currentVersion
      ? item.promptVersion && item.promptVersion !== currentVersion
      : item.promptVersion,
  );
  if (labelsWithPrompt.length) {
    await new Promise((resolve, reject) => {
      Modal.info({
        title: '版本标识存在重复',
        content: (
          <div className="coz-fg-secondary">
            所选标识
            <Space spacing={4} className="mx-1">
              {labelsWithPrompt.map(item => (
                <Tag key={item.key} size="mini" color="grey">
                  {item.key}
                </Tag>
              ))}
            </Space>
            已在当前 Prompt 其他版本生效。提交成功后，标识将仅在当前版本生效。
          </div>
        ),
        okText: '确认',
        cancelText: '取消',
        onOk: resolve,
        onCancel: reject,
      });
    });
  }
};
