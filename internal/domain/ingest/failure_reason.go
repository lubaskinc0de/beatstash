// FailureReason: why an Ingest Job failed and will not be retried.

package ingest

// FailureReason is a value object: why an IngestJob failed and will not be retried.
type FailureReason string

const (
	ReasonUnsupportedFormat FailureReason = "unsupported_format"
	ReasonCorruptFile       FailureReason = "corrupt_file"
	ReasonFetchFailed       FailureReason = "fetch_failed"
	ReasonNoProviderAccount FailureReason = "no_provider_account"
	ReasonTokenRejected     FailureReason = "token_rejected"
	ReasonQuotaExceeded     FailureReason = "quota_exceeded"
	ReasonInternal          FailureReason = "internal"
)

// AwaitsRoom reports that the job failed because the Library had no room
// for the Track: trying again makes sense once the Library has room.
func (r FailureReason) AwaitsRoom() bool {
	return r == ReasonQuotaExceeded
}
