package emitter

import "github.com/willabides/yaml/internal/yamlh"

func processLineComment(e *Emitter) error {
	if len(e.lineComment) == 0 {
		return nil
	}
	var err error
	if !e.lastCharWhitepace {
		err = e.put(' ')
		if err != nil {
			return err
		}
	}
	err = writeComment(e, e.lineComment)
	if err != nil {
		return err
	}
	e.lineComment = e.lineComment[:0]
	return nil
}

func processAnchor(e *Emitter) error {
	if e.anchorData.Anchor == nil {
		return nil
	}
	c := []byte{'&'}
	if e.anchorData.Alias {
		c[0] = '*'
	}
	if err := writeIndicator(e, c, true, false, false); err != nil {
		return err
	}
	return writeAnchor(e, e.anchorData.Anchor)
}

func processTag(e *Emitter) error {
	if len(e.tagData.Handle) == 0 && len(e.tagData.Suffix) == 0 {
		return nil
	}
	var err error
	if len(e.tagData.Handle) > 0 {
		err = writeTagHandle(e, e.tagData.Handle)
		if err != nil {
			return err
		}
		if len(e.tagData.Suffix) > 0 {
			err = writeTagContent(e, e.tagData.Suffix, false)
			if err != nil {
				return err
			}
		}
	} else {
		// [Go] Allocate these slices elsewhere.
		err = writeIndicator(e, []byte("!<"), true, false, false)
		if err != nil {
			return err
		}
		err = writeTagContent(e, e.tagData.Suffix, false)
		if err != nil {
			return err
		}
		err = writeIndicator(e, []byte{'>'}, false, false, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func processScalar(e *Emitter) error {
	switch e.scalarData.style {
	case yamlh.PLAIN_SCALAR_STYLE:
		return writePlainScalar(e, e.scalarData.value, !e.simpleKeyContext)

	case yamlh.SINGLE_QUOTED_SCALAR_STYLE:
		return writeSingleQuotedScalar(e, e.scalarData.value, !e.simpleKeyContext)

	case yamlh.DOUBLE_QUOTED_SCALAR_STYLE:
		return writeDoubleQuotedScalar(e, e.scalarData.value, !e.simpleKeyContext)

	case yamlh.LITERAL_SCALAR_STYLE:
		return writeLiteralScalar(e, e.scalarData.value)

	case yamlh.FOLDED_SCALAR_STYLE:
		return writeFoldedScalar(e, e.scalarData.value)
	}
	panic("unknown scalar style")
}

func processHeadComment(e *Emitter) error {
	var err error
	if len(e.tailComment) > 0 {
		err = writeIndent(e)
		if err != nil {
			return err
		}
		err = writeComment(e, e.tailComment)
		if err != nil {
			return err
		}
		e.tailComment = e.tailComment[:0]
		e.footIndent = e.indentLevel
		if e.footIndent < 0 {
			e.footIndent = 0
		}
	}

	if len(e.headComment) == 0 {
		return nil
	}
	err = writeIndent(e)
	if err != nil {
		return err
	}
	err = writeComment(e, e.headComment)
	if err != nil {
		return err
	}
	e.headComment = e.headComment[:0]
	return nil
}

func processFootComment(e *Emitter) error {
	if len(e.footComment) == 0 {
		return nil
	}
	err := writeIndent(e)
	if err != nil {
		return err
	}
	err = writeComment(e, e.footComment)
	if err != nil {
		return err
	}
	e.footComment = e.footComment[:0]
	e.footIndent = e.indentLevel
	if e.footIndent < 0 {
		e.footIndent = 0
	}
	return nil
}
