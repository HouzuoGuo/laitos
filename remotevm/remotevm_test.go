package remotevm

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/HouzuoGuo/laitos/misc"
)

func TestVMInteractions(t *testing.T) {
	// CircleCI doesn't have QEMU
	misc.SkipTestIfCI(t)

	tmpFile, err := ioutil.TempFile("", "laitos-test-vm-interactions-iso")
	if err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	vm := VM{
		NumCPU:    2,
		MemSizeMB: 1024,
		QMPPort:   23421,
	}
	if err := vm.Initialise(); err != nil {
		t.Fatal(err)
	}
	/*
		TinyCore linux is rather small to download, helps to speed up the test execution.
		Be aware that mouse inside TinyCore linux does not move with QEMU mouse input, hence the actual remote VM web
		front-end recommends PuppyLinux instead of TinyCore.
	*/
	if err := vm.DownloadISO("http://tinycorelinux.net/10.x/x86/release/TinyCore-current.iso", tmpFile.Name()); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(tmpFile.Name()); err != nil || stat.Size() < 10*1048576 {
		t.Fatal(err, stat)
	}

	if err := vm.Start(tmpFile.Name()); err != nil {
		t.Fatal(err)
	}
	defer vm.Kill()

	screenshotFile, err := ioutil.TempFile("", "laitos-remotevm-test-screenshot*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_ = screenshotFile.Close()
	// defer os.Remove(screenshotFile.Name())
	// Repeat the following test commands multiple times
	for i := 0; i < 10; i++ {
		// Take screenshot
		if err := vm.TakeScreenshot(screenshotFile.Name()); err != nil {
			t.Fatal(err)
		}
		if stat, err := os.Stat(screenshotFile.Name()); err != nil || stat.Size() < 1024 {
			t.Fatalf("%+v %v %+v", err, stat.Size(), stat)
		}
		t.Log(vm.GetDebugOutput())
		// Press key combo
		if err := vm.ClickKeyboard("ctrl", "alt", "f2"); err != nil {
			t.Fatal(err)
		}
		t.Log(vm.GetDebugOutput())
		// Move mouse around
		if err := vm.MoveMouse(123, 456); err != nil {
			t.Fatal(err)
		}
		// Click mouse buttons
		if err := vm.ClickMouse(true); err != nil {
			t.Fatal(err)
		}
		if err := vm.ClickMouse(false); err != nil {
			t.Fatal(err)
		}
		if err := vm.DoubleClickMouse(true); err != nil {
			t.Fatal(err)
		}
		if err := vm.DoubleClickMouse(false); err != nil {
			t.Fatal(err)
		}
		if err := vm.HoldMouse(true, true); err != nil {
			t.Fatal(err)
		}
		if err := vm.HoldMouse(true, false); err != nil {
			t.Fatal(err)
		}
		if err := vm.HoldMouse(false, true); err != nil {
			t.Fatal(err)
		}
		if err := vm.HoldMouse(false, false); err != nil {
			t.Fatal(err)
		}
		t.Log(vm.GetDebugOutput())
	}
	// Repeatedly calling kill should not lead to undesirable effect
	vm.Kill()
}
