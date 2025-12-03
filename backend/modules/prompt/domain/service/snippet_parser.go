// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SnippetReference represents a parsed snippet reference
type SnippetReference struct {
	PromptID      int64
	CommitVersion string
}

// SnippetParser defines the interface for parsing snippet references
// Supports extending to other formats in the future
type SnippetParser interface {
	// ParseReferences Parses content and returns snippet references
	ParseReferences(content string) ([]*SnippetReference, error)
	// SerializeReference Serializes a snippet reference back to string
	SerializeReference(ref *SnippetReference) string
}

// CozeLoopSnippetParser implements only cozeloop format parsing
type CozeLoopSnippetParser struct {
	referencePattern *regexp.Regexp
}

// NewCozeLoopSnippetParser creates a new parser for cozeloop format
func NewCozeLoopSnippetParser() SnippetParser {
	// Pattern matches: <cozeloop_snippet>id=123&version=v1</cozeloop_snippet>
	pattern := regexp.MustCompile(`<cozeloop_snippet>id=(\d+)&version=([^&]*)?</cozeloop_snippet>`)
	return &CozeLoopSnippetParser{
		referencePattern: pattern,
	}
}

// ParseReferences parses cozeloop snippet references from content
func (p *CozeLoopSnippetParser) ParseReferences(content string) ([]*SnippetReference, error) {
	if content == "" {
		return nil, nil
	}

	matches := p.referencePattern.FindAllStringSubmatch(content, -1)
	var refs []*SnippetReference

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		promptID, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid prompt ID in reference: %s", match[1])
		}

		version := ""
		if len(match) > 2 {
			version = strings.TrimSpace(match[2])
		}

		refs = append(refs, &SnippetReference{
			PromptID:      promptID,
			CommitVersion: version,
		})
	}

	return refs, nil
}

// SerializeReference serializes a snippet reference back to cozeloop format
func (p *CozeLoopSnippetParser) SerializeReference(ref *SnippetReference) string {
	return fmt.Sprintf("<cozeloop_snippet>id=%d&version=%s</cozeloop_snippet>", ref.PromptID, ref.CommitVersion)
}
