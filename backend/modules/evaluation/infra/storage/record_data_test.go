package storage

import (
    "bytes"
    "context"
    "errors"
    "io"
    "strings"
    "testing"

    "github.com/bytedance/gg/gptr"
    "github.com/stretchr/testify/assert"
    "go.uber.org/mock/gomock"

    fsMocks "github.com/coze-dev/coze-loop/backend/infra/fileserver/mocks"
    "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
    "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ---- test helpers ----

type fakeConfiger struct{ cfg *component.EvaluationRecordStorage }

func (f *fakeConfiger) GetEvaluationRecordStorage(ctx context.Context) *component.EvaluationRecordStorage {
    return f.cfg
}

// the following methods are not used in this package; implement stubs to satisfy interface
func (f *fakeConfiger) GetConsumerConf(ctx context.Context) *entity.ExptConsumerConf { return nil }
func (f *fakeConfiger) GetErrCtrl(ctx context.Context) *entity.ExptErrCtrl { return nil }
func (f *fakeConfiger) GetExptExecConf(ctx context.Context, spaceID int64) *entity.ExptExecConf {
    return nil
}
func (f *fakeConfiger) GetErrRetryConf(ctx context.Context, spaceID int64, err error) *entity.RetryConf {
    return nil
}
func (f *fakeConfiger) GetExptTurnResultFilterBmqProducerCfg(ctx context.Context) *entity.BmqProducerCfg {
    return nil
}
func (f *fakeConfiger) GetCKDBName(ctx context.Context) *entity.CKDBConfig { return nil }
func (f *fakeConfiger) GetExptExportWhiteList(ctx context.Context) *entity.ExptExportWhiteList {
    return nil
}
func (f *fakeConfiger) GetMaintainerUserIDs(ctx context.Context) map[string]bool { return nil }
func (f *fakeConfiger) GetSchedulerAbortCtrl(ctx context.Context) *entity.SchedulerAbortCtrl { return nil }
func (f *fakeConfiger) GetTargetTrajectoryConf(ctx context.Context) *entity.TargetTrajectoryConf { return nil }

type nopReader struct{ buf *bytes.Reader }

func newNopReader(b []byte) *nopReader { return &nopReader{buf: bytes.NewReader(b)} }
func (r *nopReader) Read(p []byte) (int, error)    { return r.buf.Read(p) }
func (r *nopReader) ReadAt(p []byte, off int64) (int, error) { return r.buf.ReadAt(p, off) }
func (r *nopReader) Close() error                  { return nil }

// ---- tests ----

func Test_getFieldMaxSize(t *testing.T) {
    s := &RecordDataStorage{configer: &fakeConfiger{cfg: &component.EvaluationRecordStorage{
        Providers: []*component.EvaluationRecordProviderConfig{
            {Provider: "RDS", MaxSize: 12345},
        },
    }}}
    got := s.getFieldMaxSize(context.Background())
    assert.Equal(t, int64(12345), got)

    s2 := &RecordDataStorage{configer: &fakeConfiger{cfg: &component.EvaluationRecordStorage{}}}
    assert.Equal(t, int64(0), s2.getFieldMaxSize(context.Background()))

    // nil cfg -> 0
    s3 := &RecordDataStorage{configer: &fakeConfiger{cfg: nil}}
    assert.Equal(t, int64(0), s3.getFieldMaxSize(context.Background()))
}

func Test_processContent_upload_when_exceeds(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

    // expect one upload whose key has expected prefix and body equals original
    var uploadedKey string
    var uploadedBody []byte
    mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
        func(_ context.Context, key string, r io.Reader, _ ...interface{}) error {
            uploadedKey = key
            body, _ := io.ReadAll(r)
            uploadedBody = body
            return nil
        },
    ).Times(1)

    s := &RecordDataStorage{batchStorage: mockS3, configer: &fakeConfiger{}}
    longText := bytes.Repeat([]byte("x"), 1024)
    c := &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of(string(longText))}
    err := s.processContent(context.Background(), c, 100) // max=100 -> should upload
    assert.NoError(t, err)
    assert.NotEmpty(t, uploadedKey)
    assert.True(t, len(uploadedBody) == 1024)
    assert.True(t, len(gptr.Indirect(c.Text)) <= 100)
    assert.True(t, gptr.Indirect(c.ContentOmitted))
    if assert.NotNil(t, c.FullContent) {
        assert.Equal(t, entity.StorageProvider_S3, gptr.Indirect(c.FullContent.Provider))
        assert.True(t, strings.HasPrefix(gptr.Indirect(c.FullContent.URI), EvalRecordFieldKeyPrefix))
    }
    assert.Equal(t, int32(1024), gptr.Indirect(c.FullContentBytes))
}

func Test_processContent_skip_cases(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

    s := &RecordDataStorage{batchStorage: mockS3, configer: &fakeConfiger{}}

    // nil content
    assert.NoError(t, s.processContent(context.Background(), nil, 10))

    // non-text content
    img := &entity.Content{ContentType: gptr.Of(entity.ContentTypeImage)}
    assert.NoError(t, s.processContent(context.Background(), img, 10))

    // short text
    txt := &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("hello")}
    assert.NoError(t, s.processContent(context.Background(), txt, 10))
}

func Test_loadContentFromS3(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

    full := "full text"
    mockS3.EXPECT().Read(gomock.Any(), gomock.Any()).Return(newNopReader([]byte(full)), nil)

    s := &RecordDataStorage{batchStorage: mockS3}
    c := &entity.Content{
        ContentType:    gptr.Of(entity.ContentTypeText),
        Text:           gptr.Of("short"),
        ContentOmitted: gptr.Of(true),
        FullContent:    &entity.ObjectStorage{Provider: entity.StorageProviderPtr(entity.StorageProvider_S3), URI: gptr.Of("some-key")},
        FullContentBytes: gptr.Of(int32(len(full))),
    }

    err := s.loadContentFromS3(context.Background(), c)
    assert.NoError(t, err)
    assert.Equal(t, full, gptr.Indirect(c.Text))
    assert.False(t, gptr.Indirect(c.ContentOmitted))
    assert.Nil(t, c.FullContent)
    assert.Nil(t, c.FullContentBytes)
}

func Test_loadContentFromS3_error(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

    mockS3.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, errors.New("read error"))

    s := &RecordDataStorage{batchStorage: mockS3}
    c := &entity.Content{
        ContentType:    gptr.Of(entity.ContentTypeText),
        ContentOmitted: gptr.Of(true),
        FullContent:    &entity.ObjectStorage{Provider: entity.StorageProviderPtr(entity.StorageProvider_S3), URI: gptr.Of("key")},
    }
    err := s.loadContentFromS3(context.Background(), c)
    assert.Error(t, err)
}

func Test_processContent_recursive_multipart(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

    mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

    s := &RecordDataStorage{batchStorage: mockS3}
    part := &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of(strings.Repeat("y", 200))}
    c := &entity.Content{ContentType: gptr.Of(entity.ContentTypeMultipart), MultiPart: []*entity.Content{part}}

    err := s.processContent(context.Background(), c, 100)
    assert.NoError(t, err)
    assert.True(t, gptr.Indirect(part.ContentOmitted))
}

func Test_SaveAndLoad_EvaluatorRecordData(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

    // expect uploads for three large fields: input_fields, evaluate_dataset_fields, evaluate_target_output_fields, and one history message
    // total 4 Upload calls
    mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(4)
    mockS3.EXPECT().Read(gomock.Any(), gomock.Any()).Return(newNopReader([]byte("XXXX")), nil).AnyTimes()

    cfg := &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 2}}}
    s := NewRecordDataStorage(mockS3, &fakeConfiger{cfg: cfg})

    rec := &entity.EvaluatorRecord{
        EvaluatorInputData: &entity.EvaluatorInputData{
            InputFields: map[string]*entity.Content{
                "if": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("abcd")},
            },
            EvaluateDatasetFields: map[string]*entity.Content{
                "df": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("abcd")},
            },
            EvaluateTargetOutputFields: map[string]*entity.Content{
                "tf": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("abcd")},
            },
            HistoryMessages: []*entity.Message{{Content: &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("abcd")}}},
        },
    }

    // save -> mark omitted and upload
    assert.NoError(t, s.SaveEvaluatorRecordData(context.Background(), rec))
    assert.True(t, gptr.Indirect(rec.EvaluatorInputData.InputFields["if"].ContentOmitted))
    assert.True(t, gptr.Indirect(rec.EvaluatorInputData.EvaluateDatasetFields["df"].ContentOmitted))
    assert.True(t, gptr.Indirect(rec.EvaluatorInputData.EvaluateTargetOutputFields["tf"].ContentOmitted))
    assert.True(t, gptr.Indirect(rec.EvaluatorInputData.HistoryMessages[0].Content.ContentOmitted))

    // load back omitted fields
    assert.NoError(t, s.LoadEvaluatorRecordData(context.Background(), rec))
}

func Test_SaveAndLoad_EvalTargetRecordData(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

    // any upload succeeds
    mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

    // config: RDS max size small to force upload
    cfg := &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 10}}}
    s := NewRecordDataStorage(mockS3, &fakeConfiger{cfg: cfg})

    // prepare record with input and output fields exceeding 10 bytes
    rec := &entity.EvalTargetRecord{
        EvalTargetInputData: &entity.EvalTargetInputData{
            InputFields: map[string]*entity.Content{
                "a": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("0123456789abcdef")},
            },
        },
        EvalTargetOutputData: &entity.EvalTargetOutputData{
            OutputFields: map[string]*entity.Content{
                "b": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("0123456789abcdef")},
            },
        },
    }

    err := s.SaveEvalTargetRecordData(context.Background(), rec)
    assert.NoError(t, err)
    // fields should be marked omitted
    assert.True(t, gptr.Indirect(rec.EvalTargetInputData.InputFields["a"].ContentOmitted))
    assert.True(t, gptr.Indirect(rec.EvalTargetOutputData.OutputFields["b"].ContentOmitted))

    // now test selective load for output fields
    // setup Read to return the original content
    mockS3.EXPECT().Read(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key string, _ ...interface{}) (interface{ io.ReadCloser; io.ReaderAt }, error) {
        return newNopReader([]byte("0123456789abcdef")), nil
    }).AnyTimes()

    err = s.LoadEvalTargetOutputFields(context.Background(), rec, []string{"b"})
    assert.NoError(t, err)
    assert.Equal(t, "0123456789abcdef", rec.EvalTargetOutputData.OutputFields["b"].GetText())
}

func Test_LoadEvalTargetRecordData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

	mockS3.EXPECT().Read(gomock.Any(), "key-a").Return(newNopReader([]byte("loaded-a")), nil)
	mockS3.EXPECT().Read(gomock.Any(), "key-b").Return(newNopReader([]byte("loaded-b")), nil)

	s := &RecordDataStorage{batchStorage: mockS3}
	rec := &entity.EvalTargetRecord{
		EvalTargetInputData: &entity.EvalTargetInputData{
			InputFields: map[string]*entity.Content{
				"a": {
					ContentType:      gptr.Of(entity.ContentTypeText),
					Text:             gptr.Of(""),
					ContentOmitted:   gptr.Of(true),
					FullContent:      &entity.ObjectStorage{URI: gptr.Of("key-a")},
					FullContentBytes: gptr.Of(int32(8)),
				},
			},
		},
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"b": {
					ContentType:      gptr.Of(entity.ContentTypeText),
					Text:             gptr.Of(""),
					ContentOmitted:   gptr.Of(true),
					FullContent:      &entity.ObjectStorage{URI: gptr.Of("key-b")},
					FullContentBytes: gptr.Of(int32(8)),
				},
			},
		},
	}
	err := s.LoadEvalTargetRecordData(context.Background(), rec)
	assert.NoError(t, err)
	assert.Equal(t, "loaded-a", rec.EvalTargetInputData.InputFields["a"].GetText())
	assert.Equal(t, "loaded-b", rec.EvalTargetOutputData.OutputFields["b"].GetText())
}

func Test_LoadEvalTargetRecordData_guard(t *testing.T) {
	s := &RecordDataStorage{}
	assert.NoError(t, s.LoadEvalTargetRecordData(context.Background(), nil))
	assert.NoError(t, s.LoadEvalTargetRecordData(context.Background(), &entity.EvalTargetRecord{}))
}

func Test_processEvalTargetInputData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	cfg := &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 2}}}
	s := &RecordDataStorage{batchStorage: mockS3, configer: &fakeConfiger{cfg: cfg}}

	input := &entity.EvalTargetInputData{
		InputFields: map[string]*entity.Content{
			"x": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("longtext")},
		},
	}
	err := s.processEvalTargetInputData(context.Background(), input, 2)
	assert.NoError(t, err)
	assert.True(t, gptr.Indirect(input.InputFields["x"].ContentOmitted))
}

func Test_processEvalTargetInputData_withHistoryMessages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	// InputFields 1 个 + HistoryMessages 1 个 = 2 次 Upload
	mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)

	cfg := &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 5}}}
	s := &RecordDataStorage{batchStorage: mockS3, configer: &fakeConfiger{cfg: cfg}}

	input := &entity.EvalTargetInputData{
		InputFields: map[string]*entity.Content{
			"a": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("longinputfield")},
		},
		HistoryMessages: []*entity.Message{
			{Content: &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("longhistorymsg")}},
		},
	}
	err := s.processEvalTargetInputData(context.Background(), input, 5)
	assert.NoError(t, err)
	assert.True(t, gptr.Indirect(input.InputFields["a"].ContentOmitted))
	assert.True(t, gptr.Indirect(input.HistoryMessages[0].Content.ContentOmitted))
}

func Test_loadOmittedContentFromInputData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	mockS3.EXPECT().Read(gomock.Any(), "key-in").Return(newNopReader([]byte("loaded-input")), nil)
	mockS3.EXPECT().Read(gomock.Any(), "key-msg").Return(newNopReader([]byte("loaded-msg")), nil)

	s := &RecordDataStorage{batchStorage: mockS3}
	input := &entity.EvalTargetInputData{
		InputFields: map[string]*entity.Content{
			"in": {
				ContentType:      gptr.Of(entity.ContentTypeText),
				Text:             gptr.Of(""),
				ContentOmitted:   gptr.Of(true),
				FullContent:      &entity.ObjectStorage{URI: gptr.Of("key-in")},
				FullContentBytes: gptr.Of(int32(12)),
			},
		},
		HistoryMessages: []*entity.Message{
			{
				Content: &entity.Content{
					ContentType:      gptr.Of(entity.ContentTypeText),
					Text:             gptr.Of(""),
					ContentOmitted:   gptr.Of(true),
					FullContent:      &entity.ObjectStorage{URI: gptr.Of("key-msg")},
					FullContentBytes: gptr.Of(int32(9)),
				},
			},
		},
	}
	err := s.loadOmittedContentFromInputData(context.Background(), input)
	assert.NoError(t, err)
	assert.Equal(t, "loaded-input", input.InputFields["in"].GetText())
	assert.False(t, gptr.Indirect(input.InputFields["in"].ContentOmitted))
	assert.Equal(t, "loaded-msg", input.HistoryMessages[0].Content.GetText())
	assert.False(t, gptr.Indirect(input.HistoryMessages[0].Content.ContentOmitted))
}


func Test_loadContentFromS3_skip_when_not_omitted(t *testing.T) {
	s := &RecordDataStorage{}
	c := &entity.Content{
		ContentType:    gptr.Of(entity.ContentTypeText),
		Text:           gptr.Of("short"),
		ContentOmitted: gptr.Of(false),
	}
	err := s.loadContentFromS3(context.Background(), c)
	assert.NoError(t, err)
	assert.Equal(t, "short", c.GetText())

	c2 := &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("x")}
	err = s.loadContentFromS3(context.Background(), c2)
	assert.NoError(t, err)

	c3 := &entity.Content{
		ContentType:    gptr.Of(entity.ContentTypeText),
		ContentOmitted: gptr.Of(true),
		FullContent:    &entity.ObjectStorage{URI: nil},
	}
	err = s.loadContentFromS3(context.Background(), c3)
	assert.NoError(t, err)
}

func Test_processContent_upload_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

	mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("upload failed"))

	cfg := &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 10}}}
	s := &RecordDataStorage{batchStorage: mockS3, configer: &fakeConfiger{cfg: cfg}}

	longText := strings.Repeat("x", 100)
	c := &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of(longText)}
	err := s.processContent(context.Background(), c, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upload")
}

func Test_loadContentFromS3_readAll_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)

	// reader that returns error on Read
	errReader := &failingReader{err: errors.New("read body failed")}
	mockS3.EXPECT().Read(gomock.Any(), gomock.Any()).Return(errReader, nil)

	s := &RecordDataStorage{batchStorage: mockS3}
	c := &entity.Content{
		ContentType:    gptr.Of(entity.ContentTypeText),
		ContentOmitted: gptr.Of(true),
		FullContent:   &entity.ObjectStorage{Provider: entity.StorageProviderPtr(entity.StorageProvider_S3), URI: gptr.Of("key")},
	}
	err := s.loadContentFromS3(context.Background(), c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read field body")
}

type failingReader struct{ err error }

func (r *failingReader) Read(p []byte) (int, error)   { return 0, r.err }
func (r *failingReader) ReadAt(p []byte, off int64) (int, error) { return 0, r.err }
func (r *failingReader) Close() error                 { return nil }

func Test_SaveEvaluatorRecordData_processInputData_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("upload err"))

	cfg := &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 5}}}
	s := NewRecordDataStorage(mockS3, &fakeConfiger{cfg: cfg})

	rec := &entity.EvaluatorRecord{
		EvaluatorInputData: &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"f": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("longtext")},
			},
		},
	}
	err := s.SaveEvaluatorRecordData(context.Background(), rec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process evaluator input data")
}

func Test_LoadEvaluatorRecordData_loadOmittedContent_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	mockS3.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, errors.New("s3 read err"))

	s := &RecordDataStorage{batchStorage: mockS3}
	rec := &entity.EvaluatorRecord{
		EvaluatorInputData: &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"f": {
					ContentType:    gptr.Of(entity.ContentTypeText),
					ContentOmitted: gptr.Of(true),
					FullContent:    &entity.ObjectStorage{URI: gptr.Of("key")},
				},
			},
		},
	}
	err := s.LoadEvaluatorRecordData(context.Background(), rec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load evaluator input omitted content")
}

func Test_SaveEvalTargetRecordData_processInput_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("upload err"))

	cfg := &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 5}}}
	s := NewRecordDataStorage(mockS3, &fakeConfiger{cfg: cfg})

	rec := &entity.EvalTargetRecord{
		EvalTargetInputData: &entity.EvalTargetInputData{
			InputFields: map[string]*entity.Content{
				"a": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("longtext")},
			},
		},
	}
	err := s.SaveEvalTargetRecordData(context.Background(), rec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process eval target input data")
}

func Test_SaveEvalTargetRecordData_processOutput_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	// input "short" (5 chars) <= maxSize 5, no upload; output "longoutput" (10 chars) triggers upload
	mockS3.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("upload err"))

	cfg := &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 5}}}
	s := NewRecordDataStorage(mockS3, &fakeConfiger{cfg: cfg})

	rec := &entity.EvalTargetRecord{
		EvalTargetInputData: &entity.EvalTargetInputData{
			InputFields: map[string]*entity.Content{
				"a": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("x")},
			},
		},
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"b": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("longoutput")},
			},
		},
	}
	err := s.SaveEvalTargetRecordData(context.Background(), rec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process eval target output data")
}

func Test_LoadEvalTargetOutputFields_loadContent_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	mockS3.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, errors.New("read err"))

	s := &RecordDataStorage{batchStorage: mockS3}
	rec := &entity.EvalTargetRecord{
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"f": {
					ContentType:    gptr.Of(entity.ContentTypeText),
					ContentOmitted: gptr.Of(true),
					FullContent:    &entity.ObjectStorage{URI: gptr.Of("key")},
				},
			},
		},
	}
	err := s.LoadEvalTargetOutputFields(context.Background(), rec, []string{"f"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load output field")
}

func Test_LoadEvalTargetRecordData_loadOmitted_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
	mockS3.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, errors.New("s3 read err"))

	s := &RecordDataStorage{batchStorage: mockS3}
	rec := &entity.EvalTargetRecord{
		EvalTargetInputData: &entity.EvalTargetInputData{
			InputFields: map[string]*entity.Content{
				"a": {
					ContentType:    gptr.Of(entity.ContentTypeText),
					ContentOmitted: gptr.Of(true),
					FullContent:    &entity.ObjectStorage{URI: gptr.Of("key-a")},
				},
			},
		},
	}
	err := s.LoadEvalTargetRecordData(context.Background(), rec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load eval target input omitted content")
}

func Test_SaveEvaluatorRecordData_guard(t *testing.T) {
    // batchStorage=nil -> no-op
    s := NewRecordDataStorage(nil, &fakeConfiger{cfg: &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 10}}}})
    assert.NoError(t, s.SaveEvaluatorRecordData(context.Background(), &entity.EvaluatorRecord{}))

    // fieldMaxSize<=0 -> no-op
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockS3 := fsMocks.NewMockBatchObjectStorage(ctrl)
    s2 := NewRecordDataStorage(mockS3, &fakeConfiger{cfg: &component.EvaluationRecordStorage{Providers: []*component.EvaluationRecordProviderConfig{{Provider: "RDS", MaxSize: 0}}}})
    assert.NoError(t, s2.SaveEvaluatorRecordData(context.Background(), &entity.EvaluatorRecord{}))
}
