package m3u8

import (
	"fmt"
	"regexp"
)

// Resolution contains the width and
// height of a MasterPlaylist stream
type Resolution struct { // 4.3.4.2
	Height int64
	Width  int64
}

// IVariant represents the "I-EXT-X-STREAM-INF" type
type IVariant struct { // 4.3.4.3
	URI          string
	Bandwidth    int64
	BandwidthAvg int64
	Codecs       string
	Resolution   Resolution
	Video        string
	HDCPLevel    string
}

// Variant represents the EXT-X-STREAM-INF type
type Variant struct { // 4.3.4.2
	IVariant
	ProgramID      int // Removed in Protocol 6
	FrameRate      float32
	Audio          string
	Subtitles      string
	ClosedCaptions string
}

// SessionData represents the "EXT-X-SESSION-DATA" variable
// Apparently a playlist can contain multiple EXT-X-SESSION-DATA,
// but they cannot have the same DATA-ID and LANGUAGE information.
// I'll add support for that later if it's really necessary
type SessionData struct { // 4.3.4.4
	DataID   string
	Value    string
	URI      string
	Language string // Should be RFC5646-compliant
}

// Rendition contains alternative renditions
// of the same content in the Master Playlist
type Rendition struct { // 4.3.4.1
	Type            string
	URI             string
	GroupID         string
	Language        string
	AssocLanguage   string
	Name            string
	Default         string // defaults to no
	AutoSelect      string // defaults to no
	Forced          string // defaults to no
	InstreamID      string
	Characteristics string
	Channels        string
}

// MasterPlaylist represents a Master Playlist M3U8 file
type MasterPlaylist struct { // 4.3.4
	Variants     []Variant
	IVariants    []IVariant
	SessionData  []SessionData //A Playlist MAY contain multiple EXT-X-SESSION-DATA tags with the same DATA-ID attribute
	SessionKey   *Key
	Renditions   []Rendition
	Independent  bool
	TimeOffset   float32
	Precise      bool
	Version      int
	VariantCount int
}

// Type returns master playlist type
func (m *MasterPlaylist) Type() int {
	return TypeMaster
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
						match := regexp.MustCompile(instreamRegex).FindStringSubmatch(value)
						if match == nil {
							err = fmt.Errorf("invalid instream id value %q", value)
							return
						}
						rend.InstreamID = match[1]
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

					if err != nil {
						err = fmt.Errorf("error parsing IVariant attribute %s: %w", attrib, err)
						return
					}
				}

				if variant.Bandwidth == 0 || variant.URI == "" {
					err = fmt.Errorf("IVariant stream MUST include uri and bandwidth information")
					return
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
	playlist.VariantCount = len(playlist.Variants) + len(playlist.IVariants)
	return
}
