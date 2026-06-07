package main

import (
	"strings"
	"testing"
)

func TestUsageErrorDoesNotSuggestInventedSpeakerNames(t *testing.T) {
	got := usageError().Error()

	if strings.Contains(got, "--speaker") {
		t.Fatalf("usage should not suggest speaker names by default: %q", got)
	}
	if !strings.Contains(got, "mp4-md --out-dir ./out ./videos") {
		t.Fatalf("usage missing default command example: %q", got)
	}
}
