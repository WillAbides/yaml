package emitter

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/willabides/yaml/internal/yamlh"
)

func analyzeAnchor(e *Emitter, anchor []byte, alias bool) error {
	if len(anchor) == 0 {
		problem := "anchor value must not be empty"
		if alias {
			problem = "alias value must not be empty"
		}
		return errors.New(problem)
	}
	for i := 0; i < len(anchor); i += yamlh.Width(anchor[i]) {
		if !yamlh.Is_alpha(anchor, i) {
			problem := "anchor value must contain alphanumerical characters only"
			if alias {
				problem = "alias value must contain alphanumerical characters only"
			}
			return errors.New(problem)
		}
	}
	e.anchorData.Anchor = anchor
	e.anchorData.Alias = alias
	return nil
}

func analyzeTag(e *Emitter, tag []byte) error {
	if len(tag) == 0 {
		return fmt.Errorf("tag value must not be empty")
	}
	for i := 0; i < len(e.tagDirectives); i++ {
		tag_directive := &e.tagDirectives[i]
		if bytes.HasPrefix(tag, tag_directive.Prefix) {
			e.tagData.Handle = tag_directive.Handle
			e.tagData.Suffix = tag[len(tag_directive.Prefix):]
			return nil
		}
	}
	e.tagData.Suffix = tag
	return nil
}

func analyzeVersionDirective(version_directive *yamlh.VersionDirective) error {
	if version_directive.Major != 1 || version_directive.Minor != 1 {
		return errors.New(`incompatible %YAML directive`)
	}
	return nil
}

func analyzeTagDirective(tag_directive *yamlh.TagDirective) error {
	handle := tag_directive.Handle
	prefix := tag_directive.Prefix
	if len(handle) == 0 {
		return errors.New(`tag handle must not be empty`)
	}
	if handle[0] != '!' {
		return errors.New(`tag handle must start with '!'`)
	}
	if handle[len(handle)-1] != '!' {
		return errors.New(`tag handle must end with '!'`)
	}
	for i := 1; i < len(handle)-1; i += yamlh.Width(handle[i]) {
		if !yamlh.Is_alpha(handle, i) {
			return errors.New(`tag handle must contain alphanumerical characters only`)
		}
	}
	if len(prefix) == 0 {
		return errors.New(`tag prefix must not be empty`)
	}
	return nil
}

func analyzeScalar(value []byte) scalarData {
	if len(value) == 0 {
		return scalarData{
			value:               value,
			blockPlainAllowed:   true,
			singleQuotedAllowed: true,
		}
	}

	sd := scalarData{
		value:               value,
		blockPlainAllowed:   true,
		flowPlainAllowed:    true,
		singleQuotedAllowed: true,
		blockAllowed:        true,
	}

	if len(value) >= 3 {
		if bytes.Equal(value[:3], []byte("---")) || bytes.Equal(value[:3], []byte("...")) {
			sd.blockPlainAllowed = false
			sd.flowPlainAllowed = false
		}
	}

	prevWhitespace := true
	var prevSpace, prevBreak bool
	first := true
	for len(value) > 0 {
		w := yamlh.Width(value[0])
		char := value[0]
		nextChar := byte(0)
		if len(value) > w {
			nextChar = value[w]
		}
		last := w >= len(value)
		nextWhitespace := last || yamlh.IsBlank(nextChar)

		if first {
			switch char {
			case '#', ',', '[', ']', '{', '}', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`', ' ':
				sd.flowPlainAllowed = false
				sd.blockPlainAllowed = false
			case '?', ':':
				sd.flowPlainAllowed = false
				if nextWhitespace {
					sd.blockPlainAllowed = false
				}
			case '-':
				if nextWhitespace {
					sd.flowPlainAllowed = false
					sd.blockPlainAllowed = false
				}
			}
			first = false
		} else {
			switch char {
			case ',', '?', '[', ']', '{', '}':
				sd.flowPlainAllowed = false
			case ':':
				sd.flowPlainAllowed = false
				if nextWhitespace {
					sd.blockPlainAllowed = false
				}
			case '#':
				if prevWhitespace {
					sd.flowPlainAllowed = false
					sd.blockPlainAllowed = false
				}
			}
		}

		if char == '\t' {
			sd.blockPlainAllowed = false
			sd.singleQuotedAllowed = false
		} else if !yamlh.IsPrintable(value) {
			sd.flowPlainAllowed = false
			sd.blockPlainAllowed = false
			sd.singleQuotedAllowed = false
			sd.blockAllowed = false
		}

		switch {
		case char == ' ':
			if last {
				sd.blockPlainAllowed = false
				sd.flowPlainAllowed = false
				sd.blockAllowed = false
			}
			if prevBreak {
				sd.blockPlainAllowed = false
				sd.singleQuotedAllowed = false
				sd.flowPlainAllowed = false
			}
			prevSpace = true
			prevBreak = false
		case yamlh.IsBreak(value):
			sd.multiline = true
			sd.blockPlainAllowed = false
			sd.flowPlainAllowed = false
			if prevSpace {
				sd.singleQuotedAllowed = false
				sd.blockAllowed = false
			}
			prevSpace = false
			prevBreak = true
		default:
			prevSpace = false
			prevBreak = false
		}
		// [Go]: Why 'z'? Couldn't be the end of the string as that's the loop condition.
		prevWhitespace = yamlh.IsBlankz(value)
		value = value[w:]
	}
	return sd
}

func analyzeEvent(e *Emitter, event *yamlh.Event) error {
	e.anchorData.Anchor = nil
	e.tagData.Handle = nil
	e.tagData.Suffix = nil
	e.scalarData.value = nil

	if len(event.Head_comment) > 0 {
		e.headComment = event.Head_comment
	}
	if len(event.Line_comment) > 0 {
		e.lineComment = event.Line_comment
	}
	if len(event.Foot_comment) > 0 {
		e.footComment = event.Foot_comment
	}
	if len(event.Tail_comment) > 0 {
		e.tailComment = event.Tail_comment
	}
	var err error
	switch event.Type {
	case yamlh.ALIAS_EVENT:
		err = analyzeAnchor(e, event.Anchor, true)
		if err != nil {
			return err
		}
	case yamlh.SCALAR_EVENT:
		if len(event.Anchor) > 0 {
			err = analyzeAnchor(e, event.Anchor, false)
			if err != nil {
				return err
			}
		}
		if len(event.Tag) > 0 && !event.Implicit && !event.Quoted_implicit {
			err = analyzeTag(e, event.Tag)
			if err != nil {
				return err
			}
		}
		e.scalarData = analyzeScalar(event.Value)
	case yamlh.SEQUENCE_START_EVENT, yamlh.MAPPING_START_EVENT:
		if len(event.Anchor) > 0 {
			err = analyzeAnchor(e, event.Anchor, true)
			if err != nil {
				return err
			}
		}
		if len(event.Tag) > 0 && !event.Implicit {
			err = analyzeTag(e, event.Tag)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
