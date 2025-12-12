/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */
import { PlatformType, SpanListType } from '@cozeloop/api-schema/observation';
import { tag } from '@cozeloop/api-schema/data';
import { type OptionProps } from '@coze-arch/coze-design';

import { BIZ } from '@/shared/constants';
import {
  CategoricalSelect,
  type CategoricalSelectProps,
} from '@/shared/components/filter-bar/categorical-select';
import {
  API_FEEDBACK,
  MANUAL_FEEDBACK,
} from '@/shared/components/analytics-logic-expr/const';
import { i18nService } from '@/i18n';

import {
  ApiFeedbackExpr as ApiFeedbackExprRight,
  type ApiFeedbackExprProps as ApiFeedbackExprRightProps,
} from './components/logic-right-expr/index';
import {
  MetadataExpr,
  type MetadataExprProps,
} from './components/logic-left-expr/metadata-expr';
import {
  ManualFeedbackExpr,
  type ManualFeedbackExprProps,
} from './components/logic-left-expr/manual-feedback-expr';
import {
  ApiFeedbackExpr,
  type ApiFeedbackExprProps,
} from './components/logic-left-expr/api-feedback-expr';

export const getBizConfig = (customParams: Record<string, any>) => ({
  [BIZ.Cozeloop]: {
    initPlatformConfig: {
      value: [
        PlatformType.Cozeloop,
        PlatformType.Prompt,
        PlatformType.Project,
        PlatformType.Workflow,
        PlatformType.CozeBot,
        PlatformType.VeADK,
        PlatformType.VeAgentkit,
        PlatformType.Ark,
      ] as string[],
      defaultValue: PlatformType.Cozeloop,
      format: 'string',
    },
    initSpanListTypeConfig: {
      value: [
        SpanListType.AllSpan,
        SpanListType.LlmSpan,
        SpanListType.RootSpan,
      ] as string[],
      defaultValue: SpanListType.RootSpan,
      format: 'string',
    },
    platformTypeOptions: [
      {
        value: PlatformType.Workflow,
        label: i18nService.t('filter_coze_workflow'),
      },
      {
        value: PlatformType.CozeBot,
        label: i18nService.t('filter_coze_agent'),
      },
      {
        value: PlatformType.Prompt,
        label: i18nService.t('prompt_development'),
      },
      {
        value: PlatformType.Project,
        label: i18nService.t('filter_coze_app'),
      },
      {
        value: PlatformType.VeADK,
        label: i18nService.t('filter_volcano_ve_agent'),
      },
      {
        value: PlatformType.VeAgentkit,
        label: i18nService.t('filter_volcano_agent_kit'),
      },
      {
        value: PlatformType.Ark,
        label: i18nService.t('filter_ark_app'),
      },
      {
        value: PlatformType.Cozeloop,
        label: i18nService.t('sdk_reporting'),
      },
    ] as OptionProps[],
    spanListTypeOptions: [
      {
        value: SpanListType.RootSpan,
        label: 'Root Span',
      },
      {
        value: SpanListType.AllSpan,
        label: 'All Span',
      },
      {
        value: SpanListType.LlmSpan,
        label: 'Model Span',
      },
    ] as OptionProps[],
    logicExprConfig: {
      customRightRenderMap: {
        [tag.TagContentType.Categorical]: v => (
          <CategoricalSelect
            {...(v as CategoricalSelectProps)}
            customParams={customParams}
          />
        ),
        [tag.TagContentType.Boolean]: v => (
          <CategoricalSelect
            {...(v as CategoricalSelectProps)}
            customParams={customParams}
          />
        ),
        [API_FEEDBACK]: v => (
          <ApiFeedbackExprRight {...(v as ApiFeedbackExprRightProps)} />
        ),
      },
      customLeftRenderMap: {
        [MANUAL_FEEDBACK]: v => (
          <ManualFeedbackExpr
            {...(v as ManualFeedbackExprProps)}
            customParams={customParams}
          />
        ),
        [API_FEEDBACK]: v => (
          <ApiFeedbackExpr {...(v as ApiFeedbackExprProps)} />
        ),
        metadata: v => <MetadataExpr {...(v as MetadataExprProps)} />,
      },
    },
    customViewConfig: {
      visibility: true,
    },
    banner: () => null,
  },
  [BIZ.Fornax]: {
    initPlatformConfig: {
      value: [
        PlatformType.InnerCozeloop,
        PlatformType.InnerDoubao,
        PlatformType.InnerPrompt,
        PlatformType.InnerCozeBot,
      ] as string[],
      defaultValue: PlatformType.InnerCozeloop,
      format: 'string',
    },
    initSpanListTypeConfig: {
      value: [
        SpanListType.AllSpan,
        SpanListType.LlmSpan,
        SpanListType.RootSpan,
      ] as string[],
      defaultValue: SpanListType.RootSpan,
      format: 'string',
    },
    platformTypeOptions: [
      {
        value: PlatformType.InnerCozeloop,
        label: i18nService.t('filter_custom_report'),
      },
      {
        value: PlatformType.InnerDoubao,
        label: i18nService.t('filter_doubao'),
      },
      {
        value: PlatformType.InnerPrompt,
        label: 'Prompt',
      },
      {
        value: PlatformType.InnerCozeBot,
        label: 'Coze Bot',
      },
    ] as OptionProps[],
    spanListTypeOptions: [
      {
        value: SpanListType.AllSpan,
        label: 'All Span',
      },
      {
        value: SpanListType.LlmSpan,
        label: 'Model Span',
      },
      {
        value: SpanListType.RootSpan,
        label: 'Root Span',
      },
    ] as OptionProps[],
    logicExprConfig: {
      customRightRenderMap: {
        [tag.TagContentType.Categorical]: v => (
          <CategoricalSelect
            {...(v as CategoricalSelectProps)}
            customParams={customParams}
          />
        ),
        [tag.TagContentType.Boolean]: v => (
          <CategoricalSelect
            {...(v as CategoricalSelectProps)}
            customParams={customParams}
          />
        ),
        [API_FEEDBACK]: v => (
          <ApiFeedbackExprRight {...(v as ApiFeedbackExprRightProps)} />
        ),
      },
      customLeftRenderMap: {
        [tag.TagContentType.Categorical]: v => (
          <CategoricalSelect
            {...(v as CategoricalSelectProps)}
            customParams={customParams}
          />
        ),
        [tag.TagContentType.Boolean]: v => (
          <CategoricalSelect
            {...(v as CategoricalSelectProps)}
            customParams={customParams}
          />
        ),
        [MANUAL_FEEDBACK]: v => (
          <ManualFeedbackExpr
            {...(v as ManualFeedbackExprProps)}
            customParams={customParams}
          />
        ),
        [API_FEEDBACK]: v => (
          <ApiFeedbackExpr {...(v as ApiFeedbackExprProps)} />
        ),
        metadata: v => <MetadataExpr {...(v as MetadataExprProps)} />,
      },
    },
    customViewConfig: {
      visibility: true,
    },
  },
});

export const getDefaultBizConfig = () => ({
  initPlatformConfig: {
    value: ['open_api'] as string[],
    defaultValue: 'open_api',
    format: 'string',
  },
  initSpanListTypeConfig: {
    value: [
      SpanListType.AllSpan,
      SpanListType.LlmSpan,
      SpanListType.RootSpan,
    ] as string[],
    defaultValue: SpanListType.RootSpan,
    format: 'string',
  },
  platformTypeOptions: [
    {
      value: 'open_api',
      label: 'OpenApi',
    },
  ] as OptionProps[],
  spanListTypeOptions: [
    {
      value: SpanListType.AllSpan,
      label: 'All Span',
    },
    {
      value: SpanListType.LlmSpan,
      label: 'LLM Span',
    },
    {
      value: SpanListType.RootSpan,
      label: 'Root Span',
    },
  ] as OptionProps[],
  customViewConfig: {
    visibility: false,
  },
  logicExprConfig: {
    customLeftRenderMap: {
      metadata: (v: unknown) => <MetadataExpr {...(v as MetadataExprProps)} />,
    },
  },
});
