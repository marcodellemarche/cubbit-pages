package snippets

import (
	"strings"
	"testing"
)

func TestBucketPolicyContainsBucketName(t *testing.T) {
	policy := BucketPolicy("my-test-bucket")
	if !strings.Contains(policy, "my-test-bucket") {
		t.Fatal("bucket policy does not contain bucket name")
	}
	if !strings.Contains(policy, "s3:GetObject") {
		t.Fatal("bucket policy does not contain s3:GetObject")
	}
}

func TestIAMPolicyContainsRequiredActions(t *testing.T) {
	policy := IAMPolicy("deploy-bucket")
	if !strings.Contains(policy, "s3:PutObject") {
		t.Fatal("IAM policy missing s3:PutObject")
	}
	if !strings.Contains(policy, "s3:PutObjectAcl") {
		t.Fatal("IAM policy missing s3:PutObjectAcl")
	}
	if !strings.Contains(policy, "deploy-bucket") {
		t.Fatal("IAM policy missing bucket name")
	}
}

func TestLifecycleCLIContainsWarning(t *testing.T) {
	snippet := LifecycleCLI("my-bucket", 30)
	if !strings.Contains(snippet, "dedicato esclusivamente") {
		t.Fatal("lifecycle snippet missing dedication warning")
	}
}

func TestLifecycleCLIContainsDays(t *testing.T) {
	snippet := LifecycleCLI("my-bucket", 90)
	if !strings.Contains(snippet, "90") {
		t.Fatal("lifecycle snippet missing days count")
	}
}

func TestCORSConfigurationValid(t *testing.T) {
	cors := CORSConfiguration()
	if !strings.Contains(cors, "AllowedOrigins") {
		t.Fatal("CORS missing AllowedOrigins")
	}
	if !strings.Contains(cors, "GET") {
		t.Fatal("CORS missing GET method")
	}
}

func TestAllSnippetsContainsAllSections(t *testing.T) {
	all := AllSnippets("test-bucket")
	sections := []string{"Bucket Policy", "CORS", "IAM Policy", "Lifecycle"}
	for _, s := range sections {
		if !strings.Contains(all, s) {
			t.Fatalf("AllSnippets missing section: %s", s)
		}
	}
}
