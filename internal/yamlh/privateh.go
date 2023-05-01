package yamlh

const (
	// The size of the input raw buffer.
	Input_raw_buffer_size = 512

	// The size of the input buffer.
	// It should be possible to decode the whole raw buffer.
	Input_buffer_size = Input_raw_buffer_size * 3

	// The size of other stacks and queues.
	Initial_stack_size = 16
	Initial_queue_size = 16
)

// Check if the character at the specified position is an alphabetical
// character, a digit, '_', or '-'.
func Is_alpha(b []byte, i int) bool {
	return b[i] >= '0' && b[i] <= '9' || b[i] >= 'A' && b[i] <= 'Z' || b[i] >= 'a' && b[i] <= 'z' || b[i] == '_' || b[i] == '-'
}

// Check if the character at the specified position is a digit.
func Is_digit(b []byte, i int) bool {
	return b[i] >= '0' && b[i] <= '9'
}

// Get the value of a digit.
func As_digit(b []byte, i int) int {
	return int(b[i]) - '0'
}

// Check if the character at the specified position is a hex-digit.
func Is_hex(b []byte, i int) bool {
	return b[i] >= '0' && b[i] <= '9' || b[i] >= 'A' && b[i] <= 'F' || b[i] >= 'a' && b[i] <= 'f'
}

// Get the value of a hex-digit.
func As_hex(b []byte, i int) int {
	bi := b[i]
	if bi >= 'A' && bi <= 'F' {
		return int(bi) - 'A' + 10
	}
	if bi >= 'a' && bi <= 'f' {
		return int(bi) - 'a' + 10
	}
	return int(bi) - '0'
}

// Check if the character at the start of the buffer can be printed unescaped.
func IsPrintable(b []byte) bool {
	return (b[0] == 0x0A) || // . == #x0A
		(b[0] >= 0x20 && b[0] <= 0x7E) || // #x20 <= . <= #x7E
		(b[0] == 0xC2 && b[0+1] >= 0xA0) || // #0xA0 <= . <= #xD7FF
		(b[0] > 0xC2 && b[0] < 0xED) ||
		(b[0] == 0xED && b[0+1] < 0xA0) ||
		(b[0] == 0xEE) ||
		(b[0] == 0xEF && // #xE000 <= . <= #xFFFD
			!(b[0+1] == 0xBB && b[0+2] == 0xBF) && // && . != #xFEFF
			!(b[0+1] == 0xBF && (b[0+2] == 0xBE || b[0+2] == 0xBF)))
}

// Check if the character at the specified position is NUL.
func Is_z(b []byte, i int) bool {
	return b[i] == 0x00
}

// Check if the beginning of the buffer is a BOM.
func Is_bom(b []byte) bool {
	return b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF
}

// Check if the character at the specified position is space.
func Is_space(b []byte, i int) bool {
	return b[i] == ' '
}

// Check if the character at the specified position is tab.
func Is_tab(b []byte, i int) bool {
	return b[i] == '\t'
}

// Check if the character at the specified position is blank (space or tab).
func Is_blank(b []byte, i int) bool {
	return b[i] == ' ' || b[i] == '\t'
}

func IsBlank(b byte) bool {
	return b == ' ' || b == '\t'
}

// Is_break - Check if the character at the specified position is a line break.
func Is_break(b []byte, i int) bool {
	return b[i] == '\r' || // CR (#xD)
		b[i] == '\n' || // LF (#xA)
		b[i] == 0xC2 && b[i+1] == 0x85 || // NEL (#x85)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA8 || // LS (#x2028)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA9 // PS (#x2029)
}

func IsBreak(b []byte) bool {
	return b[0] == '\r' || // CR (#xD)
		b[0] == '\n' || // LF (#xA)
		b[0] == 0xC2 && b[1] == 0x85 || // NEL (#x85)
		b[0] == 0xE2 && b[1] == 0x80 && b[2] == 0xA8 || // LS (#x2028)
		b[0] == 0xE2 && b[1] == 0x80 && b[2] == 0xA9 // PS (#x2029)
}

func Is_crlf(b []byte, i int) bool {
	return b[i] == '\r' && b[i+1] == '\n'
}

// Check if the character is a line break or NUL.
func Is_breakz(b []byte, i int) bool {
	return b[i] == '\r' || // CR (#xD)
		b[i] == '\n' || // LF (#xA)
		b[i] == 0xC2 && b[i+1] == 0x85 || // NEL (#x85)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA8 || // LS (#x2028)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA9 || // PS (#x2029)
		// is_z:
		b[i] == 0
}

// Check if the character is a line break, space, or NUL.
func Is_spacez(b []byte, i int) bool {
	return b[i] == ' ' ||
		// is_breakz:
		b[i] == '\r' || // CR (#xD)
		b[i] == '\n' || // LF (#xA)
		b[i] == 0xC2 && b[i+1] == 0x85 || // NEL (#x85)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA8 || // LS (#x2028)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA9 || // PS (#x2029)
		b[i] == 0
}

// Check if the character is a line break, space, tab, or NUL.
func Is_blankz(b []byte, i int) bool {
	return b[i] == ' ' || b[i] == '\t' ||
		b[i] == '\r' || // CR (#xD)
		b[i] == '\n' || // LF (#xA)
		b[i] == 0xC2 && b[i+1] == 0x85 || // NEL (#x85)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA8 || // LS (#x2028)
		b[i] == 0xE2 && b[i+1] == 0x80 && b[i+2] == 0xA9 || // PS (#x2029)
		b[i] == 0
}

func IsBlankz(b []byte) bool {
	return b[0] == ' ' || b[0] == '\t' ||
		b[0] == '\r' || // CR (#xD)
		b[0] == '\n' || // LF (#xA)
		b[0] == 0xC2 && b[1] == 0x85 || // NEL (#x85)
		b[0] == 0xE2 && b[1] == 0x80 && b[2] == 0xA8 || // LS (#x2028)
		b[0] == 0xE2 && b[1] == 0x80 && b[2] == 0xA9 || // PS (#x2029)
		b[0] == 0
}

// Determine the width of the character.
func Width(b byte) int {
	// Don't replace these by a switch without first
	// confirming that it is being inlined.
	if b&0x80 == 0x00 {
		return 1
	}
	if b&0xE0 == 0xC0 {
		return 2
	}
	if b&0xF0 == 0xE0 {
		return 3
	}
	if b&0xF8 == 0xF0 {
		return 4
	}
	return 0
}
