package misc

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

const (
	/*
		  EncryptionIVSizeBytes is the number of random bytes to use in an encrypted file as initialisation vector.
			According to AES implementation, the IV length must be equal to block size.
	*/
	EncryptionIVSizeBytes = aes.BlockSize
	// EncryptionFileHeader is a piece of plain text prepended to encrypted files as a clue to file readers.
	EncryptionFileHeader = "encrypted-by-laitos-software"
)

// EditKeyValue modifies or inserts a key=value pair into the specified file.
func EditKeyValue(filePath, key, value string) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	originalLines := strings.Split(string(content), "\n")
	newLines := make([]string, 0, len(originalLines)+1)
	var foundKey bool
	// Look for all instances of the key appearing as line prefix
	for _, line := range originalLines {
		if trimmedLine := strings.TrimSpace(line); strings.HasPrefix(trimmedLine, key+"=") || strings.HasPrefix(trimmedLine, key+" ") {
			// Successfully matched "key = value" or "key=value"
			foundKey = true
			newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
		} else {
			// Preserve prefix and suffix spaces
			newLines = append(newLines, line)
		}
	}
	if !foundKey {
		newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
	}
	return ioutil.WriteFile(filePath, []byte(strings.Join(newLines, "\n")), 0600)
}

var (
	// ErrInputReaderNil is
	ErrInputReaderNil       = errors.New("input reader is nil")
	ErrInputCapacityInvalid = errors.New("input capacity is invalid")
)

// ReadAllUpTo reads data from input reader until the limited capacity is reached or reader is exhausted (EOF).
func ReadAllUpTo(r io.Reader, upTo int) (ret []byte, err error) {
	ret = []byte{}
	if r == nil {
		err = ErrInputReaderNil
		return
	}
	if upTo < 0 {
		err = ErrInputCapacityInvalid
		return
	}

	return ioutil.ReadAll(io.LimitReader(r, int64(upTo)))
}

/*
DecryptIfNecessary uses the input key to decrypt each of the possibly encrypted input files and returns their content.
If an inpuit file is not encrypted, its content is simply read and returned.
*/
func DecryptIfNecessary(key string, filePaths ...string) (decryptedContent [][]byte, isEncrypted []bool, err error) {
	decryptedContent = make([][]byte, 0)
	isEncrypted = make([]bool, 0)
	for _, aPath := range filePaths {
		var content []byte
		var encrypted bool
		content, encrypted, err = IsEncrypted(aPath)
		if err != nil {
			return
		}
		isEncrypted = append(isEncrypted, encrypted)
		if encrypted {
			content, err = Decrypt(aPath, key)
			if err != nil {
				return
			}
		}
		decryptedContent = append(decryptedContent, content)
	}
	return
}

// IsEncrypted returns true only if the input file is encrypted by laitos program.
func IsEncrypted(filePath string) (content []byte, encrypted bool, err error) {
	// Read the input data in its entirety
	content, err = ioutil.ReadFile(filePath)
	if err != nil {
		return
	}
	if len(content) > len(EncryptionFileHeader) && string(content[:len(EncryptionFileHeader)]) == EncryptionFileHeader {
		encrypted = true
	}
	return
}

/*
Encrypt encrypts the input file in-place via AES. The entire operation is conducted in memory, hence it is
most suited for important yet small files, such as configuration files and certificate keys.
*/
func Encrypt(filePath string, key []byte) error {
	// Read the input data in its entirety in preparation for encryption
	content, encrypted, err := IsEncrypted(filePath)
	if err != nil {
		return err
	}
	// Make sure input file is not already encrypted
	if encrypted {
		return fmt.Errorf("Encrypt: input file \"%s\" is already encrypted", filePath)
	}
	// Generate a random IV
	iv := make([]byte, EncryptionIVSizeBytes)
	_, err = rand.Read(iv)
	if err != nil {
		return fmt.Errorf("failed to acquire random numbers - %v", err)
	}
	// In preparation to overwrite input file with encrypted data, prepend it with header string and IV.
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write([]byte(EncryptionFileHeader)); err != nil {
		return err
	}
	if _, err := file.Write(iv); err != nil {
		return err
	}
	// Initialise encryption data stream using input key and the randomly generated IV
	if len(key) < 32 {
		key = append(key, bytes.Repeat([]byte{0}, 32-len(key))...)
	}
	keyCipher, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to initialise cipher - %v", err)
	}
	ctrStream := cipher.NewCTR(keyCipher, iv)
	cipherWriter := &cipher.StreamWriter{S: ctrStream, W: file}
	// Copy data into encrypted file stream to complete encryptioin
	_, err = cipherWriter.Write(content)
	return err
}

// Decrypt decrypts the input file and returns its content. The entire operation is conducted in memory.
func Decrypt(filePath string, key string) (content []byte, err error) {
	// Read the input encrypted data in its entirety
	encryptedContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	// Make sure input file was encrypted by laitos
	if len(encryptedContent) < len(EncryptionFileHeader)+EncryptionIVSizeBytes || string(encryptedContent[:len(EncryptionFileHeader)]) != EncryptionFileHeader {
		return nil, fmt.Errorf("Decrypt: input file \"%s\" does not appear to have been encrypted by laitos", filePath)
	}
	// Read original IV that was prepended to file
	iv := encryptedContent[len(EncryptionFileHeader) : len(EncryptionFileHeader)+EncryptionIVSizeBytes]
	// Initialise decryption stream using input key and the original IV
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		keyBytes = append(keyBytes, bytes.Repeat([]byte{0}, 32-len(keyBytes))...)
	}
	keyCipher, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise cipher - %v", err)
	}
	ctrStream := cipher.NewCTR(keyCipher, iv)
	cipherReader := &cipher.StreamReader{S: ctrStream, R: bytes.NewReader(encryptedContent[len(EncryptionFileHeader)+EncryptionIVSizeBytes:])}
	content, err = ioutil.ReadAll(cipherReader)
	return
}
