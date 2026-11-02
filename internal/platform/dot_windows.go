package platform

import (
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                           = windows.NewLazySystemDLL("user32.dll")
	gdi32                            = windows.NewLazySystemDLL("gdi32.dll")
	procRegisterClassExW             = user32.NewProc("RegisterClassExW")
	procCreateWindowExW              = user32.NewProc("CreateWindowExW")
	procDefWindowProcW               = user32.NewProc("DefWindowProcW")
	procShowWindow                   = user32.NewProc("ShowWindow")
	procSetWindowPos                 = user32.NewProc("SetWindowPos")
	procSetLayeredWindowAttributes   = user32.NewProc("SetLayeredWindowAttributes")
	procGetMessageW                  = user32.NewProc("GetMessageW")
	procDispatchMessageW             = user32.NewProc("DispatchMessageW")
	procPostMessageW                 = user32.NewProc("PostMessageW")
	procPostQuitMessage              = user32.NewProc("PostQuitMessage")
	procBeginPaint                   = user32.NewProc("BeginPaint")
	procEndPaint                     = user32.NewProc("EndPaint")
	procGetClientRect                = user32.NewProc("GetClientRect")
	procFillRect                     = user32.NewProc("FillRect")
	procGetCursorPos                 = user32.NewProc("GetCursorPos")
	procGetDpiForWindow              = user32.NewProc("GetDpiForWindow")
	procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")
	procCreateSolidBrush             = gdi32.NewProc("CreateSolidBrush")
	procCreatePen                    = gdi32.NewProc("CreatePen")
	procSelectObject                 = gdi32.NewProc("SelectObject")
	procDeleteObject                 = gdi32.NewProc("DeleteObject")
	procEllipse                      = gdi32.NewProc("Ellipse")
)

const (
	wsPopup         = 0x80000000
	wsExTopmost     = 0x00000008
	wsExTransparent = 0x00000020
	wsExToolWindow  = 0x00000080
	wsExLayered     = 0x00080000
	wsExNoActivate  = 0x08000000

	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmPaint         = 0x000f
	wmMouseActivate = 0x0021
	wmNcHitTest     = 0x0084

	maNoActivate     = 3
	swShowNoActivate = 4
	swpNoZOrder      = 0x0004
	swpNoActivate    = 0x0010
	lwaColorKey      = 0x1

	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is (HANDLE)-4
	perMonitorAwareV2 = ^uintptr(3)
	htTransparent     = ^uintptr(0)

	dotSize = 12 // pixels across at 100% scaling
)

var (
	keyColor     = rgb(255, 0, 255) // painted where the window should be see-through
	fillColor    = rgb(0, 255, 170)
	outlineColor = rgb(16, 16, 16)
)

func rgb(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type point struct{ x, y int32 }

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

type rect struct{ left, top, right, bottom int32 }

type paintStruct struct {
	hdc       uintptr
	erase     int32
	paint     rect
	restore   int32
	incUpdate int32
	reserved  [32]byte
}

type dotWindow struct {
	hwnd uintptr
	done chan struct{}
}

var (
	dotMu      sync.Mutex
	shownDot   *dotWindow
	classOnce  sync.Once
	classErr   error
	className  = windows.StringToUTF16Ptr("lowkeyAgentDot")
	dotProcPtr = windows.NewCallback(dotProc)
)

// ShowDot draws the agent dot centered on a point in physical pixels - its window and thread live
// only until HideDot so nothing of it is left over the game afterwards
func ShowDot(x, y int) error {
	HideDot()
	w := &dotWindow{done: make(chan struct{})}
	created := make(chan error, 1)
	go w.run(x, y, created)
	if err := <-created; err != nil {
		return err
	}
	dotMu.Lock()
	shownDot = w
	dotMu.Unlock()
	return nil
}

// HideDot destroys the window rather than hiding it - an invisible window over a game can still
// keep Windows compositing it
func HideDot() {
	dotMu.Lock()
	w := shownDot
	shownDot = nil
	dotMu.Unlock()
	if w == nil {
		return
	}
	procPostMessageW.Call(w.hwnd, wmClose, 0, 0)
	select {
	case <-w.done:
	case <-time.After(time.Second):
	}
}

// CursorPos reads the cursor in physical pixels so it matches where ShowDot draws on a scaled display
func CursorPos() (int, int, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if old := perMonitorDPI(); old != 0 {
		defer procSetThreadDpiAwarenessContext.Call(old)
	}
	var p point
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return int(p.x), int(p.y), r != 0
}

func (w *dotWindow) run(x, y int, created chan<- error) {
	// never unlocked - a goroutine that ends while locked takes its thread with it
	runtime.LockOSThread()
	defer close(w.done)
	if err := registerDotClass(); err != nil {
		created <- err
		return
	}
	perMonitorDPI()
	hwnd, err := createDot(x, y)
	if err != nil {
		created <- err
		return
	}
	w.hwnd = hwnd
	created <- nil
	var m msg
	for {
		if r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0); int32(r) <= 0 {
			return
		}
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func registerDotClass() error {
	classOnce.Do(func() {
		var instance windows.Handle
		if classErr = windows.GetModuleHandleEx(0, nil, &instance); classErr != nil {
			return
		}
		wc := wndClassEx{wndProc: dotProcPtr, instance: instance, className: className}
		wc.size = uint32(unsafe.Sizeof(wc))
		if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			classErr = err
		}
	})
	return classErr
}

func createDot(x, y int) (uintptr, error) {
	exStyle := uintptr(wsExTopmost | wsExTransparent | wsExToolWindow | wsExLayered | wsExNoActivate)
	hwnd, _, err := procCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(className)), 0, wsPopup,
		uintptr(x-dotSize/2), uintptr(y-dotSize/2), dotSize, dotSize, 0, 0, 0, 0)
	if hwnd == 0 {
		return 0, err
	}
	size := dotSize
	if procGetDpiForWindow.Find() == nil {
		if dpi, _, _ := procGetDpiForWindow.Call(hwnd); dpi > 0 {
			size = dotSize * int(dpi) / 96
		}
	}
	procSetWindowPos.Call(hwnd, 0, uintptr(x-size/2), uintptr(y-size/2), uintptr(size), uintptr(size), swpNoZOrder|swpNoActivate)
	// a layered window stays invisible until it has its attributes
	if r, _, err := procSetLayeredWindowAttributes.Call(hwnd, keyColor, 0, lwaColorKey); r == 0 {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
		return 0, err
	}
	procShowWindow.Call(hwnd, swShowNoActivate)
	return hwnd, nil
}

func perMonitorDPI() uintptr {
	if procSetThreadDpiAwarenessContext.Find() != nil {
		return 0
	}
	old, _, _ := procSetThreadDpiAwarenessContext.Call(perMonitorAwareV2)
	return old
}

func dotProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmPaint:
		paintDot(hwnd)
		return 0
	case wmNcHitTest:
		return htTransparent
	case wmMouseActivate:
		return maNoActivate
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func paintDot(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	var rc rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	key, _, _ := procCreateSolidBrush.Call(keyColor)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), key)
	procDeleteObject.Call(key)
	width := max(2, int(rc.right)/6)
	pen, _, _ := procCreatePen.Call(0, uintptr(width), outlineColor)
	brush, _, _ := procCreateSolidBrush.Call(fillColor)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	// the pen straddles the edge it draws - pull it in by half so the outline isn't clipped
	in := width / 2
	procEllipse.Call(hdc, uintptr(in), uintptr(in), uintptr(int(rc.right)-in), uintptr(int(rc.bottom)-in))
	procSelectObject.Call(hdc, oldPen)
	procSelectObject.Call(hdc, oldBrush)
	procDeleteObject.Call(pen)
	procDeleteObject.Call(brush)
}
