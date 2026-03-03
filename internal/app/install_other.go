//go:build !windows

package app

func ensureToastShortcut(exePath, appID string) error {
	return nil
}

func ensureURIProtocol(exePath string) error {
	return nil
}
