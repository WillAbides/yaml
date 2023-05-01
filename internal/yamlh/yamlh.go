//
// Copyright (c) 2011-2019 Canonical Ltd
// Copyright (c) 2006-2010 Kirill Simonov
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
// of the Software, and to permit persons to whom the Software is furnished to do
// so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package yamlh

import (
	"fmt"
)

type VersionDirective struct {
	Major int8 // The Major version number.
	Minor int8 // The Minor version number.
}

type TagDirective struct {
	Handle []byte // The tag Handle.
	Prefix []byte // The tag Prefix.
}

type Encoding int

// The stream encoding.
const (
	// Let the parser choose the encoding.
	ANY_ENCODING Encoding = iota

	UTF8_ENCODING    // The default UTF-8 encoding.
	UTF16LE_ENCODING // The UTF-16-LE encoding with BOM.
	UTF16BE_ENCODING // The UTF-16-BE encoding with BOM.
)

type Break int

// Line break types.
const (
	// Let the parser choose the break type.
	ANY_BREAK Break = iota

	CR_BREAK   // Use CR for line breaks (Mac style).
	LN_BREAK   // Use LN for line breaks (Unix style).
	CRLN_BREAK // Use CR LN for line breaks (DOS style).
)

type ErrorType int

// Many bad things could happen with the parser and emitter.
const (
	// No error is produced.
	NO_ERROR ErrorType = iota

	READER_ERROR  // Cannot read or decode the input stream.
	SCANNER_ERROR // Cannot scan the input stream.
	PARSER_ERROR  // Cannot parsePanic the input stream.
	WRITER_ERROR  // Cannot write to the output stream.
	EMITTER_ERROR // Cannot emit a YAML stream.
)

// Position is he pointer position.
type Position struct {
	Index  int // The position Index.
	Line   int // The position Line.
	Column int // The position Column.
}

type YamlStyle int8

type YamlScalarStyle YamlStyle

// Scalar styles.
const (
	// Let the emitter choose the style.
	ANY_SCALAR_STYLE YamlScalarStyle = 0

	PLAIN_SCALAR_STYLE         YamlScalarStyle = 1 << iota // The plain scalar style.
	SINGLE_QUOTED_SCALAR_STYLE                             // The single-quoted scalar style.
	DOUBLE_QUOTED_SCALAR_STYLE                             // The double-quoted scalar style.
	LITERAL_SCALAR_STYLE                                   // The literal scalar style.
	FOLDED_SCALAR_STYLE                                    // The folded scalar style.
)

type YamlSequenceStyle YamlStyle

// Sequence styles.
const (
	// Let the emitter choose the style.
	ANY_SEQUENCE_STYLE YamlSequenceStyle = iota

	BLOCK_SEQUENCE_STYLE // The block sequence style.
	FLOW_SEQUENCE_STYLE  // The flow sequence style.
)

type YamlMappingStyle YamlStyle

// Mapping styles.
const (
	// Let the emitter choose the style.
	ANY_MAPPING_STYLE YamlMappingStyle = iota

	BLOCK_MAPPING_STYLE // The block mapping style.
	FLOW_MAPPING_STYLE  // The flow mapping style.
)

type TokenType int

// Token types.
const (
	// An empty token.
	NO_TOKEN TokenType = iota

	STREAM_START_TOKEN // A STREAM-START token.
	STREAM_END_TOKEN   // A STREAM-END token.

	VERSION_DIRECTIVE_TOKEN // A VERSION-DIRECTIVE token.
	TAG_DIRECTIVE_TOKEN     // A TAG-DIRECTIVE token.
	DOCUMENT_START_TOKEN    // A DOCUMENT-START token.
	DOCUMENT_END_TOKEN      // A DOCUMENT-END token.

	BLOCK_SEQUENCE_START_TOKEN // A BLOCK-SEQUENCE-START token.
	BLOCK_MAPPING_START_TOKEN  // A BLOCK-SEQUENCE-END token.
	BLOCK_END_TOKEN            // A BLOCK-END token.

	FLOW_SEQUENCE_START_TOKEN // A FLOW-SEQUENCE-START token.
	FLOW_SEQUENCE_END_TOKEN   // A FLOW-SEQUENCE-END token.
	FLOW_MAPPING_START_TOKEN  // A FLOW-MAPPING-START token.
	FLOW_MAPPING_END_TOKEN    // A FLOW-MAPPING-END token.

	BLOCK_ENTRY_TOKEN // A BLOCK-ENTRY token.
	FLOW_ENTRY_TOKEN  // A FLOW-ENTRY token.
	KEY_TOKEN         // A KEY token.
	VALUE_TOKEN       // A VALUE token.

	ALIAS_TOKEN  // An ALIAS token.
	ANCHOR_TOKEN // An ANCHOR token.
	TAG_TOKEN    // A TAG token.
	SCALAR_TOKEN // A SCALAR token.
)

func (tt TokenType) String() string {
	switch tt {
	case NO_TOKEN:
		return "NO_TOKEN"
	case STREAM_START_TOKEN:
		return "STREAM_START_TOKEN"
	case STREAM_END_TOKEN:
		return "STREAM_END_TOKEN"
	case VERSION_DIRECTIVE_TOKEN:
		return "VERSION_DIRECTIVE_TOKEN"
	case TAG_DIRECTIVE_TOKEN:
		return "TAG_DIRECTIVE_TOKEN"
	case DOCUMENT_START_TOKEN:
		return "DOCUMENT_START_TOKEN"
	case DOCUMENT_END_TOKEN:
		return "DOCUMENT_END_TOKEN"
	case BLOCK_SEQUENCE_START_TOKEN:
		return "BLOCK_SEQUENCE_START_TOKEN"
	case BLOCK_MAPPING_START_TOKEN:
		return "BLOCK_MAPPING_START_TOKEN"
	case BLOCK_END_TOKEN:
		return "BLOCK_END_TOKEN"
	case FLOW_SEQUENCE_START_TOKEN:
		return "FLOW_SEQUENCE_START_TOKEN"
	case FLOW_SEQUENCE_END_TOKEN:
		return "FLOW_SEQUENCE_END_TOKEN"
	case FLOW_MAPPING_START_TOKEN:
		return "FLOW_MAPPING_START_TOKEN"
	case FLOW_MAPPING_END_TOKEN:
		return "FLOW_MAPPING_END_TOKEN"
	case BLOCK_ENTRY_TOKEN:
		return "BLOCK_ENTRY_TOKEN"
	case FLOW_ENTRY_TOKEN:
		return "FLOW_ENTRY_TOKEN"
	case KEY_TOKEN:
		return "KEY_TOKEN"
	case VALUE_TOKEN:
		return "VALUE_TOKEN"
	case ALIAS_TOKEN:
		return "ALIAS_TOKEN"
	case ANCHOR_TOKEN:
		return "ANCHOR_TOKEN"
	case TAG_TOKEN:
		return "TAG_TOKEN"
	case SCALAR_TOKEN:
		return "SCALAR_TOKEN"
	}
	return "<unknown token>"
}

type YamlToken struct {
	// The token type.
	Type TokenType

	// The start/end of the token.
	Start_mark, End_mark Position

	// The stream Encoding (for STREAM_START_TOKEN).
	Encoding Encoding

	// The alias/anchor/scalar Value or tag/tag directive handle
	// (for ALIAS_TOKEN, ANCHOR_TOKEN, yaml_SCALAR_TOKEN, yaml_TAG_TOKEN, yaml_TAG_DIRECTIVE_TOKEN).
	Value []byte

	// The tag Suffix (for TAG_TOKEN).
	Suffix []byte

	// The tag directive Prefix (for TAG_DIRECTIVE_TOKEN).
	Prefix []byte

	// The scalar Style (for SCALAR_TOKEN).
	Style YamlScalarStyle

	// The version directive Major/minor (for VERSION_DIRECTIVE_TOKEN).
	Major, Minor int8
}

type EventType int8

// Event types.
const (
	NO_EVENT EventType = iota

	STREAM_START_EVENT   // A STREAM-START event.
	STREAM_END_EVENT     // A STREAM-END event.
	DOCUMENT_START_EVENT // A DOCUMENT-START event.
	DOCUMENT_END_EVENT   // A DOCUMENT-END event.
	ALIAS_EVENT          // An ALIAS event.
	SCALAR_EVENT         // A SCALAR event.
	SEQUENCE_START_EVENT // A SEQUENCE-START event.
	SEQUENCE_END_EVENT   // A SEQUENCE-END event.
	MAPPING_START_EVENT  // A MAPPING-START event.
	MAPPING_END_EVENT    // A MAPPING-END event.
	TAIL_COMMENT_EVENT
)

var eventStrings = []string{
	NO_EVENT:             "none",
	STREAM_START_EVENT:   "stream start",
	STREAM_END_EVENT:     "stream end",
	DOCUMENT_START_EVENT: "document start",
	DOCUMENT_END_EVENT:   "document end",
	ALIAS_EVENT:          "alias",
	SCALAR_EVENT:         "scalar",
	SEQUENCE_START_EVENT: "sequence start",
	SEQUENCE_END_EVENT:   "sequence end",
	MAPPING_START_EVENT:  "mapping start",
	MAPPING_END_EVENT:    "mapping end",
	TAIL_COMMENT_EVENT:   "tail comment",
}

func (e EventType) String() string {
	if e < 0 || int(e) >= len(eventStrings) {
		return fmt.Sprintf("unknown event %d", e)
	}
	return eventStrings[e]
}

// The Event structure.
type Event struct {
	// The event type.
	Type EventType

	// The start and end of the event.
	Start_mark, End_mark Position

	// The document Encoding (for STREAM_START_EVENT).
	Encoding Encoding

	// The version directive (for DOCUMENT_START_EVENT).
	Version_directive *VersionDirective

	// The list of tag directives (for DOCUMENT_START_EVENT).
	Tag_directives []TagDirective

	// The comments
	Head_comment []byte
	Line_comment []byte
	Foot_comment []byte
	Tail_comment []byte

	// The Anchor (for SCALAR_EVENT, SEQUENCE_START_EVENT, MAPPING_START_EVENT, ALIAS_EVENT).
	Anchor []byte

	// The Tag (for SCALAR_EVENT, SEQUENCE_START_EVENT, MAPPING_START_EVENT).
	Tag []byte

	// The scalar Value (for SCALAR_EVENT).
	Value []byte

	// Is the document start/end indicator Implicit, or the Tag optional?
	// (for DOCUMENT_START_EVENT, DOCUMENT_END_EVENT, SEQUENCE_START_EVENT, MAPPING_START_EVENT, SCALAR_EVENT).
	Implicit bool

	// Is the Tag optional for any non-plain style? (for SCALAR_EVENT).
	Quoted_implicit bool

	// The Style (for SCALAR_EVENT, SEQUENCE_START_EVENT, MAPPING_START_EVENT).
	Style YamlStyle
}

func (e *Event) Scalar_style() YamlScalarStyle     { return YamlScalarStyle(e.Style) }
func (e *Event) Sequence_style() YamlSequenceStyle { return YamlSequenceStyle(e.Style) }
func (e *Event) Mapping_style() YamlMappingStyle   { return YamlMappingStyle(e.Style) }

const (
	NULL_TAG      = "tag:yaml.org,2002:null"      // The tag !!null with the only possible value: null.
	BOOL_TAG      = "tag:yaml.org,2002:bool"      // The tag !!bool with the values: true and false.
	STR_TAG       = "tag:yaml.org,2002:str"       // The tag !!str for string values.
	INT_TAG       = "tag:yaml.org,2002:int"       // The tag !!int for integer values.
	FLOAT_TAG     = "tag:yaml.org,2002:float"     // The tag !!float for float values.
	TIMESTAMP_TAG = "tag:yaml.org,2002:timestamp" // The tag !!timestamp for date and time values.

	SEQ_TAG = "tag:yaml.org,2002:seq" // The tag !!seq is used to denote sequences.
	MAP_TAG = "tag:yaml.org,2002:map" // The tag !!map is used to denote mapping.

	// Not in original libyaml.
	BINARY_TAG = "tag:yaml.org,2002:binary"
	MERGE_TAG  = "tag:yaml.org,2002:merge"

	DEFAULT_SCALAR_TAG   = STR_TAG // The default scalar tag is !!str.
	DEFAULT_SEQUENCE_TAG = SEQ_TAG // The default sequence tag is !!seq.
	DEFAULT_MAPPING_TAG  = MAP_TAG // The default mapping tag is !!map.
)

// SimpleKey holds information about a potential simple key.
type SimpleKey struct {
	Possible     bool     // Is a simple key Possible?
	Required     bool     // Is a simple key Required?
	Token_number int      // The number of the token.
	Mark         Position // The position Mark.
}

type YamlComment struct {
	Scan_mark  Position // Position where scanning for comments started
	Token_mark Position // Position after which tokens will be associated with this comment
	Start_mark Position // Position of '#' comment mark
	End_mark   Position // Position where comment terminated

	Head []byte
	Line []byte
	Foot []byte
}
