package scripts

import (
	"context"
	"fmt"
	"io"
	"time"
	pb "yt-text/protos/transcribe"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClient is a client for the transcription gRPC service
type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.TranscriptionServiceClient
	config GRPCConfig
}

// GRPCConfig holds configuration for the gRPC client
type GRPCConfig struct {
	ServerAddress string        // gRPC server address (host:port)
	Timeout       time.Duration // Default timeout for requests
	DialOptions   []grpc.DialOption
}

// NewGRPCClient creates a new gRPC client for transcription services
func NewGRPCClient(config GRPCConfig) (*GRPCClient, error) {
	// Set default dial options if none provided
	if len(config.DialOptions) == 0 {
		config.DialOptions = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(5 * time.Second),
		}
	}

	// Connect to the gRPC server
	conn, err := grpc.Dial(config.ServerAddress, config.DialOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	// Create client
	client := pb.NewTranscriptionServiceClient(conn)

	return &GRPCClient{
		conn:   conn,
		client: client,
		config: config,
	}, nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Validate checks if a video URL is valid and can be processed
func (c *GRPCClient) Validate(ctx context.Context, url string) (VideoInfo, error) {
	const op = "GRPCClient.Validate"

	// Create request
	req := &pb.ValidateRequest{
		Url: url,
	}

	// Apply context timeout if not already set
	if _, ok := ctx.Deadline(); !ok && c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	// Call gRPC method
	resp, err := c.client.Validate(ctx, req)
	if err != nil {
		return VideoInfo{}, newScriptError(op, err, "validation request failed")
	}

	// Convert response to our model
	return VideoInfo{
		Valid:    resp.Valid,
		Duration: resp.Duration,
		Format:   resp.Format,
		Error:    resp.Error,
		URL:      resp.Url,
	}, nil
}

// Transcribe transcribes a video and returns the result
func (c *GRPCClient) Transcribe(
	ctx context.Context,
	url string,
	opts map[string]string,
	enableConstraints bool,
) (*TranscriptionResult, error) {
	const op = "GRPCClient.Transcribe"

	// Create request
	req := &pb.TranscribeRequest{
		Url:              url,
		EnableConstraints: enableConstraints,
		Options:          opts,
	}

	// Apply context timeout if not already set
	if _, ok := ctx.Deadline(); !ok && c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	// Call gRPC streaming method
	stream, err := c.client.Transcribe(ctx, req)
	if err != nil {
		return nil, newScriptError(op, err, "failed to start transcription")
	}

	var finalResponse *pb.TranscribeResponse
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, newScriptError(op, err, "error receiving transcription updates")
		}

		// Handle progress updates (could emit to a channel if needed)
		// For now, we just store the final response
		finalResponse = resp
	}

	// If we didn't get a final response
	if finalResponse == nil {
		return nil, newScriptError(op, nil, "no transcription response received")
	}

	// Check for errors
	if finalResponse.Error != "" {
		return nil, newScriptError(op, fmt.Errorf(finalResponse.Error), "transcription failed")
	}

	// Convert to our model
	var title, language, url_ *string
	if finalResponse.Title != "" {
		title = &finalResponse.Title
	}
	if finalResponse.Language != "" {
		language = &finalResponse.Language
	}
	if url != "" {
		url_ = &url
	}

	result := &TranscriptionResult{
		Text:                finalResponse.Text,
		ModelName:           finalResponse.ModelName,
		Duration:            finalResponse.Duration,
		Title:               title,
		URL:                 url_,
		Language:            language,
		LanguageProbability: finalResponse.LanguageProbability,
		Source:              finalResponse.Source,
	}

	return result, nil
}

// FetchYouTubeCaptions fetches captions from YouTube API
func (c *GRPCClient) FetchYouTubeCaptions(
	ctx context.Context,
	videoID string,
	apiKey string,
) (*TranscriptionResult, error) {
	const op = "GRPCClient.FetchYouTubeCaptions"

	// Create request
	req := &pb.CaptionRequest{
		VideoId: videoID,
		ApiKey:  apiKey,
	}

	// Apply context timeout if not already set
	if _, ok := ctx.Deadline(); !ok && c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	// Call gRPC method
	resp, err := c.client.FetchYouTubeCaptions(ctx, req)
	if err != nil {
		return nil, newScriptError(op, err, "failed to fetch YouTube captions")
	}

	// Check for errors
	if resp.Error != "" {
		return nil, newScriptError(op, fmt.Errorf(resp.Error), "failed to fetch YouTube captions")
	}

	// Convert to our model
	var title, language *string
	if resp.Title != "" {
		title = &resp.Title
	}
	if resp.Language != "" {
		language = &resp.Language
	}

	result := &TranscriptionResult{
		Text:     resp.Text,
		Title:    title,
		Language: language,
		Source:   resp.Source,
	}

	return result, nil
}