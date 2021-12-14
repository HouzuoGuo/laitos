## Introduction

The app helps to inspect program and system status, including program logs,
resource usage, and system load.

When invoked with certain action names, the app offers limited control over the
life-cycle of the program itself (e.g. put it into an unrecoverable locked
state).

## Configuration

This app is always available for use and does not require configuration.

## Usage

Use any laitos daemon capable of executing app commands to invoke the app:

    .e <action>

These actions help to inspect the program and system status:

- `info` - Get a program and system status report including info such as PID,
  system load, memory usage, etc.
- `log` - Get the latest log entries of all kinds - information and warnings.
- `warn` - Get the latest warning log entries.
- `stack` - Get the latest stack traces.
- `tune` - Automatically tune server kernel parameters for enhanced performance and security.

These actions offer limited control over the life-cycle of the laitos program:

- `lock` - Disable app command execution and disable nearly all daemons with the
  exception of HTTP servers. Web server handlers will respond with status 200 OK
  without processing the incoming request.
  * The only way to recover from this state is to login to the host OS and
    manually end and then restart the laitos program.
- `stop` - Cause the laitos program to crash with a panic.
- `kill` - Erase all disks on the computer host for as long as its OS can
  endure, which effectively destroys all data on the computer running laitos.
  * The laitos program will eventually crash along with the host OS.

## Tips

- The locked-down mode (after executing the `lock` action) does not stop the
  HTP servers because HTTP is often used for health check. By responding with
  status 200 OK the health check will continue to consider laitos program
  healthy. Without leaving HTTP servers running the health check may decide to
  restart laitos program which renders the `lock` action ineffective.
- The `kill` action runs indefinitely for as long as the host OS stays online.
  It however cannot possibly guarantee that all disks are completely erased
  before the host OS inevitably crashes.

About the retrieval of log entries:

- laitos keeps the most recent log entries - warnings and information, buffered
  in memory. Please use the host operating system's facilities
  (e.g. `journalctl`) to inspect older log entries.
- To prevent an individual source from spamming the server and generates and
  thus generate an exceeding quatity of warning logs, the logger will only log a
  single warning entry for each offender and then suppresses further entries
  from the same offender for a short period of time.
