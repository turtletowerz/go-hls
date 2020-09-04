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
	"github.com/turtletowerz/go-hls/progressbar"
)

var (
	tempStorage    string = filepath.Join(os.TempDir(), "hls-go")
	errNoKeyNeeded error  = fmt.Errorf("ERR-NO-KEY-NEEDED")
)

// Downloader is the struct which contains
// all of the information and methods to download
type Downloader struct {
	sync.Mutex
	client   *http.Client
	m3u8     *m3u8.MediaPlaylist
	keyCache map[int][]byte
	index    []int
	filename string
	bar      *progressbar.Bar
	complete int
}

func (d *Downloader) getKey(segment *m3u8.Segment) error {
	// TODO: apparently there may be some cases where key isn't a url
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
	d.Lock() // is this process really necessary?
	segment := d.m3u8.Segments[idx]
	d.Unlock()

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
		keyBytes := d.keyCache[segment.KeyIndex]

		if len(respBytes)%aes.BlockSize != 0 {
			return fmt.Errorf("data is not a valid multiple of aes block size")
		}

		block, err := aes.NewCipher(keyBytes)
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
		//mode := cipher.NewCBCDecrypter(block, iv)
		//mode.CryptBlocks(data, respBytes)

		// idk if this is specific to VRV/Crunchyroll or not....
		respBytes = data[:(length - int(data[length-1]))]
	}

	// Credits to github.com/oopsguy/m3u8 for this
	// Some TS files do not start with SyncByte 0x47, they can not be played after merging,
	// Need to remove the bytes before the SyncByte 0x47(71).
	syncByte := uint8(71) // 0x47
	for j := 0; j < len(respBytes); j++ {
		if respBytes[j] == syncByte {
			respBytes = respBytes[j:]
			break
		}
	}

	if _, err := file.Write(respBytes); err != nil {
		return fmt.Errorf("writing bytes to file: %w", err)
	}
	//d.bar.Add(1)
	d.complete++
	if d.complete%50 == 0 {
		fmt.Printf("%d / %d segments downloaded\n", d.complete, d.m3u8.Count())
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
	defer os.RemoveAll(tempStorage)
	/*
		concatStr := "concat:"

		err := filepath.Walk(tempStorage, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			concatStr = concatStr + path + "|"
			return nil
		})

		concatStr = concatStr[:len(concatStr)-1]

		if err != nil {
			return fmt.Errorf("walking through files: %w", err)
		}

		cmd := exec.Command("ffmpeg", "-i", fmt.Sprintf("%q", concatStr), "-c", "copy", "-y", d.filename)
		//"-metadata", `encoding_tool="no_variable_data"`, "-y", d.filename)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stdout
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error running command")
		}
	*/
	fmt.Println("Merging segments...")
	fpath := filepath.Join(tempStorage, "list.txt")
	file, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	defer file.Close()

	w := bufio.NewWriter(file)
	for i := 0; i < d.m3u8.Count(); i++ {
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
	// TODO: add some sort of error channel so if the segment fails it gets added to the error chan, if the chan is maxed out then return an error
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
	//d.bar.Done()

	if err := d.merge(); err != nil {
		return fmt.Errorf("merging files: %w", err)
	}
	return nil
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
		return nil, fmt.Errorf("Error: Playlist type MUST be media")
	}

	media := playlist.(*m3u8.MediaPlaylist)
	seglen := media.Count()

	//fmt.Printf("%v\n", media)

	download := &Downloader{
		client:   client,
		m3u8:     media,
		keyCache: map[int][]byte{},
		index:    make([]int, seglen),
		filename: filename,
		bar:      progressbar.New(seglen),
	}

	for i := 0; i < seglen; i++ {
		download.index[i] = i
	}
	return download, nil
}
