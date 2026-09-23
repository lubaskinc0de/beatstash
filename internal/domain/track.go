package domain

import "time"

type Track struct {
	ID uint `gorm:"primaryKey"`

	// Path is relative to the Library root.
	Path string `gorm:"uniqueIndex;not null"`

	Metadata
	Quality

	DurationMs int
	Format     Format

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Quality struct {
	Lossless bool
	Bitrate  int
}

// Better tells whether q beats other: lossless beats lossy,
// among lossy files the higher bitrate wins, ties keep other.
func (q Quality) Better(other Quality) bool {
	if q.Lossless != other.Lossless {
		return q.Lossless
	}
	if q.Lossless {
		return false
	}
	return q.Bitrate > other.Bitrate
}

type TelegramFile struct {
	ID   string
	Kind TelegramFileKind
}

type TelegramFileKind string

const (
	TelegramFileAudio    TelegramFileKind = "audio"
	TelegramFileDocument TelegramFileKind = "document"
)

type TrackSource struct {
	ID uint `gorm:"primaryKey"`

	TrackID uint  `gorm:"not null;index"`
	Track   Track `gorm:"constraint:OnDelete:CASCADE;"`

	Provider ProviderName `gorm:"not null;uniqueIndex:idx_track_source_ref"`
	Ref      string       `gorm:"not null;uniqueIndex:idx_track_source_ref"`

	TelegramFileID   string
	TelegramFileKind TelegramFileKind

	CreatedAt time.Time
}

func (s *TrackSource) TelegramFile() *TelegramFile {
	if s.TelegramFileID == "" {
		return nil
	}
	return &TelegramFile{ID: s.TelegramFileID, Kind: s.TelegramFileKind}
}

type Upload struct {
	ID uint `gorm:"primaryKey"`

	UserID uint `gorm:"not null;index"`
	User   User `gorm:"constraint:OnDelete:CASCADE;"`

	TrackID uint  `gorm:"not null;index"`
	Track   Track `gorm:"constraint:OnDelete:CASCADE;"`

	TrackSourceID uint        `gorm:"not null"`
	TrackSource   TrackSource `gorm:"constraint:OnDelete:CASCADE;"`

	CreatedAt time.Time
}
