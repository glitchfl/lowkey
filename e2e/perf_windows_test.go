//go:build windows

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/glitchfl/lowkey/internal/fakeriot"
)

var procGetProcessMemoryInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

type memoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

func handle(t *testing.T, pid int) windows.Handle {
	t.Helper()
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.CloseHandle(h) })
	return h
}

// cpu is kernel plus user time - FILETIME durations count 100ns ticks
func cpu(t *testing.T, h windows.Handle) time.Duration {
	t.Helper()
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		t.Fatal(err)
	}
	ticks := func(f windows.Filetime) int64 { return int64(f.HighDateTime)<<32 | int64(f.LowDateTime) }
	return time.Duration((ticks(kernel) + ticks(user)) * 100)
}

func peakWorkingSet(t *testing.T, h windows.Handle) uint64 {
	t.Helper()
	m := memoryCounters{cb: uint32(unsafe.Sizeof(memoryCounters{}))}
	if r, _, err := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&m)), uintptr(m.cb)); r == 0 {
		t.Fatal(err)
	}
	return uint64(m.peakWorkingSetSize)
}

func idleCPU(t *testing.T, lk *lowkey) time.Duration {
	h := handle(t, lk.cmd.Process.Pid)
	time.Sleep(5 * time.Second)
	before := cpu(t, h)
	time.Sleep(60 * time.Second)
	return cpu(t, h) - before
}

func TestIdleUsesAlmostNoCPU(t *testing.T) {
	t.Run("waiting for the Riot Client", func(t *testing.T) {
		t.Parallel()
		path, dir := exe(t), t.TempDir()
		cmd := exec.Command(path)
		cmd.Env = append(os.Environ(), "LOWKEY_LOCKFILE="+filepath.Join(dir, "Riot Games", "Riot Client", "Config", "lockfile"), "LOWKEY_DIR="+dir)
		lk := run(t, cmd, dir)
		lk.screen("waiting for the Riot Client")
		if used := idleCPU(t, lk); used > 100*time.Millisecond {
			t.Errorf("lowkey used %v of CPU in 60 s while waiting for the Riot Client", used)
		} else {
			t.Logf("%v of CPU in 60 s", used)
		}
	})
	t.Run("in the menus", func(t *testing.T) {
		t.Parallel()
		fr := startRiot(t, fakeriot.LobbyWorld(time.Now(), false))
		lk := startLowkey(t, fr, fastSettings)
		lk.screen("waiting for the next match", "SESSION")
		quiet(t, fr)
		if used := idleCPU(t, lk); used > 100*time.Millisecond {
			t.Errorf("lowkey used %v of CPU in 60 s sitting in the menus", used)
		} else {
			t.Logf("%v of CPU in 60 s", used)
		}
	})
}

func TestPeakMemoryStaysUnder100MB(t *testing.T) {
	fr := startRiot(t, fakeriot.LobbyWorld(time.Now(), true))
	lk := startLowkey(t, fr, "cutoff_seconds = 300\n")
	lk.screen("waiting for the next match", "SESSION")
	subscribed(t, fr)
	h := handle(t, lk.cmd.Process.Pid)
	for round := 1; round <= 5; round++ {
		var pregame, coreGame string
		fr.Update(func(w *fakeriot.World) { pregame, coreGame = fakeriot.NextLobby(w, round, true) })
		fr.StartAgentSelect(pregame)
		lk.screen("AGENT SELECT", "AVERAGE")
		quiet(t, fr)
		fr.LoadMatch(coreGame)
		lk.screen("MATCH LOADING", "US - THEM")
		quiet(t, fr)
		fr.EndMatch()
		lk.screen("LAST MATCH")
	}
	peak := peakWorkingSet(t, h)
	t.Logf("peak working set %d MB after %d match details", peak>>20, count(fr, "/match-details/"))
	if peak > 100<<20 {
		t.Errorf("peak working set %d MB is over the 100 MB cap", peak>>20)
	}
}
