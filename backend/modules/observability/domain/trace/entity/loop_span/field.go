// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"fmt"
	"reflect"

	"github.com/fatih/structs"
)

const TagNameJson = "json"

type Field struct {
	f *structs.Field
}

func (p *Field) Name() string {
	return p.f.Name()
}

func (p *Field) Kind() reflect.Kind {
	return p.f.Kind()
}

func (p *Field) TagJson() (alias string, err error) {
	alias = p.f.Tag(TagNameJson)
	if len(alias) <= 0 {
		return "", fmt.Errorf("%s json tag is not set", p.Name())
	}
	return alias, err
}

func (p *Field) Set(val any) error {
	if p.Kind() == reflect.Ptr {
		ptr := reflect.New(reflect.TypeOf(val))
		ptr.Elem().Set(reflect.ValueOf(val))
		val = ptr.Interface()
	}
	err := p.f.Set(val)
	if err != nil {
		return err
	}
	return nil
}

func (p *Field) ValueType() (TagValueType, error) {
	var valueType TagValueType
	fieldKind := p.f.Kind()
	if fieldKind == reflect.Ptr {
		fieldValue := reflect.ValueOf(p.f.Value())
		fieldKind = fieldValue.Type().Elem().Kind()
	}
	switch fieldKind {
	case reflect.Int64:
		valueType = TagValueTypeInt64
	case reflect.String:
		valueType = TagValueTypeString
	case reflect.Bool:
		valueType = TagValueTypeBool
	case reflect.Float64:
		valueType = TagValueTypeFloat64
	default:
		return TagValueTypeUnknown, fmt.Errorf("unsupported value type: %v", p.f.Kind())
	}
	return valueType, nil
}

type Struct struct {
	s *structs.Struct
}

func NewStruct(s any) *Struct {
	return &Struct{
		s: structs.New(s),
	}
}

func (s *Struct) Fields() []*Field {
	src := s.s.Fields()
	dst := make([]*Field, 0, len(src))
	for _, v := range src {
		dst = append(dst, &Field{f: v})
	}
	return dst
}
