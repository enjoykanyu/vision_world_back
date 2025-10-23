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

// Client MinIO客户端
type Client struct {
	client     *minio.Client
	bucketName string
	location   string
}

// Config MinIO配置
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Location        string
}

// NewClient 创建新的MinIO客户端
func NewClient(cfg Config) (*Client, error) {
	// 初始化MinIO客户端
	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	client := &Client{
		client:     minioClient,
		bucketName: cfg.BucketName,
		location:   cfg.Location,
	}

	// 检查并创建bucket
	if err := client.ensureBucketExists(); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	return client, nil
}

// ensureBucketExists 确保bucket存在
func (c *Client) ensureBucketExists() error {
	ctx := context.Background()

	// 检查bucket是否存在
	exists, err := c.client.BucketExists(ctx, c.bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	// 如果bucket不存在，创建它
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

// UploadFile 上传文件到MinIO
func (c *Client) UploadFile(ctx context.Context, objectName string, filePath string, contentType string) (string, error) {
	// 如果未指定content type，尝试自动检测
	if contentType == "" {
		ext := filepath.Ext(filePath)
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	// 上传文件
	info, err := c.client.FPutObject(ctx, c.bucketName, objectName, filePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)

	// 返回文件URL
	fileURL := fmt.Sprintf("%s/%s/%s", c.client.EndpointURL(), c.bucketName, objectName)
	return fileURL, nil
}

// UploadFileFromReader 从reader上传文件到MinIO
func (c *Client) UploadFileFromReader(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	// 上传文件
	info, err := c.client.PutObject(ctx, c.bucketName, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file from reader: %w", err)
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)

	// 返回文件URL
	fileURL := fmt.Sprintf("%s/%s/%s", c.client.EndpointURL(), c.bucketName, objectName)
	return fileURL, nil
}

// GeneratePresignedURL 生成预签名URL
func (c *Client) GeneratePresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	// 生成预签名URL
	presignedURL, err := c.client.PresignedGetObject(ctx, c.bucketName, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

// DeleteFile 删除文件
func (c *Client) DeleteFile(ctx context.Context, objectName string) error {
	err := c.client.RemoveObject(ctx, c.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	log.Printf("Successfully deleted %s\n", objectName)
	return nil
}

// GetFileInfo 获取文件信息
func (c *Client) GetFileInfo(ctx context.Context, objectName string) (*minio.ObjectInfo, error) {
	info, err := c.client.StatObject(ctx, c.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &info, nil
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	// MinIO客户端没有显式的关闭方法
	return nil
}
