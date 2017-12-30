# Toolbox feature: find text in AES encrypted files

## Introduction
Via any of enabled laitos daemons, you may look for search terms among AES encrypted files, such as contact list or
password book.

## Preparation
Prepare AES-encrypted files for laitos:
1. Encrypt a plain text file (such as contact list or password book) using OpenSSL command. When it asks for a password,
   make sure to use a strong password:

        openssl enc -aes256 -in password-book.txt -out encrypted-password-book.bin
2. Delete the plain text file (`password-book.txt`).
3. Use OpenSSL command to reveal the encryption parameters. You will need to enter the password that encrypted the file:

        openssl enc -aes256 -in encrypted-password-book.bin -d -p

   The output will look something like:

        salt=52F078F92F6B5745
        key=EE26A871D2478C5115091B142E09639F8F001163D89EE6DF21A19C5322236368
        iv =9355455468BA2C19961B89F6874ADECC
4. There is now an important decision to make. The toolbox feature configuration stores part of the key prefix, and when
   you use the feature you will have to supply rest of the key. In the example, if configuration has the key prefix
   `EE26A871D2478C5115091B142E09639F8F001163D89EE6DF21A19C5322`, then you will have to enter the rest `236368` every
   time you use this feature. Do not reveal too much of the key in feature configuration, the balance between
   convenience VS security is your choice. Generally speaking, leaving 12 key characters to be supplied every time is
   secure enough.
5. Note down the entire IV value and key prefix of your desired length, they will now be used in feature configuration.

Repeat the procedure for as many files as you wish.s

## Configuration
Under JSON object `Features`, construct a JSON object called `AESDecrypt` that has an inner object called
`EncryptedFiles`. Each key of the inner object is a "shortcut word" that may not include space, later the shortcut word
will be specified in command usage; value of the shortcut word key must come with the following mandatory properties:
<table>
<tr>
    <th>Property</th>
    <th>Type</th>
    <th>Meaning</th>
</tr>
<tr>
    <td>FilePath</td>
    <td>string</td>
    <td>
        Absolute or relative path to the encrypted secrets file.<br>
        (e.g. /root/encrypted-password-book.bin)
    </td>
</tr>
<tr>
    <td>HexIV</td>
    <td>string</td>
    <td>
        The entire "iv =" value from OpenSSL decryption output.<br/>
        Do not include the "iv =" prefix in this string.
    </td>
</tr>
<tr>
    <td>HexKeyPrefix</td>
    <td>string</td>
    <td>The key prefix of your desired length.</td>
</tr>
</table>

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

        "AESDecrypt": {
            "EncryptedFiles": {
                "password-book": {
                    "FilePath": "/root/encrypted-password-book.bin",
                    "HexIV": "9355455468BA2C19961B89F6874ADECC",
                    "HexKeyPrefix": "EE26A871D2478C5115091B142E09639F8F001163D89EE6DF21A19C5322"
                },
                "contacts": {
                    "FilePath": "/root/encrypted-contacts.bin",
                    "HexIV": "A8384FEC68BA865E87B68977A0099D11",
                    "HexKeyPrefix": "4AD724BEF7567DDD411907A3646134BDE23676523445787963135E34CF"
                },
            }
        },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to run the following toolbox command:

    .a shortcut-word rest-of-the-key search-text

Where:
- `shortcut-word` is a single word (may contain hyphen) corresponding to an encrypted file from configuration.
- `rest-of-the-key` is the key suffix that completes the encryption key.
- `search-text` is case insensitive text to be found among decrypted file content.

The command response will be the plain text lines among which `search-text` is found.

## Tips
Generally:
- Do not use any program but OpenSSL to prepare the encrypted secrets file. laitos only recognises the encrypted file
  format specific to OpenSSL.
- For safety reasons, the decryption operation is conducted entirely in system memory, therefore make sure that free
  system memory amounts to at least twice the size of all encrypted files combined.

About OpenSSL versions and compatibility:
- laitos can decrypt files made by OpenSSL version 1.0.x and 1.1.x.
- Version 1.1.x use SHA256 as default method of message digest, if your computer recently upgraded OpenSSL from version
  1.0.x to 1.1.x, laitos will continue to function with both old and newly encrypted files. However, if you wish to use
  command line to manually decrypt an old file after the upgrade, remember to specify parameter `-md md5` in order for
  OpenSSL to successfully decrypt file content.
- The OpenSSL version distributed via Cygwin does not function with laitos.