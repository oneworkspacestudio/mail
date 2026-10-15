//go:build !sharedengine

package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"errors"
	"io"
	"os"
)

//go:embed payload/stalwart.zip
var privateEngineArchive []byte

func loadPrivateEngine() ([]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(privateEngineArchive), int64(len(privateEngineArchive)))
	if err != nil {
		return nil, err
	}
	for _, file := range archive.File {
		if file.Name == "stalwart.exe" {
			reader, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			return io.ReadAll(io.LimitReader(reader, 200*1024*1024))
		}
	}
	return nil, errors.New("The embedded Mail engine is missing")
}

func privateEngineDefault() bool { return os.Getenv("WORKSPACE_MAIL_BACKEND") != "shared" }
