package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartupFile(t *testing.T) {
	dir := t.TempDir()
	last := filepath.Join(dir, "last.pymovie")
	if err := os.WriteFile(last, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "gone.pymovie")

	for _, c := range []struct {
		name     string
		args     []string
		lastFile string
		want     string
	}{
		{"command line first", []string{"app", "given.pymovie"}, last, "given.pymovie"},
		{"last file", []string{"app"}, last, last},
		{"last file gone", []string{"app"}, gone, "LC-test.pymovie"}, // the tests run in the project folder
		{"no last file", []string{"app"}, "", "LC-test.pymovie"},
		{"last file is a folder", []string{"app"}, dir, "LC-test.pymovie"},
	} {
		if got := startupFile(c.args, c.lastFile); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
