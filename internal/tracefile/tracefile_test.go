package tracefile

import "testing"

func TestSmoke(t *testing.T) {
	tl, err := Load(`.`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tl) == 0 {
		t.Fatal(`unexpected length`)
	}

	per := len(tl) / len(Versions[:])
	for _, ver := range Versions {
		if exp := len(Versions[:]); len(tl.ByName(`log.trace`)) != exp {
			t.Fatalf(`expected %v trace files for ByName(log.trace)`, exp)
		}
		if exp := len(Versions[:]); len(tl.ByMaxSize(1024*32)) != exp {
			t.Fatalf(`expected %v trace files for ByMaxSize(32k)`, exp)
		}

		vtl := tl.ByVersion(ver)
		if len(vtl) != per {
			t.Fatalf(`expected %v trace files for ByVersion(%v)`, per, ver)
		}
		if len(vtl.ByName(`log.trace`)) != 1 {
			t.Fatalf(`expected 1 log.trace file for version %v`, ver)
		}
	}
}
