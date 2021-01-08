package hls

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/turtletowerz/go-hls/m3u8"
)

var (
	tempStorage string = filepath.Join(os.TempDir(), "hls-go")
	emptyKey           = []byte{0}
)

// ProgressFunc represents the function type
// required to be passed to the SetProgressFunc method
type ProgressFunc func(int, int) error

// Downloader is the struct which contains
// all of the information and methods to download
type Downloader struct {
	sync.Mutex
	client   *http.Client
	quality  string
	threads  int
	baseURL  string
	keyCache []*m3u8.Key
	progress ProgressFunc
}

// SetProgressFunc assigns a function that gets
// called after every new segment that is downloaded
func (d *Downloader) SetProgressFunc(f ProgressFunc) {
	d.progress = f
}

// SetBaseURL sets an optional Base URL that will be
// used if segments are paths as opposed to proper URLS
func (d *Downloader) SetBaseURL(base string) {
	d.baseURL = base
}

func (d *Downloader) downloadSegment(segment *m3u8.Segment, index int, mediaSequence int64) error {
	if !strings.HasPrefix(segment.URI, "http") {
		segment.URI = d.baseURL + segment.URI
	}

	resp, err := d.client.Get(segment.URI)
	if err != nil {
		return fmt.Errorf("getting segment uri: %w", err)
	}

	defer resp.Body.Close()
	file, err := os.Create(filepath.Join(tempStorage, strconv.Itoa(index)+".ts"))
	if err != nil {
		return fmt.Errorf("creating ts file: %w", err)
	}

	defer file.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading segment response: %w", err)
	}

	noKey := segment.KeyIndex == -1
	var key *m3u8.Key
	if !noKey {
		key = d.keyCache[segment.KeyIndex]
		noKey = bytes.Equal(key.Value, m3u8.EmptyKey)
	}

	var out []byte
	if !noKey {
		length := len(respBytes)
		if length%aes.BlockSize != 0 {
			return fmt.Errorf("data is not a valid multiple of aes block size")
		}

		block, err := aes.NewCipher(key.Value)
		if err != nil {
			return fmt.Errorf("creating aes cipher: %w", err)
		}

		var iv []byte
		if key.IV != "" {
			iv = []byte(key.IV)
		} else {
			iv = []byte(fmt.Sprintf("%016d", mediaSequence))
		}

		data := make([]byte, length)
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(data, respBytes)
		out = data[:(length - int(data[length-1]))]
	}

	// Credits to github.com/oopsguy/m3u8 for this
	// Remove all bytes before SyncByte so TS files can be merged
	syncByte := uint8(71) // 0x47
	for j := 0; j < len(out); j++ {
		if out[j] == syncByte {
			out = out[j:]
			break
		}
	}

	if _, err := file.Write(out); err != nil {
		return fmt.Errorf("writing segment to file: %w", err)
	}
	/*
		// TODO
			d.complete++
			if d.progress != nil {
				if err := d.progress(d.complete, d.m3u8.SegmentCount); err != nil {
					return fmt.Errorf("progress func error: %w", err)
				}
			}
	*/
	return nil
}

func (d *Downloader) downloadMediaPlaylist(playlist *m3u8.MediaPlaylist, output, subs, format string) error {
	// TODO: maybe reimplement actual key caching?
	for _, key := range playlist.Keys {
		if err := key.Load(d.client, d.baseURL); err != nil {
			return fmt.Errorf("loading key value: %w", err)
		}
	}
	d.keyCache = playlist.Keys

	segCount := len(playlist.Segments)
	indexes := make([]int, segCount, segCount)
	concatStr := "concat:0.ts"
	for i := 0; i < segCount; i++ {
		indexes[i] = i
		concatStr += "|" + strconv.Itoa(i) + ".ts"
	}

	var wg sync.WaitGroup
	for i := 0; i < d.threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				d.Lock()
				if len(indexes) == 0 {
					break
				}
				idx := indexes[0]
				indexes = indexes[1:]
				segment := playlist.Segments[idx]
				d.Unlock()

				if err := d.downloadSegment(segment, idx, playlist.MediaSequence); err != nil {
					fmt.Printf("error downloading segment %d (returning to queue): %v\n", idx, err)
					d.Lock()
					indexes = append(indexes, idx)
					d.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	cmd := exec.Command("ffmpeg", "-i", concatStr, "-c", "copy", "-y", output)
	//"-metadata", `encoding_tool="no_variable_data"`, "-y", d.filename)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running command: " + stderr.String())
	}
	return nil
}

// Download downloads the supplied stream url and subtitles.
// If the subtitle url is empty, Download will ignore the subtitles
func (d *Downloader) Download(output, stream, subs, format string) error {
	os.Mkdir(tempStorage, os.ModePerm)
	defer d.Close()

	maplaylist, err := m3u8.DecodeURL(stream)
	if err != nil {
		return fmt.Errorf("decoding m3u8 playlist to url: %w", err)
	}

	if typ := maplaylist.Type(); typ == m3u8.TypeMedia {
		if err := d.downloadMediaPlaylist(maplaylist.(*m3u8.MediaPlaylist), output, subs, format); err != nil {
			return fmt.Errorf("downloading media segment (1): %w", err)
		}
	}

	master := maplaylist.(*m3u8.MasterPlaylist)
	variants := master.Variants
	sort.SliceStable(variants, func(i, j int) bool { return variants[i].Resolution.Height < variants[j].Resolution.Height })

	var best *m3u8.Variant
	switch strings.ToLower(d.quality) {
	case "best":
		best = &variants[len(variants)-1]
	case "worst":
		best = &variants[0]
	default:
		split := strings.Split(d.quality, "x")
		if width, err := strconv.ParseInt(split[0], 10, 64); err == nil {
			for _, vars := range variants {
				if vars.Resolution.Width == width {
					best = &vars
					break
				}
			}
		}
	}

	if best == nil {
		return fmt.Errorf("no good string found for quality %q", d.quality)
	}

	meplaylist, err := m3u8.DecodeURL(best.URI)
	if err != nil {
		return fmt.Errorf("getting media playlist from master: %w", err)
	}

	if typ := meplaylist.Type(); typ != m3u8.TypeMedia {
		return fmt.Errorf("got master playlist from master playlist url (?)")
	}

	media := meplaylist.(*m3u8.MediaPlaylist)
	if err := d.downloadMediaPlaylist(media, output, subs, format); err != nil {
		return fmt.Errorf("downloading media segment (2): %w", err)
	}
	return nil
}

// Close performs any cleanup necessary after the mpd download completes. This
// is called internally by Download, so it is not typically necessary to call
func (d *Downloader) Close() {
	os.RemoveAll(tempStorage)
}

// New creates a new downloader for the user to download content with
func New(client *http.Client, quality string, threads int) *Downloader {
	return &Downloader{client: client, quality: quality, threads: threads}
}
