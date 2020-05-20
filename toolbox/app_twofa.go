package toolbox

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	// RegexKeyAndAccountName finds a suffix encryption key and account name.
	RegexKeyAndAccountName = regexp.MustCompile(`(\w+)[^\w]+(.*)`)
	ErrBadTwoFAParam       = errors.New(`example: key account_name`)
)

/*
GetTwoFACodeForTimeDivision returns two factor authentication code calculated for the
specific time division using SHA1 method. The caller is responsible for linting the input secret.
The function is heavily inspired by Pierre Carrier's "gauth" (https://github.com/pcarrier/gauth).
*/
func GetTwoFACodeForTimeDivision(secret string, time int64) (string, error) {
	secret = strings.ToUpper(strings.TrimSpace(strings.Replace(secret, " ", "", -1)))
	// Secret is linted and padded with = to nearest 8 bytes
	paddingLength := 8 - (len(secret) % 8)
	if paddingLength < 8 {
		secret += strings.Repeat("=", paddingLength)
	}
	secretBin, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}
	shaHMAC := hmac.New(sha1.New, secretBin)
	timeMessage := make([]byte, 8)
	timeMessage[0] = (byte)(time >> (7 * 8) & 0xff)
	timeMessage[1] = (byte)(time >> (6 * 8) & 0xff)
	timeMessage[2] = (byte)(time >> (5 * 8) & 0xff)
	timeMessage[3] = (byte)(time >> (4 * 8) & 0xff)
	timeMessage[4] = (byte)(time >> (3 * 8) & 0xff)
	timeMessage[5] = (byte)(time >> (2 * 8) & 0xff)
	timeMessage[6] = (byte)(time >> (1 * 8) & 0xff)
	timeMessage[7] = (byte)(time >> (0 * 8) & 0xff)
	if _, err := shaHMAC.Write(timeMessage); err != nil {
		return "", err
	}
	hash := shaHMAC.Sum(nil)
	offset := hash[19] & 0x0f
	truncated := hash[offset : offset+4]
	truncated[0] &= 0x7F
	result := new(big.Int).Mod(new(big.Int).SetBytes(truncated), big.NewInt(1000000))
	return fmt.Sprintf("%06d", result), nil
}

/*
GetTwoFACodes calculates two factor authentication codes using system clock and secret seed
as input. Return previous, current, and next authentication codes in strings.
*/
func GetTwoFACodes(secret string) (previous, current, next string, err error) {
	if previous, err = GetTwoFACodeForTimeDivision(secret, time.Now().Unix()/30-1); err != nil {
		return
	} else if current, err = GetTwoFACodeForTimeDivision(secret, time.Now().Unix()/30); err != nil {
		return
	}
	next, err = GetTwoFACodeForTimeDivision(secret, time.Now().Unix()/30+1)
	return
}

const TwoFATrigger = ".2" // TwoFATrigger is the trigger prefix string of TwoFACodeGenerator feature.

/*
TwoFACodeGenerator generates two factor authentication codes upon request. The generator
takes an AES encrypted secret seed file as input, that looks like "account_name: secret\n...".
*/
type TwoFACodeGenerator struct {
	SecretFile *AESEncryptedFile `json:"SecretFile"` // SecretFile has encrypted account name and 2fa secrets
}

func (codegen *TwoFACodeGenerator) IsConfigured() bool {
	return codegen.SecretFile != nil && codegen.SecretFile.FilePath != ""
}

func (codegen *TwoFACodeGenerator) SelfTest() error {
	if !codegen.IsConfigured() {
		return ErrIncompleteConfig
	}
	if _, err := os.Stat(codegen.SecretFile.FilePath); err != nil {
		return fmt.Errorf("TwoFACodeGenerator.SelfTest: file \"%s\" is not readable - %v", codegen.SecretFile.FilePath, err)
	}
	return nil
}

func (codegen *TwoFACodeGenerator) Initialise() error {
	if err := codegen.SecretFile.Initialise(); err != nil {
		return fmt.Errorf("TwoFACodeGenerator: failed to initialise encrypted secret file - %w", err)
	}
	return nil
}

func (codegen *TwoFACodeGenerator) Trigger() Trigger {
	return TwoFATrigger
}

func (codegen *TwoFACodeGenerator) Execute(cmd Command) (ret *Result) {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	params := RegexKeyAndAccountName.FindStringSubmatch(cmd.Content)
	if len(params) != 3 {
		return &Result{Error: ErrBadTwoFAParam}
	}
	hexKeySuffix := params[1]
	accountName := params[2]
	// Use combination of configured key and input suffix key to decrypt the account secret file
	keySuffix, err := hex.DecodeString(hexKeySuffix)
	if err != nil {
		return &Result{Error: errors.New("Cannot decode hex key")}
	}
	plainContent, err := codegen.SecretFile.Decrypt(keySuffix)
	if err != nil {
		return &Result{Error: err}
	}
	var accountFound bool
	var codeOutput bytes.Buffer
	// Read the account name and secrets among the lines
	for _, line := range strings.Split(string(plainContent), "\n") {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}
		entryName := strings.TrimSpace(fields[0])
		secret := strings.TrimSpace(fields[1])
		// If requested word is among the entry's account name, calculate its code.
		if strings.Contains(entryName, accountName) {
			accountFound = true
			prev, current, next, err := GetTwoFACodes(secret)
			if err != nil {
				return &Result{Error: err}
			}
			codeOutput.WriteString(fmt.Sprintf("%s: %s %s %s\n", entryName, prev, current, next))
		}
	}
	if !accountFound {
		return &Result{Error: errors.New("Cannot find the account")}
	}
	// Calculate 2fa code and return
	return &Result{Output: codeOutput.String()}
}

// GetTestTwoFACodeGenerator returns a configured but uninitialised code generator.
func GetTestTwoFACodeGenerator() TwoFACodeGenerator {
	/*
		Generate sample encrypted file via openssl:
		openssl enc -aes256 -in myinput -out mysample

		openssl enc -aes256 -in mysample -d -p then says:
		salt=81C4054F6CBF34C7
		key=88979C47A572607EB8BD8D2E127B6777602ABCBEE59B5FE5A52ABCB36DE35512
		iv =528B1ED7200E6D7C32AF698DB03BE2CA
		( ^^^ decryption parameters)
		test account: iuu3xchz3ftf6hdh
		( ^^^ decrypted content)
	*/
	sample := []byte{
		0x53, 0x61, 0x6c, 0x74, 0x65, 0x64, 0x5f, 0x5f, 0x81, 0xc4, 0x05, 0x4f, 0x6c, 0xbf, 0x34, 0xc7,
		0x1b, 0x33, 0xc1, 0x7d, 0x30, 0x45, 0x9c, 0x98, 0x62, 0xe7, 0x6e, 0xef, 0x33, 0x61, 0xcc, 0x43,
		0x8f, 0x09, 0x0d, 0xd8, 0x88, 0x04, 0x4e, 0x96, 0x76, 0x3b, 0x7f, 0x6c, 0x95, 0x8f, 0xd8, 0x22,
	}
	filePath := "/tmp/laitos-test2facodegenerator"
	err := ioutil.WriteFile(filePath, sample, 0644)
	if err != nil {
		panic(err)
	}
	return TwoFACodeGenerator{
		SecretFile: &AESEncryptedFile{
			FilePath:     filePath,
			HexIV:        "528B1ED7200E6D7C32AF698DB03BE2CA",
			HexKeyPrefix: "88979C47A572607EB8BD8D2E127B6777602ABCBEE59B5FE5A52ABCB36DE3",
		},
	}
}
