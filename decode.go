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
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"time"
)

// ----------------------------------------------------------------------------
// Parser, produces a node tree out of a libyaml event stream.

type parser struct {
	parser   yaml_parser_t
	event    yaml_event_t
	doc      *Node
	anchors  map[string]*Node
	doneInit bool
	textless bool
}

func newParser(b []byte) *parser {
	p := parser{}
	yaml_parser_initialize(&p.parser)
	if len(b) == 0 {
		b = []byte{'\n'}
	}
	yaml_parser_set_input_string(&p.parser, b)
	return &p
}

func newParserFromReader(r io.Reader) *parser {
	p := parser{}
	yaml_parser_initialize(&p.parser)
	yaml_parser_set_input_reader(&p.parser, r)
	return &p
}

func (p *parser) init() error {
	if p.doneInit {
		return nil
	}
	p.anchors = make(map[string]*Node)
	err := p.expect(yaml_STREAM_START_EVENT)
	if err != nil {
		return err
	}
	p.doneInit = true
	return nil
}

func (p *parser) destroy() {
	if p.event.typ != yaml_NO_EVENT {
		yaml_event_delete(&p.event)
	}
	yaml_parser_delete(&p.parser)
}

// expect consumes an event from the event stream and
// checks that it's of the expected type.
func (p *parser) expect(e yaml_event_type_t) error {
	if p.event.typ == yaml_NO_EVENT {
		if !yaml_parser_parse(&p.parser, &p.event) {
			return p.fail()
		}
	}
	if p.event.typ == yaml_STREAM_END_EVENT {
		return fmt.Errorf("yaml: attempted to go past the end of stream; corrupted value?")
	}
	if p.event.typ != e {
		p.parser.problem = fmt.Sprintf("expected %s event but got %s", e, p.event.typ)
		return p.fail()
	}
	yaml_event_delete(&p.event)
	p.event.typ = yaml_NO_EVENT
	return nil
}

// peek peeks at the next event in the event stream,
// puts the results into p.event and returns the event type.
func (p *parser) peek() (yaml_event_type_t, error) {
	if p.event.typ != yaml_NO_EVENT {
		return p.event.typ, nil
	}
	// It's curious choice from the underlying API to generally return a
	// positive result on success, but on this case return true in an error
	// scenario. This was the source of bugs in the past (issue #666).
	if !yaml_parser_parse(&p.parser, &p.event) || p.parser.error != yaml_NO_ERROR {
		return 0, p.fail()
	}
	return p.event.typ, nil
}

func (p *parser) fail() error {
	var where string
	var line int
	if p.parser.context_mark.line != 0 {
		line = p.parser.context_mark.line
		// Scanner errors don't iterate line before returning error
		if p.parser.error == yaml_SCANNER_ERROR {
			line++
		}
	} else if p.parser.problem_mark.line != 0 {
		line = p.parser.problem_mark.line
		// Scanner errors don't iterate line before returning error
		if p.parser.error == yaml_SCANNER_ERROR {
			line++
		}
	}
	if line != 0 {
		where = "line " + strconv.Itoa(line) + ": "
	}
	var msg string
	if len(p.parser.problem) > 0 {
		msg = p.parser.problem
	} else {
		msg = "unknown problem parsing YAML content"
	}
	return fmt.Errorf("yaml: %s%s", where, msg)
}

func (p *parser) anchor(n *Node, anchor []byte) {
	if anchor != nil {
		n.Anchor = string(anchor)
		p.anchors[n.Anchor] = n
	}
}

func (p *parser) parse() (*Node, error) {
	err := p.init()
	if err != nil {
		return nil, err
	}
	nextEvent, err := p.peek()
	if err != nil {
		return nil, err
	}
	switch nextEvent {
	case yaml_SCALAR_EVENT:
		return p.scalar()
	case yaml_ALIAS_EVENT:
		return p.alias()
	case yaml_MAPPING_START_EVENT:
		return p.mapping()
	case yaml_SEQUENCE_START_EVENT:
		return p.sequence()
	case yaml_DOCUMENT_START_EVENT:
		return p.document()
	case yaml_STREAM_END_EVENT:
		// Happens when attempting to decode an empty buffer.
		return nil, nil
	case yaml_TAIL_COMMENT_EVENT:
		panic("internal error: unexpected tail comment event (please report)")
	default:
		panic("internal error: attempted to parse unknown event (please report): " + p.event.typ.String())
	}
}

func (p *parser) node(kind Kind, defaultTag, tag, value string) (*Node, error) {
	var style Style
	var err error
	if tag != "" && tag != "!" {
		tag = shortTag(tag)
		style = TaggedStyle
	} else if defaultTag != "" {
		tag = defaultTag
	} else if kind == ScalarNode {
		tag, _, err = resolve("", value)
		if err != nil {
			return nil, err
		}
	}
	n := &Node{
		Kind:  kind,
		Tag:   tag,
		Value: value,
		Style: style,
	}
	if !p.textless {
		n.Line = p.event.start_mark.line + 1
		n.Column = p.event.start_mark.column + 1
		n.HeadComment = string(p.event.head_comment)
		n.LineComment = string(p.event.line_comment)
		n.FootComment = string(p.event.foot_comment)
	}
	return n, nil
}

func (p *parser) parseChild(parent *Node) (*Node, error) {
	child, err := p.parse()
	if err != nil {
		return nil, err
	}
	parent.Content = append(parent.Content, child)
	return child, nil
}

func (p *parser) document() (*Node, error) {
	n, err := p.node(DocumentNode, "", "", "")
	if err != nil {
		return nil, err
	}
	p.doc = n
	err = p.expect(yaml_DOCUMENT_START_EVENT)
	if err != nil {
		return nil, err
	}
	_, err = p.parseChild(n)
	if err != nil {
		return nil, err
	}
	nextEvent, err := p.peek()
	if err != nil {
		return nil, err
	}
	if nextEvent == yaml_DOCUMENT_END_EVENT {
		n.FootComment = string(p.event.foot_comment)
	}
	err = p.expect(yaml_DOCUMENT_END_EVENT)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (p *parser) alias() (*Node, error) {
	n, err := p.node(AliasNode, "", "", string(p.event.anchor))
	if err != nil {
		return nil, err
	}
	n.Alias = p.anchors[n.Value]
	if n.Alias == nil {
		return nil, fmt.Errorf("yaml: unknown anchor '%s' referenced", n.Value)
	}
	err = p.expect(yaml_ALIAS_EVENT)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (p *parser) scalar() (*Node, error) {
	var parsedStyle = p.event.scalar_style()
	var nodeStyle Style
	switch {
	case parsedStyle&yaml_DOUBLE_QUOTED_SCALAR_STYLE != 0:
		nodeStyle = DoubleQuotedStyle
	case parsedStyle&yaml_SINGLE_QUOTED_SCALAR_STYLE != 0:
		nodeStyle = SingleQuotedStyle
	case parsedStyle&yaml_LITERAL_SCALAR_STYLE != 0:
		nodeStyle = LiteralStyle
	case parsedStyle&yaml_FOLDED_SCALAR_STYLE != 0:
		nodeStyle = FoldedStyle
	}
	var nodeValue = string(p.event.value)
	var nodeTag = string(p.event.tag)
	var defaultTag string
	if nodeStyle == 0 {
		if nodeValue == "<<" {
			defaultTag = mergeTag
		}
	} else {
		defaultTag = strTag
	}
	n, err := p.node(ScalarNode, defaultTag, nodeTag, nodeValue)
	if err != nil {
		return nil, err
	}
	n.Style |= nodeStyle
	p.anchor(n, p.event.anchor)
	err = p.expect(yaml_SCALAR_EVENT)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (p *parser) sequence() (*Node, error) {
	n, err := p.node(SequenceNode, seqTag, string(p.event.tag), "")
	if err != nil {
		return nil, err
	}
	if p.event.sequence_style()&yaml_FLOW_SEQUENCE_STYLE != 0 {
		n.Style |= FlowStyle
	}
	p.anchor(n, p.event.anchor)
	err = p.expect(yaml_SEQUENCE_START_EVENT)
	if err != nil {
		return nil, err
	}
	for {
		nextEvent, err := p.peek()
		if err != nil {
			return nil, err
		}
		if nextEvent == yaml_SEQUENCE_END_EVENT {
			break
		}
		_, err = p.parseChild(n)
		if err != nil {
			return nil, err
		}
	}
	n.LineComment = string(p.event.line_comment)
	n.FootComment = string(p.event.foot_comment)
	err = p.expect(yaml_SEQUENCE_END_EVENT)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (p *parser) mapping() (*Node, error) {
	n, err := p.node(MappingNode, mapTag, string(p.event.tag), "")
	if err != nil {
		return nil, err
	}
	block := true
	if p.event.mapping_style()&yaml_FLOW_MAPPING_STYLE != 0 {
		block = false
		n.Style |= FlowStyle
	}
	p.anchor(n, p.event.anchor)
	err = p.expect(yaml_MAPPING_START_EVENT)
	if err != nil {
		return nil, err
	}
	for {
		nextEvent, err := p.peek()
		if err != nil {
			return nil, err
		}
		if nextEvent == yaml_MAPPING_END_EVENT {
			break
		}

		k, err := p.parseChild(n)
		if err != nil {
			return nil, err
		}
		if block && k.FootComment != "" {
			// Must be a foot comment for the prior value when being dedented.
			if len(n.Content) > 2 {
				n.Content[len(n.Content)-3].FootComment = k.FootComment
				k.FootComment = ""
			}
		}
		v, err := p.parseChild(n)
		if err != nil {
			return nil, err
		}
		if k.FootComment == "" && v.FootComment != "" {
			k.FootComment = v.FootComment
			v.FootComment = ""
		}
		nextEvent, err = p.peek()
		if err != nil {
			return nil, err
		}
		if nextEvent == yaml_TAIL_COMMENT_EVENT {
			if k.FootComment == "" {
				k.FootComment = string(p.event.foot_comment)
			}
			err = p.expect(yaml_TAIL_COMMENT_EVENT)
			if err != nil {
				return nil, err
			}
		}
	}
	n.LineComment = string(p.event.line_comment)
	n.FootComment = string(p.event.foot_comment)
	if n.Style&FlowStyle == 0 && n.FootComment != "" && len(n.Content) > 1 {
		n.Content[len(n.Content)-2].FootComment = n.FootComment
		n.FootComment = ""
	}
	err = p.expect(yaml_MAPPING_END_EVENT)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// ----------------------------------------------------------------------------
// Decoder, unmarshals a node into a provided value.

type decoder struct {
	doc        *Node
	aliases    map[*Node]bool
	typeErrors []string

	stringMapType  reflect.Type
	generalMapType reflect.Type

	knownFields bool
	uniqueKeys  bool
	decodeCount int
	aliasCount  int
	aliasDepth  int

	mergedFields map[interface{}]bool
}

var (
	nodeType       = reflect.TypeOf(Node{})
	durationType   = reflect.TypeOf(time.Duration(0))
	stringMapType  = reflect.TypeOf(map[string]interface{}{})
	generalMapType = reflect.TypeOf(map[interface{}]interface{}{})
	ifaceType      = generalMapType.Elem()
	timeType       = reflect.TypeOf(time.Time{})
	ptrTimeType    = reflect.TypeOf(&time.Time{})
)

func newDecoder() *decoder {
	d := &decoder{
		stringMapType:  stringMapType,
		generalMapType: generalMapType,
		uniqueKeys:     true,
	}
	d.aliases = make(map[*Node]bool)
	return d
}

func (d *decoder) terror(n *Node, tag string, out reflect.Value) {
	if n.Tag != "" {
		tag = n.Tag
	}
	value := n.Value
	if tag != seqTag && tag != mapTag {
		if len(value) > 10 {
			value = " `" + value[:7] + "...`"
		} else {
			value = " `" + value + "`"
		}
	}
	d.typeErrors = append(d.typeErrors, fmt.Sprintf("line %d: cannot unmarshal %s%s into %s", n.Line, shortTag(tag), value, out.Type()))
}

func (d *decoder) callUnmarshaler(n *Node, u Unmarshaler) (bool, error) {
	err := u.UnmarshalYAML(n)
	if e, ok := err.(*TypeError); ok {
		d.typeErrors = append(d.typeErrors, e.Errors...)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *decoder) callObsoleteUnmarshaler(n *Node, u obsoleteUnmarshaler) (bool, error) {
	terrlen := len(d.typeErrors)
	err := u.UnmarshalYAML(func(v interface{}) (err error) {
		_, uErr := d.unmarshal(n, reflect.ValueOf(v))
		if uErr != nil {
			return err
		}
		if len(d.typeErrors) > terrlen {
			issues := d.typeErrors[terrlen:]
			d.typeErrors = d.typeErrors[:terrlen]
			return &TypeError{issues}
		}
		return nil
	})
	if e, ok := err.(*TypeError); ok {
		d.typeErrors = append(d.typeErrors, e.Errors...)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// d.prepare initializes and dereferences pointers and calls UnmarshalYAML
// if a value is found to implement it.
// It returns the initialized and dereferenced out value, whether
// unmarshalling was already done by UnmarshalYAML, and if so whether
// its types unmarshalled appropriately.
//
// If n holds a null value, prepare returns before doing anything.
func (d *decoder) prepare(n *Node, out reflect.Value) (newout reflect.Value, unmarshaled, good bool, _ error) {
	if n.ShortTag() == nullTag {
		return out, false, false, nil
	}
	var err error
	again := true
	for again {
		again = false
		if out.Kind() == reflect.Ptr {
			if out.IsNil() {
				out.Set(reflect.New(out.Type().Elem()))
			}
			out = out.Elem()
			again = true
		}
		if out.CanAddr() {
			outi := out.Addr().Interface()
			if u, ok := outi.(Unmarshaler); ok {
				good, err = d.callUnmarshaler(n, u)
				if err != nil {
					return reflect.Value{}, false, false, err
				}
				return out, true, good, nil
			}
			if u, ok := outi.(obsoleteUnmarshaler); ok {
				good, err = d.callObsoleteUnmarshaler(n, u)
				if err != nil {
					return reflect.Value{}, false, false, err
				}
				return out, true, good, nil
			}
		}
	}
	return out, false, false, nil
}

func (d *decoder) fieldByIndex(n *Node, v reflect.Value, index []int) (field reflect.Value) {
	if n.ShortTag() == nullTag {
		return reflect.Value{}
	}
	for _, num := range index {
		for {
			if v.Kind() == reflect.Ptr {
				if v.IsNil() {
					v.Set(reflect.New(v.Type().Elem()))
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

const (
	// 400,000 decode operations is ~500kb of dense object declarations, or
	// ~5kb of dense object declarations with 10000% alias expansion
	alias_ratio_range_low = 400000

	// 4,000,000 decode operations is ~5MB of dense object declarations, or
	// ~4.5MB of dense object declarations with 10% alias expansion
	alias_ratio_range_high = 4000000

	// alias_ratio_range is the range over which we scale allowed alias ratios
	alias_ratio_range = float64(alias_ratio_range_high - alias_ratio_range_low)
)

func allowedAliasRatio(decodeCount int) float64 {
	switch {
	case decodeCount <= alias_ratio_range_low:
		// allow 99% to come from alias expansion for small-to-medium documents
		return 0.99
	case decodeCount >= alias_ratio_range_high:
		// allow 10% to come from alias expansion for very large documents
		return 0.10
	default:
		// scale smoothly from 99% down to 10% over the range.
		// this maps to 396,000 - 400,000 allowed alias-driven decodes over the range.
		// 400,000 decode operations is ~100MB of allocations in worst-case scenarios (single-item maps).
		return 0.99 - 0.89*(float64(decodeCount-alias_ratio_range_low)/alias_ratio_range)
	}
}

func (d *decoder) unmarshal(n *Node, out reflect.Value) (bool, error) {
	d.decodeCount++
	if d.aliasDepth > 0 {
		d.aliasCount++
	}
	if d.aliasCount > 100 && d.decodeCount > 1000 && float64(d.aliasCount)/float64(d.decodeCount) > allowedAliasRatio(d.decodeCount) {
		return false, fmt.Errorf("yaml: document contains excessive aliasing")
	}
	if out.Type() == nodeType {
		out.Set(reflect.ValueOf(n).Elem())
		return true, nil
	}
	switch n.Kind {
	case DocumentNode:
		return d.document(n, out)
	case AliasNode:
		return d.alias(n, out)
	}
	out, unmarshaled, good, err := d.prepare(n, out)
	if err != nil {
		return false, err
	}
	if unmarshaled {
		return good, nil
	}
	switch n.Kind {
	case ScalarNode:
		return d.scalar(n, out)
	case MappingNode:
		return d.mapping(n, out)
	case SequenceNode:
		return d.sequence(n, out)
	case 0:
		if n.IsZero() {
			return d.null(out), nil
		}
	}
	return false, fmt.Errorf("yaml: cannot decode node with unknown kind %d", n.Kind)
}

func (d *decoder) document(n *Node, out reflect.Value) (bool, error) {
	if len(n.Content) == 1 {
		d.doc = n
		return d.unmarshal(n.Content[0], out)
	}
	return false, nil
}

func (d *decoder) alias(n *Node, out reflect.Value) (bool, error) {
	if d.aliases[n] {
		// TODO this could actually be allowed in some circumstances.
		return false, fmt.Errorf("yaml: anchor '%s' value contains itself", n.Value)
	}
	d.aliases[n] = true
	d.aliasDepth++
	good, err := d.unmarshal(n.Alias, out)
	if err != nil {
		return false, err
	}
	d.aliasDepth--
	delete(d.aliases, n)
	return good, nil
}

var zeroValue reflect.Value

func resetMap(out reflect.Value) {
	for _, k := range out.MapKeys() {
		out.SetMapIndex(k, zeroValue)
	}
}

func (d *decoder) null(out reflect.Value) bool {
	if out.CanAddr() {
		switch out.Kind() {
		case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice:
			out.Set(reflect.Zero(out.Type()))
			return true
		}
	}
	return false
}

func (d *decoder) scalar(n *Node, out reflect.Value) (bool, error) {
	var tag string
	var resolved interface{}
	var err error
	if n.indicatedString() {
		tag = strTag
		resolved = n.Value
	} else {
		tag, resolved, err = resolve(n.Tag, n.Value)
		if err != nil {
			return false, err
		}
		if tag == binaryTag {
			data, err := base64.StdEncoding.DecodeString(resolved.(string))
			if err != nil {
				return false, fmt.Errorf("yaml: !!binary value contains invalid base64 data")
			}
			resolved = string(data)
		}
	}
	if resolved == nil {
		return d.null(out), nil
	}
	if resolvedv := reflect.ValueOf(resolved); out.Type() == resolvedv.Type() {
		// We've resolved to exactly the type we want, so use that.
		out.Set(resolvedv)
		return true, nil
	}
	// Perhaps we can use the value as a TextUnmarshaler to
	// set its value.
	if out.CanAddr() {
		u, ok := out.Addr().Interface().(encoding.TextUnmarshaler)
		if ok {
			var text []byte
			if tag == binaryTag {
				text = []byte(resolved.(string))
			} else {
				// We let any value be unmarshaled into TextUnmarshaler.
				// That might be more lax than we'd like, but the
				// TextUnmarshaler itself should bowl out any dubious values.
				text = []byte(n.Value)
			}
			err = u.UnmarshalText(text)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}
	switch out.Kind() {
	case reflect.String:
		if tag == binaryTag {
			out.SetString(resolved.(string))
			return true, nil
		}
		out.SetString(n.Value)
		return true, nil
	case reflect.Interface:
		out.Set(reflect.ValueOf(resolved))
		return true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// This used to work in v2, but it's very unfriendly.
		isDuration := out.Type() == durationType

		switch resolved := resolved.(type) {
		case int:
			if !isDuration && !out.OverflowInt(int64(resolved)) {
				out.SetInt(int64(resolved))
				return true, nil
			}
		case int64:
			if !isDuration && !out.OverflowInt(resolved) {
				out.SetInt(resolved)
				return true, nil
			}
		case uint64:
			if !isDuration && resolved <= math.MaxInt64 && !out.OverflowInt(int64(resolved)) {
				out.SetInt(int64(resolved))
				return true, nil
			}
		case float64:
			if !isDuration && resolved <= math.MaxInt64 && !out.OverflowInt(int64(resolved)) {
				out.SetInt(int64(resolved))
				return true, nil
			}
		case string:
			if out.Type() == durationType {
				d, err := time.ParseDuration(resolved)
				if err == nil {
					out.SetInt(int64(d))
					return true, nil
				}
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		switch resolved := resolved.(type) {
		case int:
			if resolved >= 0 && !out.OverflowUint(uint64(resolved)) {
				out.SetUint(uint64(resolved))
				return true, nil
			}
		case int64:
			if resolved >= 0 && !out.OverflowUint(uint64(resolved)) {
				out.SetUint(uint64(resolved))
				return true, nil
			}
		case uint64:
			if !out.OverflowUint(uint64(resolved)) {
				out.SetUint(uint64(resolved))
				return true, nil
			}
		case float64:
			if resolved <= math.MaxUint64 && !out.OverflowUint(uint64(resolved)) {
				out.SetUint(uint64(resolved))
				return true, nil
			}
		}
	case reflect.Bool:
		switch resolved := resolved.(type) {
		case bool:
			out.SetBool(resolved)
			return true, nil
		case string:
			// This offers some compatibility with the 1.1 spec (https://yaml.org/type/bool.html).
			// It only works if explicitly attempting to unmarshal into a typed bool value.
			switch resolved {
			case "y", "Y", "yes", "Yes", "YES", "on", "On", "ON":
				out.SetBool(true)
				return true, nil
			case "n", "N", "no", "No", "NO", "off", "Off", "OFF":
				out.SetBool(false)
				return true, nil
			}
		}
	case reflect.Float32, reflect.Float64:
		switch resolved := resolved.(type) {
		case int:
			out.SetFloat(float64(resolved))
			return true, nil
		case int64:
			out.SetFloat(float64(resolved))
			return true, nil
		case uint64:
			out.SetFloat(float64(resolved))
			return true, nil
		case float64:
			out.SetFloat(resolved)
			return true, nil
		}
	case reflect.Struct:
		if resolvedv := reflect.ValueOf(resolved); out.Type() == resolvedv.Type() {
			out.Set(resolvedv)
			return true, nil
		}
	case reflect.Ptr:
		panic("yaml internal error: please report the issue")
	}
	d.terror(n, tag, out)
	return false, nil
}

func settableValueOf(i interface{}) reflect.Value {
	v := reflect.ValueOf(i)
	sv := reflect.New(v.Type()).Elem()
	sv.Set(v)
	return sv
}

func (d *decoder) sequence(n *Node, out reflect.Value) (bool, error) {
	l := len(n.Content)

	var iface reflect.Value
	switch out.Kind() {
	case reflect.Slice:
		out.Set(reflect.MakeSlice(out.Type(), l, l))
	case reflect.Array:
		if l != out.Len() {
			return false, fmt.Errorf("yaml: invalid array: want %d elements but got %d", out.Len(), l)
		}
	case reflect.Interface:
		// No type hints. Will have to use a generic sequence.
		iface = out
		out = settableValueOf(make([]interface{}, l))
	default:
		d.terror(n, seqTag, out)
		return false, nil
	}
	et := out.Type().Elem()

	j := 0
	for i := 0; i < l; i++ {
		e := reflect.New(et).Elem()

		ok, err := d.unmarshal(n.Content[i], e)
		if err != nil {
			return false, err
		}
		if ok {
			out.Index(j).Set(e)
			j++
		}
	}
	if out.Kind() != reflect.Array {
		out.Set(out.Slice(0, j))
	}
	if iface.IsValid() {
		iface.Set(out)
	}
	return true, nil
}

func (d *decoder) mapping(n *Node, out reflect.Value) (bool, error) {
	l := len(n.Content)
	if d.uniqueKeys {
		newErr := false
		for i := 0; i < l; i += 2 {
			ni := n.Content[i]
			for j := i + 2; j < l; j += 2 {
				nj := n.Content[j]
				if ni.Kind == nj.Kind && ni.Value == nj.Value {
					d.typeErrors = append(d.typeErrors, fmt.Sprintf("line %d: mapping key %#v already defined at line %d", nj.Line, nj.Value, ni.Line))
					newErr = true
				}
			}
		}
		if newErr {
			return false, nil
		}
	}
	switch out.Kind() {
	case reflect.Struct:
		return d.mappingStruct(n, out)
	case reflect.Map:
		// okay
	case reflect.Interface:
		iface := out
		if isStringMap(n) {
			out = reflect.MakeMap(d.stringMapType)
		} else {
			out = reflect.MakeMap(d.generalMapType)
		}
		iface.Set(out)
	default:
		d.terror(n, mapTag, out)
		return false, nil
	}

	outt := out.Type()
	kt := outt.Key()
	et := outt.Elem()

	stringMapType := d.stringMapType
	generalMapType := d.generalMapType
	if outt.Elem() == ifaceType {
		if outt.Key().Kind() == reflect.String {
			d.stringMapType = outt
		} else if outt.Key() == ifaceType {
			d.generalMapType = outt
		}
	}

	mergedFields := d.mergedFields
	d.mergedFields = nil

	var mergeNode *Node

	mapIsNew := false
	if out.IsNil() {
		out.Set(reflect.MakeMap(outt))
		mapIsNew = true
	}
	for i := 0; i < l; i += 2 {
		if isMerge(n.Content[i]) {
			mergeNode = n.Content[i+1]
			continue
		}
		k := reflect.New(kt).Elem()
		ok, err := d.unmarshal(n.Content[i], k)
		if err != nil {
			return false, err
		}
		if ok {
			if mergedFields != nil {
				ki := k.Interface()
				if mergedFields[ki] {
					continue
				}
				mergedFields[ki] = true
			}
			kkind := k.Kind()
			if kkind == reflect.Interface {
				kkind = k.Elem().Kind()
			}
			if kkind == reflect.Map || kkind == reflect.Slice {
				return false, fmt.Errorf("yaml: invalid map key: %#v", k.Interface())
			}
			e := reflect.New(et).Elem()
			ok, err = d.unmarshal(n.Content[i+1], e)
			if err != nil {
				return false, err
			}
			if ok || n.Content[i+1].ShortTag() == nullTag && (mapIsNew || !out.MapIndex(k).IsValid()) {
				out.SetMapIndex(k, e)
			}
		}
	}

	d.mergedFields = mergedFields
	if mergeNode != nil {
		err := d.merge(n, mergeNode, out)
		if err != nil {
			return false, err
		}
	}

	d.stringMapType = stringMapType
	d.generalMapType = generalMapType
	return true, nil
}

func isStringMap(n *Node) bool {
	if n.Kind != MappingNode {
		return false
	}
	l := len(n.Content)
	for i := 0; i < l; i += 2 {
		short := n.Content[i].ShortTag()
		if short != strTag && short != mergeTag {
			return false
		}
	}
	return true
}

func (d *decoder) mappingStruct(n *Node, out reflect.Value) (bool, error) {
	sinfo, err := getStructInfo(out.Type())
	if err != nil {
		panic(err)
	}

	var inlineMap reflect.Value
	var elemType reflect.Type
	if sinfo.InlineMap != -1 {
		inlineMap = out.Field(sinfo.InlineMap)
		elemType = inlineMap.Type().Elem()
	}

	for _, index := range sinfo.InlineUnmarshalers {
		field := d.fieldByIndex(n, out, index)
		_, _, _, err = d.prepare(n, field)
		if err != nil {
			return false, err
		}
	}

	mergedFields := d.mergedFields
	d.mergedFields = nil
	var mergeNode *Node
	var doneFields []bool
	if d.uniqueKeys {
		doneFields = make([]bool, len(sinfo.FieldsList))
	}
	name := settableValueOf("")
	l := len(n.Content)
	for i := 0; i < l; i += 2 {
		ni := n.Content[i]
		if isMerge(ni) {
			mergeNode = n.Content[i+1]
			continue
		}
		var ok bool
		ok, err = d.unmarshal(ni, name)
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		sname := name.String()
		if mergedFields != nil {
			if mergedFields[sname] {
				continue
			}
			mergedFields[sname] = true
		}
		if info, ok := sinfo.FieldsMap[sname]; ok {
			if d.uniqueKeys {
				if doneFields[info.Id] {
					d.typeErrors = append(d.typeErrors, fmt.Sprintf("line %d: field %s already set in type %s", ni.Line, name.String(), out.Type()))
					continue
				}
				doneFields[info.Id] = true
			}
			var field reflect.Value
			if info.Inline == nil {
				field = out.Field(info.Num)
			} else {
				field = d.fieldByIndex(n, out, info.Inline)
			}
			_, err = d.unmarshal(n.Content[i+1], field)
			if err != nil {
				return false, err
			}
		} else if sinfo.InlineMap != -1 {
			if inlineMap.IsNil() {
				inlineMap.Set(reflect.MakeMap(inlineMap.Type()))
			}
			value := reflect.New(elemType).Elem()
			_, err = d.unmarshal(n.Content[i+1], value)
			if err != nil {
				return false, err
			}
			inlineMap.SetMapIndex(name, value)
		} else if d.knownFields {
			d.typeErrors = append(d.typeErrors, fmt.Sprintf("line %d: field %s not found in type %s", ni.Line, name.String(), out.Type()))
		}
	}

	d.mergedFields = mergedFields
	if mergeNode != nil {
		err = d.merge(n, mergeNode, out)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (d *decoder) merge(parent *Node, merge *Node, out reflect.Value) error {
	mergedFields := d.mergedFields
	if mergedFields == nil {
		d.mergedFields = make(map[interface{}]bool)
		for i := 0; i < len(parent.Content); i += 2 {
			k := reflect.New(ifaceType).Elem()
			ok, err := d.unmarshal(parent.Content[i], k)
			if err != nil {
				return err
			}
			if ok {
				d.mergedFields[k.Interface()] = true
			}
		}
	}

	wantMapErr := fmt.Errorf("yaml: map merge requires map or sequence of maps as the value")

	switch merge.Kind {
	case MappingNode:
		_, err := d.unmarshal(merge, out)
		if err != nil {
			return err
		}
	case AliasNode:
		if merge.Alias != nil && merge.Alias.Kind != MappingNode {
			return wantMapErr
		}
		_, err := d.unmarshal(merge, out)
		if err != nil {
			return err
		}
	case SequenceNode:
		for i := 0; i < len(merge.Content); i++ {
			ni := merge.Content[i]
			if ni.Kind == AliasNode {
				if ni.Alias != nil && ni.Alias.Kind != MappingNode {
					return wantMapErr
				}
			} else if ni.Kind != MappingNode {
				return wantMapErr
			}
			_, err := d.unmarshal(ni, out)
			if err != nil {
				return err
			}
		}
	default:
		return wantMapErr
	}

	d.mergedFields = mergedFields
	return nil
}

func isMerge(n *Node) bool {
	return n.Kind == ScalarNode && n.Value == "<<" && (n.Tag == "" || n.Tag == "!" || shortTag(n.Tag) == mergeTag)
}
