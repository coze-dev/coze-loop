// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package fileserver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type S3Client struct {
	s3  *s3.S3
	cfg *S3Config
}

var (
	_ ObjectStorage      = (*S3Client)(nil)
	_ BatchObjectStorage = (*S3Client)(nil)
)

func NewS3Client(cfg *S3Config) (*S3Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.WithMessagef(err, "invalid s3 config")
	}

	session, err := session.NewSession(&aws.Config{
		Region:           lo.ToPtr(cfg.Region),
		Endpoint:         lo.ToPtr(cfg.Endpoint),
		Credentials:      credentials.NewStaticCredentials(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		S3ForcePathStyle: cfg.ForcePathStyle,
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "create aws session")
	}

	// TODO: add telemetry
	cli := s3.New(session)
	return &S3Client{s3: cli, cfg: cfg}, nil
}

func (c *S3Client) Stat(ctx context.Context, key string, opts ...StatOpt) (*ObjectInfo, error) {
	option := NewStatOption(opts...)
	bucket := lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)
	input := &s3.HeadObjectInput{
		Bucket: lo.ToPtr(bucket),
		Key:    lo.ToPtr(key),
	}
	output, err := c.s3.HeadObjectWithContext(ctx, input)
	if err != nil {
		return nil, errors.WithMessagef(err, "head object '%s'", key)
	}
	metadata := lo.MapEntries(output.Metadata, func(key string, value *string) (string, string) {
		return key, lo.FromPtr(value)
	})
	return NewObjectInfo(key, lo.FromPtr(output.ContentLength), lo.FromPtr(output.LastModified), metadata), nil
}

func (c *S3Client) Upload(ctx context.Context, key string, r io.Reader, opts ...UploadOpt) error {
	option := NewUploadOption(opts...)
	bucket := lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)
	metadata := lo.MapEntries(option.Metadata, func(key string, value string) (string, *string) {
		return key, lo.ToPtr(value)
	})
	contentType := lo.FirstOrEmpty(option.ContentTypes)

	uploader := s3manager.NewUploaderWithClient(c.s3, func(u *s3manager.Uploader) {
		u.PartSize = c.cfg.UploadPartSize
		u.Concurrency = option.Concurrency
	})
	output, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      lo.ToPtr(bucket),
		Key:         lo.ToPtr(key),
		Body:        r,
		Metadata:    metadata,
		ContentType: lo.ToPtr(contentType),
	})
	if err != nil {
		return errors.Wrapf(err, "upload object '%s'", key)
	}
	logs.CtxInfo(ctx, "uploaded object '%s' done, upload_id=%s", key, output.UploadID)
	return nil
}

func (c *S3Client) Download(ctx context.Context, key string, writer io.WriterAt, opts ...DownloadOpt) error {
	option := NewDownloadOption(opts...)
	ctx, cancel := option.ContextWithTimeout(ctx)
	defer cancel()

	return c.download(ctx, key, writer, option)
}

func (c *S3Client) Read(ctx context.Context, key string, opts ...DownloadOpt) (Reader, error) {
	option := NewDownloadOption(opts...)
	bucket := lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)
	ctx, cancel := option.ContextWithTimeout(ctx)
	defer cancel()

	info, err := c.Stat(ctx, key)
	if err != nil {
		return nil, errors.WithMessagef(err, "stat object '%s'", key)
	}

	// small object, read in memory
	if info.FSize <= c.cfg.CacheSizeGT {
		input := &s3.GetObjectInput{Bucket: lo.ToPtr(bucket), Key: lo.ToPtr(key)}
		obj, err := c.s3.GetObjectWithContext(ctx, input)
		if err != nil {
			return nil, errors.WithMessagef(err, "get object '%s'", key)
		}
		data, err := io.ReadAll(obj.Body)
		if err != nil {
			return nil, errors.Wrapf(err, "read object body for '%s'", key)
		}
		return NopCloser(bytes.NewReader(data)), nil
	}

	// big object, download to local file in parts
	file, err := os.CreateTemp("", "s3-download-*.tmp")
	if err != nil {
		return nil, errors.WithMessagef(err, "create temp file for '%s'", key)
	}
	logs.CtxInfo(ctx, "s3 object %s will be downloaded to %s, size=%d", key, file.Name(), info.FSize)
	if err := c.download(ctx, key, file, option); err != nil {
		return nil, errors.WithMessagef(err, "download object '%s'", key)
	}
	// seek to start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.WithMessagef(err, "seek to start of file '%s'", file.Name())
	}
	return file, nil
}

func (c *S3Client) download(ctx context.Context, key string, w io.WriterAt, option *DownloadOption) error {
	input := &s3.GetObjectInput{
		Bucket: lo.ToPtr(lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)),
		Key:    lo.ToPtr(key),
	}

	downloader := s3manager.NewDownloaderWithClient(c.s3, func(d *s3manager.Downloader) {
		d.PartSize = c.cfg.DownloadPartSize
		d.Concurrency = option.Concurrency
	})

	if _, err := downloader.DownloadWithContext(ctx, w, input); err != nil {
		return errors.Wrapf(err, "download object %s", key)
	}

	return nil
}

func (c *S3Client) Remove(ctx context.Context, key string, opts ...RemoveOpt) error {
	option := NewRemoveOption(opts...)
	bucket := lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)
	input := &s3.DeleteObjectInput{
		Bucket: lo.ToPtr(bucket),
		Key:    lo.ToPtr(key),
	}
	_, err := c.s3.DeleteObjectWithContext(ctx, input)
	if err != nil {
		return errors.Wrapf(err, "delete object '%s'", key)
	}

	logs.CtxInfo(ctx, "deleted object '%s'", key)
	return nil
}

func (c *S3Client) SignDownloadReq(ctx context.Context, key string, opts ...SignOpt) (url string, header http.Header, err error) {
	option := NewSignOption(opts...)
	bucket := lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)
	input := &s3.GetObjectInput{
		Bucket: lo.ToPtr(bucket),
		Key:    lo.ToPtr(key),
	}
	req, _ := c.s3.GetObjectRequest(input)
	return req.PresignRequest(option.TTL)
}

func (c *S3Client) SignUploadReq(ctx context.Context, key string, opts ...SignOpt) (url string, header http.Header, err error) {
	option := NewSignOption(opts...)
	bucket := lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)
	input := &s3.PutObjectInput{
		Bucket: lo.ToPtr(bucket),
		Key:    lo.ToPtr(key),
	}
	req, _ := c.s3.PutObjectRequest(input)
	return req.PresignRequest(option.TTL)
}

func (c *S3Client) BatchUpload(ctx context.Context, keys []string, readers []io.Reader, opts ...UploadOpt) error {
	option := NewUploadOption(opts...)
	if len(keys) != len(readers) {
		return errors.New("keys and readers must have the same length")
	}
	if len(option.ContentTypes) > 0 && len(keys) != len(option.ContentTypes) {
		return errors.New("content_types and keys must have the same length")
	}

	bucket := lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)
	uploader := s3manager.NewUploaderWithClient(c.s3, func(u *s3manager.Uploader) {
		u.PartSize = c.cfg.UploadPartSize
		u.Concurrency = option.Concurrency
	})
	objects := lo.Map(keys, func(key string, i int) s3manager.BatchUploadObject {
		obj := s3manager.BatchUploadObject{
			Object: &s3manager.UploadInput{
				Bucket: lo.ToPtr(bucket),
				Key:    lo.ToPtr(key),
				Body:   readers[i],
			},
		}
		if contentType, err := lo.Nth(option.ContentTypes, i); err == nil {
			obj.Object.ContentType = lo.ToPtr(contentType)
		}
		return obj
	})

	iter := &s3manager.UploadObjectsIterator{Objects: objects}
	err := uploader.UploadWithIterator(ctx, iter)
	return errors.Wrapf(err, "batch upload objects")
}

func (c *S3Client) BatchRead(ctx context.Context, keys []string, opts ...DownloadOpt) ([]Reader, error) {
	option := NewDownloadOption(opts...)
	ctx, cancel := option.ContextWithTimeout(ctx)
	defer cancel()

	// prepare temp files
	files := make([]*os.File, 0, len(keys))
	for _, key := range keys {
		file, err := os.CreateTemp("", "s3-download-*.tmp")
		if err != nil {
			return nil, errors.WithMessagef(err, "create temp file for '%s'", key)
		}
		files = append(files, file)
	}

	writers := lo.Map(files, func(file *os.File, i int) io.WriterAt { return file })
	if err := c.batchDownload(ctx, keys, writers, option); err != nil {
		return nil, errors.Wrapf(err, "batch download objects")
	}

	readers := make([]Reader, 0, len(files))
	for _, file := range files {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, errors.WithMessagef(err, "seek to start of file '%s'", file.Name())
		}
		readers = append(readers, file)
	}
	return readers, nil
}

func (c *S3Client) BatchDownload(ctx context.Context, keys []string, writers []io.WriterAt, opts ...DownloadOpt) error {
	option := NewDownloadOption(opts...)
	ctx, cancel := option.ContextWithTimeout(ctx)
	defer cancel()

	return c.batchDownload(ctx, keys, writers, option)
}

func (c *S3Client) batchDownload(ctx context.Context, keys []string, writers []io.WriterAt, option *DownloadOption) error {
	if len(keys) != len(writers) {
		return errors.New("keys and writers must have the same length")
	}

	bucket := lo.CoalesceOrEmpty(option.Bucket, c.cfg.Bucket)
	concurrency := option.Concurrency
	if concurrency <= 0 {
		return errors.New("concurrency must be greater than 0")
	}

	downloader := s3manager.NewDownloaderWithClient(c.s3, func(d *s3manager.Downloader) {
		d.PartSize = c.cfg.DownloadPartSize
		d.Concurrency = concurrency
	})
	objects := lo.Map(keys, func(key string, i int) s3manager.BatchDownloadObject {
		return s3manager.BatchDownloadObject{
			Object: &s3.GetObjectInput{Bucket: lo.ToPtr(bucket), Key: lo.ToPtr(key)},
			Writer: writers[i],
		}
	})
	iter := &s3manager.DownloadObjectsIterator{Objects: objects}
	if err := downloader.DownloadWithIterator(ctx, iter); err != nil {
		return errors.Wrapf(err, "batch download objects")
	}
	return nil
}

func (c *S3Client) BatchSignDownloadReq(ctx context.Context, keys []string, opts ...SignOpt) ([]string, []http.Header, error) {
	urls := make([]string, 0, len(keys))
	headers := make([]http.Header, 0, len(keys))
	for _, key := range keys {
		url, header, err := c.SignDownloadReq(ctx, key, opts...)
		if err != nil {
			return nil, nil, errors.WithMessagef(err, "sign download request for '%s'", key)
		}
		urls = append(urls, url)
		headers = append(headers, header)
	}
	return urls, headers, nil
}

func (c *S3Client) BatchSignUploadReq(ctx context.Context, keys []string, opts ...SignOpt) ([]string, []http.Header, error) {
	urls := make([]string, 0, len(keys))
	headers := make([]http.Header, 0, len(keys))
	for _, key := range keys {
		url, header, err := c.SignUploadReq(ctx, key, opts...)
		if err != nil {
			return nil, nil, errors.WithMessagef(err, "sign upload request for '%s'", key)
		}
		urls = append(urls, url)
		headers = append(headers, header)
	}
	return urls, headers, nil
}
