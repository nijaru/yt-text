package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type SpacesConfig struct {
    AccessKey string
    SecretKey string
    Region    string
    Endpoint  string
    Bucket    string
}

type SpacesClient struct {
    client *s3.Client
    bucket string
}

func NewSpacesClient(cfg SpacesConfig) (*SpacesClient, error) {
    resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
        return aws.Endpoint{
            URL: cfg.Endpoint,
        }, nil
    })

    awsCfg, err := config.LoadDefaultConfig(context.Background(),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
        config.WithEndpointResolverWithOptions(resolver),
        config.WithRegion(cfg.Region),
    )
    if err != nil {
        return nil, fmt.Errorf("unable to load SDK config: %v", err)
    }

    return &SpacesClient{
        client: s3.NewFromConfig(awsCfg),
        bucket: cfg.Bucket,
    }, nil
}

func (s *SpacesClient) SaveTranscription(ctx context.Context, url, text, modelName string) error {
    data := struct {
        Text      string    `json:"text"`
        ModelName string    `json:"model_name"`
        Timestamp time.Time `json:"timestamp"`
    }{
        Text:      text,
        ModelName: modelName,
        Timestamp: time.Now(),
    }

    jsonData, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("failed to marshal data: %v", err)
    }

    key := fmt.Sprintf("transcriptions/%s.json", url)
    _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: aws.String(s.bucket),
        Key:    aws.String(key),
        Body:   bytes.NewReader(jsonData),
    })
    if err != nil {
        return fmt.Errorf("failed to save to Spaces: %v", err)
    }

    return nil
}

func (s *SpacesClient) GetTranscription(ctx context.Context, url string) (string, string, error) {
    key := fmt.Sprintf("transcriptions/%s.json", url)
    result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(s.bucket),
        Key:    aws.String(key),
    })
    if err != nil {
        return "", "", fmt.Errorf("failed to get from Spaces: %v", err)
    }
    defer result.Body.Close()

    var data struct {
        Text      string    `json:"text"`
        ModelName string    `json:"model_name"`
        Timestamp time.Time `json:"timestamp"`
    }

    if err := json.NewDecoder(result.Body).Decode(&data); err != nil {
        return "", "", fmt.Errorf("failed to decode data: %v", err)
    }

    return data.Text, data.ModelName, nil
}
