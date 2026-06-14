// Package assert provides lightweight assertion helpers for use in tests.
//
// Each helper calls t.Helper() so failures are reported at the call site, and
// takes an optional trailing message — extra values joined with spaces, à la
// fmt.Sprint — for context:
//
//	assert.Equal(t, got, want)
//	assert.NoError(t, err)
//	assert.True(t, ok, "expected ok for input", input)
//
// Helpers accept a TestingT rather than *testing.T directly so the assertions
// can themselves be unit tested with a spy. *testing.T and *testing.B both
// satisfy TestingT.
package assert

import (
	"fmt"
	"reflect"
	"strings"
)

// TestingT is the subset of testing.TB the assertions need. *testing.T and
// *testing.B satisfy it.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Equal fails the test (non-fatally) if got != want.
func Equal[T comparable](t TestingT, got, want T, msgAndArgs ...any) {
	t.Helper()
	if got != want {
		t.Errorf("not equal:\n  got:  %#v\n  want: %#v%s", got, want, optionalMsg(msgAndArgs...))
	}
}

// NotEqual fails the test (non-fatally) if got == want.
func NotEqual[T comparable](t TestingT, got, want T, msgAndArgs ...any) {
	t.Helper()
	if got == want {
		t.Errorf("expected values to differ, both were %#v%s", got, optionalMsg(msgAndArgs...))
	}
}

// DeepEqual fails the test (non-fatally) if got and want are not deeply equal.
// Use it for slices, maps, and structs that Equal's comparable constraint
// cannot accept.
func DeepEqual(t TestingT, got, want any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("not deeply equal:\n  got:  %#v\n  want: %#v%s", got, want, optionalMsg(msgAndArgs...))
	}
}

// True fails the test (non-fatally) if got is false.
func True(t TestingT, got bool, msgAndArgs ...any) {
	t.Helper()
	if !got {
		t.Errorf("expected true, got false%s", optionalMsg(msgAndArgs...))
	}
}

// False fails the test (non-fatally) if got is true.
func False(t TestingT, got bool, msgAndArgs ...any) {
	t.Helper()
	if got {
		t.Errorf("expected false, got true%s", optionalMsg(msgAndArgs...))
	}
}

// NoError fails the test fatally if err is non-nil. It is fatal because the
// rest of a test usually cannot proceed meaningfully after an unexpected error.
func NoError(t TestingT, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v%s", err, optionalMsg(msgAndArgs...))
	}
}

// Error fails the test (non-fatally) if err is nil.
func Error(t TestingT, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		t.Errorf("expected an error, got nil%s", optionalMsg(msgAndArgs...))
	}
}

// Nil fails the test (non-fatally) if got is not nil. It handles typed nils
// (nil pointers, slices, maps, etc.) as well as an untyped nil interface.
func Nil(t TestingT, got any, msgAndArgs ...any) {
	t.Helper()
	if !isNil(got) {
		t.Errorf("expected nil, got %#v%s", got, optionalMsg(msgAndArgs...))
	}
}

// NotNil fails the test (non-fatally) if got is nil.
func NotNil(t TestingT, got any, msgAndArgs ...any) {
	t.Helper()
	if isNil(got) {
		t.Errorf("expected non-nil value%s", optionalMsg(msgAndArgs...))
	}
}

// Contains fails the test (non-fatally) if s does not contain substr.
func Contains(t TestingT, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q%s", s, substr, optionalMsg(msgAndArgs...))
	}
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// optionalMsg renders the trailing message args as a space-joined suffix. It
// deliberately uses Sprint rather than Sprintf semantics so the helpers are not
// printf-style wrappers — callers pass values, not format verbs, which keeps
// `go vet` from flagging literals like %q at every call site.
func optionalMsg(msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	return ": " + strings.TrimSpace(fmt.Sprintln(msgAndArgs...))
}
