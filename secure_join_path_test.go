package goserver

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSecureJoinPath_RejectsSiblingWithSharedPrefix(t *testing.T) {
	cases := []string{
		"../app-secrets/key.pem",
		"../app2",
		"sub/../../app-secrets",
	}
	for _, relativePath := range cases {
		path, err := NewServer().SecureJoinPath("/srv/app", relativePath)
		if err == nil {
			t.Errorf("%q: expected an error, got %q", relativePath, path)
		}
	}
}

func TestSecureJoinPath_RejectsParent(t *testing.T) {
	path, err := NewServer().SecureJoinPath("/srv/app", "..")
	if err == nil {
		t.Errorf("expected an error, got %q", path)
	}
}

func TestSecureJoinPath_AllowsNamesStartingWithDots(t *testing.T) {
	path, err := NewServer().SecureJoinPath("/srv/app", "..config")
	if err != nil {
		t.Fatalf("expected ..config inside the base to be allowed, got %v", err)
	}
	if want := filepath.FromSlash("/srv/app/..config"); path != want {
		t.Errorf("expected %q, got %q", want, path)
	}
}

func TestSecureJoinPath_AllowsBaseItself(t *testing.T) {
	path, err := NewServer().SecureJoinPath("/srv/app", "sub/..")
	if err != nil {
		t.Fatalf("expected the base itself to be allowed, got %v", err)
	}
	if want := filepath.FromSlash("/srv/app"); path != want {
		t.Errorf("expected %q, got %q", want, path)
	}
}

func TestSecureJoinPath_PackageLevelRejectsSibling(t *testing.T) {
	path, err := SecureJoinPath("/srv/app", "../app-secrets/key.pem")
	if err == nil {
		t.Errorf("expected an error, got %q", path)
	}
}

func TestSecureJoinPath_ErrorsWhenWorkingDirectoryIsGone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot remove the current directory")
	}
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(gone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	path, err := NewServer().SecureJoinPath("relative-base", "file.txt")
	if err == nil {
		t.Errorf("expected an error for a relative base without a working directory, got %q", path)
	}
}
