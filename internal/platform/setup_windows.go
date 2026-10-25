package platform

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procSetProcessInformation = kernel32.NewProc("SetProcessInformation")
	procGetProcessInformation = kernel32.NewProc("GetProcessInformation")
	procSetConsoleTitleW      = kernel32.NewProc("SetConsoleTitleW")
)

const (
	processPowerThrottling          = 4
	powerThrottlingCurrentVersion   = 1
	powerThrottlingExecutionSpeed   = 0x1
	powerThrottlingStateStructBytes = 12
)

type powerThrottlingState struct {
	Version     uint32
	ControlMask uint32
	StateMask   uint32
}

// Prepare runs once at startup - below-normal priority plus EcoQoS and the window title
// idle priority would let the game starve lookups while a match loads
func Prepare() Setup {
	var s Setup
	self := windows.CurrentProcess()
	s.PriorityErr = windows.SetPriorityClass(self, windows.BELOW_NORMAL_PRIORITY_CLASS)
	state := powerThrottlingState{
		Version:     powerThrottlingCurrentVersion,
		ControlMask: powerThrottlingExecutionSpeed,
		StateMask:   powerThrottlingExecutionSpeed,
	}
	if err := procSetProcessInformation.Find(); err != nil {
		s.EcoErr = err
	} else if r, _, err := procSetProcessInformation.Call(uintptr(self), processPowerThrottling,
		uintptr(unsafe.Pointer(&state)), powerThrottlingStateStructBytes); r == 0 {
		s.EcoErr = err
	}
	if title, err := windows.UTF16PtrFromString("lowkey"); err == nil && procSetConsoleTitleW.Find() == nil {
		procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(title)))
	}
	return s
}

func Priority() (string, error) {
	class, err := windows.GetPriorityClass(windows.CurrentProcess())
	if err != nil {
		return "", err
	}
	names := map[uint32]string{
		windows.IDLE_PRIORITY_CLASS:         "idle",
		windows.BELOW_NORMAL_PRIORITY_CLASS: "below normal",
		windows.NORMAL_PRIORITY_CLASS:       "normal",
		windows.ABOVE_NORMAL_PRIORITY_CLASS: "above normal",
		windows.HIGH_PRIORITY_CLASS:         "high",
		windows.REALTIME_PRIORITY_CLASS:     "realtime",
	}
	if name, ok := names[class]; ok {
		return name, nil
	}
	return "unknown", nil
}

// ErrCantReadBack means this Windows can set EcoQoS but not report it
var ErrCantReadBack = errors.New("this Windows can't report Efficiency mode")

func EfficiencyMode() (bool, error) {
	if err := procGetProcessInformation.Find(); err != nil {
		return false, ErrCantReadBack
	}
	state := powerThrottlingState{Version: powerThrottlingCurrentVersion}
	r, _, _ := procGetProcessInformation.Call(uintptr(windows.CurrentProcess()), processPowerThrottling,
		uintptr(unsafe.Pointer(&state)), powerThrottlingStateStructBytes)
	if r == 0 {
		return false, ErrCantReadBack
	}
	return state.ControlMask&powerThrottlingExecutionSpeed != 0 && state.StateMask&powerThrottlingExecutionSpeed != 0, nil
}

// EnableANSI turns on VT processing so redraws and rank colors work - false means plain output
func EnableANSI() bool {
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if windows.GetConsoleMode(h, &mode) != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}

const Windows = true
