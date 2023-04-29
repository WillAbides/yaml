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

package yaml_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/willabides/yaml"
)

var unmarshalIntTest = 123

var unmarshalTests = []struct {
	data  string
	value interface{}
	error string
}{
	{
		value: (*struct{})(nil),
	},
	{
		data: "{}", value: &struct{}{},
	}, {
		data:  "v: hi",
		value: map[string]string{"v": "hi"},
	}, {
		data: "v: hi", value: map[string]interface{}{"v": "hi"},
	}, {
		data:  "v: true",
		value: map[string]string{"v": "true"},
	}, {
		data:  "v: true",
		value: map[string]interface{}{"v": true},
	}, {
		data:  "v: 10",
		value: map[string]interface{}{"v": 10},
	}, {
		data:  "v: 0b10",
		value: map[string]interface{}{"v": 2},
	}, {
		data:  "v: 0xA",
		value: map[string]interface{}{"v": 10},
	}, {
		data:  "v: 4294967296",
		value: map[string]int64{"v": 4294967296},
	}, {
		data:  "v: 0.1",
		value: map[string]interface{}{"v": 0.1},
	}, {
		data:  "v: .1",
		value: map[string]interface{}{"v": 0.1},
	}, {
		data:  "v: .Inf",
		value: map[string]interface{}{"v": math.Inf(+1)},
	}, {
		data:  "v: -.Inf",
		value: map[string]interface{}{"v": math.Inf(-1)},
	}, {
		data:  "v: -10",
		value: map[string]interface{}{"v": -10},
	}, {
		data:  "v: -.1",
		value: map[string]interface{}{"v": -0.1},
	},

	// Simple values.
	{
		data:  "123",
		value: &unmarshalIntTest,
	},

	// Floats from spec
	{
		data:  "canonical: 6.8523e+5",
		value: map[string]interface{}{"canonical": 6.8523e+5},
	}, {
		data:  "expo: 685.230_15e+03",
		value: map[string]interface{}{"expo": 685.23015e+03},
	}, {
		data:  "fixed: 685_230.15",
		value: map[string]interface{}{"fixed": 685230.15},
	}, {
		data:  "neginf: -.inf",
		value: map[string]interface{}{"neginf": math.Inf(-1)},
	}, {
		data:  "fixed: 685_230.15",
		value: map[string]float64{"fixed": 685230.15},
	},
	//{"sexa: 190:20:30.15", map[string]interface{}{"sexa": 0}}, // Unsupported
	//{"notanum: .NaN", map[string]interface{}{"notanum": math.NaN()}}, // Equality of NaN fails.

	// Bools are per 1.2 spec.
	{
		data:  "canonical: true",
		value: map[string]interface{}{"canonical": true},
	}, {
		data:  "canonical: false",
		value: map[string]interface{}{"canonical": false},
	}, {
		data:  "bool: True",
		value: map[string]interface{}{"bool": true},
	}, {
		data:  "bool: False",
		value: map[string]interface{}{"bool": false},
	}, {
		data:  "bool: TRUE",
		value: map[string]interface{}{"bool": true},
	}, {
		data:  "bool: FALSE",
		value: map[string]interface{}{"bool": false},
	},
	// For backwards compatibility with 1.1, decoding old strings into typed values still works.
	{
		data:  "option: on",
		value: map[string]bool{"option": true},
	}, {
		data:  "option: y",
		value: map[string]bool{"option": true},
	}, {
		data:  "option: Off",
		value: map[string]bool{"option": false},
	}, {
		data:  "option: No",
		value: map[string]bool{"option": false},
	}, {
		data:  "option: other",
		value: map[string]bool{},
		error: "line 1: cannot unmarshal !!str `other` into bool",
	},
	// Ints from spec
	{
		data:  "canonical: 685230",
		value: map[string]interface{}{"canonical": 685230},
	}, {
		data:  "decimal: +685_230",
		value: map[string]interface{}{"decimal": 685230},
	}, {
		data:  "octal: 02472256",
		value: map[string]interface{}{"octal": 685230},
	}, {
		data:  "octal: -02472256",
		value: map[string]interface{}{"octal": -685230},
	}, {
		data:  "octal: 0o2472256",
		value: map[string]interface{}{"octal": 685230},
	}, {
		data:  "octal: -0o2472256",
		value: map[string]interface{}{"octal": -685230},
	}, {
		data:  "hexa: 0x_0A_74_AE",
		value: map[string]interface{}{"hexa": 685230},
	}, {
		data:  "bin: 0b1010_0111_0100_1010_1110",
		value: map[string]interface{}{"bin": 685230},
	}, {
		data:  "bin: -0b101010",
		value: map[string]interface{}{"bin": -42},
	}, {
		data:  "bin: -0b1000000000000000000000000000000000000000000000000000000000000000",
		value: map[string]interface{}{"bin": -9223372036854775808},
	}, {
		data:  "decimal: +685_230",
		value: map[string]int{"decimal": 685230},
	},

	//{"sexa: 190:20:30", map[string]interface{}{"sexa": 0}}, // Unsupported

	// Nulls from spec
	{
		data:  "empty:",
		value: map[string]interface{}{"empty": nil},
	}, {
		data:  "canonical: ~",
		value: map[string]interface{}{"canonical": nil},
	}, {
		data:  "english: null",
		value: map[string]interface{}{"english": nil},
	}, {
		data:  "~: null key",
		value: map[interface{}]string{nil: "null key"},
	}, {
		data:  "empty:",
		value: map[string]*bool{"empty": nil},
	},

	// Flow sequence
	{
		data:  "seq: [A,B]",
		value: map[string]interface{}{"seq": []interface{}{"A", "B"}},
	}, {
		data:  "seq: [A,B,C,]",
		value: map[string][]string{"seq": []string{"A", "B", "C"}},
	}, {
		data:  "seq: [A,1,C]",
		value: map[string][]string{"seq": []string{"A", "1", "C"}},
	}, {
		data:  "seq: [A,1,C]",
		value: map[string][]int{"seq": []int{1}},
		error: "line 1: cannot unmarshal !!str `A` into int",
	}, {
		data:  "seq: [A,1,C]",
		value: map[string]interface{}{"seq": []interface{}{"A", 1, "C"}},
	},
	// Block sequence
	{
		data:  "seq:\n - A\n - B",
		value: map[string]interface{}{"seq": []interface{}{"A", "B"}},
	}, {
		data:  "seq:\n - A\n - B\n - C",
		value: map[string][]string{"seq": []string{"A", "B", "C"}},
	}, {
		data:  "seq:\n - A\n - 1\n - C",
		value: map[string][]string{"seq": []string{"A", "1", "C"}},
	}, {
		data:  "seq:\n - A\n - 1\n - C",
		value: map[string][]int{"seq": []int{1}},
		error: "line 2: cannot unmarshal !!str `A` into int",
	}, {
		data:  "seq:\n - A\n - 1\n - C",
		value: map[string]interface{}{"seq": []interface{}{"A", 1, "C"}},
	},

	// Literal block scalar
	{
		data:  "scalar: | # Comment\n\n literal\n\n \ttext\n\n",
		value: map[string]string{"scalar": "\nliteral\n\n\ttext\n"},
	},

	// Folded block scalar
	{
		data:  "scalar: > # Comment\n\n folded\n line\n \n next\n line\n  * one\n  * two\n\n last\n line\n\n",
		value: map[string]string{"scalar": "\nfolded line\nnext line\n * one\n * two\n\nlast line\n"},
	},

	// Map inside interface with no type hints.
	{
		data:  "a: {b: c}",
		value: map[interface{}]interface{}{"a": map[string]interface{}{"b": "c"}},
	},
	// Non-string map inside interface with no type hints.
	{
		data:  "a: {b: c, 1: d}",
		value: map[interface{}]interface{}{"a": map[interface{}]interface{}{"b": "c", 1: "d"}},
	},

	// Structs and type conversions.
	{
		data:  "hello: world",
		value: &struct{ Hello string }{Hello: "world"},
	}, {
		data:  "a: {b: c}",
		value: &struct{ A struct{ B string } }{A: struct{ B string }{B: "c"}},
	}, {
		data:  "a: {b: c}",
		value: &struct{ A *struct{ B string } }{A: &struct{ B string }{B: "c"}},
	}, {
		data:  "a: 'null'",
		value: &struct{ A *unmarshalerType }{A: &unmarshalerType{value: "null"}},
	}, {
		data:  "a: {b: c}",
		value: &struct{ A map[string]string }{A: map[string]string{"b": "c"}},
	}, {
		data:  "a: {b: c}",
		value: &struct{ A *map[string]string }{A: &map[string]string{"b": "c"}},
	}, {
		data:  "a:",
		value: &struct{ A map[string]string }{},
	}, {
		data:  "a: 1",
		value: &struct{ A int }{A: 1},
	}, {
		data:  "a: 1",
		value: &struct{ A float64 }{A: 1},
	}, {
		data:  "a: 1.0",
		value: &struct{ A int }{A: 1},
	}, {
		data:  "a: 1.0",
		value: &struct{ A uint }{A: 1},
	}, {
		data:  "a: [1, 2]",
		value: &struct{ A []int }{A: []int{1, 2}},
	}, {
		data:  "a: [1, 2]",
		value: &struct{ A [2]int }{A: [2]int{1, 2}},
	}, {
		data:  "a: 1",
		value: &struct{ B int }{},
	}, {
		data: "a: 1",
		value: &struct {
			B int "a"
		}{B: 1},
	}, {
		// Some limited backwards compatibility with the 1.1 spec.
		data:  "a: YES",
		value: &struct{ A bool }{A: true},
	},

	// Some cross type conversions
	{
		data:  "v: 42",
		value: map[string]uint{"v": 42},
	}, {
		data:  "v: -42",
		value: map[string]uint{},
		error: "line 1: cannot unmarshal !!int `-42` into uint",
	}, {
		data:  "v: 4294967296",
		value: map[string]uint64{"v": 4294967296},
	}, {
		data:  "v: -4294967296",
		value: map[string]uint64{},
		error: "line 1: cannot unmarshal !!int `-429496...` into uint64",
	},

	// int
	{
		data:  "int_max: 2147483647",
		value: map[string]int{"int_max": math.MaxInt32},
	},
	{
		data:  "int_min: -2147483648",
		value: map[string]int{"int_min": math.MinInt32},
	},
	{
		data:  "int_overflow: 9223372036854775808", // math.MaxInt64 + 1
		value: map[string]int{},
		error: "line 1: cannot unmarshal !!int `9223372...` into int",
	},

	// int64
	{
		data:  "int64_max: 9223372036854775807",
		value: map[string]int64{"int64_max": math.MaxInt64},
	},
	{
		data:  "int64_max_base2: 0b111111111111111111111111111111111111111111111111111111111111111",
		value: map[string]int64{"int64_max_base2": math.MaxInt64},
	},
	{
		data:  "int64_min: -9223372036854775808",
		value: map[string]int64{"int64_min": math.MinInt64},
	},
	{
		data:  "int64_neg_base2: -0b111111111111111111111111111111111111111111111111111111111111111",
		value: map[string]int64{"int64_neg_base2": -math.MaxInt64},
	},
	{
		data:  "int64_overflow: 9223372036854775808", // math.MaxInt64 + 1
		value: map[string]int64{},
		error: "line 1: cannot unmarshal !!int `9223372...` into int64",
	},

	// uint
	{
		data:  "uint_min: 0",
		value: map[string]uint{"uint_min": 0},
	},
	{
		data:  "uint_max: 4294967295",
		value: map[string]uint{"uint_max": math.MaxUint32},
	},
	{
		data:  "uint_underflow: -1",
		value: map[string]uint{},
		error: "line 1: cannot unmarshal !!int `-1` into uint",
	},

	// uint64
	{
		data:  "uint64_min: 0",
		value: map[string]uint{"uint64_min": 0},
	},
	{
		data:  "uint64_max: 18446744073709551615",
		value: map[string]uint64{"uint64_max": math.MaxUint64},
	},
	{
		data:  "uint64_max_base2: 0b1111111111111111111111111111111111111111111111111111111111111111",
		value: map[string]uint64{"uint64_max_base2": math.MaxUint64},
	},
	{
		data:  "uint64_maxint64: 9223372036854775807",
		value: map[string]uint64{"uint64_maxint64": math.MaxInt64},
	},
	{
		data:  "uint64_underflow: -1",
		value: map[string]uint64{},
		error: "line 1: cannot unmarshal !!int `-1` into uint64",
	},

	// float32
	{
		data:  "float32_max: 3.40282346638528859811704183484516925440e+38",
		value: map[string]float32{"float32_max": math.MaxFloat32},
	},
	{
		data:  "float32_nonzero: 1.401298464324817070923729583289916131280e-45",
		value: map[string]float32{"float32_nonzero": math.SmallestNonzeroFloat32},
	},
	{
		data:  "float32_maxuint64: 18446744073709551615",
		value: map[string]float32{"float32_maxuint64": float32(math.MaxUint64)},
	},
	{
		data:  "float32_maxuint64+1: 18446744073709551616",
		value: map[string]float32{"float32_maxuint64+1": float32(math.MaxUint64 + 1)},
	},

	// float64
	{
		data:  "float64_max: 1.797693134862315708145274237317043567981e+308",
		value: map[string]float64{"float64_max": math.MaxFloat64},
	},
	{
		data:  "float64_nonzero: 4.940656458412465441765687928682213723651e-324",
		value: map[string]float64{"float64_nonzero": math.SmallestNonzeroFloat64},
	},
	{
		data:  "float64_maxuint64: 18446744073709551615",
		value: map[string]float64{"float64_maxuint64": float64(math.MaxUint64)},
	},
	{
		data:  "float64_maxuint64+1: 18446744073709551616",
		value: map[string]float64{"float64_maxuint64+1": float64(math.MaxUint64 + 1)},
	},

	// Overflow cases.
	{
		data:  "v: 4294967297",
		value: map[string]int32{},
		error: "line 1: cannot unmarshal !!int `4294967297` into int32",
	}, {
		data:  "v: 128",
		value: map[string]int8{},
		error: "line 1: cannot unmarshal !!int `128` into int8",
	},

	// Quoted values.
	{
		data:  "'1': '\"2\"'",
		value: map[interface{}]interface{}{"1": "\"2\""},
	}, {
		data:  "v:\n- A\n- 'B\n\n  C'\n",
		value: map[string][]string{"v": []string{"A", "B\nC"}},
	},

	// Explicit tags.
	{
		data:  "v: !!float '1.1'",
		value: map[string]interface{}{"v": 1.1},
	}, {
		data:  "v: !!float 0",
		value: map[string]interface{}{"v": float64(0)},
	}, {
		data:  "v: !!float -1",
		value: map[string]interface{}{"v": float64(-1)},
	}, {
		data:  "v: !!null ''",
		value: map[string]interface{}{"v": nil},
	}, {
		data:  "%TAG !y! tag:yaml.org,2002:\n---\nv: !y!int '1'",
		value: map[string]interface{}{"v": 1},
	},

	// Non-specific tag (Issue #75)
	{
		data:  "v: ! test",
		value: map[string]interface{}{"v": "test"},
	},

	// Anchors and aliases.
	{
		data:  "a: &x 1\nb: &y 2\nc: *x\nd: *y\n",
		value: &struct{ A, B, C, D int }{A: 1, B: 2, C: 1, D: 2},
	}, {
		data: "a: &a {c: 1}\nb: *a",
		value: &struct {
			A, B struct {
				C int
			}
		}{A: struct{ C int }{C: 1}, B: struct{ C int }{C: 1}},
	}, {
		data:  "a: &a [1, 2]\nb: *a",
		value: &struct{ B []int }{B: []int{1, 2}},
	},

	// Bug #1133337
	{
		data:  "foo: ''",
		value: map[string]*string{"foo": new(string)},
	}, {
		data:  "foo: null",
		value: map[string]*string{"foo": nil},
	}, {
		data:  "foo: null",
		value: map[string]string{"foo": ""},
	}, {
		data:  "foo: null",
		value: map[string]interface{}{"foo": nil},
	},

	// Support for ~
	{
		data:  "foo: ~",
		value: map[string]*string{"foo": nil},
	}, {
		data:  "foo: ~",
		value: map[string]string{"foo": ""},
	}, {
		data:  "foo: ~",
		value: map[string]interface{}{"foo": nil},
	},

	// Ignored field
	{
		data: "a: 1\nb: 2\n",
		value: &struct {
			A int
			B int "-"
		}{A: 1},
	},

	// Bug #1191981
	{
		data: "" +
			"%YAML 1.1\n" +
			"--- !!str\n" +
			`"Generic line break (no glyph)\n\` + "\n" +
			` Generic line break (glyphed)\n\` + "\n" +
			` Line separator\u2028\` + "\n" +
			` Paragraph separator\u2029"` + "\n",
		value: "" +
			"Generic line break (no glyph)\n" +
			"Generic line break (glyphed)\n" +
			"Line separator\u2028Paragraph separator\u2029",
	},

	// Struct inlining
	{
		data: "a: 1\nb: 2\nc: 3\n",
		value: &struct {
			A int
			C inlineB `yaml:",inline"`
		}{A: 1, C: inlineB{B: 2, inlineC: inlineC{C: 3}}},
	},

	// Struct inlining as a pointer.
	{
		data: "a: 1\nb: 2\nc: 3\n",
		value: &struct {
			A int
			C *inlineB `yaml:",inline"`
		}{A: 1, C: &inlineB{B: 2, inlineC: inlineC{C: 3}}},
	}, {
		data: "a: 1\n",
		value: &struct {
			A int
			C *inlineB `yaml:",inline"`
		}{A: 1},
	}, {
		data: "a: 1\nc: 3\nd: 4\n",
		value: &struct {
			A int
			C *inlineD `yaml:",inline"`
		}{A: 1, C: &inlineD{C: &inlineC{C: 3}, D: 4}},
	},

	// Map inlining
	{
		data: "a: 1\nb: 2\nc: 3\n",
		value: &struct {
			A int
			C map[string]int `yaml:",inline"`
		}{A: 1, C: map[string]int{"b": 2, "c": 3}},
	},

	// bug 1243827
	{
		data:  "a: -b_c",
		value: map[string]interface{}{"a": "-b_c"},
	},
	{
		data:  "a: +b_c",
		value: map[string]interface{}{"a": "+b_c"},
	},
	{
		data:  "a: 50cent_of_dollar",
		value: map[string]interface{}{"a": "50cent_of_dollar"},
	},

	// issue #295 (allow scalars with colons in flow mappings and sequences)
	{
		data: "a: {b: https://github.com/go-yaml/yaml}",
		value: map[string]interface{}{"a": map[string]interface{}{
			"b": "https://github.com/go-yaml/yaml",
		}},
	},
	{
		data:  "a: [https://github.com/go-yaml/yaml]",
		value: map[string]interface{}{"a": []interface{}{"https://github.com/go-yaml/yaml"}},
	},

	// Duration
	{
		data:  "a: 3s",
		value: map[string]time.Duration{"a": 3 * time.Second},
	},

	// Issue #24.
	{
		data:  "a: <foo>",
		value: map[string]string{"a": "<foo>"},
	},

	// Base 60 floats are obsolete and unsupported.
	{
		data:  "a: 1:1\n",
		value: map[string]string{"a": "1:1"},
	},

	// Binary data.
	{
		data:  "a: !!binary gIGC\n",
		value: map[string]string{"a": "\x80\x81\x82"},
	}, {
		data:  "a: !!binary |\n  " + strings.Repeat("kJCQ", 17) + "kJ\n  CQ\n",
		value: map[string]string{"a": strings.Repeat("\x90", 54)},
	}, {
		data:  "a: !!binary |\n  " + strings.Repeat("A", 70) + "\n  ==\n",
		value: map[string]string{"a": strings.Repeat("\x00", 52)},
	},

	// Issue #39.
	{
		data:  "a:\n b:\n  c: d\n",
		value: map[string]struct{ B interface{} }{"a": {B: map[string]interface{}{"c": "d"}}},
	},

	// Custom map type.
	{
		data:  "a: {b: c}",
		value: M{"a": M{"b": "c"}},
	},

	// Support encoding.TextUnmarshaler.
	{
		data:  "a: 1.2.3.4\n",
		value: map[string]textUnmarshaler{"a": textUnmarshaler{S: "1.2.3.4"}},
	},
	{
		data:  "a: 2015-02-24T18:19:39Z\n",
		value: map[string]textUnmarshaler{"a": textUnmarshaler{S: "2015-02-24T18:19:39Z"}},
	},

	// Timestamps
	{
		// Date only.
		data:  "a: 2015-01-01\n",
		value: map[string]time.Time{"a": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
	},
	{
		// RFC3339
		data:  "a: 2015-02-24T18:19:39.12Z\n",
		value: map[string]time.Time{"a": time.Date(2015, 2, 24, 18, 19, 39, .12e9, time.UTC)},
	},
	{
		// RFC3339 with short dates.
		data:  "a: 2015-2-3T3:4:5Z",
		value: map[string]time.Time{"a": time.Date(2015, 2, 3, 3, 4, 5, 0, time.UTC)},
	},
	{
		// ISO8601 lower case t
		data:  "a: 2015-02-24t18:19:39Z\n",
		value: map[string]time.Time{"a": time.Date(2015, 2, 24, 18, 19, 39, 0, time.UTC)},
	},
	{
		// space separate, no time zone
		data:  "a: 2015-02-24 18:19:39\n",
		value: map[string]time.Time{"a": time.Date(2015, 2, 24, 18, 19, 39, 0, time.UTC)},
	},
	// Some cases not currently handled. Uncomment these when
	// the code is fixed.
	//	{
	//		// space separated with time zone
	//		"a: 2001-12-14 21:59:43.10 -5",
	//		map[string]interface{}{"a": time.Date(2001, 12, 14, 21, 59, 43, .1e9, time.UTC)},
	//	},
	//	{
	//		// arbitrary whitespace between fields
	//		"a: 2001-12-14 \t\t \t21:59:43.10 \t Z",
	//		map[string]interface{}{"a": time.Date(2001, 12, 14, 21, 59, 43, .1e9, time.UTC)},
	//	},
	{
		// explicit string tag
		data:  "a: !!str 2015-01-01",
		value: map[string]interface{}{"a": "2015-01-01"},
	},
	{
		// explicit timestamp tag on quoted string
		data:  "a: !!timestamp \"2015-01-01\"",
		value: map[string]time.Time{"a": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
	},
	{
		// explicit timestamp tag on unquoted string
		data:  "a: !!timestamp 2015-01-01",
		value: map[string]time.Time{"a": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
	},
	{
		// quoted string that's a valid timestamp
		data:  "a: \"2015-01-01\"",
		value: map[string]interface{}{"a": "2015-01-01"},
	},
	{
		// explicit timestamp tag into interface.
		data:  "a: !!timestamp \"2015-01-01\"",
		value: map[string]interface{}{"a": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
	},
	{
		// implicit timestamp tag into interface.
		data:  "a: 2015-01-01",
		value: map[string]interface{}{"a": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)},
	},

	// Encode empty lists as zero-length slices.
	{
		data:  "a: []",
		value: &struct{ A []int }{A: []int{}},
	},

	// UTF-16-LE
	{
		data:  "\xff\xfe\xf1\x00o\x00\xf1\x00o\x00:\x00 \x00v\x00e\x00r\x00y\x00 \x00y\x00e\x00s\x00\n\x00",
		value: M{"ñoño": "very yes"},
	},
	// UTF-16-LE with surrogate.
	{
		data:  "\xff\xfe\xf1\x00o\x00\xf1\x00o\x00:\x00 \x00v\x00e\x00r\x00y\x00 \x00y\x00e\x00s\x00 \x00=\xd8\xd4\xdf\n\x00",
		value: M{"ñoño": "very yes 🟔"},
	},

	// UTF-16-BE
	{
		data:  "\xfe\xff\x00\xf1\x00o\x00\xf1\x00o\x00:\x00 \x00v\x00e\x00r\x00y\x00 \x00y\x00e\x00s\x00\n",
		value: M{"ñoño": "very yes"},
	},
	// UTF-16-BE with surrogate.
	{
		data:  "\xfe\xff\x00\xf1\x00o\x00\xf1\x00o\x00:\x00 \x00v\x00e\x00r\x00y\x00 \x00y\x00e\x00s\x00 \xd8=\xdf\xd4\x00\n",
		value: M{"ñoño": "very yes 🟔"},
	},

	// This *is* in fact a float number, per the spec. #171 was a mistake.
	{
		data:  "a: 123456e1\n",
		value: M{"a": 123456e1},
	}, {
		data:  "a: 123456E1\n",
		value: M{"a": 123456e1},
	},
	// yaml-test-suite 3GZX: Spec Example 7.1. Alias Nodes
	{
		data: "First occurrence: &anchor Foo\nSecond occurrence: *anchor\nOverride anchor: &anchor Bar\nReuse anchor: *anchor\n",
		value: map[string]interface{}{
			"First occurrence":  "Foo",
			"Second occurrence": "Foo",
			"Override anchor":   "Bar",
			"Reuse anchor":      "Bar",
		},
	},
	// Single document with garbage following it.
	{
		data:  "---\nhello\n...\n}not yaml",
		value: "hello",
	},

	// Comment scan exhausting the input buffer (issue #469).
	{
		data:  "true\n#" + strings.Repeat(" ", 512*3),
		value: "true",
	}, {
		data:  "true #" + strings.Repeat(" ", 512*3),
		value: "true",
	},

	// CRLF
	{
		data: "a: b\r\nc:\r\n- d\r\n- e\r\n",
		value: map[string]interface{}{
			"a": "b",
			"c": []interface{}{"d", "e"},
		},
	},
}

type M map[string]interface{}

type inlineB struct {
	B       int
	inlineC `yaml:",inline"`
}

type inlineC struct {
	C int
}

type inlineD struct {
	C *inlineC `yaml:",inline"`
	D int
}

func TestUnmarshal(t *testing.T) {
	for i, item := range unmarshalTests {
		t.Run(fmt.Sprintf("%d-%q", i, item.data), func(t *testing.T) {
			value := reflect.New(reflect.ValueOf(item.value).Type())
			err := yaml.Unmarshal([]byte(item.data), value.Interface())
			assertTypeError(t, err, item.error)
			require.Equal(t, item.value, value.Elem().Interface())
		})
	}
}

func assertTypeError(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		require.NoError(t, err)
		return
	}
	typeErr, ok := err.(*yaml.TypeError)
	if !ok {
		t.Fatalf("error is not a TypeError: %v", err)
	}
	matcher := regexp.MustCompile(want)
	if !matcher.MatchString(typeErr.Error()) {
		t.Fatalf("error %q does not match %q", typeErr.Error(), want)
	}
}

func assertError(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		require.NoError(t, err)
		return
	}
	matcher := regexp.MustCompile(want)
	if !matcher.MatchString(err.Error()) {
		t.Fatalf("error %q does not match %q", err.Error(), want)
	}
}

//func (s *S) TestUnmarshalFullTimestamp(c *C) {
//	// Full timestamp in same format as encoded. This is confirmed to be
//	// properly decoded by Python as a timestamp as well.
//	var str = "2015-02-24T18:19:39.123456789-03:00"
//	var t interface{}
//	err := yaml.Unmarshal([]byte(str), &t)
//	c.Assert(err, IsNil)
//	c.Assert(t, Equals, time.Date(2015, 2, 24, 18, 19, 39, 123456789, t.(time.Time).Location()))
//	c.Assert(t.(time.Time).In(time.UTC), Equals, time.Date(2015, 2, 24, 21, 19, 39, 123456789, time.UTC))
//}

func TestUnmarshalFullTimestamp(t *testing.T) {
	// Full timestamp in same format as encoded. This is confirmed to be
	// properly decoded by Python as a timestamp as well.
	str := "2015-02-24T18:19:39.123456789-03:00"
	var val interface{}
	err := yaml.Unmarshal([]byte(str), &val)
	require.NoError(t, err)
	require.Equal(t, time.Date(2015, 2, 24, 18, 19, 39, 123456789, val.(time.Time).Location()), val)
	require.Equal(t, time.Date(2015, 2, 24, 21, 19, 39, 123456789, time.UTC), val.(time.Time).In(time.UTC))
}

func TestDecoderSingleDocument(t *testing.T) {
	for i, item := range unmarshalTests {
		t.Run(fmt.Sprintf("%d-%q", i, item.data), func(t *testing.T) {
			if item.data == "" {
				t.Skip("Behaviour differs when there's no YAML.")
			}
			value := reflect.New(reflect.ValueOf(item.value).Type())
			err := yaml.NewDecoder(strings.NewReader(item.data)).Decode(value.Interface())
			assertTypeError(t, err, item.error)
			require.Equal(t, item.value, value.Elem().Interface())
		})
	}
}

var decoderTests = []struct {
	data   string
	values []interface{}
}{{}, {
	data: "a: b",
	values: []interface{}{
		map[string]interface{}{"a": "b"},
	},
}, {
	data: "---\na: b\n...\n",
	values: []interface{}{
		map[string]interface{}{"a": "b"},
	},
}, {
	data: "---\n'hello'\n...\n---\ngoodbye\n...\n",
	values: []interface{}{
		"hello",
		"goodbye",
	},
}}

func TestDecoder(t *testing.T) {
	for i, item := range decoderTests {
		t.Run(fmt.Sprintf("%d-%q", i, item.data), func(t *testing.T) {
			var values []interface{}
			dec := yaml.NewDecoder(strings.NewReader(item.data))
			for {
				var value interface{}
				err := dec.Decode(&value)
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				values = append(values, value)
			}
			require.Equal(t, item.values, values)
		})
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("some read error")
}

func TestDecoderReadError(t *testing.T) {
	err := yaml.NewDecoder(errReader{}).Decode(&struct{}{})
	require.EqualError(t, err, `yaml: input error: some read error`)
}

func TestUnmarshalNaN(t *testing.T) {
	value := map[string]interface{}{}
	err := yaml.Unmarshal([]byte("notanum: .NaN"), &value)
	require.NoError(t, err)
	require.True(t, math.IsNaN(value["notanum"].(float64)))
}

func TestUnmarshalDurationInt(t *testing.T) {
	// Don't accept plain ints as durations as it's unclear (issue #200).
	var d time.Duration
	err := yaml.Unmarshal([]byte("123"), &d)
	assertTypeError(t, err, "(?s).* line 1: cannot unmarshal !!int `123` into time.Duration")
}

var unmarshalErrorTests = []struct {
	data, error string
}{
	{data: "v: !!float 'error'", error: "yaml: cannot decode !!str `error` as a !!float"},
	{data: "v: [A,", error: "yaml: line 1: did not find expected node content"},
	{data: "v:\n- [A,", error: "yaml: line 2: did not find expected node content"},
	{data: "a:\n- b: *,", error: "yaml: line 2: did not find expected alphabetic or numeric character"},
	{data: "a: *b\n", error: "yaml: unknown anchor 'b' referenced"},
	{data: "a: &a\n  b: *a\n", error: "yaml: anchor 'a' value contains itself"},
	{data: "value: -", error: "yaml: block sequence entries are not allowed in this context"},
	{data: "a: !!binary ==", error: "yaml: !!binary value contains invalid base64 data"},
	{data: "{[.]}", error: `yaml: invalid map key: \[\]interface \{\}\{"\."\}`},
	{data: "{{.}}", error: `yaml: invalid map key: map\[string]interface \{\}\{".":interface \{\}\(nil\)\}`},
	{data: "b: *a\na: &a {c: 1}", error: `yaml: unknown anchor 'a' referenced`},
	{data: "%TAG !%79! tag:yaml.org,2002:\n---\nv: !%79!int '1'", error: "yaml: did not find expected whitespace"},
	{data: "a:\n  1:\nb\n  2:", error: ".*could not find expected ':'"},
	{data: "a: 1\nb: 2\nc 2\nd: 3\n", error: "^yaml: line 3: could not find expected ':'$"},
	{data: "#\n-\n{", error: "yaml: line 3: could not find expected ':'"},   // Issue #665
	{data: "0: [:!00 \xef", error: "yaml: incomplete UTF-8 octet sequence"}, // Issue #666
	{
		data: "a: &a [00,00,00,00,00,00,00,00,00]\n" +
			"b: &b [*a,*a,*a,*a,*a,*a,*a,*a,*a]\n" +
			"c: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b]\n" +
			"d: &d [*c,*c,*c,*c,*c,*c,*c,*c,*c]\n" +
			"e: &e [*d,*d,*d,*d,*d,*d,*d,*d,*d]\n" +
			"f: &f [*e,*e,*e,*e,*e,*e,*e,*e,*e]\n" +
			"g: &g [*f,*f,*f,*f,*f,*f,*f,*f,*f]\n" +
			"h: &h [*g,*g,*g,*g,*g,*g,*g,*g,*g]\n" +
			"i: &i [*h,*h,*h,*h,*h,*h,*h,*h,*h]\n",
		error: "yaml: document contains excessive aliasing",
	},
}

func TestUnmarshalErrors(t *testing.T) {
	for i, item := range unmarshalErrorTests {
		t.Run(fmt.Sprintf("test %d: %q", i, item.data), func(t *testing.T) {
			var value interface{}
			err := yaml.Unmarshal([]byte(item.data), &value)
			assertError(t, err, item.error)
		})
	}
}

func TestDecoderErrors(t *testing.T) {
	for _, item := range unmarshalErrorTests {
		t.Run(item.data, func(t *testing.T) {
			var value interface{}
			err := yaml.NewDecoder(strings.NewReader(item.data)).Decode(&value)
			assertError(t, err, item.error)
		})
	}
}

var unmarshalerTests = []struct {
	data, tag string
	value     interface{}
}{
	{data: "_: {hi: there}", tag: "!!map", value: map[string]interface{}{"hi": "there"}},
	{data: "_: [1,A]", tag: "!!seq", value: []interface{}{1, "A"}},
	{data: "_: 10", tag: "!!int", value: 10},
	{data: "_: null", tag: "!!null"},
	{data: `_: BAR!`, tag: "!!str", value: "BAR!"},
	{data: `_: "BAR!"`, tag: "!!str", value: "BAR!"},
	{data: "_: !!foo 'BAR!'", tag: "!!foo", value: "BAR!"},
	{data: `_: ""`, tag: "!!str", value: ""},
}

var unmarshalerResult = map[int]error{}

type unmarshalerType struct {
	value interface{}
}

func (o *unmarshalerType) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&o.value); err != nil {
		return err
	}
	if i, ok := o.value.(int); ok {
		if result, ok := unmarshalerResult[i]; ok {
			return result
		}
	}
	return nil
}

type unmarshalerPointer struct {
	Field *unmarshalerType "_"
}

type unmarshalerValue struct {
	Field unmarshalerType "_"
}

type unmarshalerInlined struct {
	Field   *unmarshalerType "_"
	Inlined unmarshalerType  `yaml:",inline"`
}

type unmarshalerInlinedTwice struct {
	InlinedTwice unmarshalerInlined `yaml:",inline"`
}

type obsoleteUnmarshalerType struct {
	value interface{}
}

func (o *obsoleteUnmarshalerType) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	if err := unmarshal(&o.value); err != nil {
		return err
	}
	if i, ok := o.value.(int); ok {
		if result, ok := unmarshalerResult[i]; ok {
			return result
		}
	}
	return nil
}

type obsoleteUnmarshalerPointer struct {
	Field *obsoleteUnmarshalerType "_"
}

type obsoleteUnmarshalerValue struct {
	Field obsoleteUnmarshalerType "_"
}

func TestUnmarshalerPointerField(t *testing.T) {
	for _, item := range unmarshalerTests {
		obj := &unmarshalerPointer{}
		err := yaml.Unmarshal([]byte(item.data), obj)
		require.NoError(t, err)
		if item.value == nil {
			require.Nil(t, obj.Field)
		} else {
			require.NotNil(t, obj.Field, "Pointer not initialized (%#v)", item.value)
			require.Equal(t, item.value, obj.Field.value)
		}
	}
	for _, item := range unmarshalerTests {
		obj := &obsoleteUnmarshalerPointer{}
		err := yaml.Unmarshal([]byte(item.data), obj)
		require.NoError(t, err)
		if item.value == nil {
			require.Nil(t, obj.Field)
		} else {
			require.NotNil(t, obj.Field, "Pointer not initialized (%#v)", item.value)
			require.Equal(t, item.value, obj.Field.value)
		}
	}
}

func TestUnmarshalerValueField(t *testing.T) {
	for _, item := range unmarshalerTests {
		obj := &obsoleteUnmarshalerValue{}
		err := yaml.Unmarshal([]byte(item.data), obj)
		require.NoError(t, err)
		require.NotNil(t, obj.Field, "Pointer not initialized (%#v)", item.value)
		require.Equal(t, item.value, obj.Field.value)
	}
}

func TestUnmarshalerInlinedField(t *testing.T) {
	obj := &unmarshalerInlined{}
	err := yaml.Unmarshal([]byte("_: a\ninlined: b\n"), obj)
	require.NoError(t, err)
	require.Equal(t, &unmarshalerType{value: "a"}, obj.Field)
	require.Equal(t, unmarshalerType{value: map[string]interface{}{"_": "a", "inlined": "b"}}, obj.Inlined)

	twc := &unmarshalerInlinedTwice{}
	err = yaml.Unmarshal([]byte("_: a\ninlined: b\n"), twc)
	require.NoError(t, err)
	require.Equal(t, &unmarshalerType{value: "a"}, twc.InlinedTwice.Field)
	require.Equal(t, unmarshalerType{value: map[string]interface{}{"_": "a", "inlined": "b"}}, twc.InlinedTwice.Inlined)
}

func TestUnmarshalerWholeDocument(t *testing.T) {
	obj := &obsoleteUnmarshalerType{}
	err := yaml.Unmarshal([]byte(unmarshalerTests[0].data), obj)
	require.NoError(t, err)
	value, ok := obj.value.(map[string]interface{})
	require.True(t, ok, "value: %#v", obj.value)
	require.Equal(t, unmarshalerTests[0].value, value["_"])
}

func TestUnmarshalerTypeError(t *testing.T) {
	unmarshalerResult[2] = &yaml.TypeError{Errors: []string{"foo"}}
	unmarshalerResult[4] = &yaml.TypeError{Errors: []string{"bar"}}
	defer func() {
		delete(unmarshalerResult, 2)
		delete(unmarshalerResult, 4)
	}()

	type T struct {
		Before int
		After  int
		M      map[string]*unmarshalerType
	}
	var v T
	data := `{before: A, m: {abc: 1, def: 2, ghi: 3, jkl: 4}, after: B}`
	err := yaml.Unmarshal([]byte(data), &v)
	require.Error(t, err)
	require.Equal(t, ""+
		"yaml: unmarshal errors:\n"+
		"  line 1: cannot unmarshal !!str `A` into int\n"+
		"  foo\n"+
		"  bar\n"+
		"  line 1: cannot unmarshal !!str `B` into int", err.Error())
	require.NotNil(t, v.M["abc"])
	require.Nil(t, v.M["def"])
	require.NotNil(t, v.M["ghi"])
	require.Nil(t, v.M["jkl"])

	require.Equal(t, 1, v.M["abc"].value)
	require.Equal(t, 3, v.M["ghi"].value)
}

func TestObsoleteUnmarshalerTypeError(t *testing.T) {
	unmarshalerResult[2] = &yaml.TypeError{Errors: []string{"foo"}}
	unmarshalerResult[4] = &yaml.TypeError{Errors: []string{"bar"}}
	defer func() {
		delete(unmarshalerResult, 2)
		delete(unmarshalerResult, 4)
	}()

	type T struct {
		Before int
		After  int
		M      map[string]*obsoleteUnmarshalerType
	}
	var v T
	data := `{before: A, m: {abc: 1, def: 2, ghi: 3, jkl: 4}, after: B}`
	err := yaml.Unmarshal([]byte(data), &v)
	require.Error(t, err)
	require.Equal(t, ""+
		"yaml: unmarshal errors:\n"+
		"  line 1: cannot unmarshal !!str `A` into int\n"+
		"  foo\n"+
		"  bar\n"+
		"  line 1: cannot unmarshal !!str `B` into int", err.Error())
	require.NotNil(t, v.M["abc"])
	require.Nil(t, v.M["def"])
	require.NotNil(t, v.M["ghi"])
	require.Nil(t, v.M["jkl"])

	require.Equal(t, 1, v.M["abc"].value)
	require.Equal(t, 3, v.M["ghi"].value)
}

type proxyTypeError struct{}

func (v *proxyTypeError) UnmarshalYAML(node *yaml.Node) error {
	var s string
	var a int32
	var b int64
	if err := node.Decode(&s); err != nil {
		panic(err)
	}
	if s == "a" {
		if err := node.Decode(&b); err == nil {
			panic("should have failed")
		}
		return node.Decode(&a)
	}
	if err := node.Decode(&a); err == nil {
		panic("should have failed")
	}
	return node.Decode(&b)
}

func TestUnmarshalerTypeErrorProxying(t *testing.T) {
	type T struct {
		Before int
		After  int
		M      map[string]*proxyTypeError
	}
	var v T
	data := `{before: A, m: {abc: a, def: b}, after: B}`
	err := yaml.Unmarshal([]byte(data), &v)
	require.Error(t, err)
	require.Equal(t, ""+
		"yaml: unmarshal errors:\n"+
		"  line 1: cannot unmarshal !!str `A` into int\n"+
		"  line 1: cannot unmarshal !!str `a` into int32\n"+
		"  line 1: cannot unmarshal !!str `b` into int64\n"+
		"  line 1: cannot unmarshal !!str `B` into int", err.Error())
}

type obsoleteProxyTypeError struct{}

func (v *obsoleteProxyTypeError) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	var a int32
	var b int64
	if err := unmarshal(&s); err != nil {
		panic(err)
	}
	if s == "a" {
		if err := unmarshal(&b); err == nil {
			panic("should have failed")
		}
		return unmarshal(&a)
	}
	if err := unmarshal(&a); err == nil {
		panic("should have failed")
	}
	return unmarshal(&b)
}

func TestObsoleteUnmarshalerTypeErrorProxying(t *testing.T) {
	type T struct {
		Before int
		After  int
		M      map[string]*obsoleteProxyTypeError
	}
	var v T
	data := `{before: A, m: {abc: a, def: b}, after: B}`
	err := yaml.Unmarshal([]byte(data), &v)
	require.Error(t, err)
	require.Equal(t, ""+
		"yaml: unmarshal errors:\n"+
		"  line 1: cannot unmarshal !!str `A` into int\n"+
		"  line 1: cannot unmarshal !!str `a` into int32\n"+
		"  line 1: cannot unmarshal !!str `b` into int64\n"+
		"  line 1: cannot unmarshal !!str `B` into int", err.Error())
}

var failingErr = errors.New("failingErr")

type failingUnmarshaler struct{}

func (ft *failingUnmarshaler) UnmarshalYAML(node *yaml.Node) error {
	return failingErr
}

func TestUnmarshalerError(t *testing.T) {
	err := yaml.Unmarshal([]byte("a: b"), &failingUnmarshaler{})
	require.Equal(t, failingErr, err)
}

type obsoleteFailingUnmarshaler struct{}

func (ft *obsoleteFailingUnmarshaler) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return failingErr
}

func TestObsoleteUnmarshalerError(t *testing.T) {
	err := yaml.Unmarshal([]byte("a: b"), &obsoleteFailingUnmarshaler{})
	require.Equal(t, failingErr, err)
}

type sliceUnmarshaler []int

func (su *sliceUnmarshaler) UnmarshalYAML(node *yaml.Node) error {
	var slice []int
	err := node.Decode(&slice)
	if err == nil {
		*su = slice
		return nil
	}

	var intVal int
	err = node.Decode(&intVal)
	if err == nil {
		*su = []int{intVal}
		return nil
	}

	return err
}

func TestUnmarshalerRetry(t *testing.T) {
	var su sliceUnmarshaler
	err := yaml.Unmarshal([]byte("[1, 2, 3]"), &su)
	require.NoError(t, err)
	require.Equal(t, sliceUnmarshaler([]int{1, 2, 3}), su)

	err = yaml.Unmarshal([]byte("1"), &su)
	require.NoError(t, err)
	require.Equal(t, sliceUnmarshaler([]int{1}), su)
}

type obsoleteSliceUnmarshaler []int

func (su *obsoleteSliceUnmarshaler) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var slice []int
	err := unmarshal(&slice)
	if err == nil {
		*su = slice
		return nil
	}

	var intVal int
	err = unmarshal(&intVal)
	if err == nil {
		*su = []int{intVal}
		return nil
	}

	return err
}

func TestObsoleteUnmarshalerRetry(t *testing.T) {
	var su obsoleteSliceUnmarshaler
	err := yaml.Unmarshal([]byte("[1, 2, 3]"), &su)
	require.NoError(t, err)
	require.Equal(t, obsoleteSliceUnmarshaler([]int{1, 2, 3}), su)

	err = yaml.Unmarshal([]byte("1"), &su)
	require.NoError(t, err)
	require.Equal(t, obsoleteSliceUnmarshaler([]int{1}), su)
}

// From http://yaml.org/type/merge.html
var mergeTests = `
anchors:
  list:
    - &CENTER { "x": 1, "y": 2 }
    - &LEFT   { "x": 0, "y": 2 }
    - &BIG    { "r": 10 }
    - &SMALL  { "r": 1 }

# All the following maps are equal:

plain:
  # Explicit keys
  "x": 1
  "y": 2
  "r": 10
  label: center/big

mergeOne:
  # Merge one map
  << : *CENTER
  "r": 10
  label: center/big

mergeMultiple:
  # Merge multiple maps
  << : [ *CENTER, *BIG ]
  label: center/big

override:
  # Override
  << : [ *BIG, *LEFT, *SMALL ]
  "x": 1
  label: center/big

shortTag:
  # Explicit short merge tag
  !!merge "<<" : [ *CENTER, *BIG ]
  label: center/big

longTag:
  # Explicit merge long tag
  !<tag:yaml.org,2002:merge> "<<" : [ *CENTER, *BIG ]
  label: center/big

inlineMap:
  # Inlined map 
  << : {"x": 1, "y": 2, "r": 10}
  label: center/big

inlineSequenceMap:
  # Inlined map in sequence
  << : [ *CENTER, {"r": 10} ]
  label: center/big
`

func TestMerge(t *testing.T) {
	want := map[string]interface{}{
		"x":     1,
		"y":     2,
		"r":     10,
		"label": "center/big",
	}

	wantStringMap := make(map[string]interface{})
	for k, v := range want {
		wantStringMap[fmt.Sprintf("%v", k)] = v
	}

	var m map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(mergeTests), &m)
	require.NoError(t, err)
	for name, test := range m {
		if name == "anchors" {
			continue
		}
		if name == "plain" {
			require.Equal(t, wantStringMap, test, "test %q failed", name)
			continue
		}
		require.Equal(t, want, test, "test %q failed", name)
	}
}

func TestMergeStruct(t *testing.T) {
	type Data struct {
		X, Y, R int
		Label   string
	}
	want := Data{X: 1, Y: 2, R: 10, Label: "center/big"}

	var m map[string]Data
	err := yaml.Unmarshal([]byte(mergeTests), &m)
	require.NoError(t, err)
	for name, test := range m {
		if name == "anchors" {
			continue
		}
		require.Equal(t, want, test, "test %q failed", name)
	}
}

var mergeTestsNested = `
mergeouter1: &mergeouter1
    d: 40
    e: 50

mergeouter2: &mergeouter2
    e: 5
    f: 6
    g: 70

mergeinner1: &mergeinner1
    <<: *mergeouter1
    inner:
        a: 1
        b: 2

mergeinner2: &mergeinner2
    <<: *mergeouter2
    inner:
        a: -1
        b: -2

outer:
    <<: [*mergeinner1, *mergeinner2]
    f: 60
    inner:
        a: 10
`

func TestMergeNestedStruct(t *testing.T) {
	// Issue #818: Merging used to just unmarshal twice on the target
	// value, which worked for maps as these were replaced by the new map,
	// but not on struct values as these are preserved. This resulted in
	// the nested data from the merged map to be mixed up with the data
	// from the map being merged into.
	//
	// This test also prevents two potential bugs from showing up:
	//
	// 1) A simple implementation might just zero out the nested value
	//    before unmarshaling the second time, but this would clobber previous
	//    data that is usually respected ({C: 30} below).
	//
	// 2) A simple implementation might attempt to handle the key skipping
	//    directly by iterating over the merging map without recursion, but
	//    there are more complex cases that require recursion.
	//
	// Quick summary of the fields:
	//
	// - A must come from outer and not overriden
	// - B must not be set as its in the ignored merge
	// - C should still be set as it's preset in the value
	// - D should be set from the recursive merge
	// - E should be set from the first recursive merge, ignored on the second
	// - F should be set in the inlined map from outer, ignored later
	// - G should be set in the inlined map from the second recursive merge
	//

	type Inner struct {
		A, B, C int
	}
	type Outer struct {
		D, E   int
		Inner  Inner
		Inline map[string]int `yaml:",inline"`
	}
	type Data struct {
		Outer Outer
	}

	test := Data{Outer: Outer{Inner: Inner{C: 30}}}
	want := Data{Outer: Outer{D: 40, E: 50, Inner: Inner{A: 10, C: 30}, Inline: map[string]int{"f": 60, "g": 70}}}

	err := yaml.Unmarshal([]byte(mergeTestsNested), &test)
	require.NoError(t, err)
	require.Equal(t, want, test)

	// Repeat test with a map.

	var testm map[string]interface{}
	var wantm = map[string]interface{}{
		"f": 60,
		"inner": map[string]interface{}{
			"a": 10,
		},
		"d": 40,
		"e": 50,
		"g": 70,
	}
	err = yaml.Unmarshal([]byte(mergeTestsNested), &testm)
	require.NoError(t, err)
	require.Equal(t, wantm, testm["outer"])
}

var unmarshalNullTests = []struct {
	input              string
	pristine, expected func() interface{}
}{{
	input:    "null",
	pristine: func() interface{} { var v interface{}; v = "v"; return &v },
	expected: func() interface{} { var v interface{}; v = nil; return &v },
}, {
	input:    "null",
	pristine: func() interface{} { var s = "s"; return &s },
	expected: func() interface{} { var s = "s"; return &s },
}, {
	input:    "null",
	pristine: func() interface{} { var s = "s"; sptr := &s; return &sptr },
	expected: func() interface{} { var sptr *string; return &sptr },
}, {
	input:    "null",
	pristine: func() interface{} { var i = 1; return &i },
	expected: func() interface{} { var i = 1; return &i },
}, {
	input:    "null",
	pristine: func() interface{} { var i = 1; iptr := &i; return &iptr },
	expected: func() interface{} { var iptr *int; return &iptr },
}, {
	input:    "null",
	pristine: func() interface{} { var m = map[string]int{"s": 1}; return &m },
	expected: func() interface{} { var m map[string]int; return &m },
}, {
	input:    "null",
	pristine: func() interface{} { var m = map[string]int{"s": 1}; return m },
	expected: func() interface{} { var m = map[string]int{"s": 1}; return m },
}, {
	input:    "s2: null\ns3: null",
	pristine: func() interface{} { var m = map[string]int{"s1": 1, "s2": 2}; return m },
	expected: func() interface{} { var m = map[string]int{"s1": 1, "s2": 2, "s3": 0}; return m },
}, {
	input:    "s2: null\ns3: null",
	pristine: func() interface{} { var m = map[string]interface{}{"s1": 1, "s2": 2}; return m },
	expected: func() interface{} { var m = map[string]interface{}{"s1": 1, "s2": nil, "s3": nil}; return m },
}}

func TestUnmarshalNull(t *testing.T) {
	for _, test := range unmarshalNullTests {
		pristine := test.pristine()
		expected := test.expected()
		err := yaml.Unmarshal([]byte(test.input), pristine)
		require.NoError(t, err)
		require.Equal(t, expected, pristine)
	}
}

func TestUnmarshalPreservesData(t *testing.T) {
	var v struct {
		A, B int
		C    int `yaml:"-"`
	}
	v.A = 42
	v.C = 88
	err := yaml.Unmarshal([]byte("---"), &v)
	require.NoError(t, err)
	require.Equal(t, 42, v.A)
	require.Equal(t, 0, v.B)
	require.Equal(t, 88, v.C)

	err = yaml.Unmarshal([]byte("b: 21\nc: 99"), &v)
	require.NoError(t, err)
	require.Equal(t, 42, v.A)
	require.Equal(t, 21, v.B)
	require.Equal(t, 88, v.C)
}

func TestUnmarshalSliceOnPreset(t *testing.T) {
	// Issue #48.
	v := struct{ A []int }{A: []int{1}}
	err := yaml.Unmarshal([]byte("a: [2]"), &v)
	require.NoError(t, err)
	require.Equal(t, []int{2}, v.A)
}

var unmarshalStrictTests = []struct {
	known  bool
	unique bool
	data   string
	value  interface{}
	error  string
}{{
	known: true,
	data:  "a: 1\nc: 2\n",
	value: struct{ A, B int }{A: 1},
	error: "yaml: unmarshal errors:\n  line 2: field c not found in type struct { A int; B int }",
}, {
	unique: true,
	data:   "a: 1\nb: 2\na: 3\n",
	value:  struct{ A, B int }{A: 3, B: 2},
	error:  "yaml: unmarshal errors:\n  line 3: mapping key \"a\" already defined at line 1",
}, {
	unique: true,
	data:   "c: 3\na: 1\nb: 2\nc: 4\n",
	value: struct {
		A       int
		inlineB `yaml:",inline"`
	}{
		A: 1,
		inlineB: inlineB{
			B: 2,
			inlineC: inlineC{
				C: 4,
			},
		},
	},
	error: "yaml: unmarshal errors:\n  line 4: mapping key \"c\" already defined at line 1",
}, {
	unique: true,
	data:   "c: 0\na: 1\nb: 2\nc: 1\n",
	value: struct {
		A       int
		inlineB `yaml:",inline"`
	}{
		A: 1,
		inlineB: inlineB{
			B: 2,
			inlineC: inlineC{
				C: 1,
			},
		},
	},
	error: "yaml: unmarshal errors:\n  line 4: mapping key \"c\" already defined at line 1",
}, {
	unique: true,
	data:   "c: 1\na: 1\nb: 2\nc: 3\n",
	value: struct {
		A int
		M map[string]interface{} `yaml:",inline"`
	}{
		A: 1,
		M: map[string]interface{}{
			"b": 2,
			"c": 3,
		},
	},
	error: "yaml: unmarshal errors:\n  line 4: mapping key \"c\" already defined at line 1",
}, {
	unique: true,
	data:   "a: 1\n9: 2\nnull: 3\n9: 4",
	value: map[interface{}]interface{}{
		"a": 1,
		nil: 3,
		9:   4,
	},
	error: "yaml: unmarshal errors:\n  line 4: mapping key \"9\" already defined at line 2",
}}

func TestUnmarshalKnownFields(t *testing.T) {
	for i, item := range unmarshalStrictTests {
		t.Logf("test %d: %q", i, item.data)
		// First test that normal Unmarshal unmarshals to the expected value.
		if !item.unique {
			value := reflect.New(reflect.ValueOf(item.value).Type())
			err := yaml.Unmarshal([]byte(item.data), value.Interface())
			require.NoError(t, err)
			require.Equal(t, item.value, value.Elem().Interface())
		}

		// Then test that it fails on the same thing with KnownFields on.
		value := reflect.New(reflect.ValueOf(item.value).Type())
		dec := yaml.NewDecoder(bytes.NewBuffer([]byte(item.data)))
		dec.KnownFields(item.known)
		err := dec.Decode(value.Interface())
		require.EqualError(t, err, item.error)
	}
}

type textUnmarshaler struct {
	S string
}

func (t *textUnmarshaler) UnmarshalText(s []byte) error {
	t.S = string(s)
	return nil
}

func TestFuzzCrashers(t *testing.T) {
	cases := []string{
		// runtime error: index out of range
		"\"\\0\\\r\n",

		// should not happen
		"  0: [\n] 0",
		"? ? \"\n\" 0",
		"    - {\n000}0",
		"0:\n  0: [0\n] 0",
		"    - \"\n000\"0",
		"    - \"\n000\"\"",
		"0:\n    - {\n000}0",
		"0:\n    - \"\n000\"0",
		"0:\n    - \"\n000\"\"",

		// runtime error: index out of range
		" \ufeff\n",
		"? \ufeff\n",
		"? \ufeff:\n",
		"0: \ufeff\n",
		"? \ufeff: \ufeff\n",
	}
	for _, data := range cases {
		var v interface{}
		_ = yaml.Unmarshal([]byte(data), &v)
	}
}
