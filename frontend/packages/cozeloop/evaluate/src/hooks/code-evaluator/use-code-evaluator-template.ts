import { useMemo } from 'react';

import { useRequest } from 'ahooks';
import {
  LanguageType,
  TemplateType,
  type EvaluatorContent,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

interface CodeTemplateData {
  jsTemplates: EvaluatorContent[];
  pythonTemplates: EvaluatorContent[];
}

/**
 * 代码评估器模板Hook
 * - 初始化时获取代码评估器模板，并缓存JS和Python两种语言的模板数据
 * - 后续调用时使用缓存数据，不会重新请求
 * - 提供refetch方法供用户手动刷新数据
 */
const useCodeEvaluatorTemplate = () => {
  const { data, loading, error, refresh } = useRequest(
    async () => {
      const res = await StoneEvaluationApi.ListTemplates({
        builtin_template_type: TemplateType.Code,
      });

      return res.builtin_template_keys || [];
    },
    {
      cacheKey: 'code-evaluator-templates', // 使用cacheKey实现跨组件数据共享
      staleTime: 60 * 60 * 1000, // 缓存1小时
      retryCount: 2, // 请求失败时重试2次
    },
  );

  // 处理并分类模板数据
  const processedData = useMemo(() => {
    const result: CodeTemplateData = {
      jsTemplates: [],
      pythonTemplates: [],
    };

    if (data && Array.isArray(data)) {
      // 分类JS和Python模板
      data.forEach(template => {
        if (template.code_evaluator) {
          // 假设template中有language_type字段，1表示JS，2表示Python
          if (template.code_evaluator.language_type === LanguageType.JS) {
            result.jsTemplates.push(template);
          } else if (
            template.code_evaluator.language_type === LanguageType.Python
          ) {
            result.pythonTemplates.push(template);
          }
        }
      });
    }

    return result;
  }, [data]);

  return {
    loading,
    error,
    refresh, // 重命名refresh为refetch以符合需求
    jsTemplates: processedData.jsTemplates,
    pythonTemplates: processedData.pythonTemplates,
    allTemplates: data || [],
  };
};

export default useCodeEvaluatorTemplate;
