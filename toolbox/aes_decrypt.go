package toolbox

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

var (
	// RegexAESShortcutKeySearch finds a shortcut name, encryption key, and search string.
	RegexAESShortcutKeySearch = regexp.MustCompile(`(\w+)[^\w]+(\w+)[^\w]+(.*)`)
	ErrBadAESDecryptParam     = errors.New(`Example: shortcut key to_search`)
)

const OpensslSaltedContentOffset = 16 // openssl writes down irrelevant salt in position 8:16

/*
Attributes about an AES-256-CBC encrypted file.
The file must be encrypted by openssl using password and password salt, e.g.:
openssl enc -aes256 -in <file_to_encrypt> -out <encrypted_file>
*/
type AESEncryptedFile struct {
	FilePath     string `json:"FilePath"`     // Path to the AES-256-CBC file encrypted by openssl-enc with salt
	FileContent  []byte `json:"-"`            // Encrypted file content
	HexIV        string `json:"HexIV"`        // Hex-encoded AES IV
	IV           []byte `json:"-"`            // IV in bytes
	HexKeyPrefix string `json:"HexKeyPrefix"` // Hex-encoded encryption key, to be prepended to the key given in the command.
	KeyPrefix    []byte `json:"-"`            // Key prefix in bytes
}

// Initialise reads encrypted file into memory.
func (file *AESEncryptedFile) Initialise() error {
	if file.HexIV == "" || file.FilePath == "" || file.HexKeyPrefix == "" {
		return fmt.Errorf("AESEncryptedFile.Initialise: file \"%s\" is missing configuration", file.FilePath)
	}
	var err error
	if file.FileContent, err = ioutil.ReadFile(file.FilePath); err != nil {
		return fmt.Errorf("AESEncryptedFile.Initialise: failed to read AES encrypted file \"%s\" - %v", file.FilePath, err)
	}
	if len(file.FileContent) <= OpensslSaltedContentOffset {
		return fmt.Errorf("AESEncryptedFile.Initialise: \"%s\" does not appear to be a file encrypted by openssl", file.FilePath)
	}
	if file.IV, err = hex.DecodeString(file.HexIV); err != nil {
		return fmt.Errorf("AESEncryptedFile.Initialise: failed to decode IV of file \"%s\" - %v", file.FilePath, err)
	}
	if file.KeyPrefix, err = hex.DecodeString(file.HexKeyPrefix); err != nil {
		return fmt.Errorf("AESEncryptedFile.Initialise: failed to decide key prefix of file \"%s\" - %v", file.FilePath, err)
	}
	return nil
}

// Decrypt uses combination of encryption key from configuration and parameter to decrypt the entire file.
func (file *AESEncryptedFile) Decrypt(keySuffix []byte) (plainContent []byte, err error) {
	keyTogether := make([]byte, len(file.KeyPrefix)+len(keySuffix))
	copy(keyTogether, file.KeyPrefix[:])
	copy(keyTogether[len(file.KeyPrefix):], keySuffix[:])
	aesCipher, err := aes.NewCipher(keyTogether)
	if err != nil {
		return
	}
	decryptor := cipher.NewCBCDecrypter(aesCipher, file.IV)
	plainContent = make([]byte, len(file.FileContent))
	decryptor.CryptBlocks(plainContent, file.FileContent[16:])
	return
}

const AESDecryptTrigger = ".a" // AESDecryptTrigger is the trigger prefix string of AESDecrypt feature.

// Decrypt AES-encrypted file and return lines sought by incoming command.
type AESDecrypt struct {
	EncryptedFiles map[string]*AESEncryptedFile `json:"EncryptedFiles"` // shortcut (\w+) vs file attributes
}

func (crypt *AESDecrypt) IsConfigured() bool {
	return crypt.EncryptedFiles != nil && len(crypt.EncryptedFiles) > 0
}

func (crypt *AESDecrypt) SelfTest() error {
	if !crypt.IsConfigured() {
		return ErrIncompleteConfig
	}
	for _, file := range crypt.EncryptedFiles {
		if _, err := os.Stat(file.FilePath); err != nil {
			return fmt.Errorf("AESDecrypt.SelfTest: file \"%s\" is no longer readable - %v", file.FilePath, err)
		}
	}
	return nil
}

func (crypt *AESDecrypt) Initialise() error {
	// Read all encrypted files into memory
	for _, encrypted := range crypt.EncryptedFiles {
		if err := encrypted.Initialise(); err != nil {
			return err
		}
	}
	return nil
}

func (crypt *AESDecrypt) Trigger() Trigger {
	return AESDecryptTrigger
}

func (crypt *AESDecrypt) Execute(cmd Command) (ret *Result) {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	params := RegexAESShortcutKeySearch.FindStringSubmatch(cmd.Content)
	if len(params) != 4 {
		return &Result{Error: ErrBadAESDecryptParam}
	}
	shortcutName := params[1]
	hexKeySuffix := params[2]
	// Use combination of configured key and input suffix key to decrypt the entire file
	keySuffix, err := hex.DecodeString(hexKeySuffix)
	if err != nil {
		return &Result{Error: errors.New("Cannot decode hex key")}
	}
	searchString := strings.ToLower(params[3])
	file, found := crypt.EncryptedFiles[shortcutName]
	if !found {
		return &Result{Error: errors.New("Cannot find " + shortcutName)}
	}
	plainContent, err := file.Decrypt(keySuffix)
	if err != nil {
		return &Result{Error: err}
	}
	// Conduct case insensitive search for the specified text among all lines of decrypted file
	var match bytes.Buffer
	var numMatch int
	for _, line := range strings.Split(string(plainContent), "\n") {
		if strings.Contains(strings.ToLower(line), searchString) {
			match.WriteString(line)
			numMatch++
		}
	}
	// Output is number of matched lines followed by the lines
	return &Result{Output: fmt.Sprintf("%d %s", numMatch, match.String())}
}

const (
	TestAESDecryptFileAlphaName = "alpha"
	TestAESDecryptFileBetaName  = "beta"
)

// Return a configured but uninitialised AESDecrypt.
func GetTestAESDecrypt() AESDecrypt {
	/*
		Generate sample encrypted file via:
		openssl enc -aes256 -in myinput -out mysample

		openssl enc -aes256 -in mysample -d -p then says:
		salt=B332DF2D2F95A86B
		key=F2A515CDDC967C5B0C73FD09264BF67F08A6E1BD273A598F013F6691AAF144A4
		iv =A28DB439E2D112AB6E9FC2B09A73B605
		( ^^^ decryption parameters)
		abc
		def
		ghi
		( ^^^ decrypted content)
	*/
	sample := []byte{
		0x53, 0x61, 0x6c, 0x74, 0x65, 0x64, 0x5f, 0x5f, 0xb3, 0x32, 0xdf, 0x2d, 0x2f, 0x95, 0xa8, 0x6b,
		0xb3, 0x89, 0x59, 0x8c, 0xac, 0x54, 0x27, 0xd1, 0xb3, 0x3c, 0xa1, 0x56, 0xd6, 0x5a, 0x38, 0xe5,
	}
	filePath := "/tmp/laitos-testaesdecrypt"
	err := ioutil.WriteFile(filePath, sample, 0644)
	if err != nil {
		panic(err)
	}
	return AESDecrypt{
		EncryptedFiles: map[string]*AESEncryptedFile{
			TestAESDecryptFileAlphaName: {
				FilePath:     filePath,
				HexIV:        "A28DB439E2D112AB6E9FC2B09A73B605",
				HexKeyPrefix: "F2A515CDDC967C5B0C73FD09264BF67F08A6E1BD273A598F013F6691AAF1",
			},
			TestAESDecryptFileBetaName: {
				FilePath:     filePath,
				HexIV:        "A28DB439E2D112AB6E9FC2B09A73B605",
				HexKeyPrefix: "F2A515CDDC967C5B0C73FD09264BF67F08A6E1BD273A598F013F6691AAF1",
			},
		},
	}
}
