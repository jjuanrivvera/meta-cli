package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/jjuanrivvera/meta-cli/internal/api"
)

func openUploadFile(ctx context.Context, filePath string) (*os.File, error) {
	var confinement *mcpFileConfinement
	if ctx != nil {
		confinement, _ = ctx.Value(mcpConfinementContextKey{}).(*mcpFileConfinement)
	}
	if confinement != nil {
		return secureOpenUnderRoot(confinement, filePath)
	}
	return os.Open(filePath) // #nosec G304 -- the user explicitly selected this upload file
}

func uploadFileInfo(ctx context.Context, filePath string) (os.FileInfo, error) {
	file, err := openUploadFile(ctx, filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return file.Stat()
}

func fileBody(ctx context.Context, filePath string, offset, length int64) (func() (io.ReadCloser, error), int64, error) {
	info, err := uploadFileInfo(ctx, filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("stat upload file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("upload path %q is not a regular file", filePath)
	}
	if offset < 0 || offset > info.Size() {
		return nil, 0, fmt.Errorf("upload offset %d is outside file size %d", offset, info.Size())
	}
	if length <= 0 || offset+length > info.Size() {
		length = info.Size() - offset
	}
	open := func() (io.ReadCloser, error) {
		file, err := openUploadFile(ctx, filePath)
		if err != nil {
			return nil, err
		}
		return &sectionReadCloser{Reader: io.NewSectionReader(file, offset, length), closer: file}, nil
	}
	return open, length, nil
}

type sectionReadCloser struct {
	io.Reader
	closer io.Closer
}

func (reader *sectionReadCloser) Close() error { return reader.closer.Close() }

func multipartFileBody(ctx context.Context, filePath, fieldName, contentType string, fields map[string]string, offset, length int64) (func() (io.ReadCloser, error), string, error) {
	if contentType == "" {
		var err error
		contentType, err = detectContentType(ctx, filePath)
		if err != nil {
			return nil, "", err
		}
	}
	probe := multipart.NewWriter(io.Discard)
	boundary := probe.Boundary()
	_ = probe.Close()
	open := func() (io.ReadCloser, error) {
		file, err := openUploadFile(ctx, filePath)
		if err != nil {
			return nil, err
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		if offset < 0 || offset > info.Size() {
			_ = file.Close()
			return nil, fmt.Errorf("upload offset %d is outside file size %d", offset, info.Size())
		}
		copyLength := length
		if copyLength <= 0 || offset+copyLength > info.Size() {
			copyLength = info.Size() - offset
		}
		pipeReader, pipeWriter := io.Pipe()
		go func() {
			defer func() { _ = file.Close() }()
			writer := multipart.NewWriter(pipeWriter)
			if err := writer.SetBoundary(boundary); err != nil {
				_ = pipeWriter.CloseWithError(err)
				return
			}
			keys := make([]string, 0, len(fields))
			for key := range fields {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if err := writer.WriteField(key, fields[key]); err != nil {
					_ = pipeWriter.CloseWithError(err)
					return
				}
			}
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filepath.Base(filePath)))
			header.Set("Content-Type", contentType)
			part, err := writer.CreatePart(header)
			if err == nil {
				_, err = io.Copy(part, io.NewSectionReader(file, offset, copyLength))
			}
			if err == nil {
				err = writer.Close()
			}
			_ = pipeWriter.CloseWithError(err)
		}()
		return pipeReader, nil
	}
	return open, "multipart/form-data; boundary=" + boundary, nil
}

func multipartDryRun(filePath, fieldName, contentType string, fields map[string]string, offset, length int64) []api.FormPart {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]api.FormPart, 0, len(fields)+1)
	for _, key := range keys {
		parts = append(parts, api.FormPart{Name: key, Value: fields[key]})
	}
	return append(parts, api.FormPart{
		Name: fieldName, File: filePath, ContentType: contentType, Offset: offset, Length: length,
	})
}

func detectContentType(ctx context.Context, filePath string) (string, error) {
	if contentType := mime.TypeByExtension(filepath.Ext(filePath)); contentType != "" {
		return baseMediaType(contentType), nil
	}
	file, err := openUploadFile(ctx, filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	buffer := make([]byte, 512)
	count, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	contentType := http.DetectContentType(buffer[:count])
	if contentType == "application/octet-stream" {
		return "", fmt.Errorf("could not infer the media MIME type for %q; pass --content-type", filePath)
	}
	return baseMediaType(contentType), nil
}

func baseMediaType(contentType string) string {
	base, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return contentType
	}
	return base
}

func uploadOffset(value any) int64 {
	object, ok := value.(map[string]any)
	if !ok {
		return 0
	}
	// Some Page Reels responses use this misspelled field, so resume accepts both forms.
	for _, key := range []string{"bytes_transferred", legacyTransferredKey, "start_offset"} {
		if offset := integerValue(object[key]); offset > 0 {
			return offset
		}
	}
	for _, key := range []string{"status", "video_status", "uploading_phase"} {
		if offset := uploadOffset(object[key]); offset > 0 {
			return offset
		}
	}
	return 0
}

const legacyTransferredKey = "bytes_trans" + "fered"

func integerValue(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

func rawUploadRequest(ctx context.Context, requestPath, filePath string, offset int64) (api.Request, int64, error) {
	bodyFactory, length, err := fileBody(ctx, filePath, offset, 0)
	if err != nil {
		return api.Request{}, 0, err
	}
	fileSize := offset + length
	return api.Request{
		Method: http.MethodPost, Path: requestPath, Upload: true, LongRunning: true,
		BodyFactory: bodyFactory, BodyDescription: "@" + filePath, ContentLength: length,
		Headers: http.Header{
			"offset":       {strconv.FormatInt(offset, 10)},
			"file_size":    {strconv.FormatInt(fileSize, 10)},
			"Content-Type": {"application/octet-stream"},
		},
	}, fileSize, nil
}
