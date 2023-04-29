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

package parserc

import (
	"bytes"
	"fmt"
	"github.com/willabides/yaml/internal/yamlh"
)

// Introduction
// ************
//
// The following notes assume that you are familiar with the YAML specification
// (http://yaml.org/spec/1.2/spec.html).  We mostly follow it, although in
// some cases we are less restrictive that it requires.
//
// The process of transforming a YAML stream into a sequence of events is
// divided on two steps: Scanning and Parsing.
//
// The Scanner transforms the input stream into a sequence of tokens, while the
// parser transform the sequence of tokens produced by the Scanner into a
// sequence of parsing events.
//
// The Scanner is rather clever and complicated. The parser, on the contrary,
// is a straightforward implementation of a recursive-descendant parser (or,
// LL(1) parser, as it is usually called).
//
// Actually there are two issues of Scanning that might be called "clever", the
// rest is quite straightforward.  The issues are "block collection start" and
// "simple keys".  Both issues are explained below in details.
//
// Here the Scanning step is explained and implemented.  We start with the list
// of all the tokens produced by the Scanner together with short descriptions.
//
// Now, tokens:
//
//      STREAM-START(encoding)          # The stream start.
//      STREAM-END                      # The stream end.
//      VERSION-DIRECTIVE(major,minor)  # The '%YAML' directive.
//      TAG-DIRECTIVE(handle,prefix)    # The '%TAG' directive.
//      DOCUMENT-START                  # '---'
//      DOCUMENT-END                    # '...'
//      BLOCK-SEQUENCE-START            # Indentation increase denoting a block
//      BLOCK-MAPPING-START             # sequence or a block mapping.
//      BLOCK-END                       # Indentation decrease.
//      FLOW-SEQUENCE-START             # '['
//      FLOW-SEQUENCE-END               # ']'
//      BLOCK-SEQUENCE-START            # '{'
//      BLOCK-SEQUENCE-END              # '}'
//      BLOCK-ENTRY                     # '-'
//      FLOW-ENTRY                      # ','
//      KEY                             # '?' or nothing (simple keys).
//      VALUE                           # ':'
//      ALIAS(anchor)                   # '*anchor'
//      ANCHOR(anchor)                  # '&anchor'
//      TAG(handle,suffix)              # '!handle!suffix'
//      SCALAR(value,style)             # A scalar.
//
// The following two tokens are "virtual" tokens denoting the beginning and the
// end of the stream:
//
//      STREAM-START(encoding)
//      STREAM-END
//
// We pass the information about the input stream encoding with the
// STREAM-START token.
//
// The next two tokens are responsible for tags:
//
//      VERSION-DIRECTIVE(major,minor)
//      TAG-DIRECTIVE(handle,prefix)
//
// Example:
//
//      %YAML   1.1
//      %TAG    !   !foo
//      %TAG    !yaml!  tag:yaml.org,2002:
//      ---
//
// The correspoding sequence of tokens:
//
//      STREAM-START(utf-8)
//      VERSION-DIRECTIVE(1,1)
//      TAG-DIRECTIVE("!","!foo")
//      TAG-DIRECTIVE("!yaml","tag:yaml.org,2002:")
//      DOCUMENT-START
//      STREAM-END
//
// Note that the VERSION-DIRECTIVE and TAG-DIRECTIVE tokens occupy a whole
// line.
//
// The document start and end indicators are represented by:
//
//      DOCUMENT-START
//      DOCUMENT-END
//
// Note that if a YAML stream contains an implicit document (without '---'
// and '...' indicators), no DOCUMENT-START and DOCUMENT-END tokens will be
// produced.
//
// In the following examples, we present whole documents together with the
// produced tokens.
//
//      1. An implicit document:
//
//          'a scalar'
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          SCALAR("a scalar",single-quoted)
//          STREAM-END
//
//      2. An explicit document:
//
//          ---
//          'a scalar'
//          ...
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          DOCUMENT-START
//          SCALAR("a scalar",single-quoted)
//          DOCUMENT-END
//          STREAM-END
//
//      3. Several documents in a stream:
//
//          'a scalar'
//          ---
//          'another scalar'
//          ---
//          'yet another scalar'
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          SCALAR("a scalar",single-quoted)
//          DOCUMENT-START
//          SCALAR("another scalar",single-quoted)
//          DOCUMENT-START
//          SCALAR("yet another scalar",single-quoted)
//          STREAM-END
//
// We have already introduced the SCALAR token above.  The following tokens are
// used to describe aliases, anchors, tag, and scalars:
//
//      ALIAS(anchor)
//      ANCHOR(anchor)
//      TAG(handle,suffix)
//      SCALAR(value,style)
//
// The following series of examples illustrate the usage of these tokens:
//
//      1. A recursive sequence:
//
//          &A [ *A ]
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          ANCHOR("A")
//          FLOW-SEQUENCE-START
//          ALIAS("A")
//          FLOW-SEQUENCE-END
//          STREAM-END
//
//      2. A tagged scalar:
//
//          !!float "3.14"  # A good approximation.
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          TAG("!!","float")
//          SCALAR("3.14",double-quoted)
//          STREAM-END
//
//      3. Various scalar styles:
//
//          --- # Implicit empty plain scalars do not produce tokens.
//          --- a plain scalar
//          --- 'a single-quoted scalar'
//          --- "a double-quoted scalar"
//          --- |-
//            a literal scalar
//          --- >-
//            a folded
//            scalar
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          DOCUMENT-START
//          DOCUMENT-START
//          SCALAR("a plain scalar",plain)
//          DOCUMENT-START
//          SCALAR("a single-quoted scalar",single-quoted)
//          DOCUMENT-START
//          SCALAR("a double-quoted scalar",double-quoted)
//          DOCUMENT-START
//          SCALAR("a literal scalar",literal)
//          DOCUMENT-START
//          SCALAR("a folded scalar",folded)
//          STREAM-END
//
// Now it's time to review collection-related tokens. We will start with
// flow collections:
//
//      FLOW-SEQUENCE-START
//      FLOW-SEQUENCE-END
//      FLOW-MAPPING-START
//      FLOW-MAPPING-END
//      FLOW-ENTRY
//      KEY
//      VALUE
//
// The tokens FLOW-SEQUENCE-START, FLOW-SEQUENCE-END, FLOW-MAPPING-START, and
// FLOW-MAPPING-END represent the indicators '[', ']', '{', and '}'
// correspondingly.  FLOW-ENTRY represent the ',' indicator.  Finally the
// indicators '?' and ':', which are used for denoting mapping keys and values,
// are represented by the KEY and VALUE tokens.
//
// The following examples show flow collections:
//
//      1. A flow sequence:
//
//          [item 1, item 2, item 3]
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          FLOW-SEQUENCE-START
//          SCALAR("item 1",plain)
//          FLOW-ENTRY
//          SCALAR("item 2",plain)
//          FLOW-ENTRY
//          SCALAR("item 3",plain)
//          FLOW-SEQUENCE-END
//          STREAM-END
//
//      2. A flow mapping:
//
//          {
//              a simple key: a value,  # Note that the KEY token is produced.
//              ? a complex key: another value,
//          }
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          FLOW-MAPPING-START
//          KEY
//          SCALAR("a simple key",plain)
//          VALUE
//          SCALAR("a value",plain)
//          FLOW-ENTRY
//          KEY
//          SCALAR("a complex key",plain)
//          VALUE
//          SCALAR("another value",plain)
//          FLOW-ENTRY
//          FLOW-MAPPING-END
//          STREAM-END
//
// A simple key is a key which is not denoted by the '?' indicator.  Note that
// the Scanner still produce the KEY token whenever it encounters a simple key.
//
// For scanning block collections, the following tokens are used (note that we
// repeat KEY and VALUE here):
//
//      BLOCK-SEQUENCE-START
//      BLOCK-MAPPING-START
//      BLOCK-END
//      BLOCK-ENTRY
//      KEY
//      VALUE
//
// The tokens BLOCK-SEQUENCE-START and BLOCK-MAPPING-START denote indentation
// increase that precedes a block collection (cf. the INDENT token in Python).
// The token BLOCK-END denote indentation decrease that ends a block collection
// (cf. the DEDENT token in Python).  However YAML has some syntax pecularities
// that makes detections of these tokens more complex.
//
// The tokens BLOCK-ENTRY, KEY, and VALUE are used to represent the indicators
// '-', '?', and ':' correspondingly.
//
// The following examples show how the tokens BLOCK-SEQUENCE-START,
// BLOCK-MAPPING-START, and BLOCK-END are emitted by the Scanner:
//
//      1. Block sequences:
//
//          - item 1
//          - item 2
//          -
//            - item 3.1
//            - item 3.2
//          -
//            key 1: value 1
//            key 2: value 2
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          BLOCK-SEQUENCE-START
//          BLOCK-ENTRY
//          SCALAR("item 1",plain)
//          BLOCK-ENTRY
//          SCALAR("item 2",plain)
//          BLOCK-ENTRY
//          BLOCK-SEQUENCE-START
//          BLOCK-ENTRY
//          SCALAR("item 3.1",plain)
//          BLOCK-ENTRY
//          SCALAR("item 3.2",plain)
//          BLOCK-END
//          BLOCK-ENTRY
//          BLOCK-MAPPING-START
//          KEY
//          SCALAR("key 1",plain)
//          VALUE
//          SCALAR("value 1",plain)
//          KEY
//          SCALAR("key 2",plain)
//          VALUE
//          SCALAR("value 2",plain)
//          BLOCK-END
//          BLOCK-END
//          STREAM-END
//
//      2. Block mappings:
//
//          a simple key: a value   # The KEY token is produced here.
//          ? a complex key
//          : another value
//          a mapping:
//            key 1: value 1
//            key 2: value 2
//          a sequence:
//            - item 1
//            - item 2
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          BLOCK-MAPPING-START
//          KEY
//          SCALAR("a simple key",plain)
//          VALUE
//          SCALAR("a value",plain)
//          KEY
//          SCALAR("a complex key",plain)
//          VALUE
//          SCALAR("another value",plain)
//          KEY
//          SCALAR("a mapping",plain)
//          BLOCK-MAPPING-START
//          KEY
//          SCALAR("key 1",plain)
//          VALUE
//          SCALAR("value 1",plain)
//          KEY
//          SCALAR("key 2",plain)
//          VALUE
//          SCALAR("value 2",plain)
//          BLOCK-END
//          KEY
//          SCALAR("a sequence",plain)
//          VALUE
//          BLOCK-SEQUENCE-START
//          BLOCK-ENTRY
//          SCALAR("item 1",plain)
//          BLOCK-ENTRY
//          SCALAR("item 2",plain)
//          BLOCK-END
//          BLOCK-END
//          STREAM-END
//
// YAML does not always require to start a new block collection from a new
// line.  If the current line contains only '-', '?', and ':' indicators, a new
// block collection may start at the current line.  The following examples
// illustrate this case:
//
//      1. Collections in a sequence:
//
//          - - item 1
//            - item 2
//          - key 1: value 1
//            key 2: value 2
//          - ? complex key
//            : complex value
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          BLOCK-SEQUENCE-START
//          BLOCK-ENTRY
//          BLOCK-SEQUENCE-START
//          BLOCK-ENTRY
//          SCALAR("item 1",plain)
//          BLOCK-ENTRY
//          SCALAR("item 2",plain)
//          BLOCK-END
//          BLOCK-ENTRY
//          BLOCK-MAPPING-START
//          KEY
//          SCALAR("key 1",plain)
//          VALUE
//          SCALAR("value 1",plain)
//          KEY
//          SCALAR("key 2",plain)
//          VALUE
//          SCALAR("value 2",plain)
//          BLOCK-END
//          BLOCK-ENTRY
//          BLOCK-MAPPING-START
//          KEY
//          SCALAR("complex key")
//          VALUE
//          SCALAR("complex value")
//          BLOCK-END
//          BLOCK-END
//          STREAM-END
//
//      2. Collections in a mapping:
//
//          ? a sequence
//          : - item 1
//            - item 2
//          ? a mapping
//          : key 1: value 1
//            key 2: value 2
//
//      Tokens:
//
//          STREAM-START(utf-8)
//          BLOCK-MAPPING-START
//          KEY
//          SCALAR("a sequence",plain)
//          VALUE
//          BLOCK-SEQUENCE-START
//          BLOCK-ENTRY
//          SCALAR("item 1",plain)
//          BLOCK-ENTRY
//          SCALAR("item 2",plain)
//          BLOCK-END
//          KEY
//          SCALAR("a mapping",plain)
//          VALUE
//          BLOCK-MAPPING-START
//          KEY
//          SCALAR("key 1",plain)
//          VALUE
//          SCALAR("value 1",plain)
//          KEY
//          SCALAR("key 2",plain)
//          VALUE
//          SCALAR("value 2",plain)
//          BLOCK-END
//          BLOCK-END
//          STREAM-END
//
// YAML also permits non-indented sequences if they are included into a block
// mapping.  In this case, the token BLOCK-SEQUENCE-START is not produced:
//
//      key:
//      - item 1    # BLOCK-SEQUENCE-START is NOT produced here.
//      - item 2
//
// Tokens:
//
//      STREAM-START(utf-8)
//      BLOCK-MAPPING-START
//      KEY
//      SCALAR("key",plain)
//      VALUE
//      BLOCK-ENTRY
//      SCALAR("item 1",plain)
//      BLOCK-ENTRY
//      SCALAR("item 2",plain)
//      BLOCK-END
//

func yaml_insert_token(parser *YamlParser, pos int, token *yamlh.YamlToken) {
	// Check if we can move the queue at the beginning of the buffer.
	if parser.Tokens_head > 0 && len(parser.Tokens) == cap(parser.Tokens) {
		if parser.Tokens_head != len(parser.Tokens) {
			copy(parser.Tokens, parser.Tokens[parser.Tokens_head:])
		}
		parser.Tokens = parser.Tokens[:len(parser.Tokens)-parser.Tokens_head]
		parser.Tokens_head = 0
	}
	parser.Tokens = append(parser.Tokens, *token)
	if pos < 0 {
		return
	}
	copy(parser.Tokens[parser.Tokens_head+pos+1:], parser.Tokens[parser.Tokens_head+pos:])
	parser.Tokens[parser.Tokens_head+pos] = *token
}

// Advance the buffer pointer.
func skip(parser *YamlParser) {
	if !yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
		parser.Newlines = 0
	}
	parser.Mark.Index++
	parser.Mark.Column++
	parser.Unread--
	parser.Buffer_pos += yamlh.Width(parser.Buffer[parser.Buffer_pos])
}

func skip_line(parser *YamlParser) {
	if yamlh.Is_crlf(parser.Buffer, parser.Buffer_pos) {
		parser.Mark.Index += 2
		parser.Mark.Column = 0
		parser.Mark.Line++
		parser.Unread -= 2
		parser.Buffer_pos += 2
		parser.Newlines++
	} else if yamlh.Is_break(parser.Buffer, parser.Buffer_pos) {
		parser.Mark.Index++
		parser.Mark.Column = 0
		parser.Mark.Line++
		parser.Unread--
		parser.Buffer_pos += yamlh.Width(parser.Buffer[parser.Buffer_pos])
		parser.Newlines++
	}
}

// Copy a character to a string buffer and advance pointers.
func read(parser *YamlParser, s []byte) []byte {
	if !yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
		parser.Newlines = 0
	}
	w := yamlh.Width(parser.Buffer[parser.Buffer_pos])
	if w == 0 {
		panic("invalid character sequence")
	}
	if len(s) == 0 {
		s = make([]byte, 0, 32)
	}
	if w == 1 && len(s)+w <= cap(s) {
		s = s[:len(s)+1]
		s[len(s)-1] = parser.Buffer[parser.Buffer_pos]
		parser.Buffer_pos++
	} else {
		s = append(s, parser.Buffer[parser.Buffer_pos:parser.Buffer_pos+w]...)
		parser.Buffer_pos += w
	}
	parser.Mark.Index++
	parser.Mark.Column++
	parser.Unread--
	return s
}

// Copy a line break character to a string buffer and advance pointers.
func read_line(parser *YamlParser, s []byte) []byte {
	buf := parser.Buffer
	pos := parser.Buffer_pos
	switch {
	case buf[pos] == '\r' && buf[pos+1] == '\n':
		// CR LF . LF
		s = append(s, '\n')
		parser.Buffer_pos += 2
		parser.Mark.Index++
		parser.Unread--
	case buf[pos] == '\r' || buf[pos] == '\n':
		// CR|LF . LF
		s = append(s, '\n')
		parser.Buffer_pos += 1
	case buf[pos] == '\xC2' && buf[pos+1] == '\x85':
		// NEL . LF
		s = append(s, '\n')
		parser.Buffer_pos += 2
	case buf[pos] == '\xE2' && buf[pos+1] == '\x80' && (buf[pos+2] == '\xA8' || buf[pos+2] == '\xA9'):
		// LS|PS . LS|PS
		s = append(s, buf[parser.Buffer_pos:pos+3]...)
		parser.Buffer_pos += 3
	default:
		return s
	}
	parser.Mark.Index++
	parser.Mark.Column = 0
	parser.Mark.Line++
	parser.Unread--
	parser.Newlines++
	return s
}

// Set the scanner error and return the error.
func newScannerError(parser *YamlParser, context_mark yamlh.Position, problem string) error {
	return buildParserError(yamlh.SCANNER_ERROR, problem, parser.Mark.Line, context_mark.Line)
}

// Ensure that the tokens queue contains at least one token which can be
// returned to the parser.
func yaml_parser_fetch_more_tokens(parser *YamlParser) error {
	// While we need more tokens to fetch, do it.
	for {
		// [Go] The comment parsing logic requires a lookahead of two tokens
		// so that foot comments may be parsed in time of associating them
		// with the tokens that are parsed before them, and also for line
		// comments to be transformed into head comments in some edge cases.
		if parser.Tokens_head < len(parser.Tokens)-2 {
			// If a potential simple key is at the head position, we need to fetch
			// the next token to disambiguate it.
			head_tok_idx, ok := parser.Simple_keys_by_tok[parser.Tokens_parsed]
			if !ok {
				break
			}
			valid, err := yaml_simple_key_is_valid(parser, &parser.Simple_keys[head_tok_idx])
			if err != nil {
				return err
			}
			if !valid {
				break
			}
		}
		// Fetch the next token.
		err := yaml_parser_fetch_next_token(parser)
		if err != nil {
			return err
		}
	}

	parser.Token_available = true
	return nil
}

// The dispatcher for token fetchers.
func yaml_parser_fetch_next_token(parser *YamlParser) (errOut error) {
	// Ensure that the buffer is initialized.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return err
		}
	}

	// Check if we just started scanning.  Fetch STREAM-START then.
	if !parser.Stream_start_produced {
		yaml_parser_fetch_stream_start(parser)
		return nil
	}

	scan_mark := parser.Mark

	// Eat whitespaces and comments until we reach the next token.
	err := yaml_parser_scan_to_next_token(parser)
	if err != nil {
		return err
	}

	// [Go] While unrolling indents, transform the head comments of prior
	// indentation levels observed after scan_start into foot comments at
	// the respective indexes.

	// Check the indentation level against the current column.
	yaml_parser_unroll_indent(parser, parser.Mark.Column, scan_mark)

	// Ensure that the buffer contains at least 4 characters.  4 is the length
	// of the longest indicators ('--- ' and '... ').
	if parser.Unread < 4 {
		err = yaml_parser_update_buffer(parser, 4)
		if err != nil {
			return err
		}
	}
	// Is it the end of the stream?
	if yamlh.Is_z(parser.Buffer, parser.Buffer_pos) {
		return yaml_parser_fetch_stream_end(parser)
	}

	// Is it a directive?
	if parser.Mark.Column == 0 && parser.Buffer[parser.Buffer_pos] == '%' {
		return yaml_parser_fetch_directive(parser)
	}

	buf := parser.Buffer
	pos := parser.Buffer_pos

	// Is it the document start indicator?
	if parser.Mark.Column == 0 && buf[pos] == '-' && buf[pos+1] == '-' && buf[pos+2] == '-' && yamlh.Is_blankz(buf, pos+3) {
		return yaml_parser_fetch_document_indicator(parser, yamlh.DOCUMENT_START_TOKEN)
	}

	// Is it the document end indicator?
	if parser.Mark.Column == 0 && buf[pos] == '.' && buf[pos+1] == '.' && buf[pos+2] == '.' && yamlh.Is_blankz(buf, pos+3) {
		return yaml_parser_fetch_document_indicator(parser, yamlh.DOCUMENT_END_TOKEN)
	}

	comment_mark := parser.Mark
	if len(parser.Tokens) > 0 && (parser.Flow_level == 0 && buf[pos] == ':' || parser.Flow_level > 0 && buf[pos] == ',') {
		// Associate any following comments with the prior token.
		comment_mark = parser.Tokens[len(parser.Tokens)-1].Start_mark
	}
	defer func() {
		if errOut != nil {
			return
		}
		if len(parser.Tokens) > 0 && parser.Tokens[len(parser.Tokens)-1].Type == yamlh.BLOCK_ENTRY_TOKEN {
			// Sequence indicators alone have no line comments. It becomes
			// a head comment for whatever follows.
			return
		}
		errOut = yaml_parser_scan_line_comment(parser, comment_mark)
	}()

	switch {
	case buf[pos] == '[':
		return yaml_parser_fetch_flow_collection_start(parser, yamlh.FLOW_SEQUENCE_START_TOKEN)
	case parser.Buffer[parser.Buffer_pos] == '{':
		return yaml_parser_fetch_flow_collection_start(parser, yamlh.FLOW_MAPPING_START_TOKEN)
	case parser.Buffer[parser.Buffer_pos] == ']':
		return yaml_parser_fetch_flow_collection_end(parser, yamlh.FLOW_SEQUENCE_END_TOKEN)
	case parser.Buffer[parser.Buffer_pos] == '}':
		return yaml_parser_fetch_flow_collection_end(parser, yamlh.FLOW_MAPPING_END_TOKEN)
	case parser.Buffer[parser.Buffer_pos] == ',':
		return yaml_parser_fetch_flow_entry(parser)
	case parser.Buffer[parser.Buffer_pos] == '-' && yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos+1):
		return yaml_parser_fetch_block_entry(parser)
	case parser.Buffer[parser.Buffer_pos] == '?' && (parser.Flow_level > 0 || yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos+1)):
		return yaml_parser_fetch_key(parser)
	case parser.Buffer[parser.Buffer_pos] == ':' && (parser.Flow_level > 0 || yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos+1)):
		return yaml_parser_fetch_value(parser)
	case parser.Buffer[parser.Buffer_pos] == '*':
		return yaml_parser_fetch_anchor(parser, yamlh.ALIAS_TOKEN)
	case parser.Buffer[parser.Buffer_pos] == '&':
		return yaml_parser_fetch_anchor(parser, yamlh.ANCHOR_TOKEN)
	case parser.Buffer[parser.Buffer_pos] == '!':
		return yaml_parser_fetch_tag(parser)
	case parser.Buffer[parser.Buffer_pos] == '|' && parser.Flow_level == 0:
		return yaml_parser_fetch_block_scalar(parser, true)
	case parser.Buffer[parser.Buffer_pos] == '>' && parser.Flow_level == 0:
		return yaml_parser_fetch_block_scalar(parser, false)
	case parser.Buffer[parser.Buffer_pos] == '\'':
		return yaml_parser_fetch_flow_scalar(parser, true)
	case parser.Buffer[parser.Buffer_pos] == '"':
		return yaml_parser_fetch_flow_scalar(parser, false)
	}
	// Is it a plain scalar?
	//
	// A plain scalar may start with any non-blank characters except
	//
	//      '-', '?', ':', ',', '[', ']', '{', '}',
	//      '#', '&', '*', '!', '|', '>', '\'', '\"',
	//      '%', '@', '`'.
	//
	// In the block context (and, for the '-' indicator, in the flow context
	// too), it may also start with the characters
	//
	//      '-', '?', ':'
	//
	// if it is followed by a non-space character.
	//
	// The last rule is more restrictive than the specification requires.
	// [Go] TODO Make this logic more reasonable.
	//switch parser.buffer[parser.buffer_pos] {
	//case '-', '?', ':', ',', '?', '-', ',', ':', ']', '[', '}', '{', '&', '#', '!', '*', '>', '|', '"', '\'', '@', '%', '-', '`':
	//}
	if !(yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos) || parser.Buffer[parser.Buffer_pos] == '-' ||
		parser.Buffer[parser.Buffer_pos] == '?' || parser.Buffer[parser.Buffer_pos] == ':' ||
		parser.Buffer[parser.Buffer_pos] == ',' || parser.Buffer[parser.Buffer_pos] == '[' ||
		parser.Buffer[parser.Buffer_pos] == ']' || parser.Buffer[parser.Buffer_pos] == '{' ||
		parser.Buffer[parser.Buffer_pos] == '}' || parser.Buffer[parser.Buffer_pos] == '#' ||
		parser.Buffer[parser.Buffer_pos] == '&' || parser.Buffer[parser.Buffer_pos] == '*' ||
		parser.Buffer[parser.Buffer_pos] == '!' || parser.Buffer[parser.Buffer_pos] == '|' ||
		parser.Buffer[parser.Buffer_pos] == '>' || parser.Buffer[parser.Buffer_pos] == '\'' ||
		parser.Buffer[parser.Buffer_pos] == '"' || parser.Buffer[parser.Buffer_pos] == '%' ||
		parser.Buffer[parser.Buffer_pos] == '@' || parser.Buffer[parser.Buffer_pos] == '`') ||
		(parser.Buffer[parser.Buffer_pos] == '-' && !yamlh.Is_blank(parser.Buffer, parser.Buffer_pos+1)) ||
		(parser.Flow_level == 0 &&
			(parser.Buffer[parser.Buffer_pos] == '?' || parser.Buffer[parser.Buffer_pos] == ':') &&
			!yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos+1)) {
		return yaml_parser_fetch_plain_scalar(parser)
	}

	return newScannerError(parser, parser.Mark, "found character that cannot start any token")
}

func yaml_simple_key_is_valid(parser *YamlParser, simple_key *yamlh.SimpleKey) (bool, error) {
	if !simple_key.Possible {
		return false, nil
	}

	// The 1.2 specification says:
	//
	//     "If the ? indicator is omitted, parsing needs to see past the
	//     implicit key to recognize it as such. To limit the amount of
	//     lookahead required, the “:” indicator must appear at most 1024
	//     Unicode characters beyond the start of the key. In addition, the key
	//     is restricted to a single line."
	//
	if simple_key.Mark.Line < parser.Mark.Line || simple_key.Mark.Index+1024 < parser.Mark.Index {
		// Check if the potential simple key to be removed is required.
		if simple_key.Required {
			return false, newScannerError(parser, simple_key.Mark, "could not find expected ':'")
		}
		simple_key.Possible = false
		return false, nil
	}
	return true, nil
}

// Check if a simple key may start at the current position and add it if
// needed.
func yaml_parser_save_simple_key(parser *YamlParser) error {
	// A simple key is required at the current position if the scanner is in
	// the block context and the current column coincides with the indentation
	// level.

	required := parser.Flow_level == 0 && parser.Indent == parser.Mark.Column

	//
	// If the current position may start a simple key, save it.
	//
	if parser.Simple_key_allowed {
		simple_key := yamlh.SimpleKey{
			Possible:     true,
			Required:     required,
			Token_number: parser.Tokens_parsed + (len(parser.Tokens) - parser.Tokens_head),
			Mark:         parser.Mark,
		}

		err := yaml_parser_remove_simple_key(parser)
		if err != nil {
			return err
		}
		parser.Simple_keys[len(parser.Simple_keys)-1] = simple_key
		parser.Simple_keys_by_tok[simple_key.Token_number] = len(parser.Simple_keys) - 1
	}
	return nil
}

// Remove a potential simple key at the current flow level.
func yaml_parser_remove_simple_key(parser *YamlParser) error {
	i := len(parser.Simple_keys) - 1
	if parser.Simple_keys[i].Possible {
		// If the key is required, it is an error.
		if parser.Simple_keys[i].Required {
			return newScannerError(parser, parser.Simple_keys[i].Mark, "could not find expected ':'")
		}
		// Remove the key from the stack.
		parser.Simple_keys[i].Possible = false
		delete(parser.Simple_keys_by_tok, parser.Simple_keys[i].Token_number)
	}
	return nil
}

// max_flow_level limits the flow_level
const max_flow_level = 10000

// Increase the flow level and resize the simple key list if needed.
func yaml_parser_increase_flow_level(parser *YamlParser) error {
	// Reset the simple key on the next level.
	parser.Simple_keys = append(parser.Simple_keys, yamlh.SimpleKey{
		Possible:     false,
		Required:     false,
		Token_number: parser.Tokens_parsed + (len(parser.Tokens) - parser.Tokens_head),
		Mark:         parser.Mark,
	})

	// Increase the flow level.
	parser.Flow_level++
	if parser.Flow_level > max_flow_level {
		return newScannerError(parser, parser.Simple_keys[len(parser.Simple_keys)-1].Mark, fmt.Sprintf("exceeded max depth of %d", max_flow_level))
	}
	return nil
}

// Decrease the flow level.
func yaml_parser_decrease_flow_level(parser *YamlParser) {
	if parser.Flow_level > 0 {
		parser.Flow_level--
		last := len(parser.Simple_keys) - 1
		delete(parser.Simple_keys_by_tok, parser.Simple_keys[last].Token_number)
		parser.Simple_keys = parser.Simple_keys[:last]
	}
}

// max_indents limits the indents stack size
const max_indents = 10000

// Push the current indentation level to the stack and set the new level
// the current column is greater than the indentation level.  In this case,
// append or insert the specified token into the token queue.
func yaml_parser_roll_indent(parser *YamlParser, column, number int, typ yamlh.TokenType, mark yamlh.Position) error {
	// In the flow context, do nothing.
	if parser.Flow_level > 0 {
		return nil
	}

	if parser.Indent < column {
		// Push the current indentation level to the stack and set the new
		// indentation level.
		parser.Indents = append(parser.Indents, parser.Indent)
		parser.Indent = column
		if len(parser.Indents) > max_indents {
			return newScannerError(parser, parser.Simple_keys[len(parser.Simple_keys)-1].Mark, fmt.Sprintf("exceeded max depth of %d", max_indents))
		}

		// Create a token and insert it into the queue.
		token := yamlh.YamlToken{
			Type:       typ,
			Start_mark: mark,
			End_mark:   mark,
		}
		if number > -1 {
			number -= parser.Tokens_parsed
		}
		yaml_insert_token(parser, number, &token)
	}
	return nil
}

// Pop indentation levels from the indents stack until the current level
// becomes less or equal to the column.  For each indentation level, append
// the BLOCK-END token.
func yaml_parser_unroll_indent(parser *YamlParser, column int, scan_mark yamlh.Position) {
	// In the flow context, do nothing.
	if parser.Flow_level > 0 {
		return
	}

	block_mark := scan_mark
	block_mark.Index--

	// Loop through the indentation levels in the stack.
	for parser.Indent > column {

		// [Go] Reposition the end token before potential following
		//      foot comments of parent blocks. For that, search
		//      backwards for recent comments that were at the same
		//      indent as the block that is ending now.
		stop_index := block_mark.Index
		for i := len(parser.Comments) - 1; i >= 0; i-- {
			comment := &parser.Comments[i]

			if comment.End_mark.Index < stop_index {
				// Don't go back beyond the start of the comment/whitespace scan, unless column < 0.
				// If requested indent column is < 0, then the document is over and everything else
				// is a foot anyway.
				break
			}
			if comment.Start_mark.Column == parser.Indent+1 {
				// This is a good match. But maybe there's a former comment
				// at that same indent level, so keep searching.
				block_mark = comment.Start_mark
			}

			// While the end of the former comment matches with
			// the start of the following one, we know there's
			// nothing in between and scanning is still safe.
			stop_index = comment.Scan_mark.Index
		}

		// Create a token and append it to the queue.
		token := yamlh.YamlToken{
			Type:       yamlh.BLOCK_END_TOKEN,
			Start_mark: block_mark,
			End_mark:   block_mark,
		}
		yaml_insert_token(parser, -1, &token)

		// Pop the indentation level.
		parser.Indent = parser.Indents[len(parser.Indents)-1]
		parser.Indents = parser.Indents[:len(parser.Indents)-1]
	}
}

// Initialize the scanner and produce the STREAM-START token.
func yaml_parser_fetch_stream_start(parser *YamlParser) {

	// Set the initial indentation.
	parser.Indent = -1

	// Initialize the simple key stack.
	parser.Simple_keys = append(parser.Simple_keys, yamlh.SimpleKey{})

	parser.Simple_keys_by_tok = make(map[int]int)

	// A simple key is allowed at the beginning of the stream.
	parser.Simple_key_allowed = true

	// We have started.
	parser.Stream_start_produced = true

	// Create the STREAM-START token and append it to the queue.
	token := yamlh.YamlToken{
		Type:       yamlh.STREAM_START_TOKEN,
		Start_mark: parser.Mark,
		End_mark:   parser.Mark,
		Encoding:   parser.Encoding,
	}
	yaml_insert_token(parser, -1, &token)
}

// Produce the STREAM-END token and shut down the scanner.
func yaml_parser_fetch_stream_end(parser *YamlParser) error {

	// Force new line.
	if parser.Mark.Column != 0 {
		parser.Mark.Column = 0
		parser.Mark.Line++
	}

	// Reset the indentation level.
	yaml_parser_unroll_indent(parser, -1, parser.Mark)

	// Reset simple keys.
	err := yaml_parser_remove_simple_key(parser)
	if err != nil {
		return err
	}

	parser.Simple_key_allowed = false

	// Create the STREAM-END token and append it to the queue.
	token := yamlh.YamlToken{
		Type:       yamlh.STREAM_END_TOKEN,
		Start_mark: parser.Mark,
		End_mark:   parser.Mark,
	}
	yaml_insert_token(parser, -1, &token)
	return nil
}

// Produce a VERSION-DIRECTIVE or TAG-DIRECTIVE token.
func yaml_parser_fetch_directive(parser *YamlParser) error {
	// Reset the indentation level.
	yaml_parser_unroll_indent(parser, -1, parser.Mark)

	// Reset simple keys.
	err := yaml_parser_remove_simple_key(parser)
	if err != nil {
		return err
	}

	parser.Simple_key_allowed = false

	// Create the YAML-DIRECTIVE or TAG-DIRECTIVE token.
	token, err := yaml_parser_scan_directive(parser)
	if err != nil {
		return err
	}
	// Append the token to the queue.
	yaml_insert_token(parser, -1, token)
	return nil
}

// Produce the DOCUMENT-START or DOCUMENT-END token.
func yaml_parser_fetch_document_indicator(parser *YamlParser, typ yamlh.TokenType) error {
	// Reset the indentation level.
	yaml_parser_unroll_indent(parser, -1, parser.Mark)

	// Reset simple keys.
	err := yaml_parser_remove_simple_key(parser)
	if err != nil {
		return err
	}

	parser.Simple_key_allowed = false

	// Consume the token.
	start_mark := parser.Mark

	skip(parser)
	skip(parser)
	skip(parser)

	end_mark := parser.Mark

	// Create the DOCUMENT-START or DOCUMENT-END token.
	token := yamlh.YamlToken{
		Type:       typ,
		Start_mark: start_mark,
		End_mark:   end_mark,
	}
	// Append the token to the queue.
	yaml_insert_token(parser, -1, &token)
	return nil
}

// Produce the FLOW-SEQUENCE-START or FLOW-MAPPING-START token.
func yaml_parser_fetch_flow_collection_start(parser *YamlParser, typ yamlh.TokenType) error {

	// The indicators '[' and '{' may start a simple key.
	err := yaml_parser_save_simple_key(parser)
	if err != nil {
		return err
	}

	// Increase the flow level.
	err = yaml_parser_increase_flow_level(parser)
	if err != nil {
		return err
	}

	// A simple key may follow the indicators '[' and '{'.
	parser.Simple_key_allowed = true

	// Consume the token.
	start_mark := parser.Mark
	skip(parser)
	end_mark := parser.Mark

	// Create the FLOW-SEQUENCE-START of FLOW-MAPPING-START token.
	token := yamlh.YamlToken{
		Type:       typ,
		Start_mark: start_mark,
		End_mark:   end_mark,
	}
	// Append the token to the queue.
	yaml_insert_token(parser, -1, &token)
	return nil
}

// Produce the FLOW-SEQUENCE-END or FLOW-MAPPING-END token.
func yaml_parser_fetch_flow_collection_end(parser *YamlParser, typ yamlh.TokenType) error {
	// Reset any potential simple key on the current flow level.
	err := yaml_parser_remove_simple_key(parser)
	if err != nil {
		return err
	}

	// Decrease the flow level.
	yaml_parser_decrease_flow_level(parser)

	// No simple keys after the indicators ']' and '}'.
	parser.Simple_key_allowed = false

	// Consume the token.

	start_mark := parser.Mark
	skip(parser)
	end_mark := parser.Mark

	// Create the FLOW-SEQUENCE-END of FLOW-MAPPING-END token.
	token := yamlh.YamlToken{
		Type:       typ,
		Start_mark: start_mark,
		End_mark:   end_mark,
	}
	// Append the token to the queue.
	yaml_insert_token(parser, -1, &token)
	return nil
}

// Produce the FLOW-ENTRY token.
func yaml_parser_fetch_flow_entry(parser *YamlParser) error {
	// Reset any potential simple keys on the current flow level.
	err := yaml_parser_remove_simple_key(parser)
	if err != nil {
		return err
	}

	// Simple keys are allowed after ','.
	parser.Simple_key_allowed = true

	// Consume the token.
	start_mark := parser.Mark
	skip(parser)
	end_mark := parser.Mark

	// Create the FLOW-ENTRY token and append it to the queue.
	token := yamlh.YamlToken{
		Type:       yamlh.FLOW_ENTRY_TOKEN,
		Start_mark: start_mark,
		End_mark:   end_mark,
	}
	yaml_insert_token(parser, -1, &token)
	return nil
}

// Produce the BLOCK-ENTRY token.
func yaml_parser_fetch_block_entry(parser *YamlParser) error {
	// Check if the scanner is in the block context.
	if parser.Flow_level == 0 {
		// Check if we are allowed to start a new entry.
		if !parser.Simple_key_allowed {
			return newScannerError(parser, parser.Mark, "block sequence entries are not allowed in this context")
		}
		// Add the BLOCK-SEQUENCE-START token if needed.
		err := yaml_parser_roll_indent(parser, parser.Mark.Column, -1, yamlh.BLOCK_SEQUENCE_START_TOKEN, parser.Mark)
		if err != nil {
			return err
		}
	}

	// Reset any potential simple keys on the current flow level.
	err := yaml_parser_remove_simple_key(parser)
	if err != nil {
		return err
	}

	// Simple keys are allowed after '-'.
	parser.Simple_key_allowed = true

	// Consume the token.
	start_mark := parser.Mark
	skip(parser)
	end_mark := parser.Mark

	// Create the BLOCK-ENTRY token and append it to the queue.
	token := yamlh.YamlToken{
		Type:       yamlh.BLOCK_ENTRY_TOKEN,
		Start_mark: start_mark,
		End_mark:   end_mark,
	}
	yaml_insert_token(parser, -1, &token)
	return nil
}

// Produce the KEY token.
func yaml_parser_fetch_key(parser *YamlParser) error {

	// In the block context, additional checks are required.
	if parser.Flow_level == 0 {
		// Check if we are allowed to start a new key (not nessesary simple).
		if !parser.Simple_key_allowed {
			return newScannerError(parser, parser.Mark, "mapping keys are not allowed in this context")
		}
		// Add the BLOCK-MAPPING-START token if needed.
		err := yaml_parser_roll_indent(parser, parser.Mark.Column, -1, yamlh.BLOCK_MAPPING_START_TOKEN, parser.Mark)
		if err != nil {
			return err
		}
	}

	// Reset any potential simple keys on the current flow level.
	err := yaml_parser_remove_simple_key(parser)
	if err != nil {
		return err
	}

	// Simple keys are allowed after '?' in the block context.
	parser.Simple_key_allowed = parser.Flow_level == 0

	// Consume the token.
	start_mark := parser.Mark
	skip(parser)
	end_mark := parser.Mark

	// Create the KEY token and append it to the queue.
	token := yamlh.YamlToken{
		Type:       yamlh.KEY_TOKEN,
		Start_mark: start_mark,
		End_mark:   end_mark,
	}
	yaml_insert_token(parser, -1, &token)
	return nil
}

// Produce the VALUE token.
func yaml_parser_fetch_value(parser *YamlParser) error {

	simple_key := &parser.Simple_keys[len(parser.Simple_keys)-1]

	// Have we found a simple key?
	valid, err := yaml_simple_key_is_valid(parser, simple_key)
	if err != nil {
		return err
	}
	if valid {

		// Create the KEY token and insert it into the queue.
		token := yamlh.YamlToken{
			Type:       yamlh.KEY_TOKEN,
			Start_mark: simple_key.Mark,
			End_mark:   simple_key.Mark,
		}
		yaml_insert_token(parser, simple_key.Token_number-parser.Tokens_parsed, &token)

		// In the block context, we may need to add the BLOCK-MAPPING-START token.
		err = yaml_parser_roll_indent(parser, simple_key.Mark.Column, simple_key.Token_number, yamlh.BLOCK_MAPPING_START_TOKEN, simple_key.Mark)
		if err != nil {
			return err
		}

		// Remove the simple key.
		simple_key.Possible = false
		delete(parser.Simple_keys_by_tok, simple_key.Token_number)

		// A simple key cannot follow another simple key.
		parser.Simple_key_allowed = false

	} else {
		// The ':' indicator follows a complex key.

		// In the block context, extra checks are required.
		if parser.Flow_level == 0 {

			// Check if we are allowed to start a complex value.
			if !parser.Simple_key_allowed {
				return newScannerError(parser, parser.Mark, "mapping values are not allowed in this context")
			}

			// Add the BLOCK-MAPPING-START token if needed.
			err = yaml_parser_roll_indent(parser, parser.Mark.Column, -1, yamlh.BLOCK_MAPPING_START_TOKEN, parser.Mark)
			if err != nil {
				return err
			}
		}

		// Simple keys after ':' are allowed in the block context.
		parser.Simple_key_allowed = parser.Flow_level == 0
	}

	// Consume the token.
	start_mark := parser.Mark
	skip(parser)
	end_mark := parser.Mark

	// Create the VALUE token and append it to the queue.
	token := yamlh.YamlToken{
		Type:       yamlh.VALUE_TOKEN,
		Start_mark: start_mark,
		End_mark:   end_mark,
	}
	yaml_insert_token(parser, -1, &token)
	return nil
}

// Produce the ALIAS or ANCHOR token.
func yaml_parser_fetch_anchor(parser *YamlParser, typ yamlh.TokenType) error {
	// An anchor or an alias could be a simple key.
	err := yaml_parser_save_simple_key(parser)
	if err != nil {
		return err
	}

	// A simple key cannot follow an anchor or an alias.
	parser.Simple_key_allowed = false

	// Create the ALIAS or ANCHOR token and append it to the queue.
	token, err := yaml_parser_scan_anchor(parser, typ)
	if err != nil {
		return err
	}
	yaml_insert_token(parser, -1, token)
	return nil
}

// Produce the TAG token.
func yaml_parser_fetch_tag(parser *YamlParser) error {
	// A tag could be a simple key.
	err := yaml_parser_save_simple_key(parser)
	if err != nil {
		return err
	}

	// A simple key cannot follow a tag.
	parser.Simple_key_allowed = false

	// Create the TAG token and append it to the queue.
	token, err := yaml_parser_scan_tag(parser)
	if err != nil {
		return err
	}
	yaml_insert_token(parser, -1, token)
	return nil
}

// Produce the SCALAR(...,literal) or SCALAR(...,folded) tokens.
func yaml_parser_fetch_block_scalar(parser *YamlParser, literal bool) error {
	// Remove any potential simple keys.
	err := yaml_parser_remove_simple_key(parser)
	if err != nil {
		return err
	}

	// A simple key may follow a block scalar.
	parser.Simple_key_allowed = true

	// Create the SCALAR token and append it to the queue.
	token, err := yaml_parser_scan_block_scalar(parser, literal)
	if err != nil {
		return err
	}
	yaml_insert_token(parser, -1, token)
	return nil
}

// Produce the SCALAR(...,single-quoted) or SCALAR(...,double-quoted) tokens.
func yaml_parser_fetch_flow_scalar(parser *YamlParser, single bool) error {
	// A plain scalar could be a simple key.
	err := yaml_parser_save_simple_key(parser)
	if err != nil {
		return err
	}

	// A simple key cannot follow a flow scalar.
	parser.Simple_key_allowed = false

	// Create the SCALAR token and append it to the queue.
	token, err := yaml_parser_scan_flow_scalar(parser, single)
	if err != nil {
		return err
	}
	yaml_insert_token(parser, -1, token)
	return nil
}

// Produce the SCALAR(...,plain) token.
func yaml_parser_fetch_plain_scalar(parser *YamlParser) error {
	// A plain scalar could be a simple key.
	err := yaml_parser_save_simple_key(parser)
	if err != nil {
		return err
	}

	// A simple key cannot follow a flow scalar.
	parser.Simple_key_allowed = false

	// Create the SCALAR token and append it to the queue.
	token, err := yaml_parser_scan_plain_scalar(parser)
	if err != nil {
		return err
	}
	yaml_insert_token(parser, -1, token)
	return nil
}

// Eat whitespaces and comments until the next token is found.
func yaml_parser_scan_to_next_token(parser *YamlParser) error {

	scan_mark := parser.Mark

	// Until the next token is not found.
	for {
		// Allow the BOM mark to start a line.
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return err
			}
		}
		if parser.Mark.Column == 0 && yamlh.Is_bom(parser.Buffer, parser.Buffer_pos) {
			skip(parser)
		}

		// Eat whitespaces.
		// Tabs are allowed:
		//  - in the flow context
		//  - in the block context, but not at the beginning of the line or
		//  after '-', '?', or ':' (complex value).
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return err
			}
		}

		for parser.Buffer[parser.Buffer_pos] == ' ' || ((parser.Flow_level > 0 || !parser.Simple_key_allowed) && parser.Buffer[parser.Buffer_pos] == '\t') {
			skip(parser)
			if parser.Unread < 1 {
				err := yaml_parser_update_buffer(parser, 1)
				if err != nil {
					return err
				}
			}
		}

		// Check if we just had a line comment under a sequence entry that
		// looks more like a header to the following content. Similar to this:
		//
		// - # The comment
		//   - Some data
		//
		// If so, transform the line comment to a head comment and reposition.
		if len(parser.Comments) > 0 && len(parser.Tokens) > 1 {
			tokenA := parser.Tokens[len(parser.Tokens)-2]
			tokenB := parser.Tokens[len(parser.Tokens)-1]
			comment := &parser.Comments[len(parser.Comments)-1]
			if tokenA.Type == yamlh.BLOCK_SEQUENCE_START_TOKEN && tokenB.Type == yamlh.BLOCK_ENTRY_TOKEN && len(comment.Line) > 0 && !yamlh.Is_break(parser.Buffer, parser.Buffer_pos) {
				// If it was in the prior line, reposition so it becomes a
				// header of the follow up token. Otherwise, keep it in place
				// so it becomes a header of the former.
				comment.Head = comment.Line
				comment.Line = nil
				if comment.Start_mark.Line == parser.Mark.Line-1 {
					comment.Token_mark = parser.Mark
				}
			}
		}

		// Eat a comment until a line break.
		if parser.Buffer[parser.Buffer_pos] == '#' {
			err := yaml_parser_scan_comments(parser, scan_mark)
			if err != nil {
				return err
			}
		}

		// If it is a line break, eat it.
		if yamlh.Is_break(parser.Buffer, parser.Buffer_pos) {
			if parser.Unread < 2 {
				err := yaml_parser_update_buffer(parser, 2)
				if err != nil {
					return err
				}
			}
			skip_line(parser)

			// In the block context, a new line may start a simple key.
			if parser.Flow_level == 0 {
				parser.Simple_key_allowed = true
			}
		} else {
			break // We have found a token.
		}
	}

	return nil
}

// Scan a YAML-DIRECTIVE or TAG-DIRECTIVE token.
//
// Scope:
//
//	%YAML    1.1    # a comment \n
//	^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
//	%TAG    !yaml!  tag:yaml.org,2002:  \n
//	^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
func yaml_parser_scan_directive(parser *YamlParser) (*yamlh.YamlToken, error) {
	// Eat '%'.
	start_mark := parser.Mark
	skip(parser)

	// Scan the directive name.
	name, err := yaml_parser_scan_directive_name(parser, start_mark)
	if err != nil {
		return nil, err
	}

	var token yamlh.YamlToken

	// Is it a YAML directive?
	if bytes.Equal(name, []byte("YAML")) {
		// Scan the VERSION directive value.
		var major, minor int8
		major, minor, err = yaml_parser_scan_version_directive_value(parser, start_mark)
		if err != nil {
			return nil, err
		}
		end_mark := parser.Mark

		// Create a VERSION-DIRECTIVE token.
		token = yamlh.YamlToken{
			Type:       yamlh.VERSION_DIRECTIVE_TOKEN,
			Start_mark: start_mark,
			End_mark:   end_mark,
			Major:      major,
			Minor:      minor,
		}

		// Is it a TAG directive?
	} else if bytes.Equal(name, []byte("TAG")) {
		// Scan the TAG directive value.
		var handle, prefix []byte
		handle, prefix, err = yaml_parser_scan_tag_directive_value(parser, start_mark)
		if err != nil {
			return nil, err
		}
		end_mark := parser.Mark

		// Create a TAG-DIRECTIVE token.
		token = yamlh.YamlToken{
			Type:       yamlh.TAG_DIRECTIVE_TOKEN,
			Start_mark: start_mark,
			End_mark:   end_mark,
			Value:      handle,
			Prefix:     prefix,
		}

		// Unknown directive.
	} else {
		return nil, newScannerError(parser, start_mark, "found unknown directive name")
	}

	// Eat the rest of the line including any comments.
	if parser.Unread < 1 {
		err = yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, err
		}
	}

	for yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
		skip(parser)
		if parser.Unread < 1 {
			err = yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}
	}

	if parser.Buffer[parser.Buffer_pos] == '#' {
		// [Go] Discard this inline comment for the time being.
		//if !yaml_parser_scan_line_comment(parser, start_mark) {
		//	return false
		//}
		for !yamlh.Is_breakz(parser.Buffer, parser.Buffer_pos) {
			skip(parser)
			if parser.Unread < 1 {
				err = yaml_parser_update_buffer(parser, 1)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// Check if we are at the end of the line.
	if !yamlh.Is_breakz(parser.Buffer, parser.Buffer_pos) {
		return nil, newScannerError(parser, start_mark, "did not find expected comment or line break")
	}

	// Eat a line break.
	if yamlh.Is_break(parser.Buffer, parser.Buffer_pos) {
		if parser.Unread < 2 {
			err = yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}
		skip_line(parser)
	}

	return &token, nil
}

// Scan the directive name.
//
// Scope:
//
//	%YAML   1.1     # a comment \n
//	 ^^^^
//	%TAG    !yaml!  tag:yaml.org,2002:  \n
//	 ^^^
func yaml_parser_scan_directive_name(parser *YamlParser, start_mark yamlh.Position) ([]byte, error) {
	// Consume the directive name.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, err
		}
	}

	var s []byte
	for yamlh.Is_alpha(parser.Buffer, parser.Buffer_pos) {
		s = read(parser, s)
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}
	}

	// Check if the name is empty.
	if len(s) == 0 {
		return nil, newScannerError(parser, start_mark, "could not find expected directive name")
	}

	// Check for an blank character after the name.
	if !yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos) {
		return nil, newScannerError(parser, start_mark, "found unexpected non-alphabetical character")
	}
	return s, nil
}

// Scan the value of VERSION-DIRECTIVE.
//
// Scope:
//
//	%YAML   1.1     # a comment \n
//	     ^^^^^^
func yaml_parser_scan_version_directive_value(parser *YamlParser, start_mark yamlh.Position) (major, minor int8, _ error) {
	// Eat whitespaces.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return 0, 0, err
		}
	}
	for yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
		skip(parser)
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return 0, 0, err
			}
		}
	}

	// Consume the major version number.
	major, err := yaml_parser_scan_version_directive_number(parser, start_mark)
	if err != nil {
		return 0, 0, err
	}

	// Eat '.'.
	if parser.Buffer[parser.Buffer_pos] != '.' {
		return 0, 0, newScannerError(parser, start_mark, "did not find expected digit or '.' character")
	}

	skip(parser)

	// Consume the minor version number.
	minor, err = yaml_parser_scan_version_directive_number(parser, start_mark)
	if err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

const max_number_length = 2

// Scan the version number of VERSION-DIRECTIVE.
//
// Scope:
//
//	%YAML   1.1     # a comment \n
//	        ^
//	%YAML   1.1     # a comment \n
//	          ^
func yaml_parser_scan_version_directive_number(parser *YamlParser, start_mark yamlh.Position) (int8, error) {

	// Repeat while the next character is digit.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return 0, err
		}
	}
	var value, length int8
	for yamlh.Is_digit(parser.Buffer, parser.Buffer_pos) {
		// Check if the number is too long.
		length++
		if length > max_number_length {
			return 0, newScannerError(parser, start_mark, "found extremely long version number")
		}
		value = value*10 + int8(yamlh.As_digit(parser.Buffer, parser.Buffer_pos))
		skip(parser)
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return 0, err
			}
		}
	}

	// Check if the number was present.
	if length == 0 {
		return 0, newScannerError(parser, start_mark, "did not find expected version number")
	}
	return value, nil
}

// Scan the value of a TAG-DIRECTIVE token.
//
// Scope:
//
//	%TAG    !yaml!  tag:yaml.org,2002:  \n
//	    ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
func yaml_parser_scan_tag_directive_value(parser *YamlParser, start_mark yamlh.Position) (handle, prefix []byte, _ error) {
	var handle_value, prefix_value []byte

	// Eat whitespaces.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, nil, err
		}
	}

	for yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
		skip(parser)
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// Scan a handle.
	err := yaml_parser_scan_tag_handle(parser, true, start_mark, &handle_value)
	if err != nil {
		return nil, nil, err
	}

	// expect a whitespace.
	if parser.Unread < 1 {
		err = yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, nil, err
		}
	}
	if !yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
		return nil, nil, newScannerError(parser, start_mark, "did not find expected whitespace")
	}

	// Eat whitespaces.
	for yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
		skip(parser)
		if parser.Unread < 1 {
			err = yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// Scan a prefix.
	err = yaml_parser_scan_tag_uri(parser, true, nil, start_mark, &prefix_value)
	if err != nil {
		return nil, nil, err
	}

	// expect a whitespace or line break.
	if parser.Unread < 1 {
		err = yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, nil, err
		}
	}
	if !yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos) {
		return nil, nil, newScannerError(parser, start_mark, "did not find expected whitespace or line break")

	}

	return handle_value, prefix_value, nil
}

func yaml_parser_scan_anchor(parser *YamlParser, typ yamlh.TokenType) (*yamlh.YamlToken, error) {
	var s []byte

	// Eat the indicator character.
	start_mark := parser.Mark
	skip(parser)

	// Consume the value.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, err
		}
	}

	for yamlh.Is_alpha(parser.Buffer, parser.Buffer_pos) {
		s = read(parser, s)
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}
	}

	end_mark := parser.Mark

	/*
	 * Check if length of the anchor is greater than 0 and it is followed by
	 * a whitespace character or one of the indicators:
	 *
	 *      '?', ':', ',', ']', '}', '%', '@', '`'.
	 */

	if len(s) == 0 ||
		!(yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos) || parser.Buffer[parser.Buffer_pos] == '?' ||
			parser.Buffer[parser.Buffer_pos] == ':' || parser.Buffer[parser.Buffer_pos] == ',' ||
			parser.Buffer[parser.Buffer_pos] == ']' || parser.Buffer[parser.Buffer_pos] == '}' ||
			parser.Buffer[parser.Buffer_pos] == '%' || parser.Buffer[parser.Buffer_pos] == '@' ||
			parser.Buffer[parser.Buffer_pos] == '`') {
		return nil, newScannerError(parser, start_mark, "did not find expected alphabetic or numeric character")
	}

	// Create a token.
	token := yamlh.YamlToken{
		Type:       typ,
		Start_mark: start_mark,
		End_mark:   end_mark,
		Value:      s,
	}

	return &token, nil
}

/*
 * Scan a TAG token.
 */

func yaml_parser_scan_tag(parser *YamlParser) (*yamlh.YamlToken, error) {
	var handle, suffix []byte

	start_mark := parser.Mark

	// Check if the tag is in the canonical form.
	if parser.Unread < 2 {
		err := yaml_parser_update_buffer(parser, 2)
		if err != nil {
			return nil, err
		}
	}

	if parser.Buffer[parser.Buffer_pos+1] == '<' {
		// Keep the handle as ''

		// Eat '!<'
		skip(parser)
		skip(parser)

		// Consume the tag value.
		err := yaml_parser_scan_tag_uri(parser, false, nil, start_mark, &suffix)
		if err != nil {
			return nil, err
		}

		// Check for '>' and eat it.
		if parser.Buffer[parser.Buffer_pos] != '>' {
			return nil, newScannerError(parser, start_mark, "did not find the expected '>'")
		}

		skip(parser)
	} else {
		// The tag has either the '!suffix' or the '!handle!suffix' form.

		// First, try to scan a handle.
		err := yaml_parser_scan_tag_handle(parser, false, start_mark, &handle)
		if err != nil {
			return nil, err
		}

		// Check if it is, indeed, handle.
		if handle[0] == '!' && len(handle) > 1 && handle[len(handle)-1] == '!' {
			// Scan the suffix now.
			err = yaml_parser_scan_tag_uri(parser, false, nil, start_mark, &suffix)
			if err != nil {
				return nil, err
			}
		} else {
			// It wasn't a handle after all.  Scan the rest of the tag.
			err = yaml_parser_scan_tag_uri(parser, false, handle, start_mark, &suffix)
			if err != nil {
				return nil, err
			}

			// Set the handle to '!'.
			handle = []byte{'!'}

			// A special case: the '!' tag.  Set the handle to '' and the
			// suffix to '!'.
			if len(suffix) == 0 {
				handle, suffix = suffix, handle
			}
		}
	}

	// Check the character which ends the tag.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, err
		}
	}
	if !yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos) {
		return nil, newScannerError(parser, start_mark, "did not find expected whitespace or line break")
	}

	end_mark := parser.Mark

	// Create a token.
	token := yamlh.YamlToken{
		Type:       yamlh.TAG_TOKEN,
		Start_mark: start_mark,
		End_mark:   end_mark,
		Value:      handle,
		Suffix:     suffix,
	}
	return &token, nil
}

// Scan a tag handle.
func yaml_parser_scan_tag_handle(parser *YamlParser, directive bool, start_mark yamlh.Position, handle *[]byte) error {
	// Check the initial '!' character.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return err
		}
	}
	if parser.Buffer[parser.Buffer_pos] != '!' {
		return newScannerError(parser, start_mark, "did not find expected '!'")
	}

	var s []byte

	// Copy the '!' character.
	s = read(parser, s)

	// Copy all subsequent alphabetical and numerical characters.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return err
		}
	}
	for yamlh.Is_alpha(parser.Buffer, parser.Buffer_pos) {
		s = read(parser, s)
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return err
			}
		}
	}

	// Check if the trailing character is '!' and copy it.
	if parser.Buffer[parser.Buffer_pos] == '!' {
		s = read(parser, s)
	} else {
		// It's either the '!' tag or not really a tag handle.  If it's a %TAG
		// directive, it's an error.  If it's a tag token, it must be a part of URI.
		if directive && string(s) != "!" {
			return newScannerError(parser, start_mark, "did not find expected '!'")
		}
	}

	*handle = s
	return nil
}

// Scan a tag.
func yaml_parser_scan_tag_uri(parser *YamlParser, directive bool, head []byte, start_mark yamlh.Position, uri *[]byte) error {
	//size_t length = head ? strlen((char *)head) : 0
	var s []byte
	hasTag := len(head) > 0

	// Copy the head if needed.
	//
	// Note that we don't copy the leading '!' character.
	if len(head) > 1 {
		s = append(s, head[1:]...)
	}

	// Scan the tag.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return err
		}
	}

	// The set of characters that may appear in URI is as follows:
	//
	//      '0'-'9', 'A'-'Z', 'a'-'z', '_', '-', ';', '/', '?', ':', '@', '&',
	//      '=', '+', '$', ',', '.', '!', '~', '*', '\'', '(', ')', '[', ']',
	//      '%'.
	// [Go] TODO Convert this into more reasonable logic.
	for yamlh.Is_alpha(parser.Buffer, parser.Buffer_pos) || parser.Buffer[parser.Buffer_pos] == ';' ||
		parser.Buffer[parser.Buffer_pos] == '/' || parser.Buffer[parser.Buffer_pos] == '?' ||
		parser.Buffer[parser.Buffer_pos] == ':' || parser.Buffer[parser.Buffer_pos] == '@' ||
		parser.Buffer[parser.Buffer_pos] == '&' || parser.Buffer[parser.Buffer_pos] == '=' ||
		parser.Buffer[parser.Buffer_pos] == '+' || parser.Buffer[parser.Buffer_pos] == '$' ||
		parser.Buffer[parser.Buffer_pos] == ',' || parser.Buffer[parser.Buffer_pos] == '.' ||
		parser.Buffer[parser.Buffer_pos] == '!' || parser.Buffer[parser.Buffer_pos] == '~' ||
		parser.Buffer[parser.Buffer_pos] == '*' || parser.Buffer[parser.Buffer_pos] == '\'' ||
		parser.Buffer[parser.Buffer_pos] == '(' || parser.Buffer[parser.Buffer_pos] == ')' ||
		parser.Buffer[parser.Buffer_pos] == '[' || parser.Buffer[parser.Buffer_pos] == ']' ||
		parser.Buffer[parser.Buffer_pos] == '%' {
		// Check if it is a URI-escape sequence.
		if parser.Buffer[parser.Buffer_pos] == '%' {
			err := yaml_parser_scan_uri_escapes(parser, directive, start_mark, &s)
			if err != nil {
				return err
			}
		} else {
			s = read(parser, s)
		}
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return err
			}
		}
		hasTag = true
	}

	if !hasTag {
		return newScannerError(parser, start_mark, "did not find expected tag URI")
	}
	*uri = s
	return nil
}

// Decode an URI-escape sequence corresponding to a single UTF-8 character.
func yaml_parser_scan_uri_escapes(parser *YamlParser, directive bool, start_mark yamlh.Position, s *[]byte) error {

	// Decode the required number of characters.
	w := 1024
	for w > 0 {
		// Check for a URI-escaped octet.
		if parser.Unread < 3 {
			err := yaml_parser_update_buffer(parser, 3)
			if err != nil {
				return err
			}
		}

		if !(parser.Buffer[parser.Buffer_pos] == '%' &&
			yamlh.Is_hex(parser.Buffer, parser.Buffer_pos+1) &&
			yamlh.Is_hex(parser.Buffer, parser.Buffer_pos+2)) {
			return newScannerError(parser, start_mark, "did not find URI escaped octet")
		}

		// Get the octet.
		octet := byte((yamlh.As_hex(parser.Buffer, parser.Buffer_pos+1) << 4) + yamlh.As_hex(parser.Buffer, parser.Buffer_pos+2))

		// If it is the leading octet, determine the length of the UTF-8 sequence.
		if w == 1024 {
			w = yamlh.Width(octet)
			if w == 0 {
				return newScannerError(parser, start_mark, "found an incorrect leading UTF-8 octet")
			}
		} else {
			// Check if the trailing octet is correct.
			if octet&0xC0 != 0x80 {
				return newScannerError(parser, start_mark, "found an incorrect trailing UTF-8 octet")
			}
		}

		// Copy the octet and move the pointers.
		*s = append(*s, octet)
		skip(parser)
		skip(parser)
		skip(parser)
		w--
	}
	return nil
}

// Scan a block scalar.
func yaml_parser_scan_block_scalar(parser *YamlParser, literal bool) (*yamlh.YamlToken, error) {
	// Eat the indicator '|' or '>'.
	start_mark := parser.Mark
	skip(parser)

	// Scan the additional block scalar indicators.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, err
		}
	}

	// Check for a chomping indicator.
	var chomping, increment int
	if parser.Buffer[parser.Buffer_pos] == '+' || parser.Buffer[parser.Buffer_pos] == '-' {
		// Set the chomping method and eat the indicator.
		if parser.Buffer[parser.Buffer_pos] == '+' {
			chomping = +1
		} else {
			chomping = -1
		}
		skip(parser)

		// Check for an indentation indicator.
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}
		if yamlh.Is_digit(parser.Buffer, parser.Buffer_pos) {
			// Check that the indentation is greater than 0.
			if parser.Buffer[parser.Buffer_pos] == '0' {
				return nil, newScannerError(parser, start_mark, "found an indentation indicator equal to 0")
			}

			// Get the indentation level and eat the indicator.
			increment = yamlh.As_digit(parser.Buffer, parser.Buffer_pos)
			skip(parser)
		}

	} else if yamlh.Is_digit(parser.Buffer, parser.Buffer_pos) {
		// Do the same as above, but in the opposite order.

		if parser.Buffer[parser.Buffer_pos] == '0' {
			return nil, newScannerError(parser, start_mark, "found an indentation indicator equal to 0")
		}
		increment = yamlh.As_digit(parser.Buffer, parser.Buffer_pos)
		skip(parser)

		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}
		if parser.Buffer[parser.Buffer_pos] == '+' || parser.Buffer[parser.Buffer_pos] == '-' {
			if parser.Buffer[parser.Buffer_pos] == '+' {
				chomping = +1
			} else {
				chomping = -1
			}
			skip(parser)
		}
	}

	// Eat whitespaces and comments to the end of the line.
	if parser.Unread < 1 {
		err := yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, err
		}
	}
	for yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
		skip(parser)
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}
	}
	if parser.Buffer[parser.Buffer_pos] == '#' {
		err := yaml_parser_scan_line_comment(parser, start_mark)
		if err != nil {
			return nil, err
		}
		for !yamlh.Is_breakz(parser.Buffer, parser.Buffer_pos) {
			skip(parser)
			if parser.Unread < 1 {
				err = yaml_parser_update_buffer(parser, 1)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// Check if we are at the end of the line.
	if !yamlh.Is_breakz(parser.Buffer, parser.Buffer_pos) {
		return nil, newScannerError(parser, start_mark, "did not find expected comment or line break")

	}

	// Eat a line break.
	if yamlh.Is_break(parser.Buffer, parser.Buffer_pos) {
		if parser.Unread < 2 {
			err := yaml_parser_update_buffer(parser, 2)
			if err != nil {
				return nil, err
			}
		}
		skip_line(parser)
	}

	end_mark := parser.Mark

	// Set the indentation level if it was specified.
	var indent int
	if increment > 0 {
		if parser.Indent >= 0 {
			indent = parser.Indent + increment
		} else {
			indent = increment
		}
	}

	// Scan the leading line breaks and determine the indentation level if needed.
	var s, leading_break, trailing_breaks []byte
	err := yaml_parser_scan_block_scalar_breaks(parser, &indent, &trailing_breaks, start_mark, &end_mark)
	if err != nil {
		return nil, err
	}

	// Scan the block scalar content.
	if parser.Unread < 1 {
		err = yaml_parser_update_buffer(parser, 1)
		if err != nil {
			return nil, err
		}
	}
	var leading_blank, trailing_blank bool
	for parser.Mark.Column == indent && !yamlh.Is_z(parser.Buffer, parser.Buffer_pos) {
		// We are at the beginning of a non-empty line.

		// Is it a trailing whitespace?
		trailing_blank = yamlh.Is_blank(parser.Buffer, parser.Buffer_pos)

		// Check if we need to fold the leading line break.
		if !literal && !leading_blank && !trailing_blank && len(leading_break) > 0 && leading_break[0] == '\n' {
			// Do we need to join the lines by space?
			if len(trailing_breaks) == 0 {
				s = append(s, ' ')
			}
		} else {
			s = append(s, leading_break...)
		}
		leading_break = leading_break[:0]

		// Append the remaining line breaks.
		s = append(s, trailing_breaks...)
		trailing_breaks = trailing_breaks[:0]

		// Is it a leading whitespace?
		leading_blank = yamlh.Is_blank(parser.Buffer, parser.Buffer_pos)

		// Consume the current line.
		for !yamlh.Is_breakz(parser.Buffer, parser.Buffer_pos) {
			s = read(parser, s)
			if parser.Unread < 1 {
				err = yaml_parser_update_buffer(parser, 1)
				if err != nil {
					return nil, err
				}
			}
		}

		// Consume the line break.
		if parser.Unread < 2 {
			err = yaml_parser_update_buffer(parser, 2)
			if err != nil {
				return nil, err
			}
		}

		leading_break = read_line(parser, leading_break)

		// Eat the following indentation spaces and line breaks.
		err = yaml_parser_scan_block_scalar_breaks(parser, &indent, &trailing_breaks, start_mark, &end_mark)
		if err != nil {
			return nil, err
		}
	}

	// Chomp the tail.
	if chomping != -1 {
		s = append(s, leading_break...)
	}
	if chomping == 1 {
		s = append(s, trailing_breaks...)
	}

	// Create a token.
	token := yamlh.YamlToken{
		Type:       yamlh.SCALAR_TOKEN,
		Start_mark: start_mark,
		End_mark:   end_mark,
		Value:      s,
		Style:      yamlh.LITERAL_SCALAR_STYLE,
	}
	if !literal {
		token.Style = yamlh.FOLDED_SCALAR_STYLE
	}
	return &token, nil
}

// Scan indentation spaces and line breaks for a block scalar.  Determine the
// indentation level if needed.
func yaml_parser_scan_block_scalar_breaks(parser *YamlParser, indent *int, breaks *[]byte, start_mark yamlh.Position, end_mark *yamlh.Position) error {
	*end_mark = parser.Mark

	// Eat the indentation spaces and line breaks.
	max_indent := 0
	for {
		// Eat the indentation spaces.
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return err
			}
		}
		for (*indent == 0 || parser.Mark.Column < *indent) && yamlh.Is_space(parser.Buffer, parser.Buffer_pos) {
			skip(parser)
			if parser.Unread < 1 {
				err := yaml_parser_update_buffer(parser, 1)
				if err != nil {
					return err
				}
			}
		}
		if parser.Mark.Column > max_indent {
			max_indent = parser.Mark.Column
		}

		// Check for a tab character messing the indentation.
		if (*indent == 0 || parser.Mark.Column < *indent) && yamlh.Is_tab(parser.Buffer, parser.Buffer_pos) {
			return newScannerError(parser, start_mark, "found a tab character where an indentation space is expected")
		}

		// Have we found a non-empty line?
		if !yamlh.Is_break(parser.Buffer, parser.Buffer_pos) {
			break
		}

		// Consume the line break.
		if parser.Unread < 2 {
			err := yaml_parser_update_buffer(parser, 2)
			if err != nil {
				return err
			}
		}
		// [Go] Should really be returning breaks instead.
		*breaks = read_line(parser, *breaks)
		*end_mark = parser.Mark
	}

	// Determine the indentation level if needed.
	if *indent == 0 {
		*indent = max_indent
		if *indent < parser.Indent+1 {
			*indent = parser.Indent + 1
		}
		if *indent < 1 {
			*indent = 1
		}
	}
	return nil
}

// Scan a quoted scalar.
func yaml_parser_scan_flow_scalar(parser *YamlParser, single bool) (*yamlh.YamlToken, error) {
	// Eat the left quote.
	start_mark := parser.Mark
	skip(parser)

	// Consume the content of the quoted scalar.
	var s, leading_break, trailing_breaks, whitespaces []byte
	for {
		// Check that there are no document indicators at the beginning of the line.
		if parser.Unread < 4 {
			err := yaml_parser_update_buffer(parser, 4)
			if err != nil {
				return nil, err
			}
		}

		if parser.Mark.Column == 0 &&
			((parser.Buffer[parser.Buffer_pos+0] == '-' &&
				parser.Buffer[parser.Buffer_pos+1] == '-' &&
				parser.Buffer[parser.Buffer_pos+2] == '-') ||
				(parser.Buffer[parser.Buffer_pos+0] == '.' &&
					parser.Buffer[parser.Buffer_pos+1] == '.' &&
					parser.Buffer[parser.Buffer_pos+2] == '.')) &&
			yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos+3) {
			return nil, newScannerError(parser, start_mark, "found unexpected document indicator")
		}

		// Check for EOF.
		if yamlh.Is_z(parser.Buffer, parser.Buffer_pos) {
			return nil, newScannerError(parser, start_mark, "found unexpected end of stream")
		}

		// Consume non-blank characters.
		leading_blanks := false
		for !yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos) {
			if single && parser.Buffer[parser.Buffer_pos] == '\'' && parser.Buffer[parser.Buffer_pos+1] == '\'' {
				// Is is an escaped single quote.
				s = append(s, '\'')
				skip(parser)
				skip(parser)

			} else if single && parser.Buffer[parser.Buffer_pos] == '\'' {
				// It is a right single quote.
				break
			} else if !single && parser.Buffer[parser.Buffer_pos] == '"' {
				// It is a right double quote.
				break

			} else if !single && parser.Buffer[parser.Buffer_pos] == '\\' && yamlh.Is_break(parser.Buffer, parser.Buffer_pos+1) {
				// It is an escaped line break.
				if parser.Unread < 3 {
					err := yaml_parser_update_buffer(parser, 3)
					if err != nil {
						return nil, err
					}
				}
				skip(parser)
				skip_line(parser)
				leading_blanks = true
				break

			} else if !single && parser.Buffer[parser.Buffer_pos] == '\\' {
				// It is an escape sequence.
				code_length := 0

				// Check the escape character.
				switch parser.Buffer[parser.Buffer_pos+1] {
				case '0':
					s = append(s, 0)
				case 'a':
					s = append(s, '\x07')
				case 'b':
					s = append(s, '\x08')
				case 't', '\t':
					s = append(s, '\x09')
				case 'n':
					s = append(s, '\x0A')
				case 'v':
					s = append(s, '\x0B')
				case 'f':
					s = append(s, '\x0C')
				case 'r':
					s = append(s, '\x0D')
				case 'e':
					s = append(s, '\x1B')
				case ' ':
					s = append(s, '\x20')
				case '"':
					s = append(s, '"')
				case '\'':
					s = append(s, '\'')
				case '\\':
					s = append(s, '\\')
				case 'N': // NEL (#x85)
					s = append(s, '\xC2')
					s = append(s, '\x85')
				case '_': // #xA0
					s = append(s, '\xC2')
					s = append(s, '\xA0')
				case 'L': // LS (#x2028)
					s = append(s, '\xE2')
					s = append(s, '\x80')
					s = append(s, '\xA8')
				case 'P': // PS (#x2029)
					s = append(s, '\xE2')
					s = append(s, '\x80')
					s = append(s, '\xA9')
				case 'x':
					code_length = 2
				case 'u':
					code_length = 4
				case 'U':
					code_length = 8
				default:
					return nil, newScannerError(parser, start_mark, "found unknown escape character")
				}

				skip(parser)
				skip(parser)

				// Consume an arbitrary escape code.
				if code_length > 0 {
					var value int

					// Scan the character value.
					if parser.Unread < code_length {
						err := yaml_parser_update_buffer(parser, code_length)
						if err != nil {
							return nil, err
						}
					}
					for k := 0; k < code_length; k++ {
						if !yamlh.Is_hex(parser.Buffer, parser.Buffer_pos+k) {
							return nil, newScannerError(parser, start_mark, "did not find expected hexdecimal number")
						}
						value = (value << 4) + yamlh.As_hex(parser.Buffer, parser.Buffer_pos+k)
					}

					// Check the value and write the character.
					if (value >= 0xD800 && value <= 0xDFFF) || value > 0x10FFFF {
						return nil, newScannerError(parser, start_mark, "found invalid Unicode character escape code")
					}
					if value <= 0x7F {
						s = append(s, byte(value))
					} else if value <= 0x7FF {
						s = append(s, byte(0xC0+(value>>6)))
						s = append(s, byte(0x80+(value&0x3F)))
					} else if value <= 0xFFFF {
						s = append(s, byte(0xE0+(value>>12)))
						s = append(s, byte(0x80+((value>>6)&0x3F)))
						s = append(s, byte(0x80+(value&0x3F)))
					} else {
						s = append(s, byte(0xF0+(value>>18)))
						s = append(s, byte(0x80+((value>>12)&0x3F)))
						s = append(s, byte(0x80+((value>>6)&0x3F)))
						s = append(s, byte(0x80+(value&0x3F)))
					}

					// Advance the pointer.
					for k := 0; k < code_length; k++ {
						skip(parser)
					}
				}
			} else {
				// It is a non-escaped non-blank character.
				s = read(parser, s)
			}
			if parser.Unread < 2 {
				err := yaml_parser_update_buffer(parser, 2)
				if err != nil {
					return nil, err
				}
			}
		}

		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}

		// Check if we are at the end of the scalar.
		if single {
			if parser.Buffer[parser.Buffer_pos] == '\'' {
				break
			}
		} else {
			if parser.Buffer[parser.Buffer_pos] == '"' {
				break
			}
		}

		// Consume blank characters.
		for yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) || yamlh.Is_break(parser.Buffer, parser.Buffer_pos) {
			if yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {
				// Consume a space or a tab character.
				if !leading_blanks {
					whitespaces = read(parser, whitespaces)
				} else {
					skip(parser)
				}
			} else {
				if parser.Unread < 2 {
					err := yaml_parser_update_buffer(parser, 2)
					if err != nil {
						return nil, err
					}
				}
				// Check if it is a first line break.
				if !leading_blanks {
					whitespaces = whitespaces[:0]
					leading_break = read_line(parser, leading_break)
					leading_blanks = true
				} else {
					trailing_breaks = read_line(parser, trailing_breaks)
				}
			}
			if parser.Unread < 1 {
				err := yaml_parser_update_buffer(parser, 1)
				if err != nil {
					return nil, err
				}
			}
		}

		// Join the whitespaces or fold line breaks.
		if leading_blanks {
			// Do we need to fold line breaks?
			if len(leading_break) > 0 && leading_break[0] == '\n' {
				if len(trailing_breaks) == 0 {
					s = append(s, ' ')
				} else {
					s = append(s, trailing_breaks...)
				}
			} else {
				s = append(s, leading_break...)
				s = append(s, trailing_breaks...)
			}
			trailing_breaks = trailing_breaks[:0]
			leading_break = leading_break[:0]
		} else {
			s = append(s, whitespaces...)
			whitespaces = whitespaces[:0]
		}
	}

	// Eat the right quote.
	skip(parser)
	end_mark := parser.Mark

	// Create a token.
	token := yamlh.YamlToken{
		Type:       yamlh.SCALAR_TOKEN,
		Start_mark: start_mark,
		End_mark:   end_mark,
		Value:      s,
		Style:      yamlh.SINGLE_QUOTED_SCALAR_STYLE,
	}
	if !single {
		token.Style = yamlh.DOUBLE_QUOTED_SCALAR_STYLE
	}
	return &token, nil
}

// Scan a plain scalar.
func yaml_parser_scan_plain_scalar(parser *YamlParser) (*yamlh.YamlToken, error) {

	var s, leading_break, trailing_breaks, whitespaces []byte
	var leading_blanks bool
	var indent = parser.Indent + 1

	start_mark := parser.Mark
	end_mark := parser.Mark

	// Consume the content of the plain scalar.
	for {
		// Check for a document indicator.
		if parser.Unread < 4 {
			err := yaml_parser_update_buffer(parser, 4)
			if err != nil {
				return nil, err
			}
		}
		if parser.Mark.Column == 0 &&
			((parser.Buffer[parser.Buffer_pos+0] == '-' &&
				parser.Buffer[parser.Buffer_pos+1] == '-' &&
				parser.Buffer[parser.Buffer_pos+2] == '-') ||
				(parser.Buffer[parser.Buffer_pos+0] == '.' &&
					parser.Buffer[parser.Buffer_pos+1] == '.' &&
					parser.Buffer[parser.Buffer_pos+2] == '.')) &&
			yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos+3) {
			break
		}

		// Check for a comment.
		if parser.Buffer[parser.Buffer_pos] == '#' {
			break
		}

		// Consume non-blank characters.
		for !yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos) {

			// Check for indicators that may end a plain scalar.
			if (parser.Buffer[parser.Buffer_pos] == ':' && yamlh.Is_blankz(parser.Buffer, parser.Buffer_pos+1)) ||
				(parser.Flow_level > 0 &&
					(parser.Buffer[parser.Buffer_pos] == ',' ||
						parser.Buffer[parser.Buffer_pos] == '?' || parser.Buffer[parser.Buffer_pos] == '[' ||
						parser.Buffer[parser.Buffer_pos] == ']' || parser.Buffer[parser.Buffer_pos] == '{' ||
						parser.Buffer[parser.Buffer_pos] == '}')) {
				break
			}

			// Check if we need to join whitespaces and breaks.
			if leading_blanks || len(whitespaces) > 0 {
				if leading_blanks {
					// Do we need to fold line breaks?
					if leading_break[0] == '\n' {
						if len(trailing_breaks) == 0 {
							s = append(s, ' ')
						} else {
							s = append(s, trailing_breaks...)
						}
					} else {
						s = append(s, leading_break...)
						s = append(s, trailing_breaks...)
					}
					trailing_breaks = trailing_breaks[:0]
					leading_break = leading_break[:0]
					leading_blanks = false
				} else {
					s = append(s, whitespaces...)
					whitespaces = whitespaces[:0]
				}
			}

			// Copy the character.
			s = read(parser, s)

			end_mark = parser.Mark
			if parser.Unread < 2 {
				err := yaml_parser_update_buffer(parser, 2)
				if err != nil {
					return nil, err
				}
			}
		}

		// Is it the end?
		if !(yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) || yamlh.Is_break(parser.Buffer, parser.Buffer_pos)) {
			break
		}

		// Consume blank characters.
		if parser.Unread < 1 {
			err := yaml_parser_update_buffer(parser, 1)
			if err != nil {
				return nil, err
			}
		}

		for yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) || yamlh.Is_break(parser.Buffer, parser.Buffer_pos) {
			if yamlh.Is_blank(parser.Buffer, parser.Buffer_pos) {

				// Check for tab characters that abuse indentation.
				if leading_blanks && parser.Mark.Column < indent && yamlh.Is_tab(parser.Buffer, parser.Buffer_pos) {
					return nil, newScannerError(parser, start_mark, "found a tab character that violates indentation")
				}

				// Consume a space or a tab character.
				if !leading_blanks {
					whitespaces = read(parser, whitespaces)
				} else {
					skip(parser)
				}
			} else {
				if parser.Unread < 2 {
					err := yaml_parser_update_buffer(parser, 2)
					if err != nil {
						return nil, err
					}
				}

				// Check if it is a first line break.
				if !leading_blanks {
					whitespaces = whitespaces[:0]
					leading_break = read_line(parser, leading_break)
					leading_blanks = true
				} else {
					trailing_breaks = read_line(parser, trailing_breaks)
				}
			}
			if parser.Unread < 1 {
				err := yaml_parser_update_buffer(parser, 1)
				if err != nil {
					return nil, err
				}
			}
		}

		// Check indentation level.
		if parser.Flow_level == 0 && parser.Mark.Column < indent {
			break
		}
	}

	// Create a token.
	token := yamlh.YamlToken{
		Type:       yamlh.SCALAR_TOKEN,
		Start_mark: start_mark,
		End_mark:   end_mark,
		Value:      s,
		Style:      yamlh.PLAIN_SCALAR_STYLE,
	}

	// Note that we change the 'simple_key_allowed' flag.
	if leading_blanks {
		parser.Simple_key_allowed = true
	}
	return &token, nil
}

func yaml_parser_scan_line_comment(parser *YamlParser, token_mark yamlh.Position) error {
	if parser.Newlines > 0 {
		return nil
	}

	var start_mark yamlh.Position
	var text []byte

	for peek := 0; peek < 512; peek++ {
		if parser.Unread < peek+1 {
			err := yaml_parser_update_buffer(parser, peek+1)
			if err != nil {
				return err
			}
		}
		if yamlh.Is_blank(parser.Buffer, parser.Buffer_pos+peek) {
			continue
		}
		if parser.Buffer[parser.Buffer_pos+peek] == '#' {
			seen := parser.Mark.Index + peek
			for {
				if parser.Unread < 1 {
					err := yaml_parser_update_buffer(parser, 1)
					if err != nil {
						return err
					}
				}
				if yamlh.Is_breakz(parser.Buffer, parser.Buffer_pos) {
					if parser.Mark.Index >= seen {
						break
					}
					if parser.Unread < 2 {
						err := yaml_parser_update_buffer(parser, 2)
						if err != nil {
							return err
						}
					}
					skip_line(parser)
				} else if parser.Mark.Index >= seen {
					if len(text) == 0 {
						start_mark = parser.Mark
					}
					text = read(parser, text)
				} else {
					skip(parser)
				}
			}
		}
		break
	}
	if len(text) > 0 {
		parser.Comments = append(parser.Comments, yamlh.YamlComment{
			Token_mark: token_mark,
			Start_mark: start_mark,
			Line:       text,
		})
	}
	return nil
}

func yaml_parser_scan_comments(parser *YamlParser, scan_mark yamlh.Position) error {
	token := parser.Tokens[len(parser.Tokens)-1]

	if token.Type == yamlh.FLOW_ENTRY_TOKEN && len(parser.Tokens) > 1 {
		token = parser.Tokens[len(parser.Tokens)-2]
	}

	var token_mark = token.Start_mark
	var start_mark yamlh.Position
	var next_indent = parser.Indent
	if next_indent < 0 {
		next_indent = 0
	}

	var recent_empty = false
	var first_empty = parser.Newlines <= 1

	var line = parser.Mark.Line
	var column = parser.Mark.Column

	var text []byte

	// The foot line is the place where a comment must start to
	// still be considered as a foot of the prior content.
	// If there's some content in the currently parsed line, then
	// the foot is the line below it.
	var foot_line = -1
	if scan_mark.Line > 0 {
		foot_line = parser.Mark.Line - parser.Newlines + 1
		if parser.Newlines == 0 && parser.Mark.Column > 1 {
			foot_line++
		}
	}

	var peek = 0
	for ; peek < 512; peek++ {
		if parser.Unread < peek+1 && yaml_parser_update_buffer(parser, peek+1) != nil {
			break
		}
		column++
		if yamlh.Is_blank(parser.Buffer, parser.Buffer_pos+peek) {
			continue
		}
		c := parser.Buffer[parser.Buffer_pos+peek]
		var close_flow = parser.Flow_level > 0 && (c == ']' || c == '}')
		if close_flow || yamlh.Is_breakz(parser.Buffer, parser.Buffer_pos+peek) {
			// Got line break or terminator.
			if close_flow || !recent_empty {
				if close_flow || first_empty && (start_mark.Line == foot_line && token.Type != yamlh.VALUE_TOKEN || start_mark.Column-1 < next_indent) {
					// This is the first empty line and there were no empty lines before,
					// so this initial part of the comment is a foot of the prior token
					// instead of being a head for the following one. Split it up.
					// Alternatively, this might also be the last comment inside a flow
					// scope, so it must be a footer.
					if len(text) > 0 {
						if start_mark.Column-1 < next_indent {
							// If dedented it's unrelated to the prior token.
							token_mark = start_mark
						}
						parser.Comments = append(parser.Comments, yamlh.YamlComment{
							Scan_mark:  scan_mark,
							Token_mark: token_mark,
							Start_mark: start_mark,
							End_mark:   yamlh.Position{parser.Mark.Index + peek, line, column},
							Foot:       text,
						})
						scan_mark = yamlh.Position{parser.Mark.Index + peek, line, column}
						token_mark = scan_mark
						text = nil
					}
				} else {
					if len(text) > 0 && parser.Buffer[parser.Buffer_pos+peek] != 0 {
						text = append(text, '\n')
					}
				}
			}
			if !yamlh.Is_break(parser.Buffer, parser.Buffer_pos+peek) {
				break
			}
			first_empty = false
			recent_empty = true
			column = 0
			line++
			continue
		}

		if len(text) > 0 && (close_flow || column-1 < next_indent && column != start_mark.Column) {
			// The comment at the different indentation is a foot of the
			// preceding data rather than a head of the upcoming one.
			parser.Comments = append(parser.Comments, yamlh.YamlComment{
				Scan_mark:  scan_mark,
				Token_mark: token_mark,
				Start_mark: start_mark,
				End_mark:   yamlh.Position{parser.Mark.Index + peek, line, column},
				Foot:       text,
			})
			scan_mark = yamlh.Position{parser.Mark.Index + peek, line, column}
			token_mark = scan_mark
			text = nil
		}

		if parser.Buffer[parser.Buffer_pos+peek] != '#' {
			break
		}

		if len(text) == 0 {
			start_mark = yamlh.Position{parser.Mark.Index + peek, line, column}
		} else {
			text = append(text, '\n')
		}

		recent_empty = false

		// Consume until after the consumed comment line.
		seen := parser.Mark.Index + peek
		for {
			if parser.Unread < 1 {
				err := yaml_parser_update_buffer(parser, 1)
				if err != nil {
					return err
				}
			}
			if yamlh.Is_breakz(parser.Buffer, parser.Buffer_pos) {
				if parser.Mark.Index >= seen {
					break
				}
				if parser.Unread < 2 {
					err := yaml_parser_update_buffer(parser, 2)
					if err != nil {
						return err
					}
				}
				skip_line(parser)
			} else if parser.Mark.Index >= seen {
				text = read(parser, text)
			} else {
				skip(parser)
			}
		}

		peek = 0
		column = 0
		line = parser.Mark.Line
		next_indent = parser.Indent
		if next_indent < 0 {
			next_indent = 0
		}
	}

	if len(text) > 0 {
		parser.Comments = append(parser.Comments, yamlh.YamlComment{
			Scan_mark:  scan_mark,
			Token_mark: start_mark,
			Start_mark: start_mark,
			End_mark:   yamlh.Position{parser.Mark.Index + peek - 1, line, column},
			Head:       text,
		})
	}
	return nil
}
