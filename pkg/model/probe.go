package model

// ProbeResult holds media metadata extracted by ffprobe.
type ProbeResult struct {
	Width       int
	Height      int
	DurationSec float64
	VideoCodec  string
	AudioCodec  string
	FormatName  string
	Bitrate     int64
	HasAudio    bool
	Rotation    int

	VideoFrameRate   float64
	VideoBitrate     int64
	AudioBitrate     int64
	AudioSampleRate  int
	AudioChannels    int
	BitsPerRawSample int
}

// HasVideoStream reports whether the probed file carries a video stream.
//
// Probing accepts audio-only input because audio transcoding needs it, so any
// caller about to do something geometric has to ask first. Skipping the check
// is not a graceful degradation: Width and Height come back as 0, the target
// dimensions compute to 0x0, and ffmpeg is handed a "scale=0:0" filter that
// fails deep inside the encoder with a message saying nothing about the actual
// problem.
//
// This does not imply usable dimensions — cover art in an audio file is a
// video stream with a still image in it. Callers that need a real picture
// should check Width and Height too.
func (r *ProbeResult) HasVideoStream() bool {
	return r.VideoCodec != ""
}

// TranscodeParams holds parameters for building the ffmpeg command.
type TranscodeParams struct {
	InputPath     string
	OutputPath    string
	TargetWidth   int
	TargetHeight  int
	TargetBitrate int64
	MaxDuration   float64
	VideoCodec    string
	AudioCodec    string
	Preset        string
}

// ExtractFrameParams holds parameters for extracting a video frame.
type ExtractFrameParams struct {
	InputPath  string
	OutputPath string
	MaxWidth   int
}
