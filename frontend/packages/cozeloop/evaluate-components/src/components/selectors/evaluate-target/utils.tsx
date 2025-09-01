/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
import {
  type EvalTargetVersion,
  type EvalTarget,
} from '@cozeloop/api-schema/evaluation';
import { Typography } from '@coze-arch/coze-design';

const ellipsis = {
  showTooltip: true,
};

export const getPromptEvalTargetOption = (
  item: EvalTarget,
  onlyShowOptionName?: boolean,
): { value?: string; label?: React.ReactNode } => {
  const etc = item.eval_target_version?.eval_target_content;
  const avatar = '';
  const title = etc?.prompt?.prompt_key || '';
  const subTitle = etc?.prompt?.name || '';

  return {
    value: item.source_target_id,
    label: onlyShowOptionName ? (
      <Typography.Text ellipsis={ellipsis}>{subTitle}</Typography.Text>
    ) : (
      <div className="flex flex-row items-center w-full overflow-hidden">
        {avatar ? (
          <img
            className="w-5 h-5 rounded-[4px] mr-2 flex-shrink-0"
            src={avatar}
          />
        ) : null}
        <Typography.Text
          className={'flex-shrink !max-w-[600px] text-[13px]'}
          ellipsis={ellipsis}
        >
          {title}
        </Typography.Text>
        <Typography.Text
          className={'flex-1 w-0 ml-3 text-xs font-medium coz-fg-secondary'}
          ellipsis={ellipsis}
        >
          {subTitle}
        </Typography.Text>
      </div>
    ),
    ...item,
  };
};

export function getPromptEvalTargetVersionOption(item: EvalTargetVersion): {
  value?: string;
  label?: React.ReactNode;
} {
  return {
    value: item.source_target_version,
    label: (
      <div className="flex flex-row items-center w-full pr-2">
        <div className="flex-shrink-0 text-[13px] coz-fg-plus">
          {item.source_target_version}
        </div>
        <Typography.Text
          className="flex-1 w-0 ml-3 text-xs font-medium coz-fg-secondary"
          ellipsis={{
            showTooltip: true,
            rows: 1,
          }}
        >
          {item.eval_target_content?.prompt?.description}
        </Typography.Text>
      </div>
    ),
    ...item,
  };
}

// 以下为 workflow 的处理逻辑
/**
 * 递归处理 schema 字段，将所有字段路径和类型记录到输出对象中
 * @param schemaObj - 包含 schema 结构的对象
 * @returns 扁平化的字段路径到类型的映射
 */
export function flattenSchemaFields(
  schemaObj: Record<string, any>,
  prefix = '',
): Record<string, string> {
  const result: Record<string, string> = {};

  /**
   * 递归处理单个 schema 项
   * @param item - schema 项
   * @param parentPath - 父级路径
   */
  const processSchemaItem = (item: Record<string, any>, parentPath: string) => {
    const itemName = item.name || '';
    const itemType = item.type || '';
    const currentPath = parentPath ? `${parentPath}.${itemName}` : itemName;

    // 记录当前字段
    if (currentPath && itemType) {
      result[currentPath] = itemType;
    }

    // 如果当前项有 schema 子字段，递归处理子字段
    if (item.schema && Array.isArray(item.schema) && item.schema.length > 0) {
      item.schema.forEach((childItem: Record<string, any>) => {
        processSchemaItem(childItem, currentPath);
      });
    }
  };

  // 开始处理根对象的 schema（不包含根对象名称作为前缀）
  if (schemaObj.schema && Array.isArray(schemaObj.schema)) {
    schemaObj.schema.forEach((item: Record<string, any>) => {
      processSchemaItem(item, prefix);
    });
  }

  return result;
}

/**
 * JSON Schema 的基本结构定义
 */
interface SchemaProperty {
  type?: string;
  properties?: Record<string, SchemaProperty>;
  items?: { type?: string };
  [key: string]: unknown;
}

/**
 * 递归解析评测集 JSON Schema，提取所有节点的路径和类型
 * @param schema - JSON Schema 对象
 * @param prefix - 当前路径前缀
 * @returns 扁平化的路径到类型的映射
 */
export function flattenSchemaProperties(
  schema: SchemaProperty,
  prefix = '',
): Record<string, string> {
  const result: Record<string, string> = {};

  // 记录当前节点（如果有 prefix）
  if (prefix && schema.type) {
    if (schema.type === 'array') {
      result[prefix] = `array<${schema?.items?.type}>`;
    } else {
      result[prefix] = schema.type;
    }
  }

  // 如果是 object 类型且有 properties，继续遍历子节点
  if (
    schema.type === 'object' &&
    schema.properties &&
    Object.keys(schema.properties).length > 0
  ) {
    // 遍历 properties
    for (const [key, value] of Object.entries(schema.properties)) {
      const currentPath = prefix ? `${prefix}.${key}` : key;
      const subSchema = value;

      // 递归处理子节点
      Object.assign(result, flattenSchemaProperties(subSchema, currentPath));
    }
  }

  return result;
}

/**
 * 处理wf数据字段, 递归处理JSON schema数据结构，将嵌套路径作为key，除了schema字段的数据作为value
 * @param schemaArray - JSON schema数组
 * @param prefix - 当前路径前缀
 * @returns 扁平化的路径到数据的映射
 */
export function flattenJsonSchemaData(
  schemaArray: Record<string, any>[],
  prefix = '',
): Record<string, any> {
  if (!schemaArray || !Array.isArray(schemaArray) || !schemaArray.length) {
    return {};
  }
  const result: Record<string, any> = {};

  /**
   * 递归处理单个schema项
   * @param item - schema项
   * @param parentPath - 父级路径
   */
  const processSchemaItem = (item: Record<string, any>, parentPath: string) => {
    const itemName = item.name || '';
    const currentPath = parentPath ? `${parentPath}.${itemName}` : itemName;

    // 创建当前项的数据副本，排除schema字段
    const itemData: Record<string, any> = {
      ...item,
      type: item?.type === 'list' ? `array<${item?.schema?.type}>` : item?.type,
    };
    delete itemData.schema;

    // 记录当前字段
    if (currentPath) {
      result[currentPath] = itemData;
    }

    // 只有object类型才进行递归处理
    if (item.type === 'object') {
      // 处理schema字段（数组形式）
      if (item.schema && Array.isArray(item.schema) && item.schema.length > 0) {
        item.schema.forEach((childItem: Record<string, any>) => {
          processSchemaItem(childItem, currentPath);
        });
      }

      // 处理schema字段（对象形式，如work_experiences的情况）
      if (
        item.schema &&
        typeof item.schema === 'object' &&
        !Array.isArray(item.schema)
      ) {
        const schemaObj = item.schema;
        if (schemaObj.schema && Array.isArray(schemaObj.schema)) {
          schemaObj.schema.forEach((childItem: Record<string, any>) => {
            processSchemaItem(childItem, currentPath);
          });
        }
      }
    }
  };

  // 处理schema数组
  schemaArray.forEach((item: Record<string, any>) => {
    processSchemaItem(item, prefix);
  });

  return result;
}
