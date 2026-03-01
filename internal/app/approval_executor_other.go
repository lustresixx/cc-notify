//go:build !windows

package app

import "fmt"

type noopApprovalExecutor struct{}

func newDefaultApprovalExecutor() ApprovalExecutor {
	return noopApprovalExecutor{}
}

func (noopApprovalExecutor) Deliver(_ int, _ approvalDecision) error {
	return fmt.Errorf("approval delivery is only supported on windows")
}
