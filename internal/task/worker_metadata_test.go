package task

import (
	"testing"

	"github.com/yuerchu/media-gateway/pkg/model"
)

func TestBuildStreamMetadata_IncludesProbeFields(t *testing.T) {
	metadata := buildStreamMetadata(&model.ProbeResult{
		Width:       1920,
		Height:      1080,
		DurationSec: 120.5,
		VideoCodec:  "h264",
		AudioCodec:  "aac",
		HasAudio:    true,
		Rotation:    -90,
		FormatName:  "mov,mp4,m4a,3gp,3g2,mj2",
		Bitrate:     5_000_000,
	})

	if metadata["has_audio"] != true {
		t.Errorf("has_audio = %v, want true", metadata["has_audio"])
	}
	if metadata["rotation"] != -90 {
		t.Errorf("rotation = %v, want -90", metadata["rotation"])
	}
	if metadata["video_codec"] != "h264" || metadata["audio_codec"] != "aac" {
		t.Errorf("codec metadata = %#v", metadata)
	}
}

func TestBuildStreamMetadata_PreservesFalseAudioAndZeroRotation(t *testing.T) {
	metadata := buildStreamMetadata(&model.ProbeResult{})

	if metadata["has_audio"] != false {
		t.Errorf("has_audio = %v, want false", metadata["has_audio"])
	}
	if metadata["rotation"] != 0 {
		t.Errorf("rotation = %v, want 0", metadata["rotation"])
	}
}
