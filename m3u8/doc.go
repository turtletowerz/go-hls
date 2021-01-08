package m3u8

/*
All Section definitions and references are from RFC 8216 Protocol Version 7

An AttributeValue is one of the following:

o  decimal-integer: an unquoted string of characters from the set
  [0..9] expressing an integer in base-10 arithmetic in the range
  from 0 to 2^64-1 (18446744073709551615).  A decimal-integer may be
  from 1 to 20 characters long.

o  hexadecimal-sequence: an unquoted string of characters from the
  set [0..9] and [A..F] that is prefixed with 0x or 0X.  The maximum
  length of a hexadecimal-sequence depends on its AttributeNames.

o  decimal-floating-point: an unquoted string of characters from the
  set [0..9] and '.' that expresses a non-negative floating-point
  number in decimal positional notation.

o  signed-decimal-floating-point: an unquoted string of characters
  from the set [0..9], '-', and '.' that expresses a signed
  floating-point number in decimal positional notation.

o  quoted-string: a string of characters within a pair of double
  quotes (0x22).  The following characters MUST NOT appear in a
  quoted-string: line feed (0xA), carriage return (0xD), or double
  quote (0x22).  Quoted-string AttributeValues SHOULD be constructed
  so that byte-wise comparison is sufficient to test two quoted-
  string AttributeValues for equality.  Note that this implies case-
  sensitive comparison.

o  enumerated-string: an unquoted character string from a set that is
  explicitly defined by the AttributeName.  An enumerated-string
  will never contain double quotes ("), commas (,), or whitespace.

o  decimal-resolution: two decimal-integers separated by the "x"
  character.  The first integer is a horizontal pixel dimension


-----     #([A-Z0-9-]+)(:.+)?
#EXTM3U // Matches this but it doesn't really matter beacuse it's discarded before parsing
#EXT-X-VERSION
#EXTINF
#EXT-X-BYTERANGE
#EXT-X-DISCONTINUITY
#EXT-X-KEY
#EXT-X-MAP
#EXT-X-PROGRAM-DATE-TIME
#EXT-X-DATERANGE
#EXT-X-TARGETDURATION
#EXT-X-MEDIA-SEQUENCE
#EXT-X-DISCONTINUITY-SEQUENCE
#EXT-X-ENDLIST
#EXT-X-PLAYLIST-TYPE
#EXT-X-I-FRAMES-ONLY
#EXT-X-MEDIA
#EXT-X-STREAM-INF
#EXT-X-I-FRAME-STREAM-INF
#EXT-X-SESSION-DATA
#EXT-X-SESSION-KEY
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-START
#EXT-X-VERSION:3
#EXT-X-MEDIA-SEQUENCE:7794
#EXT-X-TARGETDURATION:15
#EXT-X-KEY:METHOD=AES-128,URI="https://priv.example.com/key.php?r=52"
#EXTINF:15.0,
#EXT-X-STREAM-INF:BANDWIDTH=65000,CODECS="mp4a.40.5",AUDIO="aac"
#EXT-X-STREAM-INF:BANDWIDTH=2560000,CODECS="...",VIDEO="mid"


-----     ([A-Z0-9-]+)=(0[xX][0-9A-F]+|[0-9\.-]+|[A-Z0-9-]+|"?[^\x0A\x0D\x22]+"?)
METHOD=AES-128,URI="https://priv.example.com/key.php?r=52"
PROGRAM-ID=1,BANDWIDTH=65000,CODECS="mp4a.40.5,avc1.42801e"
URI="https://priv.example.com/key.php?r=53"
DURATION=59.99
BANDWIDTH=1280000
URI="hi/main/audio-video.m3u8"
GROUP-ID="low"
TYPE=AUDIO
METHOD=AES-128
CODECS="mp4a.40.5"
ID="splice-6FFFFFF0"
START-DATE="2014-03-05T11:15:00Z"
SCTE35-OUT=0xFC002F0000000000FF000014056FFFFFF000E011622DCAFF00005263620000000000A0008029896F50000008700000000
CHARACTERISTICS="public.accessibility.transcribes-spoken-dialog,public.easy-to-read"


-----     "CC[1-4]"|SERVICE[1-5][0-9]?"|SERVICE6[0-3]"
"CC1"
"CC2"
"CC3"
"CC4"
"SERVICE1"
"SERVICE2.6"  [FAILED] (intended)
"SERVICE5"
"SERVICE10"
"SERVICE14"
"SERVICE20"
"SERVICE31"
"SERVICE57.5" [FAILED] (intended)
"SERVICE42"
"SERVICE442"  [FAILED] (intended)
"SERVICE45"
"SERVICE50"
"SERVICE60"
"SERVICE63"
"SERVICE72"   [FAILED] (intended)
*/
