package ffmpeg

import (
	"testing"

	"github.com/yuerchu/media-gateway/pkg/model"
)

func TestAvailableAudioProfiles_LosslessFLAC(t *testing.T) {
	profiles := AvailableAudioProfiles("flac", 1411000, 44100)

	if len(profiles) != 4 {
		t.Fatalf("expected 4 profiles for FLAC, got %d", len(profiles))
	}

	byProfile := make(map[model.AudioProfile]model.AudioProfileInfo)
	for _, p := range profiles {
		byProfile[p.Profile] = p
	}

	std := byProfile[model.AudioProfileStandard]
	if std.Codec != "aac" || std.Bitrate != 128000 || std.SampleRate != 44100 {
		t.Errorf("standard = codec:%s bitrate:%d sr:%d", std.Codec, std.Bitrate, std.SampleRate)
	}
	if std.IsDirect {
		t.Error("standard should not be direct for FLAC source")
	}

	high := byProfile[model.AudioProfileHigh]
	if high.Codec != "aac" || high.Bitrate != 320000 || high.SampleRate != 48000 {
		t.Errorf("high = codec:%s bitrate:%d sr:%d", high.Codec, high.Bitrate, high.SampleRate)
	}

	lossless := byProfile[model.AudioProfileLossless]
	if lossless.Codec != "flac" || lossless.SampleRate != 44100 {
		t.Errorf("lossless = codec:%s sr:%d", lossless.Codec, lossless.SampleRate)
	}
	if !lossless.IsDirect {
		t.Error("lossless should be direct for FLAC source")
	}

	orig := byProfile[model.AudioProfileOriginal]
	if !orig.IsDirect {
		t.Error("original should always be direct")
	}
}

func TestAvailableAudioProfiles_LosslessALAC(t *testing.T) {
	profiles := AvailableAudioProfiles("alac", 1411000, 48000)

	byProfile := make(map[model.AudioProfile]model.AudioProfileInfo)
	for _, p := range profiles {
		byProfile[p.Profile] = p
	}

	lossless := byProfile[model.AudioProfileLossless]
	if lossless.IsDirect {
		t.Error("lossless should not be direct for ALAC (not FLAC)")
	}
	if lossless.SampleRate != 48000 {
		t.Errorf("lossless sample rate = %d, want 48000 (source)", lossless.SampleRate)
	}
}

func TestAvailableAudioProfiles_LosslessZeroSampleRate(t *testing.T) {
	profiles := AvailableAudioProfiles("flac", 1411000, 0)

	for _, p := range profiles {
		if p.Profile == model.AudioProfileLossless {
			if p.SampleRate != 44100 {
				t.Errorf("lossless default sample rate = %d, want 44100", p.SampleRate)
			}
			return
		}
	}
	t.Fatal("lossless profile not found")
}

func TestAvailableAudioProfiles_HighBitrateAAC(t *testing.T) {
	profiles := AvailableAudioProfiles("aac", 350000, 48000)

	byProfile := make(map[model.AudioProfile]model.AudioProfileInfo)
	for _, p := range profiles {
		byProfile[p.Profile] = p
	}

	if _, ok := byProfile[model.AudioProfileLossless]; ok {
		t.Error("lossless should not be available for lossy AAC source")
	}

	std := byProfile[model.AudioProfileStandard]
	if std.IsDirect {
		t.Error("standard should not be direct for 350k AAC")
	}

	high := byProfile[model.AudioProfileHigh]
	if !high.IsDirect {
		t.Error("high should be direct for AAC at 350kbps (within 320k-400k range)")
	}
}

func TestAvailableAudioProfiles_LowBitrateAAC(t *testing.T) {
	profiles := AvailableAudioProfiles("aac", 128000, 44100)

	byProfile := make(map[model.AudioProfile]model.AudioProfileInfo)
	for _, p := range profiles {
		byProfile[p.Profile] = p
	}

	if _, ok := byProfile[model.AudioProfileHigh]; ok {
		t.Error("high should not be available for 128k source")
	}
	if _, ok := byProfile[model.AudioProfileStandard]; ok {
		t.Error("standard should not be available when source bitrate = 128k (not > 128k)")
	}
}

func TestAvailableAudioProfiles_VeryLowBitrate(t *testing.T) {
	profiles := AvailableAudioProfiles("mp3", 64000, 22050)

	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile (original only) for 64k source, got %d", len(profiles))
	}
	if profiles[0].Profile != model.AudioProfileOriginal {
		t.Errorf("profile = %s, want original", profiles[0].Profile)
	}
}

func TestAvailableAudioProfiles_AlwaysHasOriginal(t *testing.T) {
	cases := []struct {
		codec   string
		bitrate int64
	}{
		{"flac", 1411000},
		{"aac", 320000},
		{"mp3", 64000},
		{"opus", 48000},
	}
	for _, tt := range cases {
		profiles := AvailableAudioProfiles(tt.codec, tt.bitrate, 44100)
		last := profiles[len(profiles)-1]
		if last.Profile != model.AudioProfileOriginal {
			t.Errorf("codec=%s: last profile = %s, want original", tt.codec, last.Profile)
		}
		if !last.IsDirect {
			t.Errorf("codec=%s: original should be direct", tt.codec)
		}
	}
}

func TestAvailableAudioProfiles_StandardIsDirect(t *testing.T) {
	profiles := AvailableAudioProfiles("aac", 150000, 44100)
	for _, p := range profiles {
		if p.Profile == model.AudioProfileStandard {
			if !p.IsDirect {
				t.Error("standard should be direct for AAC at 150kbps (within 128k-192k)")
			}
			return
		}
	}
	t.Fatal("standard profile not found for 150k AAC")
}

func TestLosslessCodecs(t *testing.T) {
	lossless := []string{"flac", "alac", "ape", "wavpack", "wav", "pcm_s16le", "pcm_s24le"}
	for _, c := range lossless {
		if !LosslessCodecs[c] {
			t.Errorf("%s should be lossless", c)
		}
	}

	lossy := []string{"aac", "mp3", "opus", "vorbis"}
	for _, c := range lossy {
		if LosslessCodecs[c] {
			t.Errorf("%s should not be lossless", c)
		}
	}
}
