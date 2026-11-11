package tests

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mediadock/backend/internal/modules/processing"
	"github.com/mediadock/backend/internal/utils"
)

func TestSSRFProtection_BlocksRestrictedTargets(t *testing.T) {
	ctx := context.Background()

	blocked := []string{
		"http://localhost/video.mp4",
		"http://127.0.0.1:8080/secret",
		"http://[::1]/secret",
		"http://169.254.169.254/latest/meta-data/",
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://10.0.0.5/internal.mp4",
		"http://172.16.10.2/internal.mp4",
		"http://192.168.1.50/internal.mp4",
		"http://100.64.0.1/cgnat.mp4",
		"file:///etc/passwd",
		"ftp://example.com/video.mp4",
		"https://user:pass@example.com/video.mp4",
	}

	for _, raw := range blocked {
		_, err := utils.ValidatePublicURL(ctx, raw, false)
		if err == nil {
			t.Errorf("expected URL %q to be blocked, but it was allowed", raw)
		}
	}
}

func TestSSRFProtection_PublicIPChecks(t *testing.T) {
	cases := []struct {
		ip     string
		public bool
	}{
		{"93.184.216.34", true},
		{"8.8.8.8", true},
		{"127.0.0.1", false},
		{"10.1.2.3", false},
		{"172.31.255.255", false},
		{"192.168.0.1", false},
		{"169.254.169.254", false},
		{"100.64.0.1", false},
		{"::1", false},
		{"fc00::1", false},
		{"fe80::1", false},
	}

	for _, tc := range cases {
		addr := netip.MustParseAddr(tc.ip)
		got := utils.IsPublicIP(addr)
		if got != tc.public {
			t.Errorf("IsPublicIP(%s) = %v, want %v", tc.ip, got, tc.public)
		}
	}
}

func TestFileValidation_And_PathTraversal(t *testing.T) {
	dir := t.TempDir()

	// Valid MP4 header
	validMP4 := filepath.Join(dir, "valid.mp4")
	if err := os.WriteFile(validMP4, []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0}, 0o640); err != nil {
		t.Fatal(err)
	}
	mimeType, size, err := utils.ValidateMediaFile(validMP4, 1024*1024)
	if err != nil {
		t.Fatalf("expected valid MP4 to pass validation, got: %v", err)
	}
	if mimeType != "video/mp4" || size != 16 {
		t.Fatalf("unexpected mime/size: %s / %d", mimeType, size)
	}

	// Spoofed HTML file disguised as .mp4
	spoofed := filepath.Join(dir, "evil.mp4")
	if err := os.WriteFile(spoofed, []byte("<html><script>alert(1)</script></html>"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := utils.ValidateMediaFile(spoofed, 1024*1024); !errors.Is(err, utils.ErrUnsupportedFileType) {
		t.Fatalf("expected ErrUnsupportedFileType for spoofed mp4, got: %v", err)
	}

	// Oversized file
	if _, _, err := utils.ValidateMediaFile(validMP4, 8); !errors.Is(err, utils.ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got: %v", err)
	}

	// Path traversal prevention
	if _, err := utils.SafeJoinPath(dir, "../../etc/passwd"); !errors.Is(err, utils.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got: %v", err)
	}
}

func TestFFmpegArgsBuilder_PreservesAttributionAndValidatesFormats(t *testing.T) {
	args, err := processing.BuildFFmpegArgs(processing.ProcessOptions{
		InputPath:        "/tmp/in.mp4",
		OutputPath:       "/tmp/out.mp4",
		TargetFormat:     "mp4",
		TargetResolution: "720p",
		Compress:         true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-map_metadata 0") {
		t.Fatalf("expected metadata preservation flag -map_metadata 0, got: %s", joined)
	}
	if !strings.Contains(joined, "scale=-2:720") {
		t.Fatalf("expected 720p scaling filter, got: %s", joined)
	}

	// Reject audio extraction to video container
	_, err = processing.BuildFFmpegArgs(processing.ProcessOptions{
		InputPath:    "/tmp/in.mp4",
		OutputPath:   "/tmp/out.mp4",
		TargetFormat: "mp4",
		ExtractAudio: true,
	})
	if !errors.Is(err, processing.ErrUnsupportedTargetFormat) {
		t.Fatalf("expected ErrUnsupportedTargetFormat, got: %v", err)
	}
}

