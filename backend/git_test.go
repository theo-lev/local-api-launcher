package main

import (
	"reflect"
	"testing"
)

func TestParseDirtyFiles(t *testing.T) {
	// " M" lines start with a significant space: trimming the raw output
	// used to eat the first letter of the first file ("pom.xml" -> "om.xml")
	out := " M pom.xml\n" +
		"M  src/main/java/App.java\n" +
		"?? untracked.txt\n" +
		"\n"
	got := parseDirtyFiles(out)
	want := []string{"pom.xml", "src/main/java/App.java"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseDirtyFilesClean(t *testing.T) {
	if got := parseDirtyFiles(""); len(got) != 0 {
		t.Fatalf("expected no files for empty output, got %v", got)
	}
}
