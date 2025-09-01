/* eslint-disable complexity */
import type React from 'react';
import { useState } from 'react';

import { IconCozArrowDown } from '@coze-arch/coze-design/icons';

import { type NodeData } from './types';

import styles from './tree-styles.module.less';

interface TreeProps {
  treeData: NodeData;
  labelRender?: React.FC<LabelRenderProps>;
  expandedKeys?: Set<string>;
  showLine?: boolean;
  defaultExpandAll?: boolean;
  onExpandedKeysChange?: (expandedKeys: Set<string>) => void;
  onChange?: (treeData: NodeData) => void;
}

interface LabelRenderProps {
  nodeData: NodeData;
  path: string;
  handleToggle?: (key: string, expanded?: boolean) => void;
  parentNodeData?: NodeData | null;
}

interface TreeNodeProps {
  node: NodeData;
  parent: NodeData | null;
  level: number;
  isLast: boolean;
  path: string;
  parentLines: boolean[];
  labelRender?: React.FC<LabelRenderProps>;
  onToggle?: (key: string, expanded?: boolean) => void;
  expandedKeys: Set<string>;
}

export const collectAllKeys = (tree: NodeData): string[] => {
  const keys: string[] = [];

  const traverse = (node: NodeData) => {
    keys.push(node.key); // 收集当前节点的 key
    if (node.children) {
      for (const child of node.children) {
        traverse(child); // 递归处理子节点
      }
    }
  };

  traverse(tree);
  return keys;
};

/**
 * 树节点组件
 * 负责渲染单个树节点及其子节点，包括展开/收起功能、连接线显示等
 *
 * @param node - 当前节点数据
 * @param parent - 父节点数据，根节点时为 null
 * @param path - 节点在树中的路径
 * @param level - 节点层级深度
 * @param isLast - 是否为父节点的最后一个子节点
 * @param parentLines - 父节点连接线显示状态数组
 * @param labelRender - 自定义标签渲染函数
 * @param onToggle - 展开/收起切换回调函数
 * @param expandedKeys - 当前展开的节点 key 集合
 */
const TreeNode: React.FC<TreeNodeProps> = ({
  node,
  parent,
  path = '',
  level,
  isLast,
  parentLines,
  labelRender,
  onToggle,
  expandedKeys,
}) => {
  // 判断当前节点是否有子节点
  const hasChildren = node.children && node.children.length > 0;
  // 判断当前节点是否处于展开状态
  const isExpanded = expandedKeys?.has(node.key) ?? false;
  // 判断是否显示子节点
  const showChildren = hasChildren && isExpanded;
  // 判断是否为根节点
  const isRoot = parent === null;

  /**
   * 处理展开/收起切换
   * 只有当节点有子节点且提供了 onToggle 回调时才执行
   */
  const handleToggle = (key: string, expanded?: boolean) => {
    if (onToggle) {
      onToggle(key, expanded);
    }
  };

  return (
    <>
      {/* 树节点容器 */}
      <div className="tree-node">
        <div className="tree-node-content">
          {/* 连接线渲染区域 */}
          <div className="tree-lines">
            {/* 渲染垂直连接线 */}
            {parentLines.map((isShowLine, index) => (
              <div
                key={index}
                className={`tree-line-vertical ${isShowLine ? 'visible' : ''}`}
              />
            ))}
            {/* 渲染水平连接线和连接点（非根节点时显示） */}
            {level > 0 && (
              <>
                <div
                  className={`tree-line-horizontal ${isLast ? 'last' : ''}`}
                />
                <div className="tree-line-connector" />
              </>
            )}
          </div>

          {/* 展开/收起按钮区域 */}
          <div className="tree-lines !flex-col">
            {/* 展开/收起按钮 */}
            <div
              className="tree-expand-button"
              onClick={() => {
                handleToggle(node?.key);
              }}
            >
              {hasChildren ? (
                // 有子节点时显示箭头图标
                <span
                  className={`tree-arrow ${isExpanded ? 'expanded' : 'shrink'}`}
                >
                  <IconCozArrowDown />
                </span>
              ) : (
                // 无子节点时显示占位符
                <span className="tree-arrow-placeholder" />
              )}
            </div>
            {/* 展开时的垂直连接线 */}
            {hasChildren ? (
              <div
                className={`tree-expand-line grow relative ${isExpanded ? 'expanded' : ''} ${isLast ? 'last' : 'not-last'}`}
              ></div>
            ) : null}
          </div>

          {/* 节点标签内容区域 */}
          <div className="tree-label">
            {labelRender
              ? labelRender({
                  nodeData: node,
                  path,
                  handleToggle,
                  parentNodeData: parent,
                })
              : node.label}
          </div>
        </div>
      </div>

      {/* 子节点渲染区域 */}
      <div
        className="tree-children"
        style={{ display: showChildren ? 'block' : 'none' }}
      >
        {/* 递归渲染子节点 */}
        {node.children?.map((child, index) => (
          <TreeNode
            key={child.key}
            node={child}
            parent={node}
            level={level + 1}
            isLast={
              (node.children && index === node.children.length - 1) ?? false
            }
            path={`${path}.children[${index}]`}
            parentLines={isRoot ? [] : [...parentLines, !isLast]}
            labelRender={labelRender}
            onToggle={onToggle}
            expandedKeys={expandedKeys}
          />
        ))}
      </div>
    </>
  );
};

/**
 * 树形组件
 * 提供树形数据的展示和交互功能，支持节点的展开/收起操作
 *
 * @param treeData - 树形数据的根节点
 * @param labelRender - 自定义节点标签渲染函数
 * @param showLine - 是否显示连接线，默认为 true
 * @param defaultExpandAll - 是否默认展开所有节点，默认为 true
 */
export const Tree: React.FC<TreeProps> = ({
  treeData,
  labelRender,
  showLine = true,
  defaultExpandAll = true,
  expandedKeys,
  onExpandedKeysChange,
}) => {
  // 管理展开状态的节点集合
  // 如果 defaultExpandAll 为 true，则初始展开所有节点
  const [_expandedKeys, _onExpandedKeysChange] = useState<Set<string>>(
    new Set(defaultExpandAll ? collectAllKeys(treeData) : []),
  );
  /**
   * 处理节点展开/收起切换
   * @param key - 要切换展开状态的节点 key
   */
  const handleToggle = (key: string, expanded?: boolean) => {
    const ks = expandedKeys ? expandedKeys : _expandedKeys;
    const onCb = onExpandedKeysChange
      ? onExpandedKeysChange
      : _onExpandedKeysChange;

    const newExpandedKeys = new Set(ks ?? []);
    if (newExpandedKeys.has(key)) {
      // 如果节点已展开，则收起
      if (!expanded) {
        newExpandedKeys.delete(key);
      }
    } else {
      // 如果节点已收起，则展开
      newExpandedKeys.add(key);
    }
    onCb?.(newExpandedKeys);
  };

  return (
    <div className={`tree-container ${styles.tree}`}>
      {/* 渲染根节点，开始递归构建树形结构 */}
      <TreeNode
        key={treeData.key}
        node={treeData}
        parent={null}
        level={0}
        isLast={true}
        path={''}
        parentLines={[]}
        labelRender={labelRender}
        onToggle={handleToggle}
        expandedKeys={expandedKeys ?? _expandedKeys}
      />
    </div>
  );
};
