package term

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestReadKey(t *testing.T) {
	tests := map[string][]rune{
		"a":            {'a'},
		"ü":            {'ü'},
		"\033":         {rune(KeyESC)},
		"\033[A\033[B": {ArrowUp, ArrowDown},
		"\033OC\033OD": {ArrowRight, ArrowLeft}, // application cursor key mode
		"\033[1;5Cx":   {ArrowRight, 'x'},       // ctrl+right
		"\033[3~x":     {rune(KeyCtrlR), 'x'},   // delete
		"\033[5~x":     {rune(KeyCtrlSpace), 'x'},
		"\033[15~x":    {rune(KeyCtrlSpace), 'x'}, // F5
		"\033bx":       {rune(KeyCtrlSpace), 'x'}, // alt+b
	}
	for input, want := range tests {
		in := bufio.NewReader(strings.NewReader(input))
		var got []rune
		for range want {
			r, _, err := readKey(in)
			if err != nil {
				t.Fatalf("%q: %v", input, err)
			}
			got = append(got, r)
		}
		if string(got) != string(want) {
			t.Errorf("%q: got %q, want %q", input, got, want)
		}
		if in.Buffered() != 0 {
			t.Errorf("%q: %d bytes left unread", input, in.Buffered())
		}
	}
}

func TestWriteCells(t *testing.T) {
	defer func(noColor bool) { color.NoColor = noColor }(color.NoColor)
	color.NoColor = false

	var out bytes.Buffer
	w := NewBufferedWriter(&out)
	cells := append(Cprint("Speed up by 50%", color.FgWhite), Cprint(" 100%d", color.FgRed, color.Bold)...)
	if _, err := w.WriteCells(cells); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Speed up by 50%") || !strings.Contains(got, " 100%d") || strings.Contains(got, "NOVERB") {
		t.Fatalf("text was not written as is: %q", got)
	}
	if n := strings.Count(got, "\x1b[37m"); n != 1 {
		t.Fatalf("got the white color sequence %d times, want 1 for the whole run: %q", n, got)
	}
}
