package browser

import (
	"sync"
	"os"
	"fmt"
	"errors"
)

// Manage lifecycle of a fixed number of browser server instances.
type ServerInstances struct {
	PhantomJSExecPath string `json:"PhantomJSExecPath"` // Absolute or relative path to PhantomJS executable
	MaxInstances   int `json:"MaxInstances"`            // Maximum number of instances
	MaxLifetimeSec int `json:"MaxLifetimeSec"`          // Unconditionally kill instance after this number of seconds elapse
	BasePortNumber int `json:"BasePortNumber"`          // Server instances listen on a port number beginning from this one
	Mutex          *sync.Mutex `json:"-"`               // Protect against concurrent modification to browsers
	Browsers       []*Server `json:"-"`                 // All browsers
	BrowserCounter int64 `json:"-"`                     // Increment only counter
}

// Check configuration and initialise internal states.
func (instances *ServerInstances) Initialise() error {
	if _, err := os.Stat(instances.PhantomJSExecPath); err != nil{
		return fmt.Errorf("ServerInstances.Initialise: cannot find PhantomJS executable \"%s\" - %v", instances.PhantomJSExecPath, err)
	}
	if instances.MaxInstances < 1 {
		return errors.New("ServerInstances.Initialise: MaxInstances must be greater than 0")
	}
	if instances.MaxLifetimeSec < 60 {
		return errors.New("ServerInstances.Initialise: MaxLifetimeSec must be greater than 60")
	}
	if instances.BasePortNumber< 1024 {
		return errors.New("ServerInstances.Initialise: BasePortNumber must be greater than 1023")
	}
	instances.Mutex = new(sync.Mutex)
	instances.Browsers = make([]*Server, instances.MaxInstances)
	instances.BrowserCounter = -1
	return nil
}

// Acquire a new browser instance. If necessary, kill an existing browser to free up the space for the new browser.
func (instances *ServerInstances) Acquire() (index int64, browser *Server) {
	instances.Mutex.Lock()
	defer instances.Mutex.Unlock()
	instances.BrowserCounter++
	index = instances.BrowserCounter% int64(len(instances.Browsers))
	if browser := instances.Browsers[index]; browser != nil {
		browser.Stop()
	}
	// Create a temporary file for
	browser = &Server{
		PhantomJSExecPath:instances.PhantomJSExecPath,
		RenderImagePath:
	}
	instances.Browsers[index] =
}

// Forcibly stop all browser instances.
func (instances *ServerInstances) StopAll() {
	instances.Mutex.Lock()
	defer instances.Mutex.Unlock()
	for _, browser := range instances.Browsers {
		browser.Stop()
	}
}