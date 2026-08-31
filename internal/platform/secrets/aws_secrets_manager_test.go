package secrets

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type secretClientStub struct {
	result *secretsmanager.GetSecretValueOutput
}

func (s secretClientStub) GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return s.result, nil
}

func TestAWSSecretsManagerResolvesBase64AndBinaryKeys(t *testing.T) {
	key := []byte("this-managed-webhook-key-is-long-enough")
	for name, output := range map[string]*secretsmanager.GetSecretValueOutput{
		"string": {SecretString: aws.String(base64.RawStdEncoding.EncodeToString(key))},
		"binary": {SecretBinary: key},
	} {
		t.Run(name, func(t *testing.T) {
			provider, err := NewAWSSecretsManagerWithClient(secretClientStub{result: output})
			if err != nil {
				t.Fatal(err)
			}
			resolved, err := provider.Resolve(context.Background(), "arn:aws:secretsmanager:ap-south-1:000000000000:secret:webhook")
			if err != nil || string(resolved) != string(key) {
				t.Fatalf("resolved=%q err=%v", resolved, err)
			}
		})
	}
}
