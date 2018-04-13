package misc

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
