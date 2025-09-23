// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { useEffect, useMemo, useState } from 'react';

import { debounce } from 'lodash-es';
import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  annotation,
  type PlatformType,
} from '@cozeloop/api-schema/observation';
import { tag } from '@cozeloop/api-schema/data';
import { observabilityTrace } from '@cozeloop/api-schema';
import {
  FormInputNumber,
  withField,
  Typography,
  type SelectProps,
  Tooltip,
  Select,
  TextArea,
} from '@coze-arch/coze-design';

import {
  composeValidate,
  tagEmptyValueValidate,
  tagLengthMaxLengthValidate,
} from '@/utils/validate';

import { useAnnotationPanelContext } from './annotation-panel-context.ts';
import { type CreateAnnotationFormValues } from './annotation-content';

const { TagContentType } = tag;

const TAG_CONTENT_TYPE_TO_VALUE_TYPE = {
  [TagContentType.Categorical]: annotation.ValueType.Long,
  [TagContentType.ContinuousNumber]: annotation.ValueType.Double,
  [TagContentType.FreeText]: annotation.ValueType.String,
  [TagContentType.Boolean]: annotation.ValueType.Long,
};

interface DisableOptions {
  disableAllTags?: boolean;
  disableTagItem?: {
    id: string[];
    disable: boolean;
  };
}

interface TagItemProps {
  tagItem?: CreateAnnotationFormValues['tags'][number];
  field: string;
  disableOptions?: DisableOptions;
  onCreateAnnotationSuccess?: (value: string, annotationId?: string) => void;
  span_id?: string;
  trace_id?: string;
  onLoadingChange?: (loading: boolean) => void;
  isTagDisabled?: boolean;
  start_time?: string;
}

export const TagItem = (props: TagItemProps) => {
  const {
    tagItem,
    field,
    onCreateAnnotationSuccess,
    onLoadingChange,
    isTagDisabled,
  } = props;
  const { isRemoteValue = false } = tagItem || {};
  const { spaceID } = useSpace();
  const { platformType, setEditChanged } = useAnnotationPanelContext();

  const { runAsync: createManualAnnotation, loading: createLoading } =
    useRequest(
      async (value: string) => {
        const { annotation_id } =
          await observabilityTrace.CreateManualAnnotation({
            annotation: {
              span_id: props.span_id,
              trace_id: props.trace_id,
              workspace_id: spaceID,
              key: tagItem?.annotation?.manual_feedback?.tag_key_id ?? '',
              value_type:
                TAG_CONTENT_TYPE_TO_VALUE_TYPE[
                  tagItem?.tagInfo?.content_type ?? ''
                ] ?? annotation.ValueType.String,
              value,
              start_time: tagItem?.annotation?.start_time ?? props.start_time,
            },
            platform_type: platformType as PlatformType,
          });
        return annotation_id;
      },
      { manual: true },
    );

  const { runAsync: updateManualAnnotation, loading: updateLoading } =
    useRequest(
      async (params: {
        valueType: annotation.ValueType;
        value: string;
        annotation_id: string;
      }) => {
        await observabilityTrace.UpdateManualAnnotation({
          annotation_id: params.annotation_id,
          annotation: {
            span_id: props.span_id,
            trace_id: props.trace_id,
            workspace_id: spaceID,
            key: tagItem?.annotation?.manual_feedback?.tag_key_id ?? '',
            value: params.value,
            value_type: params.valueType,
            start_time: tagItem?.annotation?.start_time ?? props.start_time,
          },
          platform_type: platformType as PlatformType,
        });
      },
      { manual: true },
    );

  useEffect(() => {
    onLoadingChange?.(createLoading || updateLoading);
  }, [createLoading, updateLoading, onLoadingChange]);

  const handleChange = (v: string, valueType: annotation.ValueType) => {
    if (!v) {
      return;
    }
    if (isRemoteValue) {
      updateManualAnnotation({
        valueType,
        value: v,
        annotation_id: tagItem?.annotation?.id ?? '',
      });
      setEditChanged?.(true);
    } else {
      createManualAnnotation(v).then(annotationId => {
        onCreateAnnotationSuccess?.(v, annotationId);
      });
      setEditChanged?.(true);
    }
  };
  const disabled = createLoading || updateLoading || isTagDisabled;

  if (!tagItem) {
    return null;
  }
  if (tagItem.tagInfo?.content_type === TagContentType.Categorical) {
    const optionList = tagItem.tagInfo?.tag_values
      ?.filter(
        tagValue =>
          tagValue.status !== tag.TagStatus.Inactive ||
          tagValue.tag_value_id ===
            tagItem.annotation?.manual_feedback?.tag_value_id,
      )
      .map(tagValue => ({
        value: tagValue.tag_value_id,
        disabled: tagValue.status === tag.TagStatus.Inactive,
        label: (
          <>
            {tagValue?.status === tag.TagStatus.Active ? (
              <Typography.Text
                className="max-w-full overflow-hidden"
                ellipsis={{
                  showTooltip: {
                    opts: {
                      theme: 'dark',
                    },
                  },
                }}
              >
                {tagValue.tag_value_name}
              </Typography.Text>
            ) : (
              <Tooltip
                content={I18n.t('category_tag_option_disabled_warning')}
                position="left"
                theme="dark"
              >
                <div className="w-full max-w-full overflow-hidden">
                  <Typography.Text
                    className="max-w-full overflow-hidden !text-[var(--coz-fg-dim)]"
                    ellipsis={{
                      showTooltip: {
                        opts: {
                          theme: 'dark',
                        },
                      },
                    }}
                  >
                    {tagValue.tag_value_name}
                  </Typography.Text>
                </div>
              </Tooltip>
            )}
          </>
        ),
      }));
    return (
      <CategoryTag
        noLabel
        className="w-full max-w-full overflow-hidden"
        field={`${field}.tag_value_id`}
        placeholder={I18n.t('enter_subtag_name')}
        disabled={disabled}
        optionList={optionList}
        onChange={v => {
          handleChange(v as string, annotation.ValueType.Long);
        }}
      />
    );
  }

  if (tagItem.tagInfo?.content_type === TagContentType.ContinuousNumber) {
    const numberInputFormatter = (v: string | number) =>
      !Number.isNaN(parseFloat(`${v}`)) ? parseFloat(`${v}`).toString() : '';
    return (
      <FormInputNumber
        field={`${field}.tag_value`}
        noLabel
        max={Number.MAX_SAFE_INTEGER}
        formatter={numberInputFormatter}
        className="w-full max-w-full overflow-hidden"
        placeholder={I18n.t('enter_number')}
        onBlur={debounce(event => {
          handleChange(
            numberInputFormatter(event.target.value),
            annotation.ValueType.Double,
          );
        }, 500)}
        validate={composeValidate([tagEmptyValueValidate])}
        disabled={disabled}
      />
    );
  }

  if (tagItem.tagInfo?.content_type === TagContentType.FreeText) {
    return (
      <FreeTextTag
        field={`${field}.tag_value`}
        noLabel
        className="w-full max-w-full overflow-hidden"
        placeholder={I18n.t('enter_text')}
        onBlur={debounce(event => {
          if (event.target.value.length > 200) {
            return;
          }
          handleChange(event.target.value, annotation.ValueType.String);
        }, 500)}
        validate={composeValidate([
          tagEmptyValueValidate,
          tagLengthMaxLengthValidate,
        ])}
        disabled={disabled}
      />
    );
  }

  if (tagItem.tagInfo?.content_type === TagContentType.Boolean) {
    return (
      <FormBooleanTag
        field={`${field}.tag_value_id`}
        noLabel
        disabled={disabled}
        options={tagItem.tagInfo?.tag_values?.map(tagValue => ({
          value: tagValue.tag_value_id ?? '',
          label: tagValue.tag_value_name ?? '',
        }))}
        onChange={v => {
          handleChange(v as string, annotation.ValueType.Long);
        }}
      />
    );
  }
};

interface BooleanTagProps {
  value: string;
  onChange?: (value: string) => void;
  options?: Array<{ label: string; value: string }>;
  disableOptions?: DisableOptions;
  disabled?: boolean;
}

const BooleanTag: (props: BooleanTagProps) => JSX.Element = (
  props: BooleanTagProps,
) => {
  const { value, disabled, onChange, options, disableOptions = {} } = props;
  const [selectedValue, setSelectedValue] = useState(value);

  const { disableAllTags, disableTagItem } = disableOptions;

  const optionsList = useMemo(() => {
    if (!disableTagItem?.disable) {
      return options;
    }

    return options?.filter(option => !disableTagItem.id.includes(option.value));
  }, [disableTagItem, options]);

  const handleChange = (v: string) => {
    setSelectedValue(v);
    onChange?.(v);
  };
  return (
    <div
      className={classNames(
        'flex items-center gap-x-2 w-full max-w-full overflow-hidden',
        {
          'bg-gray-50 pointer-events-none': disableAllTags,
        },
      )}
    >
      {optionsList?.map(option => (
        <div
          className={classNames('flex-1 overflow-hidden', {})}
          key={option.value}
          onClick={() => {
            if (disabled) {
              return;
            }
            handleChange(option.value);
          }}
        >
          <Select
            className={classNames('max-w-full overflow-hidden w-full', {
              '!bg-brand-3 !border-brand-9': selectedValue === option.value,
              'pointer-events-none': disabled,
            })}
            disabled={disabled}
            emptyContent={null}
            showArrow={false}
            value={option.label}
            optionList={undefined}
          />
        </div>
      ))}
    </div>
  );
};
const FormBooleanTag = withField(BooleanTag);

interface CategoryTagProps extends SelectProps {
  disableOptions?: DisableOptions;
  field: string;
}

const CategoryTag = withField((props: CategoryTagProps) => {
  const { onChange, optionList = [], disableOptions, value, disabled } = props;

  const { disableTagItem } = disableOptions || {};
  const options = useMemo(() => {
    if (!disableTagItem?.disable) {
      return optionList;
    }

    return optionList.filter(
      option => !disableTagItem.id.includes(option.value as string),
    );
  }, [disableTagItem, optionList]);

  return (
    <Select
      className="w-full"
      placeholder={I18n.t('select_category')}
      optionList={options}
      disabled={disabled}
      onChange={onChange}
      value={value}
    />
  );
});

interface FreeTextTagProps {
  disabled?: boolean;
  field: string;
  noLabel: boolean;
  className: string;
  placeholder: string;
  onBlur?: (event: React.FocusEvent<HTMLTextAreaElement>) => void;
  onFocus?: (event: React.FocusEvent<HTMLTextAreaElement>) => void;
  onChange?: (value: string) => void;
}
const FreeTextTag = withField((props: FreeTextTagProps) => {
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
});
