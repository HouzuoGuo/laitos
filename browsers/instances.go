package browsers

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"os"
	"path"
	"sync"
	"time"
)

// SlimerJSImageTag is the latet name and tag of the SlimerJS+Firefox docker image that works best on this version of laitos.
const SlimerJSImageTag = "registry.hub.docker.com/hzgl/slimerjs:20180520"

// Instances manage lifecycle of a fixed number of browser server instances (SlimerJS via Docker).
type Instances struct {
	MaxInstances   int `json:"MaxInstances"`   // Maximum number of instances
	MaxLifetimeSec int `json:"MaxLifetimeSec"` // Unconditionally kill instance after this number of seconds elapse
	BasePortNumber int `json:"BasePortNumber"` // Browser instances listen on a port number beginning from this one

	browserMutex   *sync.Mutex // Protect against concurrent modification to browsers
	browsers       []*Instance // All browsers
	browserCounter int         // Increment only counter
	logger         misc.Logger
}

// Check configuration and initialise internal states.
func (instances *Instances) Initialise() error {
	instances.logger = misc.Logger{
		ComponentName: "browsers.Instances",
		ComponentID:   []misc.LoggerIDField{{"MaxInst", instances.MaxInstances}, {"MaxLifetime", instances.MaxLifetimeSec}},
	}
	if instances.MaxInstances < 1 {
		instances.MaxInstances = 5 // reasonable for a few consumers
	}
	if instances.MaxLifetimeSec < 1 {
		instances.MaxLifetimeSec = 1800 // half hour is quite reasonable
	}
	if instances.BasePortNumber < 1024 {
		return errors.New("browsers.Instances.Initialise: BasePortNumber must be greater than 1023")
	}

	instances.browserMutex = new(sync.Mutex)
	instances.browsers = make([]*Instance, instances.MaxInstances)
	instances.browserCounter = -1
	return nil
}

/*
PrepareDockerImage assumes that docker daemon is already running on the host, and downloads the SlimerJS image.
This may take a while, so caller may consider running this in background.
*/
func (instances *Instances) PrepareDockerImage() error {
	out, err := misc.InvokeProgram(nil, 1800, "docker", "pull", SlimerJSImageTag)
	if err != nil {
		return fmt.Errorf("PrepareDockerImage: failed to pull image - %v: %s", err, out)
	}
	return nil
}

// Acquire a new instance instance. If necessary, kill an existing instance to free up the space for the new instance.
func (instances *Instances) Acquire() (index int, browser *Instance, err error) {
	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	instances.browserCounter++
	index = instances.browserCounter % len(instances.browsers)
	if instance := instances.browsers[index]; instance != nil {
		instance.Kill()
	}
	browser = &Instance{
		RenderImagePath:    path.Join(os.TempDir(), fmt.Sprintf("laitos-browser-instance-render-slimerjs-%d-%d.png", time.Now().Unix(), index)),
		Port:               instances.BasePortNumber + int(index),
		AutoKillTimeoutSec: instances.MaxLifetimeSec,
		Index:              index,
	}
	instances.browsers[index] = browser
	err = browser.Start()
	return
}

/*
Return instance instance of the specified index and match its tag against expectation.
If instance instance does not exist or tag does not match, return nil.
*/
func (instances *Instances) Retrieve(index int, expectedTag string) *Instance {
	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	browser := instances.browsers[index]
	if browser == nil || browser.Tag != expectedTag {
		return nil
	}
	return browser
}

// Forcibly stop all instance instances.
func (instances *Instances) KillAll() {
	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	for _, instance := range instances.browsers {
		if instance != nil {
			instance.Kill()
		}
	}
}
