// TrackRef: a track as a Provider names it.

package provider

// TrackRef is a value object: a track in a Provider. Payload holds whatever
// else the Provider needs to fetch it.
type TrackRef struct {
	Provider ProviderName
	ID       string
	Payload  string
}

// ProviderName is a value object.
type ProviderName string

const ProviderTelegram ProviderName = "telegram"
