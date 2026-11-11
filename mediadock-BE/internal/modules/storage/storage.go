package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mediadock/backend/internal/config"
	"github.com/mediadock/backend/internal/utils"
)

var (
	ErrObjectNotFound  = errors.New("stored object not found")
	ErrInvalidSigned   = errors.New("invalid or expired signed download URL")
	ErrUnsupportedType = errors.New("unsupported storage type")
)

// Storage abstracts local development storage and S3-compatible production storage.
type Storage interface {
	SaveFile(ctx context.Context, key string, localPath string, contentType string) (string, error)
	Open(ctx context.Context, key string) (io.ReadCloser, int64, error)
	Delete(ctx context.Context, key string) error
	SignedURL(ctx context.Context, key string, expiresIn time.Duration) (string, error)
	TempDir() string
}

// NewFromConfig instantiates the configured Storage backend ("local" or "s3").
func NewFromConfig(ctx context.Context, cfg *config.Config) (Storage, error) {
	switch cfg.StorageType {
	case "local":
		return NewLocalStorage(cfg.StoragePath, cfg.LocalSigningSecret)
	case "s3":
		return NewS3Storage(ctx, cfg)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, cfg.StorageType)
	}
}

// LocalStorage stores media artifacts on local disk under a controlled base directory.
type LocalStorage struct {
	baseDir       string
	tempDir       string
	signingSecret []byte
}

// NewLocalStorage initializes local artifact and temp directories.
func NewLocalStorage(baseDir, signingSecret string) (*LocalStorage, error) {
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return nil, err
	}
	tempDir := filepath.Join(absBase, "tmp")
	if err := os.MkdirAll(absBase, 0o750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return nil, err
	}
	return &LocalStorage{
		baseDir:       absBase,
		tempDir:       tempDir,
		signingSecret: []byte(signingSecret),
	}, nil
}

func (l *LocalStorage) TempDir() string {
	return l.tempDir
}

func (l *LocalStorage) SaveFile(_ context.Context, key string, localPath string, _ string) (string, error) {
	dest, err := utils.SafeJoinPath(l.baseDir, key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return "", err
	}

	src, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return key, nil
}

func (l *LocalStorage) Open(_ context.Context, key string) (io.ReadCloser, int64, error) {
	fullPath, err := utils.SafeJoinPath(l.baseDir, key)
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, ErrObjectNotFound
		}
		return nil, 0, err
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, err
	}
	return f, info.Size(), nil
}

func (l *LocalStorage) Delete(_ context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	fullPath, err := utils.SafeJoinPath(l.baseDir, key)
	if err != nil {
		return err
	}
	err = os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SignedURL generates an HMAC-SHA256 signed query string for local authenticated downloads.
func (l *LocalStorage) SignedURL(_ context.Context, key string, expiresIn time.Duration) (string, error) {
	exp := time.Now().UTC().Add(expiresIn).Unix()
	sig := l.sign(key, exp)
	q := url.Values{}
	q.Set("exp", strconv.FormatInt(exp, 10))
	q.Set("sig", sig)
	return "?" + q.Encode(), nil
}

// VerifySignedToken validates an HMAC signature and expiry timestamp for a storage key.
func (l *LocalStorage) VerifySignedToken(key string, expStr, sig string) error {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().UTC().Unix() > exp {
		return ErrInvalidSigned
	}
	expected := l.sign(key, exp)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return ErrInvalidSigned
	}
	return nil
}

func (l *LocalStorage) sign(key string, exp int64) string {
	mac := hmac.New(sha256.New, l.signingSecret)
	mac.Write([]byte(fmt.Sprintf("%s:%d", key, exp)))
	return hex.EncodeToString(mac.Sum(nil))
}

// S3Storage stores media artifacts in an S3-compatible object store.
type S3Storage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	tempDir   string
}

// NewS3Storage initializes an AWS SDK v2 S3 storage backend.
func NewS3Storage(ctx context.Context, cfg *config.Config) (*S3Storage, error) {
	tempDir := filepath.Join(os.TempDir(), "mediadock-tmp")
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return nil, err
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.S3Region),
	}
	if cfg.S3AccessKey != "" && cfg.S3SecretKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = true
		}
	})

	return &S3Storage{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.S3Bucket,
		tempDir:   tempDir,
	}, nil
}

func (s *S3Storage) TempDir() string {
	return s.tempDir
}

func (s *S3Storage) SaveFile(ctx context.Context, key string, localPath string, contentType string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   f,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if _, err := s.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("s3 PutObject: %w", err)
	}
	return key, nil
}

func (s *S3Storage) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, err
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, size, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Storage) SignedURL(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	res, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", err
	}
	return res.URL, nil
}

