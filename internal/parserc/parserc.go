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
	"gopkg.in/yaml.v3/internal/common"
	"gopkg.in/yaml.v3/internal/yamlh"
	"strconv"
)

// The parser implements the following grammar:
//
// stream               ::= STREAM-START implicit_document? explicit_document* STREAM-END
// implicit_document    ::= block_node DOCUMENT-END*
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
// block_node_or_indentless_sequence    ::=
//                          ALIAS
//                          | properties (block_content | indentless_block_sequence)?
//                          | block_content
//                          | indentless_block_sequence
// block_node           ::= ALIAS
//                          | properties block_content?
//                          | block_content
// flow_node            ::= ALIAS
//                          | properties flow_content?
//                          | flow_content
// properties           ::= TAG ANCHOR? | ANCHOR TAG?
// block_content        ::= block_collection | flow_collection | SCALAR
// flow_content         ::= flow_collection | SCALAR
// block_collection     ::= block_sequence | block_mapping
// flow_collection      ::= flow_sequence | flow_mapping
// block_sequence       ::= BLOCK-SEQUENCE-START (BLOCK-ENTRY block_node?)* BLOCK-END
// indentless_sequence  ::= (BLOCK-ENTRY block_node?)+
// block_mapping        ::= BLOCK-MAPPING_START
//                          ((KEY block_node_or_indentless_sequence?)?
//                          (VALUE block_node_or_indentless_sequence?)?)*
//                          BLOCK-END
// flow_sequence        ::= FLOW-SEQUENCE-START
//                          (flow_sequence_entry FLOW-ENTRY)*
//                          flow_sequence_entry?
//                          FLOW-SEQUENCE-END
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
// flow_mapping         ::= FLOW-MAPPING-START
//                          (flow_mapping_entry FLOW-ENTRY)*
//                          flow_mapping_entry?
//                          FLOW-MAPPING-END
// flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?

// Parse - Get the next event.
func Parse(parser *YamlParser) (*yamlh.Event, error) {
	// No events after the end of the stream or error.
	if parser.Stream_end_produced || parser.State == PARSE_END_STATE {
		return &yamlh.Event{}, nil
	}
	// Generate the next event.
	return yaml_parser_state_machine(parser)
}

// peek the next token in the token queue.
func peek_token(parser *YamlParser) (*yamlh.YamlToken, error) {
	if !parser.Token_available {
		err := yaml_parser_fetch_more_tokens(parser)
		if err != nil {
			return nil, err
		}
	}
	token := &parser.Tokens[parser.Tokens_head]
	yaml_parser_unfold_comments(parser, token)
	return token, nil
}

// yaml_parser_unfold_comments walks through the comments queue and joins all
// comments behind the position of the provided token into the respective
// top-level comment slices in the parser.
func yaml_parser_unfold_comments(parser *YamlParser, token *yamlh.YamlToken) {
	for parser.Comments_head < len(parser.Comments) && token.Start_mark.Index >= parser.Comments[parser.Comments_head].Token_mark.Index {
		comment := &parser.Comments[parser.Comments_head]
		if len(comment.Head) > 0 {
			if token.Type == yamlh.BLOCK_END_TOKEN {
				// No heads on ends, so keep comment.head for a follow up token.
				break
			}
			if len(parser.Head_comment) > 0 {
				parser.Head_comment = append(parser.Head_comment, '\n')
			}
			parser.Head_comment = append(parser.Head_comment, comment.Head...)
		}
		if len(comment.Foot) > 0 {
			if len(parser.Foot_comment) > 0 {
				parser.Foot_comment = append(parser.Foot_comment, '\n')
			}
			parser.Foot_comment = append(parser.Foot_comment, comment.Foot...)
		}
		if len(comment.Line) > 0 {
			if len(parser.Line_comment) > 0 {
				parser.Line_comment = append(parser.Line_comment, '\n')
			}
			parser.Line_comment = append(parser.Line_comment, comment.Line...)
		}
		*comment = yamlh.YamlComment{}
		parser.Comments_head++
	}
}

// Remove the next token from the queue (must be called after peek_token).
func skip_token(parser *YamlParser) {
	parser.Token_available = false
	parser.Tokens_parsed++
	parser.Stream_end_produced = parser.Tokens[parser.Tokens_head].Type == yamlh.STREAM_END_TOKEN
	parser.Tokens_head++
}

func buildParserError(errType yamlh.ErrorType, problem string, problemLine, contextLine int) error {
	if errType == yamlh.NO_ERROR {
		return nil
	}
	var where string
	line := contextLine
	if line == 0 {
		line = problemLine
	}
	if line != 0 {
		// Scanner errors don't iterate line before returning error
		if errType == yamlh.SCANNER_ERROR {
			line++
		}
		where = "line " + strconv.Itoa(line) + ": "
	}
	if problem == "" {
		problem = "unknown problem parsing YAML content"
	}
	return fmt.Errorf("yaml: %s%s", where, problem)
}

// State dispatcher.
func yaml_parser_state_machine(parser *YamlParser) (*yamlh.Event, error) {
	switch parser.State {
	case PARSE_STREAM_START_STATE:
		return yaml_parser_parse_stream_start(parser)

	case PARSE_IMPLICIT_DOCUMENT_START_STATE:
		return yaml_parser_parse_document_start(parser, true)

	case PARSE_DOCUMENT_START_STATE:
		return yaml_parser_parse_document_start(parser, false)

	case PARSE_DOCUMENT_CONTENT_STATE:
		return yaml_parser_parse_document_content(parser)

	case PARSE_DOCUMENT_END_STATE:
		return yaml_parser_parse_document_end(parser)

	case PARSE_BLOCK_NODE_STATE:
		return yaml_parser_parse_node(parser, true, false)

	case PARSE_BLOCK_NODE_OR_INDENTLESS_SEQUENCE_STATE:
		return yaml_parser_parse_node(parser, true, true)

	case PARSE_FLOW_NODE_STATE:
		return yaml_parser_parse_node(parser, false, false)

	case PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE:
		return yaml_parser_parse_block_sequence_entry(parser, true)

	case PARSE_BLOCK_SEQUENCE_ENTRY_STATE:
		return yaml_parser_parse_block_sequence_entry(parser, false)

	case PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE:
		return yaml_parser_parse_indentless_sequence_entry(parser)

	case PARSE_BLOCK_MAPPING_FIRST_KEY_STATE:
		return yaml_parser_parse_block_mapping_key(parser, true)

	case PARSE_BLOCK_MAPPING_KEY_STATE:
		return yaml_parser_parse_block_mapping_key(parser, false)

	case PARSE_BLOCK_MAPPING_VALUE_STATE:
		return yaml_parser_parse_block_mapping_value(parser)

	case PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE:
		return yaml_parser_parse_flow_sequence_entry(parser, true)

	case PARSE_FLOW_SEQUENCE_ENTRY_STATE:
		return yaml_parser_parse_flow_sequence_entry(parser, false)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE:
		return yaml_parser_parse_flow_sequence_entry_mapping_key(parser)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE:
		return yaml_parser_parse_flow_sequence_entry_mapping_value(parser)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE:
		return yaml_parser_parse_flow_sequence_entry_mapping_end(parser)

	case PARSE_FLOW_MAPPING_FIRST_KEY_STATE:
		return yaml_parser_parse_flow_mapping_key(parser, true)

	case PARSE_FLOW_MAPPING_KEY_STATE:
		return yaml_parser_parse_flow_mapping_key(parser, false)

	case PARSE_FLOW_MAPPING_VALUE_STATE:
		return yaml_parser_parse_flow_mapping_value(parser, false)

	case PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE:
		return yaml_parser_parse_flow_mapping_value(parser, true)

	default:
		panic("invalid parser state")
	}
}

// Parse the production:
// stream   ::= STREAM-START implicit_document? explicit_document* STREAM-END
//
//	************
func yaml_parser_parse_stream_start(parser *YamlParser) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}
	if token.Type != yamlh.STREAM_START_TOKEN {
		return nil, buildParserError(yamlh.PARSER_ERROR, "did not find expected <stream-start>", token.Start_mark.Line, 0)
	}
	parser.State = PARSE_IMPLICIT_DOCUMENT_START_STATE
	event := yamlh.Event{
		Type:       yamlh.STREAM_START_EVENT,
		Start_mark: token.Start_mark,
		End_mark:   token.End_mark,
		Encoding:   token.Encoding,
	}
	skip_token(parser)
	return &event, nil
}

// Parse the productions:
// implicit_document    ::= block_node DOCUMENT-END*
//
//	*
//
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
//
//	*************************
func yaml_parser_parse_document_start(parser *YamlParser, implicit bool) (*yamlh.Event, error) {

	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}

	// Parse extra document end indicators.
	if !implicit {
		for token.Type == yamlh.DOCUMENT_END_TOKEN {
			skip_token(parser)
			token, err = peek_token(parser)
			if err != nil {
				return nil, err
			}
		}
	}

	if implicit && token.Type != yamlh.VERSION_DIRECTIVE_TOKEN &&
		token.Type != yamlh.TAG_DIRECTIVE_TOKEN &&
		token.Type != yamlh.DOCUMENT_START_TOKEN &&
		token.Type != yamlh.STREAM_END_TOKEN {
		// Parse an implicit document.
		err = yaml_parser_process_directives(parser, nil, nil)
		if err != nil {
			return nil, err
		}
		parser.States = append(parser.States, PARSE_DOCUMENT_END_STATE)
		parser.State = PARSE_BLOCK_NODE_STATE

		var head_comment []byte
		if len(parser.Head_comment) > 0 {
			// [Go] Scan the header comment backwards, and if an empty line is found, break
			//      the header so the part before the last empty line goes into the
			//      document header, while the bottom of it goes into a follow up event.
			for i := len(parser.Head_comment) - 1; i > 0; i-- {
				if parser.Head_comment[i] == '\n' {
					if i == len(parser.Head_comment)-1 {
						head_comment = parser.Head_comment[:i]
						parser.Head_comment = parser.Head_comment[i+1:]
						break
					}
					if parser.Head_comment[i-1] == '\n' {
						head_comment = parser.Head_comment[:i-1]
						parser.Head_comment = parser.Head_comment[i+1:]
						break
					}
				}
			}
		}

		return &yamlh.Event{
			Type:       yamlh.DOCUMENT_START_EVENT,
			Start_mark: token.Start_mark,
			End_mark:   token.End_mark,

			Head_comment: head_comment,
		}, nil

	}
	if token.Type != yamlh.STREAM_END_TOKEN {
		// Parse an explicit document.
		var version_directive *yamlh.VersionDirective
		var tag_directives []yamlh.TagDirective
		start_mark := token.Start_mark
		err = yaml_parser_process_directives(parser, &version_directive, &tag_directives)
		if err != nil {
			return nil, err
		}
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type != yamlh.DOCUMENT_START_TOKEN {
			return nil, buildParserError(yamlh.PARSER_ERROR, "did not find expected <document start>", token.Start_mark.Line, 0)
		}
		parser.States = append(parser.States, PARSE_DOCUMENT_END_STATE)
		parser.State = PARSE_DOCUMENT_CONTENT_STATE
		end_mark := token.End_mark

		event := yamlh.Event{
			Type:              yamlh.DOCUMENT_START_EVENT,
			Start_mark:        start_mark,
			End_mark:          end_mark,
			Version_directive: version_directive,
			Tag_directives:    tag_directives,
			Implicit:          false,
		}
		skip_token(parser)
		return &event, nil
	}

	// Parse the stream end.
	parser.State = PARSE_END_STATE
	event := yamlh.Event{
		Type:       yamlh.STREAM_END_EVENT,
		Start_mark: token.Start_mark,
		End_mark:   token.End_mark,
	}
	skip_token(parser)

	return &event, nil
}

// Parse the productions:
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
//
//	***********
func yaml_parser_parse_document_content(parser *YamlParser) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}

	if token.Type == yamlh.VERSION_DIRECTIVE_TOKEN ||
		token.Type == yamlh.TAG_DIRECTIVE_TOKEN ||
		token.Type == yamlh.DOCUMENT_START_TOKEN ||
		token.Type == yamlh.DOCUMENT_END_TOKEN ||
		token.Type == yamlh.STREAM_END_TOKEN {
		parser.State = parser.States[len(parser.States)-1]
		parser.States = parser.States[:len(parser.States)-1]
		return yaml_parser_process_empty_scalar(token.Start_mark), nil

	}
	return yaml_parser_parse_node(parser, true, false)
}

// Parse the productions:
// implicit_document    ::= block_node DOCUMENT-END*
//
//	*************
//
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
func yaml_parser_parse_document_end(parser *YamlParser) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}

	start_mark := token.Start_mark
	end_mark := token.Start_mark

	implicit := true
	if token.Type == yamlh.DOCUMENT_END_TOKEN {
		end_mark = token.End_mark
		skip_token(parser)
		implicit = false
	}

	parser.Tag_directives = parser.Tag_directives[:0]

	parser.State = PARSE_DOCUMENT_START_STATE
	event := yamlh.Event{
		Type:       yamlh.DOCUMENT_END_EVENT,
		Start_mark: start_mark,
		End_mark:   end_mark,
		Implicit:   implicit,
	}
	yaml_parser_set_event_comments(parser, &event)
	if len(event.Head_comment) > 0 && len(event.Foot_comment) == 0 {
		event.Foot_comment = event.Head_comment
		event.Head_comment = nil
	}
	return &event, nil
}

func yaml_parser_set_event_comments(parser *YamlParser, event *yamlh.Event) {
	event.Head_comment = parser.Head_comment
	event.Line_comment = parser.Line_comment
	event.Foot_comment = parser.Foot_comment
	parser.Head_comment = nil
	parser.Line_comment = nil
	parser.Foot_comment = nil
	parser.Tail_comment = nil
	parser.Stem_comment = nil
}

// Parse the productions:
// block_node_or_indentless_sequence    ::=
//
//	ALIAS
//	*****
//	| properties (block_content | indentless_block_sequence)?
//	  **********  *
//	| block_content | indentless_block_sequence
//	  *
//
// block_node           ::= ALIAS
//
//	*****
//	| properties block_content?
//	  ********** *
//	| block_content
//	  *
//
// flow_node            ::= ALIAS
//
//	*****
//	| properties flow_content?
//	  ********** *
//	| flow_content
//	  *
//
// properties           ::= TAG ANCHOR? | ANCHOR TAG?
//
//	*************************
//
// block_content        ::= block_collection | flow_collection | SCALAR
//
//	******
//
// flow_content         ::= flow_collection | SCALAR
//
//	******
func yaml_parser_parse_node(parser *YamlParser, block, indentless_sequence bool) (*yamlh.Event, error) {
	var event yamlh.Event
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}

	if token.Type == yamlh.ALIAS_TOKEN {
		parser.State = parser.States[len(parser.States)-1]
		parser.States = parser.States[:len(parser.States)-1]
		event = yamlh.Event{
			Type:       yamlh.ALIAS_EVENT,
			Start_mark: token.Start_mark,
			End_mark:   token.End_mark,
			Anchor:     token.Value,
		}
		yaml_parser_set_event_comments(parser, &event)
		skip_token(parser)
		return &event, nil
	}

	start_mark := token.Start_mark
	end_mark := token.Start_mark

	var tag_token bool
	var tag_handle, tag_suffix, anchor []byte
	var tag_mark yamlh.Position
	if token.Type == yamlh.ANCHOR_TOKEN {
		anchor = token.Value
		start_mark = token.Start_mark
		end_mark = token.End_mark
		skip_token(parser)
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type == yamlh.TAG_TOKEN {
			tag_token = true
			tag_handle = token.Value
			tag_suffix = token.Suffix
			tag_mark = token.Start_mark
			end_mark = token.End_mark
			skip_token(parser)
			token, err = peek_token(parser)
			if err != nil {
				return nil, err
			}
		}
	} else if token.Type == yamlh.TAG_TOKEN {
		tag_token = true
		tag_handle = token.Value
		tag_suffix = token.Suffix
		start_mark = token.Start_mark
		tag_mark = token.Start_mark
		end_mark = token.End_mark
		skip_token(parser)
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type == yamlh.ANCHOR_TOKEN {
			anchor = token.Value
			end_mark = token.End_mark
			skip_token(parser)
			token, err = peek_token(parser)
			if err != nil {
				return nil, err
			}
		}
	}

	var tag []byte
	if tag_token {
		if len(tag_handle) == 0 {
			tag = tag_suffix
			tag_suffix = nil
		} else {
			for i := range parser.Tag_directives {
				if bytes.Equal(parser.Tag_directives[i].Handle, tag_handle) {
					tag = append([]byte(nil), parser.Tag_directives[i].Prefix...)
					tag = append(tag, tag_suffix...)
					break
				}
			}
			if len(tag) == 0 {
				return nil, buildParserError(yamlh.PARSER_ERROR, "found undefined tag handle", tag_mark.Line, start_mark.Line)
			}
		}
	}

	implicit := len(tag) == 0
	if indentless_sequence && token.Type == yamlh.BLOCK_ENTRY_TOKEN {
		end_mark = token.End_mark
		parser.State = PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE
		event = yamlh.Event{
			Type:       yamlh.SEQUENCE_START_EVENT,
			Start_mark: start_mark,
			End_mark:   end_mark,
			Anchor:     anchor,
			Tag:        tag,
			Implicit:   implicit,
			Style:      yamlh.YamlStyle(yamlh.BLOCK_SEQUENCE_STYLE),
		}
		return &event, nil
	}
	if token.Type == yamlh.SCALAR_TOKEN {
		var plain_implicit, quoted_implicit bool
		end_mark = token.End_mark
		if (len(tag) == 0 && token.Style == yamlh.PLAIN_SCALAR_STYLE) || (len(tag) == 1 && tag[0] == '!') {
			plain_implicit = true
		} else if len(tag) == 0 {
			quoted_implicit = true
		}
		parser.State = parser.States[len(parser.States)-1]
		parser.States = parser.States[:len(parser.States)-1]

		event = yamlh.Event{
			Type:            yamlh.SCALAR_EVENT,
			Start_mark:      start_mark,
			End_mark:        end_mark,
			Anchor:          anchor,
			Tag:             tag,
			Value:           token.Value,
			Implicit:        plain_implicit,
			Quoted_implicit: quoted_implicit,
			Style:           yamlh.YamlStyle(token.Style),
		}
		yaml_parser_set_event_comments(parser, &event)
		skip_token(parser)
		return &event, nil
	}
	if token.Type == yamlh.FLOW_SEQUENCE_START_TOKEN {
		// [Go] Some of the events below can be merged as they differ only on style.
		end_mark = token.End_mark
		parser.State = PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE
		event = yamlh.Event{
			Type:       yamlh.SEQUENCE_START_EVENT,
			Start_mark: start_mark,
			End_mark:   end_mark,
			Anchor:     anchor,
			Tag:        tag,
			Implicit:   implicit,
			Style:      yamlh.YamlStyle(yamlh.FLOW_SEQUENCE_STYLE),
		}
		yaml_parser_set_event_comments(parser, &event)
		return &event, nil
	}
	if token.Type == yamlh.FLOW_MAPPING_START_TOKEN {
		end_mark = token.End_mark
		parser.State = PARSE_FLOW_MAPPING_FIRST_KEY_STATE
		event = yamlh.Event{
			Type:       yamlh.MAPPING_START_EVENT,
			Start_mark: start_mark,
			End_mark:   end_mark,
			Anchor:     anchor,
			Tag:        tag,
			Implicit:   implicit,
			Style:      yamlh.YamlStyle(yamlh.FLOW_MAPPING_STYLE),
		}
		yaml_parser_set_event_comments(parser, &event)
		return &event, nil
	}
	if block && token.Type == yamlh.BLOCK_SEQUENCE_START_TOKEN {
		end_mark = token.End_mark
		parser.State = PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE
		event = yamlh.Event{
			Type:       yamlh.SEQUENCE_START_EVENT,
			Start_mark: start_mark,
			End_mark:   end_mark,
			Anchor:     anchor,
			Tag:        tag,
			Implicit:   implicit,
			Style:      yamlh.YamlStyle(yamlh.BLOCK_SEQUENCE_STYLE),
		}
		if parser.Stem_comment != nil {
			event.Head_comment = parser.Stem_comment
			parser.Stem_comment = nil
		}
		return &event, nil
	}
	if block && token.Type == yamlh.BLOCK_MAPPING_START_TOKEN {
		end_mark = token.End_mark
		parser.State = PARSE_BLOCK_MAPPING_FIRST_KEY_STATE
		event = yamlh.Event{
			Type:       yamlh.MAPPING_START_EVENT,
			Start_mark: start_mark,
			End_mark:   end_mark,
			Anchor:     anchor,
			Tag:        tag,
			Implicit:   implicit,
			Style:      yamlh.YamlStyle(yamlh.BLOCK_MAPPING_STYLE),
		}
		if parser.Stem_comment != nil {
			event.Head_comment = parser.Stem_comment
			parser.Stem_comment = nil
		}
		return &event, nil
	}
	if len(anchor) > 0 || len(tag) > 0 {
		parser.State = parser.States[len(parser.States)-1]
		parser.States = parser.States[:len(parser.States)-1]

		event = yamlh.Event{
			Type:            yamlh.SCALAR_EVENT,
			Start_mark:      start_mark,
			End_mark:        end_mark,
			Anchor:          anchor,
			Tag:             tag,
			Implicit:        implicit,
			Quoted_implicit: false,
			Style:           yamlh.YamlStyle(yamlh.PLAIN_SCALAR_STYLE),
		}
		return &event, nil
	}

	return nil, buildParserError(yamlh.PARSER_ERROR, "did not find expected node content", token.Start_mark.Line, start_mark.Line)
}

// Parse the productions:
// block_sequence ::= BLOCK-SEQUENCE-START (BLOCK-ENTRY block_node?)* BLOCK-END
//
//	********************  *********** *             *********
func yaml_parser_parse_block_sequence_entry(parser *YamlParser, first bool) (*yamlh.Event, error) {
	if first {
		token, err := peek_token(parser)
		if err != nil {
			return nil, err
		}
		parser.Marks = append(parser.Marks, token.Start_mark)
		skip_token(parser)
	}

	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}

	if token.Type == yamlh.BLOCK_ENTRY_TOKEN {
		mark := token.End_mark
		prior_head_len := len(parser.Head_comment)
		skip_token(parser)
		err = yaml_parser_split_stem_comment(parser, prior_head_len)
		if err != nil {
			return nil, err
		}
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type != yamlh.BLOCK_ENTRY_TOKEN && token.Type != yamlh.BLOCK_END_TOKEN {
			parser.States = append(parser.States, PARSE_BLOCK_SEQUENCE_ENTRY_STATE)
			return yaml_parser_parse_node(parser, true, false)
		}
		parser.State = PARSE_BLOCK_SEQUENCE_ENTRY_STATE
		return yaml_parser_process_empty_scalar(mark), nil
	}
	if token.Type == yamlh.BLOCK_END_TOKEN {
		parser.State = parser.States[len(parser.States)-1]
		parser.States = parser.States[:len(parser.States)-1]
		parser.Marks = parser.Marks[:len(parser.Marks)-1]

		event := yamlh.Event{
			Type:       yamlh.SEQUENCE_END_EVENT,
			Start_mark: token.Start_mark,
			End_mark:   token.End_mark,
		}

		skip_token(parser)
		return &event, nil
	}

	context_mark := parser.Marks[len(parser.Marks)-1]
	parser.Marks = parser.Marks[:len(parser.Marks)-1]
	return nil, buildParserError(yamlh.PARSER_ERROR, "did not find expected '-' indicator", token.Start_mark.Line, context_mark.Line)
}

// Parse the productions:
// indentless_sequence  ::= (BLOCK-ENTRY block_node?)+
//
//	*********** *
func yaml_parser_parse_indentless_sequence_entry(parser *YamlParser) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}

	if token.Type == yamlh.BLOCK_ENTRY_TOKEN {
		mark := token.End_mark
		prior_head_len := len(parser.Head_comment)
		skip_token(parser)
		err = yaml_parser_split_stem_comment(parser, prior_head_len)
		if err != nil {
			return nil, err
		}
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type != yamlh.BLOCK_ENTRY_TOKEN &&
			token.Type != yamlh.KEY_TOKEN &&
			token.Type != yamlh.VALUE_TOKEN &&
			token.Type != yamlh.BLOCK_END_TOKEN {
			parser.States = append(parser.States, PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE)
			return yaml_parser_parse_node(parser, true, false)
		}
		parser.State = PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE
		return yaml_parser_process_empty_scalar(mark), nil
	}
	parser.State = parser.States[len(parser.States)-1]
	parser.States = parser.States[:len(parser.States)-1]

	return &yamlh.Event{
		Type:       yamlh.SEQUENCE_END_EVENT,
		Start_mark: token.Start_mark,
		End_mark:   token.Start_mark, // [Go] Shouldn't this be token.end_mark?
	}, nil
}

// Split stem comment from head comment.
//
// When a sequence or map is found under a sequence entry, the former head comment
// is assigned to the underlying sequence or map as a whole, not the individual
// sequence or map entry as would be expected otherwise. To handle this case the
// previous head comment is moved aside as the stem comment.
func yaml_parser_split_stem_comment(parser *YamlParser, stem_len int) error {
	if stem_len == 0 {
		return nil
	}

	token, err := peek_token(parser)
	if err != nil {
		return err
	}
	if token.Type != yamlh.BLOCK_SEQUENCE_START_TOKEN && token.Type != yamlh.BLOCK_MAPPING_START_TOKEN {
		return nil
	}

	parser.Stem_comment = parser.Head_comment[:stem_len]
	if len(parser.Head_comment) == stem_len {
		parser.Head_comment = nil
	} else {
		// Copy suffix to prevent very strange bugs if someone ever appends
		// further bytes to the prefix in the stem_comment slice above.
		parser.Head_comment = append([]byte(nil), parser.Head_comment[stem_len+1:]...)
	}
	return nil
}

// Parse the productions:
// block_mapping        ::= BLOCK-MAPPING_START
//
//	*******************
//	((KEY block_node_or_indentless_sequence?)?
//	  *** *
//	(VALUE block_node_or_indentless_sequence?)?)*
//
//	BLOCK-END
//	*********
func yaml_parser_parse_block_mapping_key(parser *YamlParser, first bool) (*yamlh.Event, error) {
	if first {
		token, err := peek_token(parser)
		if err != nil {
			return nil, err
		}
		parser.Marks = append(parser.Marks, token.Start_mark)
		skip_token(parser)
	}

	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}

	// [Go] A tail comment was left from the prior mapping value processed. Emit an event
	//      as it needs to be processed with that value and not the following key.
	if len(parser.Tail_comment) > 0 {
		parser.Tail_comment = nil
		return &yamlh.Event{
			Type:         yamlh.TAIL_COMMENT_EVENT,
			Start_mark:   token.Start_mark,
			End_mark:     token.End_mark,
			Foot_comment: parser.Tail_comment,
		}, nil
	}

	if token.Type == yamlh.KEY_TOKEN {
		mark := token.End_mark
		skip_token(parser)
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type != yamlh.KEY_TOKEN &&
			token.Type != yamlh.VALUE_TOKEN &&
			token.Type != yamlh.BLOCK_END_TOKEN {
			parser.States = append(parser.States, PARSE_BLOCK_MAPPING_VALUE_STATE)
			return yaml_parser_parse_node(parser, true, true)
		}
		parser.State = PARSE_BLOCK_MAPPING_VALUE_STATE
		return yaml_parser_process_empty_scalar(mark), nil
	}
	if token.Type == yamlh.BLOCK_END_TOKEN {
		parser.State = parser.States[len(parser.States)-1]
		parser.States = parser.States[:len(parser.States)-1]
		parser.Marks = parser.Marks[:len(parser.Marks)-1]
		event := yamlh.Event{
			Type:       yamlh.MAPPING_END_EVENT,
			Start_mark: token.Start_mark,
			End_mark:   token.End_mark,
		}
		yaml_parser_set_event_comments(parser, &event)
		skip_token(parser)
		return &event, nil
	}

	context_mark := parser.Marks[len(parser.Marks)-1]
	parser.Marks = parser.Marks[:len(parser.Marks)-1]
	return nil, buildParserError(yamlh.PARSER_ERROR, "did not find expected key", token.Start_mark.Line, context_mark.Line)
}

// Parse the productions:
// block_mapping        ::= BLOCK-MAPPING_START
//
//	((KEY block_node_or_indentless_sequence?)?
//
//	(VALUE block_node_or_indentless_sequence?)?)*
//	 ***** *
//	BLOCK-END
func yaml_parser_parse_block_mapping_value(parser *YamlParser) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}
	if token.Type == yamlh.VALUE_TOKEN {
		mark := token.End_mark
		skip_token(parser)
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type != yamlh.KEY_TOKEN &&
			token.Type != yamlh.VALUE_TOKEN &&
			token.Type != yamlh.BLOCK_END_TOKEN {
			parser.States = append(parser.States, PARSE_BLOCK_MAPPING_KEY_STATE)
			return yaml_parser_parse_node(parser, true, true)
		}
		parser.State = PARSE_BLOCK_MAPPING_KEY_STATE
		return yaml_parser_process_empty_scalar(mark), nil
	}
	parser.State = PARSE_BLOCK_MAPPING_KEY_STATE
	return yaml_parser_process_empty_scalar(token.Start_mark), nil
}

// Parse the productions:
// flow_sequence        ::= FLOW-SEQUENCE-START
//
//	*******************
//	(flow_sequence_entry FLOW-ENTRY)*
//	 *                   **********
//	flow_sequence_entry?
//	*
//	FLOW-SEQUENCE-END
//	*****************
//
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//
//	*
func yaml_parser_parse_flow_sequence_entry(parser *YamlParser, first bool) (*yamlh.Event, error) {
	if first {
		token, err := peek_token(parser)
		if err != nil {
			return nil, err
		}
		parser.Marks = append(parser.Marks, token.Start_mark)
		skip_token(parser)
	}
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}
	if token.Type != yamlh.FLOW_SEQUENCE_END_TOKEN {
		if !first {
			if token.Type == yamlh.FLOW_ENTRY_TOKEN {
				skip_token(parser)
				token, err = peek_token(parser)
				if err != nil {
					return nil, err
				}
			} else {
				context_mark := parser.Marks[len(parser.Marks)-1]
				parser.Marks = parser.Marks[:len(parser.Marks)-1]
				return nil, buildParserError(yamlh.PARSER_ERROR, "did not find expected ',' or ']'", token.Start_mark.Line, context_mark.Line)
			}
		}

		if token.Type == yamlh.KEY_TOKEN {
			parser.State = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE
			event := yamlh.Event{
				Type:       yamlh.MAPPING_START_EVENT,
				Start_mark: token.Start_mark,
				End_mark:   token.End_mark,
				Implicit:   true,
				Style:      yamlh.YamlStyle(yamlh.FLOW_MAPPING_STYLE),
			}
			skip_token(parser)
			return &event, nil
		}
		if token.Type != yamlh.FLOW_SEQUENCE_END_TOKEN {
			parser.States = append(parser.States, PARSE_FLOW_SEQUENCE_ENTRY_STATE)
			return yaml_parser_parse_node(parser, false, false)
		}
	}

	parser.State = parser.States[len(parser.States)-1]
	parser.States = parser.States[:len(parser.States)-1]
	parser.Marks = parser.Marks[:len(parser.Marks)-1]

	event := yamlh.Event{
		Type:       yamlh.SEQUENCE_END_EVENT,
		Start_mark: token.Start_mark,
		End_mark:   token.End_mark,
	}
	yaml_parser_set_event_comments(parser, &event)

	skip_token(parser)
	return &event, nil
}

// Parse the productions:
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//
//	*** *
func yaml_parser_parse_flow_sequence_entry_mapping_key(parser *YamlParser) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}
	if token.Type != yamlh.VALUE_TOKEN &&
		token.Type != yamlh.FLOW_ENTRY_TOKEN &&
		token.Type != yamlh.FLOW_SEQUENCE_END_TOKEN {
		parser.States = append(parser.States, PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE)
		return yaml_parser_parse_node(parser, false, false)
	}
	mark := token.End_mark
	skip_token(parser)
	parser.State = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE
	return yaml_parser_process_empty_scalar(mark), nil
}

// Parse the productions:
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//
//	***** *
func yaml_parser_parse_flow_sequence_entry_mapping_value(parser *YamlParser) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}
	if token.Type == yamlh.VALUE_TOKEN {
		skip_token(parser)
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type != yamlh.FLOW_ENTRY_TOKEN && token.Type != yamlh.FLOW_SEQUENCE_END_TOKEN {
			parser.States = append(parser.States, PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE)
			return yaml_parser_parse_node(parser, false, false)
		}
	}
	parser.State = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE
	return yaml_parser_process_empty_scalar(token.Start_mark), nil
}

// Parse the productions:
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//
//	*
func yaml_parser_parse_flow_sequence_entry_mapping_end(parser *YamlParser) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}
	parser.State = PARSE_FLOW_SEQUENCE_ENTRY_STATE
	event := yamlh.Event{
		Type:       yamlh.MAPPING_END_EVENT,
		Start_mark: token.Start_mark,
		End_mark:   token.Start_mark, // [Go] Shouldn't this be end_mark?
	}
	return &event, nil
}

// Parse the productions:
// flow_mapping         ::= FLOW-MAPPING-START
//
//	******************
//	(flow_mapping_entry FLOW-ENTRY)*
//	 *                  **********
//	flow_mapping_entry?
//	******************
//	FLOW-MAPPING-END
//	****************
//
// flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//   - *** *
func yaml_parser_parse_flow_mapping_key(parser *YamlParser, first bool) (*yamlh.Event, error) {
	if first {
		token, err := peek_token(parser)
		if err != nil {
			return nil, err
		}
		parser.Marks = append(parser.Marks, token.Start_mark)
		skip_token(parser)
	}

	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}

	if token.Type != yamlh.FLOW_MAPPING_END_TOKEN {
		if !first {
			if token.Type == yamlh.FLOW_ENTRY_TOKEN {
				skip_token(parser)
				token, err = peek_token(parser)
				if err != nil {
					return nil, err
				}
			} else {
				context_mark := parser.Marks[len(parser.Marks)-1]
				parser.Marks = parser.Marks[:len(parser.Marks)-1]
				return nil, buildParserError(yamlh.PARSER_ERROR, "did not find expected ',' or '}'", token.Start_mark.Line, context_mark.Line)
			}
		}

		if token.Type == yamlh.KEY_TOKEN {
			skip_token(parser)
			token, err = peek_token(parser)
			if err != nil {
				return nil, err
			}
			if token.Type != yamlh.VALUE_TOKEN &&
				token.Type != yamlh.FLOW_ENTRY_TOKEN &&
				token.Type != yamlh.FLOW_MAPPING_END_TOKEN {
				parser.States = append(parser.States, PARSE_FLOW_MAPPING_VALUE_STATE)
				return yaml_parser_parse_node(parser, false, false)
			}
			parser.State = PARSE_FLOW_MAPPING_VALUE_STATE
			return yaml_parser_process_empty_scalar(token.Start_mark), nil
		}
		if token.Type != yamlh.FLOW_MAPPING_END_TOKEN {
			parser.States = append(parser.States, PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE)
			return yaml_parser_parse_node(parser, false, false)
		}
	}

	parser.State = parser.States[len(parser.States)-1]
	parser.States = parser.States[:len(parser.States)-1]
	parser.Marks = parser.Marks[:len(parser.Marks)-1]
	event := yamlh.Event{
		Type:       yamlh.MAPPING_END_EVENT,
		Start_mark: token.Start_mark,
		End_mark:   token.End_mark,
	}
	yaml_parser_set_event_comments(parser, &event)
	skip_token(parser)
	return &event, nil
}

// Parse the productions:
// flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//   - ***** *
func yaml_parser_parse_flow_mapping_value(parser *YamlParser, empty bool) (*yamlh.Event, error) {
	token, err := peek_token(parser)
	if err != nil {
		return nil, err
	}
	if empty {
		parser.State = PARSE_FLOW_MAPPING_KEY_STATE
		return yaml_parser_process_empty_scalar(token.Start_mark), nil
	}
	if token.Type == yamlh.VALUE_TOKEN {
		skip_token(parser)
		token, err = peek_token(parser)
		if err != nil {
			return nil, err
		}
		if token.Type != yamlh.FLOW_ENTRY_TOKEN && token.Type != yamlh.FLOW_MAPPING_END_TOKEN {
			parser.States = append(parser.States, PARSE_FLOW_MAPPING_KEY_STATE)
			return yaml_parser_parse_node(parser, false, false)
		}
	}
	parser.State = PARSE_FLOW_MAPPING_KEY_STATE
	return yaml_parser_process_empty_scalar(token.Start_mark), nil
}

// Generate an empty scalar event.
func yaml_parser_process_empty_scalar(mark yamlh.Position) *yamlh.Event {
	return &yamlh.Event{
		Type:       yamlh.SCALAR_EVENT,
		Start_mark: mark,
		End_mark:   mark,
		Value:      nil, // Empty
		Implicit:   true,
		Style:      yamlh.YamlStyle(yamlh.PLAIN_SCALAR_STYLE),
	}
}

// Parse directives.
func yaml_parser_process_directives(parser *YamlParser,
	version_directive_ref **yamlh.VersionDirective,
	tag_directives_ref *[]yamlh.TagDirective) error {

	var version_directive *yamlh.VersionDirective
	var tag_directives []yamlh.TagDirective

	token, err := peek_token(parser)
	if err != nil {
		return err
	}

	for token.Type == yamlh.VERSION_DIRECTIVE_TOKEN || token.Type == yamlh.TAG_DIRECTIVE_TOKEN {
		if token.Type == yamlh.VERSION_DIRECTIVE_TOKEN {
			if version_directive != nil {
				return buildParserError(yamlh.PARSER_ERROR, "found duplicate %YAML directive", token.Start_mark.Line, 0)
			}
			if token.Major != 1 || token.Minor != 1 {
				return buildParserError(yamlh.PARSER_ERROR, "found incompatible YAML document", token.Start_mark.Line, 0)
			}
			version_directive = &yamlh.VersionDirective{
				Major: token.Major,
				Minor: token.Minor,
			}
		} else if token.Type == yamlh.TAG_DIRECTIVE_TOKEN {
			value := yamlh.TagDirective{
				Handle: token.Value,
				Prefix: token.Prefix,
			}
			err = yaml_parser_append_tag_directive(parser, value, false, token.Start_mark)
			if err != nil {
				return err
			}
			tag_directives = append(tag_directives, value)
		}

		skip_token(parser)
		token, err = peek_token(parser)
		if err != nil {
			return err
		}
	}

	for i := range common.DefaultTagDirectives {
		err = yaml_parser_append_tag_directive(parser, common.DefaultTagDirectives[i], true, token.Start_mark)
		if err != nil {
			return err
		}
	}

	if version_directive_ref != nil {
		*version_directive_ref = version_directive
	}
	if tag_directives_ref != nil {
		*tag_directives_ref = tag_directives
	}
	return nil
}

// Append a tag directive to the directives stack.
func yaml_parser_append_tag_directive(parser *YamlParser, value yamlh.TagDirective, allow_duplicates bool, mark yamlh.Position) error {
	for i := range parser.Tag_directives {
		if bytes.Equal(value.Handle, parser.Tag_directives[i].Handle) {
			if allow_duplicates {
				return nil
			}
			return buildParserError(yamlh.PARSER_ERROR, "found duplicate %TAG directive", mark.Line, 0)
		}
	}

	// [Go] I suspect the copy is unnecessary. This was likely done
	// because there was no way to track ownership of the data.
	value_copy := yamlh.TagDirective{
		Handle: make([]byte, len(value.Handle)),
		Prefix: make([]byte, len(value.Prefix)),
	}
	copy(value_copy.Handle, value.Handle)
	copy(value_copy.Prefix, value.Prefix)
	parser.Tag_directives = append(parser.Tag_directives, value_copy)
	return nil
}
