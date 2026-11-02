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
)

const (
	gwlExStyle = ^uintptr(19) // GWL_EXSTYLE is -20
	// topmost, click-through, layered, kept off the taskbar and alt-tab, never activated
	wantExStyle = 0x00000008 | 0x00000020 | 0x00080000 | 0x00000080 | 0x08000000
)

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

func TestTheDotWindowComesAndGoesWithoutTakingFocus(t *testing.T) {
	before := foreground()
	if err := platform.ShowDot(300, 200); err != nil {
		t.Skipf("this session can't open windows: %v", err)
	}
	t.Cleanup(platform.HideDot)
	found := dotWindows()
	if len(found) != 1 {
		t.Fatalf("%d dot windows while one dot is up", len(found))
	}
	if style, _, _ := procGetWindowLongPtrW.Call(found[0], gwlExStyle); style&wantExStyle != wantExStyle {
		t.Errorf("dot window extended style %#x is missing some of %#x", style, wantExStyle)
	}
	var r struct{ left, top, right, bottom int32 }
	procGetWindowRect.Call(found[0], uintptr(unsafe.Pointer(&r)))
	if cx, cy := (r.left+r.right)/2, (r.top+r.bottom)/2; cx < 298 || cx > 302 || cy < 198 || cy > 202 {
		t.Errorf("dot window centered on %d, %d - want 300, 200", cx, cy)
	}
	if w := r.right - r.left; w < 12 || w > 48 {
		t.Errorf("dot window is %d px wide", w)
	}
	if after := foreground(); after != before {
		t.Errorf("showing the dot moved the foreground window from %#x to %#x", before, after)
	}

	if err := platform.ShowDot(500, 400); err != nil {
		t.Fatal(err)
	}
	if n := len(dotWindows()); n != 1 {
		t.Errorf("%d dot windows after moving the dot", n)
	}
	platform.HideDot()
	if n := len(dotWindows()); n != 0 {
		t.Errorf("%d dot windows left after HideDot", n)
	}
	if after := foreground(); after != before {
		t.Errorf("the dot moved the foreground window from %#x to %#x", before, after)
	}
}
