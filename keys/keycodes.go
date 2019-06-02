package keys

const (
	ArrowLeft       = '\x02'
	ArrowRight      = '\x06'
	ArrowUp         = '\x10'
	ArrowDown       = '\x0e'
	Space           = ' '
	Enter           = '\r'
	Backspace       = '\b'
	Delete          = '\x7f'
	Interrupt       = '\x03'
	EndTransmission = '\x04'
	Escape          = '\x1b'
	DeleteWord      = '\x17' // Ctrl+W
	DeleteLine      = '\x18' // Ctrl+X
	Home            = '\x01'
	End             = '\x11'
	Delete2         = '\x12'
	IgnoreKey       = '\000'
)

type Key int16

// These are the defined ASCII values for key codes.  They generally match
// with KeyCtrl values.
const (
	KeyNUL Key = iota
	KeySOH
	KeySTX
	KeyETX
	KeyEOT
	KeyENQ
	KeyACK
	KeyBEL
	KeyBS
	KeyTAB
	KeyLF
	KeyVT
	KeyFF
	KeyCR
	KeySO
	KeySI
	KeyDLE
	KeyDC1
	KeyDC2
	KeyDC3
	KeyDC4
	KeyNAK
	KeySYN
	KeyETB
	KeyCAN
	KeyEM
	KeySUB
	KeyESC
	KeyFS
	KeyGS
	KeyRS
	KeyUS
	KeyDEL Key = 0x7F
	KeyBS2 Key = 127
)

// These are the control keys.  Note that they overlap with other keys,
// perhaps.  For example, KeyCtrlH is the same as KeyBackspace.
const (
	KeyCtrlSpace Key = iota
	KeyCtrlA
	KeyCtrlB
	KeyCtrlC
	KeyCtrlD
	KeyCtrlE
	KeyCtrlF
	KeyCtrlG
	KeyCtrlH
	KeyCtrlI
	KeyCtrlJ
	KeyCtrlK
	KeyCtrlL
	KeyCtrlM
	KeyCtrlN
	KeyCtrlO
	KeyCtrlP
	KeyCtrlQ
	KeyCtrlR
	KeyCtrlS
	KeyCtrlT
	KeyCtrlU
	KeyCtrlV
	KeyCtrlW
	KeyCtrlX
	KeyCtrlY
	KeyCtrlZ
	KeyCtrlLeftSq // Escape
	KeyCtrlBackslash
	KeyCtrlRightSq
	KeyCtrlCarat
	KeyCtrlUnderscore
)
