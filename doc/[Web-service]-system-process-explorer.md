## Introduction

Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the service finds all processes
running on the host OS and allows you to pick an individual process to inspect its status (e.g. privileges, PIDs) and resource usage.

The process explorer web service heavily depends on `procfs`, therefore it is only usable on Linux hosts.

## Configuration

Under the JSON key `HTTPHandlers`, add a string property called `ProcessExplorerEndpoint`, value being the URL location of the service.

Keep the location a secret to yourself and make it difficult to guess. Here is an example:

<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "ProcessExplorerEndpoint": "/my-process-explorer",

        ...
    },

    ...
}
</pre>

## Run

The service is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage

### Find the PIDs of live processes

In a web browser, navigate to `ProcessExplorerEndpoint` of laitos web server. In the absence of URL query parameters, the endpoint will
respond with a JSON array of process PIDs that are currently running on the host OS.

### Retrieve the status and resource usage of individual process

Navigate to `ProcessExplorerEndpoint?pid=N` where `N` is the process ID, the endpoint will respond with a JSON struct consisting of:

- Process identity - PID, parent PID, group ID, session ID, etc.
- Privileges - real and effective UID and GID.
- Memory usage and CPU usage (in seconds).
- Process task (thread) stack and wait channel.
- Scheduler statistics - number of context switches, time spent on run-queue and wait-queue.

As a special case, by querying `?pid=0`, the endpoint will retrieve status and information for the laitos program itself.

## Tips

- Make the endpoint difficult to guess, this helps to prevent misuse of the service.
- The process explorer often yields useful performance insights,
  `SchedulerStatsSum` is especially useful:
  - Record the performance reading over time for a trend analysis.
  * If the time spent on wait-queue is trending higher compared to the time
    spent on run-queue, the computer host is becoming over-utilised (too busy).
  - If the number of involuntary context switches is trending higher than the
    voluntary context switches, the process either becoming more
    computing-intensive (in contrast to IO-intensive), or the computer host is
    becoming over-utilised.
