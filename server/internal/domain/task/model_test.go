package task

import "testing"

func TestParseStatus(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"todo", "in_progress", "done"} {
		if got, ok := ParseStatus(value); !ok || string(got) != value {
			t.Fatalf("ParseStatus(%q) = %q, %v", value, got, ok)
		}
	}
	if _, ok := ParseStatus("blocked"); ok {
		t.Fatal("unexpected valid status")
	}
}
