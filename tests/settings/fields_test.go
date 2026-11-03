package settings_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glitchfl/lowkey/internal/settings"
)

// TestTheFirstRunWritesEveryDefaultWithAComment in tests/app keeps the file's exact text by hand - these
// loops cover every key the same way so a new one needs no test of its own

// changed gives a key a value that isn't its default - through the tab's own step where it has one
func changed(f settings.Field) func(*settings.Settings) {
	switch k := f.Kind.(type) {
	case settings.List:
		all := k.All()
		return func(s *settings.Settings) { *k.Field(s) = []string{all[len(all)-1], all[0]} }
	case settings.Custom:
		return func(s *settings.Settings) {
			s.AgentDots = []settings.DotSlot{{Agent: "jett", At: settings.Spot{X: 960, Y: 540, Placed: true}}, {Agent: "raze"}}
		}
	}
	return func(s *settings.Settings) { f.Step(s, 1) }
}

func TestEveryKeyIsWrittenUnderItsSectionWithItsComment(t *testing.T) {
	f, path := openWith(t, "")
	if err := f.Create(); err != nil {
		t.Fatal(err)
	}
	file := read(t, path)
	for _, fd := range settings.Fields {
		start := strings.Index(file, "["+fd.Section+"]\n")
		if start < 0 {
			t.Errorf("no [%s] for %s", fd.Section, fd.Key)
			continue
		}
		section := file[start+1:]
		if end := strings.Index(section, "\n["); end >= 0 {
			section = section[:end]
		}
		want := "# " + strings.Join(fd.Comment, "\n# ") + "\n" + fd.Key + " = "
		if !strings.Contains(section, want) {
			t.Errorf("%s should sit under [%s] after its comment:\n%s", fd.Key, fd.Section, file)
		}
	}
}

func TestEveryKeyReadsBackWhatTheTabWrote(t *testing.T) {
	for _, fd := range settings.Fields {
		want := settings.Default()
		changed(fd)(&want)
		t.Run(fd.Key+" in a new file", func(t *testing.T) {
			f, path := openWith(t, "")
			if _, err := f.Change(changed(fd)); err != nil {
				t.Fatal(err)
			}
			if got, note, err := settings.Parse([]byte(read(t, path))); err != nil || note != "" || !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v note %q err %v\nwant %+v", got, note, err, want)
			}
		})
		t.Run(fd.Key+" edited in place", func(t *testing.T) {
			f, path := openWith(t, "")
			if err := f.Create(); err != nil {
				t.Fatal(err)
			}
			before := read(t, path)
			if _, err := f.Change(changed(fd)); err != nil {
				t.Fatal(err)
			}
			after := read(t, path)
			if got, _, err := settings.Parse([]byte(after)); err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v err %v\nwant %+v", got, err, want)
			}
			if strings.Count(before, "\n") != strings.Count(after, "\n") {
				t.Errorf("an edit in place should only swap the value:\n%s", after)
			}
		})
	}
}

func TestEveryKeyRefusesAValueOfTheWrongType(t *testing.T) {
	for _, fd := range settings.Fields {
		wrong := "true"
		switch fd.Kind.(type) {
		case settings.OnOff:
			wrong = `"yes"`
		case settings.Custom:
			// agent dots keep what they can use and say what they dropped
			continue
		}
		t.Run(fd.Key, func(t *testing.T) {
			_, _, err := settings.Parse([]byte("[" + fd.Section + "]\n" + fd.Key + " = " + wrong + "\n"))
			if err == nil || !strings.Contains(err.Error(), fd.Key) {
				t.Errorf("want an error naming %s, got %v", fd.Key, err)
			}
		})
	}
}

func TestEveryNumberStopsAtItsLimitsTheSameOnTheTabAndInTheFile(t *testing.T) {
	for _, fd := range settings.Fields {
		k, ok := fd.Kind.(settings.Number)
		if !ok {
			continue
		}
		t.Run(fd.Key, func(t *testing.T) {
			s := settings.Default()
			*k.Field(&s) = k.Max
			fd.Step(&s, 1)
			if want := map[bool]int{false: k.Max, true: k.Min}[k.Wrap]; *k.Field(&s) != want {
				t.Errorf("right from %d: got %d, want %d", k.Max, *k.Field(&s), want)
			}
			*k.Field(&s) = k.Min
			fd.Step(&s, -1)
			if want := map[bool]int{false: k.Min, true: k.Max}[k.Wrap]; *k.Field(&s) != want {
				t.Errorf("left from %d: got %d, want %d", k.Min, *k.Field(&s), want)
			}

			got, note, err := settings.Parse([]byte(fmt.Sprintf("[%s]\n%s = %d\n", fd.Section, fd.Key, k.Max+1)))
			switch {
			case k.Refuse != "":
				if err == nil || !strings.Contains(err.Error(), fd.Key) {
					t.Errorf("%d should break the file, got %v", k.Max+1, err)
				}
			case err != nil || *k.Field(&got) != k.Max || !strings.Contains(note, fmt.Sprintf("using %d", k.Max)):
				t.Errorf("%d should clamp to %d with a note, got %d note %q err %v", k.Max+1, k.Max, *k.Field(&got), note, err)
			}
		})
	}
}
