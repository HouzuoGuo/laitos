package encarchive

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// CLIStartWebServer invokes the specialised launcher feature.
func CLIStartWebServer(port int, url, archivePath string) {
	ws := WebServer{
		Port:            port,
		URL:             url,
		ArchiveFilePath: archivePath,
	}
	ws.Start()
}

// CLIExtract reads password from standard input and extracts archive file into the directory.
func CLIExtract(destDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter password to decrypt archive:")
	password, _, err := reader.ReadLine()
	if err != nil {
		log.Fatalf("CLIExtract: failed to read password - %v", err)
		return
	}
	/*
		This time, the temp file does not have to live in a ramdisk, because the extracted content does not have to be
		in the memory anyways.
	*/
	tmpFile, err := ioutil.TempFile("", "laitos-launcher-utility-extract")
	if err != nil {
		log.Fatalf("CLIExtract: failed to create temporary file - %v", err)
		return
	}
	tmpFile.Close()
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			log.Printf("CLIExtract: failed to delete temporary file %s - %v", tmpFile.Name(), err)
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
		log.Fatalf("CLIArchive: failed to read password - %v", err)
		return
	}
	password = []byte(strings.TrimSpace(string(password)))
	fmt.Println("Result is (nil means success): ", Archive(srcDir, archivePath, password))
}
