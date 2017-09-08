# Toolbox feature: reading Emails

## Introduction
Via any of enabled laitos daemons, you may list and read Emails via IMAP protocol.

laitos enforces secure communication with your Email accounts via TLS. 

## Configuration
Under JSON object `Features`, construct a JSON object called `IMAPAccounts` that has an object `Accounts`.
 
Give each of your accounts a nick name (such as "personal", "work"), then create an object for each account in
`Accounts`. The object must have the following properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>Host</td>
    <td>string</td>
    <td>IMAP(TLS) server's host name, such as "imap.gmail.com".</td>
</tr>
<tr>
    <td>Port</td>
    <td>integer</td>
    <td>Port number of IMAP(TLS) service, it is usually 993.</td>
</tr>
<tr>
    <td>IOTimeoutSec</td>
    <td>integer</td>
    <td>Timeout for IO operations in seconds. 30 is usually good enough.</td>
</tr>
<tr>
    <td>InsecureSkipVerify</td>
    <td>true/false</td>
    <td>
        Set it to "false" for maximum security. If your mail server host does not have a valid TLS certificate, then set
        it to less-secure "true".
    </td>
</tr>
<tr>
    <td>AuthUsername</td>
    <td>string</td>
    <td>
        Email account name, depending on your mail service provider, it usually does not include the @domain.com suffix.
    </td>
</tr>
<tr>
    <td>AuthPassword</td>
    <td>string</td>
    <td>Email account password.</td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

      "IMAPAccounts": {
            "Accounts": {
              "personal-mail": {
                "AuthPassword": "my-gmail-password",
                "AuthUsername": "guohouzuo",
                "Host": "imap.gmail.com",
                "IOTimeoutSec": 30,
                "InsecureSkipVerify": false,
                "MailboxName": "INBOX",
                "Port": 993
              },
              "work-mail": {
                "AuthPassword": "my-work-mail-password",
                "AuthUsername": "hguo",
                "Host": "gwmail.nue.novell.com",
                "IOTimeoutSec": 30,
                "InsecureSkipVerify": true,
                "MailboxName": "INBOX",
                "Port": 993
              }
            }
          },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to run commands:
- List latest Emails: `.il account-nick skip count`, where `account-nick` is the account nick name from configuration,
  `skip` is the number of latest Emails to discard, and `count` is the number of Emails to list after discarding.
- To read Email content: `.ir account-nick message-number`, where `account-nick` is the account nick name from
  configuration, `message-number` is the Email message number from email list response.