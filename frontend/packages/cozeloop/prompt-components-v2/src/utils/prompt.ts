/* eslint-disable max-lines */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-magic-numbers */
/* eslint-disable security/detect-object-injection */
/* eslint-disable security/detect-unsafe-regex */
/* eslint-disable security/detect-non-literal-regexp */
import { isEqual } from 'lodash-es';
import dayjs from 'dayjs';
import { CozeLoopStorage, safeJsonParse } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type ContentPart,
  ContentType,
  type Message,
  type Prompt,
  Role,
  type Tool,
  ToolType,
  type VariableDef,
  VariableType,
  type VariableVal,
} from '@cozeloop/api-schema/prompt';
import { type Model } from '@cozeloop/api-schema/llm-manage';

import { usePromptStore } from '@/store/use-prompt-store';
import { VARIABLE_MAX_LEN, type PromptStorageKey } from '@/consts';

// 防止快速操作导致的id重复，增加messageCounter，最多到100，个人维度一致即可
let messageCounter = 0;
export const messageId = () => {
  if (messageCounter > 100) {
    messageCounter = 0;
  }
  messageCounter++;
  const date = new Date();
  return `${date.getTime() + messageCounter}`;
};

export const getPlaceholderErrorContent = (
  message?: Message,
  variables?: VariableDef[],
) => {
  if (message?.role === Role.Placeholder) {
    if (!message?.content) {
      return I18n.t('prompt_placeholder_variable_name_not_empty');
    }
    if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(message?.content)) {
      return I18n.t('placeholder_format');
    }
    const normalVariables = variables?.filter(
      it => it.type !== VariableType.Placeholder,
    );
    const hasSameKey = normalVariables?.find(it => it.key === message?.content);
    if (hasSameKey) {
      return I18n.t('prompt_placeholder_variable_name_duplication');
    }
  }
  return '';
};

/**
 * 拆分多模态变量内容
 * @param content 包含多模态变量标签的内容
 * @returns 拆分后的数组，每个元素包含 type 和 text 属性
 */
export const splitMultimodalContent = (content: string) => {
  const result: Array<{ type: ContentType; text: string }> = [];

  // 使用正则表达式匹配 <multimodal-variable>xxx</multimodal-variable> 标签
  const regex = /<multimodal-variable>(.*?)<\/multimodal-variable>/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while (true) {
    match = regex.exec(content);
    if (!match) {
      break;
    }
    // 添加标签前的文本（如果有的话）
    if (match.index > lastIndex) {
      const textBefore = content.slice(lastIndex, match.index);
      if (textBefore) {
        result.push({ type: ContentType.Text, text: textBefore });
      }
    }

    // 添加多模态变量内容
    result.push({ type: ContentType.MultiPartVariable, text: match[1] });

    // 更新索引位置
    lastIndex = match.index + match[0].length;
  }

  // 添加标签后的剩余文本（如果有的话）
  if (lastIndex < content.length && result.length) {
    const textAfter = content.slice(lastIndex);
    if (textAfter) {
      result.push({ type: ContentType.Text, text: textAfter });
    }
  }

  // 如果没有匹配到任何标签，返回原始文本
  if (result.length === 0) {
    return [];
  }

  return result;
};

export const multimodalPartsToContent = (parts: ContentPart[]) => {
  const newPartsText = parts.map(part => {
    if (part.type === ContentType.MultiPartVariable) {
      return `<multimodal-variable>${part.text}</multimodal-variable>`;
    }
    return part.text;
  });
  return newPartsText.join('');
};

export function getMultimodalVariableText(variableName: string) {
  return `<multimodal-variable>${variableName}</multimodal-variable>`;
}

export const convertMultimodalMessage = (message: Message) => {
  const { parts, content } = message;
  if (parts?.length && content) {
    return {
      ...message,
      content: '',
      parts: parts.concat({
        type: ContentType.Text,
        text: content,
      }),
    };
  }
  return message;
};

export const convertMultimodalMessageToSend = (message: Message) => {
  const { parts, content } = message;
  if (parts?.length && content) {
    const newParts = parts.map(it => {
      if (it.type === ContentType.ImageURL) {
        return {
          ...it,
        };
      }
      return it;
    });
    return {
      ...message,
      content: '',
      parts: newParts.concat({
        type: ContentType.Text,
        text: content,
      }),
    };
  } else if (parts?.length) {
    const newParts = parts.map(it => {
      if (it.type === ContentType.ImageURL) {
        return {
          ...it,
        };
      }
      return it;
    });
    return {
      ...message,
      content: '',
      parts: newParts,
    };
  }
  return message;
};

function flattenArray(arr: unknown[]) {
  let flattened: unknown[] = [];
  for (const item of arr) {
    if (Array.isArray(item)) {
      flattened = flattened.concat(flattenArray(item));
    } else {
      flattened.push(item);
    }
  }
  return flattened;
}

export const getMultiModalVariableKeys = (
  messageList: Message[],
  existKeys: string[],
) => {
  const multiModalMessageArray = messageList.filter(it => it.parts?.length);
  const multiModalVariableKeys = multiModalMessageArray
    .map(it =>
      it.parts?.filter(part => part.type === ContentType.MultiPartVariable),
    )
    .flat()
    .map(it => it?.text);
  const multiModalVariableKeysSet = new Set(multiModalVariableKeys);
  const multiModalVariableKeysArray = Array.from(multiModalVariableKeysSet);
  const multiModalVariableArray: VariableDef[] = multiModalVariableKeysArray
    ?.filter(key => key && existKeys.every(k => k !== key))
    ?.map(key => ({
      key,
      type: VariableType.MultiPart,
    }));
  return multiModalVariableArray;
};

export const getPlaceholderVariableKeys = (
  messageList: Message[],
  existKeys: string[],
) => {
  const placeholderArray = messageList.filter(
    it => it.role === Role.Placeholder,
  );

  const placeholderKeys = placeholderArray.map(it => it?.content);
  const placeholderKeysSet = new Set(placeholderKeys);
  const placeholderKeysArray = Array.from(placeholderKeysSet);

  const placeholderVariablesArray: VariableDef[] = placeholderKeysArray
    ?.filter(key => key && existKeys.every(k => k !== key))
    ?.map(key => ({
      key,
      type: VariableType.Placeholder,
    }));
  return placeholderVariablesArray;
};

export const getInputVariablesFromPrompt = (messageList: Message[]) => {
  const regex = new RegExp(`{{[a-zA-Z]\\w{0,${VARIABLE_MAX_LEN - 1}}}}`, 'gm');
  const messageContents = messageList
    .filter(it => it.role !== Role.Placeholder)
    .map(item => {
      if (item.parts?.length) {
        return item.parts
          .map(it => {
            if (it.type === ContentType.MultiPartVariable) {
              return `<multimodal-variable>${it?.text}</multimodal-variable>`;
            }
            return it.text;
          })
          .join('');
      }
      return item.content || '';
    });

  const resultArr = messageContents.map(str =>
    str.match(regex)?.map(key => key.replace('{{', '').replace('}}', '')),
  );

  const flatArr = flattenArray(resultArr)?.filter(v => Boolean(v)) as string[];
  const resultSet = new Set(flatArr);

  const result = Array.from(resultSet);

  const array: VariableDef[] = result.map(key => ({
    key,
    type: VariableType.String,
  }));

  const multiModalVariableArray = getMultiModalVariableKeys(
    messageList,
    result,
  );

  if (multiModalVariableArray?.length) {
    result.push(...multiModalVariableArray.map(it => it.key || ''));
    array.push(...multiModalVariableArray);
  }

  const placeholderVariableArray = getPlaceholderVariableKeys(
    messageList,
    result,
  );

  return placeholderVariableArray?.length
    ? array.concat(placeholderVariableArray)
    : array;
};

export const getMockVariables = (
  variables: VariableDef[],
  mockVariables: VariableVal[],
) => {
  const map = new Map();
  variables.forEach((item, index) => {
    map.set(item.key, index);
  });
  return variables.map(item => {
    const mockVariable = mockVariables.find(it => it.key === item.key);
    return {
      ...item,
      value: mockVariable?.value,
      multi_part_values: mockVariable?.multi_part_values,
      placeholder_messages: mockVariable?.placeholder_messages,
    };
  });
};

export function getToolNameList(tools: Array<Tool> = []): Array<string> {
  const toolNameList: Array<string> = [];

  tools.forEach(item => {
    if (item?.type === ToolType.Function && item?.function?.name) {
      toolNameList.push(item?.function?.name);
    }
  });
  return toolNameList;
}

export function versionValidate(val?: string, basedVersion?: string): string {
  if (!val) {
    return I18n.t('prompt_version_number_needed');
  }
  const pattern = /^(?:0|[1-9]\d{0,3})(?:\.(?:0|[1-9]\d{0,3})){2}$/;
  const isValid = pattern.test(val);
  if (!isValid) {
    return I18n.t('incorrect_version_number');
  }
  const versionNos = val.split('.') || [];
  const basedNos = basedVersion?.split('.') || [0, 0, 0];
  const comparedVersions: Array<Array<number>> = versionNos.map(
    (item, index) => [Number(item), Number(basedNos[index])],
  );
  for (const [curV, baseV] of comparedVersions) {
    if (curV > baseV) {
      return '';
    }
    if (curV < baseV) {
      return I18n.t('version_number_lt_error');
    }
  }
  return '';
}

/**
 * 递增版本号
 * @param version 当前版本号，格式为 a.b.c
 * @returns 下一个版本号
 */
export function nextVersion(version?: string): string {
  if (!version) {
    return '0.0.1';
  }
  const parts = version.split('.').map(Number);
  if (parts.length !== 3 || parts.some(n => isNaN(n) || n < 0 || n > 9999)) {
    return '0.0.1';
  }
  let [a, b, c] = parts;
  c += 1;
  if (c > 9999) {
    c = 0;
    b += 1;
    if (b > 9999) {
      b = 0;
      a += 1;
      if (a > 9999) {
        return '10000.0.0';
      }
    }
  }
  return [a, b, c].join('.');
}

const storage = new CozeLoopStorage({ field: 'prompt' });

export function getPromptStorageInfo<T>(storageKey: PromptStorageKey) {
  const infoStr = storage.getItem(storageKey) || '';
  return safeJsonParse<T>(infoStr);
}

export function setPromptStorageInfo<T>(storageKey: PromptStorageKey, info: T) {
  storage.setItem(storageKey, JSON.stringify(info));
}

// 解决 trace 详情异步上报的问题
const offsetTime = 24;

export const getEndTime = (
  startTime: number | string,
  latency: number | string,
) =>
  dayjs(Number(startTime))
    .add(Number(latency) + 1000, 'millisecond')
    .add(offsetTime, 'hours')
    .valueOf()
    .toString();

export const getStartTime = (startTime: number | string) =>
  dayjs(Number(startTime)).subtract(offsetTime, 'hours').valueOf().toString();

export function convertSnippetsToMap(snippets: Prompt[]) {
  const map: Record<string, Prompt> = {};
  snippets.forEach(snippet => {
    map[
      `<fornax_prompt>id=${snippet.id}&version=${snippet.prompt_commit?.commit_info?.version}</fornax_prompt>`
    ] = snippet;
  });
  return map;
}

const snippetRegex = new RegExp(
  '<fornax_prompt>id=\\d+&version=[\\w.]+</fornax_prompt>',
  'gm',
);

export function messagesHasSnippet(messageList: Message[]) {
  return messageList.some(it => {
    if (it.parts?.length) {
      const array = it.parts.filter(
        partItem => (partItem.type === ContentType.Text && partItem.text) || '',
      );
      return array.some(item => snippetRegex.test(item.text || ''));
    }

    return snippetRegex.test(it.content || '');
  });
}

export function messageHasSnippetError(
  messageList: Message[],
  tempateType: string,
) {
  const storeState = usePromptStore.getState();
  const snippetMap = storeState.snippetMap || {};
  const processedKeys = new Set<string>();

  // 检查文本中是否包含不匹配tempateType的snippet
  const checkTextForInvalidSnippet = (text: string): boolean => {
    const matches = text.match(snippetRegex);
    if (!matches) {
      return false;
    }

    for (const match of matches) {
      // 如果已经处理过这个key，跳过
      if (processedKeys.has(match)) {
        continue;
      }

      // 获取对应的prompt并检查
      const prompt = snippetMap[match];
      if (
        prompt?.prompt_commit?.detail?.prompt_template?.template_type !==
        tempateType
      ) {
        return true; // 找到不匹配的立即返回
      }

      processedKeys.add(match);
    }

    return false;
  };

  // 遍历所有消息
  for (const message of messageList) {
    // 检查消息内容
    if (message.content && checkTextForInvalidSnippet(message.content)) {
      return true; // 找到不匹配的立即返回
    }

    // 检查消息parts
    if (message.parts?.length) {
      for (const part of message.parts) {
        if (part.text && checkTextForInvalidSnippet(part.text)) {
          return true; // 找到不匹配的立即返回
        }
      }
    }
  }

  return false;
}

/**
 * Find all keys with different values between two objects
 * @param obj1 First object to compare
 * @param obj2 Second object to compare
 * @returns Array of keys with different values
 */
export function diffKeys(
  obj1: Record<string, any>,
  obj2: Record<string, any>,
): string[] {
  // Get all unique keys from both objects
  const allKeys = new Set([...Object.keys(obj1), ...Object.keys(obj2)]);

  // Filter keys where values are different
  return Array.from(allKeys).filter(key => !isEqual(obj1[key], obj2[key]));
}

export const isGeminiV2Model = (currentModel?: Model) => {
  if (currentModel) {
    return Boolean(currentModel?.name?.includes('gemini-2'));
  }
};

export function addVariablesInMap(key: string, text: string) {
  const { setVariablesVersionMap } = usePromptStore.getState();
  setVariablesVersionMap(map => {
    const newMap = map || {};
    const currentValues = newMap?.[key] || [];

    if (Array.isArray(currentValues)) {
      const newValues = [...currentValues, text];
      return {
        ...newMap,
        [key]: Array.from(new Set(newValues)),
      };
    } else {
      return {
        ...newMap,
        [key]: [text],
      };
    }
  });
}

export function removeVariablesInMap(key: string, text: string) {
  const { setVariablesVersionMap } = usePromptStore.getState();
  setVariablesVersionMap(map => {
    const newMap = map || {};
    const currentValues = newMap?.[key] || [];

    if (Array.isArray(currentValues)) {
      const newValues = currentValues.filter(it => it !== text);
      if (newValues.length === 0) {
        return {
          ...newMap,
          [key]: [],
        };
      } else {
        return {
          ...newMap,
          [key]: Array.from(new Set(newValues)),
        };
      }
    }
    return map;
  });
}

function strHasVariable(str: string, varibale: string) {
  return str.includes(`{{${varibale}}}`) || str.includes(`{{.${varibale}}}`);
}

export function messagesHasVariable(messageList: Message[], varibale: string) {
  return messageList.some(it => {
    if (it.parts?.length) {
      const array = it.parts.filter(
        partItem => (partItem.type === ContentType.Text && partItem.text) || '',
      );
      const multiVariables = it.parts.filter(
        partItem => partItem.type === ContentType.MultiPartVariable,
      );
      const inMultiPart = multiVariables.some(mp => mp.text === varibale);

      return (
        inMultiPart ||
        array.some(item => strHasVariable(item.text || '', varibale))
      );
    } else if (it.role === Role.Placeholder) {
      return it.content === varibale;
    }

    return strHasVariable(it.content || '', varibale);
  });
}

export function extractSnippetStrings(messageList: Message[]): string[] {
  const snippets: string[] = [];

  messageList.forEach(message => {
    const content = message.content || '';
    const contentMatches = content.match(snippetRegex);
    if (contentMatches) {
      snippets.push(...contentMatches);
    }

    if (message.parts?.length) {
      message.parts.forEach(part => {
        if (part.type === ContentType.Text && part.text) {
          const partMatches = part.text.match(snippetRegex);
          if (partMatches) {
            snippets.push(...partMatches);
          }
        }
      });
    }
  });

  return snippets;
}

export function variablesAddSourceMap(
  messageList: Message[],
  variablesDefs: VariableDef[],
) {
  variablesDefs.forEach(it => {
    if (it.key) {
      const hasVariable = messagesHasVariable(messageList, it.key);
      if (hasVariable) {
        addVariablesInMap(it.key, 'Prompt');
      } else {
        removeVariablesInMap(it.key, 'Prompt');
      }
    }
  });

  const snippetMap = usePromptStore.getState().snippetMap || {};
  const currentSnippetKeys = extractSnippetStrings(messageList || []);
  const currentSnippetKeysSet = new Set(currentSnippetKeys);
  const allSnippetMapKeys = Object.keys(snippetMap);

  allSnippetMapKeys.forEach(key => {
    const prompt = snippetMap[key];
    if (!prompt) {
      return;
    }

    const variables =
      prompt.prompt_commit?.detail?.prompt_template?.variable_defs || [];

    if (currentSnippetKeysSet.has(key)) {
      // Snippet is in use, add its variables
      variables.forEach(it => {
        it.key && addVariablesInMap(it.key, key);
      });
    } else {
      // Snippet is not in use, remove its variables
      variables.forEach(it => {
        it.key && removeVariablesInMap(it.key, key);
      });
    }
  });
}
