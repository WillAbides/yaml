package emitter

import "github.com/willabides/yaml/internal/yamlh"

// writeBom writes the BOM character.
func writeBom(e *Emitter) error {
	return e.writeAll([]byte("\xEF\xBB\xBF"))
}

func writeIndent(e *Emitter) error {
	indent := e.indentLevel
	if indent < 0 {
		indent = 0
	}
	if !e.lastCharIndent || e.column > indent || (e.column == indent && !e.lastCharWhitepace) {
		err := e.putBreak()
		if err != nil {
			return err
		}
	}
	if e.footIndent == indent {
		err := e.putBreak()
		if err != nil {
			return err
		}
	}
	for e.column < indent {
		err := e.put(' ')
		if err != nil {
			return err
		}
	}
	e.lastCharWhitepace = true
	e.footIndent = -1
	return nil
}

func writeIndicator(e *Emitter, indicator []byte, need_whitespace, is_whitespace, is_indention bool) error {
	if need_whitespace && !e.lastCharWhitepace {
		err := e.put(' ')
		if err != nil {
			return err
		}
	}
	err := e.writeAll(indicator)
	if err != nil {
		return err
	}
	e.lastCharWhitepace = is_whitespace
	e.lastCharIndent = (e.lastCharIndent && is_indention)
	e.openEnded = false
	return nil
}

func writeAnchor(e *Emitter, value []byte) error {
	err := e.writeAll(value)
	if err != nil {
		return err
	}
	e.lastCharWhitepace = false
	e.lastCharIndent = false
	return nil
}

func writeTagHandle(e *Emitter, value []byte) error {
	if !e.lastCharWhitepace {
		err := e.put(' ')
		if err != nil {
			return err
		}
	}
	err := e.writeAll(value)
	if err != nil {
		return err
	}
	e.lastCharWhitepace = false
	e.lastCharIndent = false
	return nil
}

func writeTagContent(e *Emitter, value []byte, need_whitespace bool) error {
	if need_whitespace && !e.lastCharWhitepace {
		err := e.put(' ')
		if err != nil {
			return err
		}
	}
	for len(value) > 0 {
		var must_write bool
		switch value[0] {
		case ';', '/', '?', ':', '@', '&', '=', '+', '$', ',', '_', '.', '~', '*', '\'', '(', ')', '[', ']':
			must_write = true
		default:
			must_write = yamlh.Is_alpha(value, 0)
		}
		if must_write {
			n, err := e.write(value)
			if err != nil {
				return err
			}
			value = value[n:]
			continue
		}
		w := yamlh.Width(value[0])
		for k := 0; k < w; k++ {
			octet := value[0]
			err := e.put('%')
			if err != nil {
				return err
			}

			c := octet >> 4
			if c < 10 {
				c += '0'
			} else {
				c += 'A' - 10
			}
			err = e.put(c)
			if err != nil {
				return err
			}

			c = octet & 0x0f
			if c < 10 {
				c += '0'
			} else {
				c += 'A' - 10
			}
			err = e.put(c)
			if err != nil {
				return err
			}
		}
		value = value[w:]
	}
	e.lastCharWhitepace = false
	e.lastCharIndent = false
	return nil
}

func writePlainScalar(e *Emitter, value []byte, allowBreaks bool) error {
	var err error
	totalLen := len(value)
	if totalLen > 0 && !e.lastCharWhitepace {
		err = e.put(' ')
		if err != nil {
			return err
		}
	}

	spaces := false
	breaks := false
	for len(value) > 0 {
		w := yamlh.Width(value[0])
		if yamlh.Is_space(value, 0) {
			nextIsSpace := len(value) > w && yamlh.Is_space(value, w)
			if allowBreaks && !spaces && e.column > e.width && !nextIsSpace {
				err = writeIndent(e)
				if err != nil {
					return err
				}
			} else {
				w, err = e.write(value)
				if err != nil {
					return err
				}
			}
			value = value[w:]
			spaces = true
			continue
		}
		if yamlh.Is_break(value, 0) {
			if !breaks && value[0] == '\n' {
				err = e.putBreak()
				if err != nil {
					return err
				}
			}
			w, err = e.writeBreak(value)
			if err != nil {
				return err
			}
			value = value[w:]
			breaks = true
			continue
		}
		if breaks {
			err = writeIndent(e)
			if err != nil {
				return err
			}
		}
		w, err = e.write(value)
		if err != nil {
			return err
		}
		value = value[w:]
		e.lastCharIndent = false
		spaces = false
		breaks = false
	}

	if totalLen > 0 {
		e.lastCharWhitepace = false
	}
	e.lastCharIndent = false
	if e.rootContext {
		e.openEnded = true
	}

	return nil
}

func writeSingleQuotedScalar(e *Emitter, value []byte, allow_breaks bool) error {
	err := writeIndicator(e, []byte{'\''}, true, false, false)
	if err != nil {
		return err
	}

	spaces := false
	breaks := false
	count := 0
	for len(value) > 0 {
		count++
		w := yamlh.Width(value[0])
		hasMore := len(value) > w
		if yamlh.Is_space(value, 0) {
			if allow_breaks &&
				!spaces &&
				e.column > e.width &&
				count > 1 &&
				hasMore &&
				!yamlh.Is_space(value, 1) {
				err = writeIndent(e)
				if err != nil {
					return err
				}
			} else {
				w, err = e.write(value)
				if err != nil {
					return err
				}
			}
			spaces = true
			value = value[w:]
			continue
		}
		if yamlh.Is_break(value, 0) {
			if !breaks && value[0] == '\n' {
				err = e.putBreak()
				if err != nil {
					return err
				}
			}
			w, err = e.writeBreak(value)
			if err != nil {
				return err
			}
			breaks = true
			value = value[w:]
			continue
		}
		if breaks {
			err = writeIndent(e)
			if err != nil {
				return err
			}
		}
		if value[0] == '\'' {
			err = e.put('\'')
			if err != nil {
				return err
			}
		}
		w, err = e.write(value)
		if err != nil {
			return err
		}
		value = value[w:]
		e.lastCharIndent = false
		spaces = false
		breaks = false
	}
	err = writeIndicator(e, []byte{'\''}, false, false, false)
	if err != nil {
		return err
	}
	e.lastCharWhitepace = false
	e.lastCharIndent = false
	return nil
}

func writeDoubleQuotedScalar(e *Emitter, value []byte, allow_breaks bool) error {
	spaces := false
	err := writeIndicator(e, []byte{'"'}, true, false, false)
	if err != nil {
		return err
	}
	isBom := false
	if len(value) >= 3 {
		isBom = yamlh.Is_bom(value)
	}
	count := 0
	for len(value) > 0 {
		var w int
		count++
		if !yamlh.IsPrintable(value) ||
			isBom || yamlh.Is_break(value, 0) ||
			value[0] == '"' || value[0] == '\\' {

			value, err = writeDoubleQuotedEscapedChar(e, value)
			if err != nil {
				return err
			}
			spaces = false
			continue
		}
		if yamlh.Is_space(value, 0) {
			w = yamlh.Width(value[0])
			if allow_breaks && !spaces && e.column > e.width && count > 1 && len(value) > w {
				err = writeIndent(e)
				if err != nil {
					return err
				}
				if yamlh.Is_space(value, 1) {
					err = e.put('\\')
					if err != nil {
						return err
					}
				}
			} else {
				w, err = e.write(value)
				if err != nil {
					return err
				}
			}
			value = value[w:]
			spaces = true
			continue
		}
		w, err = e.write(value)
		if err != nil {
			return err
		}
		value = value[w:]
		spaces = false
	}
	err = writeIndicator(e, []byte{'"'}, false, false, false)
	if err != nil {
		return err
	}
	e.lastCharWhitepace = false
	e.lastCharIndent = false
	return nil
}

func writeDoubleQuotedEscapedChar(e *Emitter, value []byte) ([]byte, error) {
	octet := value[0]

	var v rune
	var w int
	switch {
	case octet&0x80 == 0x00:
		w, v = 1, rune(octet&0x7F)
	case octet&0xE0 == 0xC0:
		w, v = 2, rune(octet&0x1F)
	case octet&0xF0 == 0xE0:
		w, v = 3, rune(octet&0x0F)
	case octet&0xF8 == 0xF0:
		w, v = 4, rune(octet&0x07)
	}
	for k := 1; k < w; k++ {
		octet = value[k]
		v = (v << 6) + (rune(octet) & 0x3F)
	}
	value = value[w:]

	err := e.put('\\')
	if err != nil {
		return nil, err
	}

	switch v {
	case 0x00:
		err = e.put('0')
	case 0x07:
		err = e.put('a')
	case 0x08:
		err = e.put('b')
	case 0x09:
		err = e.put('t')
	case 0x0A:
		err = e.put('n')
	case 0x0b:
		err = e.put('v')
	case 0x0c:
		err = e.put('f')
	case 0x0d:
		err = e.put('r')
	case 0x1b:
		err = e.put('e')
	case 0x22:
		err = e.put('"')
	case 0x5c:
		err = e.put('\\')
	case 0x85:
		err = e.put('N')
	case 0xA0:
		err = e.put('_')
	case 0x2028:
		err = e.put('L')
	case 0x2029:
		err = e.put('P')
	default:
		switch {
		case v <= 0xFF:
			err = e.put('x')
			w = 2
		case v <= 0xFFFF:
			err = e.put('u')
			w = 4
		default:
			err = e.put('U')
			w = 8
		}
		if err != nil {
			return nil, err
		}
		for k := (w - 1) * 4; err == nil && k >= 0; k -= 4 {
			digit := byte((v >> uint(k)) & 0x0F)
			if digit < 10 {
				err = e.put(digit + '0')
			} else {
				err = e.put(digit + 'A' - 10)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

func writeBlockScalarHints(e *Emitter, value []byte) error {
	var err error
	if yamlh.Is_space(value, 0) || yamlh.Is_break(value, 0) {
		indent_hint := []byte{'0' + byte(e.indent)}
		err = writeIndicator(e, indent_hint, false, false, false)
		if err != nil {
			return err
		}
	}

	e.openEnded = false

	var chomp_hint [1]byte
	if len(value) == 0 {
		chomp_hint[0] = '-'
	} else {
		i := len(value) - 1
		for value[i]&0xC0 == 0x80 {
			i--
		}
		switch {
		case !yamlh.Is_break(value, i):
			chomp_hint[0] = '-'
		case i == 0:
			chomp_hint[0] = '+'
			e.openEnded = true
		default:
			i--
			for value[i]&0xC0 == 0x80 {
				i--
			}
			if yamlh.Is_break(value, i) {
				chomp_hint[0] = '+'
				e.openEnded = true
			}
		}
	}
	if chomp_hint[0] != 0 {
		err = writeIndicator(e, chomp_hint[:], false, false, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeLiteralScalar(e *Emitter, value []byte) error {
	err := writeIndicator(e, []byte{'|'}, true, false, false)
	if err != nil {
		return err
	}
	err = writeBlockScalarHints(e, value)
	if err != nil {
		return err
	}
	err = processLineComment(e)
	if err != nil {
		return err
	}
	e.lastCharWhitepace = true
	breaks := true
	for len(value) > 0 {
		var w int
		if yamlh.Is_break(value, 0) {
			w, err = e.writeBreak(value)
			if err != nil {
				return err
			}
			breaks = true
			value = value[w:]
			continue
		}
		if breaks {
			err = writeIndent(e)
			if err != nil {
				return err
			}
		}
		w, err = e.write(value)
		if err != nil {
			return err
		}
		value = value[w:]
		e.lastCharIndent = false
		breaks = false
	}
	return nil
}

func writeFoldedScalar(e *Emitter, value []byte) error {
	err := writeIndicator(e, []byte{'>'}, true, false, false)
	if err != nil {
		return err
	}
	err = writeBlockScalarHints(e, value)
	if err != nil {
		return err
	}
	err = processLineComment(e)
	if err != nil {
		return err
	}

	e.lastCharWhitepace = true

	breaks := true
	leading_spaces := true
	for len(value) > 0 {
		w := yamlh.Width(value[0])
		if yamlh.Is_break(value, 0) {
			if !breaks && !leading_spaces && value[0] == '\n' {
				k := 0
				for yamlh.Is_break(value, k) {
					k += yamlh.Width(value[k])
				}
				if !yamlh.Is_blankz(value, k) {
					err = e.putBreak()
					if err != nil {
						return err
					}
				}
			}
			w, err = e.writeBreak(value)
			if err != nil {
				return err
			}
			value = value[w:]
			breaks = true
			continue
		}
		if breaks {
			err = writeIndent(e)
			if err != nil {
				return err
			}
			leading_spaces = yamlh.Is_blank(value, 0)
		}
		nextIsSpace := len(value) > w && yamlh.Is_space(value, w)
		if !breaks && yamlh.Is_space(value, 0) && !nextIsSpace && e.column > e.width {
			err = writeIndent(e)
			if err != nil {
				return err
			}
		} else {
			w, err = e.write(value)
			if err != nil {
				return err
			}
		}
		value = value[w:]
		e.lastCharIndent = false
		breaks = false
	}
	return nil
}

func writeComment(e *Emitter, comment []byte) error {
	breaks := false
	pound := false
	for len(comment) > 0 {
		if yamlh.Is_break(comment, 0) {
			n, err := e.writeBreak(comment)
			if err != nil {
				return err
			}
			comment = comment[n:]
			breaks = true
			pound = false
			continue
		}
		if breaks {
			err := writeIndent(e)
			if err != nil {
				return err
			}
		}
		if !pound {
			if comment[0] != '#' {
				err := e.writeAll([]byte("# "))
				if err != nil {
					return err
				}
			}
			pound = true
		}
		n, err := e.write(comment)
		if err != nil {
			return err
		}
		comment = comment[n:]
		e.lastCharIndent = false
		breaks = false
	}
	if !breaks {
		err := e.putBreak()
		if err != nil {
			return err
		}
	}
	e.lastCharWhitepace = true
	return nil
}
