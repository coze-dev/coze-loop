/* eslint-disable security/detect-object-injection */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { uniqueId } from 'lodash-es';
import dayjs from 'dayjs';
import {
  CozeLoopStorage,
  formatTimestampToString,
  safeParseJson,
} from '@cozeloop/toolkit';
import {
  ContentType,
  type Message,
  Role,
  type Tool,
  ToolType,
  type VariableDef,
  VariableType,
  type VariableVal,
} from '@cozeloop/api-schema/prompt';

import { type PromptStorageKey, VARIABLE_MAX_LEN } from '@/consts';

export const messageId = () => {
  const date = new Date();
  return date.getTime() + uniqueId();
};

export function versionValidate(val?: string, basedVersion?: string): string {
  if (!val) {
    return '需要提供 Prompt 版本号';
  }
  const pattern = /^(?:0|[1-9]\d{0,3})(?:\.(?:0|[1-9]\d{0,3})){2}$/;
  const isValid = pattern.test(val);
  if (!isValid) {
    return '版本号格式不正确';
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
      return '版本号不能小于当前版本';
    }
  }
  return '';
}

export function sleep(timer = 600) {
  return new Promise<void>(resolve => {
    setTimeout(() => resolve(), timer);
  });
}

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

export const convertDisplayTime = (time: string) => {
  const date = formatTimestampToString(time, 'YYYY/MM/DD HH:mm:ss');
  const isToday = dayjs().isSame(dayjs(date), 'day');
  if (isToday) {
    return formatTimestampToString(time, 'HH:mm:ss');
  }
  return date;
};

export const scrollToBottom = (ref: React.RefObject<HTMLDivElement>) => {
  if (ref.current) {
    ref.current.scrollTop = ref.current.scrollHeight; // 滚动到容器的底部
  }
};

export function stringifyWithSortedKeys(
  obj: Record<string, any>,
  replacer?: (number | string)[] | null,
  space?: string | number,
) {
  if (!obj) {
    return undefined;
  }
  const sortedKeys = Object.keys(obj).sort();
  const orderedObj: Record<string, any> = {};
  sortedKeys.forEach(key => {
    orderedObj[key] = obj[key];
  });
  return JSON.stringify(orderedObj, replacer, space);
}

export function objSortedKeys(obj: Record<string, any>) {
  if (!obj) {
    return undefined;
  }
  const sortedKeys = Object.keys(obj).sort();
  const orderedObj: Record<string, any> = {};
  sortedKeys.forEach(key => {
    orderedObj[key] =
      typeof obj[key] === 'object' &&
      obj[key] !== null &&
      !Array.isArray(obj[key])
        ? objSortedKeys(obj[key])
        : obj[key];
  });
  return orderedObj;
}

const storage = new CozeLoopStorage({ field: 'prompt' });

export function getPromptStorageInfo<T>(storageKey: PromptStorageKey) {
  const infoStr = storage.getItem(storageKey) || '';
  return safeParseJson<T>(infoStr);
}

export function setPromptStorageInfo<T>(storageKey: PromptStorageKey, info: T) {
  storage.setItem(storageKey, JSON.stringify(info));
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
