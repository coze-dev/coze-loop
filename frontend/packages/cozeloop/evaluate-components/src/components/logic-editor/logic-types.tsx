import { type Expr, type ExprGroup } from '@cozeloop/components';
import { UserSelect } from '@cozeloop/biz-components-adapter';
import {
  CozInputNumber,
  DatePicker,
  Input,
  Select,
  TextArea,
} from '@coze-arch/coze-design';

export interface LogicOperation {
  label: string;
  value: string;
}

export type LogicFilterLeft = string | string[];

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type LogicFilter = ExprGroup<LogicFilterLeft, string, any>;

export interface RenderProps {
  disabled?: boolean;
  fields: LogicField[];
  /** 开启级联模式，佐治会变成数组 */
  enableCascadeMode?: boolean;
}

/** 逻辑编辑器的字段 */
export interface LogicField {
  /** 字段标题 */
  title: React.ReactNode;
  /** 字段名称 */
  name: string;
  /** 字段类型 */
  type: 'string' | 'number' | 'options' | 'coze_user' | 'custom';
  /* 自定义操作符右边的输入编辑器的属性，例如给下拉框传递optionList */
  setterProps?: Record<string, unknown>;
  /** 自定义操作符右边的输入编辑器 */
  setter?: LogicSetter;
  /** 禁用操作符列表 */
  disabledOperations?: string[];
  /** operator 自定义属性 */
  operatorProps?: Record<string, unknown>;
  /** 自定义操作符列表，会覆盖原有列表 */
  customOperations?: LogicOperation[];
  /** 子字段 */
  children?: LogicField[];
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export interface DataTypeSetterProps<T = any> {
  value: T;
  expr: Expr | undefined;
  field: LogicField;
  disabled: boolean;
  onChange: (val: T) => void;
}

export type LogicSetter = (props: DataTypeSetterProps) => JSX.Element | null;

export interface LogicDataType {
  type: 'string' | 'number' | 'date' | 'options' | 'coze_user';
  operations: LogicOperation[];
  setter: LogicSetter;
}

const baseOperations: LogicOperation[] = [
  {
    label: '包含',
    value: 'contains',
  },
  {
    label: '不包含',
    value: 'not-contains',
  },
  {
    label: '等于',
    value: 'equals',
  },
  {
    label: '不等于',
    value: 'not-equals',
  },
];

const stringOperations: LogicOperation[] = [
  // 注意：字符串类型的包含不包含和选项类的包含不包含枚举值不同，需要like模式
  {
    label: '包含',
    value: 'like',
  },
  {
    label: '不包含',
    value: 'not-like',
  },
  {
    label: '等于',
    value: 'equals',
  },
  {
    label: '不等于',
    value: 'not-equals',
  },
];

const numberOperations: LogicOperation[] = [
  {
    label: '等于',
    value: 'equals',
  },
  {
    label: '不等于',
    value: 'not-equals',
  },
  {
    label: '大于',
    value: 'greater-than',
  },
  {
    label: '大于等于',
    value: 'greater-than-equals',
  },
  {
    label: '小于',
    value: 'less-than',
  },
  {
    label: '小于等于',
    value: 'less-than-equals',
  },
];

const dateOperations: LogicOperation[] = [
  {
    label: '等于',
    value: 'equals',
  },
  {
    label: '不等于',
    value: 'not-equals',
  },
  {
    label: '晚于',
    value: 'greater-than',
  },
  {
    label: '早于',
    value: 'less-than',
  },
];

const selectOperations: LogicOperation[] = [
  {
    label: '包含',
    value: 'contains',
  },
  {
    label: '不包含',
    value: 'not-contains',
  },
];

const userOperations: LogicOperation[] = [...baseOperations];

function StringSetter({
  /** 默认为多行文本模式 */
  textAreaMode = true,
  ...props
}: DataTypeSetterProps<string> & { textAreaMode?: boolean }) {
  if (textAreaMode === false) {
    return <Input placeholder="请输入" {...props} />;
  }
  return <TextArea placeholder="请输入" rows={1} {...props} />;
}

function NumberSetter(props: DataTypeSetterProps<number>) {
  const { value, onChange, ...rest } = props;
  return (
    <CozInputNumber
      placeholder="请输入"
      {...rest}
      className={`w-full ${(props as { className?: string }).className ?? ''}`}
      value={value ?? ''}
      onChange={onChange as (val: number | string) => void}
    />
  );
}
function DateSetter(props: DataTypeSetterProps<string>) {
  const { value, onChange, ...rest } = props;
  return (
    <DatePicker
      {...rest}
      value={value}
      onChange={val => onChange(val as string)}
    />
  );
}

function SelectSetter(
  props: DataTypeSetterProps<string> & {
    className?: string;
    optionList?: { label: string; value: string }[];
  },
) {
  const { value, onChange, optionList = [], className = '', ...rest } = props;
  return (
    <Select
      placeholder="请选择"
      {...rest}
      className={`w-full ${className}`}
      optionList={optionList}
      value={value}
      onChange={val => onChange(val as string)}
    />
  );
}

function CozeUserSetter(
  props: DataTypeSetterProps<string[]> & { className?: string },
) {
  const { value, onChange, className = '', ...rest } = props;
  return (
    <UserSelect
      placeholder="请选择"
      {...rest}
      className={`w-full ${className}`}
      value={value}
      onChange={val => onChange(val as string[])}
    />
  );
}

export const dataTypeList: LogicDataType[] = [
  {
    type: 'string',
    operations: stringOperations,
    setter: StringSetter,
  },
  {
    type: 'number',
    operations: numberOperations,
    setter: NumberSetter as unknown as LogicSetter,
  },
  {
    type: 'date',
    operations: dateOperations,
    setter: DateSetter,
  },
  {
    type: 'options',
    operations: selectOperations,
    setter: SelectSetter,
  },
  {
    type: 'coze_user',
    operations: userOperations,
    setter: CozeUserSetter,
  },
];
