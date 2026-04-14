package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
