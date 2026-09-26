package ingest

// FailureReason is a value object: why an IngestJob failed and will not be retried.
type FailureReason string

const (
	ReasonUnsupportedFormat FailureReason = "unsupported_format"
	ReasonCorruptFile       FailureReason = "corrupt_file"
	ReasonFetchFailed       FailureReason = "fetch_failed"
	ReasonNoProviderAccount FailureReason = "no_provider_account"
	ReasonTokenRejected     FailureReason = "token_rejected"
	ReasonInternal          FailureReason = "internal"
)
