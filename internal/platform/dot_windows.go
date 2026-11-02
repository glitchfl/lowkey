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
	procGetDC                        = user32.NewProc("GetDC")
	procReleaseDC                    = user32.NewProc("ReleaseDC")
	procGetCursorPos                 = user32.NewProc("GetCursorPos")
	procGetDpiForWindow              = user32.NewProc("GetDpiForWindow")
	procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")
	procCreateSolidBrush             = gdi32.NewProc("CreateSolidBrush")
	procCreatePen                    = gdi32.NewProc("CreatePen")
	procCreateFontW                  = gdi32.NewProc("CreateFontW")
	procSelectObject                 = gdi32.NewProc("SelectObject")
	procDeleteObject                 = gdi32.NewProc("DeleteObject")
	procEllipse                      = gdi32.NewProc("Ellipse")
	procSetBkMode                    = gdi32.NewProc("SetBkMode")
	procSetTextColor                 = gdi32.NewProc("SetTextColor")
	procTextOutW                     = gdi32.NewProc("TextOutW")
	procGetTextExtentPoint32W        = gdi32.NewProc("GetTextExtentPoint32W")
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
	bkTransparent    = 1
	fwBold           = 700
	// smoothed edges would blend into the see-through key color and leave pink fringes
	nonAntialiasedQuality = 3

	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is (HANDLE)-4
	perMonitorAwareV2 = ^uintptr(3)
	htTransparent     = ^uintptr(0)

	// sizes in pixels at 100% scaling
	dotSize   = 12
	labelSize = 14
	labelGap  = 2
)

var keyColor = rgb(255, 0, 255) // painted where the window should be see-through

func rgb(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }

// colorRef turns 0xRRGGBB into the 0x00BBGGRR Windows draws with
func colorRef(c uint32) uintptr { return rgb(byte(c>>16), byte(c>>8), byte(c)) }

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

type size struct{ cx, cy int32 }

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
	mark  Mark
	label []uint16
	hwnd  uintptr
	font  uintptr
	done  chan struct{}
	// layout in physical pixels once the monitor's scaling is known
	dot, outline, textW, textH, gap int
}

var (
	dotMu      sync.Mutex
	shownDots  []*dotWindow
	byHandle   = map[uintptr]*dotWindow{}
	classOnce  sync.Once
	classErr   error
	className  = windows.StringToUTF16Ptr("lowkeyAgentDot")
	dotProcPtr = windows.NewCallback(dotProc)
)

// ShowDots replaces whatever dots are up with these - each window and its thread live only until
// HideDots or the next ShowDots so nothing of them is left over the game afterwards
func ShowDots(marks []Mark) error {
	HideDots()
	var opened []*dotWindow
	for _, m := range marks {
		label, err := windows.UTF16FromString(m.Label)
		if err != nil {
			closeDots(opened)
			return err
		}
		w := &dotWindow{mark: m, label: label[:len(label)-1], done: make(chan struct{})}
		created := make(chan error, 1)
		go w.run(created)
		if err := <-created; err != nil {
			closeDots(opened)
			return err
		}
		opened = append(opened, w)
	}
	dotMu.Lock()
	shownDots = opened
	dotMu.Unlock()
	return nil
}

// HideDots destroys the windows rather than hiding them - an invisible window over a game can still
// keep Windows compositing it
func HideDots() {
	dotMu.Lock()
	ws := shownDots
	shownDots = nil
	dotMu.Unlock()
	closeDots(ws)
}

func closeDots(ws []*dotWindow) {
	for _, w := range ws {
		procPostMessageW.Call(w.hwnd, wmClose, 0, 0)
	}
	deadline := time.After(time.Second)
	for _, w := range ws {
		select {
		case <-w.done:
		case <-deadline:
			return
		}
	}
}

// CursorPos reads the cursor in physical pixels so it matches where ShowDots draws on a scaled display
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

func (w *dotWindow) run(created chan<- error) {
	// never unlocked - a goroutine that ends while locked takes its thread with it
	runtime.LockOSThread()
	defer close(w.done)
	if err := registerDotClass(); err != nil {
		created <- err
		return
	}
	perMonitorDPI()
	if err := w.open(); err != nil {
		created <- err
		return
	}
	defer procDeleteObject.Call(w.font)
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

// open makes the window at its final size - it has to exist first to learn its monitor's scaling
func (w *dotWindow) open() error {
	exStyle := uintptr(wsExTopmost | wsExTransparent | wsExToolWindow | wsExLayered | wsExNoActivate)
	hwnd, _, err := procCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(className)), 0, wsPopup,
		uintptr(w.mark.X), uintptr(w.mark.Y), 1, 1, 0, 0, 0, 0)
	if hwnd == 0 {
		return err
	}
	w.hwnd = hwnd
	dpi := 96
	if procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(hwnd); d > 0 {
			dpi = int(d)
		}
	}
	scale := func(px int) int { return max(1, px*dpi/96) }
	w.dot, w.outline = scale(dotSize), scale(1)
	if len(w.label) > 0 {
		face := windows.StringToUTF16Ptr("Segoe UI")
		w.font, _, _ = procCreateFontW.Call(uintptr(-scale(labelSize)), 0, 0, 0, fwBold, 0, 0, 0, 1, 0, 0,
			nonAntialiasedQuality, 0, uintptr(unsafe.Pointer(face)))
		var sz size
		hdc, _, _ := procGetDC.Call(hwnd)
		old, _, _ := procSelectObject.Call(hdc, w.font)
		procGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&w.label[0])), uintptr(len(w.label)), uintptr(unsafe.Pointer(&sz)))
		procSelectObject.Call(hdc, old)
		procReleaseDC.Call(hwnd, hdc)
		w.textW, w.textH, w.gap = int(sz.cx), int(sz.cy)+2*w.outline, scale(labelGap)
	}
	width := max(w.dot, w.textW+2*w.outline)
	height := w.textH + w.gap + w.dot
	left, top := w.mark.X-width/2, w.mark.Y-w.textH-w.gap-w.dot/2
	procSetWindowPos.Call(hwnd, 0, uintptr(left), uintptr(top), uintptr(width), uintptr(height), swpNoZOrder|swpNoActivate)
	dotMu.Lock()
	byHandle[hwnd] = w
	dotMu.Unlock()
	// a layered window stays invisible until it has its attributes
	if r, _, err := procSetLayeredWindowAttributes.Call(hwnd, keyColor, 0, lwaColorKey); r == 0 {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
		return err
	}
	procShowWindow.Call(hwnd, swShowNoActivate)
	return nil
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
		dotMu.Lock()
		w := byHandle[hwnd]
		dotMu.Unlock()
		if w != nil {
			w.paint()
			return 0
		}
	case wmNcHitTest:
		return htTransparent
	case wmMouseActivate:
		return maNoActivate
	case wmDestroy:
		dotMu.Lock()
		delete(byHandle, hwnd)
		dotMu.Unlock()
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func (w *dotWindow) paint() {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	var rc rect
	procGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&rc)))
	key, _, _ := procCreateSolidBrush.Call(keyColor)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), key)
	procDeleteObject.Call(key)
	fill, ring := colorRef(w.mark.Fill), colorRef(w.mark.Ring)
	if len(w.label) > 0 {
		oldFont, _, _ := procSelectObject.Call(hdc, w.font)
		procSetBkMode.Call(hdc, bkTransparent)
		x, y := (int(rc.right)-w.textW)/2, w.outline
		text, n := uintptr(unsafe.Pointer(&w.label[0])), uintptr(len(w.label))
		// the ring color drawn around the name first is its outline
		procSetTextColor.Call(hdc, ring)
		for dx := -w.outline; dx <= w.outline; dx++ {
			for dy := -w.outline; dy <= w.outline; dy++ {
				if dx != 0 || dy != 0 {
					procTextOutW.Call(hdc, uintptr(x+dx), uintptr(y+dy), text, n)
				}
			}
		}
		procSetTextColor.Call(hdc, fill)
		procTextOutW.Call(hdc, uintptr(x), uintptr(y), text, n)
		procSelectObject.Call(hdc, oldFont)
	}
	width := max(2, w.dot/6)
	pen, _, _ := procCreatePen.Call(0, uintptr(width), ring)
	brush, _, _ := procCreateSolidBrush.Call(fill)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	// the pen straddles the edge it draws - pull it in by half so the outline isn't clipped
	in := width / 2
	left, top := (int(rc.right)-w.dot)/2, w.textH+w.gap
	procEllipse.Call(hdc, uintptr(left+in), uintptr(top+in), uintptr(left+w.dot-in), uintptr(top+w.dot-in))
	procSelectObject.Call(hdc, oldPen)
	procSelectObject.Call(hdc, oldBrush)
	procDeleteObject.Call(pen)
	procDeleteObject.Call(brush)
}
