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
	"io"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type encoder struct {
	emitter  yaml_emitter_t
	event    yaml_event_t
	out      []byte
	flow     bool
	indent   int
	doneInit bool
}

func newEncoder() *encoder {
	e := &encoder{}
	yaml_emitter_initialize(&e.emitter)
	yaml_emitter_set_output_string(&e.emitter, &e.out)
	yaml_emitter_set_unicode(&e.emitter, true)
	return e
}

func newEncoderWithWriter(w io.Writer) *encoder {
	e := &encoder{}
	yaml_emitter_initialize(&e.emitter)
	yaml_emitter_set_output_writer(&e.emitter, w)
	yaml_emitter_set_unicode(&e.emitter, true)
	return e
}

func (e *encoder) init() error {
	if e.doneInit {
		return nil
	}
	if e.indent == 0 {
		e.indent = 4
	}
	e.emitter.best_indent = e.indent
	yaml_stream_start_event_initialize(&e.event, yaml_UTF8_ENCODING)
	err := e.emit()
	if err != nil {
		return err
	}
	e.doneInit = true
	return nil
}

func (e *encoder) finish() error {
	e.emitter.open_ended = false
	yaml_stream_end_event_initialize(&e.event)
	return e.emit()
}

func (e *encoder) destroy() {
	yaml_emitter_delete(&e.emitter)
}

func (e *encoder) emit() error {
	// This will internally delete the e.event value.
	ok := yaml_emitter_emit(&e.emitter, &e.event)
	if !ok {
		msg := "unknown problem generating YAML content"
		if e.emitter.problem != "" {
			msg = e.emitter.problem
		}
		return fmt.Errorf("yaml: " + msg)
	}
	return nil
}

func (e *encoder) marshalDoc(tag string, in reflect.Value) error {
	err := e.init()
	if err != nil {
		return err
	}
	var node *Node
	if in.IsValid() {
		node, _ = in.Interface().(*Node)
	}
	if node != nil && node.Kind == DocumentNode {
		return e.nodev(in)
	}

	yaml_document_start_event_initialize(&e.event, nil, nil, true)
	err = e.emit()
	if err != nil {
		return err
	}
	err = e.marshal(tag, in)
	if err != nil {
		return err
	}
	yaml_document_end_event_initialize(&e.event, true)
	return e.emit()
}

func (e *encoder) marshal(tag string, in reflect.Value) error {
	tag = shortTag(tag)
	if !in.IsValid() || in.Kind() == reflect.Ptr && in.IsNil() {
		return e.nilv()
	}
	iface := in.Interface()
	switch value := iface.(type) {
	case *Node:
		return e.nodev(in)
	case Node:
		if !in.CanAddr() {
			var n = reflect.New(in.Type()).Elem()
			n.Set(in)
			in = n
		}
		return e.nodev(in.Addr())
	case time.Time:
		return e.timev(tag, in)
	case *time.Time:
		return e.timev(tag, in.Elem())
	case time.Duration:
		return e.stringv(tag, reflect.ValueOf(value.String()))
	case Marshaler:
		v, err := value.MarshalYAML()
		if err != nil {
			return err
		}
		if v == nil {
			return e.nilv()
		}
		return e.marshal(tag, reflect.ValueOf(v))
	case encoding.TextMarshaler:
		text, err := value.MarshalText()
		if err != nil {
			return err
		}
		in = reflect.ValueOf(string(text))
	case nil:
		return e.nilv()
	}
	switch in.Kind() {
	case reflect.Interface:
		return e.marshal(tag, in.Elem())
	case reflect.Map:
		return e.mapv(tag, in)
	case reflect.Ptr:
		return e.marshal(tag, in.Elem())
	case reflect.Struct:
		return e.structv(tag, in)
	case reflect.Slice, reflect.Array:
		return e.slicev(tag, in)
	case reflect.String:
		return e.stringv(tag, in)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return e.intv(tag, in)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return e.uintv(tag, in)
	case reflect.Float32, reflect.Float64:
		return e.floatv(tag, in)
	case reflect.Bool:
		return e.boolv(tag, in)
	default:
		panic("cannot marshal type: " + in.Type().String())
	}
	return nil
}

func (e *encoder) mapv(tag string, in reflect.Value) error {
	return e.mappingv(tag, func() error {
		keys := keyList(in.MapKeys())
		sort.Sort(keys)
		for _, k := range keys {
			err := e.marshal("", k)
			if err != nil {
				return err
			}
			err = e.marshal("", in.MapIndex(k))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *encoder) fieldByIndex(v reflect.Value, index []int) (field reflect.Value) {
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

func (e *encoder) structv(tag string, in reflect.Value) error {
	sinfo, err := getStructInfo(in.Type())
	if err != nil {
		panic(err)
	}
	return e.mappingv(tag, func() error {
		for _, info := range sinfo.FieldsList {
			var value reflect.Value
			if info.Inline == nil {
				value = in.Field(info.Num)
			} else {
				value = e.fieldByIndex(in, info.Inline)
				if !value.IsValid() {
					continue
				}
			}
			if info.OmitEmpty && isZero(value) {
				continue
			}
			err = e.marshal("", reflect.ValueOf(info.Key))
			if err != nil {
				return err
			}
			e.flow = info.Flow
			err = e.marshal("", value)
			if err != nil {
				return err
			}
		}
		if sinfo.InlineMap >= 0 {
			m := in.Field(sinfo.InlineMap)
			if m.Len() > 0 {
				e.flow = false
				keys := keyList(m.MapKeys())
				sort.Sort(keys)
				for _, k := range keys {
					if _, found := sinfo.FieldsMap[k.String()]; found {
						panic(fmt.Sprintf("cannot have key %q in inlined map: conflicts with struct field", k.String()))
					}
					err = e.marshal("", k)
					if err != nil {
						return err
					}
					e.flow = false
					err = e.marshal("", m.MapIndex(k))
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (e *encoder) mappingv(tag string, f func() error) error {
	implicit := tag == ""
	style := yaml_BLOCK_MAPPING_STYLE
	if e.flow {
		e.flow = false
		style = yaml_FLOW_MAPPING_STYLE
	}
	yaml_mapping_start_event_initialize(&e.event, nil, []byte(tag), implicit, style)
	err := e.emit()
	if err != nil {
		return err
	}
	err = f()
	if err != nil {
		return err
	}
	yaml_mapping_end_event_initialize(&e.event)
	err = e.emit()
	if err != nil {
		return err
	}
	return nil
}

func (e *encoder) slicev(tag string, in reflect.Value) error {
	implicit := tag == ""
	style := yaml_BLOCK_SEQUENCE_STYLE
	if e.flow {
		e.flow = false
		style = yaml_FLOW_SEQUENCE_STYLE
	}
	yaml_sequence_start_event_initialize(&e.event, nil, []byte(tag), implicit, style)
	err := e.emit()
	if err != nil {
		return err
	}
	n := in.Len()
	for i := 0; i < n; i++ {
		err = e.marshal("", in.Index(i))
		if err != nil {
			return err
		}
	}
	yaml_sequence_end_event_initialize(&e.event)
	return e.emit()
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

func (e *encoder) stringv(tag string, in reflect.Value) error {
	var style yaml_scalar_style_t
	s := in.String()
	canUsePlain := true
	switch {
	case !utf8.ValidString(s):
		if tag == binaryTag {
			return fmt.Errorf("yaml: explicitly tagged !!binary data must be base64-encoded")
		}
		if tag != "" {
			return fmt.Errorf("yaml: cannot marshal invalid UTF-8 data as %s", shortTag(tag))
		}
		// It can't be encoded directly as YAML so use a binary tag
		// and encode it as base64.
		tag = binaryTag
		s = encodeBase64(s)
	case tag == "":
		// Check to see if it would resolve to a specific
		// tag when encoded unquoted. If it doesn't,
		// there's no need to quote it.
		rtag, _ := resolve("", s)
		canUsePlain = rtag == strTag && !(isBase60Float(s) || isOldBool(s))
	}
	// Note: it's possible for user code to emitPanic invalid YAML
	// if they explicitly specify a tag and a string containing
	// text that's incompatible with that tag.
	switch {
	case strings.Contains(s, "\n"):
		if e.flow {
			style = yaml_DOUBLE_QUOTED_SCALAR_STYLE
		} else {
			style = yaml_LITERAL_SCALAR_STYLE
		}
	case canUsePlain:
		style = yaml_PLAIN_SCALAR_STYLE
	default:
		style = yaml_DOUBLE_QUOTED_SCALAR_STYLE
	}
	return e.emitScalar(s, "", tag, style, nil, nil, nil, nil)
}

func (e *encoder) boolv(tag string, in reflect.Value) error {
	var s string
	if in.Bool() {
		s = "true"
	} else {
		s = "false"
	}
	return e.emitScalar(s, "", tag, yaml_PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) intv(tag string, in reflect.Value) error {
	s := strconv.FormatInt(in.Int(), 10)
	return e.emitScalar(s, "", tag, yaml_PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) uintv(tag string, in reflect.Value) error {
	s := strconv.FormatUint(in.Uint(), 10)
	return e.emitScalar(s, "", tag, yaml_PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) timev(tag string, in reflect.Value) error {
	t := in.Interface().(time.Time)
	s := t.Format(time.RFC3339Nano)
	return e.emitScalar(s, "", tag, yaml_PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) floatv(tag string, in reflect.Value) error {
	// Issue #352: When formatting, use the precision of the underlying value
	precision := 64
	if in.Kind() == reflect.Float32 {
		precision = 32
	}

	s := strconv.FormatFloat(in.Float(), 'g', -1, precision)
	switch s {
	case "+Inf":
		s = ".inf"
	case "-Inf":
		s = "-.inf"
	case "NaN":
		s = ".nan"
	}
	return e.emitScalar(s, "", tag, yaml_PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) nilv() error {
	return e.emitScalar("null", "", "", yaml_PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
}

func (e *encoder) emitScalar(value, anchor, tag string, style yaml_scalar_style_t, head, line, foot, tail []byte) error {
	// TODO Kill this function. Replace all initialize calls by their underlining Go literals.
	implicit := tag == ""
	if !implicit {
		tag = longTag(tag)
	}
	yaml_scalar_event_initialize(&e.event, []byte(anchor), []byte(tag), []byte(value), implicit, implicit, style)
	e.event.head_comment = head
	e.event.line_comment = line
	e.event.foot_comment = foot
	e.event.tail_comment = tail
	return e.emit()
}

func (e *encoder) nodev(in reflect.Value) error {
	return e.node(in.Interface().(*Node), "")
}

func (e *encoder) node(node *Node, tail string) error {
	// Zero nodes behave as nil.
	if node.Kind == 0 && node.IsZero() {
		return e.nilv()
	}

	// If the tag was not explicitly requested, and dropping it won't change the
	// implicit tag of the value, don't include it in the presentation.
	var tag = node.Tag
	var stag = shortTag(tag)
	var forceQuoting bool
	if tag != "" && node.Style&TaggedStyle == 0 {
		if node.Kind == ScalarNode {
			if stag == strTag && node.Style&(SingleQuotedStyle|DoubleQuotedStyle|LiteralStyle|FoldedStyle) != 0 {
				tag = ""
			} else {
				rtag, _ := resolve("", node.Value)
				if rtag == stag {
					tag = ""
				} else if stag == strTag {
					tag = ""
					forceQuoting = true
				}
			}
		} else {
			var rtag string
			switch node.Kind {
			case MappingNode:
				rtag = mapTag
			case SequenceNode:
				rtag = seqTag
			}
			if rtag == stag {
				tag = ""
			}
		}
	}

	switch node.Kind {
	case DocumentNode:
		yaml_document_start_event_initialize(&e.event, nil, nil, true)
		e.event.head_comment = []byte(node.HeadComment)
		err := e.emit()
		if err != nil {
			return err
		}
		for _, node := range node.Content {
			err := e.node(node, "")
			if err != nil {
				return err
			}
		}
		yaml_document_end_event_initialize(&e.event, true)
		e.event.foot_comment = []byte(node.FootComment)
		return e.emit()

	case SequenceNode:
		style := yaml_BLOCK_SEQUENCE_STYLE
		if node.Style&FlowStyle != 0 {
			style = yaml_FLOW_SEQUENCE_STYLE
		}
		yaml_sequence_start_event_initialize(&e.event, []byte(node.Anchor), []byte(longTag(tag)), tag == "", style)
		e.event.head_comment = []byte(node.HeadComment)
		err := e.emit()
		if err != nil {
			return err
		}
		for _, node := range node.Content {
			err := e.node(node, "")
			if err != nil {
				return err
			}
		}
		yaml_sequence_end_event_initialize(&e.event)
		e.event.line_comment = []byte(node.LineComment)
		e.event.foot_comment = []byte(node.FootComment)
		return e.emit()

	case MappingNode:
		style := yaml_BLOCK_MAPPING_STYLE
		if node.Style&FlowStyle != 0 {
			style = yaml_FLOW_MAPPING_STYLE
		}
		yaml_mapping_start_event_initialize(&e.event, []byte(node.Anchor), []byte(longTag(tag)), tag == "", style)
		e.event.tail_comment = []byte(tail)
		e.event.head_comment = []byte(node.HeadComment)
		err := e.emit()
		if err != nil {
			return err
		}

		// The tail logic below moves the foot comment of prior keys to the following key,
		// since the value for each key may be a nested structure and the foot needs to be
		// processed only the entirety of the value is streamed. The last tail is processed
		// with the mapping end event.
		var tail string
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			foot := k.FootComment
			if foot != "" {
				kopy := *k
				kopy.FootComment = ""
				k = &kopy
			}
			err := e.node(k, tail)
			if err != nil {
				return err
			}
			tail = foot

			v := node.Content[i+1]
			err = e.node(v, "")
			if err != nil {
				return err
			}
		}

		yaml_mapping_end_event_initialize(&e.event)
		e.event.tail_comment = []byte(tail)
		e.event.line_comment = []byte(node.LineComment)
		e.event.foot_comment = []byte(node.FootComment)
		return e.emit()

	case AliasNode:
		yaml_alias_event_initialize(&e.event, []byte(node.Value))
		e.event.head_comment = []byte(node.HeadComment)
		e.event.line_comment = []byte(node.LineComment)
		e.event.foot_comment = []byte(node.FootComment)
		return e.emit()

	case ScalarNode:
		value := node.Value
		if !utf8.ValidString(value) {
			if stag == binaryTag {
				return fmt.Errorf("yaml: explicitly tagged !!binary data must be base64-encoded")
			}
			if stag != "" {
				return fmt.Errorf("yaml: cannot marshal invalid UTF-8 data as %s", stag)
			}
			// It can't be encoded directly as YAML so use a binary tag
			// and encode it as base64.
			tag = binaryTag
			value = encodeBase64(value)
		}

		style := yaml_PLAIN_SCALAR_STYLE
		switch {
		case node.Style&DoubleQuotedStyle != 0:
			style = yaml_DOUBLE_QUOTED_SCALAR_STYLE
		case node.Style&SingleQuotedStyle != 0:
			style = yaml_SINGLE_QUOTED_SCALAR_STYLE
		case node.Style&LiteralStyle != 0:
			style = yaml_LITERAL_SCALAR_STYLE
		case node.Style&FoldedStyle != 0:
			style = yaml_FOLDED_SCALAR_STYLE
		case strings.Contains(value, "\n"):
			style = yaml_LITERAL_SCALAR_STYLE
		case forceQuoting:
			style = yaml_DOUBLE_QUOTED_SCALAR_STYLE
		}

		return e.emitScalar(value, node.Anchor, tag, style, []byte(node.HeadComment), []byte(node.LineComment), []byte(node.FootComment), []byte(tail))
	default:
		return fmt.Errorf("yaml: cannot encode node with unknown kind %d", node.Kind)
	}
}
