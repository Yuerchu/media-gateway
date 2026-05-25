package ffmpeg

import (
	"testing"
)

func TestParseFrameRate(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"30/1", 30.0},
		{"24000/1001", 23.976023976023978},
		{"30000/1001", 29.97002997002997},
		{"25/1", 25.0},
		{"60/1", 60.0},
		{"0/1", 0},
		{"30/0", 0},
		{"invalid", 0},
		{"", 0},
		{"abc/def", 0},
		{"30", 0},
		{"/1", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFrameRate(tt.input)
			if tt.want == 0 {
				if got != 0 {
					t.Errorf("parseFrameRate(%q) = %f, want 0", tt.input, got)
				}
				return
			}
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("parseFrameRate(%q) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseProbeOutput(t *testing.T) {
	t.Run("full video with audio", func(t *testing.T) {
		data := []byte(`{
			"format": {
				"format_name": "mov,mp4,m4a,3gp,3g2,mj2",
				"duration": "120.500",
				"bit_rate": "5000000"
			},
			"streams": [
				{
					"codec_type": "video",
					"codec_name": "h264",
					"width": 1920,
					"height": 1080,
					"r_frame_rate": "30/1",
					"bit_rate": "4800000"
				},
				{
					"codec_type": "audio",
					"codec_name": "aac",
					"bit_rate": "128000",
					"sample_rate": "44100",
					"channels": 2,
					"bits_per_raw_sample": "16"
				}
			]
		}`)
		result, err := parseProbeOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Width != 1920 {
			t.Errorf("Width = %d, want 1920", result.Width)
		}
		if result.Height != 1080 {
			t.Errorf("Height = %d, want 1080", result.Height)
		}
		if result.DurationSec != 120.5 {
			t.Errorf("DurationSec = %f, want 120.5", result.DurationSec)
		}
		if result.Bitrate != 5000000 {
			t.Errorf("Bitrate = %d, want 5000000", result.Bitrate)
		}
		if result.VideoCodec != "h264" {
			t.Errorf("VideoCodec = %s, want h264", result.VideoCodec)
		}
		if result.AudioCodec != "aac" {
			t.Errorf("AudioCodec = %s, want aac", result.AudioCodec)
		}
		if result.VideoFrameRate != 30.0 {
			t.Errorf("VideoFrameRate = %f, want 30.0", result.VideoFrameRate)
		}
		if result.VideoBitrate != 4800000 {
			t.Errorf("VideoBitrate = %d, want 4800000", result.VideoBitrate)
		}
		if result.AudioBitrate != 128000 {
			t.Errorf("AudioBitrate = %d, want 128000", result.AudioBitrate)
		}
		if result.AudioSampleRate != 44100 {
			t.Errorf("AudioSampleRate = %d, want 44100", result.AudioSampleRate)
		}
		if result.AudioChannels != 2 {
			t.Errorf("AudioChannels = %d, want 2", result.AudioChannels)
		}
		if result.BitsPerRawSample != 16 {
			t.Errorf("BitsPerRawSample = %d, want 16", result.BitsPerRawSample)
		}
		if result.FormatName != "mov,mp4,m4a,3gp,3g2,mj2" {
			t.Errorf("FormatName = %s, want mov,mp4,m4a,3gp,3g2,mj2", result.FormatName)
		}
	})

	t.Run("audio only", func(t *testing.T) {
		data := []byte(`{
			"format": {
				"format_name": "mp3",
				"duration": "240.000",
				"bit_rate": "320000"
			},
			"streams": [
				{
					"codec_type": "audio",
					"codec_name": "mp3",
					"bit_rate": "320000",
					"sample_rate": "48000"
				}
			]
		}`)
		result, err := parseProbeOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Width != 0 || result.Height != 0 {
			t.Error("video dimensions should be 0 for audio-only")
		}
		if result.AudioCodec != "mp3" {
			t.Errorf("AudioCodec = %s, want mp3", result.AudioCodec)
		}
		if result.AudioSampleRate != 48000 {
			t.Errorf("AudioSampleRate = %d, want 48000", result.AudioSampleRate)
		}
	})

	t.Run("empty streams", func(t *testing.T) {
		data := []byte(`{
			"format": {
				"format_name": "unknown",
				"duration": "",
				"bit_rate": ""
			},
			"streams": []
		}`)
		result, err := parseProbeOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.DurationSec != 0 {
			t.Errorf("DurationSec = %f, want 0 for empty duration", result.DurationSec)
		}
		if result.Bitrate != 0 {
			t.Errorf("Bitrate = %d, want 0 for empty bitrate", result.Bitrate)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseProbeOutput([]byte(`{invalid}`))
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("NTSC frame rate", func(t *testing.T) {
		data := []byte(`{
			"format": {"format_name": "mp4", "duration": "10", "bit_rate": "1000"},
			"streams": [{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 1920,
				"height": 1080,
				"r_frame_rate": "24000/1001",
				"bit_rate": "1000"
			}]
		}`)
		result, err := parseProbeOutput(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		diff := result.VideoFrameRate - 23.976
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.001 {
			t.Errorf("VideoFrameRate = %f, want ~23.976", result.VideoFrameRate)
		}
	})
}
