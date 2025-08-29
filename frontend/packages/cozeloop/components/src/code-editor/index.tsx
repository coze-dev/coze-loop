export { default as CodeEditor, DiffEditor } from '@monaco-editor/react';
export { type Monaco, type MonacoDiffEditor } from '@monaco-editor/react';
export { type editor } from 'monaco-editor';
import { loader } from '@monaco-editor/react';

loader.config({
  paths: { vs: MONACO_UNPKG },
});
