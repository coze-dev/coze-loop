// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
)

// scalarField builds a valid scalar declaration (String by default): the wire
// form is content_type=text with the type in text_schema and schema_key unset.
func scalarField(name string) *CustomFieldSchema {
	return &CustomFieldSchema{
		Name:        name,
		ContentType: ContentTypeText,
		TextSchema:  `{"type":"string"}`,
	}
}

// Every invalid declaration is rejected, and the error message contains the
// offending field name.
func TestValidateCustomFieldSchemas_Invalid(t *testing.T) {
	name51 := strings.Repeat("a", 51)

	cases := []struct {
		name    string
		schemas []*CustomFieldSchema
		// wantMsgContains is a substring the error message must contain
		// (usually the offending field name).
		wantMsgContains string
	}{
		// --- nil element fails loud ---
		{
			name:            "nil element rejected",
			schemas:         []*CustomFieldSchema{nil},
			wantMsgContains: "custom_field_schemas[0]",
		},
		// --- name must be non-empty ---
		{
			name:            "empty name rejected",
			schemas:         []*CustomFieldSchema{scalarField("")},
			wantMsgContains: "must not be empty",
		},
		{
			name:            "blank name rejected",
			schemas:         []*CustomFieldSchema{scalarField("   ")},
			wantMsgContains: "must not be empty",
		},
		// --- no unicode whitespace ---
		{
			name:            "name with space rejected",
			schemas:         []*CustomFieldSchema{scalarField("my field")},
			wantMsgContains: "my field",
		},
		{
			name:            "name with full-width space U+3000 rejected",
			schemas:         []*CustomFieldSchema{scalarField("my　field")},
			wantMsgContains: "whitespace",
		},
		{
			name:            "name with tab rejected",
			schemas:         []*CustomFieldSchema{scalarField("my\tfield")},
			wantMsgContains: "whitespace",
		},
		// --- charset and first char ---
		{
			name:            "name with dot rejected (JSONPath separator)",
			schemas:         []*CustomFieldSchema{scalarField("my.field")},
			wantMsgContains: "my.field",
		},
		{
			name:            "name with left bracket rejected (JSONPath separator)",
			schemas:         []*CustomFieldSchema{scalarField("my[0]")},
			wantMsgContains: "my[0]",
		},
		{
			name:            "name starting with digit rejected",
			schemas:         []*CustomFieldSchema{scalarField("1abc")},
			wantMsgContains: "1abc",
		},
		{
			name:            "name starting with underscore rejected",
			schemas:         []*CustomFieldSchema{scalarField("_field")},
			wantMsgContains: "_field",
		},
		{
			name:            "non-ASCII name rejected",
			schemas:         []*CustomFieldSchema{scalarField("输出内容")},
			wantMsgContains: "输出内容",
		},
		{
			name:            "name with hyphen rejected",
			schemas:         []*CustomFieldSchema{scalarField("my-field")},
			wantMsgContains: "my-field",
		},
		// --- length limit 50 ---
		{
			name:            "51-char name rejected (limit 50)",
			schemas:         []*CustomFieldSchema{scalarField(name51)},
			wantMsgContains: name51,
		},
		// --- duplicate names within the batch ---
		{
			name:            "duplicate name in batch rejected",
			schemas:         []*CustomFieldSchema{scalarField("dup"), scalarField("dup")},
			wantMsgContains: "more than once",
		},
		// --- type must be resolvable ---
		{
			name: "missing text_schema on non-multipart rejected (scalar has no type carrier)",
			schemas: []*CustomFieldSchema{
				{Name: "no_type", ContentType: ContentTypeText},
			},
			wantMsgContains: "no_type",
		},
		{
			name: "empty ContentType, SchemaKey and text_schema rejected",
			schemas: []*CustomFieldSchema{
				{Name: "obj_field"},
			},
			wantMsgContains: "obj_field",
		},
		{
			name: "invalid JSON text_schema rejected",
			schemas: []*CustomFieldSchema{
				{Name: "bad_json", ContentType: ContentTypeText, TextSchema: `{"type":`},
			},
			wantMsgContains: "bad_json",
		},
		{
			name: "text_schema without top-level type rejected",
			schemas: []*CustomFieldSchema{
				{Name: "no_top_type", ContentType: ContentTypeText, TextSchema: `{"description":"x"}`},
			},
			wantMsgContains: "no_top_type",
		},
		// --- object / array not allowed (resolved by text_schema top-level type) ---
		{
			name: "text_schema type=object rejected (object not allowed)",
			schemas: []*CustomFieldSchema{
				{Name: "obj_ts", ContentType: ContentTypeText, TextSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`},
			},
			wantMsgContains: "obj_ts",
		},
		{
			name: "text_schema type=array rejected (array not allowed)",
			schemas: []*CustomFieldSchema{
				{Name: "arr_ts", ContentType: ContentTypeText, TextSchema: `{"type":"array","items":{"type":"string"}}`},
			},
			wantMsgContains: "arr_ts",
		},
		// --- SchemaKey outside the scalar whitelist (checked when explicitly set) ---
		{
			name: "SchemaKey=Message(5) rejected",
			schemas: []*CustomFieldSchema{
				{Name: "msg_field", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_Message)},
			},
			wantMsgContains: "msg_field",
		},
		{
			name: "SchemaKey=SingleChoice(6) rejected",
			schemas: []*CustomFieldSchema{
				{Name: "choice_field", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_SingleChoice)},
			},
			wantMsgContains: "choice_field",
		},
		{
			// A Trajectory declaration would be silently dropped downstream.
			name: "Trajectory(7) rejected (would otherwise be silently dropped)",
			schemas: []*CustomFieldSchema{
				{Name: "traj_field", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_Trajectory)},
			},
			wantMsgContains: "traj_field",
		},
		{
			name: "Trajectory(7) cannot bypass the whitelist via the multipart branch",
			schemas: []*CustomFieldSchema{
				{Name: "traj_mp", ContentType: ContentTypeMultipart, SchemaKey: gptr.Of(SchemaKey_Trajectory)},
			},
			wantMsgContains: "traj_mp",
		},
		{
			name: "SchemaKey=MessageList(8) rejected",
			schemas: []*CustomFieldSchema{
				{Name: "msg_list", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_MessageList)},
			},
			wantMsgContains: "msg_list",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := ValidateCustomFieldSchemas(c.schemas)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsgContains, "error message must contain the offending field name")
		})
	}
}

// Empty declarations and valid cases (incl. multipart and exactly 50 chars) pass.
func TestValidateCustomFieldSchemas_Valid(t *testing.T) {
	name50 := strings.Repeat("a", 50)

	cases := []struct {
		name    string
		schemas []*CustomFieldSchema
	}{
		// --- empty declaration allowed ---
		{name: "nil allowed", schemas: nil},
		{name: "empty slice allowed", schemas: []*CustomFieldSchema{}},
		// --- valid field names ---
		{name: "valid name abc", schemas: []*CustomFieldSchema{scalarField("abc")}},
		{name: "valid name a1", schemas: []*CustomFieldSchema{scalarField("a1")}},
		{name: "valid name my_field", schemas: []*CustomFieldSchema{scalarField("my_field")}},
		{name: "valid name uppercase start Abc", schemas: []*CustomFieldSchema{scalarField("Abc")}},
		{name: "exactly 50 chars allowed (boundary)", schemas: []*CustomFieldSchema{scalarField(name50)}},
		// --- system field names are not reserved ---
		{
			// actual_output is a stable read-only row and must be allowed.
			name:    "actual_output allowed (stable read-only row from the form)",
			schemas: []*CustomFieldSchema{scalarField(consts.EvalTargetOutputFieldKeyActualOutput)},
		},
		{
			// The preset trajectory row arrives as name=trajectory + SchemaKey=Trajectory(7);
			// downstream skips it by name, so the validator accepts this exact shape.
			name: "preset trajectory row allowed (name=trajectory, SchemaKey=Trajectory)",
			schemas: []*CustomFieldSchema{
				{
					Name:        consts.EvalTargetOutputFieldKeyTrajectory,
					ContentType: ContentTypeText,
					SchemaKey:   gptr.Of(SchemaKey_Trajectory),
				},
			},
		},
		{
			// trajectory is allowed as a name; the Trajectory type is what is rejected.
			name:    "trajectory field name allowed (the Trajectory type is what is rejected, not this name)",
			schemas: []*CustomFieldSchema{scalarField(consts.EvalTargetOutputFieldKeyTrajectory)},
		},
		{
			name: "distinct names in batch allowed",
			schemas: []*CustomFieldSchema{
				scalarField("field_a"),
				scalarField("field_b"),
			},
		},
		// --- all 4 scalar types allowed (real wire: schema_key nil, type in text_schema) ---
		{
			name: "4 scalar types String/Integer/Float/Bool all allowed (schema_key=nil)",
			schemas: []*CustomFieldSchema{
				{Name: "f_string", ContentType: ContentTypeText, TextSchema: `{"type":"string"}`},
				{Name: "f_integer", ContentType: ContentTypeText, TextSchema: `{"type":"integer"}`},
				// Float maps to JSON Schema "number", not "float".
				{Name: "f_float", ContentType: ContentTypeText, TextSchema: `{"type":"number"}`},
				{Name: "f_bool", ContentType: ContentTypeText, TextSchema: `{"type":"boolean"}`},
			},
		},
		// --- multipart allowed (real wire: schema_key=nil and no text_schema) ---
		{
			name: "multipart with nil SchemaKey and no text_schema allowed",
			schemas: []*CustomFieldSchema{
				{Name: "f_multipart", ContentType: ContentTypeMultipart},
			},
		},
		{
			name: "multipart with SchemaKey in scalar whitelist allowed",
			schemas: []*CustomFieldSchema{
				{Name: "f_multipart_str", ContentType: ContentTypeMultipart, SchemaKey: gptr.Of(SchemaKey_String)},
			},
		},
		{
			name: "mixed scalar + multipart declaration allowed",
			schemas: []*CustomFieldSchema{
				{Name: "f_scalar", ContentType: ContentTypeText, TextSchema: `{"type":"integer"}`},
				{Name: "f_mp", ContentType: ContentTypeMultipart},
			},
		},
		// --- API-client compat: explicit whitelisted SchemaKey without text_schema is allowed ---
		{
			name: "explicit SchemaKey=String without text_schema allowed (API client)",
			schemas: []*CustomFieldSchema{
				{Name: "f_key_only", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_String)},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.NoError(t, ValidateCustomFieldSchemas(c.schemas))
		})
	}
}

// Regression guard for the real wire form: scalars are content_type=text with
// the type in text_schema and schema_key unset; multipart is content_type=
// multipart. Treating schema_key as required would reject every real scalar.
func TestValidateCustomFieldSchemas_FrontendWireFormat(t *testing.T) {
	t.Run("content_type=text + schema_key=nil + valid text_schema must pass", func(t *testing.T) {
		schemas := []*CustomFieldSchema{
			{Name: "score", ContentType: ContentTypeText, TextSchema: `{"type":"integer"}`},
			{Name: "ratio", ContentType: ContentTypeText, TextSchema: `{"type":"number"}`},
			{Name: "passed", ContentType: ContentTypeText, TextSchema: `{"type":"boolean"}`},
			{Name: "summary", ContentType: ContentTypeText, TextSchema: `{"type":"string"}`},
		}
		assert.NoError(t, ValidateCustomFieldSchemas(schemas),
			"scalar schema_key is always unset; treating it as required rejects real scalar fields")
	})

	t.Run("content_type=multi_part + schema_key=nil must be recognized as multipart", func(t *testing.T) {
		s := &CustomFieldSchema{Name: "screenshot", ContentType: ContentTypeMultipart}
		// If recognized as multipart, an empty text_schema is allowed;
		// if misresolved as scalar it would be rejected for a missing type.
		assert.NoError(t, ValidateCustomFieldSchemas([]*CustomFieldSchema{s}),
			"multipart must be resolved by ContentType; a raw unmapped \"multi_part\" value would fail here")
		assert.Equal(t, ContentTypeMultipart, s.ContentType,
			"the multipart constant here is \"MultiPart\" and must have been mapped from the raw \"multi_part\" value")
	})

	t.Run("an unmapped raw literal is treated as scalar and rejected for missing text_schema", func(t *testing.T) {
		// A raw "multi_part" value != ContentTypeMultipart ("MultiPart"), so it
		// falls into the scalar branch and is rejected for a missing type.
		err := ValidateCustomFieldSchemas([]*CustomFieldSchema{
			{Name: "screenshot", ContentType: ContentType("multi_part")},
		})
		assert.Error(t, err, "a raw literal must not be treated as multipart; this is why the value has to be mapped first")
	})
}

// scalarFields builds n distinctly-named valid scalars, avoiding the duplicate
// and length rules so cases exercise the count/type rules instead.
func scalarFields(n int) []*CustomFieldSchema {
	schemas := make([]*CustomFieldSchema, 0, n)
	for i := 0; i < n; i++ {
		schemas = append(schemas, scalarField(fmt.Sprintf("field_%d", i)))
	}
	return schemas
}

// Boundary of the declaration-count limit. The literals 20/21 are written out
// rather than referencing customFieldSchemasMaxCount, whose value is itself
// under test.
func TestValidateCustomFieldSchemas_MaxCount(t *testing.T) {
	t.Run("exactly 20 allowed (inner boundary)", func(t *testing.T) {
		assert.NoError(t, ValidateCustomFieldSchemas(scalarFields(20)),
			"20 is the form's max column count; the backend must not be stricter than the form")
	})

	t.Run("21 rejected and the error carries the counts (outer boundary)", func(t *testing.T) {
		err := ValidateCustomFieldSchemas(scalarFields(21))
		assert.Error(t, err, "more declarations than the form allows can only come from an API client and must be rejected at the entry")
		// The error carries "limit / current" so the user knows how many to remove.
		assert.Contains(t, err.Error(), "20")
		assert.Contains(t, err.Error(), "21")
	})
}

// The two text_schema bypass paths: the consumer overwrites JsonSchema with a
// non-empty text_schema regardless of branch, so object/array must not be
// smuggled in via the multipart or SchemaKey branches. The control cases
// confirm a valid text_schema still passes.
func TestValidateCustomFieldSchemas_TextSchemaBypass(t *testing.T) {
	invalidCases := []struct {
		name    string
		schemas []*CustomFieldSchema
		// wantMsgContains is a substring the error message must contain
		// (usually the offending field name).
		wantMsgContains string
	}{
		// --- bypass path A: smuggle via ContentType=multipart ---
		{
			name: "multipart + text_schema type=object rejected (consumer overwrites JsonSchema)",
			schemas: []*CustomFieldSchema{
				{Name: "mp_obj", ContentType: ContentTypeMultipart, TextSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`},
			},
			wantMsgContains: "mp_obj",
		},
		{
			name: "multipart + text_schema type=array rejected (same smuggling path)",
			schemas: []*CustomFieldSchema{
				{Name: "mp_arr", ContentType: ContentTypeMultipart, TextSchema: `{"type":"array","items":{"type":"string"}}`},
			},
			wantMsgContains: "mp_arr",
		},
		// --- bypass path B: smuggle via a whitelisted explicit SchemaKey ---
		{
			name: "SchemaKey=String + text_schema type=object rejected (SchemaKey is a decoy, overwrite still happens)",
			schemas: []*CustomFieldSchema{
				{Name: "key_obj", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_String), TextSchema: `{"type":"object"}`},
			},
			wantMsgContains: "key_obj",
		},
		{
			name: "SchemaKey=String + text_schema type=array rejected (same smuggling path)",
			schemas: []*CustomFieldSchema{
				{Name: "key_arr", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_String), TextSchema: `{"type":"array","items":{"type":"string"}}`},
			},
			wantMsgContains: "key_arr",
		},
	}

	for _, c := range invalidCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := ValidateCustomFieldSchemas(c.schemas)
			// Gate before reading err.Error(): these cases return nil if the
			// check is loosened, and an ungated call would nil-deref panic.
			if !assert.Error(t, err) {
				return
			}
			assert.Contains(t, err.Error(), c.wantMsgContains, "error message must contain the offending field name")
		})
	}

	validCases := []struct {
		name    string
		schemas []*CustomFieldSchema
	}{
		{
			// Control: a valid overwrite is harmless and allowed.
			name: "multipart + valid text_schema type=string allowed (rejects the illegal overwrite, not the presence of text_schema)",
			schemas: []*CustomFieldSchema{
				{Name: "mp_str", ContentType: ContentTypeMultipart, TextSchema: `{"type":"string"}`},
			},
		},
		{
			// Control: whitelisted SchemaKey + valid text_schema, both in range.
			name: "SchemaKey=String + valid text_schema type=integer allowed (overwrite result still whitelisted)",
			schemas: []*CustomFieldSchema{
				{Name: "key_int", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_String), TextSchema: `{"type":"integer"}`},
			},
		},
	}

	for _, c := range validCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.NoError(t, ValidateCustomFieldSchemas(c.schemas))
		})
	}
}
