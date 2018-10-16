package platform

import (
	"os"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	/*
		MaxExternalProgramOutputBytes is the maximum number of bytes (combined stdout and stderr) to keep for an
		external program for caller to retrieve.
	*/
	MaxExternalProgramOutputBytes = 1024 * 1024

	/*
	   UtilityDir is an element of PATH that points to a directory where laitos bundled utility programs are stored. The
	   utility programs are not essential to most of laitos operations, however they come in handy in certain scenarios:
	   - statically linked "busybox" (maintenance daemon uses it to synchronise system clock)
	   - statically linked "toybox" (its rich set of utilities help with shell usage)
	   - dynamically linked "phantomjs" (used by text interactive web browser feature and browser-in-browser HTTP handler)
	*/
	UtilityDir = "/tmp/laitos-util"

	/*
	   CommonPATH is a PATH environment variable value that includes most common executable locations across Unix and Linux.
	   Be aware that, when laitos launches external programs they usually should inherit all of the environment variables from
	   parent process, which may include PATH. However, as an exception, AWS ElasticBeanstalk launches programs via a
	   "supervisord" that resets PATH variable to deliberately exclude sbin directories, therefore, it is often useful to use
	   this hard coded PATH value to launch programs.
	*/
	CommonPATH = UtilityDir + ":/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:/opt/bin:/opt/sbin"
)

var (
	// logger is used by some of the OS platform specific actions that affect laitos process globally.
	logger = lalog.Logger{ComponentName: "platform", ComponentID: []lalog.LoggerIDField{{"PID", os.Getpid()}}}
)
