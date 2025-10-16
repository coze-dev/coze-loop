// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package vfs

import (
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
)

// 模拟 Reader 接口
type MockReader struct {
	content []byte
	pos     int
}

func (m *MockReader) ReadAt(p []byte, off int64) (n int, err error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockReader) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.content) {
		return 0, io.EOF
	}
	n = copy(p, m.content[m.pos:])
	m.pos += n
	return
}

func (m *MockReader) Close() error {
	return nil
}

// 模拟 fs.FileInfo 接口
type MockFileInfo struct{}

func (m *MockFileInfo) Name() string       { return "testfile" }
func (m *MockFileInfo) Size() int64        { return 1024 }
func (m *MockFileInfo) Mode() os.FileMode  { return 0o644 }
func (m *MockFileInfo) ModTime() time.Time { return time.Now() }
func (m *MockFileInfo) IsDir() bool        { return false }
func (m *MockFileInfo) Sys() interface{}   { return nil }

// MockCloser implements io.Closer interface for testing
type MockCloser struct {
	closed bool
	err    error
}

func (m *MockCloser) Close() error {
	m.closed = true
	return m.err
}

func TestNewFileReader(t *testing.T) {
	tests := []struct {
		name    string
		content string
		format  entity.FileFormat
		wantErr bool
	}{
		{
			name:    "CSV format",
			content: "col1,col2\nval1,val2",
			format:  entity.FileFormat_CSV,
			wantErr: false,
		},
		{
			name:    "JSONL format",
			content: "{\"key\": \"value\"}\n{\"key\": \"value2\"}",
			format:  entity.FileFormat_JSONL,
			wantErr: false,
		},
		{
			name:    "Unknown format",
			content: "test",
			format:  entity.FileFormat(999),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockReader := &MockReader{content: []byte(tt.content)}
			mockInfo := &MockFileInfo{}
			_, err := NewFileReader("testfile", mockReader, mockInfo, tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFileReader() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileReader_SeekToOffset(t *testing.T) {
	// 以 CSV 格式为例
	content := "col1,col2\nval1,val2\nval3,val4"
	mockReader := &MockReader{content: []byte(content)}
	mockInfo := &MockFileInfo{}
	fr, err := NewFileReader("testfile", mockReader, mockInfo, entity.FileFormat_CSV)
	if err != nil {
		t.Fatalf("NewFileReader() error = %v", err)
	}

	tests := []struct {
		name    string
		offset  int64
		wantErr bool
	}{
		{
			name:    "Seek to valid offset",
			offset:  2,
			wantErr: false,
		},
		{
			name:    "Seek to invalid offset",
			offset:  100,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fr.SeekToOffset(tt.offset)
			if (err != nil) != tt.wantErr {
				t.Errorf("SeekToOffset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileReader_Next(t *testing.T) {
	// 以 CSV 格式为例
	content := "col1,col2\nval1,val2\nval3,val4"
	mockReader := &MockReader{content: []byte(content)}
	mockInfo := &MockFileInfo{}
	fr, err := NewFileReader("testfile", mockReader, mockInfo, entity.FileFormat_CSV)
	if err != nil {
		t.Fatalf("NewFileReader() error = %v", err)
	}

	tests := []struct {
		name    string
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "Read first record",
			want: map[string]interface{}{
				"col1": "val1",
				"col2": "val2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fr.Next()
			if (err != nil) != tt.wantErr {
				t.Errorf("Next() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Next() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileReader_seekJSONL(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		line     int64
		wantErr  bool
		wantLine int64
	}{
		{
			name:     "Seek beyond file end",
			content:  `{"key1": "value1"}` + "\n" + `{"key2": "value2"}`,
			line:     3,
			wantErr:  true,
			wantLine: 2,
		},
		{
			name:     "Empty file",
			content:  "",
			line:     1,
			wantErr:  true,
			wantLine: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new FileReader with the test content
			mockReader := &MockReader{content: []byte(tt.content)}
			mockInfo := &MockFileInfo{}
			fr, err := NewFileReader("test.jsonl", mockReader, mockInfo, entity.FileFormat_JSONL)
			if err != nil {
				t.Fatalf("NewFileReader() error = %v", err)
			}

			// Try to seek to the specified line
			err = fr.seekJSONL(tt.line)

			// Check if error matches expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("seekJSONL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check if we're at the expected line
			if fr.cursor != tt.wantLine {
				t.Errorf("seekJSONL() cursor = %v, want %v", fr.cursor, tt.wantLine)
			}

			// If seek was successful, verify we can read the next line correctly
			if err == nil {
				data, err := fr.Next()
				if err != nil {
					t.Errorf("Next() unexpected error = %v", err)
					return
				}

				// Verify the content is as expected
				switch tt.line {
				case 1:
					if data["key1"] != "value1" {
						t.Errorf("Next() got = %v, want key1=value1", data)
					}
				case 2:
					if data["key2"] != "value2" {
						t.Errorf("Next() got = %v, want key2=value2", data)
					}
				case 3:
					if data["key3"] != "value3" && data["key2"] != "value2" { // depending on empty lines
						t.Errorf("Next() got = %v, want key2=value2 or key3=value3", data)
					}
				}
			}
		})
	}
}

func TestFileReader_nextInJSONL(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantData map[string]any
		wantErr  bool
	}{
		{
			name:     "Valid JSON line",
			content:  `{"key": "value"}` + "\n",
			wantData: map[string]any{"key": "value"},
			wantErr:  false,
		},
		{
			name:     "Multiple JSON lines",
			content:  `{"key1": "value1"}` + "\n" + `{"key2": "value2"}` + "\n",
			wantData: map[string]any{"key1": "value1"},
			wantErr:  false,
		},
		{
			name:     "JSON with nested structure",
			content:  `{"obj": {"nested": "value"}, "arr": [1, 2, 3]}` + "\n",
			wantData: map[string]any{"obj": map[string]any{"nested": "value"}, "arr": []any{float64(1), float64(2), float64(3)}},
			wantErr:  false,
		},
		{
			name:     "Empty lines between JSON",
			content:  `{"key1": "value1"}` + "\n\n\n" + `{"key2": "value2"}` + "\n",
			wantData: map[string]any{"key1": "value1"},
			wantErr:  false,
		},
		{
			name:    "Invalid JSON",
			content: `{"key": "value"` + "\n",
			wantErr: true,
		},
		{
			name:    "Empty file",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new FileReader with the test content
			mockReader := &MockReader{content: []byte(tt.content)}
			mockInfo := &MockFileInfo{}
			fr, err := NewFileReader("test.jsonl", mockReader, mockInfo, entity.FileFormat_JSONL)
			if err != nil {
				t.Fatalf("NewFileReader() error = %v", err)
			}

			// Try to read the next record
			data, err := fr.nextInJSONL()

			// Check if error matches expectation
			if tt.wantErr {
				if err == nil {
					t.Error("nextInJSONL() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("nextInJSONL() unexpected error = %v", err)
				return
			}

			// Verify the data
			if !reflect.DeepEqual(data, tt.wantData) {
				t.Errorf("nextInJSONL() got = %v, want %v", data, tt.wantData)
			}

			// Verify cursor was incremented
			if fr.cursor != 1 {
				t.Errorf("nextInJSONL() cursor = %v, want 1", fr.cursor)
			}

			// For multiple lines test, verify we can read the second line
			if tt.name == "Multiple JSON lines" {
				data2, err := fr.nextInJSONL()
				if err != nil {
					t.Errorf("nextInJSONL() second read error = %v", err)
				}
				want2 := map[string]any{"key2": "value2"}
				if !reflect.DeepEqual(data2, want2) {
					t.Errorf("nextInJSONL() second read got = %v, want %v", data2, want2)
				}
				if fr.cursor != 2 {
					t.Errorf("nextInJSONL() cursor after second read = %v, want 2", fr.cursor)
				}
			}

			// For empty lines test, verify we can read the next non-empty line
			if tt.name == "Empty lines between JSON" {
				data2, err := fr.nextInJSONL()
				if err != nil {
					t.Errorf("nextInJSONL() second read error = %v", err)
				}
				want2 := map[string]any{"key2": "value2"}
				if !reflect.DeepEqual(data2, want2) {
					t.Errorf("nextInJSONL() second read got = %v, want %v", data2, want2)
				}
				if fr.cursor != 4 { // should be 4 because of empty lines
					t.Errorf("nextInJSONL() cursor after second read = %v, want 4", fr.cursor)
				}
			}
		})
	}
}

func TestFileReader_close(t *testing.T) {
	tests := []struct {
		name    string
		closer  io.Closer
		wantErr bool
	}{
		{
			name:    "Nil closer",
			closer:  nil,
			wantErr: false,
		},
		{
			name:    "Successful close",
			closer:  &MockCloser{},
			wantErr: false,
		},
		{
			name:    "Close with error",
			closer:  &MockCloser{err: errors.New("close error")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &FileReader{
				closer: tt.closer,
			}

			err := fr.close()

			// Check if error matches expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("close() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If we have a mock closer, verify it was actually closed
			if mockCloser, ok := tt.closer.(*MockCloser); ok {
				if !mockCloser.closed {
					t.Error("close() did not close the closer")
				}
			}
		})
	}
}

func TestFileReader_GetName(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     string
	}{
		{
			name:     "正常文件名",
			fileName: "test.csv",
			want:     "test.csv",
		},
		{
			name:     "带路径的文件名",
			fileName: "data/test.csv",
			want:     "data/test.csv",
		},
		{
			name:     "空文件名",
			fileName: "",
			want:     "",
		},
		{
			name:     "特殊字符文件名",
			fileName: "test-file_123.csv",
			want:     "test-file_123.csv",
		},
		{
			name:     "中文文件名",
			fileName: "测试文件.csv",
			want:     "测试文件.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &FileReader{
				name: tt.fileName,
			}
			got := fr.GetName()
			assert.Equal(t, tt.want, got, "GetName() = %v, want %v", got, tt.want)
		})
	}
}

func TestFileReader_GetCursor(t *testing.T) {
	tests := []struct {
		name      string
		cursorPos int64
		want      int64
	}{
		{
			name:      "初始位置",
			cursorPos: 0,
			want:      0,
		},
		{
			name:      "正常位置",
			cursorPos: 42,
			want:      42,
		},
		{
			name:      "大数值位置",
			cursorPos: 999999,
			want:      999999,
		},
		{
			name:      "最大位置",
			cursorPos: int64(^uint64(0) >> 1), // int64的最大值
			want:      int64(^uint64(0) >> 1),
		},
		{
			name:      "负数位置",
			cursorPos: -1,
			want:      -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &FileReader{
				cursor: tt.cursorPos,
			}
			got := fr.GetCursor()
			assert.Equal(t, tt.want, got, "GetCursor() = %v, want %v", got, tt.want)
		})
	}
}
