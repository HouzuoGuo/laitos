package cli

import (
	"bufio"
	"context"
	"crypto/tls"
	"io/ioutil"
	pseudoRand "math/rand"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/netboundfileenc"
	"github.com/HouzuoGuo/laitos/netboundfileenc/unlocksvc"
	"github.com/HouzuoGuo/laitos/platform"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// PasswdRPCTimeout is the timeout used by gRPC client when performing operations involving IO, such as creating gRPC connection,
	// invoking RPC functions, etc.
	PasswdRPCTimeout = 5 * time.Second
)

/*
DecryptFile is a distinct routine of laitos main program, it reads password from standard input and uses it to decrypt the
input file in-place.
*/
func DecryptFile(filePath string) {
	platform.SetTermEcho(false)
	defer platform.SetTermEcho(true)
	reader := bufio.NewReader(os.Stdin)
	lalog.DefaultLogger.Info("DecryptFile", "", nil, "Please enter a password to decrypt file \"%s\" (terminal won't echo):\n", filePath)
	password, err := reader.ReadString('\n')
	if err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to read password")
		return
	}
	content, err := misc.Decrypt(filePath, strings.TrimSpace(password))
	if err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to decrypt file")
		return
	}
	if err := ioutil.WriteFile(filePath, content, 0600); err != nil {
		lalog.DefaultLogger.Abort("DecryptFile", "main", err, "failed to decrypt file")
		return
	}
	lalog.DefaultLogger.Info("DecryptFile", "main", nil, "the file has been decrypted in-place")
}

/*
EncryptFile is a distinct routine of laitos main program, it reads password from standard input and uses it to encrypt
the input file in-place.
*/
func EncryptFile(filePath string) {
	platform.SetTermEcho(false)
	defer platform.SetTermEcho(true)
	reader := bufio.NewReader(os.Stdin)
	lalog.DefaultLogger.Info("EncryptFile", "", nil, "please enter a password to encrypt the file \"%s\" (terminal won't echo):\n", filePath)
	password, err := reader.ReadString('\n')
	if err != nil {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "failed to read password")
		return
	}
	lalog.DefaultLogger.Info("EncryptFile", "", nil, "enter the same password again (terminal won't echo):")
	passwordAgain, err := reader.ReadString('\n')
	if err != nil {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "failed to read password")
		return
	}
	if password != passwordAgain {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "The two passwords must match")
		return
	}
	password = strings.TrimSpace(password)
	if err := misc.Encrypt(filePath, password); err != nil {
		lalog.DefaultLogger.Abort("EncryptFile", "main", err, "failed to encrypt file")
		return
	}
	lalog.DefaultLogger.Info("EncryptFile", "", nil, "the file has been encrypted in-place with a password %d characters long", len(password))
}

// GetUnlockingPassword uses a gRPC client to contact the gRPC server, registering intent to obtain unlocking password and then attempts to obtain
// the unlock password immediately.
// If a password is available and hence obtained, the function will return the password string.
// If no password is available or an IO error occurs, the function will return an empty string.
func GetUnlockingPassword(ctx context.Context, useTLS bool, logger lalog.Logger, challengeStr, serverAddr string) string {
	hostName, _ := os.Hostname()
	dialTimeoutCtx, dialTimeoutCancel := context.WithTimeout(ctx, PasswdRPCTimeout)
	defer dialTimeoutCancel()
	clientOpts := []grpc.DialOption{grpc.WithBlock()}
	if useTLS {
		clientOpts = append(clientOpts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		clientOpts = append(clientOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	clientConn, err := grpc.DialContext(dialTimeoutCtx, serverAddr, clientOpts...)
	if err != nil {
		logger.Warning("GetUnlockPassword", serverAddr, err, "failed to establish RPC client connection")
		return ""
	}
	defer func() {
		logger.MaybeMinorError(clientConn.Close())
	}()
	client := unlocksvc.NewPasswordUnlockServiceClient(clientConn)
	invokeTimeoutCtx, invokeTimeoutCancel := context.WithTimeout(ctx, PasswdRPCTimeout)
	defer invokeTimeoutCancel()
	_, err = client.PostUnlockIntent(invokeTimeoutCtx, &unlocksvc.PostUnlockIntentRequest{Identification: &unlocksvc.UnlockAttemptIdentification{
		PID:             uint64(os.Getpid()),
		RandomChallenge: challengeStr,
		HostName:        hostName,
		UserID:          uint64(os.Getuid()),
		UptimeSec:       uint64(platform.GetSystemUptimeSec()),
		SystemLoad:      platform.GetSystemLoad(),
		GOOS:            runtime.GOOS,
		GOARCH:          runtime.GOARCH,
	}})
	if err != nil {
		logger.Warning("GetUnlockPassword", serverAddr, err, "failed to invoke RPC PostUnlockIntent")
		return ""
	}
	resp, err := client.GetUnlockPassword(invokeTimeoutCtx, &unlocksvc.GetUnlockPasswordRequest{Identification: &unlocksvc.UnlockAttemptIdentification{
		PID:             uint64(os.Getpid()),
		RandomChallenge: challengeStr,
		HostName:        hostName,
	}})
	if err != nil {
		logger.Warning("GetUnlockPassword", serverAddr, err, "failed to invoke RPC PostUnlockIntent")
		return ""
	}
	if resp.Exists {
		logger.Info("GetUnlockPassword", serverAddr, nil, "successfully obtained password")
		return resp.Password
	}
	return ""
}

// GetUnlockingPasswordWithRetry contacts each of the gRPC servers, it registered an intent to obtain unlocking password for laitos program data
// and config files, and retries until this password is available and subsequently obtained.
// The function blocks caller until a password has been obtained or the input context is cancelled.
// The default source of PRNG must be seeded prior to calling this function.
func GetUnlockingPasswordWithRetry(ctx context.Context, useTLS bool, logger lalog.Logger, serverAddrs ...string) string {
	challengeStr := netboundfileenc.GetRandomChallenge()
	logger.Info("GetUnlockingPasswordWithRetry", "", nil, "trying to obtain config file decryption password from %d servers via gRPC, using magic challenge \"%s\"", len(serverAddrs), challengeStr)
	for {
		select {
		case <-ctx.Done():
			logger.Info("GetUnlockingPasswordWithRetry", "", nil, "quit on command")
			return ""
		default:
			// The context permits the next attempt to be made
		}
		serverAddr := serverAddrs[pseudoRand.Intn(len(serverAddrs))]
		logger.Info("GetUnlockingPasswordWithRetry", serverAddr, nil, "contacting the server over RPC")
		if password := GetUnlockingPassword(ctx, useTLS, logger, challengeStr, serverAddr); password != "" {
			return password
		}
		// Wait for a few seconds before trying the next server
		time.Sleep(3 * PasswdRPCTimeout)
	}
}

// HandleSecurityDataUtil the main routine of data file maintenance utilities.
func HandleSecurityDataUtil(dataUtil, dataUtilFile string, logger lalog.Logger) {
	if dataUtilFile == "" {
		logger.Abort("main", "", nil, "please provide data utility target file in parameter \"-datautilfile\"")
		return
	}
	switch dataUtil {
	case "encrypt":
		EncryptFile(dataUtilFile)
	case "decrypt":
		DecryptFile(dataUtilFile)
	default:
		logger.Abort("main", "", nil, "please provide mode of operation (encrypt|decrypt) for parameter \"-datautil\"")
	}
}
