package hls

import (
	"net/http"
	"testing"
)

const (
	filename string = "test.mkv"
	m3u8test string = "example.m3u8"
)

// This probably won't work for most tests (vscode times out after 30 seconds while it usually takes ~70 to complete the download)
func TestDownloader(t *testing.T) {
	dl, err := New(&http.Client{}, m3u8test, filename)
	if err != nil {
		t.Fatal("Error: " + err.Error())
	}

	if err := dl.Download(10); err != nil {
		t.Fatal("Error2: " + err.Error())
	}
}
