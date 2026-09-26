package provider

type ProviderName string

const ProviderTelegram ProviderName = "telegram"

// TrackRef identifies a track inside a Provider. Payload carries whatever
// else the Provider needs to fetch it, as the Provider encoded it.
type TrackRef struct {
	Provider ProviderName
	ID       string
	Payload  string
}
