// +build windows
package term

const (
	EVENT_KEY = 0x0001

	// key codes for arrow keys
	// https://msdn.microsoft.com/en-us/library/windows/desktop/dd375731(v=vs.85).aspx
	VK_DELETE = 0x2E
	VK_END    = 0x23
	VK_HOME   = 0x24
	VK_LEFT   = 0x25
	VK_UP     = 0x26
	VK_RIGHT  = 0x27
	VK_DOWN   = 0x28

	RIGHT_CTRL_PRESSED = 0x0004
	LEFT_CTRL_PRESSED  = 0x0008

	ENABLE_ECHO_INPUT      uint32 = 0x0004
	ENABLE_LINE_INPUT      uint32 = 0x0002
	ENABLE_PROCESSED_INPUT uint32 = 0x0001
)
