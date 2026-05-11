package config

import (
	"fmt"
	"os"
	"strings"
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
	Locale       string
}

// Resolve fills in missing config values from (lowest to highest priority):
// config file → environment variables → CLI flags (already set on c).
func (c *Config) Resolve() error {
	// Load file config as base
	fileCfg, _ := LoadFileConfig()

	if c.Endpoint == "" {
		if v := os.Getenv("CUBBIT_ENDPOINT"); v != "" {
			c.Endpoint = v
		} else if fileCfg != nil && fileCfg.Endpoint != "" {
			c.Endpoint = fileCfg.Endpoint
		} else {
			c.Endpoint = DefaultEndpoint
		}
	}

	if c.AccessKey == "" {
		if v := os.Getenv("CUBBIT_ACCESS_KEY"); v != "" {
			c.AccessKey = v
		} else if fileCfg != nil {
			c.AccessKey = fileCfg.AccessKey
		}
	}

	if c.SecretKey == "" {
		if v := os.Getenv("CUBBIT_SECRET_KEY"); v != "" {
			c.SecretKey = v
		} else if fileCfg != nil {
			c.SecretKey = fileCfg.SecretKey
		}
	}

	if c.Bucket == "" {
		if v := os.Getenv("CUBBIT_BUCKET"); v != "" {
			c.Bucket = v
		} else if fileCfg != nil {
			c.Bucket = fileCfg.Bucket
		}
	}

	if c.Concurrency <= 0 {
		c.Concurrency = DefaultConcurrency
	}

	if c.Region == "" {
		c.Region = DefaultRegion
	}

	// Normalize the prefix to a canonical form so downstream consumers can join it
	// with a constant separator. Without this, a user-supplied "foo/" combined with
	// the uploader's `prefix + "/" + key` join produces "foo//key" (double slash) —
	// a distinct, unreachable S3 key from the "foo/key" the published URL points to.
	c.Prefix = strings.Trim(c.Prefix, "/")

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
