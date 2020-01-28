## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the service starts a virtual
machine that offers a fully functional computer desktop completed with productivity suites and web utilities - all in a single web page.

It is often useful for:
- General web browsing.
- Perform simple office tasks such as word processing and scientific calculation.
- Enable outdated computers (e.g. Windows 95 + Mosaic browser) to enjoy rich experience of modern computer desktop.

## Configuration
Construct the following properties under JSON key `HTTPHandlers`:
1. A string property called `VirtualMachineEndpoint`, value being the URL location that will serve the service. Keep the
   location a secret to yourself and make it difficult to guess.
2. An object called `VirtualMachineEndpointConfig` that has the following properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>LocalUtilityPortNumber</td>
    <td>integer</td>
    <td>
        An arbitrary number above 20000 and below 65535.
        <br/>
        It must not clash with port numbers used by other other components, such as the web-browser-on-a-page.
    </td>
    <td>(This is a mandatory property without a default value)
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "VirtualMachineEndpoint": "/very-secret-my-desktop",
        "VirtualMachineEndpointConfig": {
            "LocalUtilityPortNumber": 15499
        }

        ...
    },

    ...
}
</pre>

## Run
The service is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage
In a web browser, navigate to `VirtualMachineEndpoint` of laitos web server.

To start the desktop (virtual machine):
- Click "Download OS" to download the default desktop OS (Puppy Linux). This only has to be done when laitos server starts for the first time.
  * If you wish to use an alternative desktop OS, enter the download URL of its ISO medium into the text box before clicking "Download OS" button.
  * Click "Refresh Screen" periodically to check the download progress.
- After download finishes, click "Start" to start the desktop.
- Click "Refresh Screen" regularly to view desktop screen.

To use mouse:
- On the desktop screen picture, click at a location of your interest. If your web browser does not understand javascript, you will have to manually
  enter the location coordinates into X and Y text boxes.
- Click on a mouse control button, e.g. "LClick" sends a left mouse click at the coordinates, and "Move To" moves mouse cursor to the coordinates.

To use keyboard:
- Enter codes of keyboard keys that are to be pressed *simultaneously* into the text box.
- Click "Press Simultaneously" to send the key presses to the desktop.
  * If you wish to type words such as "Helsinki", enter two sets of keys "h e l s i n k" and then "i".

## Tips
- The local utility port number from configuration is only for internal localhost use. It does not have to be open on your network firewall.
- laitos server has to have QEMU or KVM installed in order to start the desktop virtual machine. You may rely on [system maintenance](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance)
  to automatically install the software for you.
- laitos server prefers to use KVM to start the desktop virtual machine as KVM offers enhanced performance. When KVM is not available, laitos
  will fall back to QEMU automatically.
- The desktop virtual machine works faster with a lightweight Linux distribution ISO medium. The well-known lightweight PuppyLinux works very well, and laitos recommends it by using it as the default ISO download URL.
