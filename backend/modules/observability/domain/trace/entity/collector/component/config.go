// Original Files: open-telemetry/opentelemetry-collector
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// This file may have been modified by ByteDance Ltd.

package component

import (
	"errors"
	"reflect"
)

type Config any

type ConfigValidator interface {
	Validate() error
}

// As interface types are only used for static typing, a common idiom to find the reflection Type
// for an interface type Foo is to use a *Foo value.
var configValidatorType = reflect.TypeOf((*ConfigValidator)(nil)).Elem()

func ValidateConfig(cfg Config) error {
	return validate(reflect.ValueOf(cfg))
}

func validate(v reflect.Value) error {
	// Validate the value itself.
	switch v.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Pointer:
		return validate(v.Elem())
	case reflect.Struct:
		var errs []error
		errs = append(errs, callValidateIfPossible(v))
		// Reflect on the pointed data and check each of its fields.
		for i := 0; i < v.NumField(); i++ {
			if !v.Type().Field(i).IsExported() {
				continue
			}
			errs = append(errs, validate(v.Field(i)))
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		return nil
	case reflect.Slice, reflect.Array:
		var errs []error
		errs = append(errs, callValidateIfPossible(v))
		// Reflect on the pointed data and check each of its fields.
		for i := 0; i < v.Len(); i++ {
			errs = append(errs, validate(v.Index(i)))
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		return nil
	case reflect.Map:
		var errs []error
		errs = append(errs, callValidateIfPossible(v))
		iter := v.MapRange()
		for iter.Next() {
			errs = append(errs, validate(iter.Key()))
			errs = append(errs, validate(iter.Value()))
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		return nil
	default:
		err := callValidateIfPossible(v)
		if err != nil {
			return err
		}
		return nil
	}
}

func callValidateIfPossible(v reflect.Value) error {
	// If the value type implements ConfigValidator just call Validate
	if v.Type().Implements(configValidatorType) {
		err := v.Interface().(ConfigValidator).Validate()
		if err != nil {
			return err
		}
		return nil
	}
	// If the pointer type implements ConfigValidator call Validate on the pointer to the current value.
	if reflect.PointerTo(v.Type()).Implements(configValidatorType) {
		// If not addressable, then create a new *V pointer and set the value to current v.
		if !v.CanAddr() {
			pv := reflect.New(reflect.PointerTo(v.Type()).Elem())
			pv.Elem().Set(v)
			v = pv.Elem()
		}
		err := v.Addr().Interface().(ConfigValidator).Validate()
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}
