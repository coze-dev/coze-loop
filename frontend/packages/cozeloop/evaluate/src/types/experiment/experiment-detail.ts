import {
  type FieldData,
  type Content,
  type EvaluatorRecord,
  type TurnRunState,
  type ItemRunState,
  type FilterField,
  type AnnotateRecord,
} from '@cozeloop/api-schema/evaluation';
import { type ColumnProps } from '@coze-arch/coze-design';

export interface ExperimentItem {
  experimentID: string;
  id: Int64;
  groupID: Int64;
  turnID: Int64;
  groupIndex: number;
  turnIndex: number;
  datasetRow: Record<string, FieldData>;
  actualOutput: Content | undefined;
  targetErrorMsg: string | undefined;
  evaluatorsResult: Record<string, EvaluatorRecord | undefined>;
  annotateResult: Record<string, AnnotateRecord | undefined>;
  runState: TurnRunState | undefined;
  groupRunState: ItemRunState | undefined;
  itemErrorMsg: string | undefined;
  logID: Int64 | undefined;
  evalTargetTraceID: Int64 | undefined;
}

/** 实验详情表格专用的列配置 */
export interface ExperimentDetailColumn extends ColumnProps<ExperimentItem> {
  /** 是否隐藏 */
  hidden?: boolean;
  /** 列管理中使用的标题名字 */
  displayName?: React.ReactNode;
  /** 是否禁用列管理 */
  disableColumnManage?: boolean;
  /** 列管理对应评测服务端接口筛选对应的配置 */
  filterField?: FilterField;
}
