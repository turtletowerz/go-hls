package hls

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/turtletowerz/go-hls/m3u8"
)

var (
	tempStorage    string = filepath.Join(os.TempDir(), "hls-go")
	errNoKeyNeeded error  = fmt.Errorf("ERR-NO-KEY-NEEDED")
)

// ProgressFunc represents the function type
// required to be passed to the SetProgressFunc method
type ProgressFunc func(int, int) error

// Downloader is the struct which contains
// all of the information and methods to download
type Downloader struct {
	sync.Mutex
	client   *http.Client
	m3u8     *m3u8.MediaPlaylist
	keyCache map[int][]byte
	index    []int
	filename string
	complete int
	progress ProgressFunc
}

func (d *Downloader) getKey(segment *m3u8.Segment) error {
	// TODO: apparently there may be some cases where key isn't a url?
	if segment.KeyIndex == -1 {
		return errNoKeyNeeded
	}

	if _, exists := d.keyCache[segment.KeyIndex]; exists == true {
		return nil
	}

	key := d.m3u8.Keys[segment.KeyIndex]
	if key.Method != m3u8.CryptAES {
		if key.Method == m3u8.CryptNone {
			return errNoKeyNeeded
		}
		return fmt.Errorf("this module does not support aes sample keys, sorry")
	}

	resp, err := d.client.Get(key.URI)
	if err != nil {
		return fmt.Errorf("getting key uri: %w", err)
	}

	defer resp.Body.Close()
	keyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading key bytes: %w", err)
	}

	d.keyCache[segment.KeyIndex] = keyBytes
	return nil
}

func (d *Downloader) downloadSegment(idx int) error {
	segment := d.m3u8.Segments[idx]
	resp, err := d.client.Get(segment.URI)
	if err != nil {
		return fmt.Errorf("getting segment uri: %w", err)
	}

	defer resp.Body.Close()
	file, err := os.Create(filepath.Join(tempStorage, strconv.Itoa(idx)+".ts"))
	if err != nil {
		return fmt.Errorf("creating ts file: %w", err)
	}

	defer file.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading segment response: %w", err)
	}

	keyErr := d.getKey(&segment)
	if keyErr != nil && keyErr != errNoKeyNeeded {
		return fmt.Errorf("getting decryption key: %w", err)
	}

	if keyErr != errNoKeyNeeded {
		segKey := d.m3u8.Keys[segment.KeyIndex]
		if len(respBytes)%aes.BlockSize != 0 {
			return fmt.Errorf("data is not a valid multiple of aes block size")
		}

		block, err := aes.NewCipher(d.keyCache[segment.KeyIndex])
		if err != nil {
			return fmt.Errorf("creating aes cipher: %w", err)
		}

		iv := []byte(fmt.Sprintf("%016d", d.m3u8.MediaSequence))
		if segKey.IV != nil {
			iv = []byte(*segKey.IV)
		}

		length := len(respBytes)
		data := make([]byte, length)
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(data, respBytes)

		// idk if this is specific to VRV/Crunchyroll or not....
		respBytes = data[:(length - int(data[length-1]))]
	}

	// Credits to github.com/oopsguy/m3u8 for this
	// Remove all bytes before SyncByte to TS files can be merged
	syncByte := uint8(71) // 0x47
	for j := 0; j < len(respBytes); j++ {
		if respBytes[j] == syncByte {
			respBytes = respBytes[j:]
			break
		}
	}

	if _, err := file.Write(respBytes); err != nil {
		return fmt.Errorf("writing segment to file: %w", err)
	}

	d.complete++
	if d.progress != nil {
		d.progress(d.complete, d.m3u8.SegmentCount)
	}
	return nil
}

func (d *Downloader) nextSegment() (idx int) {
	d.Lock()
	defer d.Unlock()
	if len(d.index) > 0 {
		idx, d.index = d.index[0], d.index[1:]
	} else {
		idx = -1
	}
	return
}

func (d *Downloader) merge() error {
	fpath := filepath.Join(tempStorage, "list.txt")
	file, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	defer file.Close()

	w := bufio.NewWriter(file)
	for i := 0; i < d.m3u8.SegmentCount; i++ {
		w.WriteString(fmt.Sprintf("file '%s'\n", filepath.Join(tempStorage, strconv.Itoa(i)+".ts")))
	}
	w.Flush()

	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", fpath, "-c", "copy", "-y", d.filename)
	//"-metadata", `encoding_tool="no_variable_data"`, "-y", d.filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running command")
	}
	return nil
}

func (d *Downloader) Download(channels int) error {
	defer os.RemoveAll(tempStorage)
	// TODO: add some sort of error channel so if the segment fails it gets added to the error channel, if the chan is maxed out then return an error
	var wg sync.WaitGroup

	for i := 0; i < channels; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for {
				idx := d.nextSegment()
				if idx == -1 {
					break
				}

				if err := d.downloadSegment(idx); err != nil {
					fmt.Printf("error downloading segment %d (returning to queue): %v\n", idx, err)
					d.Lock()
					defer d.Unlock()
					d.index = append(d.index, idx)
				}
			}
		}()
	}

	wg.Wait()

	if err := d.merge(); err != nil {
		return fmt.Errorf("merging files: %w", err)
	}
	return nil
}

// SetProgressFunc assigns a function that gets
// called after every new segment that is downloaded
func (d *Downloader) SetProgressFunc(f ProgressFunc) {
	d.progress = f
}

// New creates a new downloader for the user to download content with
func New(client *http.Client, path, filename string) (*Downloader, error) {
	os.RemoveAll(tempStorage)
	os.Mkdir(tempStorage, os.ModePerm)
	var playlist m3u8.Playlist
	var err error

	if strings.HasPrefix(path, "http") {
		playlist, err = m3u8.DecodeURL(path)
	} else {
		file, err := os.Open(path)
		if err != nil {
			err = fmt.Errorf("opening file %q: %w", path, err)
		} else {
			defer file.Close()
			playlist, err = m3u8.DecodeReader(file)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("decoding m3u8: %w", err)
	}

	if playlist.Type() != m3u8.TypeMedia {
		return nil, fmt.Errorf("playlist type MUST be media")
	}

	media := playlist.(*m3u8.MediaPlaylist)

	if client == nil { // If nil, create an empty http client to use
		client = &http.Client{}
	}

	download := &Downloader{
		client:   client,
		m3u8:     media,
		keyCache: map[int][]byte{},
		index:    make([]int, media.SegmentCount),
		filename: filename,
	}

	for i := 0; i < media.SegmentCount; i++ {
		download.index[i] = i
	}
	return download, nil
}
