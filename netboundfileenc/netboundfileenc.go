package netboundfileenc

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/datastruct"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/netboundfileenc/unlocksvc"
	"google.golang.org/grpc/peer"
)

// Tip: generate go code for the unlocksvc.proto by navigating into unlocksvc
// directory and then run:
// protoc --go_out=paths=source_relative:. ./unlocksvc.proto

const (
	// MaxRandomChallengeLen is the maximum length accepted for a client-generated random challenge string.
	MaxRandomChallengeLen = 64
)

// GetRandomChallenge returns a randomly generated string that an RPC client may use as challenge string to identify its intent.
// The function uses the default PRNG source internally.
func GetRandomChallenge() string {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// getRPCClientIP returns the string representation of the client IP address that made a gRPC request.
// If the IP cannot be determined, the function will return an empty string.
func getRPCClientIP(ctx context.Context) string {
	peerInfo, ok := peer.FromContext(ctx)
	if ok {
		if tcpAddr, ok := peerInfo.Addr.(*net.TCPAddr); ok {
			return tcpAddr.IP.String()
		}
	}
	return ""
}

// UnlockAttemptRPCClientInfo contains the identification information from an unlocking request along with network transport information about the client.
type UnlockAttemptRPCClientInfo struct {
	*unlocksvc.UnlockAttemptIdentification
	ClientIP string
	Time     time.Time
}

// PasswordRegister provides facilities for an instance of laitos program to register an intent of obtaining unlocking password
// for its config/data files, and then obtain the password after a user has honoured the intent.
type PasswordRegister struct {
	unlocksvc.UnimplementedPasswordUnlockServiceServer
	// IntentsChallenge record the random string challenge generated by the client. They reside in an in-memory buffer, hence the capacity is
	// sufficient for a limited number of entries.
	// The identification of these challenge strings are stored in IntentIdentifications and kept in-sync.
	IntentsChallenge *datastruct.LeastRecentlyUsedBuffer
	// IntentIdentifications is a mapping between client's random string challenge and the identification information. The map is kept in-sync
	// with the elements stored in IntentsChallenge.
	IntentIdentifications map[string]*UnlockAttemptRPCClientInfo
	// FulfilledIntents is a mapping between client's random string challenge and the corresponding unlocking password in plain text.
	FulfilledIntents map[string]string

	rateLimit *lalog.RateLimit
	mutex     *sync.RWMutex
	logger    *lalog.Logger
}

// NewPasswordRegister returns an initialised PasswordRegister.
func NewPasswordRegister(maxIntents, maxCallsPerSec int, logger *lalog.Logger) *PasswordRegister {
	reg := &PasswordRegister{
		IntentsChallenge:      datastruct.NewLeastRecentlyUsedBuffer(maxIntents),
		IntentIdentifications: make(map[string]*UnlockAttemptRPCClientInfo),
		FulfilledIntents:      make(map[string]string),
		rateLimit:             lalog.NewRateLimit(1, maxCallsPerSec, logger),
		mutex:                 new(sync.RWMutex),
		logger:                logger,
	}
	return reg
}

// PostUnlockIntent registers an intent of obtaining unlocking password from a user.
func (reg *PasswordRegister) PostUnlockIntent(ctx context.Context, req *unlocksvc.PostUnlockIntentRequest) (*unlocksvc.PostUnlockIntentResponse, error) {
	clientIP := getRPCClientIP(ctx)
	if !reg.rateLimit.Add(clientIP, true) {
		return nil, errors.New("rate limited")
	}
	reg.mutex.Lock()
	defer reg.mutex.Unlock()
	// Perform sanity check on the request properties
	if hostNameLen := len(req.Identification.HostName); hostNameLen < 3 || hostNameLen > 254 {
		return nil, fmt.Errorf("identification host name must be more than 2 characters and less than 255 characters in length (actual: %d characters)", hostNameLen)
	} else if randChallengeLen := len(req.Identification.RandomChallenge); randChallengeLen < 3 || randChallengeLen > MaxRandomChallengeLen {
		return nil, fmt.Errorf("identification random challenge must be more than 2 characters and less than %d characters in length (actual: %d characters)", MaxRandomChallengeLen, randChallengeLen)
	}
	reg.logger.Warning(clientIP, nil, "Received request: %+v", req.Identification)
	// Memorise the intent by the random challenge string
	_, evicted := reg.IntentsChallenge.Add(req.Identification.RandomChallenge)
	clientIDInfo := &UnlockAttemptRPCClientInfo{ClientIP: clientIP, Time: time.Now()}
	clientIDInfo.UnlockAttemptIdentification = req.Identification
	reg.IntentIdentifications[req.Identification.RandomChallenge] = clientIDInfo
	// When the new intent causes the in-memory buffer of intents to overfill and evict an older intent, delete the corresponding identification record as well.
	delete(reg.IntentIdentifications, evicted)
	return &unlocksvc.PostUnlockIntentResponse{}, nil
}

// GetUnlockPassword retrieves an unlocking password a user has offered.
func (reg *PasswordRegister) GetUnlockPassword(ctx context.Context, req *unlocksvc.GetUnlockPasswordRequest) (*unlocksvc.GetUnlockPasswordResponse, error) {
	clientIP := getRPCClientIP(ctx)
	if !reg.rateLimit.Add(clientIP, true) {
		return nil, errors.New("rate limited")
	}
	reg.mutex.RLock()
	defer reg.mutex.RUnlock()
	password, exists := reg.FulfilledIntents[req.Identification.RandomChallenge]
	if exists {
		reg.logger.Warning(clientIP, nil, "Host name \"%s\", PID %d, has retrieved its unlocking password using random challenge \"%s\"",
			req.Identification.HostName, req.Identification.PID, req.Identification.RandomChallenge)
		// Delete the unlocking password after use. Even if the program instance may restart and request the password again, it will have generated a new random challenge string.
		delete(reg.FulfilledIntents, req.Identification.RandomChallenge)
		delete(reg.IntentIdentifications, req.Identification.RandomChallenge)
		reg.IntentsChallenge.Remove(req.Identification.RandomChallenge)
	}
	return &unlocksvc.GetUnlockPasswordResponse{
		Exists:   exists,
		Password: password,
	}, nil
}

// GetOutstandingIntents returns the identification records of outstanding unlocking intents that are yet to be fulfilled.
func (reg *PasswordRegister) GetOutstandingIntents() map[string]*UnlockAttemptRPCClientInfo {
	reg.mutex.RLock()
	defer reg.mutex.RUnlock()
	ret := make(map[string]*UnlockAttemptRPCClientInfo)
	for challenge, ident := range reg.IntentIdentifications {
		if _, fulfilled := reg.FulfilledIntents[challenge]; !fulfilled {
			ret[challenge] = ident
		}
	}
	return ret
}

// FulfilIntent memorises an unlocking password corresponding to a client generated challenge.
// The function returns true if an outstanding intent corresponds to the challenge, otherwise, it returns false and the password will
// be not be memorised.
func (reg *PasswordRegister) FulfilIntent(challenge, password string) bool {
	reg.mutex.Lock()
	defer reg.mutex.Unlock()
	if _, exists := reg.IntentIdentifications[challenge]; exists {
		reg.FulfilledIntents[challenge] = password
		return true
	}
	return false
}
