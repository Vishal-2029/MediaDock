package processing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mediadock/backend/internal/utils"
)

var (
	ErrUnsupportedTargetFormat = errors.New("unsupported target format")
	ErrDurationExceeded        = errors.New("media duration exceeds maximum allowed processing limit")
	ErrFFmpegUnavailable       = errors.New("ffmpeg binary is not available on PATH")
)

// Supported video and audio output formats.
var (
	SupportedVideoFormats = map[string]bool{"mp4": true, "mov": true, "mkv": true, "webm": true}
	SupportedAudioFormats = map[string]bool{"mp3": true, "aac": true, "m4a": true, "wav": true, "flac": true, "ogg": true}
)

// ResolutionPreset maps friendly quality names to ffmpeg scale heights (preserving aspect ratio).
var ResolutionPresets = map[string]string{
	"1080p": "scale=-2:1080",
	"720p":  "scale=-2:720",
	"480p":  "scale=-2:480",
	"360p":  "scale=-2:360",
}

// MediaProbeInfo contains inspected stream/container metadata from ffprobe.
type MediaProbeInfo struct {
	DurationSeconds int    `json:"duration_seconds"`
	FormatName      string `json:"format_name"`
	SizeBytes       int64  `json:"size_bytes"`
	HasVideo        bool   `json:"has_video"`
	HasAudio        bool   `json:"has_audio"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
}

// ProcessOptions configures a permitted FFmpeg processing operation.
// Note: MediaDock never strips creator attribution, copyright metadata, or watermarks.
type ProcessOptions struct {
	InputPath        string
	OutputPath       string
	TargetFormat     string
	TargetResolution string
	Compress         bool
	ExtractAudio     bool
}

// Processor defines media inspection and processing operations.
type Processor interface {
	InspectMetadata(ctx context.Context, inputPath string) (*MediaProbeInfo, error)
	ProcessMedia(ctx context.Context, opts ProcessOptions) error
	GenerateThumbnail(ctx context.Context, inputPath, outputPath string) error
}

// FFmpegProcessor implements Processor via safe exec.CommandContext invocations (no shell).
type FFmpegProcessor struct {
	ffmpegBin         string
	ffprobeBin        string
	maxFileSize       int64
	maxProcessingTime time.Duration
	maxDurationSecs   int
}

// NewFFmpegProcessor creates an FFmpegProcessor with strict size and timeout limits.
func NewFFmpegProcessor(maxFileSize int64, maxProcessingTime time.Duration) *FFmpegProcessor {
	maxDur := int(maxProcessingTime.Seconds()) * 4
	if maxDur < 600 {
		maxDur = 600
	}
	return &FFmpegProcessor{
		ffmpegBin:         "ffmpeg",
		ffprobeBin:        "ffprobe",
		maxFileSize:       maxFileSize,
		maxProcessingTime: maxProcessingTime,
		maxDurationSecs:   maxDur,
	}
}

type ffprobeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

// InspectMetadata validates the file and runs ffprobe to inspect streams and duration.
func (p *FFmpegProcessor) InspectMetadata(ctx context.Context, inputPath string) (*MediaProbeInfo, error) {
	if _, _, err := utils.ValidateMediaFile(inputPath, p.maxFileSize); err != nil {
		return nil, err
	}
	if _, err := exec.LookPath(p.ffprobeBin); err != nil {
		return nil, ErrFFmpegUnavailable
	}

	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, p.ffprobeBin,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}

	var parsed ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	durFloat, _ := strconv.ParseFloat(parsed.Format.Duration, 64)
	sizeBytes, _ := strconv.ParseInt(parsed.Format.Size, 10, 64)

	info := &MediaProbeInfo{
		DurationSeconds: int(durFloat),
		FormatName:      parsed.Format.FormatName,
		SizeBytes:       sizeBytes,
	}
	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			info.HasVideo = true
			if s.Width > info.Width {
				info.Width = s.Width
				info.Height = s.Height
			}
		case "audio":
			info.HasAudio = true
		}
	}

	if info.DurationSeconds > p.maxDurationSecs {
		return nil, fmt.Errorf("%w: %ds > %ds", ErrDurationExceeded, info.DurationSeconds, p.maxDurationSecs)
	}

	return info, nil
}

// ProcessMedia performs permitted format conversion, audio extraction, scaling, or compression.
// Existing metadata/attribution tags are preserved (-map_metadata 0) and no crop/delogo filters are allowed.
func (p *FFmpegProcessor) ProcessMedia(ctx context.Context, opts ProcessOptions) error {
	if _, _, err := utils.ValidateMediaFile(opts.InputPath, p.maxFileSize); err != nil {
		return err
	}
	if _, err := exec.LookPath(p.ffmpegBin); err != nil {
		return ErrFFmpegUnavailable
	}

	args, err := BuildFFmpegArgs(opts)
	if err != nil {
		return err
	}

	// Enforce duration limit if ffprobe is present before running long transcodes.
	if _, probeErr := exec.LookPath(p.ffprobeBin); probeErr == nil {
		if _, err := p.InspectMetadata(ctx, opts.InputPath); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o750); err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, p.maxProcessingTime)
	defer cancel()

	cmd := exec.CommandContext(runCtx, p.ffmpegBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(opts.OutputPath)
		return fmt.Errorf("ffmpeg processing failed: %v (%s)", err, tailString(stderr.String(), 300))
	}

	return nil
}

// GenerateThumbnail extracts a representative frame as a JPEG thumbnail.
func (p *FFmpegProcessor) GenerateThumbnail(ctx context.Context, inputPath, outputPath string) error {
	if _, _, err := utils.ValidateMediaFile(inputPath, p.maxFileSize); err != nil {
		return err
	}
	if _, err := exec.LookPath(p.ffmpegBin); err != nil {
		return ErrFFmpegUnavailable
	}

	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{
		"-y",
		"-ss", "00:00:01",
		"-i", inputPath,
		"-vframes", "1",
		"-vf", "scale=640:-2",
		"-q:v", "4",
		outputPath,
	}

	cmd := exec.CommandContext(runCtx, p.ffmpegBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("thumbnail generation failed: %v (%s)", err, tailString(stderr.String(), 200))
	}
	return nil
}

// BuildFFmpegArgs validates ProcessOptions and builds deterministic, injection-safe FFmpeg arguments.
func BuildFFmpegArgs(opts ProcessOptions) ([]string, error) {
	targetFmt := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(opts.TargetFormat), "."))
	if targetFmt == "" {
		return nil, fmt.Errorf("%w: empty target format", ErrUnsupportedTargetFormat)
	}

	isAudio := opts.ExtractAudio || SupportedAudioFormats[targetFmt]
	if !isAudio && !SupportedVideoFormats[targetFmt] {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedTargetFormat, targetFmt)
	}
	if opts.ExtractAudio && !SupportedAudioFormats[targetFmt] {
		return nil, fmt.Errorf("%w: audio extraction requires an audio format, got %q", ErrUnsupportedTargetFormat, targetFmt)
	}

	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", opts.InputPath,
		"-map_metadata", "0", // Preserve original creator attribution and metadata tags
	}

	if isAudio {
		args = append(args, "-vn")
		switch targetFmt {
		case "mp3":
			args = append(args, "-c:a", "libmp3lame", "-b:a", "192k")
		case "aac", "m4a":
			args = append(args, "-c:a", "aac", "-b:a", "192k")
		case "wav":
			args = append(args, "-c:a", "pcm_s16le")
		case "flac":
			args = append(args, "-c:a", "flac")
		case "ogg":
			args = append(args, "-c:a", "libvorbis", "-q:a", "5")
		}
		args = append(args, opts.OutputPath)
		return args, nil
	}

	// Video processing
	if opts.TargetResolution != "" && opts.TargetResolution != "original" && opts.TargetResolution != "audio" {
		vf, ok := ResolutionPresets[strings.ToLower(opts.TargetResolution)]
		if !ok {
			return nil, fmt.Errorf("%w: unsupported resolution preset %q", ErrUnsupportedTargetFormat, opts.TargetResolution)
		}
		args = append(args, "-vf", vf)
	}

	crf := "23"
	if opts.Compress {
		crf = "28"
	}

	switch targetFmt {
	case "mp4", "mov":
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", crf, "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart")
	case "mkv":
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", crf, "-c:a", "aac", "-b:a", "128k")
	case "webm":
		args = append(args, "-c:v", "libvpx-vp9", "-crf", "32", "-b:v", "0", "-c:a", "libopus")
	}

	args = append(args, opts.OutputPath)
	return args, nil
}

func tailString(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

