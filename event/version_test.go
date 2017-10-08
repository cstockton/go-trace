package event

import "testing"

func TestVersionDrift(t *testing.T) {
	if Latest != Version4 {
		// When adding Version4 this will help remind me to update tests that
		// literal versions are used.
		t.Fatal(`Make sure to update tests where Versions are used.`)
	}
}

func TestVersionValid(t *testing.T) {
	tests := []struct {
		ver Version
		exp bool
	}{
		{Version1, true},
		{Version2, true},
		{Version3, true},
		{Version4, true},
		{Latest, true},
		{Latest + 1, false},
		{Latest + 2, false},
		{Latest + 3, false},
		{0, false},
	}
	for i, test := range tests {
		t.Logf(`test #%v exp version %q.Valid() to be %v`, i, test.ver, test.exp)
		if got := test.ver.Valid(); test.exp != got {
			t.Errorf(`expected version %q.Valid() to be %v, got %v`,
				test.ver, test.exp, got)
		}
	}
}

func TestVersionComparable(t *testing.T) {
	order := []Version{0, Version1, Version2, Version3, Version(4), Version(5)}
	for i, ver := range order {
		if i > 0 {
			if older := order[i-1]; older > ver {
				t.Errorf(`expected Version%d(%q) > Version%[1]d(%[3]q)`, i+1, ver, older)
			}
		}
		if order[i] != ver {
			t.Errorf(`expected Version%d(%q) == Version%[1]d(%[3]q)`, i+1, ver, order[i])
		}
		if i < len(order)-1 {
			if newer := order[i+1]; newer < ver {
				t.Errorf(`expected Version%d(%q) < Version%[1]d(%[3]q)`, i+1, ver, newer)
			}
		}
	}
}

func TestVersionGo(t *testing.T) {
	tests := []struct {
		ver Version
		exp string
	}{
		{Version1, `1.5`},
		{Version2, `1.7`},
		{Version3, `1.8`},
		{Version4, `1.9`},
		{Latest, `1.9`},
		{Latest + 1, `None`},
		{Latest + 2, `None`},
		{Latest + 3, `None`},
		{0, `None`},
	}
	for i, test := range tests {
		t.Logf(`test #%v exp version %d Go() to be %v`, i, test.ver, test.exp)
		if got := test.ver.Go(); test.exp != got {
			t.Errorf(`expected version %d Go() to be %v, got %v`,
				test.ver, test.exp, got)
		}
	}
}

func TestVersionTypes(t *testing.T) {
	tests := []struct {
		ver Version
		exp int
	}{
		{Version1, 37},
		{Version2, 41},
		{Version3, 43},
		{Version4, int(EvCount)},
		{Latest, int(EvCount)},
		{Latest + 1, 0},
		{Latest + 2, 0},
		{Latest + 3, 0},
		{0, 0},
	}
	for i, test := range tests {
		t.Logf(`test #%v exp version %d Types() to have length %v`, i, test.ver, test.exp)
		types := test.ver.Types()

		if got := len(types); test.exp != got {
			t.Errorf(`expected version %d Types() to have length %v, got %v`,
				test.ver, test.exp, got)
		}
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		ver Version
		exp string
	}{
		{Version1, `Version(#1 [Go 1.5])`},
		{Version2, `Version(#2 [Go 1.7])`},
		{Version3, `Version(#3 [Go 1.8])`},
		{Version4, `Version(#4 [Go 1.9])`},
		{Latest, `Version(#4 [Go 1.9])`},
		{Latest + 1, `Version(none)`},
		{Latest + 3, `Version(none)`},
		{Latest + 2, `Version(none)`},
		{0, `Version(none)`},
	}
	for i, test := range tests {
		t.Logf(`test #%v exp version %d String() to be %v`, i, test.ver, test.exp)
		if got := test.ver.String(); test.exp != got {
			t.Errorf(`expected version %d String() to be %v, got %v`,
				test.ver, test.exp, got)
		}
	}
}
