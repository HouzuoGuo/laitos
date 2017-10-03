package browser

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"os"
	"path"
	"sync"
	"time"
)

// Renderes manages lifecycle of a fixed number of browser server instances.
type Renderers struct {
	PhantomJSExecPath string      `json:"PhantomJSExecPath"` // Absolute or relative path to PhantomJS executable
	MaxInstances      int         `json:"MaxInstances"`      // Maximum number of instances
	MaxLifetimeSec    int         `json:"MaxLifetimeSec"`    // Unconditionally kill instance after this number of seconds elapse
	BasePortNumber    int         `json:"BasePortNumber"`    // Browser instances listen on a port number beginning from this one
	Mutex             *sync.Mutex `json:"-"`                 // Protect against concurrent modification to browsers
	Browsers          []*Renderer `json:"-"`                 // All browsers
	BrowserCounter    int         `json:"-"`                 // Increment only counter
	Logger            misc.Logger `json:"-"`
}

// Check configuration and initialise internal states.
func (instances *Renderers) Initialise() error {
	instances.Logger = misc.Logger{ComponentName: "Renderers", ComponentID: ""}
	if _, err := os.Stat(instances.PhantomJSExecPath); err != nil {
		return fmt.Errorf("Renderers.Initialise: cannot find PhantomJS executable \"%s\" - %v", instances.PhantomJSExecPath, err)
	}
	if err := os.Chmod(instances.PhantomJSExecPath, 0700); err != nil {
		return fmt.Errorf("Renderers.Initialise: failed to chmod PhantomJS - %v", err)
	}
	if instances.MaxInstances < 1 {
		return errors.New("Renderers.Initialise: MaxInstances must be greater than 0")
	}
	if instances.MaxLifetimeSec < 60 {
		return errors.New("Renderers.Initialise: MaxLifetimeSec must be greater than 60")
	}
	if instances.BasePortNumber < 1024 {
		return errors.New("Renderers.Initialise: BasePortNumber must be greater than 1023")
	}
	instances.Mutex = new(sync.Mutex)
	instances.Browsers = make([]*Renderer, instances.MaxInstances)
	instances.BrowserCounter = -1
	return nil
}

// Acquire a new instance instance. If necessary, kill an existing instance to free up the space for the new instance.
func (instances *Renderers) Acquire() (index int, browser *Renderer, err error) {
	instances.Mutex.Lock()
	defer instances.Mutex.Unlock()
	instances.BrowserCounter++
	index = instances.BrowserCounter % len(instances.Browsers)
	if instance := instances.Browsers[index]; instance != nil {
		instance.Kill()
	}
	browser = &Renderer{
		PhantomJSExecPath:  instances.PhantomJSExecPath,
		RenderImagePath:    path.Join(os.TempDir(), fmt.Sprintf("laitos-browser-instance-render-%d-%d.png", time.Now().Unix(), index)),
		Port:               instances.BasePortNumber + int(index),
		AutoKillTimeoutSec: instances.MaxLifetimeSec,
		Index:              index,
	}
	instances.Browsers[index] = browser
	err = browser.Start()
	return
}

/*
Return instance instance of the specified index and match its tag against expectation.
If instance instance does not exist or tag does not match, return nil.
*/
func (instances *Renderers) Retrieve(index int, expectedTag string) *Renderer {
	instances.Mutex.Lock()
	defer instances.Mutex.Unlock()
	browser := instances.Browsers[index]
	if browser == nil || browser.Tag != expectedTag {
		return nil
	}
	return browser
}

// Forcibly stop all instance instances.
func (instances *Renderers) KillAll() {
	instances.Mutex.Lock()
	defer instances.Mutex.Unlock()
	for _, instance := range instances.Browsers {
		if instance != nil {
			instance.Kill()
		}
	}
}
