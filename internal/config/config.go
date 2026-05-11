package config

import (
	"fmt"
	"net/url"
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
	Clean        bool
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

	if c.Locale == "" {
		if v := os.Getenv("CUBBIT_LOCALE"); v != "" {
			c.Locale = v
		} else if fileCfg != nil && fileCfg.Locale != "" {
			c.Locale = fileCfg.Locale
		} else {
			c.Locale = "en"
		}
	}

	if c.Password == "" {
		if v := os.Getenv("CUBBIT_PASSWORD"); v != "" {
			c.Password = v
		}
	}

	return c.Validate()
}

// Validate checks that required fields are set.
func (c *Config) Validate() error {
	if c.Bucket == "" {
		return fmt.Errorf("bucket is required — use --bucket, CUBBIT_BUCKET, or run `cubbit-pages setup`")
	}
	if c.AccessKey == "" {
		return fmt.Errorf("access key is required — use --access-key, CUBBIT_ACCESS_KEY, or run `cubbit-pages setup`")
	}
	if c.SecretKey == "" {
		return fmt.Errorf("secret key is required — use --secret-key, CUBBIT_SECRET_KEY, or run `cubbit-pages setup`")
	}
	if c.Encrypt && c.Password == "" {
		return fmt.Errorf("encryption password is required — use --password, CUBBIT_PASSWORD, or omit --password to be prompted interactively")
	}
	return nil
}

// ResolveOpen fills only bucket and endpoint — the two fields needed by the open
// command to build a URL. Does not require or validate credentials.
func (c *Config) ResolveOpen() {
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

	if c.Bucket == "" {
		if v := os.Getenv("CUBBIT_BUCKET"); v != "" {
			c.Bucket = v
		} else if fileCfg != nil {
			c.Bucket = fileCfg.Bucket
		}
	}

	// Fall back to the prefix of the last successful deploy when none is given.
	if c.Prefix == "" && fileCfg != nil && fileCfg.LastDeploy != nil {
		c.Prefix = fileCfg.LastDeploy.Prefix
	}

	c.Prefix = strings.Trim(c.Prefix, "/")
}

// SiteURL returns the public URL of the deployed site using virtual-hosted style.
func (c *Config) SiteURL() string {
	prefix := ""
	if c.Prefix != "" {
		prefix = c.Prefix + "/"
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Host == "" || u.Port() != "" {
		// Explicit port → local/custom endpoint (e.g. MinIO) — use path-style
		return fmt.Sprintf("%s/%s/%sindex.html", c.Endpoint, c.Bucket, prefix)
	}
	return fmt.Sprintf("%s://%s.%s/%sindex.html", u.Scheme, c.Bucket, u.Host, prefix)
}
