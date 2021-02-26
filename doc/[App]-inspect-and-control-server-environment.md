## Introduction
Check server system health (such as memory usage), and issue controlling commands to the laitos program,
such as locking and stopping the server daemons.

## Configuration
This app is always available for use and does not require configuration.

## Usage
Use any capable laitos daemon to invoke the app:

    .e <action>

Where action can be:
- `info` - Get program status such as current clock, memory usage, load, etc.
- `log` - Get latest log entries of all kinds - information and warnings.
- `warn` - Get latest warning log entries.
- `stack` - Get the latest stack traces.

It may also be:
- `tune` - Automatically tune server kernel parameters for enhanced performance and security.
- `lock` - Keep laitos program running, but disable all apps and daemons, All web server URLs will return
  status 200 (OK) and an error text. The only way to recover from this state is to restart laitos program manually.
- `stop` - Crash the laitos program.
- `kill` - Destroy (nearly) all directories and files, mounted and local, on the computer hosting laitos program.
  Consequently laitos program crashes soon and the host computer will need to be reinitialised.

## Tips
- In case that a load balancer periodically checks the health status of laitos by visiting its HTTP server, the checks
  will continue to succeed (indicating a healthy server) even after `lock` action is executed. This is intentional to
  prevent the load balancer from replacing the laitos program under lock-down with a healthy instance not under lock-down.
- The `kill` action attempts to delete most of the files on disk (including those mounted on mount points), and wipes
  disk partitions with zeros. It cannot guarantee that the entire disk has been filled with zeros before the computer
  crashes.

About retrieving the log entries:
- laitos keeps the most recent log entries and warning log entries in memory, totalling several hundreds entries. They
  are available for inspection on-demand. The host operating system or hosting platform may have held more log entries
  available for your inspection.
- The warning log entries keep track of the most recent (about three dozens) repeating offenders and will not present
  their repeated offences for inspection. For example, when laitos server refuses 30 DNS clients from querying the server
  yet they keep on going, their IPs will only show up once in the recent warnings, until the server refuses another 30,
  different DNS clients from querying the server.
