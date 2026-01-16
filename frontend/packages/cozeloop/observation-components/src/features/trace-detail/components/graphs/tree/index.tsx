// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { Virtuoso } from 'react-virtuoso';
import { type FC, useEffect, useMemo, useRef, useState } from 'react';

import cs from 'classnames';
import { SpanStatus } from '@cozeloop/api-schema/observation';

import { type SpanNode } from '../trace-tree/type';
import { flattenTreeData, checkIsNodeOrChildSelected } from './util';
import type {
  TreeProps,
  TreeNode,
  TreeNodeExtra,
  MouseEventParams,
  LineStyle,
  GlobalStyle,
} from './typing';
import { PathEnum } from './typing';

import styles from './index.module.less';

export type {
  TreeProps,
  TreeNode,
  TreeNodeExtra,
  MouseEventParams,
  LineStyle,
  GlobalStyle,
};
const Tree: FC<TreeProps> = ({
  treeData,
  selectedKey,
  disableDefaultHover,
  hoverKey: customHoverKey,
  indentDisabled = false,
  className,
  onMouseMove,
  onMouseEnter,
  onMouseLeave,
  onClick,
  onSelect,
  lineStyle,
  virtuosoHeight,
}) => {
  const [hoverKey, setHoverKey] = useState<string>('');
  const virtuosoRef = useRef(null);

  const controlledHoverKey = disableDefaultHover ? customHoverKey : hoverKey;

  const treeNodes = useMemo(() => {
    const temptreeNodes =
      treeData?.map(item => {
        const { nodes } = flattenTreeData(item, {
          indentDisabled,
        });
        return nodes;
      }) || [];
    return temptreeNodes.flat();
  }, [treeData, indentDisabled]);
  const normalLineColor = lineStyle?.normal?.stroke;
  const selectLineColor = lineStyle?.select?.stroke;
  useEffect(() => {
    const index = treeNodes?.findIndex(item => item.key === selectedKey);
    if (selectedKey && index > -1) {
      setTimeout(() => {
        const element = document.querySelector(`[data-key="${selectedKey}"]`);
        if (element) {
          // 判断element是否在视窗内
          const rect = element.getBoundingClientRect();
          if (
            rect.top > 0 &&
            rect.top <=
              (window.innerHeight || document.documentElement.clientHeight)
          ) {
            return;
          }
        }
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (virtuosoRef.current as any)?.scrollToIndex({
          index,
          align: 'center',
        });
      }, 200);
    }
  }, [selectedKey, virtuosoRef]);

  const renderItemContent = (_, node) => {
    const {
      key,
      title,
      selectEnabled = true,
      linePath,
      isLastChild,
      isMatched,
    } = node;

    const selected = selectedKey === key;
    const isCurrentNodeOrChildSelected = checkIsNodeOrChildSelected(
      node,
      selectedKey,
    );
    const nodeExtra: TreeNodeExtra = {
      ...node,
      selected,
      lineStyle,
      isCurrentNodeOrChildSelected,
      hover: controlledHoverKey === key,
    };
    const spanNode = (node?.extra as { spanNode: SpanNode })?.spanNode;
    const isError = spanNode?.status !== SpanStatus.Success;
    return (
      <div
        className={cs(styles['tree-node'], {
          [styles['tree-node-select']]: selected,
          [styles['tree-node-matched']]: isMatched,
        })}
        key={node.key}
        data-key={node.key}
        onClick={event => {
          if (selectEnabled) {
            onSelect?.({ node: nodeExtra });
          }
          onClick?.({ event, node: nodeExtra });
        }}
      >
        {selected ? (
          <div
            className="absolute top-[2px] bottom-[2px] left-0 w-[2px] bg-[rgb(87 105 227)]"
            style={
              selected
                ? {
                    backgroundColor: isError ? '#D0292F' : '#5A4DED',
                  }
                : {}
            }
          ></div>
        ) : null}
        {linePath?.map((line, index) => {
          const isLast = index === linePath.length - 1;
          const isActive = line === PathEnum.Active;
          return (
            <div className="w-[24px] relative " key={index}>
              {isLast ? (
                <div
                  className="absolute left-[12px] top-0 -ml-[0.5px] w-[14px] rounded-bl-[4px]  border-b  border-solid border-l border-t-[0px] border-r-[0px] border-current z-[1] coz-fg-dim"
                  style={{
                    height: 20,
                    borderColor: isCurrentNodeOrChildSelected
                      ? selectLineColor
                      : normalLineColor,
                    zIndex: isCurrentNodeOrChildSelected ? 3 : 1,
                  }}
                ></div>
              ) : null}
              {!(isLastChild && isLast) && line !== PathEnum.Hidden ? (
                <div
                  className="absolute inset-y-0 left-1/2 -ml-[0.5px] w-[1px] coz-fg-dim"
                  style={{
                    zIndex: 2,
                    backgroundColor:
                      isActive && !isCurrentNodeOrChildSelected
                        ? selectLineColor
                        : normalLineColor,
                  }}
                ></div>
              ) : null}
            </div>
          );
        })}
        <div
          className={styles['tree-node-box']}
          onMouseMove={event => {
            onMouseMove?.({ event, node: nodeExtra });
          }}
          onMouseEnter={event => {
            if (selectEnabled) {
              setHoverKey(key);
            }
            onMouseEnter?.({
              event,
              node: { ...nodeExtra, hover: true },
            });
          }}
          onMouseLeave={event => {
            if (selectEnabled) {
              setHoverKey('');
            }
            onMouseLeave?.({
              event,
              node: { ...nodeExtra, hover: false },
            });
          }}
        >
          {typeof title === 'function' ? title(nodeExtra) : title}
        </div>
      </div>
    );
  };
  return (
    <div className={`${styles.tree} ${className ?? ''}`}>
      <div className={styles['tree-container']}>
        <div className={styles['tree-node-list']}>
          <Virtuoso
            style={{ height: virtuosoHeight }}
            className={styles.virtuoso}
            ref={virtuosoRef}
            data={treeNodes}
            itemContent={renderItemContent}
          />
        </div>
      </div>
    </div>
  );
};

export { Tree };
