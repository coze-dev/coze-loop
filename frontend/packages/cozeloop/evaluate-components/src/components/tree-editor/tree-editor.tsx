import { useMemo } from 'react';

import { nanoid } from 'nanoid';
import { cloneDeep } from 'lodash-es';
import { IconCozAddNode, IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { Button, Tooltip } from '@coze-arch/coze-design';

import { type NodeData } from './types';
import { Tree } from './tree-component';

/**
 * 将树形结构转换为扁平化的映射表
 * @param root - 根节点数据
 * @param prefixPath - 可选的路径前缀
 * @returns 返回一个映射表，key为节点key，value包含节点信息、路径和父节点
 */
function treeToMap(root: NodeData, prefixPath?: string) {
  // 定义映射表结构：key为节点key，value包含节点、路径和父节点信息
  const map: Record<
    string,
    {
      node: NodeData;
      path: string;
      parent: NodeData | null;
      parentPath: string;
    }
  > = {};

  /**
   * 递归处理节点，将树形结构扁平化
   * @param node - 当前处理的节点
   * @param parent - 父节点
   * @param path - 当前节点的路径数组
   */
  // eslint-disable-next-line max-params
  function processNode(
    node: NodeData,
    parent: NodeData | null,
    path: string[] = [],
    parentPath = '',
  ) {
    // 创建当前路径的副本
    const currentPath = [...path];
    const currentPathStr = currentPath.join('.');
    // 将当前节点信息存储到映射表中
    map[node.key] = {
      node,
      parent,
      parentPath,
      path: currentPathStr, // 将路径数组转换为点分隔的字符串
    };

    // 如果有子节点，递归处理每个子节点
    if (node.children) {
      node.children.forEach((child, i) =>
        processNode(
          child,
          node,
          [...currentPath, `children[${i}]`],
          currentPathStr,
        ),
      );
    }
  }

  // 从根节点开始处理，如果有前缀路径则添加到路径中
  processNode(root, null, prefixPath ? [prefixPath] : undefined, prefixPath);
  return map;
}

export interface TitleRenderProps {
  nodeData: NodeData;
  path: string;
  parentPath: string;
}

/**
 * 树节点标题渲染组件
 * 负责渲染树节点的标题内容以及操作按钮（添加子节点、删除节点）
 *
 * @param nodeData - 当前节点数据
 * @param path - 节点在树中的路径
 * @param titleRender - 自定义标题渲染函数
 * @param isShowAddNode - 判断是否显示添加子节点按钮的函数
 * @param onAddChild - 添加子节点的回调函数
 * @param onDelete - 删除节点的回调函数
 */
function TitleRender({
  nodeData,
  path,
  parentPath,
  titleRender,
  isShowAddNode,
  isShowAction,
  onAddChild,
  onDelete,
}: TitleRenderProps & {
  titleRender?: React.FC<TitleRenderProps>;
  isShowAddNode?: (nodeData: NodeData) => boolean;
  isShowAction?: boolean;
  onAddChild: (nodeData: NodeData) => void;
  onDelete: (nodeData: NodeData) => void;
}) {
  // 判断是否显示添加子节点按钮，默认为 false
  const showAddNode = isShowAddNode?.(nodeData) ?? false;

  const actionNode = isShowAction ? (
    <div className="flex items-center h-8 gap-1">
      {/* 添加子节点按钮 - 根据 isShowAddNode 函数决定是否显示 */}
      {showAddNode ? (
        <Tooltip content="添加子项">
          <Button
            color="secondary"
            size="mini"
            className="!w-[20px] !h-[20px]"
            icon={<IconCozAddNode className="text-[14px]" />}
            onClick={() => {
              onAddChild(nodeData);
            }}
          />
        </Tooltip>
      ) : (
        // 占位元素，保持布局一致性
        <div className="w-[20px] h-[20px]" />
      )}

      {/* 删除节点按钮 */}
      <Button
        color="secondary"
        size="mini"
        className="!w-[20px] !h-[20px]"
        icon={<IconCozTrashCan className="text-[14px]" />}
        onClick={() => {
          onDelete(nodeData);
        }}
      />
    </div>
  ) : null;

  return (
    <div className="flex items-start gap-3">
      {/* 标题内容区域, 用户传入 */}
      <div className="grow">
        {titleRender
          ? titleRender({ nodeData, path, parentPath })
          : nodeData.label}
      </div>
      {actionNode}
    </div>
  );
}

/**
 * 树形编辑器组件
 * 提供树形数据的展示、添加子节点、删除节点等功能
 */
export function TreeEditor({
  treeData,
  labelRender,
  isShowAddNode,
  isShowAction,
  onChange,
  defaultNodeData,
  ...rest
}: {
  treeData: NodeData;
  labelRender?: React.FC<TitleRenderProps>;
  isShowAddNode?: (nodeData: NodeData) => boolean;
  isShowAction?: boolean;
  onChange?: (treeData: NodeData | null) => void;
  defaultNodeData?: Object;
  expandedKeys?: Set<string>;
  onExpandedKeysChange?: (expandedKeys: Set<string>) => void;
}) {
  // 使用 useMemo 缓存节点映射，避免重复计算
  const nodeMap = useMemo(() => {
    const map = treeToMap(treeData);
    return map;
  }, [treeData]);

  /**
   * 添加子节点处理函数
   * @param parentKey 父节点的 key
   */
  const handleAddChild = (parentKey: string) => {
    // 生成唯一的节点 key
    const newKey = nanoid();
    // 创建新节点数据
    const newNode: NodeData = {
      key: newKey,
      label: `新节点${Math.floor(Math.random() * 1000)}`,
      data: {},
      ...defaultNodeData,
    };

    /**
     * 递归添加节点的内部函数
     * @param node 当前处理的节点
     * @returns 是否成功添加节点
     */
    const addNode = (node: NodeData): boolean => {
      if (node.key === parentKey) {
        // 找到父节点，初始化 children 数组并添加新节点
        if (!node.children) {
          node.children = [];
        }
        node.children.push(newNode);
        return true; // 找到父节点并添加成功
      }

      if (node.children) {
        // 递归查找子节点
        for (const child of node.children) {
          if (addNode(child)) {
            return true; // 子节点递归找到并添加成功
          }
        }
      }
      return false; // 未找到父节点
    };

    // 深拷贝树数据，避免直接修改原数据
    const clonedTree = cloneDeep(treeData);
    addNode(clonedTree);

    // 调用 onChange 回调，通知父组件数据变化
    onChange?.(clonedTree);
  };

  /**
   * 删除节点处理函数
   * @param deleteKey 要删除的节点 key
   */
  const handleDelete = (deleteKey: string) => {
    /**
     * 递归删除节点的内部函数
     * @param node 当前处理的节点
     * @returns 处理后的节点，如果返回 null 则表示该节点被删除
     */
    const removeNode = (node: NodeData): NodeData | null => {
      if (node.key === deleteKey) {
        return null; // 如果当前节点是需要删除的节点，返回 null
      }
      if (node.children) {
        // 过滤掉需要删除的子节点
        node.children = node.children
          .map(removeNode) // 递归处理子节点
          .filter((child): child is NodeData => child !== null); // 过滤掉 null 的节点
      }
      return node;
    };
    // 深拷贝树数据，避免直接修改原数据
    const clonedTree = cloneDeep(treeData);
    const newNode = removeNode(clonedTree);

    // 调用 onChange 回调，通知父组件数据变化
    onChange?.(newNode);
  };

  return (
    <div>
      <Tree
        showLine={true}
        defaultExpandAll={true}
        {...rest}
        treeData={treeData}
        labelRender={({ nodeData, handleToggle }) => {
          // 处理空节点的情况
          if (!nodeData) {
            return '-';
          }
          // 获取节点的路径信息
          const nodeInfo = nodeMap[nodeData.key ?? ''];
          const path = nodeInfo?.path;
          const parentPath = nodeInfo?.parentPath;
          return (
            <TitleRender
              nodeData={nodeData as NodeData}
              path={path}
              parentPath={parentPath}
              titleRender={labelRender}
              isShowAddNode={isShowAddNode}
              isShowAction={isShowAction}
              onAddChild={node => {
                handleAddChild(node.key);
                handleToggle?.(node.key, true);
              }}
              onDelete={node => {
                handleDelete(node.key);
              }}
            />
          );
        }}
      />
    </div>
  );
}
