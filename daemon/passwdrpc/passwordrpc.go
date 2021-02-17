package passwdrpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/netboundfileenc"
	"github.com/HouzuoGuo/laitos/netboundfileenc/unlocksvc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// DefaultPort is the port number the daemon will listen on in the absence of port number specified by user.
	DefaultPort = 16
	// TestPort is used exclusively by test cases to configure daemon under testing.
	TestPort = 18378
)

// Daemon offers a network listener for a gRPC service that allows other laitos program instances to obtain password for unlocking
// their encrypted config/data files.
type Daemon struct {
	// Address is the IP address to listen on, e.g. 0.0.0.0 to listen on all network interfaces.
	Address string `json:"Address"`
	// Port to listen on.
	Port int `json:"Port"`

	// TLSCertPath is the path in file system pointing to the TLS certificate, this is optional.
	TLSCertPath string `json:"TLSCertPath"`
	// TLSCertPath is the path in file system pointing to the TLS certificate key, this is optional.
	TLSKeyPath string `json:"TLSKeyPath"`

	// PasswordRegister is the gRPC service implementation that handles RPC calls on-the-wire.
	PasswordRegister *netboundfileenc.PasswordRegister `json:"-"`

	rpcServer *grpc.Server
	logger    lalog.Logger
}

// Initialise validates configuration parameters and initialises the internal states of the daemon.
func (daemon *Daemon) Initialise() error {
	if daemon.Address == "" {
		daemon.Address = "0.0.0.0"
	}
	if daemon.Port == 0 {
		daemon.Port = DefaultPort
	}
	daemon.logger = lalog.Logger{ComponentName: "passwdrpc", ComponentID: []lalog.LoggerIDField{{Key: "Port", Value: strconv.Itoa(daemon.Port)}}}
	return nil
}

// StartAndBlock starts a network listener and serves incoming requests using the embedded gRPC server.
// The function will block caller until Stop is called.
func (daemon *Daemon) StartAndBlock() error {
	// Prepare server-side TLS certificate
	serverOpts := make([]grpc.ServerOption, 0)
	if daemon.TLSCertPath != "" {
		contents, _, err := misc.DecryptIfNecessary(misc.ProgramDataDecryptionPassword, daemon.TLSCertPath, daemon.TLSKeyPath)
		if err != nil {
			return err
		}
		tlsCert, err := tls.X509KeyPair(contents[0], contents[1])
		if err != nil {
			return fmt.Errorf("passwdrpc.StartAndBlock: failed to load certificate or key - %v", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewServerTLSFromCert(&tlsCert)))
		daemon.logger.Info("StartAndBlock", "", nil, "using TLS certificate located at %s", daemon.TLSCertPath)
	}
	// Prepare network listener and serve clients
	listener, err := net.Listen("tcp", net.JoinHostPort(daemon.Address, strconv.Itoa(daemon.Port)))
	if err != nil {
		return err
	}
	daemon.rpcServer = grpc.NewServer(serverOpts...)
	defer daemon.Stop()
	unlocksvc.RegisterPasswordUnlockServiceServer(daemon.rpcServer, daemon.PasswordRegister)
	daemon.logger.Info("StartAndBlock", "", nil, "listening on address %s, port %d", daemon.Address, daemon.Port)
	return daemon.rpcServer.Serve(listener)
}

// Stop the daemon, its network listener, and embedded gRPC server.
func (daemon *Daemon) Stop() {
	if daemon.rpcServer != nil {
		daemon.rpcServer.Stop()
		daemon.rpcServer = nil
	}
}

// TestPasswdRPCDaemon is used exclusively by test case to run a comprehensive test routine for the daemon control functions.
// The daemon must have already been completed with all of its configuration and successfully initialised.
// See passwdrpc_test.go for the daemon initialisation routine.
func TestPasswdRPCDaemon(daemon *Daemon, t testingstub.T) {
	daemonStopped := make(chan struct{}, 1)
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			panic(err)
		}
		daemonStopped <- struct{}{}
	}()
	if !misc.ProbePort(1*time.Second, "127.0.0.1", daemon.Port) {
		t.Fatal("daemon did not start on time")
	}
	clientConn, err := grpc.Dial(net.JoinHostPort("127.0.0.1", strconv.Itoa(daemon.Port)), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = clientConn.Close()
	}()
	client := unlocksvc.NewPasswordUnlockServiceClient(clientConn)
	// Verify that RPC functions are reachable
	randChallenge := netboundfileenc.GetRandomChallenge()
	_, err = client.PostUnlockIntent(context.Background(), &unlocksvc.PostUnlockIntentRequest{
		Identification: &unlocksvc.UnlockAttemptIdentification{
			HostName:        "test",
			PID:             123,
			RandomChallenge: randChallenge,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.GetUnlockPassword(context.Background(), &unlocksvc.GetUnlockPasswordRequest{
		Identification: &unlocksvc.UnlockAttemptIdentification{
			HostName:        "test",
			PID:             123,
			RandomChallenge: randChallenge,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Exists || resp.Password != "" {
		t.Fatalf("unexpected password response - exists? %v, password: %s", resp.Exists, resp.Password)
	}
	daemon.Stop()
	<-daemonStopped
	// Repeatedly stopping the daemon should have no negative consequences
	daemon.Stop()
	daemon.Stop()
}
