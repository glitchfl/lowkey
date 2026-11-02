//go:build windows

package e2e

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/glitchfl/lowkey/internal/platform"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procFindWindowExW       = user32.NewProc("FindWindowExW")
	procGetWindowLongPtrW   = user32.NewProc("GetWindowLongPtrW")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
)

const (
	gwlExStyle = ^uintptr(19) // GWL_EXSTYLE is -20
	// topmost, click-through, layered, kept off the taskbar and alt-tab, never activated
	wantExStyle = 0x00000008 | 0x00000020 | 0x00080000 | 0x00000080 | 0x08000000
)

type winRect struct{ left, top, right, bottom int32 }

func dotWindows() []uintptr {
	class := windows.StringToUTF16Ptr("lowkeyAgentDot")
	var found []uintptr
	var after uintptr
	for {
		h, _, _ := procFindWindowExW.Call(0, after, uintptr(unsafe.Pointer(class)), 0)
		if h == 0 {
			return found
		}
		found = append(found, h)
		after = h
	}
}

func foreground() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}

func windowRect(h uintptr) winRect {
	var r winRect
	procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&r)))
	return r
}

// dotPixels is the dot's size on the window's monitor - 12 px at 100% scaling
func dotPixels(h uintptr) int32 {
	dpi, _, _ := procGetDpiForWindow.Call(h)
	if dpi == 0 {
		dpi = 96
	}
	return max(1, int32(12*dpi/96))
}

func TestTheDotWindowsComeAndGoWithoutTakingFocus(t *testing.T) {
	before := foreground()
	jett := platform.Mark{X: 300, Y: 200, Label: "Jett", Fill: 0x00E5FF, Ring: 0xF4F7FA}
	if err := platform.ShowDots([]platform.Mark{jett}); err != nil {
		t.Skipf("this session can't open windows: %v", err)
	}
	t.Cleanup(platform.HideDots)
	found := dotWindows()
	if len(found) != 1 {
		t.Fatalf("%d dot windows while one dot is up", len(found))
	}
	if style, _, _ := procGetWindowLongPtrW.Call(found[0], gwlExStyle); style&wantExStyle != wantExStyle {
		t.Errorf("dot window extended style %#x is missing some of %#x", style, wantExStyle)
	}
	r, dot := windowRect(found[0]), dotPixels(found[0])
	// the dot sits at the bottom of its window with the name above it
	if cx, cy := (r.left+r.right)/2, r.bottom-dot/2; cx < 298 || cx > 302 || cy < 198 || cy > 202 {
		t.Errorf("dot centered on %d, %d - want 300, 200 (window %+v)", cx, cy, r)
	}
	if h := r.bottom - r.top; h < dot+8 {
		t.Errorf("window is %d px tall - no room for the name above a %d px dot", h, dot)
	}
	if w := r.right - r.left; w <= dot {
		t.Errorf("window is %d px wide - narrower than the name", w)
	}
	if after := foreground(); after != before {
		t.Errorf("showing the dot moved the foreground window from %#x to %#x", before, after)
	}

	three := []platform.Mark{jett, {X: 360, Y: 200, Label: "Fade", Fill: 0x3A3F44, Ring: 0xF2F2F2}, {X: 420, Y: 200, Label: "KAY/O", Fill: 0x1FB8D6, Ring: 0x101010}}
	if err := platform.ShowDots(three); err != nil {
		t.Fatal(err)
	}
	if n := len(dotWindows()); n != 3 {
		t.Errorf("%d dot windows for three dots", n)
	}
	if err := platform.ShowDots(three[2:]); err != nil {
		t.Fatal(err)
	}
	if n := len(dotWindows()); n != 1 {
		t.Errorf("%d dot windows after redrawing with one dot", n)
	}
	platform.HideDots()
	if n := len(dotWindows()); n != 0 {
		t.Errorf("%d dot windows left after HideDots", n)
	}
	if after := foreground(); after != before {
		t.Errorf("the dots moved the foreground window from %#x to %#x", before, after)
	}
}

func TestADotWithoutANameIsJustTheDot(t *testing.T) {
	if err := platform.ShowDots([]platform.Mark{{X: 300, Y: 200, Fill: 0xFF8A00, Ring: 0x101010}}); err != nil {
		t.Skipf("this session can't open windows: %v", err)
	}
	t.Cleanup(platform.HideDots)
	found := dotWindows()
	if len(found) != 1 {
		t.Fatalf("%d dot windows for one dot", len(found))
	}
	r, dot := windowRect(found[0]), dotPixels(found[0])
	if w, h := r.right-r.left, r.bottom-r.top; w != dot || h != dot {
		t.Errorf("an unnamed dot's window is %dx%d - want %dx%d", w, h, dot, dot)
	}
}
