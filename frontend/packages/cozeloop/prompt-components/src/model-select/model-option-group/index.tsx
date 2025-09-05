import { type PropsWithChildren } from 'react';

import cls from 'classnames';
import {
  IconCozQuestionMarkCircle,
  IconCozStarFill,
} from '@coze-arch/coze-design/icons';
import { Avatar, Divider, Tooltip } from '@coze-arch/coze-design';

export type ModelOptionGroupProps =
  | {
      /** 新模型专区 */
      type: 'new';
      seriesName?: string;
      tooltip?: string;
    }
  | {
      /** 普通系列模型 */
      type?: 'normal';
      icon: string;
      desc: string;
      seriesName?: string;
      tooltip?: string;
    };

export function ModelOptionGroup(
  props: PropsWithChildren<ModelOptionGroupProps>,
) {
  const questionMark = props?.tooltip ? (
    <Tooltip content={props?.tooltip} theme="dark">
      <IconCozQuestionMarkCircle className="cursor-pointer coz-fg-secondary" />
    </Tooltip>
  ) : null;
  return (
    <div className="pb-[2px]">
      {props.type === 'new' ? (
        <div className="flex items-center gap-[4px] coz-fg-hglt">
          <IconCozStarFill />
          <span className="text-[12px] leading-[16px]">{props.seriesName}</span>
          {questionMark}
        </div>
      ) : (
        <div className="flex items-center gap-[6px]">
          <Avatar
            shape="square"
            className="w-[14px] h-[14px] rounded-[3px] !cursor-default border border-solid coz-stroke-primary"
            src={props.icon}
          />
          <div
            className={cls(
              'flex items-center gap-[4px]',
              'text-[12px] leading-[16px]',
            )}
          >
            <span className="coz-fg-secondary">{props.seriesName}</span>
            {questionMark}
            {props.desc ? (
              <>
                <Divider layout="vertical" />
                <span className="coz-fg-dim">{props.desc}</span>
              </>
            ) : null}
          </div>
        </div>
      )}
    </div>
  );
}
