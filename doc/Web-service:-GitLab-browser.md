# Web service: GitLab browser

## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server), the GitLab browser enables
you to browse and download files from git repositories hosted on GitLab.com.

## Preparation
On GitLab.com, visit [User Settings - Account](https://gitlab.com/profile/account) to retrieve "Private Token".

Then, for each project you wish to browse, visit its "Settings - General - General Project Settings", and note down the
"Project ID", which will soon be used in configuration.

## Configuration
1. Place the following JSON data under JSON key `HTTPHandlers`:
  - String `GitlabBrowserEndpoint` - URL locations that will serve GitLab browser; keep it a secret to yourself, and make
    it difficult to guess.
  - Object `GitlabBrowserEndpointConfig` that comes with the following mandatory properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>PrivateToken</td>
    <td>string</td>
    <td>GitLab account's private token.</td>
</tr>
<tr>
    <td>Recipients</td>
    <td>array of strings</td>
    <td>
        These Email addresses will be notified after files are downloaded.
        <br/>Leave it empty to disable notifications.
    </td>
</tr>
<tr>
    <td>Projects</td>
    <td>{"shortcut-name": #ProjectID#...}</td>
    <td>
        Let user identify git repositories by shortcut names, to browse the git repositories associated with the Project
        IDs.
    </td>
</tr>
</table>

2. If Email notifications are to be enabled, follow [outgoing mail configuration](https://github.com/HouzuoGuo/laitos/wiki/Outgoing-mail-configuration).

Here is an example setup:
<pre>
{
    ...


    "HTTPHandlers": {
        ...

        "GitlabBrowserEndpoint": "/very-secret-gitlab-browser",
        "GitlabBrowserEndpointConfig": {
            "PrivateToken": "zpbzwmoigtmrnkjgb",
            "Projects": {
                "GoodProject1": "3031111",
                "AwesomeProject2": "3032222",
                "BeautifulCode3": "3033333"
            },
            "Recipients": ["me@example.com"]
        },

        ...
    },

    ...
}
</pre>

## Run
GitLab browser is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-web-server#run).

## Usage
In a web browser, navigate to `GitlabBrowserEndpoint` of laitos web server.

To browse git repository:
1. Enter project shortcut name.
2. Click "Go".
3. Navigate to sub-directories by entering their full path and click "Go".

To download a file:
1. Enter project shortcut name.
2. Navigate to directory where file is located in.
3. Enter file name to download.
4. click "Download".

## Tips
The "Private Token" has API access to all of your git repositories, therefore keep it secured, and do not let untrusted
persons get hold of it!