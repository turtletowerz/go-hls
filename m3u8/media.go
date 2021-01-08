package m3u8

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

// Map represents
type Map struct { // 4.3.2.5
	URI       string
	ByteRange string
}

// Key contains information for decrypting encrypted segments
type Key struct { // 4.3.2.4
	Method      string
	URI         string
	IV          string
	KeyFormat   string
	KeyVersions string
	Value       []byte
}

// Load loads the key into the Value field, using
// client as the request client and base and a base url if necessary
func (k *Key) Load(client *http.Client, base string) error {
	if k.Method != CryptAES {
		if k.Method == CryptNone {
			k.Value = EmptyKey
			return nil
		}
		return fmt.Errorf("this parser does not yet support aes sample keys")
	}

	resp, err := client.Get(k.URI)
	if err != nil {
		return fmt.Errorf("getting key response: %w", err)
	}

	defer resp.Body.Close()
	k.Value, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("getting key bytes: %w", err)
	}
	return nil
}

// Segment represents an individual media segment from a MediaPlaylist
type Segment struct { // 4.3.2
	URI           string
	Duration      float32
	Title         string
	ByteRange     int
	Offset        int
	Discontinuity bool
	DateTime      string
	KeyIndex      int
	Map           *Map
	// TODO: 4.3.2.7.  EXT-X-DATERANGE
}

// MediaPlaylist represents a MediaPlaylist M3U8 file
type MediaPlaylist struct { // 4.3.3
	Segments         []*Segment
	Keys             []*Key
	TargetDuration   int64
	MediaSequence    int64
	DiscontinuitySeq int64 // defaults to 0
	PType            string
	IFramesOnly      bool
	Independent      bool
	TimeOffset       float32
	Precise          bool
	Version          int
}

// Type returns media playlist type
func (m *MediaPlaylist) Type() int {
	return TypeMedia
}

func parseAttributes(line string) map[string]string { // 4.2
	attribs := make(map[string]string)
	linePattern := regexp.MustCompile(attrRegex)
	if matches := linePattern.FindAllStringSubmatch(line, -1); matches != nil {
		for _, arr := range matches {
			attribs[arr[1]] = arr[2] // strings.Trim(arr[2], "\"")
		}
	}
	return attribs
}

func parseKey(attrib string) (*Key, error) {
	key := new(Key)
	attributes := parseAttributes(attrib)

	for attrib, value := range attributes {
		var err error
		switch attrib {
		case "METHOD":
			if value != CryptNone && value != CryptAES && value != CryptSampleAES {
				return nil, fmt.Errorf("invalid key METHOD value %q", value)
			}
			key.Method = value
		case "URI":
			_, err = fmt.Sscanf(value, "%q", &key.URI)
		case "IV":
			_, err = fmt.Sscanf(value, "%X", &key.IV)
		case "KEYFORMAT":
			_, err = fmt.Sscanf(value, "%q", &key.KeyFormat)
		case "KEYFORMATVERSIONS":
			_, err = fmt.Sscanf(value, "%q", &key.KeyVersions)
		}

		if err != nil {
			return nil, fmt.Errorf("error parsing session data attribute %s: %w", attrib, err)
		}
	}

	if key.Method != CryptNone && key.URI == "" {
		return nil, fmt.Errorf("if URI is empty, METHOD MUST be NONE")
	}
	return key, nil
}

func parseMediaSegment(lines []string, last, current, keyIndex int) (segment Segment, err error) {
	segment.URI = lines[current]
	segment.KeyIndex = keyIndex

	for i := last; i < current; i++ {
		results := regexp.MustCompile(tagRegex).FindStringSubmatch(lines[i])
		if results != nil {
			switch results[1] {
			case "EXTINF": // 4.3.2.1
				options := strings.Split(results[2], ",")
				if _, err = fmt.Sscanf(options[0], "%f", &segment.Duration); err == nil && len(options) > 1 && options[1] != "" {
					segment.Title = options[1]
				}
			case "EXT-X-BYTERANGE": // 4.3.2.2
				options := strings.Split(results[2], "@")
				if _, err = fmt.Sscanf(options[0], "%d", &segment.ByteRange); err == nil && len(options) > 1 {
					_, err = fmt.Sscanf(options[1], "%d", &segment.Offset)
				}
			case "EXT-X-DISCONTINUITY": // 4.3.2.3
				segment.Discontinuity = true
			//case "EXT-X-KEY": // 4.3.2.4
			case "EXT-X-MAP": // 4.3.2.5
				init := new(Map)
				attributes := parseAttributes(results[2])

				if uri, exists := attributes["URI"]; exists == false {
					_, err = fmt.Sscanf(uri, "%q", &init.URI)
				} else {
					err = fmt.Errorf("URI is REQUIRED")
					return
				}

				if byteRange, exists := attributes["BYTERANGE"]; exists == true {
					_, err = fmt.Sscanf(byteRange, "%q", &init.ByteRange)
				}
				segment.Map = init
			case "EXT-X-PROGRAM-DATE-TIME": // 4.3.2.6
				segment.DateTime = results[2]
			}
		}

		if err != nil {
			err = fmt.Errorf("parsing segment attribute %q: %w", results[1], err)
			return
		}
	}
	return
}

func parseMediaPlaylist(lines []string) (playlist *MediaPlaylist, err error) {
	playlist = new(MediaPlaylist)
	var (
		hasDuration bool
		hasEndlist  bool
		lastSegment int
		keyIndex    = -1 // EXT-X-KEY will always appear before the URL, so we start at -1 becuase keyIndex will increment before parseMediaSegment is called
	)

	for i, line := range lines {
		if hasEndlist {
			break
		}

		results := regexp.MustCompile(tagRegex).FindStringSubmatch(line)
		if results == nil { // it is a URL
			segment, segErr := parseMediaSegment(lines, lastSegment, i, keyIndex)
			if segErr != nil {
				err = fmt.Errorf("making new segment: %w", segErr)
				return
			}
			lastSegment = i
			playlist.Segments = append(playlist.Segments, &segment)
		} else {
			switch results[1] {
			case "EXT-X-MEDIA", "EXT-X-STREAM-INF", "EXT-X-I-FRAME-STREAM-INF", "EXT-X-SESSION-DATA", "EXT-X-SESSION-KEY":
				err = fmt.Errorf("found master playlists tags in media playlist")
			case "EXT-X-TARGETDURATION": // 4.3.3.1
				hasDuration = true
				_, err = fmt.Sscanf(results[2], "%d", &playlist.TargetDuration)
			case "EXT-X-KEY":
				// TODO: fix a lot of 4.3.2.4.  EXT-X-KEY weird stuff
				key, err := parseKey(results[2])
				if err != nil {
					return nil, fmt.Errorf("parsing media playlist key: %w", err)
				}
				playlist.Keys = append(playlist.Keys, key)
				keyIndex++
			case "EXT-X-MEDIA-SEQUENCE": // 4.3.3.2
				_, err = fmt.Sscanf(results[2], "%d", &playlist.MediaSequence)
			case "EXT-X-DISCONTINUITY-SEQUENCE": // 4.3.3.3
				_, err = fmt.Sscanf(results[2], "%d", &playlist.DiscontinuitySeq)
			case "EXT-X-ENDLIST": // 4.3.3.4
				hasEndlist = true
			case "EXT-X-PLAYLIST-TYPE": // 4.3.3.5
				if results[2] != PlaylistEvent && results[2] != PlaylistVOD {
					err = fmt.Errorf("invalid playlist type enum: %s", results[2])
				}
				playlist.PType = results[2]
			case "EXT-X-I-FRAMES-ONLY": // 4.3.3.6
				playlist.IFramesOnly = true
			case "EXT-X-INDEPENDENT-SEGMENTS": // 4.3.5.1
				playlist.Independent = true
			case "EXT-X-START": // 4.3.5.2
				attributes := parseAttributes(results[2])

				if value := attributes["PRECISE"]; value == PreciseYes {
					playlist.Precise = true
				}

				if value, exists := attributes["TIME-OFFSET"]; exists {
					_, err = fmt.Sscanf(value, "%f", &playlist.TimeOffset)
				}
			case "EXT-X-VERSION": // 4.3.1.2
				if playlist.Version != 0 { // It has been already set, but there cannot be more than one EXT-X-VERSION tag per playlist
					err = fmt.Errorf("media playlist contains more than one %s tag", results[1])
					return
				}

				if _, err = fmt.Sscanf(results[2], "%d", &playlist.Version); err != nil {
					err = fmt.Errorf("parsing %s to integer: %w", results[1], err)
				}
			}

			if err != nil {
				err = fmt.Errorf("error parsing Variant attribute %q: %w", results[1], err)
				return
			}
		}
	}

	if hasDuration == false {
		err = fmt.Errorf("EXT-X-TARGETDURATION is a required field, but is missing")
		return
	}
	return
}
