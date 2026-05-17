package model

// AudioProfile identifies a transcoding quality tier.
type AudioProfile string

const (
	AudioProfileStandard AudioProfile = "standard"
	AudioProfileHigh     AudioProfile = "high"
	AudioProfileLossless AudioProfile = "lossless"
	AudioProfileOriginal AudioProfile = "original"
)

// AudioTranscodeParams holds parameters for building an audio-only ffmpeg command.
type AudioTranscodeParams struct {
	InputPath  string
	OutputPath string // empty = pipe mode (stdout)
	Codec      string // "aac" | "flac"
	Bitrate    string // "128k" | "320k" | "" (lossless)
	SampleRate int    // 44100 | 48000 | 0 (keep source)
	Channels   int    // 0 = keep source
}

// AudioProfileInfo describes an available quality tier for a given source file.
type AudioProfileInfo struct {
	Profile    AudioProfile `json:"profile"`
	Label      string       `json:"label"`
	Codec      string       `json:"codec"`
	Bitrate    int          `json:"bitrate"`     // bps, 0 for lossless/original
	SampleRate int          `json:"sample_rate"` // Hz, 0 for source
	Extension  string       `json:"extension"`   // "m4a" | "flac" | source ext
	MIMEType   string       `json:"mime_type"`   // "audio/mp4" | "audio/flac" | source MIME
	IsDirect   bool         `json:"is_direct"`   // true = serve original file, no transcode needed
}
