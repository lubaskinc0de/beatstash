// QuotaSettings: the Default Quota and the Shared Library's Quota the Admin
// set in the bot.

package library

import "time"

// QuotaSettings is an aggregate root: the Default Quota and the Shared
// Library's Quota the Admin set, one row for the server. A nil Quota
// follows the config.
type QuotaSettings struct {
	ID uint `gorm:"primaryKey"`

	DefaultQuota *Quota
	SharedQuota  *Quota

	UpdatedAt time.Time
}

// quotaSettingsID is the only row of QuotaSettings.
const quotaSettingsID = 1

func NewQuotaSettings() *QuotaSettings {
	return &QuotaSettings{ID: quotaSettingsID}
}

// Apply returns the server's Quotas that hold now: the Admin's where set,
// the config's elsewhere.
func (s *QuotaSettings) Apply(config ServerQuotas) ServerQuotas {
	if s.DefaultQuota != nil {
		config.Default = *s.DefaultQuota
	}
	if s.SharedQuota != nil {
		config.Shared = *s.SharedQuota
	}
	return config
}

func (s *QuotaSettings) DefaultFromConfig() bool {
	return s.DefaultQuota == nil
}

func (s *QuotaSettings) SharedFromConfig() bool {
	return s.SharedQuota == nil
}

// SetDefault takes nil to follow the config again.
func (s *QuotaSettings) SetDefault(q *Quota) {
	s.DefaultQuota = q
}

// SetShared takes nil to follow the config again.
func (s *QuotaSettings) SetShared(q *Quota) {
	s.SharedQuota = q
}
