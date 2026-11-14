package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogRotationWithFixedRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VoiceRewrite.log")
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	write(path, "aaaaa")
	if err := rotateLogBeforeOpen(path, 5, 3); err != nil {
		t.Fatal(err)
	}
	if read(path+".1") != "aaaaa" {
		t.Fatal("first rotated log was lost")
	}
	write(path, "bbbbb")
	if err := rotateLogBeforeOpen(path, 5, 3); err != nil {
		t.Fatal(err)
	}
	if read(path+".1") != "bbbbb" || read(path+".2") != "aaaaa" {
		t.Fatal("backups were not shifted")
	}
	write(path, "ccccc")
	if err := rotateLogBeforeOpen(path, 5, 3); err != nil {
		t.Fatal(err)
	}
	write(path, "ddddd")
	if err := rotateLogBeforeOpen(path, 5, 3); err != nil {
		t.Fatal(err)
	}
	if read(path+".1") != "ddddd" || read(path+".2") != "ccccc" || read(path+".3") != "bbbbb" {
		t.Fatal("rotation did not limit backup generations")
	}
}

func TestLogRotationLeavesSmallFileUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VoiceRewrite.log")
	if err := os.WriteFile(path, []byte("kept"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := rotateLogBeforeOpen(path, 1000, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Fatalf("unexpected rotated file: %v", err)
	}
}
