package assets

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
)

//go:embed "html" "static"
var files embed.FS

var (
	HTMLFiles   = sub(files, "html")
	StaticFiles = sub(files, "static")
)

func sub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func ComputeStaticFilesEtags() (map[string]string, error) {
	etags := map[string]string{}
	err := fs.WalkDir(StaticFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		bytes, err := fs.ReadFile(StaticFiles, path)
		if err != nil {
			return err
		}

		etag := sha256.Sum256(bytes)
		etags[fmt.Sprintf("/static/%s", path)] = fmt.Sprintf(`W/"%x"`, etag[:16])
		return nil
	})
	if err != nil {
		return nil, err
	}

	return etags, nil
}
