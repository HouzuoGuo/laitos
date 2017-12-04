package main

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	pseudoRand "math/rand"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// LockMemory locks program memory to prevent swapping.
func LockMemory() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
			logger.Warningf("LockMemory", "", err, "failed to lock memory")
			return
		}
		logger.Warningf("LockMemory", "", nil, "program has been locked into memory for safety reasons")
	} else {
		logger.Warningf("LockMemory", "", nil, "program is not running as root (UID 0) hence memory cannot be locked, your private information will leak into swap.")
	}
}

// DumpGoroutinesOnInterrupt installs an interrupt signal handler that dumps all goroutine traces to standard error.
func DumpGoroutinesOnInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			pprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
		}
	}()
}

// ReseedPseudoRand regularly reseeds global pseudo random generator using cryptographic random number generator.
func ReseedPseudoRand() {
	go func() {
		numAttempts := 1
		for ; ; numAttempts++ {
			seedBytes := make([]byte, 8)
			_, err := cryptoRand.Read(seedBytes)
			if err != nil {
				logger.Fatalf("ReseedPseudoRand", "", err, "failed to read from random generator")
			}
			seed, _ := binary.Varint(seedBytes)
			if seed == 0 {
				// If random entropy decodes into an integer that overflows, simply retry.
				continue
			} else {
				pseudoRand.Seed(seed)
				break
			}
		}
		logger.Printf("ReseedPseudoRand", "", nil, "succeeded after %d attempt(s)", numAttempts)
		time.Sleep(2 * time.Minute)
	}()
}

// Stop and disable daemons that may run into port usage conflicts with laitos.
func DisableConflictingDaemons() {
	if os.Getuid() != 0 {
		logger.Fatalf("DisableConflictingDaemons", "", nil, "you must run laitos as root user if you wish to automatically disable conflicting daemons")
	}
	list := []string{"apache", "apache2", "bind", "bind9", "httpd", "lighttpd", "named", "named-chroot", "postfix", "sendmail"}
	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(len(list))
	for _, name := range list {
		go func(name string) {
			defer waitGroup.Done()
			var success bool
			// Disable+stop intensifies three times...
			for i := 0; i < 3; i++ {
				cmds := []string{
					// Some hosting providers still do not use systemd, an example is Amazon Elastic Beanstalk.
					fmt.Sprintf("/etc/init.d/%s stop", name),
					fmt.Sprintf("chkconfig %s off", name),
					fmt.Sprintf("chmod 0000 /etc/init.d/%s", name),

					fmt.Sprintf("systemctl stop %s", name),
					fmt.Sprintf("systemctl disable %s", name),
					fmt.Sprintf("systemctl mask %s", name),
				}
				for _, cmd := range cmds {
					if _, err := misc.InvokeShell(5, "/bin/sh", cmd); err == nil {
						success = true
						// Continue to run subsequent commands to further disable the service
					}
				}
				// Do not overwhelm system with too many consecutive commands
				time.Sleep(1 * time.Second)
			}
			if success {
				logger.Printf("DisableConflictingDaemons", name, nil, "the daemon has been successfully stopped and disabled")
			}
		}(name)
	}
	waitGroup.Wait()
}

// SwapOff turns off all swap files and partitions for improved system security.
func SwapOff() {
	out, err := misc.InvokeProgram([]string{"PATH=" + misc.CommonPATH}, 60, "swapoff", "-a")
	if err == nil {
		logger.Printf("SwapOff", "", nil, "swap is now off")
	} else {
		logger.Warningf("SwapOff", "", err, "failed to turn off swap - %s", out)
	}
}

// Enable or disable terminal echo.
func SetTermEcho(echo bool) {
	term := &syscall.Termios{}
	stdout := os.Stdout.Fd()
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, stdout, syscall.TCGETS, uintptr(unsafe.Pointer(term)))
	if err != 0 {
		logger.Warningf("SetTermEcho", "", err, "syscall failed")
		return
	}
	if echo {
		term.Lflag |= syscall.ECHO
	} else {
		term.Lflag &^= syscall.ECHO
	}
	_, _, err = syscall.Syscall(syscall.SYS_IOCTL, stdout, uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(term)))
	if err != 0 {
		logger.Warningf("SetTermEcho", "", err, "syscall failed")
		return
	}
}
