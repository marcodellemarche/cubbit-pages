package s3

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Uploader handles uploading files to S3.
type Uploader struct {
	client      *Client
	publicACL   bool
	prefix      string
}

// NewUploader creates a new Uploader.
func NewUploader(client *Client, publicACL bool, prefix string) *Uploader {
	return &Uploader{
		client:    client,
		publicACL: publicACL,
		prefix:    prefix,
	}
}

// Upload uploads a single file to S3.
func (u *Uploader) Upload(ctx context.Context, key string, data []byte) error {
	fullKey := key
	if u.prefix != "" {
		fullKey = u.prefix + "/" + key
	}

	contentType := detectContentType(fullKey)

	input := &s3.PutObjectInput{
		Bucket:             aws.String(u.client.Bucket),
		Key:                aws.String(fullKey),
		Body:               bytes.NewReader(data),
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String("inline"),
	}

	_, err := u.client.S3.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("uploading %q: %w", fullKey, err)
	}

	if u.publicACL {
		_, err = u.client.S3.PutObjectAcl(ctx, &s3.PutObjectAclInput{
			Bucket: aws.String(u.client.Bucket),
			Key:    aws.String(fullKey),
			ACL:    types.ObjectCannedACLPublicRead,
		})
		if err != nil {
			return fmt.Errorf("setting ACL on %q: %w", fullKey, err)
		}
	}

	return nil
}

// detectContentType returns the MIME type for a file based on its extension.
func detectContentType(filename string) string {
	// Handle .enc files — serve as binary
	if strings.HasSuffix(filename, ".enc") {
		return "application/octet-stream"
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		return "application/octet-stream"
	}

	// Custom mappings for common web types
	customTypes := map[string]string{
		".html": "text/html; charset=utf-8",
		".css":  "text/css; charset=utf-8",
		".js":   "application/javascript",
		".json": "application/json",
		".svg":  "image/svg+xml",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".ico":  "image/x-icon",
		".woff": "font/woff",
		".woff2": "font/woff2",
		".ttf":  "font/ttf",
		".txt":  "text/plain; charset=utf-8",
		".xml":  "application/xml",
		".pdf":  "application/pdf",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".mp3":  "audio/mpeg",
	}

	if ct, ok := customTypes[strings.ToLower(ext)]; ok {
		return ct
	}

	ct := mime.TypeByExtension(ext)
	if ct != "" {
		return ct
	}

	return "application/octet-stream"
}
