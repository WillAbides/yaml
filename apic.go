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

package yaml

import (
	"github.com/willabides/go-yaml/internal/yamlh"
)

// Create STREAM-START.
func streamStartEvent() *yamlh.Event {
	return &yamlh.Event{
		Type:     yamlh.STREAM_START_EVENT,
		Encoding: yamlh.UTF8_ENCODING,
	}
}

func streamEndEvent() *yamlh.Event {
	return &yamlh.Event{
		Type: yamlh.STREAM_END_EVENT,
	}
}

// Create DOCUMENT-START.
func documentStartEvent() *yamlh.Event {
	return &yamlh.Event{
		Type:     yamlh.DOCUMENT_START_EVENT,
		Implicit: true,
	}
}

// Create DOCUMENT-END.
func documentEndEvent() *yamlh.Event {
	return &yamlh.Event{
		Type:     yamlh.DOCUMENT_END_EVENT,
		Implicit: true,
	}
}

// Create ALIAS.
func aliasEvent(anchor []byte) *yamlh.Event {
	return &yamlh.Event{
		Type:   yamlh.ALIAS_EVENT,
		Anchor: anchor,
	}
}

// Create SCALAR.
func scalarEvent(anchor, tag, value []byte, plain_implicit, quoted_implicit bool, style yamlh.YamlScalarStyle) *yamlh.Event {
	return &yamlh.Event{
		Type:            yamlh.SCALAR_EVENT,
		Anchor:          anchor,
		Tag:             tag,
		Value:           value,
		Implicit:        plain_implicit,
		Quoted_implicit: quoted_implicit,
		Style:           yamlh.YamlStyle(style),
	}
}

// Create SEQUENCE-START.
func sequenceStartEvent(anchor, tag []byte, implicit bool, style yamlh.YamlSequenceStyle) *yamlh.Event {
	return &yamlh.Event{
		Type:     yamlh.SEQUENCE_START_EVENT,
		Anchor:   anchor,
		Tag:      tag,
		Implicit: implicit,
		Style:    yamlh.YamlStyle(style),
	}
}

// Create SEQUENCE-END.
func sequenceEndEvent() *yamlh.Event {
	return &yamlh.Event{
		Type: yamlh.SEQUENCE_END_EVENT,
	}
}

// Create MAPPING-START.
func mappingStartEvent(anchor, tag []byte, implicit bool, style yamlh.YamlMappingStyle) *yamlh.Event {
	return &yamlh.Event{
		Type:     yamlh.MAPPING_START_EVENT,
		Anchor:   anchor,
		Tag:      tag,
		Implicit: implicit,
		Style:    yamlh.YamlStyle(style),
	}
}

// Create MAPPING-END.
func mappingEndEvent() *yamlh.Event {
	return &yamlh.Event{
		Type: yamlh.MAPPING_END_EVENT,
	}
}
