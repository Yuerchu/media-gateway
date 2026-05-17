package ffmpeg

import "github.com/yuerchu/media-gateway/pkg/model"

// LosslessCodecs lists audio codecs considered lossless.
var LosslessCodecs = map[string]bool{
	"flac": true, "alac": true, "ape": true, "wavpack": true, "tak": true,
	"wav": true, "pcm_s16le": true, "pcm_s24le": true, "pcm_s32le": true,
	"pcm_s16be": true, "pcm_s24be": true, "pcm_s32be": true,
	"pcm_f32le": true, "pcm_f64le": true,
	"dsd_lsbf": true, "dsd_msbf": true, "dsd_lsbf_planar": true, "dsd_msbf_planar": true,
}

// AvailableAudioProfiles returns quality tiers available for the given source.
// Rules:
//   - original: always available
//   - lossless: only when source codec is lossless; IsDirect if source is already FLAC
//   - high (AAC 320k): available if source bitrate > 320kbps or source is lossless
//   - standard (AAC 128k): available if source bitrate > 128kbps or source is lossless
//
// Tiers where the source already matches (codec + bitrate) are marked IsDirect.
func AvailableAudioProfiles(sourceCodec string, sourceBitrate int64, sourceSampleRate int) []model.AudioProfileInfo {
	isLossless := LosslessCodecs[sourceCodec]
	isAAC := sourceCodec == "aac"

	var profiles []model.AudioProfileInfo

	// standard: AAC 128kbps / 44.1kHz
	if isLossless || sourceBitrate > 128000 {
		direct := isAAC && sourceBitrate >= 128000 && sourceBitrate <= 192000
		profiles = append(profiles, model.AudioProfileInfo{
			Profile:    model.AudioProfileStandard,
			Label:      "标准",
			Codec:      "aac",
			Bitrate:    128000,
			SampleRate: 44100,
			Extension:  "m4a",
			MIMEType:   "audio/mp4",
			IsDirect:   direct,
		})
	}

	// high: AAC 320kbps / 48kHz
	if isLossless || sourceBitrate > 320000 {
		direct := isAAC && sourceBitrate >= 320000 && sourceBitrate <= 400000
		profiles = append(profiles, model.AudioProfileInfo{
			Profile:    model.AudioProfileHigh,
			Label:      "高品",
			Codec:      "aac",
			Bitrate:    320000,
			SampleRate: 48000,
			Extension:  "m4a",
			MIMEType:   "audio/mp4",
			IsDirect:   direct,
		})
	}

	// lossless: FLAC / source sample rate
	if isLossless {
		sr := sourceSampleRate
		if sr == 0 {
			sr = 44100
		}
		profiles = append(profiles, model.AudioProfileInfo{
			Profile:    model.AudioProfileLossless,
			Label:      "无损",
			Codec:      "flac",
			Bitrate:    0,
			SampleRate: sr,
			Extension:  "flac",
			MIMEType:   "audio/flac",
			IsDirect:   sourceCodec == "flac",
		})
	}

	// original: always available, always direct
	profiles = append(profiles, model.AudioProfileInfo{
		Profile:  model.AudioProfileOriginal,
		Label:    "原始",
		IsDirect: true,
	})

	return profiles
}
