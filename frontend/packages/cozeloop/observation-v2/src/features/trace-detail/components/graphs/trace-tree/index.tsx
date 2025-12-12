import { type FC, useEffect, useMemo, useState } from 'react';

import cls from 'classnames';

import { type TreeNode, Tree } from '../tree';
import { spanNode2TreeNode, dealTreeNodeHighlight } from './utils';
import { type TraceTreeProps } from './type';
import { defaultProps } from './config';

import styles from './index.module.less';

const TraceTree: FC<TraceTreeProps> = props => {
  const [treeData, setTreeData] = useState<TreeNode[]>();
  const {
    dataSource: spanNodes,
    selectedSpanId,
    matchedSpanIds,
    indentDisabled,
    lineStyle: _lineStyle,
    globalStyle: _globalStyle,
    className,
    onCollapseChange,
    ...restProps
  } = props;

  const lineStyle = useMemo(
    () => ({
      normal: Object.assign(
        {},
        defaultProps.lineStyle?.normal,
        _lineStyle?.normal,
      ),
      select: Object.assign(
        {},
        defaultProps.lineStyle?.select,
        _lineStyle?.select,
      ),
      hover: Object.assign(
        {},
        defaultProps.lineStyle?.hover,
        _lineStyle?.hover,
      ),
    }),
    [_lineStyle],
  );

  const globalStyle = useMemo(
    () => Object.assign({}, defaultProps.globalStyle, _globalStyle),
    [_globalStyle],
  );

  useEffect(() => {
    if (spanNodes && spanNodes.length > 0) {
      const treeNodes = spanNodes.map(spanNode => {
        const treeNode = spanNode2TreeNode({
          spanNode,
          onCollapseChange,
          matchedSpanIds,
        });
        const treeNodeWithHighlight = dealTreeNodeHighlight(
          treeNode,
          selectedSpanId,
        );
        return treeNodeWithHighlight;
      });
      setTreeData(treeNodes);
    }
  }, [spanNodes, selectedSpanId, matchedSpanIds, onCollapseChange]);

  return treeData ? (
    <Tree
      className={cls(styles['trace-tree'], className)}
      treeData={treeData}
      selectedKey={selectedSpanId}
      indentDisabled={indentDisabled}
      lineStyle={lineStyle}
      globalStyle={globalStyle}
      {...restProps}
    />
  ) : null;
};

export { TraceTree };
