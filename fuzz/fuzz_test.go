package fuzz

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/willabides/yaml"
	yamlv3 "gopkg.in/yaml.v3"
)

var testData = []string{
	`{}`,
	`v: hi`,
	`v: true`,
	`v: 10`,
	`v: 0b10`,
	`v: 0xA`,
	`v: 4294967296`,
	`v: 0.1`,
	`v: .1`,
	`v: .Inf`,
	`v: -.Inf`,
	`v: -10`,
	`v: -.1`,
	`123`,
	`canonical: 6.8523e+5`,
	`expo: 685.230_15e+03`,
	`fixed: 685_230.15`,
	`neginf: -.inf`,
	`fixed: 685_230.15`,
	`empty:`,
	`canonical: ~`,
	`english: null`,
	`~: null key`,
	`empty:`,
	`seq: [A,B]`,
	`seq: [A,B,C,]`,
	`seq: [A,1,C]`,
	"seq:\n - A\n - B",
	"seq:\n - A\n - B\n - C",
	"seq:\n - A\n - 1\n - C",
	"scalar: | # Comment\n\n literal\n\n \ttext\n\n",
	"scalar: > # Comment\n\n folded\n line\n \n next\n line\n  * one\n  * two\n\n last\n line\n\n",
	"a: {b: c}",
	"a: {b: c, 1: d}",
	"a: [b,c,d]",
	"int_max: 2147483647",
	"int_min: -2147483648",
	"int_overflow: 9223372036854775808",
	"int_underflow: -9223372036854775809",
	"int64_max: 9223372036854775807",
	"int64_min: -9223372036854775808",
	"int64_overflow: 9223372036854775808",
	"int64_underflow: -9223372036854775809",
	"'1': '\"2\"'",
	"v:\n- A\n- 'B\n\n  C'\n",
	"v: !!float '1.1'",
	"v: !!float 0",
	"v: !!float -1",
	"v: !!null ''",
	"%TAG !y! tag:yaml.org,2002:\n---\nv: !y!int '1'",
	"v: ! test",
	"a: &x 1\nb: &y 2\nc: *x\nd: *y\n",
	"a: &a {c: 1}\nb: *a",
	"a: &a [1, 2]\nb: *a",
	"foo: ''",
	"foo: null",
	"" +
		"%YAML 1.1\n" +
		"--- !!str\n" +
		`"Generic line break (no glyph)\n\` + "\n" +
		` Generic line break (glyphed)\n\` + "\n" +
		` Line separator\u2028\` + "\n" +
		` Paragraph separator\u2029"` + "\n",
	"a: {b: https://github.com/go-yaml/yaml}",
	"a: [https://github.com/go-yaml/yaml]",
	"a: 3s",
	"a: <foo>",
	"a: 1:1\n",
	"a: !!binary gIGC\n",
	"a: !!binary |\n  " + strings.Repeat("kJCQ", 17) + "kJ\n  CQ\n",
	"a: !!binary |\n  " + strings.Repeat("A", 70) + "\n  ==\n",
	"a: 2015-01-01\n",
	"a: 2015-02-24T18:19:39.12Z\n",
	"a: 2015-2-3T3:4:5Z",
	"a: 2015-02-24t18:19:39Z\n",
	"a: 2015-02-24 18:19:39\n",
	"a: !!str 2015-01-01",
	"a: !!timestamp \"2015-01-01\"",
	"a: !!timestamp 2015-01-01",
	"a: \"2015-01-01\"",
	"\xff\xfe\xf1\x00o\x00\xf1\x00o\x00:\x00 \x00v\x00e\x00r\x00y\x00 \x00y\x00e\x00s\x00\n\x00",
	"\xff\xfe\xf1\x00o\x00\xf1\x00o\x00:\x00 \x00v\x00e\x00r\x00y\x00 \x00y\x00e\x00s\x00 \x00=\xd8\xd4\xdf\n\x00",
	"\xfe\xff\x00\xf1\x00o\x00\xf1\x00o\x00:\x00 \x00v\x00e\x00r\x00y\x00 \x00y\x00e\x00s\x00\n",
	"\xfe\xff\x00\xf1\x00o\x00\xf1\x00o\x00:\x00 \x00v\x00e\x00r\x00y\x00 \x00y\x00e\x00s\x00 \xd8=\xdf\xd4\x00\n",
	"a: 123456e1\n",
	"a: 123456E1\n",
	"First occurrence: &anchor Foo\nSecond occurrence: *anchor\nOverride anchor: &anchor Bar\nReuse anchor: *anchor\n",
	"---\nhello\n...\n}not yaml",
	"true\n#" + strings.Repeat(" ", 512*3),
	"true #" + strings.Repeat(" ", 512*3),
	"a: b\r\nc:\r\n- d\r\n- e\r\n",
	"\n0:\n<<:\n  {}:\n",
}

type M map[string]interface{}

type exStruct struct {
	V       any               `yaml:"v"`
	Foo     string            `yaml:"foo"`
	Bar2    int               `yaml:"bar"`
	FlowMap map[string]string `yaml:"flow_map,flow"`
	Self    *exStruct         `yaml:"self,flow"`
}

type obsoleteUnmarshaler struct {
	Val string
}

func (m obsoleteUnmarshaler) MarshalYAML() (interface{}, error) {
	return m.Val, nil
}

func (m *obsoleteUnmarshaler) UnmarshalYAML(f func(interface{}) error) error {
	return f(&m.Val)
}

type errMarshaler struct {
	Foo string `yaml:"foo"`
}

type textMarshaler struct {
	val string
}

func (m textMarshaler) MarshalText() ([]byte, error) {
	return []byte(m.val), nil
}

func (m *textMarshaler) UnmarshalText(text []byte) error {
	m.val = string(text)
	return nil
}

func (m errMarshaler) MarshalYAML() (interface{}, error) {
	return nil, fmt.Errorf("marshal error")
}

type marshaler struct {
	val any
}

func (m marshaler) MarshalYAML() (interface{}, error) {
	return m.val, nil
}

func (m *marshaler) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&m.val)
}

type v3marshaler struct {
	val any
}

func (m v3marshaler) MarshalYAML() (interface{}, error) {
	return m.val, nil
}

func (m *v3marshaler) UnmarshalYAML(node *yaml.Node) error {
	return node.Decode(&m.val)
}

func FuzzRoundTripCompatibility(f *testing.F) {
	for _, s := range testData {
		f.Add(s)
	}
	f.Fuzz(testRoundTrip)
}

func testRoundTrip(t *testing.T, data string) {
	t.Helper()
	typedRoundTripCompatibility[any](t, data)
	typedRoundTripCompatibility[map[string]string](t, data)
	typedRoundTripCompatibility[map[any]any](t, data)
	typedRoundTripCompatibility[map[string]any](t, data)
	typedRoundTripCompatibility[map[any]map[any]map[any]any](t, data)
	typedRoundTripCompatibility[[]any](t, data)
	typedRoundTripCompatibility[[]int64](t, data)
	typedRoundTripCompatibility[[]int8](t, data)
	typedRoundTripCompatibility[[]uint8](t, data)
	typedRoundTripCompatibility[uint](t, data)
	typedRoundTripCompatibility[string](t, data)
	typedRoundTripCompatibility[float64](t, data)
	typedRoundTripCompatibility[map[string]float64](t, data)
	typedRoundTripCompatibility[float32](t, data)
	typedRoundTripCompatibility[bool](t, data)
	typedRoundTripCompatibility[map[string]bool](t, data)
	typedRoundTripCompatibility[map[string][]string](t, data)
	typedRoundTripCompatibility[map[string][]int](t, data)
	typedRoundTripCompatibility[exStruct](t, data)
	typedRoundTripCompatibility[*exStruct](t, data)
	typedRoundTripCompatibility[map[string]*exStruct](t, data)
	typedRoundTripCompatibility[obsoleteUnmarshaler](t, data)
	typedRoundTripCompatibility[*obsoleteUnmarshaler](t, data)
	typedRoundTripCompatibility[map[string]obsoleteUnmarshaler](t, data)
	typedRoundTripCompatibility[map[string]*obsoleteUnmarshaler](t, data)
	typedRoundTripCompatibility[map[string][]obsoleteUnmarshaler](t, data)
	typedRoundTripCompatibility[map[string][]*obsoleteUnmarshaler](t, data)
	typedRoundTripCompatibility[map[string]errMarshaler](t, data)
	typedRoundTripCompatibility[M](t, data)
	typedRoundTripCompatibility[errMarshaler](t, data)
	typedRoundTripCompatibility[*textMarshaler](t, data)
	typedRoundTripCompatibility[map[string]*textMarshaler](t, data)
	typedRoundTripCompatibility[map[string]textMarshaler](t, data)
	roundTripCompatibility(t, data, yaml.Node{}, yamlv3.Node{})
	roundTripCompatibility(t, data, marshaler{}, v3marshaler{})
	roundTripCompatibility(t, data, &marshaler{}, &v3marshaler{})
}

func typedRoundTripCompatibility[V any](t *testing.T, data string) {
	t.Helper()
	var val, v3Val V
	roundTripCompatibility(t, data, val, v3Val)
}

func assertUnmarshalErr(t testing.TB, v3err, err error) {
	t.Helper()
	if v3err == nil {
		require.NoError(t, err)
		return
	}
	require.Error(t, err)
	v3msg := v3err.Error()
	msg := err.Error()
	// deal with inconsistent error messages
	// these are found by fuzzing and checking that the error message is ok when it crashes
	okMsgs := map[string][]string{
		"not find expected": {
			"not find expected",
			"invalid leading UTF-8 octet",
			"invalid trailing UTF-8 octet",
			"control characters are not allowed",
			"found unexpected end of stream",
		},
		"found character that cannot start any token": {
			"incomplete UTF-8 octet sequence",
		},
	}
	for k := range okMsgs {
		if !strings.Contains(v3msg, k) {
			continue
		}
		for _, okMsg := range okMsgs[k] {
			if strings.Contains(msg, okMsg) {
				return
			}
		}
	}
	require.EqualValues(t, v3err, err)
}

func roundTripCompatibility(t *testing.T, data string, val, v3Val any) {
	t.Helper()
	var err, v3err error
	v3recovered := capturePanic(func() {
		v3err = yamlv3.Unmarshal([]byte(data), &v3Val)
	})
	recovered := capturePanic(func() {
		err = yaml.Unmarshal([]byte(data), &val)
	})
	// fail on our panic no matter what
	require.Nil(t, recovered)
	// don't continue if v3 panicked
	if v3recovered != nil {
		return
	}
	assertUnmarshalErr(t, v3err, err)
	// compare values only if val and v3val are the same type
	if reflect.TypeOf(val) == reflect.TypeOf(v3Val) {
		require.Equal(t, v3Val, val)
	}
	v3marshalled, v3err := yamlv3.Marshal(v3Val)
	marshalled, err := yaml.Marshal(val)
	if v3err != nil {
		require.Errorf(t, err, "v3 error: %v", v3err)
		return
	}
	require.NoError(t, err)
	require.Equal(t, string(v3marshalled), string(marshalled))
}

// capturePanic runs fn and returns false and the recovered value if fn panics
func capturePanic(fn func()) (recovered any) {
	defer func() {
		recovered = recover()
	}()
	fn()
	return nil
}
