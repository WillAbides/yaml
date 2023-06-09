package common

import (
	"github.com/willabides/yaml/internal/yamlh"
)

var DefaultTagDirectives = []yamlh.TagDirective{
	{Handle: []byte("!"), Prefix: []byte("!")},
	{Handle: []byte("!!"), Prefix: []byte("tag:yaml.org,2002:")},
}
