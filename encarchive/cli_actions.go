package encarchive

import (
	"bufio"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

// CLIStartWebServer invokes the specialised launcher feature.
func CLIStartWebServer(port int, url, archivePath string) {
	ws := WebServer{
		Port:            port,
		URL:             url,
		ArchiveFilePath: archivePath,
	}
	ws.Start()
	// Wait almost indefinitely (~5 years) because this is the main routine of this CLI action
	time.Sleep(5 * 365 * 24 * time.Hour)
}

// CLIExtract reads password from standard input and extracts archive file into the directory.
func CLIExtract(destDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter password to decrypt archive:")
	password, _, err := reader.ReadLine()
	if err != nil {
		misc.DefaultLogger.Fatalf("CLIExtract", "main", err, "failed to read password")
		return
	}
	/*
		This time, the temp file does not have to live in a ramdisk, because the extracted content does not have to be
		in the memory anyways.
	*/
	tmpFile, err := ioutil.TempFile("", "laitos-launcher-utility-extract")
	if err != nil {
		misc.DefaultLogger.Fatalf("CLIExtract", "main", err, "failed to create temporary file")
		return
	}
	tmpFile.Close()
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			misc.DefaultLogger.Printf("CLIExtract", "main", err, "failed to delete temporary file")
		}
	}()
	password = []byte(strings.TrimSpace(string(password)))
	fmt.Println("Result is (nil means success): ", Extract(archivePath, tmpFile.Name(), destDir, password))
}

// CLIArchive reads password from standard input and uses it to encrypt and archive the directory.
func CLIArchive(srcDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter a password to encrypt the archive:")
	password, _, err := reader.ReadLine()
	if err != nil {
		misc.DefaultLogger.Fatalf("CLIExtract", "main", err, "failed to read password")
		return
	}
	password = []byte(strings.TrimSpace(string(password)))
	fmt.Println("Result is (nil means success): ", Archive(srcDir, archivePath, password))
}
