package app

import "testing"

func TestResolveVersionUsesBuildVersion(t *testing.T) {
	previous := BuildVersion
	BuildVersion = " 1.2.3-test "
	t.Cleanup(func() {
		BuildVersion = previous
	})

	if got := resolveVersion(); got != "1.2.3-test" {
		t.Fatalf("resolveVersion() = %q, want %q", got, "1.2.3-test")
	}
}
