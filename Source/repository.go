package main

// LabPassRepository is the application-facing storage boundary. Domain/UI code
// must not depend on SQLite details. A future server adapter can implement the
// same contract without changing laboratory calculations or screens.
type LabPassRepository interface {
	Save(pass LabPassRecord) error
	LoadAll() ([]LabPassRecord, error)
	LoadForObject(objectKey string) ([]LabPassRecord, error)
	LoadLatestSaved() ([]LabPassRecord, error)
}

// LabPassBatchRepository is an optional fast path for trusted application
// batches (currently the built-in teaching set).  Ordinary user saves still go
// through Save one record at a time.  Keeping this capability optional preserves
// compatibility with the legacy JSONL fallback and future repository adapters.
type LabPassBatchRepository interface {
	SaveBatch(passes []LabPassRecord) error
}

// repositoryFailurePolicy documents the fail-safe behaviour used by callers.
// Reads may fall back to the legacy mirror during the transition; writes are
// considered durable only after at least one durable backend succeeds.
type repositoryFailurePolicy struct {
	AllowLegacyReadFallback bool
	RequireDurableWrite     bool
}

var rosaRepositoryPolicy = repositoryFailurePolicy{
	AllowLegacyReadFallback: true,
	RequireDurableWrite:     true,
}
