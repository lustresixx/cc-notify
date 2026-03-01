package app

// approvalDecision is the user choice for a paused run.
type approvalDecision string

const (
	approvalApprove approvalDecision = "approve"
	approvalReject  approvalDecision = "reject"
)

// ApprovalExecutor applies a decision to the paused interactive session.
// Current implementation uses foreground terminal key injection.
// A future broker-based flow can implement this interface without changing app command handling.
type ApprovalExecutor interface {
	Deliver(parentPID int, decision approvalDecision) error
}
