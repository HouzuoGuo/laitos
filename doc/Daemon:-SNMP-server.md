# Daemon: SNMP server

## Introduction
The SNMP server implements industrial standard network management protocol - SNMP version 2 with mandatory community name, to offer telemetry data for remote monitoring.

The server supports these OIDs (object identifiers):

<table>
<tr>
    <th>OID</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535</td>
    <td>(for illustration only)</td>
    <td>
        IANA Private Enterprise Number for <a href="http://oid-info.com/get/1.3.6.1.4.1.52535">organisation name "hz.gl"</a> owned by the author of laitos program.
    </td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121</td>
    <td>(for illustration only)</td>
    <td>
		Supported OID nodes are directly underneath this OID. SNMP client may use this OID in a Walk (retrieve sub-tree) operation.
    </td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.100</td>
    <td>octet string</td>
    <td>Public IP address of laitos server</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.101</td>
    <td>integer</td>
    <td>System clock - number of seconds since UNIX Epoch</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.102</td>
    <td>integer</td>
    <td>Program up-time in number of seconds</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.103</td>
    <td>integer</td>
    <td>Number of system CPU cores</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.104</td>
    <td>integer</td>
    <td>Number of OS threads available to laitos (GOMAXPROCS)</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.105</td>
    <td>integer</td>
    <td>Number of goroutines</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.110</td>
    <td>integer</td>
    <td>Number of toolbox commands executed</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.111</td>
    <td>integer</td>
    <td>Number of web server requests served</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.112</td>
    <td>integer</td>
    <td>Number of SMTP conversations served</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.114</td>
    <td>integer</td>
    <td>Number of auto-unlock events</td>
</tr>
<tr>
    <td>1.3.6.1.4.1.52535.121.115</td>
    <td>integer</td>
    <td>Total amount (bytes) of outstanding mail content to be delivered</td>
</tr>
</table>

## Configuration
Construct the following JSON object and place it under key `SNMPDaemon` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>Address</td>
    <td>string</td>
    <td>The address network to listen on.</td>
    <td>"0.0.0.0" - listen on all network interfaces.</td>
</tr>
<tr>
    <td>Port</td>
    <td>integer</td>
    <td>UDP port number to listen on.</td>
    <td>161 - the well-known port number designated for SNMP.</td>
</tr>
<tr>
    <td>PerIPLimit</td>
    <td>integer</td>
    <td>Maximum number of requests a client (identified by IP) may make in a second.</td>
    <td>33 - good enough for querying all supported OIDs 3 times a second</td>
</tr>
<tr>
    <td>CommunityName</td>
    <td>string</td>
    <td>
		This passphrase must be presented by SNMP client in order for them to retrieve OID data.
		<br/>
		Be aware that the design of SNMP does not use encryption to protect this passphrase, it is transmitted in plain text.
	</td>
    <td>(This is a mandatory property without a default value)</td>
</tr>
</table>

Here is a minimal setup example:

<pre>
{
    ...

    "SNMPDaemon": {
        "CommunityName": "my-telemetry-secret-access"
    },

    ...
}
</pre>

## Run
Tell laitos to run SNMP daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,snmpd,...

## Usage
As an industrial standard, SNMP client is readily available on nearly all operating systems. For example, on Ubuntu Linux 
the client software can be installed by software package name `snmp`:

    sudo apt install snmp

Subsequently, use these commands to interact with laitos SNMP server, substitute `server-address` with the server host name or IP:

    # Retrieve all OIDs
    > snmpwalk -v2c -c my-telemetry-secret-access server-address 1.3.6.1.4.1.52535.121
	iso.3.6.1.4.1.52535.121.100 = STRING: "100.200.30.40"
	iso.3.6.1.4.1.52535.121.101 = INTEGER: 1543480241
	iso.3.6.1.4.1.52535.121.102 = INTEGER: 48680
	iso.3.6.1.4.1.52535.121.103 = INTEGER: 4
	iso.3.6.1.4.1.52535.121.104 = INTEGER: 8
	iso.3.6.1.4.1.52535.121.105 = INTEGER: 58
	iso.3.6.1.4.1.52535.121.110 = INTEGER: 0
	iso.3.6.1.4.1.52535.121.111 = INTEGER: 627
	iso.3.6.1.4.1.52535.121.112 = INTEGER: 5
	iso.3.6.1.4.1.52535.121.114 = INTEGER: 0
	iso.3.6.1.4.1.52535.121.115 = INTEGER: 0
	iso.3.6.1.4.1.52535.121.115 = No more variables left in this MIB View (It is past the end of the MIB tree)
	
	# Retrieve a single OID
	> snmpget -v2c -c my-telemetry-secret-access server-address 1.3.6.1.4.1.52535.121.100
	iso.3.6.1.4.1.52535.121.100 = STRING: "40.68.144.242"

## Tips
By design, SNMP does not support encryption, therefore the requests, responses, and most importantly the passphrase will be
transmitted in plain text. You must avoid re-using an important password in the passphrase configuration.