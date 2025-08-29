/**
 * 树节点数据结构
 * 定义树形组件中每个节点的基本属性
 */
export interface NodeData {
  /** 节点的唯一标识符 */
  key: string;
  /** 节点的标题，用于显示 */
  title?: string;
  /** 节点的标签，用于显示 */
  label?: string;
  /** 节点的自定义数据，可以存储任意键值对 */
  data?: Record<string, unknown>;
  /** 子节点数组，用于构建树形结构 */
  children?: NodeData[];
}

/**
 * 标题渲染函数类型
 * 用于自定义树节点的标题显示内容
 */
export type TitleRender = React.FC<{ nodeData: NodeData }>;

/**
 * 展开/收起节点的回调函数类型
 * @param keys - 当前展开的节点key数组
 * @param option - 展开选项，包含展开状态和节点数据
 */
export type ExpandFn = (
  keys: string[],
  option: { expanded: boolean; nodeData: NodeData },
) => void;

/**
 * 字段树组件的属性接口
 * 定义树形编辑器组件接收的props
 */
export interface FieldTreeProps {
  /** 树形数据，包含完整的节点结构 */
  treeData: NodeData;
  /** 自定义标题渲染函数 */
  titleRender?: TitleRender;
  /** 当前展开的节点key数组 */
  expandedKeys?: string[];
  /** 节点展开/收起的回调函数 */
  onExpand?: ExpandFn;
}
