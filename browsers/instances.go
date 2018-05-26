package browsers

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"path"
	"sync"
	"time"
)

const (
	// SlimerJSImageTag is the latet name and tag of the SlimerJS+Firefox docker image that works best on this version of laitos.
	SlimerJSImageTag = "registry.hub.docker.com/hzgl/slimerjs:20180520"

	// DockerMaintenanceIntervalSec is the interval between runs of docker maintenance routine - daemon startup and image pulling.
	DockerMaintenanceIntervalSec = 3600
)

var prepareDockerOnce = new(sync.Once)

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
PrepareDocker starts docker daemon, ensures that docker keeps running, and pulls the docker image for SlimerJS. The
routine requires root privilege to run.
*/
func PrepareDocker(logger misc.Logger) {
	if !misc.EnableStartDaemon("docker") {
		logger.Info("PrepareDocker", "", nil, "failed to enable/start docker daemon")
		// Nevertheless, move on, hoping that docker is actually functional.
	}
	// Download the SlimerJS docker image
	logger.Info("PrepareDocker", "", nil, "pulling %s", SlimerJSImageTag)
	out, err := misc.InvokeProgram(nil, 1800, "docker", "pull", SlimerJSImageTag)
	logger.Info("PrepareDocker", "", nil, "image pulling result: %v - %s", err, out)
	// Turn on ip forwarding so that docker containers will have access to the Internet
	out, err = misc.InvokeProgram(nil, 30, "sysctl", "-w", "net.ipv4.ip_forward=1")
	logger.Info("PrepareDocker", "", nil, "enable ip forwarding result: %v - %s", err, out)
}

// Acquire a new instance instance. If necessary, kill an existing instance to free up the space for the new instance.
func (instances *Instances) Acquire() (index int, browser *Instance, err error) {
	/*
		For improved responsiveness the PrepareDocker routine should have started as soon as Instances is initialised.
		But consider that supervisor initialises configuration and then launches laitos daemons in a child process,
		that causes the PrepareDocker routine to run twice, This inconvenience should have been addressed in the
		supervisor intiialisation process, but for now, sacrifice a bit of user convenience for workaroundability.
	*/
	prepareDockerOnce.Do(func() {
		go func() {
			// Start this background routine in an infinite loop to keep docker running and image available
			for {
				// Enable and start docker daemon
				PrepareDocker(instances.logger)
				time.Sleep(DockerMaintenanceIntervalSec * time.Second)
			}
		}()
	})

	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	instances.browserCounter++
	index = instances.browserCounter % len(instances.browsers)
	if instance := instances.browsers[index]; instance != nil {
		instance.Kill()
	}

	// Be aware that a location underneath /tmp might be private to laitos and will not be visible to container
	renderImageDir := path.Join(SecureTempFileDirectory, fmt.Sprintf("laitos-browser-instance-render-slimerjs-%d-%d", time.Now().Unix(), index))
	browser = &Instance{
		RenderImageDir:     renderImageDir,
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

/*
Forcibly stop all browser instances.
Be aware that, laitos does not use a persistent record of containers spawned, hence if laitos crashes, it will not be
able to kill containers spawned by previous laitos run, which causes failure when user attempts to start a new browser
instance. This is terribly unfortunate and there is not a good remedy other than manually running:

docker ps -a -q -f 'name=laitos-browsers.*' | xargs docker kill -f

Also be aware that the above statement must not be run automatically in KillAll function, because a computer host may
run more than one laitos programs and there is no way to tell whether any of the containers belongs to a crashed laitos.
*/
func (instances *Instances) KillAll() {
	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	for _, instance := range instances.browsers {
		if instance != nil {
			instance.Kill()
		}
	}
}
