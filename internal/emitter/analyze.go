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
		return fmt.Errorf(problem)

	}
	for i := 0; i < len(anchor); i += yamlh.Width(anchor[i]) {
		if !yamlh.Is_alpha(anchor, i) {
			problem := "anchor value must contain alphanumerical characters only"
			if alias {
				problem = "alias value must contain alphanumerical characters only"
			}
			return fmt.Errorf(problem)
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

func analyzeScalar(e *Emitter, value []byte) {
	var block_indicators, flow_indicators, line_breaks, special_characters, tab_characters bool
	var leading_space, leading_break, trailing_space, trailing_break, break_space, space_break bool
	var preceded_by_whitespace, followed_by_whitespace, previous_space, previous_break bool

	e.scalarData.value = value

	if len(value) == 0 {
		e.scalarData.multiline = false
		e.scalarData.flowPlainAllowed = false
		e.scalarData.blockPlainAllowed = true
		e.scalarData.singleQuotedAllowed = true
		e.scalarData.blockAllowed = false
		return
	}

	if len(value) >= 3 && ((value[0] == '-' && value[1] == '-' && value[2] == '-') || (value[0] == '.' && value[1] == '.' && value[2] == '.')) {
		block_indicators = true
		flow_indicators = true
	}

	preceded_by_whitespace = true
	for i, w := 0, 0; i < len(value); i += w {
		w = yamlh.Width(value[i])
		followed_by_whitespace = i+w >= len(value) || yamlh.Is_blank(value, i+w)

		if i == 0 {
			switch value[i] {
			case '#', ',', '[', ']', '{', '}', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
				flow_indicators = true
				block_indicators = true
			case '?', ':':
				flow_indicators = true
				if followed_by_whitespace {
					block_indicators = true
				}
			case '-':
				if followed_by_whitespace {
					flow_indicators = true
					block_indicators = true
				}
			}
		} else {
			switch value[i] {
			case ',', '?', '[', ']', '{', '}':
				flow_indicators = true
			case ':':
				flow_indicators = true
				if followed_by_whitespace {
					block_indicators = true
				}
			case '#':
				if preceded_by_whitespace {
					flow_indicators = true
					block_indicators = true
				}
			}
		}

		if value[i] == '\t' {
			tab_characters = true
		} else if !yamlh.Is_printable(value, i) {
			special_characters = true
		}
		if yamlh.Is_space(value, i) {
			if i == 0 {
				leading_space = true
			}
			if i+yamlh.Width(value[i]) == len(value) {
				trailing_space = true
			}
			if previous_break {
				break_space = true
			}
			previous_space = true
			previous_break = false
		} else if yamlh.Is_break(value, i) {
			line_breaks = true
			if i == 0 {
				leading_break = true
			}
			if i+yamlh.Width(value[i]) == len(value) {
				trailing_break = true
			}
			if previous_space {
				space_break = true
			}
			previous_space = false
			previous_break = true
		} else {
			previous_space = false
			previous_break = false
		}

		// [Go]: Why 'z'? Couldn't be the end of the string as that's the loop condition.
		preceded_by_whitespace = yamlh.Is_blankz(value, i)
	}

	e.scalarData.multiline = line_breaks
	e.scalarData.flowPlainAllowed = true
	e.scalarData.blockPlainAllowed = true
	e.scalarData.singleQuotedAllowed = true
	e.scalarData.blockAllowed = true

	if leading_space || leading_break || trailing_space || trailing_break {
		e.scalarData.flowPlainAllowed = false
		e.scalarData.blockPlainAllowed = false
	}
	if trailing_space {
		e.scalarData.blockAllowed = false
	}
	if break_space {
		e.scalarData.flowPlainAllowed = false
		e.scalarData.blockPlainAllowed = false
		e.scalarData.singleQuotedAllowed = false
	}
	if space_break || tab_characters || special_characters {
		e.scalarData.flowPlainAllowed = false
		e.scalarData.blockPlainAllowed = false
		e.scalarData.singleQuotedAllowed = false
	}
	if space_break || special_characters {
		e.scalarData.blockAllowed = false
	}
	if line_breaks {
		e.scalarData.flowPlainAllowed = false
		e.scalarData.blockPlainAllowed = false
	}
	if flow_indicators {
		e.scalarData.flowPlainAllowed = false
	}
	if block_indicators {
		e.scalarData.blockPlainAllowed = false
	}
	return
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
		analyzeScalar(e, event.Value)
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
