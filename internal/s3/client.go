package s3

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Client wraps the S3 client with Cubbit-specific configuration.
type Client struct {
	S3     *s3.Client
	Bucket string
}

// NewClient creates a new S3 client configured for Cubbit.
func NewClient(endpoint, accessKey, secretKey, region, bucket string) (*Client, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, fmt.Errorf("endpoint, accessKey, secretKey, and bucket are required")
	}

	if region == "" {
		region = "eu-west-1"
	}

	s3Client := s3.New(s3.Options{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		),
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true, // OBBLIGATORIO per Cubbit — non rimuovere mai
	})

	return &Client{
		S3:     s3Client,
		Bucket: bucket,
	}, nil
}

// HeadBucket checks if the bucket exists and is accessible.
func (c *Client) HeadBucket(ctx context.Context) error {
	_, err := c.S3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.Bucket),
	})
	if err != nil {
		return fmt.Errorf("checking bucket %q: %w", c.Bucket, err)
	}
	return nil
}

// BucketStatus describes the outcome of a HeadBucket probe.
type BucketStatus int

const (
	BucketExists    BucketStatus = iota // exists and belongs to us
	BucketForbidden                     // exists but owned by someone else
	BucketNotFound                      // does not exist
)

// ProbeBucket checks whether a bucket exists and whether it is ours.
func (c *Client) ProbeBucket(ctx context.Context, name string) BucketStatus {
	_, err := c.S3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(name)})
	if err == nil {
		return BucketExists
	}
	es := err.Error()
	if strings.Contains(es, "403") || strings.Contains(es, "Forbidden") {
		return BucketForbidden
	}
	return BucketNotFound
}

// CreateBucket creates a new bucket in the given region.
func (c *Client) CreateBucket(ctx context.Context, name, region string) error {
	_, err := c.S3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
		CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		},
	})
	if err != nil {
		return fmt.Errorf("creating bucket %q: %w", name, err)
	}
	return nil
}
