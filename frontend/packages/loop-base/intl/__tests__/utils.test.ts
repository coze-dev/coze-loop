// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import IntlMessageFormat from 'intl-messageformat';

import { stringifyVal, fillMissingOptions } from '../src/utils';

describe('stringifyVal', () => {
  it('should return an empty string for undefined', () => {
    expect(stringifyVal(undefined)).toBe('');
  });

  it('should return the string representation for numbers', () => {
    expect(stringifyVal(42)).toBe('42');
  });

  it('should return the string representation for booleans', () => {
    expect(stringifyVal(true)).toBe('true');
    expect(stringifyVal(false)).toBe('false');
  });

  it('should return the string representation for strings', () => {
    expect(stringifyVal('hello')).toBe('hello');
  });

  it('should return the string representation for symbols', () => {
    const symbol = Symbol('test');
    expect(stringifyVal(symbol)).toBe('Symbol(test)');
  });

  it('should return an empty string for null', () => {
    expect(stringifyVal(null)).toBe('');
  });

  it('should return the ISO string representation for dates', () => {
    const date = new Date('2023-04-19T12:34:56.789Z');
    expect(stringifyVal(date)).toBe('2023-04-19T12:34:56.789Z');
  });

  it('should return the JSON string representation for objects', () => {
    const obj = { foo: 'bar', baz: 42 };
    expect(stringifyVal(obj)).toBe('{"foo":"bar","baz":42}');
  });
});

describe('fillMissingOptions', () => {
  it('should return undefined if the AST is empty', () => {
    const messageFormat = new IntlMessageFormat('');
    const options = {};
    expect(fillMissingOptions(messageFormat, options)).toBeUndefined();
  });

  it('should return the original options if there are no missing options', () => {
    const messageFormat = new IntlMessageFormat('Hello, {name}!', 'en');
    const options = { name: 'John' };
    expect(fillMissingOptions(messageFormat, options)).toEqual(options);
  });

  it('should fill in missing options with empty strings', () => {
    const messageFormat = new IntlMessageFormat(
      'Hello, {name} and {age}!',
      'en',
    );
    const options = { name: 'John' };
    expect(fillMissingOptions(messageFormat, options)).toEqual({
      name: 'John',
      age: '',
    });
  });
});
