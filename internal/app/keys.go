package app

import (
	"bytes"
	"io"
)

type key int

const (
	keyNone key = iota
	keyTab
	keyUp
	keyDown
	keyLeft
	keyRight
	keySpace
	keyEnter
	keyEsc
)

// keyDecoder turns console bytes into key presses - the console sends arrows as escape sequences and
// one that a read cut in half waits for the rest
type keyDecoder struct {
	pending []byte
}

func (d *keyDecoder) decode(chunk []byte) []key {
	data := append(d.pending, chunk...)
	d.pending = nil
	var keys []key
	for i := 0; i < len(data); {
		k, n := keyNone, 1
		switch data[i] {
		case '\t':
			k = keyTab
		case '\r', '\n':
			k = keyEnter
		case ' ':
			k = keySpace
		case 0x1b:
			var whole bool
			if k, n, whole = escape(data[i:]); !whole {
				d.pending = bytes.Clone(data[i:])
				return keys
			}
		}
		if k != keyNone {
			keys = append(keys, k)
		}
		i += n
	}
	return keys
}

var finals = map[byte]key{'A': keyUp, 'B': keyDown, 'C': keyRight, 'D': keyLeft, 'Z': keyTab}

// escape reads one sequence starting at its ESC - an ESC with nothing behind it is the Esc key
func escape(data []byte) (k key, n int, whole bool) {
	if len(data) == 1 {
		return keyEsc, 1, true
	}
	switch data[1] {
	case 'O':
		if len(data) < 3 {
			return keyNone, 0, false
		}
		return finals[data[2]], 3, true
	case '[':
		for i := 2; i < len(data); i++ {
			switch c := data[i]; {
			case c >= 0x40 && c <= 0x7e:
				return finals[c], i + 1, true
			case c < 0x20 || c > 0x7e:
				return keyNone, i, true
			}
		}
		return keyNone, 0, false
	}
	return keyEsc, 1, true
}

func (lv *live) readKeys(r io.Reader) {
	var d keyDecoder
	buf := make([]byte, 256)
	for {
		n, err := r.Read(buf)
		for _, k := range d.decode(buf[:n]) {
			lv.loop.post(func() { lv.pressed(k) })
		}
		if err != nil {
			return
		}
	}
}

func (lv *live) pressed(k key) {
	switch {
	case k == keyTab:
		lv.onSettings = !lv.onSettings
		lv.tab.notice = ""
	case lv.onSettings:
		if lv.tab.press(k) {
			lv.onSettings = false
		}
	}
}
