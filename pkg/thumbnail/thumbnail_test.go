package thumbnail

import (
	"testing"
)

func TestScaleFilter(t *testing.T) {
	got := scaleFilter(400, 300)
	want := "scale='min(400,iw)':'min(300,ih)':force_original_aspect_ratio=decrease"
	if got != want {
		t.Errorf("scaleFilter(400, 300) = %q, want %q", got, want)
	}
}

func TestBuildImageArgs(t *testing.T) {
	p := &Params{
		InputPath:  "/tmp/photo.jpg",
		OutputPath: "/tmp/thumb.webp",
		Width:      400,
		Height:     300,
	}
	args := buildImageArgs(p)

	if args[0] != "-y" {
		t.Error("first arg should be -y")
	}
	if args[len(args)-1] != "/tmp/thumb.webp" {
		t.Error("last arg should be output path")
	}

	assertContains(t, args, "-i", "/tmp/photo.jpg")
	assertContains(t, args, "-frames:v", "1")
	assertContains(t, args, "-f", "webp")
	assertContains(t, args, "-quality", "80")
}

func TestBuildVideoArgs(t *testing.T) {
	p := &Params{
		InputPath:  "/tmp/video.mp4",
		OutputPath: "/tmp/thumb.webp",
		Width:      640,
		Height:     480,
	}
	args := buildVideoArgs(p)

	assertContains(t, args, "-i", "/tmp/video.mp4")
	assertContains(t, args, "-frames:v", "1")
	assertContains(t, args, "-f", "webp")

	hasThumbFilter := false
	for i, a := range args {
		if a == "-vf" && i+1 < len(args) {
			if len(args[i+1]) > 0 && args[i+1][:len("thumbnail")] == "thumbnail" {
				hasThumbFilter = true
			}
		}
	}
	if !hasThumbFilter {
		t.Error("video args should include thumbnail filter in -vf")
	}
}

func TestBuildAudioArgs(t *testing.T) {
	p := &Params{
		InputPath:  "/tmp/song.mp3",
		OutputPath: "/tmp/cover.webp",
		Width:      300,
		Height:     300,
	}
	args := buildAudioArgs(p)

	assertContains(t, args, "-i", "/tmp/song.mp3")
	assertContains(t, args, "-frames:v", "1")
	assertContains(t, args, "-f", "webp")

	hasAN := false
	for _, a := range args {
		if a == "-an" {
			hasAN = true
		}
	}
	if !hasAN {
		t.Error("audio args should include -an flag")
	}
}

func assertContains(t *testing.T, args []string, key, val string) {
	t.Helper()
	for i, a := range args {
		if a == key && i+1 < len(args) && args[i+1] == val {
			return
		}
	}
	t.Errorf("args missing %s %s", key, val)
}
