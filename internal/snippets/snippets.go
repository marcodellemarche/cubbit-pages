package snippets

import "fmt"

// BucketPolicy returns the JSON bucket policy for public read access.
func BucketPolicy(bucket string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": "*",
    "Action": "s3:GetObject",
    "Resource": "arn:aws:s3:::%s/*"
  }]
}`, bucket)
}

// BucketPolicyCLI returns the AWS CLI command to apply the bucket policy.
func BucketPolicyCLI(bucket string) string {
	policy := BucketPolicy(bucket)
	return fmt.Sprintf(`aws s3api put-bucket-policy \
  --endpoint-url https://s3.cubbit.eu \
  --bucket %s \
  --policy '%s'`, bucket, policy)
}

// CORSConfiguration returns the CORS JSON for encrypted site fetch() calls.
func CORSConfiguration() string {
	return `{
  "CORSRules": [{
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET"],
    "AllowedHeaders": ["*"]
  }]
}`
}

// CORSCLI returns the AWS CLI command to apply CORS configuration.
func CORSCLI(bucket string) string {
	cors := CORSConfiguration()
	return fmt.Sprintf(`aws s3api put-bucket-cors \
  --endpoint-url https://s3.cubbit.eu \
  --bucket %s \
  --cors-configuration '%s'`, bucket, cors)
}

// IAMPolicy returns the minimal IAM policy JSON for deploying.
func IAMPolicy(bucket string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["s3:PutObject", "s3:PutObjectAcl", "s3:DeleteObject"],
    "Resource": "arn:aws:s3:::%s/*"
  }]
}`, bucket)
}

// LifecycleCLI returns the AWS CLI command for setting a lifecycle policy.
func LifecycleCLI(bucket string, days int) string {
	return fmt.Sprintf(`# ATTENZIONE: Applica solo se questo bucket è dedicato esclusivamente a Cubbit Pages.
# Questa policy cancellerà TUTTI gli oggetti nel bucket dopo %d giorni.

aws s3api put-bucket-lifecycle-configuration \
  --endpoint-url https://s3.cubbit.eu \
  --bucket %s \
  --lifecycle-configuration '{
    "Rules": [{
      "ID": "cubbitpages-expiry",
      "Status": "Enabled",
      "Expiration": {"Days": %d}
    }]
  }'`, days, bucket, days)
}

// AllSnippets returns all snippets formatted for display.
func AllSnippets(bucket string) string {
	return fmt.Sprintf(`═══ Bucket Policy (accesso pubblico in lettura) ═══

%s

═══ CORS (necessario per siti cifrati) ═══

%s

═══ IAM Policy (permessi minimi per deploy) ═══

%s

═══ Lifecycle (scadenza automatica — solo bucket dedicato) ═══

%s
`,
		BucketPolicyCLI(bucket),
		CORSCLI(bucket),
		IAMPolicy(bucket),
		LifecycleCLI(bucket, 30),
	)
}
