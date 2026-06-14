package assert_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/michael-duren/boxes/internal/assert"
)

// spyT records assertion outcomes so we can test the helpers themselves.
// It satisfies assert.TestingT.
type spyT struct {
	errored bool
	fatal   bool
	lastMsg string
}

func (s *spyT) Helper() {}

func (s *spyT) Errorf(format string, args ...any) {
	s.errored = true
	s.lastMsg = fmt.Sprintf(format, args...)
}

func (s *spyT) Fatalf(format string, args ...any) {
	s.fatal = true
	s.lastMsg = fmt.Sprintf(format, args...)
}

// failed reports whether the assertion signaled any failure.
func (s *spyT) failed() bool { return s.errored || s.fatal }

// assertCase drives an assertion against a spy and records the expected outcome.
// errored covers non-fatal failures (Errorf); fatal covers Fatalf.
type assertCase struct {
	name        string
	run         func(t assert.TestingT)
	wantErrored bool
	wantFatal   bool
}

func runCases(t *testing.T, cases []assertCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &spyT{}
			c.run(s)
			if s.errored != c.wantErrored {
				t.Errorf("errored = %v, want %v (msg: %q)", s.errored, c.wantErrored, s.lastMsg)
			}
			if s.fatal != c.wantFatal {
				t.Errorf("fatal = %v, want %v (msg: %q)", s.fatal, c.wantFatal, s.lastMsg)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	runCases(t, []assertCase{
		{"equal ints pass", func(t assert.TestingT) { assert.Equal(t, 42, 42) }, false, false},
		{"unequal ints fail", func(t assert.TestingT) { assert.Equal(t, 1, 2) }, true, false},
		{"unequal strings fail", func(t assert.TestingT) { assert.Equal(t, "a", "b") }, true, false},
	})
}

func TestNotEqual(t *testing.T) {
	runCases(t, []assertCase{
		{"differing values pass", func(t assert.TestingT) { assert.NotEqual(t, 1, 2) }, false, false},
		{"equal values fail", func(t assert.TestingT) { assert.NotEqual(t, 5, 5) }, true, false},
	})
}

func TestDeepEqual(t *testing.T) {
	runCases(t, []assertCase{
		{"equal slices pass", func(t assert.TestingT) { assert.DeepEqual(t, []int{1, 2}, []int{1, 2}) }, false, false},
		{"differing slices fail", func(t assert.TestingT) { assert.DeepEqual(t, []int{1, 2}, []int{1, 3}) }, true, false},
	})
}

func TestTrueFalse(t *testing.T) {
	runCases(t, []assertCase{
		{"True(true) passes", func(t assert.TestingT) { assert.True(t, true) }, false, false},
		{"True(false) fails", func(t assert.TestingT) { assert.True(t, false) }, true, false},
		{"False(false) passes", func(t assert.TestingT) { assert.False(t, false) }, false, false},
		{"False(true) fails", func(t assert.TestingT) { assert.False(t, true) }, true, false},
	})
}

func TestNoError(t *testing.T) {
	runCases(t, []assertCase{
		{"nil error passes", func(t assert.TestingT) { assert.NoError(t, nil) }, false, false},
		{"non-nil error is fatal", func(t assert.TestingT) { assert.NoError(t, errors.New("boom")) }, false, true},
	})
}

func TestError(t *testing.T) {
	runCases(t, []assertCase{
		{"non-nil error passes", func(t assert.TestingT) { assert.Error(t, errors.New("boom")) }, false, false},
		{"nil error fails", func(t assert.TestingT) { assert.Error(t, nil) }, true, false},
	})
}

func TestNilNotNil(t *testing.T) {
	var typedNilPtr *int
	var nilSlice []int

	cases := []struct {
		name    string
		value   any
		wantNil bool
	}{
		{"untyped nil", nil, true},
		{"typed nil pointer", typedNilPtr, true},
		{"nil slice", nilSlice, true},
		{"non-nil int", 0, false},
		{"non-nil string", "", false},
		{"non-nil pointer", new(int), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Nil should fail exactly when the value is not nil.
			s := &spyT{}
			assert.Nil(s, c.value)
			if got := s.failed(); got == c.wantNil {
				t.Errorf("Nil(%s): failed = %v, want %v", c.name, got, !c.wantNil)
			}

			// NotNil is the mirror image.
			s = &spyT{}
			assert.NotNil(s, c.value)
			if got := s.failed(); got != c.wantNil {
				t.Errorf("NotNil(%s): failed = %v, want %v", c.name, got, c.wantNil)
			}
		})
	}
}

func TestContains(t *testing.T) {
	runCases(t, []assertCase{
		{"substring present passes", func(t assert.TestingT) { assert.Contains(t, "hello world", "world") }, false, false},
		{"substring absent fails", func(t assert.TestingT) { assert.Contains(t, "hello", "xyz") }, true, false},
	})
}

func TestOptionalMessageIsIncluded(t *testing.T) {
	tests := []struct {
		name string
		run  func(t assert.TestingT)
		want string
	}{
		{"string message", func(t assert.TestingT) { assert.True(t, false, "custom context") }, "custom context"},
		{"joined values", func(t assert.TestingT) { assert.True(t, false, "id", 42) }, "id 42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &spyT{}
			tt.run(s)
			if !strings.Contains(s.lastMsg, tt.want) {
				t.Errorf("message = %q, want it to contain %q", s.lastMsg, tt.want)
			}
		})
	}
}
