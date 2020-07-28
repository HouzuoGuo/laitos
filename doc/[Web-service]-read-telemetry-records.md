## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the service enables users
to inspect telemetry information records collected by this laitos server, and optionally specify an app command for a
monitored subject to execute when it sends its next telemetry record.

In this article, the term "monitored subject" refers to the telemetry record sender - the one constructing app command
in order to send a telemetry record with sender's system information; on the other hand "laitos server" refers to the
server host of laitos software, the one running daemon programs (web server, DNS server, etc) capable of executing app
commands, and stores received telemetry records in memory.

## Configuration
Under JSON key `HTTPHandlers`, write a string property called `ReportsRetrievalEndpoint`, value being the URL location of
the service. The location should be kept a secret for intended users only - make it difficult to guess.

Here is an example setup:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "`ReportsRetrievalEndpoint`": "/very-secret-telemetry-retrieval",

        ...
    },

    ...
}
</pre>

## Run
The service is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
Use a web browser or generic HTTP client (such as `curl`) to interact with the web service.

### Read telemetry records
Navigate to URL `ReportsRetrievalEndpoint` of laitos web server to read the most recent 1000 telemetry records collected from all
monitored subjects from latest to oldest:

    curl 'https://laitos-server.example.com/very-secret-telemetry-retrieval'

Optionally, specify the maximum number of records to retrieve in an optional parameter `?n=123`:

    curl 'https://laitos-server.example.com/very-secret-telemetry-retrieval?n=123'

Optionally, specify a monitored subject name (which often is the host name of the computer) for retrieving the records in an optional
parameter `?host=SubjectHostName`, the name is not case sensitive:

    curl 'https://laitos-server.example.com/very-secret-telemetry-retrieval?host=SubjectHostName

### Execute an app command on a monitored subject
To store an app command for a monitored subject to execute when it contacts this laitos server next time, use the parameter
`tohost=SubjectHostName` in combination with `cmd=`, keep in mind that the complete app command must include the password PIN of
the that monitored subject, which is often the [phone home telemetry daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-phone-home-telemetry).
This example tells `SubjectHostName` to execute `.s echo abc` when it sends the next telemetry record:

    curl 'https://laitos-server.example.com/very-secret-telemetry-retrieval?tohost=SubjectHostName&cmd=PhoneHomePasswordPIN.s+echo+abc'

Behind the scene:

1. This laitos server stores the pending app command in-memory, patiently waiting for the monitored subject to make contact next
   time.
2. The monitored subject (phone home telemetry daemon) sends the latest telemetry record by constructing a command for app
   [phone home telemetry handler](https://github.com/HouzuoGuo/laitos/wiki/%5BApp%5D-phone-home-telemetry-handler). The laitos server
   app stores the latest record, and in the response, tells monitored subject to run that pending app command.
3. The monitored subject receives the pending app command in the response, validates the password PIN, and executes the app command.
4. After the app command completes execution, the monitored subject will send the next telemetry record with the execution result.

Both the monitored subject and laitos server retain the pending app command and execution result for just over half an hour, to
ensure a high likelihood of successful delivery. Monitored subject will not repeatedly execute an identical command within the time
frame, however it will re-execute the pending app command after half hour elapses.

User may discover the execution result of the pending app command by reading the latest telemetry records collected from the monitored
subject.

User may clear the pending app command by interacting with the web service endpoint, and adding parameters `tohost=SubjectHostName&clear=1`:

    curl 'https://laitos-server.example.com/very-secret-telemetry-retrieval?tohost=SubjectHostName&clear=1'
