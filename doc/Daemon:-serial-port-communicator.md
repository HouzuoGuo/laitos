# Daemon: serial port communicator

## Introduction
The serial port communicator daemon continuously looks newly connected devices on serial ports and enables them to invoke toolbox commands at 1200 baud/second.

## Configuration
Construct the following JSON object and place it under key `SerialPortDaemon` in configuration file:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>DeviceGlobPatterns</td>
    <td>array of string</td>
    <td>Glob pattern that find newly serial devices (e.g. /dev/ttyACM*)</td>
    <td>(mandatory propery without a default value)</td>
</tr>
<tr>
    <td>PerDeviceLimit</td>
    <td>integer</td>
    <td>Maximum number of requests a serial port device may make in a second.</td>
    <td>3 - good enough for most cases</td>
</tr>
</table>

Then follow [command processor](https://github.com/HouzuoGuo/laitos/wiki/Command-processor) to construct toolbox command processor configuration in JSON key `SerialPortFilters`.

Here is a complete example:

<pre>
{
    ...

    "SerialPortDaemon": {
        "DeviceGlobPatterns": ["/dev/ttyS*"]
    },
    "SerialPortFilters": {
        "PINAndShortcuts": {
            "PIN": "verysecretpassword",
            "Shortcuts": {
                "watsup": ".eruntime",
                "EmergencyStop": ".estop",
                "EmergencyLock": ".elock"
            }
        },
        "TranslateSequences": {
            "Sequences": [
                ["#/", "|"]
            ]
        },
        "LintText": {
            "CompressSpaces": true,
            "CompressToSingleLine": true,
            "KeepVisible7BitCharOnly": true,
            "MaxLength": 255,
            "TrimSpaces": true
        },
        "NotifyViaEmail": {
            "Recipients": ["me@example.com"]
        }
    },

    ...
}
</pre>

## Run
Tell laitos to run the daemon in the command line:

    sudo ./laitos -config <CONFIG FILE> -daemons ...,serialport,...

## Usage
Configure serial port device to communicate at 1200 baud/second - this is often achieved by manipulating a physical switch or altering a device software setting.
Then connect the device to the computer running laitos software, laitos software continuously scans computer serial ports (determined by configuration `DeviceGlobPatterns`)
to look for newly connected devices every 3 seconds.

3 seconds after connection, the serial port device may begin sending toolbox commands and read their command responses.

## Tips
Arduino-compatible and ESP32-based micro-controllers are easily programmable, and work well as serial communication device operating at 1200 baud/second.
