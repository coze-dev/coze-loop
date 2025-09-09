// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useMemo, useRef, useState } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { useResourcePageJump } from '@cozeloop/biz-hooks-adapter';
import {
  type AnnotateRecord,
  type ColumnAnnotation,
} from '@cozeloop/api-schema/evaluation';
import { tag } from '@cozeloop/api-schema/data';
import { StoneEvaluationApi, DataApi } from '@cozeloop/api-schema';
import { IconCozLongArrowTopRight } from '@coze-arch/coze-design/icons';
import {
  CozInputNumber,
  type InputNumberProps,
  Select,
  type SelectProps,
  TextArea,
  type TextAreaProps,
} from '@coze-arch/coze-design';

import { TypographyText } from '@/components/text-ellipsis';

import { TagDetailLink } from './tag-detail-link';

interface Props {
  spaceID: string;
  experimentID: string;
  groupID: string;
  turnID: string;
  annotation: ColumnAnnotation;
  /** 标注结果 */
  annotateRecord?: AnnotateRecord;
  /** 禁用标注 */
  disabled?: boolean;
  /**
   * 启用选择的方式，标注 boolean 类型
   */
  useSelectBoolean?: boolean;
  /**
   * 标注值改变时回调
   */
  onChange?: () => void;
  /**
   * 新增分类标注选项时回调
   */
  onCreateOption?: () => void;
}

const getValue = (
  type?: tag.TagContentType,
  annotateRecord?: AnnotateRecord,
) => {
  switch (type) {
    case tag.TagContentType.FreeText:
      return annotateRecord?.plain_text;
    case tag.TagContentType.Categorical:
      return annotateRecord?.tag_value_id;
    case tag.TagContentType.Boolean:
      return annotateRecord?.tag_value_id;
    case tag.TagContentType.ContinuousNumber:
      return annotateRecord?.score;
    default:
      return;
  }
};
export function TagInput({
  spaceID,
  experimentID,
  groupID,
  turnID,
  annotation,
  annotateRecord,
  disabled,
  useSelectBoolean,
  onChange,
  onCreateOption,
}: Props) {
  const annotateRecordID = useRef(annotateRecord?.annotate_record_id);
  annotateRecordID.current = annotateRecord?.annotate_record_id;

  const [innerValue, setInnerValue] = useState<string>();

  useEffect(() => {
    const res = getValue(annotation.content_type, annotateRecord);
    setInnerValue(res);
  }, [annotation.content_type, annotateRecord]);

  const save = useRequest(
    async (
      value: Pick<AnnotateRecord, 'tag_value_id' | 'plain_text' | 'score'>,
    ) => {
      if (annotateRecordID.current) {
        await StoneEvaluationApi.UpdateAnnotateRecord({
          workspace_id: spaceID,
          expt_id: experimentID,
          item_id: groupID,
          turn_id: turnID,
          annotate_record_id: annotateRecordID.current || '',
          annotate_records: {
            annotate_record_id: annotateRecordID.current || '',
            tag_key_id: annotation.tag_key_id,
            tag_content_type: annotation.content_type,
            ...value,
          },
        });
      } else {
        const { annotate_record_id } =
          await StoneEvaluationApi.CreateAnnotateRecord({
            workspace_id: spaceID,
            expt_id: experimentID,
            item_id: groupID,
            turn_id: turnID,
            annotate_record: {
              tag_key_id: annotation.tag_key_id,
              tag_content_type: annotation.content_type,
              ...value,
            },
          });
        annotateRecordID.current = annotate_record_id;
      }
      onChange?.();
    },
    {
      manual: true,
    },
  );

  const allDisabled =
    save.loading || annotation.status === tag.TagStatus.Inactive || disabled;
  switch (annotation.content_type) {
    case tag.TagContentType.FreeText:
      return (
        <FreeTextLabel
          annotation={annotation}
          disabled={allDisabled}
          value={innerValue}
          onChange={val => setInnerValue(val)}
          onBlur={e => save.run({ plain_text: e.target.value })}
        />
      );
    case tag.TagContentType.Categorical:
      return (
        <CategoryLabel
          spaceID={spaceID}
          annotation={annotation}
          disabled={allDisabled}
          value={innerValue}
          onChange={val => {
            save.run({ tag_value_id: val as string });
            setInnerValue(val as string);
          }}
          onCreateOption={onCreateOption}
        />
      );
    case tag.TagContentType.Boolean:
      return useSelectBoolean ? (
        <BooleanLabelSelect
          annotation={annotation}
          disabled={allDisabled}
          value={innerValue}
          onChange={val => {
            save.run({ tag_value_id: val as string });
            setInnerValue(val as string);
          }}
        />
      ) : (
        <BooleanLabel
          annotation={annotation}
          disabled={allDisabled}
          value={innerValue}
          onChange={val => {
            save.run({ tag_value_id: val as string });
            setInnerValue(val as string);
          }}
        />
      );
    case tag.TagContentType.ContinuousNumber:
      return (
        <NumberLabel
          annotation={annotation}
          disabled={allDisabled}
          value={innerValue}
          onBlur={e => {
            if (e.target.value) {
              save.run({ score: e.target.value });
            }
          }}
          onChange={val => {
            if (val) {
              setInnerValue(val as string);
            }
          }}
        />
      );
    default:
      return null;
  }
}

interface FreeTextLabelProps extends TextAreaProps {
  annotation: ColumnAnnotation;
}
export function FreeTextLabel(props: FreeTextLabelProps) {
  const [rows, setRows] = useState(1);
  return (
    <TextArea
      maxCount={rows === 1 ? undefined : 200}
      maxLength={rows === 1 ? undefined : 200}
      {...props}
      rows={rows}
      onBlur={event => {
        setRows(1);
        props.onBlur?.(event);
      }}
      onFocus={event => {
        setRows(3);
        props.onFocus?.(event);
      }}
    />
  );
}

interface CategoryLabelProps extends SelectProps {
  annotation: ColumnAnnotation;
  spaceID: string;
  onCreateOption?: () => void;
}
export const CategoryLabel = ({
  annotation,
  spaceID,
  onCreateOption,
  ...selectProps
}: CategoryLabelProps) => {
  const { getTagDetailURL } = useResourcePageJump();

  const optionList = useMemo(
    () =>
      (annotation.tag_values || [])
        .sort(a => (a.status === tag.TagStatus.Inactive ? 1 : -1))
        .map(item => ({
          value: item.tag_value_id,
          disabled: item.status === tag.TagStatus.Inactive,
          tagName: item.tag_value_name,
          label: (
            <TooltipWhenDisabled
              content={
                <div>
                  <span className="mr-1">
                    {I18n.t(
                      'cozeloop_open_evaluate_tag_option_disabled_no_longer_selectable',
                    )}
                  </span>
                  <TagDetailLink tagKey={annotation.tag_key_id} />
                </div>
              }
              spacing={40}
              position="left"
              theme="dark"
              disabled={
                item.status !== tag.TagStatus.Active &&
                selectProps.value === item.tag_value_id
              }
            >
              <div className="group flex items-center w-full h-8 overflow-hidden px-2">
                <div className="flex-1 min-w-0">
                  <TypographyText
                    className={classNames('max-w-full overflow-hidden', {
                      '!coz-fg-dim': item.status !== tag.TagStatus.Active,
                    })}
                  >
                    {item.tag_value_name}
                  </TypographyText>
                </div>

                <IconCozLongArrowTopRight
                  className="ml-1 text-brand-9 shrink-0 cursor-pointer invisible group-hover:visible"
                  onClick={e => {
                    e.stopPropagation();
                    window.open(getTagDetailURL(annotation.tag_key_id || ''));
                  }}
                />
              </div>
            </TooltipWhenDisabled>
          ),
        })),
    [annotation, selectProps.value],
  );

  const [isCreate, setIsCreate] = useState(false);
  const [inputValue, setInputValue] = useState('');

  const handleSearch = (val: string) => {
    setInputValue(val);
    setIsCreate(!!val && optionList.every(item => item.tagName !== val));
  };

  const addTagOption = useRequest(
    async (val: string) => {
      await DataApi.UpdateTag({
        workspace_id: spaceID,
        tag_key_id: annotation.tag_key_id || '',
        tag_key_name: annotation.tag_key_name || '',
        description: annotation.description,
        tag_content_type: annotation.content_type,
        tag_content_spec: annotation.content_spec,
        tag_values: [
          ...(annotation.tag_values || []),
          {
            tag_value_name: val,
            status: tag.TagStatus.Active,
          },
        ],
      });
    },
    {
      manual: true,
      onSuccess: () => {
        onCreateOption?.();
      },
    },
  );

  const handleCreate = async () => {
    await addTagOption.runAsync(inputValue);
    setIsCreate(false);
    setInputValue('');
  };

  return (
    <Select
      className="w-full"
      placeholder={I18n.t('please_select_a_category')}
      optionList={optionList}
      filter={(val, option) => option.tagName?.includes(val)}
      onSearch={handleSearch}
      inputProps={{ maxLength: 50 }}
      emptyContent={null}
      outerBottomSlot={
        isCreate ? (
          <div
            className={classNames('coz-select-option-item p-2', {
              disabled: addTagOption.loading,
            })}
            onClick={handleCreate}
          >
            <span className="coz-fg-dim mr-1">{I18n.t('add')}</span>
            <span className="coz-fg-plus">{inputValue}</span>
          </div>
        ) : null
      }
      renderSelectedItem={item => item.tagName}
      {...selectProps}
    />
  );
};

interface BooleanLabelProps {
  annotation: ColumnAnnotation;
  value?: string;
  onChange?: (val?: string) => void;
  disabled?: boolean;
}
export const BooleanLabel = ({
  annotation,
  disabled,
  value,
  onChange,
}: BooleanLabelProps) => (
  <div
    className={classNames(
      'flex items-center gap-x-2 w-full max-w-full overflow-hidden',
      {
        'bg-gray-50 pointer-events-none': disabled,
      },
    )}
  >
    {annotation.tag_values?.map(item => (
      <div
        className={classNames('flex-1 overflow-hidden', {})}
        key={item.tag_value_id}
        onClick={() => {
          if (disabled) {
            return;
          }
          onChange?.(item.tag_value_id || '');
        }}
      >
        <Select
          className={classNames('max-w-full overflow-hidden w-full', {
            '!bg-brand-3 !border-brand-9': value === item.tag_value_id,
            'pointer-events-none': disabled,
          })}
          disabled={disabled}
          emptyContent={null}
          showArrow={false}
          value={item.tag_value_name}
          optionList={undefined}
        />
      </div>
    ))}
  </div>
);
interface BooleanLabelSelectProps extends SelectProps {
  annotation: ColumnAnnotation;
}

export function BooleanLabelSelect({
  annotation,
  ...selectProps
}: BooleanLabelSelectProps) {
  return (
    <Select
      className="w-full"
      optionList={annotation.tag_values?.map(item => ({
        label: item.tag_value_name,
        value: item.tag_value_id,
        disabled: item.status === tag.TagStatus.Inactive,
      }))}
      {...selectProps}
    />
  );
}

interface NumberLabelProps extends InputNumberProps {
  annotation: ColumnAnnotation;
}
export function NumberLabel({
  annotation,
  ...inputNumberProps
}: NumberLabelProps) {
  const numberInputFormatter = (v: string | number) => {
    if (typeof v === 'number') {
      v = String(v);
    }
    if (!v) {
      return '';
    }
    if (!/^[+-]?(\d+(\.\d*)?|\.\d+)?$/.test(v)) {
      return '';
    }

    if (v === '+' || v === '-') {
      return '';
    }

    const newValue = v.replace('+', '').replace('-', '');
    const parts = newValue.split('.');
    if (parts[0].length > 6) {
      parts[0] = parts[0].slice(0, 6);
    }
    if ((parts[1]?.length || 0) > 4) {
      parts[1] = parts[1].slice(0, 4);
    }
    if (v.startsWith('-')) {
      return `-${parts.join('.')}`;
    }
    return parts.join('.');
  };

  return (
    <CozInputNumber
      max={
        annotation.content_spec?.continuous_number_spec?.max_value ||
        Number.MAX_SAFE_INTEGER
      }
      min={
        annotation.content_spec?.continuous_number_spec?.min_value ||
        Number.MIN_SAFE_INTEGER
      }
      formatter={numberInputFormatter}
      placeholder={I18n.t('please_enter_a_value')}
      {...inputNumberProps}
    />
  );
}
