package emitter

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/willabides/yaml/internal/yamlh"
	"io"
)

type emitterState int

// The emitter states.
const (
	emitStreamStartState emitterState = iota

	emitFirstDocumentStartState      // expect the first DOCUMENT-START or STREAM-END.
	emitDocumentStartState           // expect DOCUMENT-START or STREAM-END.
	EmitDocumentContentState         // expect the content of a document.
	emitDocumentEndState             // expect DOCUMENT-END.
	emitFlowSequenceFirstItemState   // expect the first item of a flow sequence.
	emitFlowSequenceTrailItemState   // expect the next item of a flow sequence, with the comma already written out
	emitFlowSequenceItemState        // expect an item of a flow sequence.
	emitFlowMappingFirstKeyState     // expect the first key of a flow mapping.
	emitFlowMappingTrailKeyState     // expect the next key of a flow mapping, with the comma already written out
	emitFlowMappingKeyState          // expect a key of a flow mapping.
	emitFlowMappingSimpleValueState  // expect a value for a simple key of a flow mapping.
	emitFlowMappingValueState        // expect a value of a flow mapping.
	emitBlockSequenceFirstItemState  // expect the first item of a block sequence.
	emitBlockSequenceItemState       // expect an item of a block sequence.
	emitBlockMappingFirstKeyState    // expect the first key of a block mapping.
	emitBlockMappingKeyState         // expect the key of a block mapping.
	emitBlockMappingSimpleValueState // expect a value for a simple key of a block mapping.
	emitBlockMappingValueState       // expect a value of a block mapping.
	emitEndState                     // expect nothing.
)

type Emitter struct {

	// Writer stuff
	writer io.Writer

	encoding yamlh.Encoding // The stream Encoding.

	// Emitter stuff

	indent int // The number of indentation spaces.
	width  int // The preferred width of the output lines.

	state  emitterState   // The current emitter State.
	states []emitterState // The stack of States.

	eventsQueue []yamlh.Event // The event queue.
	eventsHead  int           // The head of the event queue.

	indentStack []int // The stack of indentation levels.

	tagDirectives []yamlh.TagDirective // The list of tag directives.

	indentLevel int // The current indentation level.

	flowLevel int // The current flow level.

	rootContext      bool // Is it the document root context?
	simpleKeyContext bool // Is it a simple mapping key context?

	line              int  // The current Line.
	column            int  // The current Column.
	lastCharWhitepace bool // If the last character was a Whitespace?
	lastCharIndent    bool // If the last character was an indentation character (' ', '-', '?', ':')?
	openEnded         bool // If an explicit document end is required?

	footIndent int // The Indent used to write the foot comment above, or -1 if none.

	// Anchor analysis.
	anchorData struct {
		Anchor []byte // The anchor value.
		Alias  bool   // Is it an alias?
	}

	// Tag analysis.
	tagData struct {
		Handle []byte // The tag handle.
		Suffix []byte // The tag suffix.
	}

	// Scalar analysis.
	scalarData struct {
		value               []byte                // The scalar value.
		multiline           bool                  // Does the scalar contain Line breaks?
		flowPlainAllowed    bool                  // Can the scalar be expessed in the flow plain style?
		blockPlainAllowed   bool                  // Can the scalar be expressed in the block plain style?
		singleQuotedAllowed bool                  // Can the scalar be expressed in the single quoted style?
		blockAllowed        bool                  // Can the scalar be expressed in the literal or folded styles?
		style               yamlh.YamlScalarStyle // The output style.
	}

	// Comments
	headComment    []byte
	lineComment    []byte
	footComment    []byte
	tailComment    []byte
	keyLineComment []byte
}

func New(w io.Writer) *Emitter {
	return &Emitter{
		writer:      w,
		states:      make([]emitterState, 0, yamlh.Initial_stack_size),
		eventsQueue: make([]yamlh.Event, 0, yamlh.Initial_queue_size),
		width:       -1,
		indent:      4,
	}
}

// Emit an event.
func (e *Emitter) Emit(event *yamlh.Event, final bool) error {
	if final {
		e.openEnded = false
	}
	e.eventsQueue = append(e.eventsQueue, *event)
	for e.readyToEmit() {
		err := analyzeEvent(e, &e.eventsQueue[e.eventsHead])
		if err != nil {
			return err
		}
		err = stateMachine(e, &e.eventsQueue[e.eventsHead])
		if err != nil {
			return err
		}
		e.eventsHead++
	}
	return nil
}

func (e *Emitter) SetIndent(spaces int) {
	if spaces < 0 {
		panic("yaml: cannot indent to a negative number of spaces")
	}
	e.indent = spaces
}

// put a byte on the output buffer.
func (e *Emitter) put(value byte) error {
	_, err := e.writer.Write([]byte{value})
	if err != nil {
		return fmt.Errorf("yaml: write error: %v", err)
	}
	e.column++
	return nil
}

// putBreak puts a line break to the output buffer.
func (e *Emitter) putBreak() error {
	_, err := e.writer.Write([]byte{'\n'})
	if err != nil {
		return fmt.Errorf("yaml: write error: %v", err)
	}
	e.column = 0
	e.line++
	// [Go] Do this here and below and drop from everywhere else (see commented lines).
	e.lastCharIndent = true
	return nil
}

// write a character from b onto the buffer. Returns the number of bytes read from b.
func (e *Emitter) write(b []byte) (int, error) {
	w := yamlh.Width(b[0])
	_, err := io.CopyN(e.writer, bytes.NewReader(b), int64(w))
	if err != nil {
		return 0, fmt.Errorf("yaml: write error: %v", err)
	}
	e.column++
	return w, nil
}

// writeAll writes b to the output buffer.
func (e *Emitter) writeAll(b []byte) error {
	e.column += len([]rune(string(b)))
	for len(b) > 0 {
		n, err := e.writer.Write(b)
		if err != nil {
			return fmt.Errorf("yaml: write error: %v", err)
		}
		b = b[n:]
	}
	return nil
}

// writeBreak writes a line break from b[0] to the output buffer with special handling for \n.
// Returns number of bytes read from b.
func (e *Emitter) writeBreak(b []byte) (int, error) {
	if b[0] == '\n' {
		err := e.putBreak()
		if err != nil {
			return 0, err
		}
		return 1, nil
	}
	n, err := e.write(b)
	if err != nil {
		return 0, err
	}
	e.column = 0
	e.line++
	// [Go] Do this here and above and drop from everywhere else (see commented lines).
	e.lastCharIndent = true
	return n, nil
}

// readyToEmit - Check if we need to accumulate more events before emitting.
//
// We accumulate extra
//   - 1 event for DOCUMENT-START
//   - 2 events for SEQUENCE-START
//   - 3 events for MAPPING-START
func (e *Emitter) readyToEmit() bool {
	if e.eventsHead == len(e.eventsQueue) {
		return false
	}
	var accumulate int
	switch e.eventsQueue[e.eventsHead].Type {
	case yamlh.DOCUMENT_START_EVENT:
		accumulate = 1
		break
	case yamlh.SEQUENCE_START_EVENT:
		accumulate = 2
		break
	case yamlh.MAPPING_START_EVENT:
		accumulate = 3
		break
	default:
		return true
	}
	if len(e.eventsQueue)-e.eventsHead > accumulate {
		return true
	}
	var level int
	for i := e.eventsHead; i < len(e.eventsQueue); i++ {
		switch e.eventsQueue[i].Type {
		case yamlh.STREAM_START_EVENT, yamlh.DOCUMENT_START_EVENT, yamlh.SEQUENCE_START_EVENT, yamlh.MAPPING_START_EVENT:
			level++
		case yamlh.STREAM_END_EVENT, yamlh.DOCUMENT_END_EVENT, yamlh.SEQUENCE_END_EVENT, yamlh.MAPPING_END_EVENT:
			level--
		}
		if level == 0 {
			return true
		}
	}
	return false
}

func (e *Emitter) increaseIndent(flow, indentless bool) {
	e.indentStack = append(e.indentStack, e.indentLevel)
	if e.indentLevel < 0 {
		if flow {
			e.indentLevel = e.indent
		} else {
			e.indentLevel = 0
		}
		return
	}
	if !indentless {
		// [Go] This was changed so that indentations are more regular.
		if e.states[len(e.states)-1] == emitBlockSequenceItemState {
			// The first indent inside a sequence will just skip the "- " indicator.
			e.indentLevel += 2
		} else {
			// Everything else aligns to the chosen indentation.
			e.indentLevel = e.indent * ((e.indentLevel + e.indent) / e.indent)
		}
	}
}

// appendTagDirective - Append a directive to the directives stack.
func appendTagDirective(e *Emitter, value *yamlh.TagDirective, allow_duplicates bool) error {
	for i := 0; i < len(e.tagDirectives); i++ {
		if bytes.Equal(value.Handle, e.tagDirectives[i].Handle) {
			if allow_duplicates {
				return nil
			}
			return errors.New("duplicate %TAG directive")
		}
	}

	// [Go] Do we actually need to copy this given garbage collection
	// and the lack of deallocating destructors?
	tag_copy := yamlh.TagDirective{
		Handle: make([]byte, len(value.Handle)),
		Prefix: make([]byte, len(value.Prefix)),
	}
	copy(tag_copy.Handle, value.Handle)
	copy(tag_copy.Prefix, value.Prefix)
	e.tagDirectives = append(e.tagDirectives, tag_copy)
	return nil
}
