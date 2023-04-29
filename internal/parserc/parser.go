package parserc

import (
	"github.com/willabides/go-yaml/internal/yamlh"
	"io"
)

// ParserState The states of the parser.
type ParserState int

const (
	PARSE_STREAM_START_STATE ParserState = iota

	PARSE_IMPLICIT_DOCUMENT_START_STATE           // expect the beginning of an implicit document.
	PARSE_DOCUMENT_START_STATE                    // expect DOCUMENT-START.
	PARSE_DOCUMENT_CONTENT_STATE                  // expect the content of a document.
	PARSE_DOCUMENT_END_STATE                      // expect DOCUMENT-END.
	PARSE_BLOCK_NODE_STATE                        // expect a block node.
	PARSE_BLOCK_NODE_OR_INDENTLESS_SEQUENCE_STATE // expect a block node or indentless sequence.
	PARSE_FLOW_NODE_STATE                         // expect a flow node.
	PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE        // expect the first entry of a block sequence.
	PARSE_BLOCK_SEQUENCE_ENTRY_STATE              // expect an entry of a block sequence.
	PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE         // expect an entry of an indentless sequence.
	PARSE_BLOCK_MAPPING_FIRST_KEY_STATE           // expect the first key of a block mapping.
	PARSE_BLOCK_MAPPING_KEY_STATE                 // expect a block mapping key.
	PARSE_BLOCK_MAPPING_VALUE_STATE               // expect a block mapping value.
	PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE         // expect the first entry of a flow sequence.
	PARSE_FLOW_SEQUENCE_ENTRY_STATE               // expect an entry of a flow sequence.
	PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE   // expect a key of an ordered mapping.
	PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE // expect a value of an ordered mapping.
	PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE   // expect the and of an ordered mapping entry.
	PARSE_FLOW_MAPPING_FIRST_KEY_STATE            // expect the first key of a flow mapping.
	PARSE_FLOW_MAPPING_KEY_STATE                  // expect a key of a flow mapping.
	PARSE_FLOW_MAPPING_VALUE_STATE                // expect a value of a flow mapping.
	PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE          // expect an empty value of a flow mapping.
	PARSE_END_STATE                               // expect nothing.
)

func (ps ParserState) String() string {
	switch ps {
	case PARSE_STREAM_START_STATE:
		return "PARSE_STREAM_START_STATE"
	case PARSE_IMPLICIT_DOCUMENT_START_STATE:
		return "PARSE_IMPLICIT_DOCUMENT_START_STATE"
	case PARSE_DOCUMENT_START_STATE:
		return "PARSE_DOCUMENT_START_STATE"
	case PARSE_DOCUMENT_CONTENT_STATE:
		return "PARSE_DOCUMENT_CONTENT_STATE"
	case PARSE_DOCUMENT_END_STATE:
		return "PARSE_DOCUMENT_END_STATE"
	case PARSE_BLOCK_NODE_STATE:
		return "PARSE_BLOCK_NODE_STATE"
	case PARSE_BLOCK_NODE_OR_INDENTLESS_SEQUENCE_STATE:
		return "PARSE_BLOCK_NODE_OR_INDENTLESS_SEQUENCE_STATE"
	case PARSE_FLOW_NODE_STATE:
		return "PARSE_FLOW_NODE_STATE"
	case PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE:
		return "PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE"
	case PARSE_BLOCK_SEQUENCE_ENTRY_STATE:
		return "PARSE_BLOCK_SEQUENCE_ENTRY_STATE"
	case PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE:
		return "PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE"
	case PARSE_BLOCK_MAPPING_FIRST_KEY_STATE:
		return "PARSE_BLOCK_MAPPING_FIRST_KEY_STATE"
	case PARSE_BLOCK_MAPPING_KEY_STATE:
		return "PARSE_BLOCK_MAPPING_KEY_STATE"
	case PARSE_BLOCK_MAPPING_VALUE_STATE:
		return "PARSE_BLOCK_MAPPING_VALUE_STATE"
	case PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE:
		return "PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE"
	case PARSE_FLOW_SEQUENCE_ENTRY_STATE:
		return "PARSE_FLOW_SEQUENCE_ENTRY_STATE"
	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE:
		return "PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE"
	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE:
		return "PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE"
	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE:
		return "PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE"
	case PARSE_FLOW_MAPPING_FIRST_KEY_STATE:
		return "PARSE_FLOW_MAPPING_FIRST_KEY_STATE"
	case PARSE_FLOW_MAPPING_KEY_STATE:
		return "PARSE_FLOW_MAPPING_KEY_STATE"
	case PARSE_FLOW_MAPPING_VALUE_STATE:
		return "PARSE_FLOW_MAPPING_VALUE_STATE"
	case PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE:
		return "PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE"
	case PARSE_END_STATE:
		return "PARSE_END_STATE"
	}
	return "<unknown parser state>"
}

// YamlParser is the parser structure.
type YamlParser struct {
	// Reader stuff

	Reader    io.Reader // File input data.
	Input     []byte    // String Input data.
	Input_pos int

	Eof bool // EOF flag

	Buffer     []byte // The working Buffer.
	Buffer_pos int    // The current position of the Buffer.

	Unread int // The number of Unread characters in the Buffer.

	Newlines int // The number of line breaks since last non-break/non-blank character

	Raw_buffer     []byte // The raw Buffer.
	Raw_buffer_pos int    // The current position of the Buffer.

	Encoding yamlh.Encoding // The Input Encoding.

	Offset int            // The Offset of the current position (in bytes).
	Mark   yamlh.Position // The Mark of the current position.

	// Comments

	Head_comment []byte // The current head comments
	Line_comment []byte // The current line comments
	Foot_comment []byte // The current foot comments
	Tail_comment []byte // Foot comment that happens at the end of a block.
	Stem_comment []byte // Comment in item preceding a nested structure (list inside list item, etc)

	Comments      []yamlh.YamlComment // The folded Comments for all parsed tokens
	Comments_head int

	// Scanner stuff

	Stream_start_produced bool // Have we started to scan the Input stream?
	Stream_end_produced   bool // Have we reached the end of the Input stream?

	Flow_level int // The number of unclosed '[' and '{' indicators.

	Tokens          []yamlh.YamlToken // The Tokens queue.
	Tokens_head     int               // The head of the Tokens queue.
	Tokens_parsed   int               // The number of Tokens fetched from the queue.
	Token_available bool              // Does the Tokens queue contain a token ready for dequeueing.

	Indent  int   // The current indentation level.
	Indents []int // The indentation levels stack.

	Simple_key_allowed bool              // May a simple key occur at the current position?
	Simple_keys        []yamlh.SimpleKey // The stack of simple keys.
	Simple_keys_by_tok map[int]int       // possible simple_key indexes indexed by token_number

	// parser stuff

	State          ParserState          // The current parser State.
	States         []ParserState        // The parser States stack.
	Marks          []yamlh.Position     // The stack of Marks.
	Tag_directives []yamlh.TagDirective // The list of TAG directives.
}

func New(reader io.Reader) *YamlParser {
	return &YamlParser{
		Raw_buffer: make([]byte, 0, yamlh.Input_raw_buffer_size),
		Buffer:     make([]byte, 0, yamlh.Input_buffer_size),
		Reader:     reader,
	}
}
