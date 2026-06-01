package hdx

import (
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

func ResolvePath(path string) (string, error) {
	p, err := homedir.Expand(path)
	if err != nil {
		return "", err
	}

	p, err = filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return p, nil
}

func ResolveFile(filePath string) (string, error) {
	fp, err := homedir.Expand(filePath)
	if err != nil {
		return "", err
	}

	fp, err = filepath.Abs(fp)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(fp); os.IsNotExist(err) {
		return "", err
	}
	return fp, nil
}
