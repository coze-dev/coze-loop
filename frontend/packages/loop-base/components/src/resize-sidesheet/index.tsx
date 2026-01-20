// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cn from 'classnames';
import { IconCozCrossFill } from '@coze-arch/coze-design/icons';
import {
  type SideSheetReactProps,
  Divider,
  SideSheet,
} from '@coze-arch/coze-design';
import { Button } from '@coze-arch/coze-design';

import { useDrag, type DragOptions } from './use-drag';

import styles from './index.module.less';

export const ResizeSidesheet = (
  props: SideSheetReactProps & {
    dragOptions?: DragOptions;
    showDivider?: boolean;
    disableDrag?: boolean;
  },
) => {
  const {
    children,
    dragOptions,
    title,
    onCancel,
    className,
    showDivider,
    disableDrag,
    ...rest
  } = props;
  const { containerRef, isActive, sidePaneWidth } = useDrag(dragOptions);
  return (
    <SideSheet
      title={
        <div className="flex items-center gap-2">
          <div className="flex-1">{title}</div>
          {showDivider ? (
            <Divider layout="vertical" className="h-[12px]" />
          ) : null}
          <Button
            type="primary"
            color="secondary"
            icon={<IconCozCrossFill className="w-[16px] h-[16px]" />}
            onClick={onCancel}
          />
        </div>
      }
      width={disableDrag ? undefined : sidePaneWidth}
      className={cn(styles.sheet, className)}
      {...rest}
      onCancel={onCancel}
      closable={false}
    >
      {disableDrag ? null : (
        <div
          ref={containerRef}
          className={cn(
            'absolute h-full w-[2px] z-[20000] bg-transparent  top-0 left-0 hover:cursor-col-resize hover:bg-[rgb(var(--coze-up-brand-9))] transition ',
            {
              'bg-[rgb(var(--coze-up-brand-9))] cursor-col-resize': isActive,
            },
          )}
        />
      )}
      {props.children}
    </SideSheet>
  );
};
