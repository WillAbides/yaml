package common

import (
	"gopkg.in/yaml.v3/internal/yamlh"
)

var DefaultTagDirectives = []yamlh.TagDirective{
	{Handle: []byte("!"), Prefix: []byte("!")},
	{Handle: []byte("!!"), Prefix: []byte("tag:yaml.org,2002:")},
}
