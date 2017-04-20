package browser

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/global"
	"os"
	"path"
	"sync"
)

// Manage lifecycle of a fixed number of browser server instances.
type Browsers struct {
	PhantomJSExecPath string        `json:"PhantomJSExecPath"` // Absolute or relative path to PhantomJS executable
	MaxInstances      int           `json:"MaxInstances"`      // Maximum number of instances
	MaxLifetimeSec    int           `json:"MaxLifetimeSec"`    // Unconditionally kill instance after this number of seconds elapse
	BasePortNumber    int           `json:"BasePortNumber"`    // Browser instances listen on a port number beginning from this one
	Mutex             *sync.Mutex   `json:"-"`                 // Protect against concurrent modification to browsers
	Browsers          []*Browser    `json:"-"`                 // All browsers
	BrowserCounter    int           `json:"-"`                 // Increment only counter
	Logger            global.Logger `json:"-"`
}

// Check configuration and initialise internal states.
func (instances *Browsers) Initialise() error {
	instances.Logger = global.Logger{ComponentName: "Browsers", ComponentID: ""}
	if _, err := os.Stat(instances.PhantomJSExecPath); err != nil {
		return fmt.Errorf("Browsers.Initialise: cannot find PhantomJS executable \"%s\" - %v", instances.PhantomJSExecPath, err)
	}
	if instances.MaxInstances < 1 {
		return errors.New("Browsers.Initialise: MaxInstances must be greater than 0")
	}
	if instances.MaxLifetimeSec < 60 {
		return errors.New("Browsers.Initialise: MaxLifetimeSec must be greater than 60")
	}
	if instances.BasePortNumber < 1024 {
		return errors.New("Browsers.Initialise: BasePortNumber must be greater than 1023")
	}
	instances.Mutex = new(sync.Mutex)
	instances.Browsers = make([]*Browser, instances.MaxInstances)
	instances.BrowserCounter = -1
	return nil
}

// Acquire a new browser instance. If necessary, kill an existing browser to free up the space for the new browser.
func (instances *Browsers) Acquire() (index int, browser *Browser, err error) {
	instances.Mutex.Lock()
	defer instances.Mutex.Unlock()
	instances.BrowserCounter++
	index = instances.BrowserCounter % len(instances.Browsers)
	if browser := instances.Browsers[index]; browser != nil {
		browser.Stop()
	}
	browser = &Browser{
		PhantomJSExecPath:  instances.PhantomJSExecPath,
		RenderImagePath:    path.Join(os.TempDir(), fmt.Sprintf("laitos-browser-instance-render-%d.png", index)),
		Port:               instances.BasePortNumber + int(index),
		AutoKillTimeoutSec: instances.MaxLifetimeSec,
	}
	instances.Browsers[index] = browser
	err = browser.Start()
	return
}

/*
Return browser instance of the specified index and match its tag against expectation.
If browser instance does not exist or tag does not match, return nil.
*/
func (instances *Browsers) Retrieve(index int, expectedTag string) *Browser {
	instances.Mutex.Lock()
	defer instances.Mutex.Unlock()
	browser := instances.Browsers[index]
	if browser == nil || browser.Tag != expectedTag {
		return nil
	}
	return browser
}

// Forcibly stop all browser instances.
func (instances *Browsers) StopAll() {
	instances.Mutex.Lock()
	defer instances.Mutex.Unlock()
	for _, browser := range instances.Browsers {
		browser.Stop()
	}
}
