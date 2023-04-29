package emitter

import "github.com/willabides/yaml/internal/yamlh"

// Check if the next events represent an empty sequence.
func checkEmptySequence(e *Emitter) bool {
	if len(e.eventsQueue)-e.eventsHead < 2 {
		return false
	}
	return e.eventsQueue[e.eventsHead].Type == yamlh.SEQUENCE_START_EVENT &&
		e.eventsQueue[e.eventsHead+1].Type == yamlh.SEQUENCE_END_EVENT
}

// Check if the next events represent an empty mapping.
func checkEmptyMapping(e *Emitter) bool {
	if len(e.eventsQueue)-e.eventsHead < 2 {
		return false
	}
	return e.eventsQueue[e.eventsHead].Type == yamlh.MAPPING_START_EVENT &&
		e.eventsQueue[e.eventsHead+1].Type == yamlh.MAPPING_END_EVENT
}

// Check if the next node can be expressed as a simple key.
func checkSimpleKey(e *Emitter) bool {
	length := 0
	switch e.eventsQueue[e.eventsHead].Type {
	case yamlh.ALIAS_EVENT:
		length += len(e.anchorData.Anchor)
	case yamlh.SCALAR_EVENT:
		if e.scalarData.multiline {
			return false
		}
		length += len(e.anchorData.Anchor) +
			len(e.tagData.Handle) +
			len(e.tagData.Suffix) +
			len(e.scalarData.value)
	case yamlh.SEQUENCE_START_EVENT:
		if !checkEmptySequence(e) {
			return false
		}
		length += len(e.anchorData.Anchor) +
			len(e.tagData.Handle) +
			len(e.tagData.Suffix)
	case yamlh.MAPPING_START_EVENT:
		if !checkEmptyMapping(e) {
			return false
		}
		length += len(e.anchorData.Anchor) +
			len(e.tagData.Handle) +
			len(e.tagData.Suffix)
	default:
		return false
	}
	return length <= 128
}
