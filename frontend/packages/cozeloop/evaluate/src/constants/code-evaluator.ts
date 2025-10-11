import { LanguageType } from '@cozeloop/api-schema/evaluation';

/** 前端使用语言类型 */
export enum CodeEvaluatorLanguageFE {
  Python = 'python',
  Javascript = 'javascript',
}

export enum SmallLanguageType {
  JS = 'js',
  Python = 'python',
}

/** LanguageType 服务端 -> 前端字段映射非标准, 手动转换一下 */
export const codeEvaluatorLanguageMap: Record<
  LanguageType & SmallLanguageType,
  string
> = {
  // Python -> python
  [LanguageType.Python]: 'python',
  // JS -> javascript
  [LanguageType.JS]: 'javascript',
  // 兼容一下服务端
  [SmallLanguageType.JS]: 'javascript',
  [SmallLanguageType.Python]: 'python',
};

/** LanguageType 前端 -> 服务端字段映射非标准, 手动转换一下 */
export const codeEvaluatorLanguageMapReverse: Record<string, LanguageType> = {
  // python -> Python
  python: LanguageType.Python,
  // javascript -> JS
  javascript: LanguageType.JS,
};

export const defaultJSCode =
  "function exec_evaluation(turn) {\n  /** 检查turn中某字段是否等于目标值（仅处理Equals规则） */\n  const TARGET_VALUE = \"Text\";\n\n  try {\n    // 直接访问目标字段\n    const current = turn.turn.actual_output.text;\n\n    const isEqual = current === TARGET_VALUE;\n    const score = isEqual ? 1.0 : 0.0;\n    const reason = `字段'turn.actual_output.text'的值为'${current}'，与目标值'${TARGET_VALUE}'${isEqual ? '相等' : '不相等'}`;\n\n    return { score, reason };\n  } catch (e) {\n    if (e instanceof TypeError || e instanceof ReferenceError) {\n      return { score: 0.0, reason: `字段路径不存在：${e.message}` };\n    }\n    return { score: 0.0, reason: `检查出错：${e.message}` };\n  }\n}\n";

export const defaultTestData = [
  {
    evaluate_dataset_fields: {
      input: { content_type: 'Text', text: '台湾省面积是多少？' },
      reference_output: {
        content_type: 'Text',
        text: '台湾省由中国第一大岛台湾岛与兰屿、绿岛、钓鱼岛等附属岛屿和澎湖列岛等80多个岛屿组成，总面积约3.6万平方千米。其中台湾岛面积约3.58万平方千米。 ',
      },
    },
    evaluate_target_output_fields: {
      actual_output: {
        content_type: 'Text',
        text: '台湾省由中国第一大岛台湾岛与兰屿、绿岛、钓鱼岛等附属岛屿和澎湖列岛等80多个岛屿组成，总面积约3.6万平方千米。其中台湾岛面积约3.58万平方千米。 ',
      },
    },
    ext: {},
  },
];

export const MAX_SELECT_COUNT = 10;
