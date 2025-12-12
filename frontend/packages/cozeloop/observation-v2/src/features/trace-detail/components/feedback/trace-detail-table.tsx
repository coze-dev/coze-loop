/* eslint-disable @coze-arch/max-line-per-function */
import React, { useEffect, useState } from 'react';

import dayjs from 'dayjs';
import cls from 'classnames';
import { useUpdate } from 'ahooks';
import { LoopTable } from '@cozeloop/components';
// import LoopTableSortIcon from '@cozeloop/components';
import {
  annotation as AnnotationType,
  type span,
} from '@cozeloop/api-schema/observation';
import { IconCozIllusNone } from '@coze-arch/coze-design/illustrations';
import {
  IconCozLongArrowTopRight,
  IconCozRefresh,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  EmptyState,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import { UserProfile } from '@/shared/components/user-profile';
import { useLocale } from '@/i18n/useLocale';
import { useTraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';

import { ManualAnnotation } from './score';
import { useListAnnotations } from './hooks/use-list-annotations';
const { Text } = Typography;

export const Source = ({
  annotation,
}: {
  annotation: AnnotationType.Annotation;
}) => {
  const { t } = useLocale();

  const SOURCE_TEXT = {
    [AnnotationType.AnnotationType.AutoEvaluate]: t('auto_evaluate'),
    [AnnotationType.AnnotationType.ManualFeedback]: t('manual_feedback'),
    [AnnotationType.AnnotationType.CozeFeedback]: t('coze_feedback'),
    [AnnotationType.AnnotationType.OpenAPIFeedback]: t('openapi_feedback'),
  };
  const { customParams } = useTraceDetailContext();
  const autoTaskRouterLink =
    customParams?.autoTaskRouterLink ?? 'observation/tasks';

  return (
    <div
      className="flex items-center gap-1 group cursor-pointer"
      onClick={() => {
        if (!annotation.auto_evaluate?.task_id) {
          return;
        }
        window.open(
          `${customParams?.baseURL}/${autoTaskRouterLink}/${annotation.auto_evaluate?.task_id}`,
        );
      }}
    >
      <Text ellipsis={{ showTooltip: true }} className="text-[13px]">
        {SOURCE_TEXT[annotation.type ?? ''] ?? '-'}
      </Text>
      {annotation.type === AnnotationType.AnnotationType.AutoEvaluate && (
        <div className="flex items-center group-hover:opacity-100 opacity-0  transition-opacity duration-200">
          <IconCozLongArrowTopRight />
        </div>
      )}
    </div>
  );
};

interface TraceFeedBackProps {
  span: span.OutputSpan;
  platformType: string | number;
  annotationRefreshKey: number;
  customRenderCols?: {
    [key: string]: (annotation: AnnotationType.Annotation) => React.ReactNode;
  };
  description?: React.ReactNode;
}

interface FeedbackResultProps {
  onRefresh: () => void;
  onRefreshLoading?: boolean;
}

const FeedbackResult = (props: FeedbackResultProps) => {
  const { t } = useLocale();
  const { onRefresh, onRefreshLoading } = props;

  return (
    <div className="flex items-center gap-x-1 w-full">
      <Text className="text-inherit font-inherit !font-medium leading-inherit !text-[13px]">
        {t('feedback_result')}
      </Text>
      <Tooltip content={t('refresh_tooltip')} theme="dark">
        <Button color="secondary" size="mini" onClick={onRefresh}>
          <IconCozRefresh
            className={cls('w-[14px] h-[14px] text-[var(--coz-fg-se)]', {
              'animate-spin': onRefreshLoading,
            })}
          />
        </Button>
      </Tooltip>
    </div>
  );
};
interface CreateTimeTitleProps {
  onChange?: () => void;
  descByUpdatedAt?: boolean;
}

const UpdateTimeTitle = ({
  onChange,
  descByUpdatedAt,
}: CreateTimeTitleProps) => {
  const { t } = useLocale();
  return (
    <div
      className="flex items-center gap-x-1 cursor-pointer"
      onClick={() => onChange?.()}
    >
      <span>{t('update_time')}</span>
      {/* <LoopTableSortIcon sortOrder={descByUpdatedAt ? 'descend' : 'ascend'} /> */}
    </div>
  );
};

export const TraceFeedBack = ({
  span,
  customRenderCols,
  annotationRefreshKey,
  platformType,
  description,
}: TraceFeedBackProps) => {
  const { t } = useLocale();
  const update = useUpdate();
  const [descByUpdatedAt, setDescByUpdatedAt] = useState(false);
  const { runAsync, loading } = useListAnnotations({
    span_id: span.span_id,
    trace_id: span.trace_id,
    start_time: span.started_at,
    platform_type: platformType,
  });
  useEffect(() => {
    runAsync(descByUpdatedAt)
      .then(data => {
        span.annotations = data ?? [];
        update();
      })
      .catch(console.log);
  }, [annotationRefreshKey]);
  const columns = [
    {
      title: t('source_column'),
      dataIndex: 'source',
      width: 120,
      render: (_, annotation: AnnotationType.Annotation) => (
        <Source annotation={annotation} />
      ),
    },
    {
      title: () => (
        <FeedbackResult
          onRefresh={() => {
            runAsync(descByUpdatedAt)
              .then(data => {
                span.annotations = data ?? [];
                update();
              })
              .catch(console.log);
          }}
          onRefreshLoading={loading}
        />
      ),
      dataIndex: 'feedback',
      width: 200,
      render: (_, annotation: AnnotationType.Annotation) => (
        <div className="flex items-center min-w-0">
          {customRenderCols?.feedback?.(annotation) ?? (
            <ManualAnnotation annotation={annotation} />
          )}
        </div>
      ),
    },
    {
      title: t('updater_column'),
      dataIndex: 'updater',
      width: 170,
      render: (_, annotation: AnnotationType.Annotation) => {
        const name = annotation.base_info?.updated_by?.name;
        const avatarUrl = annotation.base_info?.updated_by?.avatar_url ?? '-';
        if (!name) {
          return '-';
        }

        return <UserProfile name={name ?? '='} avatarUrl={avatarUrl} />;
      },
    },
    {
      title: t('create_time_column'),
      dataIndex: 'createTime',
      width: 170,
      render: (_, annotation: AnnotationType.Annotation) => {
        const createdAt = annotation.base_info?.created_at ?? '-';
        return (
          <Text
            className="text-right text-[13px]"
            ellipsis={{
              showTooltip: true,
            }}
          >
            {createdAt
              ? dayjs(Number(createdAt)).format('MM-DD HH:mm:ss')
              : '-'}
          </Text>
        );
      },
    },
    {
      title: () => (
        <UpdateTimeTitle
          descByUpdatedAt={descByUpdatedAt}
          onChange={() => {
            runAsync(!descByUpdatedAt)
              .then(data => {
                span.annotations = data ?? [];
                update();
              })
              .catch(console.log);
            setDescByUpdatedAt(pre => !pre);
          }}
        />
      ),
      dataIndex: 'updateTime',
      width: 170,
      render: (_, annotation: AnnotationType.Annotation) => {
        const updatedAt = annotation.base_info?.updated_at ?? '-';
        return (
          <Text
            className="text-right text-[13px]"
            ellipsis={{
              showTooltip: true,
            }}
          >
            {updatedAt
              ? dayjs(Number(updatedAt)).format('MM-DD HH:mm:ss')
              : '-'}
          </Text>
        );
      },
    },

    {
      title: () => <div className="text-left">{t('reason_column')}</div>,
      dataIndex: 'reasoning',
      width: 372,
      render: (_, annotation: AnnotationType.Annotation) => (
        <div className="flex items-center justify-start">
          <Text
            className="text-left text-[13px]"
            ellipsis={{
              showTooltip: true,
            }}
          >
            {annotation.type === AnnotationType.AnnotationType.CozeFeedback ||
            annotation.type === AnnotationType.AnnotationType.OpenAPIFeedback
              ? annotation.reasoning
              : (annotation.auto_evaluate?.evaluator_result?.correction
                  ?.explain ??
                annotation.auto_evaluate?.evaluator_result?.reasoning ??
                annotation.reasoning ??
                '-')}
          </Text>
        </div>
      ),
    },
  ];

  return (
    <LoopTable
      showTableWhenEmpty
      tableProps={{
        columns,
        dataSource: span.annotations,
        pagination: false,
        style: {
          width: '100%',
          height: '100%',
        },
        loading,
      }}
      empty={
        <EmptyState
          size="full_screen"
          icon={<IconCozIllusNone />}
          title={t('no_feedback')}
          description={
            <>
              {description ?? (
                <div className="text-sm max-w-[540px]">
                  {t('create_annotation_button')}
                </div>
              )}
            </>
          }
        />
      }
    />
  );
};
