package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// hasFFmpeg reporta se ffmpeg/ffprobe estão disponíveis (skip de testes E2E).
func hasFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

// makeTestVideo gera um vídeo de teste sintético (1s, 64x64, teste tone).
func makeTestVideo(t *testing.T, dir string) string {
	t.Helper()
	hasFFmpeg(t)
	path := filepath.Join(dir, "test.mp4")
	args := []string{
		"-y", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=64x64:rate=10",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac",
		"-shortest",
		path,
	}
	cmd := exec.Command("ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("makeTestVideo: %v: %s", err, out)
	}
	return path
}

func TestProbeVideo(t *testing.T) {
	hasFFmpeg(t)
	dir := t.TempDir()
	vid := makeTestVideo(t, dir)

	info, err := Probe(context.Background(), vid)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasVideo {
		t.Fatal("expected video stream")
	}
	if !info.HasAudio {
		t.Fatal("expected audio stream")
	}
	vs := info.VideoStream()
	if vs == nil || vs.CodecName == "" {
		t.Fatal("video stream missing codec")
	}
	if vs.Width == 0 || vs.Height == 0 {
		t.Fatalf("video dimensions missing: %dx%d", vs.Width, vs.Height)
	}
	if info.DurationSeconds() < 0.5 {
		t.Fatalf("duration = %v, want ~1s", info.DurationSeconds())
	}
}

func TestProbeNonMedia(t *testing.T) {
	hasFFmpeg(t)
	dir := t.TempDir()
	bad := filepath.Join(dir, "not-media.txt")
	if err := os.WriteFile(bad, []byte("this is not media"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Probe(context.Background(), bad); err == nil {
		t.Fatal("expected error probing non-media file")
	}
}

func TestProbeMissingFile(t *testing.T) {
	if _, err := Probe(context.Background(), "/nonexistent/x.mp4"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExtractAudioWav(t *testing.T) {
	hasFFmpeg(t)
	dir := t.TempDir()
	vid := makeTestVideo(t, dir)
	out := filepath.Join(dir, "audio.wav")

	if err := ExtractAudio(context.Background(), vid, out, PipeOptions{}); err != nil {
		t.Fatal(err)
	}
	info, err := Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasAudio {
		t.Fatal("extracted file should have audio")
	}
	if info.HasVideo {
		t.Fatal("extracted file should NOT have video (-vn)")
	}
}

func TestExtractFrame(t *testing.T) {
	hasFFmpeg(t)
	dir := t.TempDir()
	vid := makeTestVideo(t, dir)
	out := filepath.Join(dir, "frame.png")

	if err := ExtractFrame(context.Background(), vid, out, "0.5", PipeOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("frame not created: %v", err)
	}
	info, err := Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasVideo {
		t.Fatal("frame should be an image (video stream)")
	}
}

func TestTranscode(t *testing.T) {
	hasFFmpeg(t)
	dir := t.TempDir()
	vid := makeTestVideo(t, dir)
	out := filepath.Join(dir, "out.webm")

	if err := Transcode(context.Background(), vid, out, PipeOptions{Quality: "preview"}); err != nil {
		t.Fatal(err)
	}
	info, err := Validate(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasVideo {
		t.Fatal("transcoded file should have video")
	}
}

func TestValidateCorrupted(t *testing.T) {
	hasFFmpeg(t)
	dir := t.TempDir()
	bad := filepath.Join(dir, "corrupt.mp4")
	if err := os.WriteFile(bad, []byte("not a real mp4 at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(context.Background(), bad); err == nil {
		t.Fatal("expected validation error for corrupted file")
	}
}
