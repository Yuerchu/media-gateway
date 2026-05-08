package exifmeta

import (
	"bytes"
	"log/slog"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// Result holds EXIF metadata extracted from an image.
type Result struct {
	Orientation  int
	CameraMake   string
	CameraModel  string
	FocalLength  float64
	ISOSpeed     int
	FNumber      float64
	ExposureTime float64
	Flash        *bool
	LensModel    string
	ColorSpace   string
	WhiteBalance string
	TakenAt      time.Time
	GeoLatitude  float64
	GeoLongitude float64
}

// Extract parses EXIF metadata from raw image bytes.
// Returns nil if no EXIF data is found or parsing fails entirely.
// Individual fields that cannot be parsed are left at zero values.
func Extract(data []byte) *Result {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	r := &Result{}

	if tag, err := x.Get(exif.Orientation); err == nil {
		r.Orientation, _ = tag.Int(0)
	}

	if tag, err := x.Get(exif.Make); err == nil {
		r.CameraMake, _ = tag.StringVal()
	}

	if tag, err := x.Get(exif.Model); err == nil {
		r.CameraModel, _ = tag.StringVal()
	}

	if tag, err := x.Get(exif.FocalLength); err == nil {
		num, den, _ := tag.Rat2(0)
		if den != 0 {
			r.FocalLength = float64(num) / float64(den)
		}
	}

	if tag, err := x.Get(exif.ISOSpeedRatings); err == nil {
		r.ISOSpeed, _ = tag.Int(0)
	}

	if tag, err := x.Get(exif.FNumber); err == nil {
		num, den, _ := tag.Rat2(0)
		if den != 0 {
			r.FNumber = float64(num) / float64(den)
		}
	}

	if tag, err := x.Get(exif.ExposureTime); err == nil {
		num, den, _ := tag.Rat2(0)
		if den != 0 {
			r.ExposureTime = float64(num) / float64(den)
		}
	}

	if tag, err := x.Get(exif.Flash); err == nil {
		v, _ := tag.Int(0)
		fired := v&0x01 != 0
		r.Flash = &fired
	}

	if tag, err := x.Get(exif.LensModel); err == nil {
		r.LensModel, _ = tag.StringVal()
	}

	if tag, err := x.Get(exif.ColorSpace); err == nil {
		v, _ := tag.Int(0)
		switch v {
		case 1:
			r.ColorSpace = "sRGB"
		case 0xFFFF:
			r.ColorSpace = "Uncalibrated"
		}
	}

	if tag, err := x.Get(exif.WhiteBalance); err == nil {
		v, _ := tag.Int(0)
		switch v {
		case 0:
			r.WhiteBalance = "Auto"
		case 1:
			r.WhiteBalance = "Manual"
		}
	}

	if t, err := x.DateTime(); err == nil {
		r.TakenAt = t
	}

	if lat, lon, err := x.LatLong(); err == nil {
		r.GeoLatitude = lat
		r.GeoLongitude = lon
	} else if err != nil {
		slog.Debug("exifmeta: no GPS data", "error", err)
	}

	return r
}
