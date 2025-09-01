import { formatTimestampToString } from '@cozeloop/toolkit';
import {
  type FieldData,
  type Turn,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';

export const fillTurnData = ({
  turns,
  fieldSchemas,
}: {
  turns?: Turn[];
  fieldSchemas?: FieldSchema[];
}): Turn[] => {
  let fillTurns = turns;
  if (!turns?.length) {
    // 添加空数据渲染
    fillTurns = [{}];
  }
  const turnsData = fillTurns?.map(turn => {
    const fieldDataList = fieldSchemas?.map(schema => {
      let fieldData: FieldData | undefined;
      fieldData = turns?.[0]?.field_data_list?.find(
        item => item.key === schema.key,
      );
      if (!fieldData) {
        fieldData = {
          key: schema.key,
          name: schema.name,
          content: {
            content_type: schema?.content_type,
          },
        };
      }
      return fieldData;
    });
    return {
      ...turn,
      field_data_list: fieldDataList,
    };
  });
  return turnsData || [];
};

/**
 * 生成随机id
 * @method createUuid
 * @param {number} len 长度
 * @return {string}
 */
export function createUuid(length = 8) {
  const repo = '1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  let result = 'id';
  const getRandom = () => Math.floor(Math.random() * repo.length);
  for (let i = 0; i < length; i += 1) {
    result += repo[getRandom()];
  }
  return result;
}

/** 格式化时间 YYYY-MM-DD HH:mm:ss */
export function formateTime(time: string | number | undefined) {
  return time ? formatTimestampToString(time, 'YYYY-MM-DD HH:mm:ss') : '';
}

/** 等待 x 毫秒 */
export function wait(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
