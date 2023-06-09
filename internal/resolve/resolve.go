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

package resolve

import (
	"encoding/base64"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type resolveMapItem struct {
	value interface{}
	tag   string
}

var (
	resolveTable = make([]byte, 256)
	resolveMap   = make(map[string]resolveMapItem)
)

var initResolveOnce sync.Once

func initResolve() {
	t := resolveTable
	t[int('+')] = 'S' // Sign
	t[int('-')] = 'S'
	for _, c := range "0123456789" {
		t[int(c)] = 'D' // Digit
	}
	for _, c := range "yYnNtTfFoO~" {
		t[int(c)] = 'M' // In map
	}
	t[int('.')] = '.' // Float (potentially in map)

	resolveMapList := []struct {
		v   interface{}
		tag string
		l   []string
	}{
		{v: true, tag: BoolTag, l: []string{"true", "True", "TRUE"}},
		{v: false, tag: BoolTag, l: []string{"false", "False", "FALSE"}},
		{tag: NullTag, l: []string{"", "~", "null", "Null", "NULL"}},
		{v: math.NaN(), tag: FloatTag, l: []string{".nan", ".NaN", ".NAN"}},
		{v: math.Inf(+1), tag: FloatTag, l: []string{".inf", ".Inf", ".INF"}},
		{v: math.Inf(+1), tag: FloatTag, l: []string{"+.inf", "+.Inf", "+.INF"}},
		{v: math.Inf(-1), tag: FloatTag, l: []string{"-.inf", "-.Inf", "-.INF"}},
		{v: "<<", tag: MergeTag, l: []string{"<<"}},
	}

	m := resolveMap
	for _, item := range resolveMapList {
		for _, s := range item.l {
			m[s] = resolveMapItem{value: item.v, tag: item.tag}
		}
	}
}

const (
	NullTag      = "!!null"
	BoolTag      = "!!bool"
	StrTag       = "!!str"
	IntTag       = "!!int"
	FloatTag     = "!!float"
	TimestampTag = "!!timestamp"
	SeqTag       = "!!seq"
	MapTag       = "!!map"
	BinaryTag    = "!!binary"
	MergeTag     = "!!merge"
)

var (
	longTags  = make(map[string]string)
	shortTags = make(map[string]string)
)

const longTagPrefix = "tag:yaml.org,2002:"

func ShortTag(tag string) string {
	if strings.HasPrefix(tag, longTagPrefix) {
		if stag, ok := shortTags[tag]; ok {
			return stag
		}
		shortTags[tag] = "!!" + tag[len(longTagPrefix):]
		return shortTags[tag]
	}
	return tag
}

func LongTag(tag string) string {
	if strings.HasPrefix(tag, "!!") {
		if ltag, ok := longTags[tag]; ok {
			return ltag
		}
		longTags[tag] = longTagPrefix + tag[2:]
		return longTags[tag]
	}
	return tag
}

func resolvableTag(tag string) bool {
	switch tag {
	case "", StrTag, BoolTag, IntTag, FloatTag, NullTag, TimestampTag:
		return true
	}
	return false
}

var yamlStyleFloat = regexp.MustCompile(`^[-+]?(\.\d+|\d+(\.\d*)?)([eE][-+]?\d+)?$`)

//nolint:gocyclo // TODO: reduce cyclomatic complexity
func Resolve(tag, in string) (rtag string, out interface{}, errOut error) {
	initResolveOnce.Do(initResolve)
	tag = ShortTag(tag)
	if !resolvableTag(tag) {
		return tag, in, nil
	}

	defer func() {
		switch tag {
		case "", rtag, StrTag, BinaryTag:
			return
		case FloatTag:
			if rtag == IntTag {
				switch v := out.(type) {
				case int64:
					rtag = FloatTag
					out = float64(v)
					return
				case int:
					rtag = FloatTag
					out = float64(v)
					return
				}
			}
		}
		errOut = fmt.Errorf("yaml: cannot decode %s `%s` as a %s", ShortTag(rtag), in, ShortTag(tag))
	}()

	// Any data is accepted as a !!str or !!binary.
	// Otherwise, the prefix is enough of a hint about what it might be.
	hint := byte('N')
	if in != "" {
		hint = resolveTable[in[0]]
	}
	if hint != 0 && tag != StrTag && tag != BinaryTag {
		// Handle things we can lookup in a map.
		if item, ok := resolveMap[in]; ok {
			return item.tag, item.value, nil
		}

		// Base 60 floats are a bad idea, were dropped in YAML 1.2, and
		// are purposefully unsupported here. They're still quoted on
		// the way out for compatibility with other parser, though.

		switch hint {
		case 'M':
			// We've already checked the map above.

		case '.':
			// Not in the map, so maybe a normal float.
			floatv, err := strconv.ParseFloat(in, 64)
			if err == nil {
				return FloatTag, floatv, nil
			}

		case 'D', 'S':
			// Int, float, or timestamp.
			// Only try values as a timestamp if the value is unquoted or there's an explicit
			// !!timestamp tag.
			if tag == "" || tag == TimestampTag {
				t, ok := parseTimestamp(in)
				if ok {
					return TimestampTag, t, nil
				}
			}

			plain := strings.ReplaceAll(in, "_", "")
			intv, err := strconv.ParseInt(plain, 0, 64)
			if err == nil {
				if intv == int64(int(intv)) {
					return IntTag, int(intv), nil
				} else {
					return IntTag, intv, nil
				}
			}
			uintv, err := strconv.ParseUint(plain, 0, 64)
			if err == nil {
				return IntTag, uintv, nil
			}
			if yamlStyleFloat.MatchString(plain) {
				floatv, err := strconv.ParseFloat(plain, 64)
				if err == nil {
					return FloatTag, floatv, nil
				}
			}
			if strings.HasPrefix(plain, "0b") {
				intv, err := strconv.ParseInt(plain[2:], 2, 64)
				if err == nil {
					if intv == int64(int(intv)) {
						return IntTag, int(intv), nil
					} else {
						return IntTag, intv, nil
					}
				}
				uintv, err := strconv.ParseUint(plain[2:], 2, 64)
				if err == nil {
					return IntTag, uintv, nil
				}
			} else if strings.HasPrefix(plain, "-0b") {
				intv, err := strconv.ParseInt("-"+plain[3:], 2, 64)
				if err == nil {
					if true || intv == int64(int(intv)) {
						return IntTag, int(intv), nil
					} else {
						return IntTag, intv, nil
					}
				}
			}
			// Octals as introduced in version 1.2 of the spec.
			// Octals from the 1.1 spec, spelled as 0777, are still
			// decoded by default in v3 as well for compatibility.
			// May be dropped in v4 depending on how usage evolves.
			if strings.HasPrefix(plain, "0o") {
				intv, err := strconv.ParseInt(plain[2:], 8, 64)
				if err == nil {
					if intv == int64(int(intv)) {
						return IntTag, int(intv), nil
					} else {
						return IntTag, intv, nil
					}
				}
				uintv, err := strconv.ParseUint(plain[2:], 8, 64)
				if err == nil {
					return IntTag, uintv, nil
				}
			} else if strings.HasPrefix(plain, "-0o") {
				intv, err := strconv.ParseInt("-"+plain[3:], 8, 64)
				if err == nil {
					if true || intv == int64(int(intv)) {
						return IntTag, int(intv), nil
					} else {
						return IntTag, intv, nil
					}
				}
			}
		default:
			panic("internal error: missing handler for resolver table: " + string(rune(hint)) + " (with " + in + ")")
		}
	}
	return StrTag, in, nil
}

// EncodeBase64 encodes s as base64 that is broken up into multiple lines
// as appropriate for the resulting length.
func EncodeBase64(s string) string {
	const lineLen = 70
	encLen := base64.StdEncoding.EncodedLen(len(s))
	lines := encLen/lineLen + 1
	buf := make([]byte, encLen*2+lines)
	in := buf[0:encLen]
	out := buf[encLen:]
	base64.StdEncoding.Encode(in, []byte(s))
	k := 0
	for i := 0; i < len(in); i += lineLen {
		j := i + lineLen
		if j > len(in) {
			j = len(in)
		}
		k += copy(out[k:], in[i:j])
		if lines > 1 {
			out[k] = '\n'
			k++
		}
	}
	return string(out[:k])
}

// This is a subset of the formats allowed by the regular expression
// defined at http://yaml.org/type/timestamp.html.
var allowedTimestampFormats = []string{
	"2006-1-2T15:4:5.999999999Z07:00", // RCF3339Nano with short date fields.
	"2006-1-2t15:4:5.999999999Z07:00", // RFC3339Nano with short date fields and lower-case "t".
	"2006-1-2 15:4:5.999999999",       // space separated with no time zone
	"2006-1-2",                        // date only
	// Notable exception: time.Parse cannot handle: "2001-12-14 21:59:43.10 -5"
	// from the set of examples.
}

// parseTimestamp parses s as a timestamp string and
// returns the timestamp and reports whether it succeeded.
// Timestamp formats are defined at http://yaml.org/type/timestamp.html
func parseTimestamp(s string) (time.Time, bool) {
	// TODO write code to check all the formats supported by
	// http://yaml.org/type/timestamp.html instead of using time.Parse.

	// Quick check: all date formats start with YYYY-.
	i := 0
	for ; i < len(s); i++ {
		if c := s[i]; c < '0' || c > '9' {
			break
		}
	}
	if i != 4 || i == len(s) || s[i] != '-' {
		return time.Time{}, false
	}
	for _, format := range allowedTimestampFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
