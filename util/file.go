package util

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadAsString reads the contents of a file as a string.
func ReadAsString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetWorkingDir returns the current working directory.
func GetWorkingDir() (string, error) {
	return os.Getwd()
}

// SetWorkingDir sets the current working directory.
func SetWorkingDir(dir string) error {
	return os.Chdir(dir)
}

// IsExist checks whether a file exists.
func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetExecutablePath returns the path of the current executable.
func GetExecutablePath() (string, error) {
	return os.Executable()
}

// GetExecutableDir returns the directory of the current executable.
func GetExecutableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// GetVersion extracts version from a path like "library-x.y.z".
func GetVersion(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	if i := strings.LastIndex(name, "-"); i >= 0 {
		return name[i+1:]
	}
	return "unknown"
}
