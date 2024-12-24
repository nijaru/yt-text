package subtitles

import (
	"context"
	"encoding/json"
	"yt-text/errors"
	"yt-text/scripts"
)

type Service interface {
	GetAvailable(ctx context.Context, url string) (*SubtitleInfo, error)
	Download(ctx context.Context, url string, lang string, auto bool) (string, error)
}

type SubtitleTrack struct {
	URL      string `json:"url"`
	Language string `json:"language"`
	Ext      string `json:"ext"`
}

type SubtitleInfo struct {
	Available     bool                       `json:"success"`
	Subtitles     map[string][]SubtitleTrack `json:"subtitles"`
	AutoSubtitles map[string][]SubtitleTrack `json:"auto_subtitles"`
	Title         string                     `json:"title"`
}

type service struct {
	scriptRunner *scripts.ScriptRunner
}

func NewService(scriptRunner *scripts.ScriptRunner) Service {
	return &service{scriptRunner: scriptRunner}
}

func (s *service) GetAvailable(ctx context.Context, url string) (*SubtitleInfo, error) {
	output, err := s.scriptRunner.RunScript(ctx, "get_subtitles.py", map[string]string{
		"url": url,
	}, nil)
	if err != nil {
		return nil, err
	}

	var info SubtitleInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, errors.Internal("GetAvailable", err, "Failed to parse subtitle info")
	}

	return &info, nil
}

func (s *service) Download(
	ctx context.Context,
	url string,
	lang string,
	auto bool,
) (string, error) {
	args := map[string]string{
		"url":  url,
		"lang": lang,
	}

	var flags []string
	if auto {
		flags = append(flags, "auto")
	}
	flags = append(flags, "download")

	output, err := s.scriptRunner.RunScript(ctx, "get_subtitles.py", args, flags)
	if err != nil {
		return "", err
	}

	var result struct {
		Success bool   `json:"success"`
		Text    string `json:"text"`
		Error   string `json:"error"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", errors.Internal("Download", err, "Failed to parse subtitle result")
	}

	if !result.Success {
		return "", errors.NotFound("Download", nil, result.Error)
	}

	return result.Text, nil
}
