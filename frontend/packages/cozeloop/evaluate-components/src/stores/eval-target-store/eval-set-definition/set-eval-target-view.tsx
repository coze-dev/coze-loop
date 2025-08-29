import { OpenDetailButton } from '@cozeloop/components';
import { useBaseURL } from '@cozeloop/biz-hooks-adapter';
import { Tag } from '@coze-arch/coze-design';

import { type CreateExperimentValues } from '../../../types/evaluate-target';

/**
 * prompt 评测对象 直接取用 prompt 详情即可
 */
export const SetEvalTargetView = (props: {
  values: CreateExperimentValues;
}) => {
  const { values } = props;
  const { baseURL } = useBaseURL();

  const setDetail = values?.evaluationSetDetail;

  const versionDetail = values?.evaluationSetVersionDetail;

  return (
    <>
      <div className="text-[16px] leading-[22px] font-medium coz-fg-primary mb-5">
        评测对象
      </div>
      <div className="flex flex-row gap-5">
        <div className="flex-1 w-0">
          <div className="text-sm font-medium coz-fg-primary mb-2">类型</div>
          <div className="text-sm font-normal coz-fg-primary">评测集</div>
        </div>
        <div className="flex-1 w-0">
          <div className="text-sm font-medium coz-fg-primary mb-2">
            名称和版本
          </div>
          <div className="flex flex-row items-center gap-1">
            <div className={'text-sm font-normal coz-fg-primary'}>
              {setDetail?.name || '-'}
            </div>
            <Tag color="primary" className="!h-5 !px-2 !py-[2px] rounded-[3px]">
              {versionDetail?.version || '-'}
            </Tag>
            <OpenDetailButton
              url={`${baseURL}/evaluation/datasets/${setDetail?.id}?version=${versionDetail?.id}`}
            />
          </div>
        </div>
      </div>
      <div className="h-10" />
    </>
  );
};
