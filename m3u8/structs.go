package m3u8

// TODO: remove the stupid pointer stuff i was trying, there's really no point in it

// Playlist represents an interface that
// MasterPlaylist and MediaPlaylist fall under
type Playlist interface {
	Type() int
	//Decode(string) error
}

// Map represents
type Map struct { // 4.3.2.5
	URI       string
	ByteRange *string
}

// Segment represents an individual
// media segment from a MediaPlaylist
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
	Segments         []Segment
	Keys             []Key
	TargetDuration   int64
	MediaSequence    int64
	DiscontinuitySeq int64 // defaults to 0
	PType            string
	IFramesOnly      bool
	Independent      bool
	TimeOffset       float32
	Precise          bool
	Version          int
	SegmentCount     int
}

// Key contains information for
// decrypting encrypted segments
type Key struct { // 4.3.2.4
	Method      string
	URI         string
	IV          *string
	KeyFormat   *string
	KeyVersions *string
}

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
	Resolution   *Resolution
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
	Language *string // Should be RFC5646-compliant
}

// Rendition contains alternative renditions
// of the same content in the Master Playlist
type Rendition struct { // 4.3.4.1
	Type            string
	URI             *string
	GroupID         string
	Language        *string
	AssocLanguage   *string
	Name            string
	Default         string // defaults to no
	AutoSelect      string // defaults to no
	Forced          string // defaults to no
	InstreamID      *string
	Characteristics *string
	Channels        *string
}

// MasterPlaylist represents a Master Playlist M3U8 file
type MasterPlaylist struct { // 4.3.4
	Variants     []Variant
	IVariants    []IVariant
	SessionData  []SessionData //A Playlist MAY contain multiple EXT-X-SESSION-DATA tags with the same DATA-ID attribute
	SessionKey   *Key
	Renditions   []Rendition
	Independent  bool
	TimeOffset   *float32
	Precise      bool
	Version      int
	VariantCount int
}
