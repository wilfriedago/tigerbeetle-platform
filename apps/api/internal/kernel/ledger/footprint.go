package ledger

// Footprint is what a ledger holds so far: the assets used, in canonical form and sorted, and the
// number of addresses. The bookkeeping module computes it; the ledgers module shows it with each
// ledger.
type Footprint struct {
	Assets   []string
	Accounts int64
}
