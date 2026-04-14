package config

import (
	"fmt"
	"os"
)

const (
	DefaultEndpoint    = "https://s3.cubbit.eu"
	DefaultConcurrency = 5
	DefaultRegion      = "eu-west-1"
)

// Config holds all configuration for a deploy operation.
type Config struct {
	Bucket       string
	AccessKey    string
	SecretKey    string
	Endpoint     string
	Region       string
	Encrypt      bool
	Password     string
	PublicBucket bool
	DryRun       bool
	Concurrency  int
	Prefix       string
	SourceDir    string
}

// Resolve fills in missing config values from environment variables.
func (c *Config) Resolve() error {
	if c.Endpoint == "" {
		c.Endpoint = os.Getenv("CUBBIT_ENDPOINT")
	}
	if c.Endpoint == "" {
		c.Endpoint = DefaultEndpoint
	}

	if c.AccessKey == "" {
		c.AccessKey = os.Getenv("CUBBIT_ACCESS_KEY")
	}

	if c.SecretKey == "" {
		c.SecretKey = os.Getenv("CUBBIT_SECRET_KEY")
	}

	if c.Bucket == "" {
		c.Bucket = os.Getenv("CUBBIT_BUCKET")
	}

	if c.Concurrency <= 0 {
		c.Concurrency = DefaultConcurrency
	}

	if c.Region == "" {
		c.Region = DefaultRegion
	}

	return c.Validate()
}

// Validate checks that required fields are set.
func (c *Config) Validate() error {
	if c.Bucket == "" {
		return fmt.Errorf("bucket is required (--bucket or CUBBIT_BUCKET)")
	}
	if c.AccessKey == "" {
		return fmt.Errorf("access key is required (--access-key or CUBBIT_ACCESS_KEY)")
	}
	if c.SecretKey == "" {
		return fmt.Errorf("secret key is required (--secret-key or CUBBIT_SECRET_KEY)")
	}
	if c.Encrypt && c.Password == "" {
		return fmt.Errorf("password is required when --encrypt is set")
	}
	if c.SourceDir == "" {
		return fmt.Errorf("source directory is required")
	}
	return nil
}

// SiteURL returns the public URL of the deployed site.
func (c *Config) SiteURL() string {
	prefix := ""
	if c.Prefix != "" {
		prefix = c.Prefix + "/"
	}
	return fmt.Sprintf("%s/%s/%sindex.html", c.Endpoint, c.Bucket, prefix)
}
