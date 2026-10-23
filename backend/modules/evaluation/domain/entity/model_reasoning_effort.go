package entity

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func (m *ModelConfig) ValidateThinking() error {
	if m == nil || m.Thinking == nil {
		return nil
	}
	t := m.Thinking
	if t.ThinkingOption != nil {
		switch *t.ThinkingOption {
		case "enabled", "disabled", "auto":
		default:
			return fmt.Errorf("thinking_option must be enabled, disabled or auto")
		}
	}
	if t.ThinkingOption != nil && *t.ThinkingOption == "disabled" {
		return nil
	}
	if t.BudgetTokens != nil {
		if *t.BudgetTokens <= 0 {
			return fmt.Errorf("thinking budget_tokens must be positive")
		}
		if t.ReasoningEffort != nil && *t.ReasoningEffort != "" {
			return fmt.Errorf("thinking effort and budget are mutually exclusive")
		}
	}
	return nil
}

func (a *AgentConfig) Validate() error {
	if a == nil {
		return nil
	}
	if a.Runtime != nil && *a.Runtime != "claude_code" && *a.Runtime != "codex_cli" {
		return fmt.Errorf("agent runtime must be claude_code or codex_cli")
	}
	if a.SearchProvider != nil && *a.SearchProvider != "search_api" && *a.SearchProvider != "serp_api" {
		return fmt.Errorf("search_provider must be search_api or serp_api")
	}
	return nil
}

func (e *Evaluator) ValidateVersionSettings() error {
	if e == nil {
		return nil
	}
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.ModelConfig.ValidateThinking()
		}
	case EvaluatorTypeAgent:
		if e.AgentEvaluatorVersion != nil {
			if err := e.AgentEvaluatorVersion.AgentConfig.Validate(); err != nil {
				return err
			}
			return e.AgentEvaluatorVersion.ModelConfig.ValidateThinking()
		}
	}
	return nil
}

// PreserveDraftConfig protects only omitted version configuration during content replacement.
func (e *Evaluator) PreserveDraftConfig(previous *Evaluator) {
	if e == nil || previous == nil || e.EvaluatorType != previous.EvaluatorType {
		return
	}
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion == nil || previous.PromptEvaluatorVersion == nil {
			return
		}
		e.PromptEvaluatorVersion.ModelConfig = preserveModelConfig(e.PromptEvaluatorVersion.ModelConfig, previous.PromptEvaluatorVersion.ModelConfig)
	case EvaluatorTypeAgent:
		if e.AgentEvaluatorVersion == nil || previous.AgentEvaluatorVersion == nil {
			return
		}
		next, old := e.AgentEvaluatorVersion, previous.AgentEvaluatorVersion
		next.ModelConfig = preserveModelConfig(next.ModelConfig, old.ModelConfig)
		if old.AgentConfig == nil {
			return
		}
		if next.AgentConfig == nil {
			next.AgentConfig = &AgentConfig{AgentType: old.AgentConfig.AgentType}
		}
		if next.AgentConfig.Runtime == nil {
			next.AgentConfig.Runtime = copyValue(old.AgentConfig.Runtime)
		}
		if next.AgentConfig.SearchProvider == nil {
			next.AgentConfig.SearchProvider = copyValue(old.AgentConfig.SearchProvider)
		}
	}
}

func preserveModelConfig(next, old *ModelConfig) *ModelConfig {
	if old == nil {
		return next
	}
	if next == nil {
		result := *old
		result.ModelID = copyValue(old.ModelID)
		result.ProviderModelID = copyValue(old.ProviderModelID)
		result.Temperature = copyValue(old.Temperature)
		result.MaxTokens = copyValue(old.MaxTokens)
		result.TopP = copyValue(old.TopP)
		result.Protocol = copyValue(old.Protocol)
		result.Identification = copyValue(old.Identification)
		result.PresetModel = copyValue(old.PresetModel)
		result.JSONExt = copyValue(old.JSONExt)
		result.Thinking = copyThinking(old.Thinking)
		return &result
	}
	sameModelID := next.GetModelID() == old.GetModelID()
	providerOmittedForKnownModel := next.GetModelID() != 0 && next.ProviderModelID == nil
	if !sameModelID || (!providerOmittedForKnownModel && !sameString(next.ProviderModelID, old.ProviderModelID)) {
		return next
	}
	if next.JSONExt == nil {
		next.JSONExt = copyValue(old.JSONExt)
	}
	if next.Thinking == nil {
		next.Thinking = copyThinking(old.Thinking)
		return next
	}
	if old.Thinking != nil {
		if next.Thinking.ThinkingOption == nil {
			next.Thinking.ThinkingOption = copyValue(old.Thinking.ThinkingOption)
		}
		if next.Thinking.ReasoningEffort == nil && next.Thinking.BudgetTokens == nil {
			next.Thinking.ReasoningEffort = copyValue(old.Thinking.ReasoningEffort)
			next.Thinking.BudgetTokens = copyValue(old.Thinking.BudgetTokens)
		}
	}
	return next
}

func copyThinking(t *ThinkingConfig) *ThinkingConfig {
	if t == nil {
		return nil
	}
	return &ThinkingConfig{ThinkingOption: copyValue(t.ThinkingOption), ReasoningEffort: copyValue(t.ReasoningEffort), BudgetTokens: copyValue(t.BudgetTokens)}
}

func copyValue[T any](p *T) *T {
	if p == nil {
		return nil
	}
	value := *p
	return &value
}
func sameString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (m *ModelConfig) ExtraObject() (map[string]any, error) {
	if m == nil || m.JSONExt == nil {
		return nil, nil
	}
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(*m.JSONExt))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("model json_ext must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("model json_ext must be a JSON object")
	}
	return value, nil
}
