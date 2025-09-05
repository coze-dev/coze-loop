import React, { useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  EvaluatorPreview,
  formateTime,
  AutoOverflowList,
  TypographyText,
  EvaluationSetPreview,
  EvalTargetPreview,
  CozeUser,
  EvaluateTargetTypePreview,
} from '@cozeloop/evaluate-components';
import {
  type Evaluator,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';

import { DynamicParams } from './dynamic-params';

function DescriptionItem({
  label,
  content,
  className,
  contentClassName,
}: {
  label?: React.ReactNode;
  content?: React.ReactNode;
  className?: string;
  contentClassName?: string;
}) {
  return (
    <div
      className={classNames(
        'flex items-center grow basis-40 h-5 overflow-hidden',
        className,
      )}
    >
      <div className="text-[var(--coz-fg-secondary)] shrink-0 mr-2 w-[90px]">
        {label}
      </div>
      <div className={classNames('grow overflow-hidden', contentClassName)}>
        {content}
      </div>
    </div>
  );
}

const ExperimentDescription = ({
  experiment,
  spaceID,
}: {
  experiment?: Experiment;
  spaceID: Int64;
}) => {
  const [expand, setExpand] = useState(true);
  const {
    eval_set,
    eval_target,
    evaluators,
    start_time,
    end_time,
    base_info,
    desc,
    target_runtime_param,
  } = experiment ?? {};

  const header = (
    <div className="flex items-center gap-2 w-full">
      <div className="text-sm font-semibold">{I18n.t('basic_info')}</div>
      <IconCozArrowDown
        className={classNames(
          'cursor-pointer text-xxl',
          expand ? '' : '-rotate-90',
        )}
        onClick={() => setExpand(!expand)}
      />
    </div>
  );

  const content = (
    <>
      <div className="flex item-center gap-2 w-full">
        <DescriptionItem
          label={I18n.t('evaluation_set')}
          content={
            <EvaluationSetPreview evalSet={eval_set} enableLinkJump={true} />
          }
        />
        <DescriptionItem
          label={I18n.t('evaluation_object_type')}
          content={
            <EvaluateTargetTypePreview type={eval_target?.eval_target_type} />
          }
        />
        <DescriptionItem
          label={I18n.t('evaluation_object')}
          content={
            <EvalTargetPreview
              evalSet={eval_set}
              evalTarget={eval_target}
              spaceID={spaceID}
              enableLinkJump={true}
              size="small"
            />
          }
        />
      </div>
      <div className="flex item-center gap-2 w-full">
        <DescriptionItem
          contentClassName="pr-10"
          label={I18n.t('evaluator')}
          content={
            !evaluators?.length ? (
              '-'
            ) : (
              <AutoOverflowList<Evaluator>
                itemKey={'current_version.id'}
                items={evaluators ?? []}
                itemRender={({ item, inOverflowPopover }) => (
                  <EvaluatorPreview
                    evaluator={item}
                    enableLinkJump={true}
                    defaultShowLinkJump={inOverflowPopover}
                  />
                )}
              />
            )
          }
        />
        <DescriptionItem
          label={I18n.t('creator')}
          content={<CozeUser user={base_info?.created_by} size="small" />}
        />
        <DescriptionItem
          label={I18n.t('create_time')}
          content={formateTime(start_time) || '-'}
        />
      </div>
      <div className="flex item-center gap-2 w-full">
        <DescriptionItem
          label={I18n.t('end_time')}
          content={formateTime(end_time) || '-'}
        />
        <DescriptionItem
          label={I18n.t('description')}
          content={<TypographyText>{desc || '-'}</TypographyText>}
        />
        {target_runtime_param?.json_value &&
        target_runtime_param.json_value !== '{}' ? (
          <DescriptionItem
            label={I18n.t('parameter_injection')}
            content={
              <DynamicParams
                evalTarget={eval_target}
                data={target_runtime_param}
              />
            }
          />
        ) : (
          <DescriptionItem />
        )}
      </div>
    </>
  );

  return (
    <div className="flex flex-col gap-3 w-full">
      {header}
      {expand ? content : null}
    </div>
  );
};

export default ExperimentDescription;
