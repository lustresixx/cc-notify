//go:build !windows

package notifier

type noopNotifier struct{}

// New returns a no-op notifier on non-Windows platforms.
func New() Service {
	return noopNotifier{}
}

// NewWithConfig returns a no-op notifier on non-Windows platforms.
func NewWithConfig(_ Config) Service {
	return noopNotifier{}
}

func (noopNotifier) Notify(_, _ string) error {
	return nil
}
