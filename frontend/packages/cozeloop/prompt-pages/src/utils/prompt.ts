// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable arrow-body-style */
/* eslint-disable security/detect-object-injection */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { uniqueId } from 'lodash-es';
import dayjs from 'dayjs';
import {
  CozeLoopStorage,
  formatTimestampToString,
  safeParseJson,
} from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
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

export const getInputVariablesFromPrompt = (messageList: Message[]) => {
  const regex = new RegExp(`{{[a-zA-Z]\\w{0,${VARIABLE_MAX_LEN - 1}}}}`, 'gm');
  const messageContents = messageList
    .filter(it => it.role !== Role.Placeholder)
    .map(it => it.content || '');

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

  const placeholderArray = messageList.filter(
    it => it.role === Role.Placeholder,
  );
  const placeholderKeys = placeholderArray.map(it => it?.content);
  const placeholderKeysSet = new Set(placeholderKeys);
  const placeholderKeysArray = Array.from(placeholderKeysSet);

  const placeholderContentArray: VariableDef[] = placeholderKeysArray
    ?.filter(key => key && result.every(k => k !== key))
    ?.map(key => ({
      key,
      type: VariableType.Placeholder,
    }));

  return placeholderContentArray?.length
    ? array.concat(placeholderContentArray)
    : array;
};

export const getMockVariables = (
  variables: VariableDef[],
  mockVariables: VariableVal[],
) => {
  return variables.map(item => {
    const mockVariable = mockVariables.find(it => it.key === item.key);
    return {
      ...item,
      value: mockVariable?.value,
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
