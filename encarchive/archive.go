package encarchive

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
)

const (
	IVSizeBytes = aes.BlockSize //IVSizeBytes is the number of bytes to be generated as IV for encrypting an archive.
)

/*
Archive creates an AES-encrypted zip file out of the content of source directory.
The initial vector is of fixed size and prepended to beginning of output file.
*/
func Archive(sourceDirPath string, outPath string, key []byte) error {
	outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer outFile.Close()
	// Generate a random IV
	iv := make([]byte, IVSizeBytes)
	_, err = rand.Read(iv)
	if err != nil {
		return fmt.Errorf("crypto randomness is empty - %v", err)
	}
	// Prepend the IV to the archive file
	if _, err := outFile.Write(iv); err != nil {
		return err
	}
	// Initialise encryption stream using key and randomly generated IV
	if len(key) < 32 {
		key = append(key, bytes.Repeat([]byte{0}, 32-len(key))...)
	}
	keyCipher, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to initialise cipher - %v", err)
	}
	ctrStream := cipher.NewCTR(keyCipher, iv)
	cipherWriter := &cipher.StreamWriter{S: ctrStream, W: outFile}
	// Initialise zip stream on top of the encryption stream
	zipWriter := zip.NewWriter(cipherWriter)
	// Do not use compression, or the size of ramdisk to uncompress into will be difficult to determine.
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.NoCompression)
	})
	// Enumerate files underneath source directory and place them into zip file
	err = filepath.Walk(sourceDirPath, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer sourceFile.Close()
		// Parameter path is absolute path, but in zip file the path should be a relative one.
		relPath, err := filepath.Rel(sourceDirPath, path)
		if err != nil {
			return err
		}
		fileInZip, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(fileInZip, sourceFile); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := zipWriter.Close(); err != nil {
		return err
	}
	if err := cipherWriter.Close(); err != nil {
		return err
	}
	return nil
}

/*
Extract extracts an AES-encrypted zip file into the destination directory. Returns an error if output directory
does not yet exist, or other IO error occurs.
The temporary file will be the unencrypted archive and will be deleted afterwards.
*/
func Extract(archivePath, tmpPath, outDirPath string, key []byte) error {
	if stat, err := os.Stat(outDirPath); err != nil || !stat.IsDir() {
		return fmt.Errorf("output location %s does is not a readable directory", outDirPath)
	}
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	// The archive is made of only IV and zip
	defer archiveFile.Close()
	stat, err := archiveFile.Stat()
	if err != nil {
		return err
	}
	zipSize := stat.Size() - IVSizeBytes
	// Read IV that was prepended to the file
	iv := make([]byte, IVSizeBytes)
	if n, err := archiveFile.Read(iv); err != nil || n != IVSizeBytes {
		return fmt.Errorf("failed to read IV (%d bytes read) - %v", n, err)
	}
	if len(key) < 32 {
		key = append(key, bytes.Repeat([]byte{0}, 32-len(key))...)
	}
	keyCipher, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to initialise cipher - %v", err)
	}
	// Initialise encryption stream using key and original IV
	ctrStream := cipher.NewCTR(keyCipher, iv)
	cipherReader := &cipher.StreamReader{S: ctrStream, R: archiveFile}
	// Decrypt the archive into temporary file
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary output at %s - %v", tmpPath, err)
	}
	defer func() {
		if err := os.Remove(tmpPath); err != nil {
			log.Panicf("failed to delete unencrypted archive at %s - %v", tmpPath, err)
		}
	}()
	defer tmpFile.Close()
	if _, err := io.Copy(tmpFile, cipherReader); err != nil {
		return fmt.Errorf("failed to decrypt archive - %v", err)
	}
	// Go back to start and initialise zip stream on top of the decrypted file
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	zipReader, err := zip.NewReader(tmpFile, zipSize)
	if err != nil {
		return err
	}
	zipReader.RegisterDecompressor(zip.Deflate, func(in io.Reader) io.ReadCloser {
		return flate.NewReader(in)
	})
	// Unzip each file
	for _, zipFile := range zipReader.File {
		zipFileContent, err := zipFile.Open()
		if err != nil {
			return err
		}
		defer zipFileContent.Close()
		outPath := path.Join(outDirPath, zipFile.Name)
		if err := os.MkdirAll(path.Dir(outPath), 0700); err != nil {
			return err
		}
		unzipDest, err := os.Create(outPath)
		defer unzipDest.Close()
		if err != nil {
			return err
		}
		if _, err := io.Copy(unzipDest, zipFileContent); err != nil {
			return err
		}
	}
	return nil
}
