## Introduction
List mailbox content and read emails.

laitos uses IMAP to communicate with your Email accounts, and it enforces usage of secure communication via TLS.

## Configuration
Under JSON object `Features`, construct a JSON object called `IMAPAccounts` that has an object `Accounts`.

Give each of your accounts a nick name (such as "personal", "work"), then create an object for each account in
`Accounts`. The object must have the following properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
    <th>Default value</th>
</tr>
<tr>
    <td>Host</td>
    <td>string</td>
    <td>IMAP(TLS) server's host name, such as "imap.gmail.com".</td>
    <td>(This is a mandatory property without a default value)</td>
</tr>
<tr>
    <td>AuthUsername</td>
    <td>string</td>
    <td>
        Email account name, depending on your mail service provider, it usually does not include the @domain.com suffix.
    </td>
    <td>(This is a mandatory property without a default value)</td>
</tr>
<tr>
    <td>AuthPassword</td>
    <td>string</td>
    <td>Email account password.</td>
    <td>(This is a mandatory proeprty without a default value)</td>
</tr>
<tr>
    <td>MailboxName</td>
    <td>string</td>
    <td>Name of mailbox to read from, such as "INBOX".</td>
    <td>"INBOX" - the default inbox name of most Email service providers.</td>
</tr>
<tr>
    <td>Port</td>
    <td>integer</td>
    <td>Port number of IMAP(TLS) service.</td>
    <td>993 - the well-known port designated for IMAP.</td>
</tr>
<tr>
    <td>InsecureSkipVerify</td>
    <td>true/false</td>
    <td>
        Set it to "false" for maximum security. If your mail server host does not have a valid TLS certificate, then set
        it to less-secure "true".
    </td>
    <td>false - the secure choice</td>
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
                "Host": "imap.gmail.com"
              },
              "work-mail": {
                "AuthPassword": "my-work-mail-password",
                "AuthUsername": "hguo",
                "Host": "gwmail.nue.novell.com"
              }
            }
          },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to invoke the app:

- List latest emails: `.il account-nick skip count`, where `account-nick` is the account nick name from configuration
  (e.g. personal-mail), `skip` is the number of latest emails to discard (can be 0), and `count` is the number of emails
  to list after discarding.
- To read email content: `.ir account-nick message-number`, where `account-nick` is the account nick name from
  configuration, `message-number` is the email message number from email list response.

## Tips
- Popular email services such as Gmail and Hotmail (Outlook) call the primary mail box `INBOX` (in upper case) for
  incoming emails.
- Gmail has a mail box called `[Gmail]/All Mail` that corresponds to the mail box of all emails, which includes sent,
  junk, and incoming mails.
- The junk mail box of Hotmail (Outlook) is called `Junk` (in mixed case).
- To discover more mail box names, sign in to your email accounts via an email client such as Mozilla Thunderbird and
  inspect settings of each mail box.
