import { useState } from 'react';

import classNames from 'classnames';
import { type EvaluatorVersion } from '@cozeloop/api-schema/evaluation';
import { IconCozArrowRight, IconCozEmpty } from '@coze-arch/coze-design/icons';
import { EmptyState, Loading } from '@coze-arch/coze-design';

import { TemplateInfo } from './template-info';
import { ModelConfigInfo } from './model-config-info';

import emptyStyles from './empty-state.module.less';

export function EvaluatorVersionDetail({
  loading,
  versionDetail,
  className,
}: {
  loading?: boolean;
  versionDetail?: EvaluatorVersion;
  className?: string;
}) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <div
        className={classNames(
          'h-5 my-1 flex flex-row items-center cursor-pointer text-sm coz-fg-primary font-semibold',
          className,
        )}
        onClick={() => setOpen(pre => !pre)}
      >
        {'Prompt 详情'}
        <IconCozArrowRight
          className={classNames(
            'h-4 w-4 ml-2 coz-fg-plus transition-transform',
            open ? 'rotate-90' : '',
          )}
        />
      </div>

      <div className={classNames('', open ? '' : 'hidden')}>
        {loading ? (
          <div className="h-[84px] w-full flex items-center justify-center">
            <Loading
              className="!w-full"
              size="large"
              label="正在加载 Prompt 详情"
              loading={true}
            />
          </div>
        ) : !versionDetail ? (
          <div className="h-[84px] w-full flex items-center justify-center">
            <EmptyState
              size="default"
              icon={<IconCozEmpty className="coz-fg-dim text-32px" />}
              title="暂无数据"
              className={emptyStyles['empty-state']}
              // description="请选择评估器和版本号后再查看"
            />
          </div>
        ) : (
          <div className="mt-4">
            <ModelConfigInfo
              data={
                versionDetail?.evaluator_content?.prompt_evaluator?.model_config
              }
            />

            <div className="h-3" />
            <TemplateInfo
              notTemplate={true}
              data={versionDetail?.evaluator_content}
            />
          </div>
        )}
      </div>
    </>
  );
}
