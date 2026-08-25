// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
)

// Create-time validation of a CustomAgent's declared custom output fields.

const (
	customFieldNameMaxLen      = 50
	customFieldSchemasMaxCount = 20
)

var (
	// A field name becomes an OutputFields key resolved via JSONPath downstream,
	// so path separators (.[]$) are excluded.
	customFieldNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

	// Scalar whitelist, checked only when SchemaKey is explicitly set (a scalar
	// normally carries its type in text_schema and leaves SchemaKey unset).
	allowedCustomFieldScalarSchemaKeys = map[SchemaKey]struct{}{
		SchemaKey_String:  {},
		SchemaKey_Integer: {},
		SchemaKey_Float:   {},
		SchemaKey_Bool:    {},
	}

	// Allowed text_schema top-level "type" (float maps to "number").
	allowedCustomFieldJSONSchemaTypes = map[string]struct{}{
		"string":  {},
		"integer": {},
		"number":  {},
		"boolean": {},
	}
)

// ValidateCustomFieldSchemas rejects any invalid declaration (it never falls
// back, dedups, or fills a default type). An empty declaration is allowed; each
// field must have a unique, non-empty name matching customFieldNameRegexp and
// at most customFieldNameMaxLen runes, a resolvable type within the supported
// set (4 scalar types + multipart), and there may be at most
// customFieldSchemasMaxCount fields. Errors use CommonInvalidParamCode and
// include the offending field name.
func ValidateCustomFieldSchemas(schemas []*CustomFieldSchema) error {
	if len(schemas) == 0 {
		return nil
	}
	if len(schemas) > customFieldSchemasMaxCount {
		return invalidParam(fmt.Sprintf("at most %d custom output fields allowed, got %d", customFieldSchemasMaxCount, len(schemas)))
	}

	seen := make(map[string]struct{}, len(schemas))
	for i, s := range schemas {
		if s == nil {
			return invalidParam(fmt.Sprintf("custom_field_schemas[%d] is nil", i))
		}

		if strings.TrimSpace(s.Name) == "" {
			return invalidParam(fmt.Sprintf("custom_field_schemas[%d]: field name must not be empty", i))
		}
		if strings.ContainsFunc(s.Name, unicode.IsSpace) {
			return invalidParam(fmt.Sprintf("field name %q must not contain whitespace", s.Name))
		}
		if !customFieldNameRegexp.MatchString(s.Name) {
			return invalidParam(fmt.Sprintf("field name %q is invalid: only letters, digits and underscore are allowed, and it must start with a letter", s.Name))
		}
		// Count by rune, not byte.
		if utf8.RuneCountInString(s.Name) > customFieldNameMaxLen {
			return invalidParam(fmt.Sprintf("field name %q exceeds the length limit (at most %d characters)", s.Name, customFieldNameMaxLen))
		}
		if _, ok := seen[s.Name]; ok {
			return invalidParam(fmt.Sprintf("field name %q is declared more than once", s.Name))
		}
		seen[s.Name] = struct{}{}

		if err := validateCustomFieldSchemaType(s); err != nil {
			return err
		}
	}

	return nil
}

// validateCustomFieldSchemaType resolves the type: multipart by ContentType;
// else a whitelisted SchemaKey if set; else the text_schema top-level type.
// A non-empty text_schema is validated in every branch because the consumer
// overwrites JsonSchema with it regardless of branch, so object/array must not
// be smuggled in via the multipart or SchemaKey branches.
func validateCustomFieldSchemaType(s *CustomFieldSchema) error {
	// The preset trajectory row arrives on the wire as
	// name=trajectory + SchemaKey=Trajectory(7); downstream output-schema
	// building skips it by name (the report side appends trajectory itself),
	// so the validator must accept it instead of rejecting it as an
	// unsupported scalar type. Any other name declaring Trajectory is still
	// rejected: it would be silently dropped downstream.
	if s.Name == common.ArgSchemaKeyTrajectory &&
		s.SchemaKey != nil &&
		*s.SchemaKey == SchemaKey_Trajectory {
		return nil
	}

	if s.ContentType == ContentTypeMultipart {
		if s.SchemaKey != nil {
			if _, ok := allowedCustomFieldScalarSchemaKeys[*s.SchemaKey]; !ok {
				return unsupportedCustomFieldType(s.Name, *s.SchemaKey)
			}
		}
		return validateCustomFieldTextSchemaIfPresent(s)
	}

	if s.SchemaKey != nil {
		if _, ok := allowedCustomFieldScalarSchemaKeys[*s.SchemaKey]; !ok {
			return unsupportedCustomFieldType(s.Name, *s.SchemaKey)
		}
		return validateCustomFieldTextSchemaIfPresent(s)
	}

	return validateCustomFieldTextSchema(s)
}

func validateCustomFieldTextSchemaIfPresent(s *CustomFieldSchema) error {
	if strings.TrimSpace(s.TextSchema) == "" {
		return nil
	}
	return validateCustomFieldTextSchema(s)
}

// validateCustomFieldTextSchema checks the scalar type carrier: a missing,
// invalid, or non-whitelisted top-level "type" is rejected. Only the top-level
// type is parsed; this is an admission check, not a full JSON Schema audit.
func validateCustomFieldTextSchema(s *CustomFieldSchema) error {
	if strings.TrimSpace(s.TextSchema) == "" {
		return invalidParam(fmt.Sprintf("custom output field %q is missing its type", s.Name))
	}

	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(s.TextSchema), &probe); err != nil {
		return invalidParam(fmt.Sprintf("custom output field %q has a type definition that is not valid JSON Schema", s.Name))
	}
	if probe.Type == "" {
		return invalidParam(fmt.Sprintf("custom output field %q is missing its type", s.Name))
	}
	if _, ok := allowedCustomFieldJSONSchemaTypes[probe.Type]; !ok {
		return invalidParam(fmt.Sprintf("custom output field %q has unsupported type %s; supported types are text/integer/float/boolean/multipart", s.Name, probe.Type))
	}
	return nil
}

func unsupportedCustomFieldType(name string, key SchemaKey) error {
	return invalidParam(fmt.Sprintf("custom output field %q has unsupported type %s; supported types are text/integer/float/boolean/multipart", name, key.String()))
}
