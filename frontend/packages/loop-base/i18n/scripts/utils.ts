// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type MessageFormatElement,
  parse,
  TYPE,
} from '@formatjs/icu-messageformat-parser';

interface TypeInfo {
  key: string;
  type: 'string' | 'number' | 'Date' | string;
}

function filterDuplicateKeys(items: TypeInfo[]): TypeInfo[] {
  const seen = new Set<string>();
  const result: TypeInfo[] = [];

  for (const item of items) {
    if (!seen.has(item.key)) {
      seen.add(item.key);
      result.push(item);
    }
  }

  return result;
}

/**
 * Parse {@link MessageFormatElement}
 *
 * See {@link https://formatjs.github.io/docs/core-concepts/icu-syntax#basic-principles}
 */
function parseElement(el: MessageFormatElement): TypeInfo[] | undefined {
  switch (el.type) {
    case TYPE.literal:
      return undefined;
    case TYPE.argument:
      return [{ key: el.value, type: 'string' }];
    case TYPE.number:
    case TYPE.date:
    case TYPE.time:
      return [{ key: el.value, type: 'number' }];
    case TYPE.plural: {
      const types = [{ key: el.value, type: 'number' }];
      // handle var in options, depth = 1
      for (const { value } of Object.values(el.options)) {
        for (const elOfValue of value) {
          const valueTypes = parseElement(elOfValue);
          valueTypes?.length && types.push(...valueTypes);
        }
      }

      return filterDuplicateKeys(types);
    }
    case TYPE.select:
      return [
        {
          key: el.value,
          type: Object.keys(el.options)
            .map(it => (it === 'other' ? 'undefined' : `'${it}'`))
            .join(' | '),
        },
      ];
    case TYPE.pound:
    case TYPE.tag:
    default:
      return undefined;
  }
}

export function icu2Type(message: string) {
  try {
    const elements = parse(message);
    const types: TypeInfo[] = [];

    for (const element of elements) {
      const elementTypes = parseElement(element);

      elementTypes?.length && types.push(...elementTypes);
    }

    return types;
    // eslint-disable-next-line @coze-arch/use-error-in-catch -- skip
  } catch {
    return [];
  }
}
