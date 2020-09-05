package m3u8

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// got most of these tests from https://github.com/globocom/m3u8/blob/master/tests/playlists.py
// TODO: give proper credit for above comment

// Stolen from https://github.com/stretchr/testify
func objectsAreEqual(expected, actual interface{}) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}

	exp, ok := expected.([]byte)
	if !ok {
		return reflect.DeepEqual(expected, actual)
	}

	act, ok := actual.([]byte)
	if !ok {
		return false
	}

	if exp == nil || act == nil {
		return exp == nil && act == nil
	}
	return bytes.Equal(exp, act)
}

func equalValues(expected, actual interface{}) bool {
	if objectsAreEqual(expected, actual) {
		return true
	}

	actualType := reflect.TypeOf(actual)
	if actualType == nil {
		return false
	}

	expectedValue := reflect.ValueOf(expected)
	if expectedValue.IsValid() && expectedValue.Type().ConvertibleTo(actualType) {
		return reflect.DeepEqual(expectedValue.Convert(actualType).Interface(), actual)
	}
	return false
}

func assertEqual(t *testing.T, exp interface{}, act interface{}) {
	if !equalValues(exp, act) {
		t.Errorf("Not equal values\n\tExpected:%v\n\tGot:%v", exp, act)
	}
}

func makeMediaPlaylist(str string, count int, t *testing.T) *MediaPlaylist {
	playlist, err := DecodeReader(strings.NewReader(str))
	if err != nil {
		t.Fatalf("Error decoding playlist: " + err.Error())
	}
	assertEqual(t, playlist.Type(), TypeMedia)
	assertEqual(t, playlist.Count(), count)
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
	assertEqual(t, playlist.TargetDuration, 5220)
	assertEqual(t, seg.Duration, 5220)
	assertEqual(t, seg.URI, "http://media.example.com/entire.ts")
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

	assertEqual(t, playlist.TargetDuration, 5220)

	seg1 := playlist.Segments[0]
	assertEqual(t, seg1.Duration, 5220)
	assertEqual(t, seg1.URI, "http://media.example.com/entire1.ts")

	seg2 := playlist.Segments[1]
	assertEqual(t, seg2.Duration, 5218.5)
	assertEqual(t, seg2.URI, "http://media.example.com/entire2.ts")

	seg3 := playlist.Segments[2]
	assertEqual(t, seg3.Duration, float32(0.000011))
	assertEqual(t, seg3.URI, "http://media.example.com/entire3.ts")
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

	assertEqual(t, playlist.TargetDuration, 5220)
	assertEqual(t, playlist.TimeOffset, -2.0)

	seg := playlist.Segments[0]
	assertEqual(t, seg.Duration, 5220)
	assertEqual(t, seg.URI, "http://media.example.com/entire.ts")
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

	assertEqual(t, playlist.TargetDuration, 5220)
	assertEqual(t, playlist.TimeOffset, 10.5)
	assertEqual(t, playlist.Precise, true)

	seg := playlist.Segments[0]
	assertEqual(t, seg.Duration, 5220)
	assertEqual(t, seg.URI, "http://media.example.com/entire.ts")
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

	assertEqual(t, playlist.MediaSequence, 7794)
	assertEqual(t, playlist.TargetDuration, 15)
	assertEqual(t, len(playlist.Keys), 1)
	assertEqual(t, playlist.Keys[0].Method, "AES-128")
	assertEqual(t, playlist.Keys[0].URI, "https://priv.example.com/key.php?r=52")

	segments := []Segment{
		{Duration: 15, KeyIndex: 0, URI: "http://media.example.com/fileSequence52-1.ts"},
		{Duration: 15, KeyIndex: 0, URI: "http://media.example.com/fileSequence52-2.ts"},
		{Duration: 15, KeyIndex: 0, URI: "http://media.example.com/fileSequence52-3.ts"},
	}

	for i, seg := range segments {
		assertEqual(t, playlist.Segments[i], seg)
	}
}

func makeMasterPlaylist(str string, count int, t *testing.T) *MasterPlaylist {
	playlist, err := DecodeReader(strings.NewReader(str))
	if err != nil {
		t.Fatalf("Error decoding playlist: " + err.Error())
	}
	assertEqual(t, playlist.Type(), TypeMaster)
	assertEqual(t, playlist.Count(), count)
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
		assertEqual(t, playlist.Variants[i], variant)
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
		assertEqual(t, playlist.Variants[i], variant)
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
		assertEqual(t, playlist.Variants[i], variant)
	}
}
