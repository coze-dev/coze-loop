import { useLayoutEffect } from 'react';

import { type EvalTargetDefinition } from '../../types/evaluate-target';
import { promptEvalTargetDefinitionPayload } from './prompt-definition/const';
// import { evalSetDefinitionPayload } from './eval-set-definition/const';

// 根据 type 注册
const evalTargetDefinitionMap = new Map<
  string | number,
  EvalTargetDefinition
>();

/**
 * 注册评测对象选择器
 * @returns 注销评测对象选择器 方法
 */
const registerEvalTargetDefinition = (item: EvalTargetDefinition) => {
  evalTargetDefinitionMap.set(item.type, item);
  return () => {
    evalTargetDefinitionMap.delete(item.type);
  };
};

/**
 * 获取 type 评测对象选择器
 * @param type 评测对象的value
 * @returns 评测对象选择器
 */
const getEvalTargetDefinition = (type: string | number) =>
  evalTargetDefinitionMap.get(type);

const getEvalTargetDefinitionList = () =>
  Array.from(evalTargetDefinitionMap.values());

/**
 * 评测对象选择器
 */
export const useEvalTargetDefinition = () => {
  useLayoutEffect(() => {
    registerEvalTargetDefinition(promptEvalTargetDefinitionPayload);
    // registerEvalTargetDefinition(evalSetDefinitionPayload);
  }, []);

  return {
    getEvalTargetDefinition,
    registerEvalTargetDefinition,
    getEvalTargetDefinitionList,
  };
};
