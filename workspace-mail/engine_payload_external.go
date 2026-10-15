//go:build sharedengine

package main

import (
	"os"
	"path/filepath"
)

func loadPrivateEngine() ([]byte, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(filepath.Dir(self), "stalwart.exe"))
}

func privateEngineDefault() bool { return os.Getenv("WORKSPACE_MAIL_BACKEND") == "private" }
