package main

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	pseudoRand "math/rand"
	_ "net/http/pprof"
	"os"
	"os/signal"
	runtimePprof "runtime/pprof"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// LockMemory locks program memory to prevent swapping, protecting sensitive user data.
func LockMemory() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
			logger.Warning("LockMemory", "", err, "failed to lock memory")
			return
		}
		logger.Warning("LockMemory", "", nil, "program has been locked into memory for safety reasons")
	} else {
		logger.Warning("LockMemory", "", nil, "program is not running as root (UID 0) hence memory cannot be locked, your private information will leak into swap.")
	}
}

// DumpGoroutinesOnInterrupt installs an interrupt signal handler that dumps all goroutine traces to standard error.
func DumpGoroutinesOnInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			runtimePprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
		}
	}()
}

/*
ReseedPseudoRand regularly reseeds global pseudo random generator using cryptographic random number generator. Some
laitos daemons use the common PRNG instance for their operations.
*/
func ReseedPseudoRand() {
	go func() {
		numAttempts := 1
		for ; ; numAttempts++ {
			seedBytes := make([]byte, 8)
			_, err := cryptoRand.Read(seedBytes)
			if err != nil {
				logger.Abort("ReseedPseudoRand", "", err, "failed to read from random generator")
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
		logger.Info("ReseedPseudoRand", "", nil, "succeeded after %d attempt(s)", numAttempts)
		time.Sleep(2 * time.Minute)
	}()
}

// DisableConflicts prevents system daemons from conflicting with laitos, this is usually done by disabling them.
func DisableConflicts() {
	if os.Getuid() != 0 {
		logger.Abort("DisableConflicts", "", nil, "you must run laitos as root user if you wish to automatically disable system conflicts")
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
				logger.Info("DisableConflicts", name, nil, "the daemon has been successfully stopped and disabled")
			}
		}(name)
	}
	waitGroup.Wait()
	// Prevent systemd-resolved from interfering with laitos DNS daemon
	if _, err := misc.InvokeShell(5, "/bin/sh", "systemctl is-active systemd-resolved"); err == nil {
		// Distributions that use systemd-resolved usually makes resolv.conf a symbol link to an automatically generated file
		os.RemoveAll("/etc/resolv.conf")
		// Overwrite its content with a sane set of servers used as default forwarders in DNS daemon
		newContent := "options rotate timeout:3 attempts:3\n"
		for _, serverPort := range dnsd.DefaultForwarders {
			newContent += "nameserver " + strings.TrimSuffix(serverPort, ":53") + "\n"
		}
		if err := ioutil.WriteFile("/etc/resolv.conf", []byte(newContent), 0644); err == nil {
			logger.Info("DisableConflicts", "systemd-resolved", nil, "/etc/resolv.conf has been overwritten with new content")
			// Completely disable systemd-resolved
			if _, err := misc.InvokeShell(5, "/bin/sh", "systemctl stop systemd-resolved"); err != nil {
				logger.Warning("DisableConflicts", "systemd-resolved", err, "failed to stop the interfering daemon")
			}
			if _, err := misc.InvokeShell(5, "/bin/sh", "systemctl mask systemd-resolved"); err != nil {
				logger.Warning("DisableConflicts", "systemd-resolved", err, "failed to mask the interfering daemon")
			}
		} else {
			logger.Warning("DisableConflicts", "systemd-resolved", nil, "failed to overwrite /etc/resolv.conf, name resolution may malfunction!")
		}
	} else {
		logger.Info("DisableConflicts", "systemd-resolved", nil, "will not touch name resolution settings as resolved is not active")
	}
}

// SwapOff turns off all swap files and partitions for improved system security.
func SwapOff() {
	out, err := misc.InvokeProgram([]string{"PATH=" + misc.CommonPATH}, 60, "swapoff", "-a")
	if err == nil {
		logger.Info("SwapOff", "", nil, "swap is now off")
	} else {
		logger.Warning("SwapOff", "", err, "failed to turn off swap - %s", out)
	}
}

// Enable or disable terminal echo.
func SetTermEcho(echo bool) {
	term := &syscall.Termios{}
	stdout := os.Stdout.Fd()
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, stdout, syscall.TCGETS, uintptr(unsafe.Pointer(term)))
	if err != 0 {
		logger.Warning("SetTermEcho", "", err, "syscall failed")
		return
	}
	if echo {
		term.Lflag |= syscall.ECHO
	} else {
		term.Lflag &^= syscall.ECHO
	}
	_, _, err = syscall.Syscall(syscall.SYS_IOCTL, stdout, uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(term)))
	if err != 0 {
		logger.Warning("SetTermEcho", "", err, "syscall failed")
		return
	}
}

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
