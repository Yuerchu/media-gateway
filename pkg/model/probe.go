package model

// ProbeResult holds video metadata extracted by ffprobe.
type ProbeResult struct {
	Width       int
	Height      int
	DurationSec float64
	VideoCodec  string
	AudioCodec  string
	FormatName  string
	Bitrate     int64
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
