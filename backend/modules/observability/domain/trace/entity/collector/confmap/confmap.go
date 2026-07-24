// Original Files: open-telemetry/opentelemetry-collector
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// This file may have been modified by ByteDance Ltd.

package confmap

import (
	"context"
	"encoding"
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

//go:generate mockgen -destination=mocks/provider.go -package=mocks . Provider
type Provider interface {
	// Retrieve goes to the configuration source and retrieves the selected data which
	// contains the value to be injected in the configuration and the corresponding watcher that
	// will be used to monitor for updates of the retrieved value.
	//
	// If ctx is cancelled should return immediately with an error.
	// Should never be called concurrently with itself or with Shutdown.
	Retrieve(ctx context.Context, path string) (*Retrieved, error)

	// Scheme returns the scheme used by Retrieve.
	Scheme() string

	// Shutdown signals that the configuration for which this Provider was used to
	// retrieve values is no longer in use and the Provider should close and release
	// any resources that it may have created.
	//
	// This method must be called when the Collector service ends, either in case of
	// success or error. Retrieve cannot be called after Shutdown.
	//
	// Should never be called concurrently with itself or with Retrieve.
	// If ctx is cancelled should return immediately with an error.
	Shutdown(ctx context.Context) error
}

// Retrieved holds the result of a call to the Retrieve method of a Provider object.
type Retrieved struct {
	rawConf any
}

// NewRetrieved returns a new Retrieved instance that contains the data from the raw deserialized config.
// The rawConf can be one of the following types:
//   - Primitives: int, int32, int64, float32, float64, bool, string;
//   - []any;
//   - map[string]any;
func NewRetrieved(rawConf any) (*Retrieved, error) {
	if err := checkRawConfType(rawConf); err != nil {
		return nil, err
	}
	return &Retrieved{rawConf: rawConf}, nil
}

func (r *Retrieved) RawConf() any {
	return r.rawConf
}

func checkRawConfType(rawConf any) error {
	if rawConf == nil {
		return nil
	}
	switch rawConf.(type) {
	case int, int32, int64, float32, float64, bool, string, []any, map[string]any:
		return nil
	default:
		return fmt.Errorf(
			"unsupported type=%T for retrieved config,"+
				" ensure that values are wrapped in quotes", rawConf)
	}
}

func DecodeConfig(m any, result any) error {
	dc := &mapstructure.DecoderConfig{
		Result:           result,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			expandNilStructPointersHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			mapKeyStringToMapKeyTextUnmarshalerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
			zeroSliceHookFunc(),
		),
	}
	decoder, err := mapstructure.NewDecoder(dc)
	if err != nil {
		return err
	}
	err = decoder.Decode(m)
	if err != nil {
		return err
	}
	return nil
}

func expandNilStructPointersHookFunc() mapstructure.DecodeHookFuncValue {
	return func(from reflect.Value, to reflect.Value) (any, error) {
		// ensure we are dealing with map to map comparison
		if from.Kind() == reflect.Map && to.Kind() == reflect.Map {
			toElem := to.Type().Elem()
			// ensure that map values are pointers to a struct
			// (that may be nil and require manual setting w/ zero value)
			if toElem.Kind() == reflect.Pointer && toElem.Elem().Kind() == reflect.Struct {
				fromRange := from.MapRange()
				for fromRange.Next() {
					fromKey := fromRange.Key()
					fromValue := fromRange.Value()
					// ensure that we've run into a nil pointer instance
					if fromValue.IsNil() {
						newFromValue := reflect.New(toElem.Elem())
						from.SetMapIndex(fromKey, newFromValue)
					}
				}
			}
		}
		return from.Interface(), nil
	}
}

// mapKeyStringToMapKeyTextUnmarshalerHookFunc returns a DecodeHookFuncType that checks that a conversion from
// map[string]any to map[encoding.TextUnmarshaler]any does not overwrite keys,
// when UnmarshalText produces equal elements from different strings (e.g. trims whitespaces).
//
// This is needed in combination with ComponentID, which may produce equal IDs for different strings,
// and an error needs to be returned in that case, otherwise the last equivalent ID overwrites the previous one.
func mapKeyStringToMapKeyTextUnmarshalerHookFunc() mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.Map || f.Key().Kind() != reflect.String {
			return data, nil
		}

		if t.Kind() != reflect.Map {
			return data, nil
		}

		if _, ok := reflect.New(t.Key()).Interface().(encoding.TextUnmarshaler); !ok {
			return data, nil
		}

		m := reflect.MakeMap(reflect.MapOf(t.Key(), reflect.TypeOf(true)))
		for k := range data.(map[string]any) {
			tKey := reflect.New(t.Key())
			if err := tKey.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(k)); err != nil {
				return nil, err
			}

			if m.MapIndex(reflect.Indirect(tKey)).IsValid() {
				return nil, fmt.Errorf("duplicate name %q after unmarshaling %v", k, tKey)
			}
			m.SetMapIndex(reflect.Indirect(tKey), reflect.ValueOf(true))
		}
		return data, nil
	}
}

// This hook is used to solve the issue: https://github.com/open-telemetry/opentelemetry-collector/issues/4001
// We adopt the suggestion provided in this issue: https://github.com/mitchellh/mapstructure/issues/74#issuecomment-279886492
// We should empty every slice before unmarshalling unless user provided slice is nil.
// Assume that we had a struct with a field of type slice called `keys`, which has default values of ["a", "b"]
//
//	type Config struct {
//	  Keys []string `mapstructure:"keys"`
//	}
//
// The configuration provided by users may have following cases
// 1. configuration have `keys` field and have a non-nil values for this key, the output should be overrided
//   - for example, input is {"keys", ["c"]}, then output is Config{ Keys: ["c"]}
//
// 2. configuration have `keys` field and have an empty slice for this key, the output should be overrided by empty slics
//   - for example, input is {"keys", []}, then output is Config{ Keys: []}
//
// 3. configuration have `keys` field and have nil value for this key, the output should be default config
//   - for example, input is {"keys": nil}, then output is Config{ Keys: ["a", "b"]}
//
// 4. configuration have no `keys` field specified, the output should be default config
//   - for example, input is {}, then output is Config{ Keys: ["a", "b"]}
func zeroSliceHookFunc() mapstructure.DecodeHookFuncValue {
	return func(from reflect.Value, to reflect.Value) (interface{}, error) {
		if to.CanSet() && to.Kind() == reflect.Slice && from.Kind() == reflect.Slice {
			to.Set(reflect.MakeSlice(to.Type(), from.Len(), from.Cap()))
		}

		return from.Interface(), nil
	}
}
