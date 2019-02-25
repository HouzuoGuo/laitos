# Toolbox feature: inspect and control server environment

## Introduction
Via any of enabled laitos daemons, you may monitor server environment (such as memory usage) and control server program
by locking it down or stopping it.

## Configuration
Configuration is not needed for this feature.

## Usage
Use any capable laitos daemon to run the following toolbox command:

    .e <action>

Where action can be:
- `info` - Get program status such as current clock, memory usage, load, etc.
- `log` - Get latest log entries of all kinds - information and warnings.
- `warn` - Get latest warning log entries.
- `stack` - Get the latest stack traces.

It may also be:
- `tune` - Use well known techniques to automatically tune the Linux host that runs laitos.
- `lock` - Keep laitos program running, but disable all toolbox commands and daemons. All HTTP server URLs will return
  status 200 (OK) and an error text. The only way to recover from this state is to restart laitos program manually.
- `stop` - Crash the laitos program.
- `kill` - Destroy (nearly) all directories and files, mounted and local, on the computer hosting laitos program.
  Consequently laitos program crashes soon and the host computer will need to be reinitialised.

## Tips
- In case that a load balancer periodically checks the health status of laitos by visiting its HTTP server, the checks
  will continue to succeed (indicating a healthy server) even after `lock` action is executed. This is intentional.
- The `kill` action attempts to delete most of the files on disk (including those mounted on mount points), and wipes
  disk partitions with zeros. But it cannot guarantee that all disk partitions will be emptied before system crashes.