// Package secrets contains narrow managed-secret adapters. It never exposes
// values through HTTP, logs, or configuration structs.
package secrets

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SecretValueClient interface {
	GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// AWSSecretsManager resolves base64-encoded webhook HMAC values by opaque
// secret reference. IAM instance roles supply AWS credentials; configuration
// contains no raw signing material.
type AWSSecretsManager struct{ client SecretValueClient }

func NewAWSSecretsManager(ctx context.Context, region string) (*AWSSecretsManager, error) {
	region = strings.TrimSpace(region)
	if region == "" {
		return nil, errors.New("AWS region is required for managed webhook secrets")
	}
	configuration, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS secret configuration: %w", err)
	}
	return &AWSSecretsManager{client: secretsmanager.NewFromConfig(configuration)}, nil
}

func NewAWSSecretsManagerWithClient(client SecretValueClient) (*AWSSecretsManager, error) {
	if client == nil {
		return nil, errors.New("AWS Secrets Manager client is required")
	}
	return &AWSSecretsManager{client: client}, nil
}

func (p *AWSSecretsManager) Resolve(ctx context.Context, reference string) ([]byte, error) {
	if p == nil || p.client == nil || strings.TrimSpace(reference) == "" {
		return nil, errors.New("managed webhook secret reference is required")
	}
	result, err := p.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(strings.TrimSpace(reference))})
	if err != nil {
		return nil, fmt.Errorf("read managed webhook secret: %w", err)
	}
	if len(result.SecretBinary) > 0 {
		return validKey(result.SecretBinary)
	}
	if result.SecretString == nil {
		return nil, errors.New("managed webhook secret has no value")
	}
	encoded := strings.TrimSpace(*result.SecretString)
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, errors.New("managed webhook secret must be base64-encoded")
	}
	return validKey(key)
}

func validKey(value []byte) ([]byte, error) {
	if len(value) < 32 || len(value) > 8192 {
		return nil, errors.New("managed webhook secret must contain 32..8192 bytes")
	}
	return append([]byte(nil), value...), nil
}
