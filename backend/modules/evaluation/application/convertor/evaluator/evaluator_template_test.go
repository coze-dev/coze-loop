// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
    "testing"

    "github.com/bytedance/gg/gptr"
    "github.com/stretchr/testify/assert"

    commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
    evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
    evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertEvaluatorTemplateDTO2DO_Nil(t *testing.T) {
    t.Parallel()
    assert.Nil(t, ConvertEvaluatorTemplateDTO2DO(nil))
}

func TestConvertEvaluatorTemplateDTO2DO_BasicAndTagsAndInfo(t *testing.T) {
    t.Parallel()
    dto := &evaluatordto.EvaluatorTemplate{
        ID:            gptr.Of(int64(123)),
        WorkspaceID:   gptr.Of(int64(456)),
        Name:          gptr.Of("name"),
        Description:   gptr.Of("desc"),
        EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
        Popularity:    gptr.Of(int64(9)),
        BaseInfo:      &commondto.BaseInfo{CreatedBy: &commondto.UserInfo{UserID: gptr.Of("u1")}},
        EvaluatorContent: &evaluatordto.EvaluatorContent{
            ReceiveChatHistory: gptr.Of(true),
            InputSchemas: []*commondto.ArgsSchema{{Key: gptr.Of("in")}},
            OutputSchemas: []*commondto.ArgsSchema{{Key: gptr.Of("out")}},
            PromptEvaluator: &evaluatordto.PromptEvaluator{MessageList: []*commondto.Message{{Content: &commondto.Content{Text: gptr.Of("t")}}}},
        },
        Tags: map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string{
            evaluatordto.EvaluatorTagLangType("zh"): {
                evaluatordto.EvaluatorTagKeyName: {"n1", "n2"},
            },
        },
    }
    do := ConvertEvaluatorTemplateDTO2DO(dto)
    assert.NotNil(t, do)
    assert.Equal(t, int64(123), do.ID)
    assert.Equal(t, int64(456), do.SpaceID)
    assert.Equal(t, "name", do.Name)
    assert.Equal(t, "desc", do.Description)
    assert.Equal(t, evaluatordo.EvaluatorTypePrompt, do.EvaluatorType)
    assert.Equal(t, int64(9), do.Popularity)
    // EvaluatorInfo 字段在模板DTO可能不存在，忽略该字段校验
    assert.NotNil(t, do.BaseInfo)
    assert.True(t, gptr.Indirect(do.ReceiveChatHistory))
    assert.Len(t, do.InputSchemas, 1)
    assert.Len(t, do.OutputSchemas, 1)
    if assert.NotNil(t, do.PromptEvaluatorContent) {
        assert.Len(t, do.PromptEvaluatorContent.MessageList, 1)
    }
    if assert.NotNil(t, do.Tags) {
        assert.Equal(t, []string{"n1", "n2"}, do.Tags[evaluatordo.EvaluatorTagLangType("zh")][evaluatordo.EvaluatorTagKey("Name")])
    }
}

func TestConvertEvaluatorTemplateDTO2DO_CodeEval_NewAndCompat(t *testing.T) {
    t.Parallel()
    // 新字段：lang_2_code_content
    dtoNew := &evaluatordto.EvaluatorTemplate{
        EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Code),
        EvaluatorContent: &evaluatordto.EvaluatorContent{
            CodeEvaluator: &evaluatordto.CodeEvaluator{},
        },
    }
    dtoNew.EvaluatorContent.CodeEvaluator.SetLang2CodeContent(map[evaluatordto.LanguageType]string{
        evaluatordto.LanguageTypePython: "print('hi')",
    })
    doNew := ConvertEvaluatorTemplateDTO2DO(dtoNew)
    if assert.NotNil(t, doNew) && assert.NotNil(t, doNew.CodeEvaluatorContent) {
        assert.Equal(t, "print('hi')", doNew.CodeEvaluatorContent.Lang2CodeContent[evaluatordo.LanguageTypePython])
    }

    // 兼容旧字段：language_type + code_content
    dtoOld := &evaluatordto.EvaluatorTemplate{
        EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Code),
        EvaluatorContent: &evaluatordto.EvaluatorContent{
            CodeEvaluator: &evaluatordto.CodeEvaluator{LanguageType: gptr.Of(evaluatordto.LanguageTypePython), CodeContent: gptr.Of("print('ok')")},
        },
    }
    doOld := ConvertEvaluatorTemplateDTO2DO(dtoOld)
    if assert.NotNil(t, doOld) && assert.NotNil(t, doOld.CodeEvaluatorContent) {
        assert.Equal(t, "print('ok')", doOld.CodeEvaluatorContent.Lang2CodeContent[evaluatordo.LanguageTypePython])
    }
}

func TestConvertEvaluatorTemplateDO2DTO_Nil(t *testing.T) {
    t.Parallel()
    assert.Nil(t, ConvertEvaluatorTemplateDO2DTO(nil))
}

func TestConvertEvaluatorTemplateDO2DTO_Full(t *testing.T) {
    t.Parallel()
    do := &evaluatordo.EvaluatorTemplate{
        ID:            1,
        SpaceID:       2,
        Name:          "n",
        Description:   "d",
        EvaluatorType: evaluatordo.EvaluatorTypePrompt,
        Popularity:    3,
        BaseInfo:      &evaluatordo.BaseInfo{},
        InputSchemas:  []*evaluatordo.ArgsSchema{{Key: gptr.Of("in")}},
        OutputSchemas: []*evaluatordo.ArgsSchema{{Key: gptr.Of("out")}},
        ReceiveChatHistory: gptr.Of(true),
        PromptEvaluatorContent: &evaluatordo.PromptEvaluatorContent{MessageList: []*evaluatordo.Message{{Content: &evaluatordo.Content{Text: gptr.Of("t")}}}},
        Tags: map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string{
            evaluatordo.EvaluatorTagLangType("en"): {evaluatordo.EvaluatorTagKey("Name"): {"x"}},
        },
    }
    dto := ConvertEvaluatorTemplateDO2DTO(do)
    if assert.NotNil(t, dto) {
        assert.Equal(t, int64(1), dto.GetID())
        assert.Equal(t, int64(2), dto.GetWorkspaceID())
        assert.Equal(t, "n", dto.GetName())
        assert.Equal(t, "d", dto.GetDescription())
        assert.Equal(t, evaluatordto.EvaluatorType_Prompt, dto.GetEvaluatorType())
        assert.Equal(t, int64(3), dto.GetPopularity())
        // EvaluatorInfo 字段在模板DTO可能不存在，忽略该字段校验
        if assert.NotNil(t, dto.EvaluatorContent) {
            assert.True(t, gptr.Indirect(dto.EvaluatorContent.ReceiveChatHistory))
            assert.Len(t, dto.EvaluatorContent.InputSchemas, 1)
            assert.Len(t, dto.EvaluatorContent.OutputSchemas, 1)
            if assert.NotNil(t, dto.EvaluatorContent.PromptEvaluator) {
                assert.Len(t, dto.EvaluatorContent.PromptEvaluator.MessageList, 1)
            }
        }
        assert.Equal(t, []string{"x"}, dto.Tags[evaluatordto.EvaluatorTagLangType("en")][evaluatordto.EvaluatorTagKey("Name")])
    }
}

func TestConvertEvaluatorTemplateDOList2DTO(t *testing.T) {
    t.Parallel()
    doList := []*evaluatordo.EvaluatorTemplate{{Name: "a"}, {Name: "b"}}
    dtoList := ConvertEvaluatorTemplateDOList2DTO(doList)
    assert.Len(t, dtoList, 2)
    assert.Equal(t, "a", dtoList[0].GetName())
    assert.Equal(t, "b", dtoList[1].GetName())
}

func TestCodeEvaluatorContentDTOAndDO(t *testing.T) {
    t.Parallel()
    // DTO2DO 新字段
    dto := &evaluatordto.CodeEvaluator{}
    dto.SetLang2CodeContent(map[evaluatordto.LanguageType]string{
        evaluatordto.LanguageTypePython: "print(1)",
    })
    do := ConvertCodeEvaluatorContentDTO2DO(dto)
    if assert.NotNil(t, do) {
        assert.Equal(t, "print(1)", do.Lang2CodeContent[evaluatordo.LanguageTypePython])
    }

    // DTO2DO 旧字段
    dto2 := &evaluatordto.CodeEvaluator{LanguageType: gptr.Of(evaluatordto.LanguageTypeJS), CodeContent: gptr.Of("console.log(1)")}
    do2 := ConvertCodeEvaluatorContentDTO2DO(dto2)
    if assert.NotNil(t, do2) {
        assert.Equal(t, "console.log(1)", do2.Lang2CodeContent[evaluatordo.LanguageTypeJS])
    }

    // DO2DTO
    back := ConvertCodeEvaluatorContentDO2DTO(&evaluatordo.CodeEvaluatorContent{Lang2CodeContent: map[evaluatordo.LanguageType]string{
        evaluatordo.LanguageTypePython: "print(2)",
    }})
    if assert.NotNil(t, back) {
        m := back.GetLang2CodeContent()
        assert.Equal(t, "print(2)", m[evaluatordto.LanguageTypePython])
        // 回填旧字段
        assert.NotNil(t, back.LanguageType)
        assert.NotNil(t, back.CodeContent)
    }
}


