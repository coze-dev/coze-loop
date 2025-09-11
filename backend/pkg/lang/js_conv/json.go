// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package js_conv

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

var (
	once   sync.Once
	jsoner jsoniter.API
)

func GetMarshaler() func(v interface{}) ([]byte, error) {
	once.Do(func() {
		initJsonMarshalerWithExtension()
	})
	return jsoner.Marshal
}

func GetUnmarshaler() func(data []byte, v interface{}) error {
	once.Do(func() {
		initJsonMarshalerWithExtension()
	})
	return jsoner.Unmarshal
}

func initJsonMarshalerWithExtension() {
	jsoner = jsoniter.ConfigCompatibleWithStandardLibrary
	jsoner.RegisterExtension(NewJSONIterExtension())
}

type JsonIterExtension struct {
	jsoniter.DummyExtension
}

func NewJSONIterExtension() *JsonIterExtension {
	return &JsonIterExtension{}
}

func (j *JsonIterExtension) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	for _, binding := range structDescriptor.Fields {
		typ := binding.Field.Type()
		if j.isI64Field(typ) {
			c := newI64Codec()
			binding.Decoder = c
			binding.Encoder = c
			continue
		}
		if j.isI64PtrField(typ) {
			c := newI64PtrCodec()
			binding.Decoder = c
			binding.Encoder = c
			continue
		}
		if j.isI64SliceField(typ) {
			c := newI64SliceCodec()
			binding.Decoder = c
			binding.Encoder = c
			continue
		}
		if j.isI64MapField(typ) {
			c := newInt64MapCodec(typ.Type1())
			binding.Encoder = c
			binding.Decoder = c
			continue
		}
	}
}

func (j *JsonIterExtension) isI64MapField(t reflect2.Type) bool {
	if t.Kind() != reflect.Map {
		return false
	}
	typ := t.Type1()
	return typ.Key().Kind() == reflect.Int64 || typ.Elem().Kind() == reflect.Int64
}

func (j *JsonIterExtension) isI64SliceField(t reflect2.Type) bool {
	return t.String() == "[]int64"
}

func (j *JsonIterExtension) isI64Field(t reflect2.Type) bool {
	return t.String() == "int64"
}

func (j *JsonIterExtension) isI64PtrField(t reflect2.Type) bool {
	return t.String() == "*int64"
}

type i64PtrCodec struct{}

func newI64PtrCodec() *i64PtrCodec {
	return &i64PtrCodec{}
}

func (i *i64PtrCodec) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	val := iter.ReadAny()
	switch val.ValueType() {
	case jsoniter.NilValue:
		iter.ReadNil()
		*((**int64)(ptr)) = nil
	case jsoniter.StringValue, jsoniter.NumberValue:
		num := val.ToInt64()
		if val.LastError() != nil {
			iter.ReportError("decodeI64", fmt.Sprintf("parse string to int64 fail, err=%v", val.LastError()))
		} else {
			*((**int64)(ptr)) = &num
		}
	default:
		iter.ReportError("decodeI64", fmt.Sprintf("int64 must be parsed from type string or number, got %v", val.ValueType()))
	}
}

func (i *i64PtrCodec) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	numPtr := *((**int64)(ptr))
	if numPtr == nil {
		stream.WriteNil()
		return
	}
	stream.WriteString(strconv.FormatInt(*numPtr, 10))
}

func (i *i64PtrCodec) IsEmpty(ptr unsafe.Pointer) bool {
	return *((**int64)(ptr)) == nil
}

type i64Codec struct{}

func newI64Codec() *i64Codec {
	return &i64Codec{}
}

func (i *i64Codec) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	val := iter.ReadAny()
	switch val.ValueType() {
	case jsoniter.StringValue, jsoniter.NumberValue:
		num := val.ToInt64()
		if val.LastError() != nil {
			iter.ReportError("decodeI64", fmt.Sprintf("parse string to int64 fail, err=%v", val.LastError()))
		} else {
			*((*int64)(ptr)) = num
		}
	default:
		iter.ReportError("decodeI64", fmt.Sprintf("int64 must be parsed from type string or number, got %v", val.ValueType()))
	}
}

func (i *i64Codec) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	num := *((*int64)(ptr))
	stream.WriteString(strconv.FormatInt(num, 10))
}

func (i *i64Codec) IsEmpty(ptr unsafe.Pointer) bool {
	return *((*int64)(ptr)) == 0
}

func newI64SliceCodec() *i64SliceCodec {
	return &i64SliceCodec{}
}

type i64SliceCodec struct{}

func (i *i64SliceCodec) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	var slice []int64

	iter.ReadArrayCB(func(iter *jsoniter.Iterator) bool {
		elem := iter.ReadAny()
		switch elem.ValueType() {
		case jsoniter.StringValue, jsoniter.NumberValue:
			num := elem.ToInt64()
			if err := elem.LastError(); err != nil {
				iter.ReportError("decodeI64Slice", fmt.Sprintf("parse slice item to int64 fail, err=%v", err))
				return false
			}
			slice = append(slice, num)
		default:
			iter.ReportError("decodeI64Slice", fmt.Sprintf("int64 slice must be parsed from type string or number, but got %v", elem.ValueType()))
			return false
		}
		return true
	})

	if iter.Error != nil {
		return
	}

	*((*[]int64)(ptr)) = slice
}

func (i *i64SliceCodec) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	slice := *((*[]int64)(ptr))
	stream.WriteArrayStart()
	for i, num := range slice {
		if i > 0 {
			stream.WriteMore()
		}
		stream.WriteString(strconv.FormatInt(num, 10))
	}
	stream.WriteArrayEnd()
}

func (i *i64SliceCodec) IsEmpty(ptr unsafe.Pointer) bool {
	return len(*((*[]int64)(ptr))) == 0
}

func newInt64MapCodec(mapType reflect.Type) *i64MapCodec {
	return &i64MapCodec{t: mapType}
}

type i64MapCodec struct {
	t reflect.Type
}

func (i *i64MapCodec) IsEmpty(ptr unsafe.Pointer) bool {
	mapPtr := reflect.NewAt(i.t, ptr).Elem()
	return mapPtr.IsNil() || mapPtr.Len() == 0
}

func (i *i64MapCodec) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	t := i.t
	mapValue := reflect.NewAt(t, ptr).Elem()
	if mapValue.IsNil() {
		stream.WriteNil()
		return
	}

	keyIsInt64 := t.Key().Kind() == reflect.Int64
	elemIsInt64 := t.Elem().Kind() == reflect.Int64

	stream.WriteObjectStart()
	first := true
	iter := mapValue.MapRange()
	for iter.Next() {
		if !first {
			stream.WriteMore()
		}
		first = false

		key := iter.Key()
		if keyIsInt64 {
			stream.WriteString(strconv.FormatInt(key.Int(), 10))
		} else {
			stream.WriteVal(key.Interface())
		}

		stream.WriteRaw(":")

		value := iter.Value()
		if elemIsInt64 {
			stream.WriteString(strconv.FormatInt(value.Int(), 10))
		} else {
			stream.WriteVal(value.Interface())
		}
	}
	stream.WriteObjectEnd()
}

func (i *i64MapCodec) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	t := i.t
	mval := reflect.MakeMap(t)
	keyType := t.Key()
	elemType := t.Elem()
	keyIsInt64 := keyType.Kind() == reflect.Int64
	elemIsInt64 := elemType.Kind() == reflect.Int64

	rawMap := make(map[string]jsoniter.RawMessage)
	iter.ReadVal(&rawMap)
	if iter.Error != nil {
		return
	}

	for skey, rval := range rawMap {
		var key reflect.Value
		if keyIsInt64 {
			int64Key, err := strconv.ParseInt(skey, 10, 64)
			if err != nil {
				iter.ReportError("decode map key", err.Error())
				return
			}
			key = reflect.ValueOf(int64Key).Convert(keyType)
		} else {
			key = reflect.ValueOf(skey).Convert(keyType)
		}

		var value reflect.Value
		if elemIsInt64 {
			var int64Val int64
			if len(rval) > 0 && rval[0] == '"' {
				strVal := string(rval[1 : len(rval)-1])
				intVal, err := strconv.ParseInt(strVal, 10, 64)
				if err != nil {
					iter.ReportError("decode map value", fmt.Sprintf("parse string to int64 fail, raw=%s, err=%s", strVal, err.Error()))
				}
				int64Val = intVal
			} else {
				if err := jsoniter.Unmarshal(rval, &int64Val); err != nil {
					iter.ReportError("decode map value", fmt.Sprintf("parse int64 fail, raw=%s, err=%s", string(rval), err.Error()))
				}
			}
			value = reflect.ValueOf(int64Val).Convert(elemType)
		} else {
			elem := reflect.New(elemType)

			subIter := iter.Pool().BorrowIterator(rval)
			subIter.ReadVal(elem.Interface())
			iter.Pool().ReturnIterator(subIter)

			value = elem.Elem()
		}

		mval.SetMapIndex(key, value)
	}

	oldMap := reflect.NewAt(t, ptr).Elem()
	oldMap.Set(mval)
}
