package main

import (
	"bufio"
	"fmt"
	"github.com/HouzuoGuo/laitos/launcher"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// UseLauncher invokes the specialised launcher feature.
func UseLauncher(port int, url, archivePath string) {
	ws := launcher.WebServer{
		Port:            port,
		URL:             url,
		ArchiveFilePath: archivePath,
	}
	ws.Start()
}

// LauncherUtilityExtract reads password from standard input and extracts archive file into the directory.
func LauncherUtilityExtract(destDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter password to decrypt archive:")
	password, _, err := reader.ReadLine()
	if err != nil {
		log.Fatalf("LauncherUtilityExtract: failed to read password - %v", err)
		return
	}
	/*
		This time, the temp file does not have to live in a ramdisk, because the extracted content does not have to be
		in the memory anyways.
	*/
	tmpFile, err := ioutil.TempFile("", "laitos-launcher-utility-extract")
	if err != nil {
		log.Fatalf("LauncherUtilityExtract: failed to create temporary file - %v", err)
		return
	}
	tmpFile.Close()
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			log.Printf("LauncherUtilityExtract: failed to delete temporary file %s - %v", tmpFile.Name(), err)
		}
	}()
	password = []byte(strings.TrimSpace(string(password)))
	fmt.Println("Result is (nil means success): ", launcher.ExtractArchive(archivePath, tmpFile.Name(), destDir, password))
}

// LauncherUtilityArchive reads password from standard input and uses it to encrypt and archive the directory.
func LauncherUtilityArchive(srcDir, archivePath string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please enter a password to encrypt the archive:")
	password, _, err := reader.ReadLine()
	if err != nil {
		log.Fatalf("LauncherUtilityArchive: failed to read password - %v", err)
		return
	}
	password = []byte(strings.TrimSpace(string(password)))
	fmt.Println("Result is (nil means success): ", launcher.MakeArchive(srcDir, archivePath, password))
}
