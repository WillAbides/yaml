package emitter

import (
	"fmt"

	"github.com/willabides/yaml/internal/common"
	"github.com/willabides/yaml/internal/yamlh"
)

// expect DOCUMENT-START or STREAM-END.
func emitDocumentStart(e *Emitter, event *yamlh.Event, first bool) error {
	if event.Type == yamlh.DOCUMENT_START_EVENT {
		return emitDocumentStartEvent(e, event, first)
	}

	if event.Type == yamlh.STREAM_END_EVENT {
		if e.openEnded {
			err := writeIndicator(e, []byte("..."), true, false, false)
			if err != nil {
				return err
			}
			err = writeIndent(e)
			if err != nil {
				return err
			}
		}
		e.state = emitEndState
		return nil
	}

	return fmt.Errorf("expected DOCUMENT-START or STREAM-END")
}

func emitDocumentStartEvent(e *Emitter, event *yamlh.Event, first bool) error {
	if event.Version_directive != nil {
		err := analyzeVersionDirective(event.Version_directive)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(event.Tag_directives); i++ {
		tag_directive := &event.Tag_directives[i]
		err := analyzeTagDirective(tag_directive)
		if err != nil {
			return err
		}
		err = appendTagDirective(e, tag_directive, false)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(common.DefaultTagDirectives); i++ {
		tag_directive := &common.DefaultTagDirectives[i]
		err := appendTagDirective(e, tag_directive, true)
		if err != nil {
			return err
		}
	}

	implicit := event.Implicit
	if !first {
		implicit = false
	}

	if e.openEnded && (event.Version_directive != nil || len(event.Tag_directives) > 0) {
		err := writeIndicator(e, []byte("..."), true, false, false)
		if err != nil {
			return err
		}
		err = writeIndent(e)
		if err != nil {
			return err
		}
	}

	if event.Version_directive != nil {
		implicit = false
		err := writeIndicator(e, []byte("%YAML 1.1"), true, false, false)
		if err != nil {
			return err
		}
		err = writeIndent(e)
		if err != nil {
			return err
		}
	}

	if len(event.Tag_directives) > 0 {
		implicit = false
		for i := 0; i < len(event.Tag_directives); i++ {
			tag_directive := &event.Tag_directives[i]
			err := writeIndicator(e, []byte("%TAG"), true, false, false)
			if err != nil {
				return err
			}
			err = writeTagHandle(e, tag_directive.Handle)
			if err != nil {
				return err
			}
			err = writeTagContent(e, tag_directive.Prefix, true)
			if err != nil {
				return err
			}
			err = writeIndent(e)
			if err != nil {
				return err
			}
		}
	}

	if !implicit {
		err := writeIndent(e)
		if err != nil {
			return err
		}
		err = writeIndicator(e, []byte("---"), true, false, false)
		if err != nil {
			return err
		}
		err = writeIndent(e)
		if err != nil {
			return err
		}
	}

	if len(e.headComment) > 0 {
		err := processHeadComment(e)
		if err != nil {
			return err
		}
		err = e.putBreak()
		if err != nil {
			return err
		}
	}

	e.state = EmitDocumentContentState
	return nil
}

// Determine an acceptable scalar style.
func selectScalarStyle(e *Emitter, event *yamlh.Event) error {
	no_tag := len(e.tagData.Handle) == 0 && len(e.tagData.Suffix) == 0
	if no_tag && !event.Implicit && !event.Quoted_implicit {
		return fmt.Errorf("neither tag nor implicit flags are specified")
	}

	style := event.Scalar_style()
	if style == yamlh.ANY_SCALAR_STYLE {
		style = yamlh.PLAIN_SCALAR_STYLE
	}
	if e.simpleKeyContext && e.scalarData.multiline {
		style = yamlh.DOUBLE_QUOTED_SCALAR_STYLE
	}

	if style == yamlh.PLAIN_SCALAR_STYLE {
		if e.flowLevel > 0 && !e.scalarData.flowPlainAllowed ||
			e.flowLevel == 0 && !e.scalarData.blockPlainAllowed {
			style = yamlh.SINGLE_QUOTED_SCALAR_STYLE
		}
		if len(e.scalarData.value) == 0 && (e.flowLevel > 0 || e.simpleKeyContext) {
			style = yamlh.SINGLE_QUOTED_SCALAR_STYLE
		}
		if no_tag && !event.Implicit {
			style = yamlh.SINGLE_QUOTED_SCALAR_STYLE
		}
	}
	if style == yamlh.SINGLE_QUOTED_SCALAR_STYLE {
		if !e.scalarData.singleQuotedAllowed {
			style = yamlh.DOUBLE_QUOTED_SCALAR_STYLE
		}
	}
	if style == yamlh.LITERAL_SCALAR_STYLE || style == yamlh.FOLDED_SCALAR_STYLE {
		if !e.scalarData.blockAllowed || e.flowLevel > 0 || e.simpleKeyContext {
			style = yamlh.DOUBLE_QUOTED_SCALAR_STYLE
		}
	}

	if no_tag && !event.Quoted_implicit && style != yamlh.PLAIN_SCALAR_STYLE {
		e.tagData.Handle = []byte{'!'}
	}
	e.scalarData.style = style
	return nil
}

func stateMachine(e *Emitter, event *yamlh.Event) error {
	switch e.state {
	default:
	case emitStreamStartState:
		return emitStreamStart(e, event)

	case emitFirstDocumentStartState:
		return emitDocumentStart(e, event, true)

	case emitDocumentStartState:
		return emitDocumentStart(e, event, false)

	case EmitDocumentContentState:
		return emitDocumentContent(e, event)

	case emitDocumentEndState:
		return emitDocumentEnd(e, event)

	case emitFlowSequenceFirstItemState:
		return emitFlowSequenceItem(e, event, true, false)

	case emitFlowSequenceTrailItemState:
		return emitFlowSequenceItem(e, event, false, true)

	case emitFlowSequenceItemState:
		return emitFlowSequenceItem(e, event, false, false)

	case emitFlowMappingFirstKeyState:
		return emitFlowMappingKey(e, event, true, false)

	case emitFlowMappingTrailKeyState:
		return emitFlowMappingKey(e, event, false, true)

	case emitFlowMappingKeyState:
		return emitFlowMappingKey(e, event, false, false)

	case emitFlowMappingSimpleValueState:
		return emitFlowMappingValue(e, event, true)

	case emitFlowMappingValueState:
		return emitFlowMappingValue(e, event, false)

	case emitBlockSequenceFirstItemState:
		return emitBlockSequenceItem(e, event, true)

	case emitBlockSequenceItemState:
		return emitBlockSequenceItem(e, event, false)

	case emitBlockMappingFirstKeyState:
		return emitBlockMappingKey(e, event, true)

	case emitBlockMappingKeyState:
		return emitBlockMappingKey(e, event, false)

	case emitBlockMappingSimpleValueState:
		return emitBlockMappingValue(e, event, true)

	case emitBlockMappingValueState:
		return emitBlockMappingValue(e, event, false)

	case emitEndState:
		return fmt.Errorf("expected nothing after STREAM-END")
	}
	panic("invalid emitter state")
}

// expect STREAM-START.
func emitStreamStart(e *Emitter, event *yamlh.Event) error {
	if event.Type != yamlh.STREAM_START_EVENT {
		return fmt.Errorf("expected STREAM-START")
	}
	if e.encoding == yamlh.ANY_ENCODING {
		e.encoding = event.Encoding
		if e.encoding == yamlh.ANY_ENCODING {
			e.encoding = yamlh.UTF8_ENCODING
		}
	}
	if e.indent < 2 || e.indent > 9 {
		e.indent = 2
	}
	if e.width >= 0 && e.width <= e.indent*2 {
		e.width = 80
	}
	if e.width < 0 {
		e.width = 1<<31 - 1
	}

	e.indentLevel = -1
	e.line = 0
	e.column = 0
	e.lastCharWhitepace = true
	e.lastCharIndent = true
	e.footIndent = -1

	if e.encoding != yamlh.UTF8_ENCODING {
		err := writeBom(e)
		if err != nil {
			return err
		}
	}
	e.state = emitFirstDocumentStartState
	return nil
}

// expect the root node.
func emitDocumentContent(e *Emitter, event *yamlh.Event) error {
	e.states = append(e.states, emitDocumentEndState)
	err := processHeadComment(e)
	if err != nil {
		return err
	}
	err = emitNode(e, event, true, false)
	if err != nil {
		return err
	}
	err = processLineComment(e)
	if err != nil {
		return err
	}
	return processFootComment(e)
}

// expect DOCUMENT-END.
func emitDocumentEnd(e *Emitter, event *yamlh.Event) error {
	if event.Type != yamlh.DOCUMENT_END_EVENT {
		return fmt.Errorf("expected DOCUMENT-END")
	}
	// [Go] Force document foot separation.
	e.footIndent = 0
	err := processFootComment(e)
	if err != nil {
		return err
	}
	e.footIndent = -1
	err = writeIndent(e)
	if err != nil {
		return err
	}
	if !event.Implicit {
		// [Go] Allocate the slice elsewhere.
		err = writeIndicator(e, []byte("..."), true, false, false)
		if err != nil {
			return err
		}
		err = writeIndent(e)
		if err != nil {
			return err
		}
	}
	e.state = emitDocumentStartState
	e.tagDirectives = e.tagDirectives[:0]
	return nil
}

// expect a flow item node.
func emitFlowSequenceItem(e *Emitter, event *yamlh.Event, first, trail bool) error {
	var err error
	if first {
		err = writeIndicator(e, []byte{'['}, true, true, false)
		if err != nil {
			return err
		}
		e.increaseIndent(true, false)
		e.flowLevel++
	}

	if event.Type == yamlh.SEQUENCE_END_EVENT {
		e.flowLevel--
		e.indentLevel = e.indentStack[len(e.indentStack)-1]
		e.indentStack = e.indentStack[:len(e.indentStack)-1]
		if e.column == 0 {
			err = writeIndent(e)
			if err != nil {
				return err
			}
		}
		err = writeIndicator(e, []byte{']'}, false, false, false)
		if err != nil {
			return err
		}
		err = processLineComment(e)
		if err != nil {
			return err
		}
		err = processFootComment(e)
		if err != nil {
			return err
		}
		e.state = e.states[len(e.states)-1]
		e.states = e.states[:len(e.states)-1]

		return nil
	}

	if !first && !trail {
		err = writeIndicator(e, []byte{','}, false, false, false)
		if err != nil {
			return err
		}
	}

	err = processHeadComment(e)
	if err != nil {
		return err
	}
	if e.column == 0 {
		err = writeIndent(e)
		if err != nil {
			return err
		}
	}

	if e.column > e.width {
		err = writeIndent(e)
		if err != nil {
			return err
		}
	}
	if len(e.lineComment)+len(e.footComment)+len(e.tailComment) > 0 {
		e.states = append(e.states, emitFlowSequenceTrailItemState)
	} else {
		e.states = append(e.states, emitFlowSequenceItemState)
	}
	err = emitNode(e, event, false, false)
	if err != nil {
		return err
	}
	if len(e.lineComment)+len(e.footComment)+len(e.tailComment) > 0 {
		err = writeIndicator(e, []byte{','}, false, false, false)
		if err != nil {
			return err
		}
	}
	err = processLineComment(e)
	if err != nil {
		return err
	}
	err = processFootComment(e)
	if err != nil {
		return err
	}
	return nil
}

// expect a flow key node.
func emitFlowMappingKey(e *Emitter, event *yamlh.Event, first, trail bool) error {
	var err error
	if first {
		err = writeIndicator(e, []byte{'{'}, true, true, false)
		if err != nil {
			return err
		}
		e.increaseIndent(true, false)
		e.flowLevel++
	}

	if event.Type == yamlh.MAPPING_END_EVENT {
		if len(e.headComment)+len(e.footComment)+len(e.tailComment) > 0 && !first && !trail {
			err = writeIndicator(e, []byte{','}, false, false, false)
			if err != nil {
				return err
			}
		}
		err = processHeadComment(e)
		if err != nil {
			return err
		}
		e.flowLevel--
		e.indentLevel = e.indentStack[len(e.indentStack)-1]
		e.indentStack = e.indentStack[:len(e.indentStack)-1]
		err = writeIndicator(e, []byte{'}'}, false, false, false)
		if err != nil {
			return err
		}
		err = processLineComment(e)
		if err != nil {
			return err
		}
		err = processFootComment(e)
		if err != nil {
			return err
		}
		e.state = e.states[len(e.states)-1]
		e.states = e.states[:len(e.states)-1]
		return nil
	}

	if !first && !trail {
		err = writeIndicator(e, []byte{','}, false, false, false)
		if err != nil {
			return err
		}
	}

	err = processHeadComment(e)
	if err != nil {
		return err
	}

	if e.column == 0 {
		err = writeIndent(e)
		if err != nil {
			return err
		}
	}

	if e.column > e.width {
		err = writeIndent(e)
		if err != nil {
			return err
		}
	}

	if checkSimpleKey(e) {
		e.states = append(e.states, emitFlowMappingSimpleValueState)
		return emitNode(e, event, false, true)
	}
	err = writeIndicator(e, []byte{'?'}, true, false, false)
	if err != nil {
		return err
	}
	e.states = append(e.states, emitFlowMappingValueState)
	return emitNode(e, event, false, false)
}

// expect a flow value node.
func emitFlowMappingValue(e *Emitter, event *yamlh.Event, simple bool) error {
	var err error
	if simple {
		err = writeIndicator(e, []byte{':'}, false, false, false)
		if err != nil {
			return err
		}
	} else {
		if e.column > e.width {
			err = writeIndent(e)
			if err != nil {
				return err
			}
		}
		err = writeIndicator(e, []byte{':'}, true, false, false)
		if err != nil {
			return err
		}
	}
	if len(e.lineComment)+len(e.footComment)+len(e.tailComment) > 0 {
		e.states = append(e.states, emitFlowMappingTrailKeyState)
	} else {
		e.states = append(e.states, emitFlowMappingKeyState)
	}
	err = emitNode(e, event, false, false)
	if err != nil {
		return err
	}
	if len(e.lineComment)+len(e.footComment)+len(e.tailComment) > 0 {
		err = writeIndicator(e, []byte{','}, false, false, false)
		if err != nil {
			return err
		}
	}
	err = processLineComment(e)
	if err != nil {
		return err
	}
	return processFootComment(e)
}

// expect a block item node.
func emitBlockSequenceItem(e *Emitter, event *yamlh.Event, first bool) error {
	if first {
		e.increaseIndent(false, false)
	}
	if event.Type == yamlh.SEQUENCE_END_EVENT {
		e.indentLevel = e.indentStack[len(e.indentStack)-1]
		e.indentStack = e.indentStack[:len(e.indentStack)-1]
		e.state = e.states[len(e.states)-1]
		e.states = e.states[:len(e.states)-1]
		return nil
	}
	err := processHeadComment(e)
	if err != nil {
		return err
	}
	err = writeIndent(e)
	if err != nil {
		return err
	}
	err = writeIndicator(e, []byte{'-'}, true, false, true)
	if err != nil {
		return err
	}
	e.states = append(e.states, emitBlockSequenceItemState)
	err = emitNode(e, event, false, false)
	if err != nil {
		return err
	}
	err = processLineComment(e)
	if err != nil {
		return err
	}
	return processFootComment(e)
}

// expect a block key node.
func emitBlockMappingKey(e *Emitter, event *yamlh.Event, first bool) error {
	if first {
		e.increaseIndent(false, false)
	}
	err := processHeadComment(e)
	if err != nil {
		return err
	}
	if event.Type == yamlh.MAPPING_END_EVENT {
		e.indentLevel = e.indentStack[len(e.indentStack)-1]
		e.indentStack = e.indentStack[:len(e.indentStack)-1]
		e.state = e.states[len(e.states)-1]
		e.states = e.states[:len(e.states)-1]
		return nil
	}
	err = writeIndent(e)
	if err != nil {
		return err
	}
	if len(e.lineComment) > 0 {
		// [Go] A line comment was provided for the key. That's unusual as the
		//      scanner associates line comments with the value. Either way,
		//      save the line comment and render it appropriately later.
		e.keyLineComment = e.lineComment
		e.lineComment = nil
	}
	if checkSimpleKey(e) {
		e.states = append(e.states, emitBlockMappingSimpleValueState)
		return emitNode(e, event, false, true)
	}
	err = writeIndicator(e, []byte{'?'}, true, false, true)
	if err != nil {
		return err
	}
	e.states = append(e.states, emitBlockMappingValueState)
	return emitNode(e, event, false, false)
}

// expect a block value node.
func emitBlockMappingValue(e *Emitter, event *yamlh.Event, simple bool) error {
	var err error
	if simple {
		err = writeIndicator(e, []byte{':'}, false, false, false)
		if err != nil {
			return err
		}
	} else {
		err = writeIndent(e)
		if err != nil {
			return err
		}
		err = writeIndicator(e, []byte{':'}, true, false, true)
		if err != nil {
			return err
		}
	}
	if len(e.keyLineComment) > 0 {
		// [Go] Line comments are generally associated with the value, but when there's
		//      no value on the same line as a mapping key they end up attached to the
		//      key itself.
		if event.Type == yamlh.SCALAR_EVENT {
			if len(e.lineComment) == 0 {
				// A scalar is coming and it has no line comments by itself yet,
				// so just let it handle the line comment as usual. If it has a
				// line comment, we can't have both so the one from the key is lost.
				e.lineComment = e.keyLineComment
				e.keyLineComment = nil
			}
		} else if event.Sequence_style() != yamlh.FLOW_SEQUENCE_STYLE && (event.Type == yamlh.MAPPING_START_EVENT || event.Type == yamlh.SEQUENCE_START_EVENT) {
			// An indented block follows, so write the comment right now.
			e.lineComment, e.keyLineComment = e.keyLineComment, e.lineComment
			err = processLineComment(e)
			if err != nil {
				return err
			}
			e.lineComment, e.keyLineComment = e.keyLineComment, e.lineComment
		}
	}
	e.states = append(e.states, emitBlockMappingKeyState)
	err = emitNode(e, event, false, false)
	if err != nil {
		return err
	}
	err = processLineComment(e)
	if err != nil {
		return err
	}
	return processFootComment(e)
}

// expect a node.
func emitNode(e *Emitter, event *yamlh.Event, root, simpleKey bool) error {
	e.rootContext = root
	e.simpleKeyContext = simpleKey

	switch event.Type {
	case yamlh.ALIAS_EVENT:
		return emitAlias(e, event)
	case yamlh.SCALAR_EVENT:
		return emitScalar(e, event)
	case yamlh.SEQUENCE_START_EVENT:
		return emitSequenceStart(e, event)
	case yamlh.MAPPING_START_EVENT:
		return emitMappingStart(e, event)
	default:
		return fmt.Errorf("expected SCALAR, SEQUENCE-START, MAPPING-START, or ALIAS, but got %v", event.Type)
	}
}

// expect ALIAS.
func emitAlias(e *Emitter, event *yamlh.Event) error {
	err := processAnchor(e)
	if err != nil {
		return err
	}
	e.state = e.states[len(e.states)-1]
	e.states = e.states[:len(e.states)-1]
	return nil
}

// expect SCALAR.
func emitScalar(e *Emitter, event *yamlh.Event) error {
	err := selectScalarStyle(e, event)
	if err != nil {
		return err
	}
	err = processAnchor(e)
	if err != nil {
		return err
	}
	err = processTag(e)
	if err != nil {
		return err
	}
	e.increaseIndent(true, false)
	err = processScalar(e)
	if err != nil {
		return err
	}
	e.indentLevel = e.indentStack[len(e.indentStack)-1]
	e.indentStack = e.indentStack[:len(e.indentStack)-1]
	e.state = e.states[len(e.states)-1]
	e.states = e.states[:len(e.states)-1]
	return nil
}

// expect SEQUENCE-START.
func emitSequenceStart(e *Emitter, event *yamlh.Event) error {
	err := processAnchor(e)
	if err != nil {
		return err
	}
	err = processTag(e)
	if err != nil {
		return err
	}
	if e.flowLevel > 0 || event.Sequence_style() == yamlh.FLOW_SEQUENCE_STYLE ||
		checkEmptySequence(e) {
		e.state = emitFlowSequenceFirstItemState
	} else {
		e.state = emitBlockSequenceFirstItemState
	}
	return nil
}

// expect MAPPING-START.
func emitMappingStart(e *Emitter, event *yamlh.Event) error {
	err := processAnchor(e)
	if err != nil {
		return err
	}
	err = processTag(e)
	if err != nil {
		return err
	}
	if e.flowLevel > 0 || event.Mapping_style() == yamlh.FLOW_MAPPING_STYLE ||
		checkEmptyMapping(e) {
		e.state = emitFlowMappingFirstKeyState
	} else {
		e.state = emitBlockMappingFirstKeyState
	}
	return nil
}
