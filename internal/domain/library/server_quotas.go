package library

// ServerQuotas is a value object: the Default Quota and the Shared
// Library's Quota, as the config sets them or as they hold now.
type ServerQuotas struct {
	Default Quota
	Shared  Quota
}
