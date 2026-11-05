// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// ExptFilterProjectionJSONSize matches encoding/json without encoding large map keys or values.
func ExptFilterProjectionJSONSize(row *ExptTurnResultFilterEntity) (int64, error) {
	if row == nil {
		return 0, fmt.Errorf("nil filter projection")
	}
	scalar := *row
	scalar.EvalTargetData = nil
	scalar.EvaluatorScore = nil
	scalar.AnnotationFloat = nil
	scalar.AnnotationBool = nil
	scalar.AnnotationString = nil
	scalar.EvalTargetMetrics = nil
	base, err := json.Marshal(&scalar)
	if err != nil {
		return 0, err
	}
	size := int64(len(base))
	addMap := func(valueSize int64, err error) error {
		if err != nil {
			return err
		}
		size += valueSize - 4 // Replace the scalar skeleton's null map.
		return nil
	}
	if err := addMap(projectionJSONMapSize(row.EvalTargetData, projectionJSONStringSize)); err != nil {
		return 0, err
	}
	if err := addMap(projectionJSONMapSize(row.EvaluatorScore, projectionJSONScalarSize[float64])); err != nil {
		return 0, err
	}
	if err := addMap(projectionJSONMapSize(row.AnnotationFloat, projectionJSONScalarSize[float64])); err != nil {
		return 0, err
	}
	if err := addMap(projectionJSONMapSize(row.AnnotationBool, projectionJSONScalarSize[bool])); err != nil {
		return 0, err
	}
	if err := addMap(projectionJSONMapSize(row.AnnotationString, projectionJSONStringSize)); err != nil {
		return 0, err
	}
	if err := addMap(projectionJSONMapSize(row.EvalTargetMetrics, projectionJSONScalarSize[int64])); err != nil {
		return 0, err
	}
	return size, nil
}

func projectionJSONMapSize[V any](values map[string]V, valueSize func(V) (int64, error)) (int64, error) {
	if values == nil {
		return 4, nil
	}
	size := int64(2)
	first := true
	for key, value := range values {
		if !first {
			size++
		}
		first = false
		keySize, _ := projectionJSONStringSize(key)
		n, err := valueSize(value)
		if err != nil {
			return 0, err
		}
		size += keySize + 1 + n
	}
	return size, nil
}
func projectionJSONScalarSize[V any](value V) (int64, error) {
	data, err := json.Marshal(value)
	return int64(len(data)), err
}
func projectionJSONStringSize(value string) (int64, error) {
	size := int64(2)
	for i := 0; i < len(value); {
		c := value[i]
		if c < utf8.RuneSelf {
			switch c {
			case '\\', '"', '\b', '\f', '\n', '\r', '\t':
				size += 2
			case '<', '>', '&':
				size += 6
			default:
				if c < 0x20 {
					size += 6
				} else {
					size++
				}
			}
			i++
			continue
		}
		r, width := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && width == 1 || r == '\u2028' || r == '\u2029' {
			size += 6
		} else {
			size += int64(width)
		}
		i += width
	}
	return size, nil
}
