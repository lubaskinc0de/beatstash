package library

import (
	"path/filepath"
	"strings"
)

type Format string

const (
	FormatMP3  Format = "mp3"
	FormatFLAC Format = "flac"
	FormatM4A  Format = "m4a"
	FormatOGG  Format = "ogg"
	FormatOpus Format = "opus"
	FormatWAV  Format = "wav"
)

var formatsByExt = map[string]Format{
	".mp3":  FormatMP3,
	".flac": FormatFLAC,
	".m4a":  FormatM4A,
	".mp4":  FormatM4A,
	".ogg":  FormatOGG,
	".oga":  FormatOGG,
	".opus": FormatOpus,
	".wav":  FormatWAV,
}

var formatsByMime = map[string]Format{
	"audio/mpeg":   FormatMP3,
	"audio/mp3":    FormatMP3,
	"audio/flac":   FormatFLAC,
	"audio/x-flac": FormatFLAC,
	"audio/mp4":    FormatM4A,
	"audio/m4a":    FormatM4A,
	"audio/x-m4a":  FormatM4A,
	"audio/ogg":    FormatOGG,
	"audio/opus":   FormatOpus,
	"audio/wav":    FormatWAV,
	"audio/x-wav":  FormatWAV,
	"audio/wave":   FormatWAV,
}

func FormatOf(fileName, mimeType string) (Format, bool) {
	if format, ok := formatsByExt[strings.ToLower(filepath.Ext(fileName))]; ok {
		return format, true
	}
	format, ok := formatsByMime[strings.ToLower(strings.TrimSpace(mimeType))]
	return format, ok
}

func (f Format) Ext() string {
	return "." + string(f)
}
