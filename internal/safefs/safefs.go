package safefs

import "os"

// ReadFile reads a file from a user-selected path.
// #nosec G304 -- callers intentionally operate on user-selected paths.
func ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// WriteFile writes a file with an explicit mode.
// #nosec G306 -- callers intentionally write to user-selected paths.
func WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// MkdirAll creates a directory tree with an explicit mode.
// #nosec G301 -- callers intentionally manage user-selected paths.
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Chmod changes file permissions.
// #nosec G302 -- update downloads must be made executable.
func Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

// Remove deletes a file if it exists.
// #nosec G104 -- cleanup errors are intentionally ignored by callers.
func Remove(name string) error {
	return os.Remove(name)
}

// Create opens a file for writing with a private mode.
// #nosec G304 -- callers intentionally create files under user-selected directories.
func Create(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
}
