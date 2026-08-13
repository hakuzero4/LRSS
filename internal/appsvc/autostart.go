package appsvc

import "lrss/internal/autostart"

// GetLaunchAtLogin reports whether the OS will start LRSS at login.
func (s *SettingsService) GetLaunchAtLogin() (autostart.Status, error) {
	return autostart.StatusNow(), nil
}

// SetLaunchAtLogin enables or disables OS launch-at-login, then returns the current status.
func (s *SettingsService) SetLaunchAtLogin(enabled bool) (autostart.Status, error) {
	err := autostart.Set(enabled)
	return autostart.StatusNow(), err
}
