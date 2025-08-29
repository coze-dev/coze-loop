import { type RuleItem } from '@coze-arch/coze-design';

export const sourceNameRuleValidator: RuleItem['validator'] = (
  rule,
  value,
  callback,
) => {
  // 复合正则表达式验证 [4,7](@ref)
  const pattern = /^[a-zA-Z0-9\u4e00-\u9fa5][\w\u4e00-\u9fa5\-\.]*$/;
  if (!pattern.test(value)) {
    // 错误类型细分 [2,5](@ref)
    const firstChar = value.charAt(0);
    console.log(firstChar);
    if (/^[-_.]/.test(firstChar)) {
      callback('仅支持英文字母、数字、中文开头');
    } else {
      callback('仅支持英文字母、数字、中文，“-”，“_”，“.”');
    }
  }
  return true;
};

export const columnNameRuleValidator: RuleItem['validator'] = (
  rule,
  value,
  callback,
) => {
  if (!/^[a-zA-Z][a-zA-Z0-9_]*$/.test(value)) {
    callback('仅支持英文、数字、下划线，且需要以字母开头');
  }
  return true;
};
