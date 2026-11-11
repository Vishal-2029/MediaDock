package utils

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrPathTraversal       = errors.New("invalid file path outside allowed directory")
	ErrUnsupportedFileType = errors.New("unsupported or potentially unsafe media file type")
	ErrFileTooLarge        = errors.New("file exceeds maximum allowed size")
)

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// AllowedMediaExtensions lists safe container extensions for video/audio/image processing.
var AllowedMediaExtensions = map[string]bool{
	".mp4":  true,
	".mov":  true,
	".mkv":  true,
	".webm": true,
	".avi":  true,
	".mp3":  true,
	".aac":  true,
	".m4a":  true,
	".wav":  true,
	".flac": true,
	".ogg":  true,
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// AllowedMediaMIMEs lists MIME prefixes/exact types permitted for media processing.
var AllowedMediaMIMEs = map[string]bool{
	"video/mp4":                true,
	"video/quicktime":          true,
	"video/x-matroska":         true,
	"video/webm":               true,
	"video/x-msvideo":          true,
	"audio/mpeg":               true,
	"audio/mp4":                true,
	"audio/aac":                true,
	"audio/wav":                true,
	"audio/x-wav":              true,
	"audio/flac":               true,
	"audio/ogg":                true,
	"application/ogg":          true,
	"image/jpeg":               true,
	"image/png":                true,
	"image/webp":               true,
	"application/octet-stream": false, // explicitly disallowed unless verified by extension + magic bytes
}

// SafeJoinPath resolves relPath inside baseDir and ensures no directory traversal occurs.
func SafeJoinPath(baseDir, relPath string) (string, error) {
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", err
	}
	cleanedRel := filepath.Clean(relPath)
	if filepath.IsAbs(cleanedRel) || strings.HasPrefix(cleanedRel, "..") {
		return "", ErrPathTraversal
	}
	joined := filepath.Join(absBase, cleanedRel)
	if joined != absBase && !strings.HasPrefix(joined, absBase+string(filepath.Separator)) {
		return "", ErrPathTraversal
	}
	return joined, nil
}

// SanitizeFilename strips unsafe characters and directory components from a filename.
func SanitizeFilename(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == ".." {
		return "media_file"
	}
	sanitized := unsafeFilenameChars.ReplaceAllString(base, "_")
	if len(sanitized) > 180 {
		ext := filepath.Ext(sanitized)
		sanitized = sanitized[:180-len(ext)] + ext
	}
	return sanitized
}

// CreateSafeTempFile creates a uniquely named temp file inside dir (creating dir if needed).
func CreateSafeTempFile(dir, prefix, ext string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanExt := ""
	if ext != "" {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if !AllowedMediaExtensions[strings.ToLower(ext)] {
			return nil, fmt.Errorf("%w: extension %q", ErrUnsupportedFileType, ext)
		}
		cleanExt = strings.ToLower(ext)
	}
	pattern := SanitizeFilename(prefix) + "-*" + cleanExt
	return os.CreateTemp(dir, pattern)
}

// ValidateMediaFile inspects a file's size, extension, and magic bytes.
func ValidateMediaFile(path string, maxSize int64) (string, int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	if info.IsDir() {
		return "", 0, fmt.Errorf("%w: path is a directory", ErrUnsupportedFileType)
	}
	if maxSize > 0 && info.Size() > maxSize {
		return "", info.Size(), fmt.Errorf("%w: %d > %d", ErrFileTooLarge, info.Size(), maxSize)
	}
	if info.Size() == 0 {
		return "", 0, fmt.Errorf("%w: empty file", ErrUnsupportedFileType)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if !AllowedMediaExtensions[ext] {
		return "", info.Size(), fmt.Errorf("%w: extension %q", ErrUnsupportedFileType, ext)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", info.Size(), err
	}
	defer f.Close()

	header := make([]byte, 512)
	n, err := io.ReadFull(f, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", info.Size(), err
	}
	header = header[:n]

	detected := http.DetectContentType(header)
	// http.DetectContentType misses ISO BMFF (mp4/m4a/mov), Matroska/WebM, FLAC, AAC ADTS; check magic bytes.
	if hasValidMediaSignature(header, ext) {
		if detected == "application/octet-stream" {
			detected = mimeForExtension(ext)
		}
		return detected, info.Size(), nil
	}

	if AllowedMediaMIMEs[detected] {
		return detected, info.Size(), nil
	}

	return "", info.Size(), fmt.Errorf("%w: detected mime %q for extension %q", ErrUnsupportedFileType, detected, ext)
}

func hasValidMediaSignature(b []byte, ext string) bool {
	if len(b) < 4 {
		return false
	}
	// ISO Base Media File Format (MP4, M4A, MOV): bytes 4..8 == "ftyp"
	if len(b) >= 8 && string(b[4:8]) == "ftyp" {
		return ext == ".mp4" || ext == ".m4a" || ext == ".mov"
	}
	// EBML / Matroska / WebM: 1A 45 DF A3
	if b[0] == 0x1A && b[1] == 0x45 && b[2] == 0xDF && b[3] == 0xA3 {
		return ext == ".mkv" || ext == ".webm"
	}
	// RIFF AVI / WAV
	if len(b) >= 12 && string(b[0:4]) == "RIFF" {
		kind := string(b[8:12])
		if kind == "WAVE" && ext == ".wav" {
			return true
		}
		if kind == "AVI " && ext == ".avi" {
			return true
		}
		if kind == "WEBP" && ext == ".webp" {
			return true
		}
	}
	// MP3 ID3 or MPEG sync frame
	if (len(b) >= 3 && string(b[0:3]) == "ID3") || (b[0] == 0xFF && (b[1]&0xE0) == 0xE0) {
		return ext == ".mp3" || ext == ".aac"
	}
	// FLAC
	if string(b[0:4]) == "fLaC" && ext == ".flac" {
		return true
	}
	// OGG
	if string(b[0:4]) == "OggS" && ext == ".ogg" {
		return true
	}
	// JPEG
	if b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF && (ext == ".jpg" || ext == ".jpeg") {
		return true
	}
	// PNG
	if len(b) >= 8 && string(b[0:8]) == "\x89PNG\r\n\x1a\n" && ext == ".png" {
		return true
	}
	return false
}

func mimeForExtension(ext string) string {
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mp3":
		return "audio/mpeg"
	case ".aac":
		return "audio/aac"
	case ".m4a":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

