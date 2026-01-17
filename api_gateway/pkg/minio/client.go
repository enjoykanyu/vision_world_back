package minio

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	client     *minio.Core
	bucketName string
	location   string
}

type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Location        string
}

func NewClient(cfg Config) (*Client, error) {
	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	client := &Client{
		client:     &minio.Core{Client: minioClient},
		bucketName: cfg.BucketName,
		location:   cfg.Location,
	}

	if err := client.ensureBucketExists(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	return client, nil
}

func (c *Client) ensureBucketExists() error {
	ctx := context.Background()

	exists, err := c.client.BucketExists(ctx, c.bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = c.client.MakeBucket(ctx, c.bucketName, minio.MakeBucketOptions{
			Region: c.location,
		})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		log.Printf("Created bucket: %s", c.bucketName)
	}

	return nil
}

func (c *Client) UploadFile(ctx context.Context, objectName string, filePath string, contentType string) (string, error) {
	if contentType == "" {
		ext := filepath.Ext(filePath)
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	info, err := c.client.FPutObject(ctx, c.bucketName, objectName, filePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)

	fileURL := fmt.Sprintf("%s/%s/%s", c.client.EndpointURL(), c.bucketName, objectName)
	return fileURL, nil
}

func (c *Client) UploadFileFromReader(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	info, err := c.client.PutObject(ctx, c.bucketName, objectName, reader, size, "", "", minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file from reader: %w", err)
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)

	fileURL := fmt.Sprintf("%s/%s/%s", c.client.EndpointURL(), c.bucketName, objectName)
	return fileURL, nil
}

func (c *Client) GeneratePresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	presignedURL, err := c.client.PresignedGetObject(ctx, c.bucketName, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

func (c *Client) DeleteFile(ctx context.Context, objectName string) error {
	err := c.client.RemoveObject(ctx, c.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	log.Printf("Successfully deleted %s\n", objectName)
	return nil
}

func (c *Client) GetFileInfo(ctx context.Context, objectName string) (*minio.ObjectInfo, error) {
	info, err := c.client.StatObject(ctx, c.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &info, nil
}

func (c *Client) GetClient() *minio.Core {
	return c.client
}

func (c *Client) NewMultipartUpload(ctx context.Context, objectName string, opts minio.PutObjectOptions) (string, error) {
	uploadID, err := c.client.NewMultipartUpload(ctx, c.bucketName, objectName, opts)
	if err != nil {
		return "", fmt.Errorf("failed to create multipart upload: %w", err)
	}

	return uploadID, nil
}

func (c *Client) PutObjectPart(ctx context.Context, objectName, uploadID string, partNumber int, reader io.Reader, size int64, opts minio.PutObjectPartOptions) (minio.CompletePart, error) {
	part, err := c.client.PutObjectPart(ctx, c.bucketName, objectName, uploadID, partNumber, reader, size, opts)
	if err != nil {
		return minio.CompletePart{}, fmt.Errorf("failed to put object part: %w", err)
	}

	return minio.CompletePart{
		PartNumber: part.PartNumber,
		ETag:       part.ETag,
	}, nil
}

func (c *Client) ListParts(ctx context.Context, objectName, uploadID string, maxParts int, partNumberMarker int) ([]minio.CompletePart, error) {
	result, err := c.client.ListObjectParts(ctx, c.bucketName, objectName, uploadID, partNumberMarker, maxParts)
	if err != nil {
		return nil, fmt.Errorf("failed to list parts: %w", err)
	}

	parts := make([]minio.CompletePart, len(result.ObjectParts))
	for i, part := range result.ObjectParts {
		parts[i] = minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}

	return parts, nil
}

func (c *Client) CompleteMultipartUpload(ctx context.Context, objectName, uploadID string, parts []minio.CompletePart) (*minio.UploadInfo, error) {
	uploadInfo, err := c.client.CompleteMultipartUpload(ctx, c.bucketName, objectName, uploadID, parts, minio.PutObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return &uploadInfo, nil
}

func (c *Client) AbortMultipartUpload(ctx context.Context, objectName, uploadID string) error {
	err := c.client.AbortMultipartUpload(ctx, c.bucketName, objectName, uploadID)
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}

	return nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) GetFile(ctx context.Context, objectName string) (io.ReadCloser, error) {
	object, _, _, err := c.client.GetObject(ctx, c.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return object, nil
}
