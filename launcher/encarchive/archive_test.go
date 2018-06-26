package encarchive

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestMakeExtractArchive(t *testing.T) {
	// Create an archive source directory with two directories and two files underneath
	srcDir, err := ioutil.TempDir("", "laitos-launcher-test-archive-src")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)
	if err := ioutil.WriteFile(filepath.Join(srcDir, "a"), []byte("123"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "dir1", "dir2"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(srcDir, "dir1", "dir2", "b"), []byte("456"), 0600); err != nil {
		t.Fatal(err)
	}

	// Encrypt and make an archive
	key := []byte("a very secure password")
	archiveFile, err := ioutil.TempFile("", "laitos-launcher-test-archive")
	if err != nil {
		t.Fatal(err)
	}
	archiveFile.Close()
	defer os.Remove(archiveFile.Name())
	if err := Archive(srcDir, archiveFile.Name(), key); err != nil {
		t.Fatal(err)
	}
	fmt.Println("Archived at ", archiveFile.Name())

	// Decrypt the archive
	destDir, err := ioutil.TempDir("", "laitos-launcher-test-archive-dest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(destDir)
	tmpOut, err := ioutil.TempFile("", "laitos-launcher-test-archive-tmp")
	if err != nil {
		t.Fatal(err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())
	if err := Extract(archiveFile.Name(), tmpOut.Name(), destDir, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmpOut.Name()); err == nil {
		t.Fatal("did not delete tmp out")
	}

	// Verify content
	content, err := ioutil.ReadFile(filepath.Join(destDir, "dir1", "dir2", "b"))
	if err != nil || string(content) != "456" {
		t.Fatal(err, content)
	}
	content, err = ioutil.ReadFile(filepath.Join(destDir, "a"))
	if err != nil || string(content) != "123" {
		t.Fatal(err, content)
	}
}
