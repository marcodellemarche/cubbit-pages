# Cubbit S3 API

## Endpoint
- S3 API: `https://s3.cubbit.eu`
- Console: `https://console.cubbit.io`
- Docs API key: `https://console.cubbit.io/api-keys`
- Docs bucket: `https://docs.cubbit.io`

## Configurazione aws-sdk-go-v2
```go
cfg, _ := config.LoadDefaultConfig(context.TODO(),
    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
        accessKeyID, secretAccessKey, "",
    )),
    config.WithRegion("eu-west-1"),
    config.WithEndpointResolverWithOptions(
        aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
            return aws.Endpoint{URL: "https://s3.cubbit.eu"}, nil
        }),
    ),
)
client := s3.NewFromConfig(cfg, func(o *s3.Options) {
    o.UsePathStyle = true // OBBLIGATORIO per Cubbit
})
```

## CORS sul bucket (per siti cifrati)
Necessario perché il browser fa fetch() dei file .enc dallo stesso bucket.
```json
[{
  "AllowedOrigins": ["*"],
  "AllowedMethods": ["GET"],
  "AllowedHeaders": ["*"]
}]
```

## Bucket Policy per hosting pubblico
```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": "*",
    "Action": "s3:GetObject",
    "Resource": "arn:aws:s3:::NOME-BUCKET/*"
  }]
}
```

## IAM Policy minima per deploy
```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["s3:PutObject", "s3:PutObjectAcl", "s3:DeleteObject"],
    "Resource": "arn:aws:s3:::NOME-BUCKET/*"
  }]
}
```

## Note
- `UsePathStyle: true` è obbligatorio — non rimuovere mai
- Per siti cifrati, configurare CORS è necessario per i fetch() del JS di decifratura
