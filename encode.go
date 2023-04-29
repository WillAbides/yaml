//
// Copyright (c) 2011-2019 Canonical Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yaml

import (
	"encoding"
	"fmt"
	"gopkg.in/yaml.v3/internal/emitter"
	"gopkg.in/yaml.v3/internal/resolve"
	"gopkg.in/yaml.v3/internal/sorter"
	"gopkg.in/yaml.v3/internal/yamlh"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Encoder struct {
	emitter emitter.Emitter
	flow    bool
	started bool
}

// Encode writes the YAML encoding of v to the stream.
// If multiple items are encoded to the stream, the
// second and subsequent document will be preceded
// with a "---" document separator, but the first will not.
//
// See the documentation for Marshal for details about the conversion of Go
// values to YAML.
func (e *Encoder) Encode(v interface{}) error {
	if !e.started {
		err := e.emitter.Emit(streamStartEvent(), false)
		if err != nil {
			return err
		}
		e.started = true
	}

	node, ok := v.(*Node)
	if ok && node.Kind == DocumentNode {
		return e.encodeNode(node, "")
	}

	err := e.emitter.Emit(documentStartEvent(), false)
	if err != nil {
		return err
	}
	err = e.marshal("", v)
	if err != nil {
		return err
	}
	return e.emitter.Emit(documentEndEvent(), false)
}

// SetIndent changes the used indentation used when encoding.
func (e *Encoder) SetIndent(spaces int) {
	e.emitter.SetIndent(spaces)
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		emitter: *emitter.New(w),
	}
}

// Close closes the encoder by writing any remaining data.
// It does not write a stream terminating string "...".
func (e *Encoder) Close() error {
	return e.emitter.Emit(streamEndEvent(), true)
}

func (e *Encoder) marshal(tag string, v interface{}) error {
	switch value := v.(type) {
	case *Node:
		return e.encodeNode(value, tag)
	case Node:
		return e.encodeNode(&value, tag)
	case time.Time:
		return e.encodeTime(tag, value)
	case *time.Time:
		return e.encodeTime(tag, *value)
	case time.Duration:
		return e.encodeString(tag, value.String())
	case Marshaler:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr && rv.IsNil() {
			return e.encodeNil()
		}
		y, err := value.MarshalYAML()
		if err != nil {
			return err
		}
		return e.marshal(tag, y)
	case encoding.TextMarshaler:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr && rv.IsNil() {
			return e.encodeNil()
		}
		text, err := value.MarshalText()
		if err != nil {
			return err
		}
		return e.encodeString(tag, string(text))
	case int, int8, int16, int32, int64:
		return e.encodeInt(tag, value)
	case uint, uint8, uint16, uint32, uint64:
		return e.encideUint(tag, value)
	case float64:
		return e.encodeFloat(tag, value, 64)
	case float32:
		return e.encodeFloat(tag, float64(value), 32)
	case bool:
		return e.encodeBool(tag, value)
	case string:
		return e.encodeString(tag, value)
	case nil:
		return e.encodeNil()
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return e.encodeNil()
	}
	switch rv.Kind() {
	case reflect.Interface, reflect.Ptr:
		return e.marshal(tag, rv.Elem().Interface())
	case reflect.Map:
		return e.encodeMap(tag, rv)
	case reflect.Struct:
		return e.encodeStruct(tag, rv)
	case reflect.Slice, reflect.Array:
		return e.encodeSlice(tag, rv)
	case reflect.String:
		return e.encodeString(tag, rv.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return e.encodeInt(tag, rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return e.encideUint(tag, rv.Uint())
	case reflect.Float32:
		return e.encodeFloat(tag, rv.Float(), 32)
	case reflect.Float64:
		return e.encodeFloat(tag, rv.Float(), 64)
	case reflect.Bool:
		return e.encodeBool(tag, rv.Bool())
	default:
		panic("cannot marshal type: " + rv.Type().String())
	}
	return nil
}

func (e *Encoder) encodeMap(tag string, in reflect.Value) error {
	return e.encodeMapping(tag, func() error {
		keys := sorter.KeyList(in.MapKeys())
		sort.Sort(keys)
		for _, k := range keys {
			err := e.marshal("", k.Interface())
			if err != nil {
				return err
			}
			err = e.marshal("", in.MapIndex(k).Interface())
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func fieldByIndex(v reflect.Value, index []int) (field reflect.Value) {
	for _, num := range index {
		for {
			if v.Kind() == reflect.Ptr {
				if v.IsNil() {
					return reflect.Value{}
				}
				v = v.Elem()
				continue
			}
			break
		}
		v = v.Field(num)
	}
	return v
}

func (e *Encoder) encodeStruct(tag string, in reflect.Value) error {
	sinfo, err := getStructInfo(in.Type())
	if err != nil {
		panic(err)
	}
	return e.encodeMapping(tag, func() error {
		for _, info := range sinfo.FieldsList {
			var value reflect.Value
			if info.Inline == nil {
				value = in.Field(info.Num)
			} else {
				value = fieldByIndex(in, info.Inline)
				if !value.IsValid() {
					continue
				}
			}
			if info.OmitEmpty && isZero(value) {
				continue
			}
			err = e.marshal("", reflect.ValueOf(info.Key).Interface())
			if err != nil {
				return err
			}
			e.flow = info.Flow
			err = e.marshal("", value.Interface())
			if err != nil {
				return err
			}
		}
		if sinfo.InlineMap >= 0 {
			m := in.Field(sinfo.InlineMap)
			if m.Len() > 0 {
				e.flow = false
				keys := sorter.KeyList(m.MapKeys())
				sort.Sort(keys)
				for _, k := range keys {
					if _, found := sinfo.FieldsMap[k.String()]; found {
						panic(fmt.Sprintf("cannot have key %q in inlined map: conflicts with struct field", k.String()))
					}
					err = e.marshal("", k.Interface())
					if err != nil {
						return err
					}
					e.flow = false
					err = e.marshal("", m.MapIndex(k).Interface())
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (e *Encoder) encodeMapping(tag string, f func() error) error {
	implicit := tag == ""
	style := yamlh.BLOCK_MAPPING_STYLE
	if e.flow {
		e.flow = false
		style = yamlh.FLOW_MAPPING_STYLE
	}
	event := mappingStartEvent(nil, []byte(tag), implicit, style)
	err := e.emitter.Emit(event, true)
	if err != nil {
		return err
	}
	err = f()
	if err != nil {
		return err
	}
	return e.emitter.Emit(mappingEndEvent(), false)
}

func (e *Encoder) encodeSlice(tag string, in reflect.Value) error {
	implicit := tag == ""
	style := yamlh.BLOCK_SEQUENCE_STYLE
	if e.flow {
		e.flow = false
		style = yamlh.FLOW_SEQUENCE_STYLE
	}
	err := e.emitter.Emit(sequenceStartEvent(nil, []byte(tag), implicit, style), false)
	if err != nil {
		return err
	}
	n := in.Len()
	for i := 0; i < n; i++ {
		err = e.marshal("", in.Index(i).Interface())
		if err != nil {
			return err
		}
	}
	return e.emitter.Emit(sequenceEndEvent(), false)
}

// isBase60 returns whether s is in base 60 notation as defined in YAML 1.1.
//
// The base 60 float notation in YAML 1.1 is a terrible idea and is unsupported
// in YAML 1.2 and by this package, but these should be marshalled quoted for
// the time being for compatibility with other parsers.
func isBase60Float(s string) (result bool) {
	// Fast path.
	if s == "" {
		return false
	}
	c := s[0]
	if !(c == '+' || c == '-' || c >= '0' && c <= '9') || strings.IndexByte(s, ':') < 0 {
		return false
	}
	// Do the full match.
	return base60float.MatchString(s)
}

// From http://yaml.org/type/float.html, except the regular expression there
// is bogus. In practice parsers do not enforce the "\.[0-9_]*" suffix.
var base60float = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+(?:\.[0-9_]*)?$`)

// isOldBool returns whether s is bool notation as defined in YAML 1.1.
//
// We continue to force strings that YAML 1.1 would interpret as booleans to be
// rendered as quotes strings so that the marshalled output valid for YAML 1.1
// parsing.
func isOldBool(s string) (result bool) {
	switch s {
	case "y", "Y", "yes", "Yes", "YES", "on", "On", "ON",
		"n", "N", "no", "No", "NO", "off", "Off", "OFF":
		return true
	default:
		return false
	}
}

func (e *Encoder) encodeString(tag string, s string) error {
	var style yamlh.YamlScalarStyle
	canUsePlain := true
	switch {
	case !utf8.ValidString(s):
		if tag == resolve.BinaryTag {
			return fmt.Errorf("yaml: explicitly tagged !!binary data must be base64-encoded")
		}
		if tag != "" {
			return fmt.Errorf("yaml: cannot marshal invalid UTF-8 data as %s", resolve.ShortTag(tag))
		}
		// It can't be encoded directly as YAML so use a binary tag
		// and encode it as base64.
		tag = resolve.BinaryTag
		s = resolve.EncodeBase64(s)
	case tag == "":
		// Check to see if it would resolve to a specific
		// tag when encoded unquoted. If it doesn't,
		// there's no need to quote it.
		rTag, _, err := resolve.Resolve("", s)
		if err != nil {
			return err
		}
		canUsePlain = rTag == resolve.StrTag && !(isBase60Float(s) || isOldBool(s))
	}
	// Note: it's possible for user code to emitPanic invalid YAML
	// if they explicitly specify a tag and a string containing
	// text that's incompatible with that tag.
	switch {
	case strings.Contains(s, "\n"):
		if e.flow {
			style = yamlh.DOUBLE_QUOTED_SCALAR_STYLE
		} else {
			style = yamlh.LITERAL_SCALAR_STYLE
		}
	case canUsePlain:
		style = yamlh.PLAIN_SCALAR_STYLE
	default:
		style = yamlh.DOUBLE_QUOTED_SCALAR_STYLE
	}
	return e.emitScalar(s, "", tag, style, nil, nil, nil, nil)
}

func (e *Encoder) encodeBool(tag string, v bool) error {
	var s string
	if v {
		s = "true"
	} else {
		s = "false"
	}
	return e.emitScalar(s, "", tag, yamlh.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *Encoder) encodeInt(tag string, v interface{}) error {
	var vv int64
	switch v := v.(type) {
	case int:
		vv = int64(v)
	case int8:
		vv = int64(v)
	case int16:
		vv = int64(v)
	case int32:
		vv = int64(v)
	case int64:
		vv = v
	}
	s := strconv.FormatInt(vv, 10)
	return e.emitScalar(s, "", tag, yamlh.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *Encoder) encideUint(tag string, v interface{}) error {
	var vv uint64
	switch v := v.(type) {
	case uint:
		vv = uint64(v)
	case uint8:
		vv = uint64(v)
	case uint16:
		vv = uint64(v)
	case uint32:
		vv = uint64(v)
	case uint64:
		vv = v
	}
	s := strconv.FormatUint(vv, 10)
	return e.emitScalar(s, "", tag, yamlh.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *Encoder) encodeTime(tag string, v time.Time) error {
	s := v.Format(time.RFC3339Nano)
	return e.emitScalar(s, "", tag, yamlh.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *Encoder) encodeFloat(tag string, v float64, precision int) error {
	s := strconv.FormatFloat(v, 'g', -1, precision)
	switch s {
	case "+Inf":
		s = ".inf"
	case "-Inf":
		s = "-.inf"
	case "NaN":
		s = ".nan"
	}
	return e.emitScalar(s, "", tag, yamlh.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *Encoder) encodeNil() error {
	return e.emitScalar("null", "", "", yamlh.PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *Encoder) emitScalar(value, anchor, tag string, style yamlh.YamlScalarStyle, head, line, foot, tail []byte) error {
	// TODO Kill this function. Replace all initialize calls by their underlining Go literals.
	implicit := tag == ""
	if !implicit {
		tag = resolve.LongTag(tag)
	}
	event := scalarEvent([]byte(anchor), []byte(tag), []byte(value), implicit, implicit, style)
	event.Head_comment = head
	event.Line_comment = line
	event.Foot_comment = foot
	event.Tail_comment = tail
	return e.emitter.Emit(event, false)
}

func (e *Encoder) encodeNode(node *Node, tail string) error {
	// Zero nodes behave as nil.
	if node.Kind == 0 && node.IsZero() {
		return e.encodeNil()
	}

	// If the tag was not explicitly requested, and dropping it won't change the
	// implicit tag of the value, don't include it in the presentation.
	var tag = node.Tag
	var stag = resolve.ShortTag(tag)
	var forceQuoting bool
	if tag != "" && node.Style&TaggedStyle == 0 {
		if node.Kind == ScalarNode {
			if stag == resolve.StrTag && node.Style&(SingleQuotedStyle|DoubleQuotedStyle|LiteralStyle|FoldedStyle) != 0 {
				tag = ""
			} else {
				rtag, _, err := resolve.Resolve("", node.Value)
				if err != nil {
					return err
				}
				if rtag == stag {
					tag = ""
				} else if stag == resolve.StrTag {
					tag = ""
					forceQuoting = true
				}
			}
		} else {
			var rtag string
			switch node.Kind {
			case MappingNode:
				rtag = resolve.MapTag
			case SequenceNode:
				rtag = resolve.SeqTag
			}
			if rtag == stag {
				tag = ""
			}
		}
	}

	switch node.Kind {
	case DocumentNode:
		event := documentStartEvent()
		event.Head_comment = []byte(node.HeadComment)
		err := e.emitter.Emit(event, false)
		if err != nil {
			return err
		}
		for _, n := range node.Content {
			err = e.encodeNode(n, "")
			if err != nil {
				return err
			}
		}
		event = documentEndEvent()
		event.Foot_comment = []byte(node.FootComment)
		return e.emitter.Emit(event, false)

	case SequenceNode:
		style := yamlh.BLOCK_SEQUENCE_STYLE
		if node.Style&FlowStyle != 0 {
			style = yamlh.FLOW_SEQUENCE_STYLE
		}
		event := sequenceStartEvent([]byte(node.Anchor), []byte(resolve.LongTag(tag)), tag == "", style)
		event.Head_comment = []byte(node.HeadComment)
		err := e.emitter.Emit(event, false)
		if err != nil {
			return err
		}
		for _, node := range node.Content {
			err := e.encodeNode(node, "")
			if err != nil {
				return err
			}
		}
		event = sequenceEndEvent()
		event.Line_comment = []byte(node.LineComment)
		event.Foot_comment = []byte(node.FootComment)
		return e.emitter.Emit(event, false)

	case MappingNode:
		style := yamlh.BLOCK_MAPPING_STYLE
		if node.Style&FlowStyle != 0 {
			style = yamlh.FLOW_MAPPING_STYLE
		}
		event := mappingStartEvent([]byte(node.Anchor), []byte(resolve.LongTag(tag)), tag == "", style)
		event.Tail_comment = []byte(tail)
		event.Head_comment = []byte(node.HeadComment)
		err := e.emitter.Emit(event, false)
		if err != nil {
			return err
		}

		// The tail logic below moves the foot comment of prior keys to the following key,
		// since the value for each key may be a nested structure and the foot needs to be
		// processed only the entirety of the value is streamed. The last tail is processed
		// with the mapping end event.
		var tl string
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			foot := k.FootComment
			if foot != "" {
				kopy := *k
				kopy.FootComment = ""
				k = &kopy
			}
			err = e.encodeNode(k, tl)
			if err != nil {
				return err
			}
			tl = foot

			v := node.Content[i+1]
			err = e.encodeNode(v, "")
			if err != nil {
				return err
			}
		}

		event = mappingEndEvent()
		event.Tail_comment = []byte(tl)
		event.Line_comment = []byte(node.LineComment)
		event.Foot_comment = []byte(node.FootComment)
		return e.emitter.Emit(event, false)

	case AliasNode:
		event := aliasEvent([]byte(node.Value))
		event.Head_comment = []byte(node.HeadComment)
		event.Line_comment = []byte(node.LineComment)
		event.Foot_comment = []byte(node.FootComment)
		return e.emitter.Emit(event, false)

	case ScalarNode:
		value := node.Value
		if !utf8.ValidString(value) {
			if stag == resolve.BinaryTag {
				return fmt.Errorf("yaml: explicitly tagged !!binary data must be base64-encoded")
			}
			if stag != "" {
				return fmt.Errorf("yaml: cannot marshal invalid UTF-8 data as %s", stag)
			}
			// It can't be encoded directly as YAML so use a binary tag
			// and encode it as base64.
			tag = resolve.BinaryTag
			value = resolve.EncodeBase64(value)
		}

		style := yamlh.PLAIN_SCALAR_STYLE
		switch {
		case node.Style&DoubleQuotedStyle != 0:
			style = yamlh.DOUBLE_QUOTED_SCALAR_STYLE
		case node.Style&SingleQuotedStyle != 0:
			style = yamlh.SINGLE_QUOTED_SCALAR_STYLE
		case node.Style&LiteralStyle != 0:
			style = yamlh.LITERAL_SCALAR_STYLE
		case node.Style&FoldedStyle != 0:
			style = yamlh.FOLDED_SCALAR_STYLE
		case strings.Contains(value, "\n"):
			style = yamlh.LITERAL_SCALAR_STYLE
		case forceQuoting:
			style = yamlh.DOUBLE_QUOTED_SCALAR_STYLE
		}

		return e.emitScalar(value, node.Anchor, tag, style, []byte(node.HeadComment), []byte(node.LineComment), []byte(node.FootComment), []byte(tail))
	default:
		return fmt.Errorf("yaml: cannot encode node with unknown kind %d", node.Kind)
	}
}
