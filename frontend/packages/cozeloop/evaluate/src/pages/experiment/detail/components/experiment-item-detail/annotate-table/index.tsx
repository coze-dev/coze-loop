import { I18n } from '@cozeloop/i18n-adapter';
import {
  TypographyText,
  TagInput,
  TagDetailLink,
} from '@cozeloop/evaluate-components';
import { InfoTooltip, TooltipWhenDisabled } from '@cozeloop/components';
import { tag, type ColumnAnnotation } from '@cozeloop/api-schema/evaluation';

import { type ExperimentItem } from '@/types/experiment';

interface Props {
  spaceID: string;
  /**
   * 标注项
   */
  annotation?: ColumnAnnotation[];
  /**
   * 实验数据
   */
  data?: ExperimentItem;
  onChange?: () => void;
  onCreateOption?: () => void;
}
export function AnnotateTable({
  spaceID,
  annotation = [],
  data,
  onChange,
  onCreateOption,
}: Props) {
  return (
    <div className="flex flex-col gap-[20px] px-6 py-3">
      {annotation.map(item => (
        <div key={item.tag_key_id} className="flex">
          <div className="flex items-center mr-6">
            <div className="w-[156px] mr-[10px]">
              <TypographyText>{item.tag_key_name}</TypographyText>
            </div>
            {item.description ? (
              <InfoTooltip content={item.description}></InfoTooltip>
            ) : (
              <span className="w-[14px]"></span>
            )}
          </div>
          <TooltipWhenDisabled
            content={
              <div>
                <span className="coz-fg-primary mr-1">
                  {I18n.t('tag_disabled_no_modification')}
                </span>
                <TagDetailLink tagKey={item.tag_key_id} />
              </div>
            }
            theme="dark"
            position="topRight"
            disabled={item.status !== tag.TagStatus.Active}
          >
            <div className="flex-1 min-w-0">
              <TagInput
                key={data?.groupID}
                spaceID={spaceID}
                experimentID={data?.experimentID || ''}
                groupID={(data?.groupID || '') as string}
                turnID={(data?.turnID || '') as string}
                annotation={item}
                annotateRecord={data?.annotateResult?.[item.tag_key_id || '']}
                onChange={onChange}
                onCreateOption={onCreateOption}
              />
            </div>
          </TooltipWhenDisabled>
        </div>
      ))}
    </div>
  );
}
