package encarchive

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestMakeExtractArchive(t *testing.T) {
	// Create an archive source directory with two directories and two files underneath
	srcDir, err := ioutil.TempDir("", "laitos-launcher-test-archive-src")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)
	if err := ioutil.WriteFile(path.Join(srcDir, "a"), []byte("123"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path.Join(srcDir, "dir1", "dir2"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(path.Join(srcDir, "dir1", "dir2", "b"), []byte("456"), 0600); err != nil {
		t.Fatal(err)
	}

	// Encrypt and make an archive
	key := []byte("a very secure password")
	archiveFile, err := ioutil.TempFile("", "laitos-launcher-test-archive")
	if err != nil {
		t.Fatal(err)
	}
	archiveFile.Close()
	//defer os.Remove(archiveFile.Name())
	if err := MakeArchive(srcDir, archiveFile.Name(), key); err != nil {
		t.Fatal(err)
	}

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
	if err := ExtractArchive(archiveFile.Name(), tmpOut.Name(), destDir, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmpOut.Name()); err == nil {
		t.Fatal("did not delete tmp out")
	}

	// Verify content
	content, err := ioutil.ReadFile(path.Join(destDir, "dir1", "dir2", "b"))
	if err != nil || string(content) != "456" {
		t.Fatal(err, content)
	}
	content, err = ioutil.ReadFile(path.Join(destDir, "a"))
	if err != nil || string(content) != "123" {
		t.Fatal(err, content)
	}
}
