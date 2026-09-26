// Package ingest: adding audio to a Library.
//
// Aggregates:
//   - IngestJob: one track to add. Goes from pending to done or failed;
//     failed attempts are retried by a RetryPolicy.
//   - IngestBatch: the jobs of one Import. Its progress is counted from its
//     jobs by a repository query.
//
// Value objects: RetryPolicy, FailureReason.
//
// Fields are exported only because gorm needs them. Change an aggregate
// only through its methods.
package ingest
