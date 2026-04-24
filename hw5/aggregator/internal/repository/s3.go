package repository

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Repo struct {
	client *minio.Client
	bucket string
	region string
}

func NewS3Repo(endpoint, accessKey, secretKey, bucket, region string, useSSL bool) (*S3Repo, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}

	return &S3Repo{client: client, bucket: bucket, region: region}, nil
}

func (r *S3Repo) EnsureBucket(ctx context.Context) error {
	exists, err := r.client.BucketExists(ctx, r.bucket)
	if err != nil {
		return fmt.Errorf("check bucket %s: %w", r.bucket, err)
	}
	if exists {
		return nil
	}

	if err := r.client.MakeBucket(ctx, r.bucket, minio.MakeBucketOptions{Region: r.region}); err != nil {
		alreadyExists := strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") || strings.Contains(err.Error(), "BucketAlreadyExists")
		if !alreadyExists {
			return fmt.Errorf("create bucket %s: %w", r.bucket, err)
		}
	}

	return nil
}

func (r *S3Repo) PutObject(ctx context.Context, key string, body []byte, contentType string) (string, error) {
	result, err := r.client.PutObject(
		ctx,
		r.bucket,
		key,
		bytes.NewReader(body),
		int64(len(body)),
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return "", fmt.Errorf("put object %s: %w", key, err)
	}
	return result.ETag, nil
}

func (r *S3Repo) Bucket() string {
	return r.bucket
}

