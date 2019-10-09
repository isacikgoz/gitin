// +build windows
package term

import (
	"unsafe"
)

type inputRecord struct {
	eventType uint16
	padding   uint16
	event     [16]byte
}

type keyEventRecord struct {
	bKeyDown          int32
	wRepeatCount      uint16
	wVirtualKeyCode   uint16
	wVirtualScanCode  uint16
	unicodeChar       uint16
	wdControlKeyState uint32
}

// NewRuneReader creates a new instance of RuneReader
func NewRuneReader(reader Reader) *RuneReader {
	return &RuneReader{
		in: reader,
	}
}
func (rr *RuneReader) ReadRune() (rune, int, error) {
	ir := &inputRecord{}
	bytesRead := 0
	for {
		rv, _, e := readConsoleInput.Call(rr.in.Fd(), uintptr(unsafe.Pointer(ir)), 1, uintptr(unsafe.Pointer(&bytesRead)))
		// windows returns non-zero to indicate success
		if rv == 0 && e != nil {
			return 0, 0, e
		}

		if ir.eventType != EVENT_KEY {
			continue
		}

		// the event data is really a c struct union, so here we have to do an usafe
		// cast to put the data into the keyEventRecord (since we have already verified
		// above that this event does correspond to a key event
		key := (*keyEventRecord)(unsafe.Pointer(&ir.event[0]))
		// we only care about key down events
		if key.bKeyDown == 0 {
			continue
		}
		if key.wdControlKeyState&(LEFT_CTRL_PRESSED|RIGHT_CTRL_PRESSED) != 0 && key.unicodeChar == 'C' {
			return rune(KeyCtrlC), bytesRead, nil
		}
		// not a normal character so look up the input sequence from the
		// virtual key code mappings (VK_*)
		if key.unicodeChar == 0 {
			switch key.wVirtualKeyCode {
			case VK_DOWN:
				return ArrowDown, bytesRead, nil
			case VK_LEFT:
				return ArrowLeft, bytesRead, nil
			case VK_RIGHT:
				return ArrowRight, bytesRead, nil
			case VK_UP:
				return ArrowUp, bytesRead, nil
			case VK_DELETE:
				return KeyDEL, bytesRead, nil
			case VK_HOME:
				return rune(KeyCtrlA), bytesRead, nil
			case VK_END:
				return rune(KeyCtrlQ), bytesRead, nil
			default:
				// not a virtual key that we care about so just continue on to
				// the next input key
				continue
			}
		}
		r := rune(key.unicodeChar)
		return r, bytesRead, nil
	}
}
