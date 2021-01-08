package m3u8

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TODO: Figure out a good way to detect whether EXT-X-KEY is for the whole media playlist or for a media segment
const (
	// TYPE_MASTER and TYPE_MEDIA are enums that are
	// returned from DecodeReader and DecodeURL to provide
	// the user with a way to determine the playlist type
	TypeMaster = iota
	TypeMedia

	PlaylistVOD   string = "VOD"
	PlaylistEvent string = "EVENT"

	CryptNone      string = "NONE"
	CryptAES       string = "AES-128"
	CryptSampleAES string = "SAMPLE-AES"

	HDCPLevel0    string = "TYPE0"
	HDCPLevelNone string = "NONE"

	MediaAudio     string = "AUDIO"
	MediaVideo     string = "VIDEO"
	MediaSubtitles string = "SUBTITLES"
	MediaCaptions  string = "CLOSED-CAPTIONS"

	PreciseYes string = "YES"
	CCNone     string = "NONE"

	MediaDefaultYES string = "YES"
	MediaDefaultNO  string = "NO"
)

// EmptyKey represents an empty key response
var EmptyKey = []byte{0}

const (
	tagRegex string = `#([A-Z-]+):?(.+)?`

	// Defined in Section 4.2
	// end me please
	attrRegex string = `([A-Z0-9-]+)=(0[xX][0-9A-F]+|[x0-9.-]+|[A-Z0-9-]+|"?[^\x0A\x0D\x22]+"?)`

	// Longer version for comparison ...
	// ([A-Z0-9-]+)=(0[xX][0-9A-F]+|[0-9\.-]+|[0-9\.]+|[0-9]+|[A-Z0-9-]+|"?[^\x0A\x0D\x22\x2C]+"?)

	// Ugly regex for properly identifying INSTREAM-ID
	// See regex test cases above for what worked and didn't
	instreamRegex string = `"(CC[1-4]|SERVICE[1-5][0-9]?|SERVICE6[0-3])"`
)

// TODO: remove the random pointers in structs

// Playlist represents an interface that
// MasterPlaylist and MediaPlaylist fall under
type Playlist interface {
	Type() int
	//Decode(string) error
}

// DecodeReader creates a playlist and determines the type. It is recommended that
// this method be used when a m3u8 file is present, and DecodeURL be used with a URL
func DecodeReader(reader io.Reader) (playlist Playlist, err error) {
	var isMedia bool
	var lines []string

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if strings.TrimSpace(text) != "" { // Ignore stupid whitespace
			lines = append(lines, text)
		}

		// 4.3.3   - "A Media Playlist tag MUST NOT appear in a Master Playlist."
		// 4.3.3.1 - "The EXT-X-TARGETDURATION tag is REQUIRED."
		if strings.HasPrefix(text, "#EXT-X-TARGETDURATION") {
			isMedia = true
		}
	}

	if len(lines) == 0 || lines[0] != "#EXTM3U" {
		err = fmt.Errorf(`provided reader is not a valid m3u8 file (does not contain header "#EXTM3U")`)
		return
	}

	// Remove the header tag from the split
	lines = lines[1:]

	// This check assumes that all master playlists will have at least one #EXT-X-STREAM-INF. I have found no proof that Master Playlists can be made without these, so if you find an example please open an issue with it
	if isMedia {
		if playlist, err = parseMediaPlaylist(lines); err != nil {
			err = fmt.Errorf("parsing media playlist: %w", err)
		}
	} else {
		if playlist, err = parseMasterPlaylist(lines); err != nil {
			err = fmt.Errorf("parsing master playlist: %w", err)
		}
	}
	return
}

// DecodeURL passes a URL to DecodeReader. Easier for downloading from websites
func DecodeURL(url string) (playlist Playlist, err error) {
	resp, err := http.Get(url)
	if err != nil {
		err = fmt.Errorf("getting m3u8 url %q: %w", url, err)
		return
	}

	defer resp.Body.Close()
	if playlist, err = DecodeReader(resp.Body); err != nil {
		err = fmt.Errorf("decoding from reader: %w", err)
	}
	return
}

// MustDecodeReader implements DecodeReader, but panics if an error occurs
func MustDecodeReader(reader io.Reader) Playlist {
	playlist, err := DecodeReader(reader)
	if err != nil {
		panic(err)
	}
	return playlist
}

// MustDecodeURL implements DecodeURL, but panics if an error occurs
func MustDecodeURL(url string) Playlist {
	playlist, err := DecodeURL(url)
	if err != nil {
		panic(err)
	}
	return playlist
}
