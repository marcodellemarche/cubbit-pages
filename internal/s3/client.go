package s3

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// DeployInfo holds discovered metadata about a deploy found in the bucket.
type DeployInfo struct {
	Prefix      string
	Encrypted   bool
	Locale      string
	Version     string
	Timestamp   time.Time
	FileCount   int
	TotalSize   int64
	URL         string
	HasMetadata bool
}

// ObjectInfo holds metadata for a single S3 object.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

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

// ListObjects returns all objects in the bucket, optionally filtered by prefix.
func (c *Client) ListObjects(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            aws.String(c.Bucket),
			ContinuationToken: continuationToken,
		}
		if prefix != "" {
			input.Prefix = aws.String(prefix)
		}

		out, err := c.S3.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("listing objects in %q: %w", c.Bucket, err)
		}

		for _, obj := range out.Contents {
			info := ObjectInfo{Key: aws.ToString(obj.Key)}
			if obj.Size != nil {
				info.Size = *obj.Size
			}
			if obj.LastModified != nil {
				info.LastModified = *obj.LastModified
			}
			objects = append(objects, info)
		}

		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	return objects, nil
}

// DiscoverDeploys scans the bucket for deploy entry points (index.html files)
// and reads their cubbit-pages metadata to build an inventory of all deploys.
func (c *Client) DiscoverDeploys(ctx context.Context, endpoint string) ([]DeployInfo, error) {
	objects, err := c.ListObjects(ctx, "")
	if err != nil {
		return nil, err
	}

	// Find all prefixes that have an index.html (deploy entry points).
	prefixes := map[string]bool{}
	for _, obj := range objects {
		if obj.Key == "index.html" {
			prefixes[""] = true
		} else if strings.HasSuffix(obj.Key, "/index.html") {
			prefixes[strings.TrimSuffix(obj.Key, "/index.html")] = true
		}
	}
	if len(prefixes) == 0 {
		return nil, nil
	}

	// Count files and sizes per prefix.
	type stats struct {
		count int
		size  int64
	}
	prefixStats := map[string]stats{}
	for _, obj := range objects {
		assigned := false
		for pfx := range prefixes {
			if pfx == "" {
				continue
			}
			if strings.HasPrefix(obj.Key, pfx+"/") {
				s := prefixStats[pfx]
				s.count++
				s.size += obj.Size
				prefixStats[pfx] = s
				assigned = true
				break
			}
		}
		if !assigned && prefixes[""] {
			// Assign to root only if not claimed by a named prefix.
			belongsToRoot := true
			for pfx := range prefixes {
				if pfx != "" && strings.HasPrefix(obj.Key, pfx+"/") {
					belongsToRoot = false
					break
				}
			}
			if belongsToRoot {
				s := prefixStats[""]
				s.count++
				s.size += obj.Size
				prefixStats[""] = s
			}
		}
	}

	// HeadObject on each index.html to retrieve deploy metadata.
	var deploys []DeployInfo
	for pfx := range prefixes {
		indexKey := "index.html"
		if pfx != "" {
			indexKey = pfx + "/index.html"
		}

		info := DeployInfo{
			Prefix: pfx,
			URL:    buildSiteURL(endpoint, c.Bucket, pfx),
		}
		if st, ok := prefixStats[pfx]; ok {
			info.FileCount = st.count
			info.TotalSize = st.size
		}

		head, err := c.S3.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(c.Bucket),
			Key:    aws.String(indexKey),
		})
		if err == nil {
			if v := head.Metadata["cubbit-pages-encrypted"]; v == "true" {
				info.Encrypted = true
				info.HasMetadata = true
			} else if _, ok := head.Metadata["cubbit-pages-timestamp"]; ok {
				info.HasMetadata = true
			}
			if v := head.Metadata["cubbit-pages-locale"]; v != "" {
				info.Locale = v
			}
			if v := head.Metadata["cubbit-pages-version"]; v != "" {
				info.Version = v
			}
			if v := head.Metadata["cubbit-pages-timestamp"]; v != "" {
				info.Timestamp, _ = time.Parse(time.RFC3339, v)
			}
			if info.Timestamp.IsZero() && head.LastModified != nil {
				info.Timestamp = *head.LastModified
			}
		} else {
			// Fallback: use LastModified from ListObjects.
			for _, obj := range objects {
				if obj.Key == indexKey {
					info.Timestamp = obj.LastModified
					break
				}
			}
		}

		deploys = append(deploys, info)
	}

	sort.Slice(deploys, func(i, j int) bool {
		return deploys[i].Timestamp.After(deploys[j].Timestamp)
	})

	return deploys, nil
}

func buildSiteURL(endpoint, bucket, prefix string) string {
	pfx := ""
	if prefix != "" {
		pfx = prefix + "/"
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.Port() != "" {
		return fmt.Sprintf("%s/%s/%sindex.html", endpoint, bucket, pfx)
	}
	return fmt.Sprintf("%s://%s.%s/%sindex.html", u.Scheme, bucket, u.Host, pfx)
}

// DeleteObjects deletes the given keys from the bucket in batches of 1000.
func (c *Client) DeleteObjects(ctx context.Context, keys []string) error {
	const batchSize = 1000
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		identifiers := make([]s3types.ObjectIdentifier, len(keys[i:end]))
		for j, key := range keys[i:end] {
			identifiers[j] = s3types.ObjectIdentifier{Key: aws.String(key)}
		}

		_, err := c.S3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(c.Bucket),
			Delete: &s3types.Delete{
				Objects: identifiers,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("deleting objects: %w", err)
		}
	}
	return nil
}
