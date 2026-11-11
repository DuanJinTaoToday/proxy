package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const version = "28.3"

var releases = map[string]struct {
	platform string
	sha256   string
}{
	"windows/amd64": {"win64", "ce64f49bdeddef49ce4bd313a8f59bcf92fcf67b5831efbf66170386d2e66948"},
	"linux/amd64":   {"linux-x86_64", "0ad949f04a6a174da83cdcbdb36dee0a4925272a5b6d83f79a6bf9852076d53f"},
	"linux/arm64":   {"linux-aarch_64", "1de522032a8b194002fe35cab86d747848238b5e4de4f99648372079f5b46f9a"},
	"darwin/amd64":  {"osx-x86_64", "97fe5d442090b4dbc23cd1384fb9b444fa1dc6e67d15bb5e1fe4de0da7638b20"},
	"darwin/arm64":  {"osx-aarch_64", "92ceefda6a7293ec014e6ecac82d64719357145cb6fc2865badadeb5e62c0431"},
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: install-protoc <tools-directory>")
	}
	if err := install(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}

func install(root string) error {
	host := runtime.GOOS + "/" + runtime.GOARCH
	release, ok := releases[host]
	if !ok {
		return fmt.Errorf("protoc %s is not configured for %s", version, host)
	}

	url := fmt.Sprintf("https://github.com/protocolbuffers/protobuf/releases/download/v%s/protoc-%s-%s.zip", version, version, release.platform)
	client := http.Client{Timeout: time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download protoc %s: %s", version, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != release.sha256 {
		return fmt.Errorf("protoc %s archive checksum mismatch for %s", version, host)
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	binary := "bin/protoc"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	for _, file := range archive.File {
		if file.Name != binary && (!strings.HasPrefix(file.Name, "include/") || !file.Mode().IsRegular()) {
			continue
		}
		path, err := filepath.Localize(file.Name)
		if err != nil {
			return err
		}
		if err := extract(file, filepath.Join(root, path)); err != nil {
			return fmt.Errorf("install %s: %w", file.Name, err)
		}
	}
	fmt.Printf("Installed protoc %s to %s\n", version, filepath.Join(root, binary))
	return nil
}

func extract(file *zip.File, path string) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	data, err := io.ReadAll(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, file.Mode().Perm())
}
