// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import zhCH from '@cozeloop/observation-components/zh-CN';
import enUS from '@cozeloop/observation-components/en-US';
import {
  TraceSelector,
  type TraceSelectorProps,
  ConfigProvider,
  getDefaultBizConfig,
  fetchMetaInfo,
  useTraceTimeRangeOptions,
} from '@cozeloop/observation-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type FieldMeta,
  PlatformType,
  SpanListType,
} from '@cozeloop/api-schema/observation';
import { type OptionProps } from '@coze-arch/coze-design';

type CozeloopTraceSelectorProps = Pick<
  TraceSelectorProps,
  | 'items'
  | 'onChange'
  | 'spanListTypeOptionList'
  | 'platformTypeOptionList'
  | 'datePickerOptions'
  | 'fieldMetas'
  | 'customParams'
  | 'layoutMode'
  | 'initValues'
  | 'getFieldMetas'
  | 'disabled'
  | 'ignoreKeys'
  | 'disabledRowKeys'
  | 'disableFilterItemByFilterName'
>;

export const CozeloopTraceSelectorInitValues = {
  platformType: PlatformType.Cozeloop,
  spanListType: SpanListType.RootSpan,
};

export const CozeloopTraceSelector = (
  props: Partial<CozeloopTraceSelectorProps>,
) => {
  const { spaceID } = useSpace();
  const defaultBizConfig = getDefaultBizConfig();

  const platformTypeOptionList = props.platformTypeOptionList
    ? props.platformTypeOptionList
    : defaultBizConfig.platformTypeOptions;
  const spanListTypeOptionList = props.spanListTypeOptionList
    ? props.spanListTypeOptionList
    : defaultBizConfig.spanListTypeOptions;

  const datePickerOptions = useTraceTimeRangeOptions();
  const lang = I18n.language === 'zh-CN' ? zhCH : enUS;

  return (
    <ConfigProvider
      locale={{
        language: I18n.lang,
        locale: lang,
      }}
    >
      <TraceSelector
        {...props}
        items={props.items ?? ['platformType', 'spanListType', 'filterSelect']}
        initValues={props.initValues ?? CozeloopTraceSelectorInitValues}
        platformTypeOptionList={
          platformTypeOptionList as unknown as OptionProps[]
        }
        spanListTypeOptionList={
          spanListTypeOptionList as unknown as OptionProps[]
        }
        customLeftRenderMap={{}}
        customRightRenderMap={{}}
        customViewConfig={{
          visibility: true,
        }}
        datePickerOptions={datePickerOptions}
        getFieldMetas={({ platform_type, span_list_type }) =>
          fetchMetaInfo({
            selectedPlatform: platform_type,
            selectedSpanType: span_list_type,
            spaceID,
          }) as Promise<Record<string, FieldMeta>>
        }
        customParams={{
          spaceID,
        }}
      />
    </ConfigProvider>
  );
};
