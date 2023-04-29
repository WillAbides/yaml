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
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/willabides/go-yaml"
)

var nodeTests = []struct {
	yaml string
	node yaml.Node
}{
	{
		yaml: "null\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "null",
				Tag:    "!!null",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "[encode]null\n",
	}, {
		yaml: "foo\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "foo",
				Tag:    "!!str",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "\"foo\"\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Style:  yaml.DoubleQuotedStyle,
				Value:  "foo",
				Tag:    "!!str",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "'foo'\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Style:  yaml.SingleQuotedStyle,
				Value:  "foo",
				Tag:    "!!str",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "!!str 123\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Style:  yaml.TaggedStyle,
				Value:  "123",
				Tag:    "!!str",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		// Although the node isn't TaggedStyle, dropping the tag would change the value.
		yaml: "[encode]!!binary gIGC\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "gIGC",
				Tag:    "!!binary",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		// Item doesn't have a tag, but needs to be binary encoded due to its content.
		yaml: "[encode]!!binary gIGC\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "\x80\x81\x82",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		// Same, but with strings we can just quote them.
		yaml: "[encode]\"123\"\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "123",
				Tag:    "!!str",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "!tag:something 123\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Style:  yaml.TaggedStyle,
				Value:  "123",
				Tag:    "!tag:something",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "[encode]!tag:something 123\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "123",
				Tag:    "!tag:something",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "!tag:something {}\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Style:  yaml.TaggedStyle | yaml.FlowStyle,
				Tag:    "!tag:something",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "[encode]!tag:something {}\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Style:  yaml.FlowStyle,
				Tag:    "!tag:something",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "!tag:something []\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Style:  yaml.TaggedStyle | yaml.FlowStyle,
				Tag:    "!tag:something",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "[encode]!tag:something []\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Style:  yaml.FlowStyle,
				Tag:    "!tag:something",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "''\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Style:  yaml.SingleQuotedStyle,
				Value:  "",
				Tag:    "!!str",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "|\n  foo\n  bar\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Style:  yaml.LiteralStyle,
				Value:  "foo\nbar\n",
				Tag:    "!!str",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "true\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "true",
				Tag:    "!!bool",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "-10\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "-10",
				Tag:    "!!int",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "4294967296\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "4294967296",
				Tag:    "!!int",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "0.1000\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "0.1000",
				Tag:    "!!float",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "-.inf\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  "-.inf",
				Tag:    "!!float",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: ".nan\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.ScalarNode,
				Value:  ".nan",
				Tag:    "!!float",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "{}\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Style:  yaml.FlowStyle,
				Value:  "",
				Tag:    "!!map",
				Line:   1,
				Column: 1,
			}},
		},
	}, {
		yaml: "a: b c\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Value:  "",
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Value:  "a",
					Tag:    "!!str",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.ScalarNode,
					Value:  "b c",
					Tag:    "!!str",
					Line:   1,
					Column: 4,
				}},
			}},
		},
	}, {
		yaml: "a:\n  b: c\n  d: e\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Value:  "a",
					Tag:    "!!str",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Value:  "b",
						Tag:    "!!str",
						Line:   2,
						Column: 3,
					}, {
						Kind:   yaml.ScalarNode,
						Value:  "c",
						Tag:    "!!str",
						Line:   2,
						Column: 6,
					}, {
						Kind:   yaml.ScalarNode,
						Value:  "d",
						Tag:    "!!str",
						Line:   3,
						Column: 3,
					}, {
						Kind:   yaml.ScalarNode,
						Value:  "e",
						Tag:    "!!str",
						Line:   3,
						Column: 6,
					}},
				}},
			}},
		},
	}, {
		yaml: "a:\n  - b: c\n    d: e\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Value:  "a",
					Tag:    "!!str",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.SequenceNode,
					Tag:    "!!seq",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.MappingNode,
						Tag:    "!!map",
						Line:   2,
						Column: 5,
						Content: []*yaml.Node{{
							Kind:   yaml.ScalarNode,
							Value:  "b",
							Tag:    "!!str",
							Line:   2,
							Column: 5,
						}, {
							Kind:   yaml.ScalarNode,
							Value:  "c",
							Tag:    "!!str",
							Line:   2,
							Column: 8,
						}, {
							Kind:   yaml.ScalarNode,
							Value:  "d",
							Tag:    "!!str",
							Line:   3,
							Column: 5,
						}, {
							Kind:   yaml.ScalarNode,
							Value:  "e",
							Tag:    "!!str",
							Line:   3,
							Column: 8,
						}},
					}},
				}},
			}},
		},
	}, {
		yaml: "a: # AI\n  - b\nc:\n  - d\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "a",
					LineComment: "# AI",
					Line:        1,
					Column:      1,
				}, {
					Kind: yaml.SequenceNode,
					Tag:  "!!seq",
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "b",
						Line:   2,
						Column: 5,
					}},
					Line:   2,
					Column: 3,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "c",
					Line:   3,
					Column: 1,
				}, {
					Kind: yaml.SequenceNode,
					Tag:  "!!seq",
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "d",
						Line:   4,
						Column: 5,
					}},
					Line:   4,
					Column: 3,
				}},
			}},
		},
	}, {
		yaml: "[decode]a:\n  # HM\n  - # HB1\n    # HB2\n    b: # IB\n      c # IC\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Style:  0x0,
					Tag:    "!!str",
					Value:  "a",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.SequenceNode,
					Tag:    "!!seq",
					Line:   3,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.MappingNode,
						Tag:         "!!map",
						HeadComment: "# HM",
						Line:        5,
						Column:      5,
						Content: []*yaml.Node{{
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "b",
							HeadComment: "# HB1\n# HB2",
							LineComment: "# IB",
							Line:        5,
							Column:      5,
						}, {
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "c",
							LineComment: "# IC",
							Line:        6,
							Column:      7,
						}},
					}},
				}},
			}},
		},
	}, {
		// When encoding the value above, it loses b's inline comment.
		yaml: "[encode]a:\n  # HM\n  - # HB1\n    # HB2\n    b: c # IC\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Style:  0x0,
					Tag:    "!!str",
					Value:  "a",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.SequenceNode,
					Tag:    "!!seq",
					Line:   3,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.MappingNode,
						Tag:         "!!map",
						HeadComment: "# HM",
						Line:        5,
						Column:      5,
						Content: []*yaml.Node{{
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "b",
							HeadComment: "# HB1\n# HB2",
							LineComment: "# IB",
							Line:        5,
							Column:      5,
						}, {
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "c",
							LineComment: "# IC",
							Line:        6,
							Column:      7,
						}},
					}},
				}},
			}},
		},
	}, {
		// Multiple cases of comment inlining next to mapping keys.
		yaml: "a: | # IA\n  str\nb: >- # IB\n  str\nc: # IC\n  - str\nd: # ID\n  str:\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "a",
					Line:   1,
					Column: 1,
				}, {
					Kind:        yaml.ScalarNode,
					Style:       yaml.LiteralStyle,
					Tag:         "!!str",
					Value:       "str\n",
					LineComment: "# IA",
					Line:        1,
					Column:      4,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "b",
					Line:   3,
					Column: 1,
				}, {
					Kind:        yaml.ScalarNode,
					Style:       yaml.FoldedStyle,
					Tag:         "!!str",
					Value:       "str",
					LineComment: "# IB",
					Line:        3,
					Column:      4,
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "c",
					LineComment: "# IC",
					Line:        5,
					Column:      1,
				}, {
					Kind:   yaml.SequenceNode,
					Tag:    "!!seq",
					Line:   6,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "str",
						Line:   6,
						Column: 5,
					}},
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "d",
					LineComment: "# ID",
					Line:        7,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   8,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "str",
						Line:   8,
						Column: 3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!null",
						Line:   8,
						Column: 7,
					}},
				}},
			}},
		},
	}, {
		// Indentless sequence.
		yaml: "[decode]a:\n# HM\n- # HB1\n  # HB2\n  b: # IB\n    c # IC\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "a",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.SequenceNode,
					Tag:    "!!seq",
					Line:   3,
					Column: 1,
					Content: []*yaml.Node{{
						Kind:        yaml.MappingNode,
						Tag:         "!!map",
						HeadComment: "# HM",
						Line:        5,
						Column:      3,
						Content: []*yaml.Node{{
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "b",
							HeadComment: "# HB1\n# HB2",
							LineComment: "# IB",
							Line:        5,
							Column:      3,
						}, {
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "c",
							LineComment: "# IC",
							Line:        6,
							Column:      5,
						}},
					}},
				}},
			}},
		},
	}, {
		yaml: "- a\n- b\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Value:  "",
				Tag:    "!!seq",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Value:  "a",
					Tag:    "!!str",
					Line:   1,
					Column: 3,
				}, {
					Kind:   yaml.ScalarNode,
					Value:  "b",
					Tag:    "!!str",
					Line:   2,
					Column: 3,
				}},
			}},
		},
	}, {
		yaml: "- a\n- - b\n  - c\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Tag:    "!!seq",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Value:  "a",
					Tag:    "!!str",
					Line:   1,
					Column: 3,
				}, {
					Kind:   yaml.SequenceNode,
					Tag:    "!!seq",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Value:  "b",
						Tag:    "!!str",
						Line:   2,
						Column: 5,
					}, {
						Kind:   yaml.ScalarNode,
						Value:  "c",
						Tag:    "!!str",
						Line:   3,
						Column: 5,
					}},
				}},
			}},
		},
	}, {
		yaml: "[a, b]\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Style:  yaml.FlowStyle,
				Value:  "",
				Tag:    "!!seq",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Value:  "a",
					Tag:    "!!str",
					Line:   1,
					Column: 2,
				}, {
					Kind:   yaml.ScalarNode,
					Value:  "b",
					Tag:    "!!str",
					Line:   1,
					Column: 5,
				}},
			}},
		},
	}, {
		yaml: "- a\n- [b, c]\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Tag:    "!!seq",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Value:  "a",
					Tag:    "!!str",
					Line:   1,
					Column: 3,
				}, {
					Kind:   yaml.SequenceNode,
					Tag:    "!!seq",
					Style:  yaml.FlowStyle,
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Value:  "b",
						Tag:    "!!str",
						Line:   2,
						Column: 4,
					}, {
						Kind:   yaml.ScalarNode,
						Value:  "c",
						Tag:    "!!str",
						Line:   2,
						Column: 7,
					}},
				}},
			}},
		},
	}, {
		yaml: "a: &x 1\nb: &y 2\nc: *x\nd: *y\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Line:   1,
				Column: 1,
				Tag:    "!!map",
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Value:  "a",
					Tag:    "!!str",
					Line:   1,
					Column: 1,
				},
					saveNode("x", &yaml.Node{
						Kind:   yaml.ScalarNode,
						Value:  "1",
						Tag:    "!!int",
						Anchor: "x",
						Line:   1,
						Column: 4,
					}),
					{
						Kind:   yaml.ScalarNode,
						Value:  "b",
						Tag:    "!!str",
						Line:   2,
						Column: 1,
					},
					saveNode("y", &yaml.Node{
						Kind:   yaml.ScalarNode,
						Value:  "2",
						Tag:    "!!int",
						Anchor: "y",
						Line:   2,
						Column: 4,
					}),
					{
						Kind:   yaml.ScalarNode,
						Value:  "c",
						Tag:    "!!str",
						Line:   3,
						Column: 1,
					}, {
						Kind:   yaml.AliasNode,
						Value:  "x",
						Alias:  dropNode("x"),
						Line:   3,
						Column: 4,
					}, {
						Kind:   yaml.ScalarNode,
						Value:  "d",
						Tag:    "!!str",
						Line:   4,
						Column: 1,
					}, {
						Kind:   yaml.AliasNode,
						Value:  "y",
						Tag:    "",
						Alias:  dropNode("y"),
						Line:   4,
						Column: 4,
					}},
			}},
		},
	}, {

		yaml: "# One\n# Two\ntrue # Three\n# Four\n# Five\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   3,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:        yaml.ScalarNode,
				Value:       "true",
				Tag:         "!!bool",
				Line:        3,
				Column:      1,
				HeadComment: "# One\n# Two",
				LineComment: "# Three",
				FootComment: "# Four\n# Five",
			}},
		},
	}, {

		yaml: "# š\ntrue # š\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:        yaml.ScalarNode,
				Value:       "true",
				Tag:         "!!bool",
				Line:        2,
				Column:      1,
				HeadComment: "# š",
				LineComment: "# š",
			}},
		},
	}, {

		yaml: "[decode]\n# One\n\n# Two\n\n# Three\ntrue # Four\n# Five\n\n# Six\n\n# Seven\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        7,
			Column:      1,
			HeadComment: "# One\n\n# Two",
			FootComment: "# Six\n\n# Seven",
			Content: []*yaml.Node{{
				Kind:        yaml.ScalarNode,
				Value:       "true",
				Tag:         "!!bool",
				Line:        7,
				Column:      1,
				HeadComment: "# Three",
				LineComment: "# Four",
				FootComment: "# Five",
			}},
		},
	}, {
		// Write out the pound character if missing from comments.
		yaml: "[encode]# One\n# Two\ntrue # Three\n# Four\n# Five\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   3,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:        yaml.ScalarNode,
				Value:       "true",
				Tag:         "!!bool",
				Line:        3,
				Column:      1,
				HeadComment: "One\nTwo\n",
				LineComment: "Three\n",
				FootComment: "Four\nFive\n",
			}},
		},
	}, {
		yaml: "[encode]#   One\n#   Two\ntrue #   Three\n#   Four\n#   Five\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   3,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:        yaml.ScalarNode,
				Value:       "true",
				Tag:         "!!bool",
				Line:        3,
				Column:      1,
				HeadComment: "  One\n  Two",
				LineComment: "  Three",
				FootComment: "  Four\n  Five",
			}},
		},
	}, {
		yaml: "# DH1\n\n# DH2\n\n# H1\n# H2\ntrue # I\n# F1\n# F2\n\n# DF1\n\n# DF2\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        7,
			Column:      1,
			HeadComment: "# DH1\n\n# DH2",
			FootComment: "# DF1\n\n# DF2",
			Content: []*yaml.Node{{
				Kind:        yaml.ScalarNode,
				Value:       "true",
				Tag:         "!!bool",
				Line:        7,
				Column:      1,
				HeadComment: "# H1\n# H2",
				LineComment: "# I",
				FootComment: "# F1\n# F2",
			}},
		},
	}, {
		yaml: "# DH1\n\n# DH2\n\n# HA1\n# HA2\nka: va # IA\n# FA1\n# FA2\n\n# HB1\n# HB2\nkb: vb # IB\n# FB1\n# FB2\n\n# DF1\n\n# DF2\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        7,
			Column:      1,
			HeadComment: "# DH1\n\n# DH2",
			FootComment: "# DF1\n\n# DF2",
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   7,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Line:        7,
					Column:      1,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1\n# HA2",
					FootComment: "# FA1\n# FA2",
				}, {
					Kind:        yaml.ScalarNode,
					Line:        7,
					Column:      5,
					Tag:         "!!str",
					Value:       "va",
					LineComment: "# IA",
				}, {
					Kind:        yaml.ScalarNode,
					Line:        13,
					Column:      1,
					Tag:         "!!str",
					Value:       "kb",
					HeadComment: "# HB1\n# HB2",
					FootComment: "# FB1\n# FB2",
				}, {
					Kind:        yaml.ScalarNode,
					Line:        13,
					Column:      5,
					Tag:         "!!str",
					Value:       "vb",
					LineComment: "# IB",
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# DH2\n\n# HA1\n# HA2\n- la # IA\n# FA1\n# FA2\n\n# HB1\n# HB2\n- lb # IB\n# FB1\n# FB2\n\n# DF1\n\n# DF2\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        7,
			Column:      1,
			HeadComment: "# DH1\n\n# DH2",
			FootComment: "# DF1\n\n# DF2",
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Tag:    "!!seq",
				Line:   7,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        7,
					Column:      3,
					Value:       "la",
					HeadComment: "# HA1\n# HA2",
					LineComment: "# IA",
					FootComment: "# FA1\n# FA2",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        13,
					Column:      3,
					Value:       "lb",
					HeadComment: "# HB1\n# HB2",
					LineComment: "# IB",
					FootComment: "# FB1\n# FB2",
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n- la # IA\n# HB1\n- lb\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        3,
			Column:      1,
			HeadComment: "# DH1",
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Tag:    "!!seq",
				Line:   3,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        3,
					Column:      3,
					Value:       "la",
					LineComment: "# IA",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        5,
					Column:      3,
					Value:       "lb",
					HeadComment: "# HB1",
				}},
			}},
		},
	}, {
		yaml: "- la # IA\n- lb # IB\n- lc # IC\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Tag:    "!!seq",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        1,
					Column:      3,
					Value:       "la",
					LineComment: "# IA",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        2,
					Column:      3,
					Value:       "lb",
					LineComment: "# IB",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        3,
					Column:      3,
					Value:       "lc",
					LineComment: "# IC",
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# HL1\n- - la\n  # HB1\n  - lb\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Tag:    "!!seq",
				Line:   4,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.SequenceNode,
					Tag:         "!!seq",
					Line:        4,
					Column:      3,
					HeadComment: "# HL1",
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Line:   4,
						Column: 5,
						Value:  "la",
					}, {
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Line:        6,
						Column:      5,
						Value:       "lb",
						HeadComment: "# HB1",
					}},
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# HL1\n- # HA1\n  - la\n  # HB1\n  - lb\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Tag:    "!!seq",
				Line:   4,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.SequenceNode,
					Tag:         "!!seq",
					Line:        5,
					Column:      3,
					HeadComment: "# HL1",
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Line:        5,
						Column:      5,
						Value:       "la",
						HeadComment: "# HA1",
					}, {
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Line:        7,
						Column:      5,
						Value:       "lb",
						HeadComment: "# HB1",
					}},
				}},
			}},
		},
	}, {
		yaml: "[decode]# DH1\n\n# HL1\n- # HA1\n\n  - la\n  # HB1\n  - lb\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			Content: []*yaml.Node{{
				Kind:   yaml.SequenceNode,
				Tag:    "!!seq",
				Line:   4,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.SequenceNode,
					Tag:         "!!seq",
					Line:        6,
					Column:      3,
					HeadComment: "# HL1",
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Line:        6,
						Column:      5,
						Value:       "la",
						HeadComment: "# HA1\n",
					}, {
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Line:        8,
						Column:      5,
						Value:       "lb",
						HeadComment: "# HB1",
					}},
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# HA1\nka:\n  # HB1\n  kb:\n    # HC1\n    # HC2\n    - lc # IC\n    # FC1\n    # FC2\n\n    # HD1\n    - ld # ID\n    # FD1\n\n# DF1\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			FootComment: "# DF1",
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   4,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        4,
					Column:      1,
					Value:       "ka",
					HeadComment: "# HA1",
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   6,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Line:        6,
						Column:      3,
						Value:       "kb",
						HeadComment: "# HB1",
					}, {
						Kind:   yaml.SequenceNode,
						Line:   9,
						Column: 5,
						Tag:    "!!seq",
						Content: []*yaml.Node{{
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Line:        9,
							Column:      7,
							Value:       "lc",
							HeadComment: "# HC1\n# HC2",
							LineComment: "# IC",
							FootComment: "# FC1\n# FC2",
						}, {
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Line:        14,
							Column:      7,
							Value:       "ld",
							HeadComment: "# HD1",

							LineComment: "# ID",
							FootComment: "# FD1",
						}},
					}},
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# HA1\nka:\n  # HB1\n  kb:\n    # HC1\n    # HC2\n    - lc # IC\n    # FC1\n    # FC2\n\n    # HD1\n    - ld # ID\n    # FD1\nke: ve\n\n# DF1\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			FootComment: "# DF1",
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   4,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        4,
					Column:      1,
					Value:       "ka",
					HeadComment: "# HA1",
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   6,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Line:        6,
						Column:      3,
						Value:       "kb",
						HeadComment: "# HB1",
					}, {
						Kind:   yaml.SequenceNode,
						Line:   9,
						Column: 5,
						Tag:    "!!seq",
						Content: []*yaml.Node{{
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Line:        9,
							Column:      7,
							Value:       "lc",
							HeadComment: "# HC1\n# HC2",
							LineComment: "# IC",
							FootComment: "# FC1\n# FC2",
						}, {
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Line:        14,
							Column:      7,
							Value:       "ld",
							HeadComment: "# HD1",
							LineComment: "# ID",
							FootComment: "# FD1",
						}},
					}},
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Line:   16,
					Column: 1,
					Value:  "ke",
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Line:   16,
					Column: 5,
					Value:  "ve",
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# DH2\n\n# HA1\n# HA2\nka:\n  # HB1\n  # HB2\n  kb:\n" +
			"    # HC1\n    # HC2\n    kc:\n      # HD1\n      # HD2\n      kd: vd\n      # FD1\n      # FD2\n" +
			"    # FC1\n    # FC2\n  # FB1\n  # FB2\n# FA1\n# FA2\n\n# HE1\n# HE2\nke: ve\n# FE1\n# FE2\n\n# DF1\n\n# DF2\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			HeadComment: "# DH1\n\n# DH2",
			FootComment: "# DF1\n\n# DF2",
			Line:        7,
			Column:      1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   7,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1\n# HA2",
					FootComment: "# FA1\n# FA2",
					Line:        7,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   10,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1\n# HB2",
						FootComment: "# FB1\n# FB2",
						Line:        10,
						Column:      3,
					}, {
						Kind:   yaml.MappingNode,
						Tag:    "!!map",
						Line:   13,
						Column: 5,
						Content: []*yaml.Node{{
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "kc",
							HeadComment: "# HC1\n# HC2",
							FootComment: "# FC1\n# FC2",
							Line:        13,
							Column:      5,
						}, {
							Kind:   yaml.MappingNode,
							Tag:    "!!map",
							Line:   16,
							Column: 7,
							Content: []*yaml.Node{{
								Kind:        yaml.ScalarNode,
								Tag:         "!!str",
								Value:       "kd",
								HeadComment: "# HD1\n# HD2",
								FootComment: "# FD1\n# FD2",
								Line:        16,
								Column:      7,
							}, {
								Kind:   yaml.ScalarNode,
								Tag:    "!!str",
								Value:  "vd",
								Line:   16,
								Column: 11,
							}},
						}},
					}},
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ke",
					HeadComment: "# HE1\n# HE2",
					FootComment: "# FE1\n# FE2",
					Line:        28,
					Column:      1,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "ve",
					Line:   28,
					Column: 5,
				}},
			}},
		},
	}, {
		// Same as above but indenting ke in so it's also part of ka's value.
		yaml: "# DH1\n\n# DH2\n\n# HA1\n# HA2\nka:\n  # HB1\n  # HB2\n  kb:\n" +
			"    # HC1\n    # HC2\n    kc:\n      # HD1\n      # HD2\n      kd: vd\n      # FD1\n      # FD2\n" +
			"    # FC1\n    # FC2\n  # FB1\n  # FB2\n\n  # HE1\n  # HE2\n  ke: ve\n  # FE1\n  # FE2\n# FA1\n# FA2\n\n# DF1\n\n# DF2\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			HeadComment: "# DH1\n\n# DH2",
			FootComment: "# DF1\n\n# DF2",
			Line:        7,
			Column:      1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   7,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1\n# HA2",
					FootComment: "# FA1\n# FA2",
					Line:        7,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   10,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1\n# HB2",
						FootComment: "# FB1\n# FB2",
						Line:        10,
						Column:      3,
					}, {
						Kind:   yaml.MappingNode,
						Tag:    "!!map",
						Line:   13,
						Column: 5,
						Content: []*yaml.Node{{
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "kc",
							HeadComment: "# HC1\n# HC2",
							FootComment: "# FC1\n# FC2",
							Line:        13,
							Column:      5,
						}, {
							Kind:   yaml.MappingNode,
							Tag:    "!!map",
							Line:   16,
							Column: 7,
							Content: []*yaml.Node{{
								Kind:        yaml.ScalarNode,
								Tag:         "!!str",
								Value:       "kd",
								HeadComment: "# HD1\n# HD2",
								FootComment: "# FD1\n# FD2",
								Line:        16,
								Column:      7,
							}, {
								Kind:   yaml.ScalarNode,
								Tag:    "!!str",
								Value:  "vd",
								Line:   16,
								Column: 11,
							}},
						}},
					}, {
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "ke",
						HeadComment: "# HE1\n# HE2",
						FootComment: "# FE1\n# FE2",
						Line:        26,
						Column:      3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "ve",
						Line:   26,
						Column: 7,
					}},
				}},
			}},
		},
	}, {
		// Decode only due to lack of newline at the end.
		yaml: "[decode]# HA1\nka:\n  # HB1\n  kb: vb\n  # FB1\n# FA1",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   2,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1",
					FootComment: "# FA1",
					Line:        2,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   4,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1",
						FootComment: "# FB1",
						Line:        4,
						Column:      3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   4,
						Column: 7,
					}},
				}},
			}},
		},
	}, {
		// Same as above, but with newline at the end.
		yaml: "# HA1\nka:\n  # HB1\n  kb: vb\n  # FB1\n# FA1\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   2,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1",
					FootComment: "# FA1",
					Line:        2,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   4,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1",
						FootComment: "# FB1",
						Line:        4,
						Column:      3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   4,
						Column: 7,
					}},
				}},
			}},
		},
	}, {
		// Same as above, but without FB1.
		yaml: "# HA1\nka:\n  # HB1\n  kb: vb\n# FA1\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   2,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1",
					FootComment: "# FA1",
					Line:        2,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   4,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1",
						Line:        4,
						Column:      3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   4,
						Column: 7,
					}},
				}},
			}},
		},
	}, {
		// Same as above, but with two newlines at the end. Decode-only for that.
		yaml: "[decode]# HA1\nka:\n  # HB1\n  kb: vb\n  # FB1\n# FA1\n\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   2,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1",
					FootComment: "# FA1",
					Line:        2,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   4,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1",
						FootComment: "# FB1",
						Line:        4,
						Column:      3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   4,
						Column: 7,
					}},
				}},
			}},
		},
	}, {
		// Similar to above, but make HB1 look more like a footer of ka.
		yaml: "[decode]# HA1\nka:\n# HB1\n\n  kb: vb\n# FA1\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   2,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1",
					FootComment: "# FA1",
					Line:        2,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   5,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1\n",
						Line:        5,
						Column:      3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   5,
						Column: 7,
					}},
				}},
			}},
		},
	}, {
		yaml: "ka:\n  kb: vb\n# FA1\n\nkc: vc\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					Line:        1,
					Column:      1,
					FootComment: "# FA1",
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "kb",
						Line:   2,
						Column: 3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   2,
						Column: 7,
					}},
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "kc",
					Line:   5,
					Column: 1,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "vc",
					Line:   5,
					Column: 5,
				}},
			}},
		},
	}, {
		yaml: "ka:\n  kb: vb\n# HC1\nkc: vc\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "ka",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "kb",
						Line:   2,
						Column: 3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   2,
						Column: 7,
					}},
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "kc",
					HeadComment: "# HC1",
					Line:        4,
					Column:      1,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "vc",
					Line:   4,
					Column: 5,
				}},
			}},
		},
	}, {
		// Decode only due to empty line before HC1.
		yaml: "[decode]ka:\n  kb: vb\n\n# HC1\nkc: vc\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "ka",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "kb",
						Line:   2,
						Column: 3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   2,
						Column: 7,
					}},
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "kc",
					HeadComment: "# HC1",
					Line:        5,
					Column:      1,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "vc",
					Line:   5,
					Column: 5,
				}},
			}},
		},
	}, {
		// Decode-only due to empty lines around HC1.
		yaml: "[decode]ka:\n  kb: vb\n\n# HC1\n\nkc: vc\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "ka",
					Line:   1,
					Column: 1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "kb",
						Line:   2,
						Column: 3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   2,
						Column: 7,
					}},
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "kc",
					HeadComment: "# HC1\n",
					Line:        6,
					Column:      1,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "vc",
					Line:   6,
					Column: 5,
				}},
			}},
		},
	}, {
		yaml: "ka: # IA\n  kb: # IB\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					Line:        1,
					Column:      1,
					LineComment: "# IA",
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						Line:        2,
						Column:      3,
						LineComment: "# IB",
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!null",
						Line:   2,
						Column: 6,
					}},
				}},
			}},
		},
	}, {
		yaml: "# HA1\nka:\n  # HB1\n  kb: vb\n  # FB1\n# HC1\n# HC2\nkc: vc\n# FC1\n# FC2\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   2,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1",
					Line:        2,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   4,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1",
						FootComment: "# FB1",
						Line:        4,
						Column:      3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   4,
						Column: 7,
					}},
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "kc",
					HeadComment: "# HC1\n# HC2",
					FootComment: "# FC1\n# FC2",
					Line:        8,
					Column:      1,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "vc",
					Line:   8,
					Column: 5,
				}},
			}},
		},
	}, {
		// Same as above, but decode only due to empty line between ka's value and kc's headers.
		yaml: "[decode]# HA1\nka:\n  # HB1\n  kb: vb\n  # FB1\n\n# HC1\n# HC2\nkc: vc\n# FC1\n# FC2\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   2,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "ka",
					HeadComment: "# HA1",
					Line:        2,
					Column:      1,
				}, {
					Kind:   yaml.MappingNode,
					Tag:    "!!map",
					Line:   4,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:        yaml.ScalarNode,
						Tag:         "!!str",
						Value:       "kb",
						HeadComment: "# HB1",
						FootComment: "# FB1",
						Line:        4,
						Column:      3,
					}, {
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "vb",
						Line:   4,
						Column: 7,
					}},
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Value:       "kc",
					HeadComment: "# HC1\n# HC2",
					FootComment: "# FC1\n# FC2",
					Line:        9,
					Column:      1,
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "vc",
					Line:   9,
					Column: 5,
				}},
			}},
		},
	}, {
		yaml: "# H1\n[la, lb] # I\n# F1\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   2,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:        yaml.SequenceNode,
				Tag:         "!!seq",
				Style:       yaml.FlowStyle,
				Line:        2,
				Column:      1,
				HeadComment: "# H1",
				LineComment: "# I",
				FootComment: "# F1",
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Line:   2,
					Column: 2,
					Value:  "la",
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Line:   2,
					Column: 6,
					Value:  "lb",
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# SH1\n[\n  # HA1\n  la, # IA\n  # FA1\n\n  # HB1\n  lb, # IB\n  # FB1\n]\n# SF1\n\n# DF1\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			FootComment: "# DF1",
			Content: []*yaml.Node{{
				Kind:        yaml.SequenceNode,
				Tag:         "!!seq",
				Style:       yaml.FlowStyle,
				Line:        4,
				Column:      1,
				HeadComment: "# SH1",
				FootComment: "# SF1",
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        6,
					Column:      3,
					Value:       "la",
					HeadComment: "# HA1",
					LineComment: "# IA",
					FootComment: "# FA1",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        10,
					Column:      3,
					Value:       "lb",
					HeadComment: "# HB1",
					LineComment: "# IB",
					FootComment: "# FB1",
				}},
			}},
		},
	}, {
		// Same as above, but with extra newlines before FB1 and FB2
		yaml: "[decode]# DH1\n\n# SH1\n[\n  # HA1\n  la, # IA\n  # FA1\n\n  # HB1\n  lb, # IB\n\n\n  # FB1\n\n# FB2\n]\n# SF1\n\n# DF1\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			FootComment: "# DF1",
			Content: []*yaml.Node{{
				Kind:        yaml.SequenceNode,
				Tag:         "!!seq",
				Style:       yaml.FlowStyle,
				Line:        4,
				Column:      1,
				HeadComment: "# SH1",
				FootComment: "# SF1",
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        6,
					Column:      3,
					Value:       "la",
					HeadComment: "# HA1",
					LineComment: "# IA",
					FootComment: "# FA1",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        10,
					Column:      3,
					Value:       "lb",
					HeadComment: "# HB1",
					LineComment: "# IB",
					FootComment: "# FB1\n\n# FB2",
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# SH1\n[\n  # HA1\n  la,\n  # FA1\n\n  # HB1\n  lb,\n  # FB1\n]\n# SF1\n\n# DF1\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			FootComment: "# DF1",
			Content: []*yaml.Node{{
				Kind:        yaml.SequenceNode,
				Tag:         "!!seq",
				Style:       yaml.FlowStyle,
				Line:        4,
				Column:      1,
				HeadComment: "# SH1",
				FootComment: "# SF1",
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        6,
					Column:      3,
					Value:       "la",
					HeadComment: "# HA1",
					FootComment: "# FA1",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        10,
					Column:      3,
					Value:       "lb",
					HeadComment: "# HB1",
					FootComment: "# FB1",
				}},
			}},
		},
	}, {
		yaml: "ka:\n  kb: [\n    # HA1\n    la,\n    # FA1\n\n    # HB1\n    lb,\n    # FB1\n  ]\n",
		node: yaml.Node{
			Kind:   yaml.DocumentNode,
			Line:   1,
			Column: 1,
			Content: []*yaml.Node{{
				Kind:   yaml.MappingNode,
				Tag:    "!!map",
				Line:   1,
				Column: 1,
				Content: []*yaml.Node{{
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Value:  "ka",
					Line:   1,
					Column: 1,
				}, {
					Kind:   0x4,
					Tag:    "!!map",
					Line:   2,
					Column: 3,
					Content: []*yaml.Node{{
						Kind:   yaml.ScalarNode,
						Tag:    "!!str",
						Value:  "kb",
						Line:   2,
						Column: 3,
					}, {
						Kind:   yaml.SequenceNode,
						Style:  0x20,
						Tag:    "!!seq",
						Line:   2,
						Column: 7,
						Content: []*yaml.Node{{
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "la",
							HeadComment: "# HA1",
							FootComment: "# FA1",
							Line:        4,
							Column:      5,
						}, {
							Kind:        yaml.ScalarNode,
							Tag:         "!!str",
							Value:       "lb",
							HeadComment: "# HB1",
							FootComment: "# FB1",
							Line:        8,
							Column:      5,
						}},
					}},
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# MH1\n{\n  # HA1\n  ka: va, # IA\n  # FA1\n\n  # HB1\n  kb: vb, # IB\n  # FB1\n}\n# MF1\n\n# DF1\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			FootComment: "# DF1",
			Content: []*yaml.Node{{
				Kind:        yaml.MappingNode,
				Tag:         "!!map",
				Style:       yaml.FlowStyle,
				Line:        4,
				Column:      1,
				HeadComment: "# MH1",
				FootComment: "# MF1",
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        6,
					Column:      3,
					Value:       "ka",
					HeadComment: "# HA1",
					FootComment: "# FA1",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        6,
					Column:      7,
					Value:       "va",
					LineComment: "# IA",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        10,
					Column:      3,
					Value:       "kb",
					HeadComment: "# HB1",
					FootComment: "# FB1",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        10,
					Column:      7,
					Value:       "vb",
					LineComment: "# IB",
				}},
			}},
		},
	}, {
		yaml: "# DH1\n\n# MH1\n{\n  # HA1\n  ka: va,\n  # FA1\n\n  # HB1\n  kb: vb,\n  # FB1\n}\n# MF1\n\n# DF1\n",
		node: yaml.Node{
			Kind:        yaml.DocumentNode,
			Line:        4,
			Column:      1,
			HeadComment: "# DH1",
			FootComment: "# DF1",
			Content: []*yaml.Node{{
				Kind:        yaml.MappingNode,
				Tag:         "!!map",
				Style:       yaml.FlowStyle,
				Line:        4,
				Column:      1,
				HeadComment: "# MH1",
				FootComment: "# MF1",
				Content: []*yaml.Node{{
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        6,
					Column:      3,
					Value:       "ka",
					HeadComment: "# HA1",
					FootComment: "# FA1",
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Line:   6,
					Column: 7,
					Value:  "va",
				}, {
					Kind:        yaml.ScalarNode,
					Tag:         "!!str",
					Line:        10,
					Column:      3,
					Value:       "kb",
					HeadComment: "# HB1",
					FootComment: "# FB1",
				}, {
					Kind:   yaml.ScalarNode,
					Tag:    "!!str",
					Line:   10,
					Column: 7,
					Value:  "vb",
				}},
			}},
		},
	},
	//}, {
	//	yaml: "# DH1\n\n# DH2\n\n# HA1\n# HA2\n- &x la # IA\n# FA1\n# FA2\n\n# HB1\n# HB2\n- *x # IB\n# FB1\n# FB2\n\n# DF1\n\n# DF2\n",
	//	node: yaml.Node{
	//		Kind:        yaml.DocumentNode,
	//		Line:        7,
	//		Column:      1,
	//		HeadComment: "# DH1\n\n# DH2",
	//		FootComment: "# DF1\n\n# DF2",
	//		Content: []*yaml.Node{{
	//			Kind:   yaml.SequenceNode,
	//			Tag:    "!!seq",
	//			Line:   7,
	//			Column: 1,
	//			Content: []*yaml.Node{
	//				saveNode("x", &yaml.Node{
	//					Kind:        yaml.ScalarNode,
	//					Tag:         "!!str",
	//					Line:        7,
	//					Column:      3,
	//					Value:       "la",
	//					HeadComment: "# HA1\n# HA2",
	//					LineComment: "# IA",
	//					FootComment: "# FA1\n# FA2",
	//					Anchor:      "x",
	//				}), {
	//					Kind:        yaml.AliasNode,
	//					Line:        13,
	//					Column:      3,
	//					Value:       "x",
	//					Alias:       dropNode("x"),
	//					HeadComment: "# HB1\n# HB2",
	//					LineComment: "# IB",
	//					FootComment: "# FB1\n# FB2",
	//				},
	//			},
	//		}},
	//	},
	//},
}

var lpattern = "  expected comments:\n%s"

//func (s *S) TestNodeRoundtrip(c *C) {
//	defer os.Setenv("TZ", os.Getenv("TZ"))
//	os.Setenv("TZ", "UTC")
//	for i, item := range nodeTests {
//		c.Logf("test %d: %q", i, item.yaml)
//
//		if strings.Contains(item.yaml, "#") {
//			var buf bytes.Buffer
//			fprintComments(&buf, &item.node, "    ")
//			c.Logf(lpattern, buf.Bytes())
//		}
//
//		decode := true
//		encode := true
//
//		testYaml := item.yaml
//		if s := strings.TrimPrefix(testYaml, "[decode]"); s != testYaml {
//			encode = false
//			testYaml = s
//		}
//		if s := strings.TrimPrefix(testYaml, "[encode]"); s != testYaml {
//			decode = false
//			testYaml = s
//		}
//
//		if decode {
//			var node yaml.Node
//			err := yaml.Unmarshal([]byte(testYaml), &node)
//			c.Assert(err, IsNil)
//			if strings.Contains(item.yaml, "#") {
//				var buf bytes.Buffer
//				fprintComments(&buf, &node, "    ")
//				c.Logf("  obtained comments:\n%s", buf.Bytes())
//			}
//			c.Assert(&node, DeepEquals, &item.node)
//		}
//		if encode {
//			node := deepCopyNode(&item.node, nil)
//			buf := bytes.Buffer{}
//			enc := yaml.NewEncoder(&buf)
//			enc.SetIndent(2)
//			err := enc.Encode(node)
//			c.Assert(err, IsNil)
//			err = enc.Close()
//			c.Assert(err, IsNil)
//			c.Assert(buf.String(), Equals, testYaml)
//
//			// Ensure there were no mutations to the tree.
//			c.Assert(node, DeepEquals, &item.node)
//		}
//	}
//}

func TestNodeRoundtrip(t *testing.T) {
	for i, item := range nodeTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			decode := true
			encode := true

			testYaml := item.yaml
			if s := strings.TrimPrefix(testYaml, "[decode]"); s != testYaml {
				encode = false
				testYaml = s
			}
			if s := strings.TrimPrefix(testYaml, "[encode]"); s != testYaml {
				decode = false
				testYaml = s
			}

			if decode {
				var node yaml.Node
				err := yaml.Unmarshal([]byte(testYaml), &node)
				require.NoError(t, err)
				require.Equal(t, &item.node, &node)
			}
			if encode {
				node := deepCopyNode(&item.node, nil)
				buf := bytes.Buffer{}
				enc := yaml.NewEncoder(&buf)
				enc.SetIndent(2)
				err := enc.Encode(node)
				require.NoError(t, err)
				err = enc.Close()
				require.NoError(t, err)
				require.Equal(t, testYaml, buf.String())
				require.Equal(t, &item.node, node)
			}
		})
	}
}

func deepCopyNode(node *yaml.Node, cache map[*yaml.Node]*yaml.Node) *yaml.Node {
	if n, ok := cache[node]; ok {
		return n
	}
	if cache == nil {
		cache = make(map[*yaml.Node]*yaml.Node)
	}
	clone := *node
	cache[node] = &clone
	clone.Content = nil
	for _, elem := range node.Content {
		clone.Content = append(clone.Content, deepCopyNode(elem, cache))
	}
	if node.Alias != nil {
		clone.Alias = deepCopyNode(node.Alias, cache)
	}
	return &clone
}

var savedNodes = make(map[string]*yaml.Node)

func saveNode(name string, node *yaml.Node) *yaml.Node {
	savedNodes[name] = node
	return node
}

func peekNode(name string) *yaml.Node {
	return savedNodes[name]
}

func dropNode(name string) *yaml.Node {
	node := savedNodes[name]
	delete(savedNodes, name)
	return node
}

var setStringTests = []struct {
	str  string
	yaml string
	node yaml.Node
}{
	{
		str:  "something simple",
		yaml: "something simple\n",
		node: yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "something simple",
			Tag:   "!!str",
		},
	}, {
		str:  `"quoted value"`,
		yaml: "'\"quoted value\"'\n",
		node: yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: `"quoted value"`,
			Tag:   "!!str",
		},
	}, {
		str:  "multi\nline",
		yaml: "|-\n  multi\n  line\n",
		node: yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "multi\nline",
			Tag:   "!!str",
			Style: yaml.LiteralStyle,
		},
	}, {
		str:  "123",
		yaml: "\"123\"\n",
		node: yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "123",
			Tag:   "!!str",
		},
	}, {
		str:  "multi\nline\n",
		yaml: "|\n  multi\n  line\n",
		node: yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "multi\nline\n",
			Tag:   "!!str",
			Style: yaml.LiteralStyle,
		},
	}, {
		str:  "\x80\x81\x82",
		yaml: "!!binary gIGC\n",
		node: yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "gIGC",
			Tag:   "!!binary",
		},
	},
}

func TestSetString(t *testing.T) {
	for i, item := range setStringTests {
		t.Run(fmt.Sprintf("%d: %q", i, item.str), func(t *testing.T) {
			var node yaml.Node

			node.SetString(item.str)

			require.Equal(t, item.node, node)

			buf := bytes.Buffer{}
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			err := enc.Encode(&item.node)
			require.NoError(t, err)
			err = enc.Close()
			require.NoError(t, err)
			require.Equal(t, item.yaml, buf.String())

			var doc yaml.Node
			err = yaml.Unmarshal([]byte(item.yaml), &doc)
			require.NoError(t, err)

			var str string
			err = node.Decode(&str)
			require.NoError(t, err)
			require.Equal(t, item.str, str)
		})
	}
}

var nodeEncodeDecodeTests = []struct {
	value interface{}
	yaml  string
	node  yaml.Node
}{{
	value: "something simple",
	yaml:  "something simple\n",
	node: yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "something simple",
		Tag:   "!!str",
	},
}, {
	value: `"quoted value"`,
	yaml:  "'\"quoted value\"'\n",
	node: yaml.Node{
		Kind:  yaml.ScalarNode,
		Style: yaml.SingleQuotedStyle,
		Value: `"quoted value"`,
		Tag:   "!!str",
	},
}, {
	value: 123,
	yaml:  "123",
	node: yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: `123`,
		Tag:   "!!int",
	},
}, {
	value: []interface{}{1, 2},
	yaml:  "[1, 2]",
	node: yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
		Content: []*yaml.Node{{
			Kind:  yaml.ScalarNode,
			Value: "1",
			Tag:   "!!int",
		}, {
			Kind:  yaml.ScalarNode,
			Value: "2",
			Tag:   "!!int",
		}},
	},
}, {
	value: map[string]interface{}{"a": "b"},
	yaml:  "a: b",
	node: yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{{
			Kind:  yaml.ScalarNode,
			Value: "a",
			Tag:   "!!str",
		}, {
			Kind:  yaml.ScalarNode,
			Value: "b",
			Tag:   "!!str",
		}},
	},
}}

func TestNodeEncodeDecode(t *testing.T) {
	for i, item := range nodeEncodeDecodeTests {
		t.Run(fmt.Sprintf("Encode/Decode test value #%d", i), func(t *testing.T) {
			var v interface{}
			err := item.node.Decode(&v)
			require.NoError(t, err)
			require.Equal(t, item.value, v)

			var n yaml.Node
			err = n.Encode(item.value)
			require.NoError(t, err)
			require.Equal(t, item.node, n)
		})
	}
}

func TestNodeZeroEncodeDecode(t *testing.T) {
	// Zero node value behaves as nil when encoding...
	var n yaml.Node
	data, err := yaml.Marshal(&n)
	require.NoError(t, err)
	require.Equal(t, "null\n", string(data))

	// ... and decoding.
	var v *struct{} = &struct{}{}
	require.NoError(t, n.Decode(&v))
	require.Nil(t, v)

	// ... and even when looking for its tag.
	require.Equal(t, "!!null", n.ShortTag())

	// Kind zero is still unknown, though.
	n.Line = 1
	_, err = yaml.Marshal(&n)
	require.Error(t, err)
	require.Equal(t, "yaml: cannot encode node with unknown kind 0", err.Error())
	require.Error(t, n.Decode(&v))
}

func TestNodeOmitEmpty(t *testing.T) {
	var v struct {
		A int
		B yaml.Node ",omitempty"
	}
	v.A = 1
	data, err := yaml.Marshal(&v)
	require.NoError(t, err)
	require.Equal(t, "a: 1\n", string(data))

	v.B.Line = 1
	_, err = yaml.Marshal(&v)
	require.Error(t, err)
}

func fprintComments(out io.Writer, node *yaml.Node, indent string) {
	switch node.Kind {
	case yaml.ScalarNode:
		fmt.Fprintf(out, "%s<%s> ", indent, node.Value)
		fprintCommentSet(out, node)
		fmt.Fprintf(out, "\n")
	case yaml.DocumentNode:
		fmt.Fprintf(out, "%s<DOC> ", indent)
		fprintCommentSet(out, node)
		fmt.Fprintf(out, "\n")
		for i := 0; i < len(node.Content); i++ {
			fprintComments(out, node.Content[i], indent+"  ")
		}
	case yaml.MappingNode:
		fmt.Fprintf(out, "%s<MAP> ", indent)
		fprintCommentSet(out, node)
		fmt.Fprintf(out, "\n")
		for i := 0; i < len(node.Content); i += 2 {
			fprintComments(out, node.Content[i], indent+"  ")
			fprintComments(out, node.Content[i+1], indent+"  ")
		}
	case yaml.SequenceNode:
		fmt.Fprintf(out, "%s<SEQ> ", indent)
		fprintCommentSet(out, node)
		fmt.Fprintf(out, "\n")
		for i := 0; i < len(node.Content); i++ {
			fprintComments(out, node.Content[i], indent+"  ")
		}
	}
}

func fprintCommentSet(out io.Writer, node *yaml.Node) {
	if len(node.HeadComment)+len(node.LineComment)+len(node.FootComment) > 0 {
		fmt.Fprintf(out, "%q / %q / %q", node.HeadComment, node.LineComment, node.FootComment)
	}
}
