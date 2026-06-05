package service

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateServiceExtractBinaryFromZipArchive(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "sub2api_laffey_1.0.0_windows_amd64.zip")
	destPath := filepath.Join(tmp, "sub2api")
	want := []byte("windows binary")

	zipFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	zipWriter := zip.NewWriter(zipFile)
	entry, err := zipWriter.Create("sub2api.exe")
	if err != nil {
		t.Fatalf("Create zip entry error = %v", err)
	}
	if _, err := entry.Write(want); err != nil {
		t.Fatalf("Write zip entry error = %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("Close zip writer error = %v", err)
	}
	if err := zipFile.Close(); err != nil {
		t.Fatalf("Close zip file error = %v", err)
	}

	if err := (&UpdateService{}).extractBinary(archivePath, destPath); err != nil {
		t.Fatalf("extractBinary() error = %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("extracted binary=%q, want %q", got, want)
	}
}
