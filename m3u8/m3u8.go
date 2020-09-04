package m3u8

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"regexp"
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

const (
	tagRegex string = `#([A-Z-]+):?(.+)?`

	// Defined in Section 4.2
	// end me please
	attrRegex string = `([A-Z0-9-]+)=(0[xX][0-9A-F]+|[0-9\.-]+|[A-Z0-9-]+|"?[^\x0A\x0D\x22]+"?)`

	// Longer version for comparison ...
	// ([A-Z0-9-]+)=(0[xX][0-9A-F]+|[0-9\.-]+|[0-9\.]+|[0-9]+|[A-Z0-9-]+|"?[^\x0A\x0D\x22\x2C]+"?)

	// Scuffed regex for properly identifying INSTREAM-ID
	// See regex test cases above for what worked and didn't
	insteamRegex string = `"(CC[1-4]|SERVICE[1-5][0-9]?|SERVICE6[0-3])"`
)

// Count returns the total number
// of segments in a MasterPlaylist
func (ma *MasterPlaylist) Count() int {
	return len(ma.Variants)
}

// Type returns master playlist type
func (ma *MasterPlaylist) Type() int {
	return TypeMaster
}

// Count returns the total number
// of segments in a MediaPlaylist
func (me *MediaPlaylist) Count() int {
	return len(me.Segments)
}

// Type returns media playlist type
func (me *MediaPlaylist) Type() int {
	return TypeMedia
}

// Defined in Section 4.2
func parseAttributes(line string) map[string]string {
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
		keyIndex    = -1
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
			playlist.Segments = append(playlist.Segments, segment)
		} else {
			switch results[1] {
			case "EXT-X-MEDIA", "EXT-X-STREAM-INF", "EXT-X-I-FRAME-STREAM-INF", "EXT-X-SESSION-DATA", "EXT-X-SESSION-KEY":
				err = fmt.Errorf("found master playlists tags in media playlist")
			case "EXT-X-TARGETDURATION": // 4.3.3.1
				hasDuration = true
				_, err = fmt.Sscanf(results[2], "%d", &playlist.TargetDuration)
			case "EXT-X-KEY":
				// TODO: fix a lot of 4.3.2.4.  EXT-X-KEY weird stuff
				key := new(Key)
				key, err = parseKey(results[2])
				playlist.Keys = append(playlist.Keys, *key)
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

func parseMasterPlaylist(lines []string) (playlist *MasterPlaylist, err error) {
	playlist = new(MasterPlaylist)
	for i, line := range lines {
		if results := regexp.MustCompile(tagRegex).FindStringSubmatch(line); results != nil {
			switch results[1] {
			case "EXT-X-TARGETDURATION", "EXT-X-MEDIA-SEQUENCE", "EXT-X-DISCONTINUITY-SEQUENCE", "EXT-X-ENDLIST", "EXT-X-PLAYLIST-TYPE", "EXT-X-I-FRAMES-ONLY":
				err = fmt.Errorf("found media playlists tags in master playlist")
				return
			case "EXT-X-MEDIA":
				rend := new(Rendition)
				attributes := parseAttributes(results[2])

				val, exists := attributes["TYPE"]
				if exists == false {
					err = fmt.Errorf("Media tag MUST include type information")
					return
				}

				if val != MediaAudio && val != MediaVideo && val != MediaSubtitles && val != MediaCaptions {
					err = fmt.Errorf("invalid media type %q", val)
					return
				}

				if _, exists = attributes["GROUP-ID"]; exists == false {
					err = fmt.Errorf("Media tag MUST include group id")
					return
				}

				if _, exists = attributes["NAME"]; exists == false {
					err = fmt.Errorf("Media tag MUST include name")
					return
				}

				if _, exist := attributes["INSTREAM-ID"]; exist == false && val == MediaCaptions {
					err = fmt.Errorf("Media tag MUST contain instream id if media type is closed captions")
					return
				} else if exist == false && val == MediaCaptions {
					err = fmt.Errorf("Media tag MUST NOT contain instead id if media type is not closed captions")
					return
				}

				// Set default values for assumption
				rend.Default = MediaDefaultNO
				rend.AutoSelect = MediaDefaultNO
				rend.Forced = MediaDefaultNO

				for attrib, value := range attributes {
					switch attrib {
					case "TYPE":
						rend.Type = value
					case "URI":
						if val == MediaCaptions {
							err = fmt.Errorf("URI cannot exist with type defined as %q", MediaCaptions)
						} else {
							_, err = fmt.Sscanf(value, "%q", &rend.URI)
						}
					case "GROUP-ID":
						_, err = fmt.Sscanf(value, "%q", &rend.GroupID)
					case "LANGUAGE":
						_, err = fmt.Sscanf(value, "%q", &rend.Language)
					case "ASSOC-LANGUAGE":
						_, err = fmt.Sscanf(value, "%q", &rend.AssocLanguage)
					case "NAME":
						_, err = fmt.Sscanf(value, "%q", &rend.Name)
					case "DEFAULT":
						if value != MediaDefaultNO && value != MediaDefaultYES {
							err = fmt.Errorf("invalid media default value %q", value)
						}
						rend.Default = value
					case "AUTOSELECT":
						if value != MediaDefaultNO && value != MediaDefaultYES {
							err = fmt.Errorf("invalid media autoselect value %q", value)
						}
						rend.AutoSelect = value
					case "FORCED":
						if value != MediaDefaultNO && value != MediaDefaultYES {
							err = fmt.Errorf("invalid media forced value %q", value)
						}
						rend.Forced = value
					case "INSTREAM-ID":
						match := regexp.MustCompile(insteamRegex).FindStringSubmatch(value)
						if match == nil {
							err = fmt.Errorf("invalid instream id value %q", value)
							return
						}
						rend.InstreamID = &match[1]
					case "CHARACTERISTICS":
						// TODO: Properly parse this (it's comma-separated)
						_, err = fmt.Sscanf(value, "%q", &rend.Characteristics)
					case "CHANNELS":
						// TODO: Properly parse this (it's backslash-separated)
						_, err = fmt.Sscanf(value, "%q", &rend.Channels)
					}

					if err != nil {
						err = fmt.Errorf("error parsing Rendition attribute %s: %w", attrib, err)
						return
					}
				}
				playlist.Renditions = append(playlist.Renditions, *rend)
			case "EXT-X-STREAM-INF": // 4.3.4.2
				variant := new(Variant)
				attributes := parseAttributes(results[2])

				// Bandwidth is a required argument
				if _, exists := attributes["BANDWIDTH"]; exists == false {
					err = fmt.Errorf("Variant stream MUST include bandwidth information")
					return
				}

				for attrib, value := range attributes {
					switch attrib {
					case "PROGRAM-ID":
						_, err = fmt.Sscanf(value, "%d", &variant.ProgramID)
					case "BANDWIDTH":
						_, err = fmt.Sscanf(value, "%d", &variant.Bandwidth)
					case "AVERAGE-BANDWIDTH":
						_, err = fmt.Sscanf(value, "%d", &variant.BandwidthAvg)
					case "CODECS":
						_, err = fmt.Sscanf(value, "%q", &variant.Codecs)
					case "RESOLUTION":
						_, err = fmt.Sscanf(value, "%dx%d", &variant.Resolution.Width, &variant.Resolution.Height)
					case "FRAME-RATE":
						_, err = fmt.Sscanf(value, "%f", &variant.FrameRate)
					case "HDCP-LEVEL":
						variant.HDCPLevel = value
						if variant.HDCPLevel != HDCPLevel0 && variant.HDCPLevel != HDCPLevelNone {
							err = fmt.Errorf("invalid enum for %s: %q", attrib, value)
							return
						}
					case "AUDIO":
						_, err = fmt.Sscanf(value, "%q", &variant.Audio)
					case "VIDEO":
						_, err = fmt.Sscanf(value, "%q", &variant.Video)
					case "SUBTITLES":
						_, err = fmt.Sscanf(value, "%q", &variant.Subtitles)
					case "CLOSED-CAPTIONS":
						if value == CCNone {
							variant.ClosedCaptions = CCNone
						} else {
							_, err = fmt.Sscanf(value, "%q", &variant.ClosedCaptions)
						}
					}

					if err != nil {
						err = fmt.Errorf("error parsing Variant attribute %s: %w", attrib, err)
						return
					}
				}
				variant.URI = lines[i+1]
				playlist.Variants = append(playlist.Variants, *variant)
			case "EXT-X-I-FRAME-STREAM-INF": // 4.3.4.3
				variant := new(IVariant)
				attributes := parseAttributes(results[2])

				for attrib, value := range attributes {
					switch attrib {
					case "BANDWIDTH":
						_, err = fmt.Sscanf(value, "%d", &variant.Bandwidth)
					case "AVERAGE-BANDWIDTH":
						_, err = fmt.Sscanf(value, "%d", &variant.BandwidthAvg)
					case "CODECS":
						_, err = fmt.Sscanf(value, "%q", &variant.Codecs)
					case "RESOLUTION":
						_, err = fmt.Sscanf(value, "%dx%d", &variant.Resolution.Width, &variant.Resolution.Height)
					case "HDCP-LEVEL":
						switch value {
						case HDCPLevel0:
						case HDCPLevelNone:
						default:
							err = fmt.Errorf("invalid enum for %s: %q", attrib, value)
						}

						variant.HDCPLevel = value
						if variant.HDCPLevel != HDCPLevel0 && variant.HDCPLevel != HDCPLevelNone {
							err = fmt.Errorf("invalid enum for %s: %q", attrib, value)
							return
						}
					case "VIDEO":
						_, err = fmt.Sscanf(value, "%q", &variant.Video)
					case "URI":
						_, err = fmt.Sscanf(value, "%q", &variant.URI)
					}

					if variant.Bandwidth == 0 || variant.URI == "" {
						err = fmt.Errorf("IVariant stream MUST include uri and bandwidth information")
						return
					}

					if err != nil {
						err = fmt.Errorf("error parsing IVariant attribute %s: %w", attrib, err)
						return
					}
				}
				playlist.IVariants = append(playlist.IVariants, *variant)
			case "EXT-X-SESSION-DATA": // 4.3.4.4
				session := new(SessionData)
				attributes := parseAttributes(results[2])
				if _, exists := attributes["DATA-ID"]; exists == false {
					err = fmt.Errorf("session data MUST include a data id")
					return
				}

				for attrib, value := range attributes {
					switch attrib {
					case "DATA-ID":
						_, err = fmt.Sscanf(value, "%q", &session.DataID)
					case "VALUE":
						_, err = fmt.Sscanf(value, "%q", &session.Value)
					case "URI":
						_, err = fmt.Sscanf(value, "%q", &session.URI)
					case "LANGUAGE":
						_, err = fmt.Sscanf(value, "%q", &session.Language)
					}

					if err != nil {
						err = fmt.Errorf("error parsing session data attribute %s: %w", attrib, err)
						return
					}
				}

				if session.URI != "" || session.Value != "" {
					if session.URI != "" && session.Value != "" {
						err = fmt.Errorf("URI and VALUE attributes are mutually exclusive, cannot contain both")
						return
					}
				}
				playlist.SessionData = append(playlist.SessionData, *session)
			case "EXT-X-SESSION-KEY": // 4.3.4.5
				playlist.SessionKey, err = parseKey(results[2])
			case "EXT-X-INDEPENDENT-SEGMENTS": // 4.3.5.1
				playlist.Independent = true
			case "EXT-X-START": // 4.3.5.2
				attributes := parseAttributes(results[2])
				if value, _ := attributes["PRECISE"]; value == PreciseYes {
					playlist.Precise = true
				}

				if value, exists := attributes["TIME-OFFSET"]; exists {
					_, err = fmt.Sscanf(value, "%f", &playlist.TimeOffset)
				}
			case "EXT-X-VERSION": // 4.3.1.2
				if playlist.Version != 0 { // It has been already set, but there cannot be more than one EXT-X-VERSION tag per playlist
					err = fmt.Errorf("master playlist contains more than one %s", results[1])
					return
				}

				if _, err = fmt.Sscanf(results[2], "%d", &playlist.Version); err != nil {
					err = fmt.Errorf("parsing %s to integer: %w", results[1], err)
					return
				}
			}
		}
	}
	return
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
