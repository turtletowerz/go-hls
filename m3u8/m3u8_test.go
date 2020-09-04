package m3u8

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: fork testify project and make an assert-only package to use for testing
// got most of these tests from https://github.com/globocom/m3u8/blob/master/tests/playlists.py
// TODO: give proper credit for above comment

func makeMediaPlaylist(str string, count int, t *testing.T) *MediaPlaylist {
	playlist, err := DecodeReader(strings.NewReader(str))
	if err != nil {
		t.Fatalf("Error decoding playlist: " + err.Error())
	}
	assert.Equal(t, playlist.Type(), TypeMedia)
	assert.Equal(t, playlist.Count(), count)
	return playlist.(*MediaPlaylist)
}

func TestSimpleMediaPlaylist(t *testing.T) {
	playlist := makeMediaPlaylist(`
		#EXTM3U
		#EXT-X-TARGETDURATION:5220
		#EXTINF:5220,
		http://media.example.com/entire.ts
		#EXT-X-ENDLIST
	`, 1, t)

	seg := playlist.Segments[0]
	assert.EqualValues(t, playlist.TargetDuration, 5220)
	assert.EqualValues(t, seg.Duration, 5220)
	assert.Equal(t, seg.URI, "http://media.example.com/entire.ts")
}

func TestMediaPlaylistShortDuration(t *testing.T) {
	playlist := makeMediaPlaylist(`
		#EXTM3U
		#EXT-X-TARGETDURATION:5220
		#EXTINF:5220,
		http://media.example.com/entire1.ts
		#EXTINF:5218.5,
		http://media.example.com/entire2.ts
		#EXTINF:0.000011,
		http://media.example.com/entire3.ts
		#EXT-X-ENDLIST	
	`, 3, t)

	assert.EqualValues(t, playlist.TargetDuration, 5220)

	seg1 := playlist.Segments[0]
	assert.EqualValues(t, seg1.Duration, 5220)
	assert.Equal(t, seg1.URI, "http://media.example.com/entire1.ts")

	seg2 := playlist.Segments[1]
	assert.EqualValues(t, seg2.Duration, 5218.5)
	assert.Equal(t, seg2.URI, "http://media.example.com/entire2.ts")

	seg3 := playlist.Segments[2]
	assert.EqualValues(t, seg3.Duration, float32(0.000011))
	assert.Equal(t, seg3.URI, "http://media.example.com/entire3.ts")
}

func TestMediaPlaylistNegativeOffset(t *testing.T) {
	playlist := makeMediaPlaylist(`
		#EXTM3U
		#EXT-X-TARGETDURATION:5220
		#EXT-X-START:TIME-OFFSET=-2.0
		#EXTINF:5220,
		http://media.example.com/entire.ts
		#EXT-X-ENDLIST
	`, 1, t)

	assert.EqualValues(t, playlist.TargetDuration, 5220)
	assert.EqualValues(t, playlist.TimeOffset, -2.0)

	seg := playlist.Segments[0]
	assert.EqualValues(t, seg.Duration, 5220)
	assert.Equal(t, seg.URI, "http://media.example.com/entire.ts")
}

func TestMediaPlaylistStartPrecise(t *testing.T) {
	playlist := makeMediaPlaylist(`
		#EXTM3U
		#EXT-X-TARGETDURATION:5220
		#EXT-X-START:TIME-OFFSET=10.5,PRECISE=YES
		#EXTINF:5220,
		http://media.example.com/entire.ts
		#EXT-X-ENDLIST
	`, 1, t)

	assert.EqualValues(t, playlist.TargetDuration, 5220)
	assert.EqualValues(t, playlist.TimeOffset, 10.5)
	assert.Equal(t, playlist.Precise, true)

	seg := playlist.Segments[0]
	assert.EqualValues(t, seg.Duration, 5220)
	assert.Equal(t, seg.URI, "http://media.example.com/entire.ts")
}

func TestMediaPlaylistEncryptedSegments(t *testing.T) {
	playlist := makeMediaPlaylist(`
		#EXTM3U
		#EXT-X-MEDIA-SEQUENCE:7794
		#EXT-X-TARGETDURATION:15
		#EXT-X-KEY:METHOD=AES-128,URI="https://priv.example.com/key.php?r=52"
		#EXTINF:15,
		http://media.example.com/fileSequence52-1.ts
		#EXTINF:15,
		http://media.example.com/fileSequence52-2.ts
		#EXTINF:15,
		http://media.example.com/fileSequence52-3.ts
	`, 3, t)

	assert := assert.New(t)
	assert.EqualValues(playlist.MediaSequence, 7794)
	assert.EqualValues(playlist.TargetDuration, 15)
	assert.Equal(len(playlist.Keys), 1)
	assert.Equal(playlist.Keys[0].Method, "AES-128")
	assert.Equal(playlist.Keys[0].URI, "https://priv.example.com/key.php?r=52")

	segments := []Segment{
		{Duration: 15, KeyIndex: 0, URI: "http://media.example.com/fileSequence52-1.ts"},
		{Duration: 15, KeyIndex: 0, URI: "http://media.example.com/fileSequence52-2.ts"},
		{Duration: 15, KeyIndex: 0, URI: "http://media.example.com/fileSequence52-3.ts"},
	}

	for i, seg := range segments {
		assert.EqualValues(playlist.Segments[i], seg)
	}
}

func makeMasterPlaylist(str string, count int, t *testing.T) *MasterPlaylist {
	playlist, err := DecodeReader(strings.NewReader(str))
	if err != nil {
		t.Fatalf("Error decoding playlist: " + err.Error())
	}
	assert.Equal(t, playlist.Type(), TypeMaster)
	assert.Equal(t, playlist.Count(), count)
	return playlist.(*MasterPlaylist)
}

func TestMasterPlaylistSimple(t *testing.T) {
	playlist := makeMasterPlaylist(`
		#EXTM3U
		#EXT-X-STREAM-INF:PROGRAM-ID=1, BANDWIDTH=1280000
		http://example.com/low.m3u8
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=2560000
		http://example.com/mid.m3u8
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=7680000
		http://example.com/hi.m3u8
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=65000,CODECS="mp4a.40.5,avc1.42801e"
		http://example.com/audio-only.m3u8
	`, 4, t)

	variants := []Variant{
		{IVariant: IVariant{URI: "http://example.com/low.m3u8", Bandwidth: 1280000}, ProgramID: 1},
		{IVariant: IVariant{URI: "http://example.com/mid.m3u8", Bandwidth: 2560000}, ProgramID: 1},
		{IVariant: IVariant{URI: "http://example.com/hi.m3u8", Bandwidth: 7680000}, ProgramID: 1},
		{IVariant: IVariant{URI: "http://example.com/audio-only.m3u8", Bandwidth: 65000, Codecs: "mp4a.40.5,avc1.42801e"}, ProgramID: 1},
	}

	for i, variant := range variants {
		assert.EqualValues(t, playlist.Variants[i], variant)
	}
}

func TestMasterPlaylistCCVideoAudioSubs(t *testing.T) {
	playlist := makeMasterPlaylist(`
		#EXTM3U
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=7680000,CLOSED-CAPTIONS="cc",SUBTITLES="sub",AUDIO="aud",VIDEO="vid"
		http://example.com/with-cc-hi.m3u8
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=65000,CLOSED-CAPTIONS="cc",SUBTITLES="sub",AUDIO="aud",VIDEO="vid"
		http://example.com/with-cc-low.m3u8
	`, 2, t)

	variants := []Variant{
		{IVariant{URI: "http://example.com/with-cc-hi.m3u8", Bandwidth: 7680000, Video: "vid"}, 1, 0, "aud", "sub", "cc"},
		{IVariant{URI: "http://example.com/with-cc-low.m3u8", Bandwidth: 65000, Video: "vid"}, 1, 0, "aud", "sub", "cc"},
	}

	for i, variant := range variants {
		assert.EqualValues(t, playlist.Variants[i], variant)
	}
}

func TestMasterPlaylistAvgBandwidth(t *testing.T) {
	playlist := makeMasterPlaylist(`
		#EXTM3U
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=1280000,AVERAGE-BANDWIDTH=1252345
		http://example.com/low.m3u8
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=2560000,AVERAGE-BANDWIDTH=2466570
		http://example.com/mid.m3u8
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=7680000,AVERAGE-BANDWIDTH=7560423
		http://example.com/hi.m3u8
		#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=65000,AVERAGE-BANDWIDTH=63005,CODECS="mp4a.40.5,avc1.42801e"
		http://example.com/audio-only.m3u8
	`, 4, t)

	variants := []Variant{
		{IVariant: IVariant{URI: "http://example.com/low.m3u8", Bandwidth: 1280000, BandwidthAvg: 1252345}, ProgramID: 1},
		{IVariant: IVariant{URI: "http://example.com/mid.m3u8", Bandwidth: 2560000, BandwidthAvg: 2466570}, ProgramID: 1},
		{IVariant: IVariant{URI: "http://example.com/hi.m3u8", Bandwidth: 7680000, BandwidthAvg: 7560423}, ProgramID: 1},
		{IVariant: IVariant{URI: "http://example.com/audio-only.m3u8", Bandwidth: 65000, BandwidthAvg: 63005, Codecs: "mp4a.40.5,avc1.42801e"}, ProgramID: 1},
	}

	for i, variant := range variants {
		assert.EqualValues(t, playlist.Variants[i], variant)
	}
}
