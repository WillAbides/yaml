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
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/willabides/yaml"
)

var marshalIntTest = 123

var marshalTests = []struct {
	value interface{}
	data  string
}{
	{
		data: "null\n",
	}, {
		value: (*marshalerType)(nil),
		data:  "null\n",
	}, {
		value: &struct{}{},
		data:  "{}\n",
	}, {
		value: map[string]string{"v": "hi"},
		data:  "v: hi\n",
	}, {
		value: map[string]interface{}{"v": "hi"},
		data:  "v: hi\n",
	}, {
		value: map[string]string{"v": "true"},
		data:  "v: \"true\"\n",
	}, {
		value: map[string]string{"v": "false"},
		data:  "v: \"false\"\n",
	}, {
		value: map[string]interface{}{"v": true},
		data:  "v: true\n",
	}, {
		value: map[string]interface{}{"v": false},
		data:  "v: false\n",
	}, {
		value: map[string]interface{}{"v": 10},
		data:  "v: 10\n",
	}, {
		value: map[string]interface{}{"v": -10},
		data:  "v: -10\n",
	}, {
		value: map[string]uint{"v": 42},
		data:  "v: 42\n",
	}, {
		value: map[string]interface{}{"v": int64(4294967296)},
		data:  "v: 4294967296\n",
	}, {
		value: map[string]int64{"v": int64(4294967296)},
		data:  "v: 4294967296\n",
	}, {
		value: map[string]uint64{"v": 4294967296},
		data:  "v: 4294967296\n",
	}, {
		value: map[string]interface{}{"v": "10"},
		data:  "v: \"10\"\n",
	}, {
		value: map[string]interface{}{"v": 0.1},
		data:  "v: 0.1\n",
	}, {
		value: map[string]interface{}{"v": float64(0.1)},
		data:  "v: 0.1\n",
	}, {
		value: map[string]interface{}{"v": float32(0.99)},
		data:  "v: 0.99\n",
	}, {
		value: map[string]interface{}{"v": -0.1},
		data:  "v: -0.1\n",
	}, {
		value: map[string]interface{}{"v": math.Inf(+1)},
		data:  "v: .inf\n",
	}, {
		value: map[string]interface{}{"v": math.Inf(-1)},
		data:  "v: -.inf\n",
	}, {
		value: map[string]interface{}{"v": math.NaN()},
		data:  "v: .nan\n",
	}, {
		value: map[string]interface{}{"v": nil},
		data:  "v: null\n",
	}, {
		value: map[string]interface{}{"v": ""},
		data:  "v: \"\"\n",
	}, {
		value: map[string][]string{"v": []string{"A", "B"}},
		data:  "v:\n    - A\n    - B\n",
	}, {
		value: map[string][]string{"v": []string{"A", "B\nC"}},
		data:  "v:\n    - A\n    - |-\n      B\n      C\n",
	}, {
		value: map[string][]interface{}{"v": []interface{}{"A", 1, map[string][]int{"B": []int{2, 3}}}},
		data:  "v:\n    - A\n    - 1\n    - B:\n        - 2\n        - 3\n",
	}, {
		value: map[string]interface{}{"a": map[interface{}]interface{}{"b": "c"}},
		data:  "a:\n    b: c\n",
	}, {
		value: map[string]interface{}{"a": "-"},
		data:  "a: '-'\n",
	},

	// Simple values.
	{
		value: &marshalIntTest,
		data:  "123\n",
	},

	// Structures
	{
		value: &struct{ Hello string }{Hello: "world"},
		data:  "hello: world\n",
	}, {
		value: &struct {
			A struct {
				B string
			}
		}{A: struct{ B string }{B: "c"}},
		data: "a:\n    b: c\n",
	}, {
		value: &struct {
			A *struct {
				B string
			}
		}{A: &struct{ B string }{B: "c"}},
		data: "a:\n    b: c\n",
	}, {
		value: &struct {
			A *struct {
				B string
			}
		}{},
		data: "a: null\n",
	}, {
		value: &struct{ A int }{A: 1},
		data:  "a: 1\n",
	}, {
		value: &struct{ A []int }{A: []int{1, 2}},
		data:  "a:\n    - 1\n    - 2\n",
	}, {
		value: &struct{ A [2]int }{A: [2]int{1, 2}},
		data:  "a:\n    - 1\n    - 2\n",
	}, {
		value: &struct {
			B int "a"
		}{B: 1},
		data: "a: 1\n",
	}, {
		value: &struct{ A bool }{A: true},
		data:  "a: true\n",
	}, {
		value: &struct{ A string }{A: "true"},
		data:  "a: \"true\"\n",
	}, {
		value: &struct{ A string }{A: "off"},
		data:  "a: \"off\"\n",
	},

	// Conditional flag
	{
		value: &struct {
			A int "a,omitempty"
			B int "b,omitempty"
		}{A: 1},
		data: "a: 1\n",
	}, {
		value: &struct {
			A int "a,omitempty"
			B int "b,omitempty"
		}{},
		data: "{}\n",
	}, {
		value: &struct {
			A *struct{ X, y int } "a,omitempty,flow"
		}{A: &struct{ X, y int }{X: 1, y: 2}},
		data: "a: {x: 1}\n",
	}, {
		value: &struct {
			A *struct{ X, y int } "a,omitempty,flow"
		}{},
		data: "{}\n",
	}, {
		value: &struct {
			A *struct{ X, y int } "a,omitempty,flow"
		}{A: &struct{ X, y int }{}},
		data: "a: {x: 0}\n",
	}, {
		value: &struct {
			A struct{ X, y int } "a,omitempty,flow"
		}{A: struct{ X, y int }{X: 1, y: 2}},
		data: "a: {x: 1}\n",
	}, {
		value: &struct {
			A struct{ X, y int } "a,omitempty,flow"
		}{A: struct{ X, y int }{y: 1}},
		data: "{}\n",
	}, {
		value: &struct {
			A float64 "a,omitempty"
			B float64 "b,omitempty"
		}{A: 1},
		data: "a: 1\n",
	},
	{
		value: &struct {
			T1 time.Time  "t1,omitempty"
			T2 time.Time  "t2,omitempty"
			T3 *time.Time "t3,omitempty"
			T4 *time.Time "t4,omitempty"
		}{
			T2: time.Date(2018, 1, 9, 10, 40, 47, 0, time.UTC),
			T4: newTime(time.Date(2098, 1, 9, 10, 40, 47, 0, time.UTC)),
		},
		data: "t2: 2018-01-09T10:40:47Z\nt4: 2098-01-09T10:40:47Z\n",
	},
	// Nil interface that implements Marshaler.
	{
		value: map[string]yaml.Marshaler{
			"a": nil,
		},
		data: "a: null\n",
	},

	// Flow flag
	{
		value: &struct {
			A []int "a,flow"
		}{A: []int{1, 2}},
		data: "a: [1, 2]\n",
	}, {
		value: &struct {
			A map[string]string "a,flow"
		}{A: map[string]string{"b": "c", "d": "e"}},
		data: "a: {b: c, d: e}\n",
	}, {
		value: &struct {
			A struct {
				B, D string
			} "a,flow"
		}{A: struct{ B, D string }{B: "c", D: "e"}},
		data: "a: {b: c, d: e}\n",
	}, {
		value: &struct {
			A string "a,flow"
		}{A: "b\nc"},
		data: "a: \"b\\nc\"\n",
	},

	// Unexported field
	{
		value: &struct {
			u int
			A int
		}{A: 1},
		data: "a: 1\n",
	},

	// Ignored field
	{
		value: &struct {
			A int
			B int "-"
		}{A: 1, B: 2},
		data: "a: 1\n",
	},

	// Struct inlining
	{
		value: &struct {
			A int
			C inlineB `yaml:",inline"`
		}{A: 1, C: inlineB{B: 2, inlineC: inlineC{C: 3}}},
		data: "a: 1\nb: 2\nc: 3\n",
	},
	// Struct inlining as a pointer
	{
		value: &struct {
			A int
			C *inlineB `yaml:",inline"`
		}{A: 1, C: &inlineB{B: 2, inlineC: inlineC{C: 3}}},
		data: "a: 1\nb: 2\nc: 3\n",
	}, {
		value: &struct {
			A int
			C *inlineB `yaml:",inline"`
		}{A: 1},
		data: "a: 1\n",
	}, {
		value: &struct {
			A int
			D *inlineD `yaml:",inline"`
		}{A: 1, D: &inlineD{C: &inlineC{C: 3}, D: 4}},
		data: "a: 1\nc: 3\nd: 4\n",
	},

	// Map inlining
	{
		value: &struct {
			A int
			C map[string]int `yaml:",inline"`
		}{A: 1, C: map[string]int{"b": 2, "c": 3}},
		data: "a: 1\nb: 2\nc: 3\n",
	},

	// Duration
	{
		value: map[string]time.Duration{"a": 3 * time.Second},
		data:  "a: 3s\n",
	},

	// Issue #24: bug in map merging logic.
	{
		value: map[string]string{"a": "<foo>"},
		data:  "a: <foo>\n",
	},

	// Issue #34: marshal unsupported base 60 floats quoted for compatibility
	// with old YAML 1.1 parsers.
	{
		value: map[string]string{"a": "1:1"},
		data:  "a: \"1:1\"\n",
	},

	// Binary data.
	{
		value: map[string]string{"a": "\x00"},
		data:  "a: \"\\0\"\n",
	}, {
		value: map[string]string{"a": "\x80\x81\x82"},
		data:  "a: !!binary gIGC\n",
	}, {
		value: map[string]string{"a": strings.Repeat("\x90", 54)},
		data:  "a: !!binary |\n    " + strings.Repeat("kJCQ", 17) + "kJ\n    CQ\n",
	},

	// Encode unicode as utf-8 rather than in escaped form.
	{
		value: map[string]string{"a": "你好"},
		data:  "a: 你好\n",
	},

	// Support encoding.TextMarshaler.
	{
		value: map[string]net.IP{"a": net.IPv4(1, 2, 3, 4)},
		data:  "a: 1.2.3.4\n",
	},
	// time.Time gets a timestamp tag.
	{
		value: map[string]time.Time{"a": time.Date(2015, 2, 24, 18, 19, 39, 0, time.UTC)},
		data:  "a: 2015-02-24T18:19:39Z\n",
	},
	{
		value: map[string]*time.Time{"a": newTime(time.Date(2015, 2, 24, 18, 19, 39, 0, time.UTC))},
		data:  "a: 2015-02-24T18:19:39Z\n",
	},
	{
		// This is confirmed to be properly decoded in Python (libyaml) without a timestamp tag.
		value: map[string]time.Time{"a": time.Date(2015, 2, 24, 18, 19, 39, 123456789, time.FixedZone("FOO", -3*60*60))},
		data:  "a: 2015-02-24T18:19:39.123456789-03:00\n",
	},
	// Ensure timestamp-like strings are quoted.
	{
		value: map[string]string{"a": "2015-02-24T18:19:39Z"},
		data:  "a: \"2015-02-24T18:19:39Z\"\n",
	},

	// Ensure strings containing ": " are quoted (reported as PR #43, but not reproducible).
	{
		value: map[string]string{"a": "b: c"},
		data:  "a: 'b: c'\n",
	},

	// Containing hash mark ('#') in string should be quoted
	{
		value: map[string]string{"a": "Hello #comment"},
		data:  "a: 'Hello #comment'\n",
	},
	{
		value: map[string]string{"a": "你好 #comment"},
		data:  "a: '你好 #comment'\n",
	},

	// Ensure MarshalYAML also gets called on the result of MarshalYAML itself.
	{
		value: &marshalerType{value: marshalerType{value: true}},
		data:  "true\n",
	}, {
		value: &marshalerType{value: &marshalerType{value: true}},
		data:  "true\n",
	},

	// Check indentation of maps inside sequences inside maps.
	{
		value: map[string]interface{}{"a": map[string]interface{}{"b": []map[string]int{{"c": 1, "d": 2}}}},
		data:  "a:\n    b:\n        - c: 1\n          d: 2\n",
	},

	// Strings with tabs were disallowed as literals (issue #471).
	{
		value: map[string]string{"a": "\tB\n\tC\n"},
		data:  "a: |\n    \tB\n    \tC\n",
	},

	// Ensure that strings do not wrap
	{
		value: map[string]string{"a": "abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ 1234567890 abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ 1234567890 "},
		data:  "a: 'abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ 1234567890 abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ 1234567890 '\n",
	},

	// yaml.Node
	{
		value: &struct {
			Value yaml.Node
		}{
			Value: yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: "foo",
				Style: yaml.SingleQuotedStyle,
			},
		},
		data: "value: 'foo'\n",
	}, {
		value: yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "foo",
			Style: yaml.SingleQuotedStyle,
		},
		data: "'foo'\n",
	},

	// Enforced tagging with shorthand notation (issue #616).
	{
		value: &struct {
			Value yaml.Node
		}{
			Value: yaml.Node{
				Kind:  yaml.ScalarNode,
				Style: yaml.TaggedStyle,
				Value: "foo",
				Tag:   "!!str",
			},
		},
		data: "value: !!str foo\n",
	}, {
		value: &struct {
			Value yaml.Node
		}{
			Value: yaml.Node{
				Kind:  yaml.MappingNode,
				Style: yaml.TaggedStyle,
				Tag:   "!!map",
			},
		},
		data: "value: !!map {}\n",
	}, {
		value: &struct {
			Value yaml.Node
		}{
			Value: yaml.Node{
				Kind:  yaml.SequenceNode,
				Style: yaml.TaggedStyle,
				Tag:   "!!seq",
			},
		},
		data: "value: !!seq []\n",
	},
}

func TestMarshal(t *testing.T) {
	origTZ := os.Getenv("TZ")
	require.NoError(t, os.Setenv("TZ", "UTC"))
	for i, item := range marshalTests {
		t.Run(fmt.Sprintf("test %d: %q", i, item.data), func(t *testing.T) {
			b, err := yaml.Marshal(item.value)
			require.NoError(t, err)
			require.Equal(t, item.data, string(b))
		})
	}
	require.NoError(t, os.Setenv("TZ", origTZ))
}

func TestEncoderSingleDocument(t *testing.T) {
	for i, item := range marshalTests {
		t.Run(fmt.Sprintf("test %d: %q", i, item.data), func(t *testing.T) {
			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			err := enc.Encode(item.value)
			require.NoError(t, err)
			err = enc.Close()
			require.NoError(t, err)
			require.Equal(t, item.data, buf.String())
		})
	}
}

func TestEncoderMultipleDocuments(t *testing.T) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	err := enc.Encode(map[string]string{"a": "b"})
	require.NoError(t, err)
	err = enc.Encode(map[string]string{"c": "d"})
	require.NoError(t, err)
	err = enc.Close()
	require.NoError(t, err)
	require.Equal(t, "a: b\n---\nc: d\n", buf.String())
}

func TestEncoderWriteError(t *testing.T) {
	enc := yaml.NewEncoder(errorWriter{})
	err := enc.Encode(map[string]string{"a": "b"})
	require.EqualError(t, err, `yaml: write error: some write error`) // Data not flushed yet
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("some write error")
}

var marshalErrorTests = []struct {
	value interface{}
	error string
	panic string
}{{
	value: &struct {
		B       int
		inlineB ",inline"
	}{B: 1, inlineB: inlineB{B: 2, inlineC: inlineC{C: 3}}},
	panic: `duplicated key 'b' in struct struct \{ B int; .*`,
}, {
	value: &struct {
		A int
		B map[string]int ",inline"
	}{A: 1, B: map[string]int{"a": 2}},
	panic: `cannot have key "a" in inlined map: conflicts with struct field`,
}}

func TestMarshalErrors(t *testing.T) {
	for _, item := range marshalErrorTests {
		if item.panic != "" {
			func() {
				defer func() {
					r := recover()
					require.NotNil(t, r)
					require.Regexp(t, item.panic, r)
				}()
				_, err := yaml.Marshal(item.value)
				require.NoError(t, err)
			}()
		} else {
			_, err := yaml.Marshal(item.value)
			require.EqualError(t, err, item.error)
		}
	}
}

func TestMarshalTypeCache(t *testing.T) {
	var b []byte
	var err error
	func() {
		type T struct{ A int }
		b, err = yaml.Marshal(&T{})
		require.NoError(t, err)
	}()
	func() {
		type T struct{ B int }
		b, err = yaml.Marshal(&T{})
		require.NoError(t, err)
	}()
	require.Equal(t, "b: 0\n", string(b))
}

var marshalerTests = []struct {
	data  string
	value interface{}
}{
	{data: "_:\n    hi: there\n", value: map[interface{}]interface{}{"hi": "there"}},
	{data: "_:\n    - 1\n    - A\n", value: []interface{}{1, "A"}},
	{data: "_: 10\n", value: 10},
	{data: "_: null\n"},
	{data: "_: BAR!\n", value: "BAR!"},
}

type marshalerType struct {
	value interface{}
}

func (o marshalerType) MarshalText() ([]byte, error) {
	panic("MarshalText called on type with MarshalYAML")
}

func (o marshalerType) MarshalYAML() (interface{}, error) {
	return o.value, nil
}

type marshalerValue struct {
	Field marshalerType "_"
}

func TestMarshaler(t *testing.T) {
	for _, item := range marshalerTests {
		obj := &marshalerValue{}
		obj.Field.value = item.value
		b, err := yaml.Marshal(obj)
		require.NoError(t, err)
		require.Equal(t, item.data, string(b))
	}
}

func TestMarshalerWholeDocument(t *testing.T) {
	obj := &marshalerType{}
	obj.value = map[string]string{"hello": "world!"}
	b, err := yaml.Marshal(obj)
	require.NoError(t, err)
	require.Equal(t, "hello: world!\n", string(b))
}

type failingMarshaler struct{}

func (ft *failingMarshaler) MarshalYAML() (interface{}, error) {
	return nil, failingErr
}

func TestMarshalerError(t *testing.T) {
	_, err := yaml.Marshal(&failingMarshaler{})
	require.Equal(t, failingErr, err)
}

func TestSetIndent(t *testing.T) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(8)
	err := enc.Encode(map[string]interface{}{"a": map[string]interface{}{"b": map[string]string{"c": "d"}}})
	require.NoError(t, err)
	err = enc.Close()
	require.NoError(t, err)
	require.Equal(t, "a:\n        b:\n                c: d\n", buf.String())
}

func TestSortedOutput(t *testing.T) {
	order := []interface{}{
		false,
		true,
		1,
		uint(1),
		1.0,
		1.1,
		1.2,
		2,
		uint(2),
		2.0,
		2.1,
		"",
		".1",
		".2",
		".a",
		"1",
		"2",
		"a!10",
		"a/0001",
		"a/002",
		"a/3",
		"a/10",
		"a/11",
		"a/0012",
		"a/100",
		"a~10",
		"ab/1",
		"b/1",
		"b/01",
		"b/2",
		"b/02",
		"b/3",
		"b/03",
		"b1",
		"b01",
		"b3",
		"c2.10",
		"c10.2",
		"d1",
		"d7",
		"d7abc",
		"d12",
		"d12a",
		"e2b",
		"e4b",
		"e21a",
	}
	m := make(map[interface{}]int)
	for _, k := range order {
		m[k] = 1
	}
	b, err := yaml.Marshal(m)
	require.NoError(t, err)
	out := "\n" + string(b)
	last := 0
	for i, k := range order {
		repr := fmt.Sprint(k)
		if s, ok := k.(string); ok {
			if _, err = strconv.ParseFloat(repr, 32); s == "" || err == nil {
				repr = `"` + repr + `"`
			}
		}
		index := strings.Index(out, "\n"+repr+":")
		if index == -1 {
			t.Fatalf("%#v is not in the output: %#v", k, out)
		}
		if index < last {
			t.Fatalf("%#v was generated before %#v: %q", k, order[i-1], out)
		}
		last = index
	}
}

func newTime(t time.Time) *time.Time {
	return &t
}
