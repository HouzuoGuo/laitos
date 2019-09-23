# Toolbox feature: two factor authentication code generator

## Introduction
Via any of enabled laitos daemons, you may generate two 2FA code for Internet accounts such as Google, Microsoft,
Facebook, etc.

## Preparation
First, set up 2FA for your Internet account:
1. Visit 2FA settings of your Internet account.
2. When the the settings prompts you to scan a barcode, choose "I cannot scan barcode", then the settings should reveal
   a secret text. Note it down on a piece of paper.
3. Use a 2FA code generator application such as [Authy](https://authy.com/features/) to add the secret text and complete
   2FA setup.

Second, prepare account list file for laitos:
1. Create a plain text file with 2FA secret text for any number of accounts, one account per line. For example:

        amazon: 00001111CCCCddddEEEEffffGGGG
        bitbucket: aaaa 1111 2222 3333 4444 5555 6666 7777
        google: 0000bbbb222233334444555566667777

   The secret text is not case sensitive, and spaces among the text do not matter.
2. Encrypt the file using OpenSSL command. When it asks for a password, make sure to use a strong password:

        openssl enc -aes256 -md md5 -in 2fa-secrets.txt -out encrypted-secrets.bin
3. Delete the plain text file (`2fa-secrets.txt`) and shred the piece of paper on which you noted down the secret text.

Third, retrieve encryption parameters from encrypted secrets:
1. Use OpenSSL command to reveal the encryption parameters. You will need to enter the password that encrypted the file:

        openssl enc -aes256 -md md5 -in encrypted-secrets.bin -d -p

   The output will look something like:

        salt=52F078F92F6B5744
        key=EE26A871D2478C51E5091B142E09639F8F001163D89EE6DF21A19C5322236368
        iv =9355455468BA2C18137B89F6874ADECC
2. There is now an important decision to make. The toolbox feature configuration stores part of the key prefix, and when
   you use the feature you will have to supply rest of the key. In the example, if configuration has the key prefix
   `EE26A871D2478C51E5091B142E09639F8F001163D89EE6DF21A19C5322`, then you will have to enter the rest `236368` every
   time you use this feature. Do not reveal too much of the key in feature configuration, the balance between
   convenience VS security is your choice. Generally speaking, leaving 12 key characters to be supplied every time is
   secure enough.
3. Note down the entire IV value and key prefix of your desired length, they will now be used in feature configuration.

## Configuration
Under JSON object `Features`, construct a JSON object called `TwoFACodeGenerator` that has an inner object called
`SecretFile` with the following mandatory properties:
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
        (e.g. /root/encrypted-secrets.bin)
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

        "TwoFACodeGenerator": {
            "SecretFile": {
                "FilePath": "/root/encrypted-secrets.bin",
                "HexIV": "9355455468BA2C18137B89F6874ADECC",
                "HexKeyPrefix": "EE26A871D2478C51E5091B142E09639F8F001163D89EE6DF21A19C5322"
            }
        },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to run the following toolbox command:

    .2 rest-of-the-key account-search

Using the example setup, this command will find Amazon account code:

    .2 236368 amaz

The output will contain three sequences of digits:

    amazon: 123456 23456 34567

The first sequence is the previous code from 30 seconds ago; the middle code is the current code to use for sign-in; and
the last code is for 30 seconds into future. Use the middle code to sign-in to your Internet account right away.

## Tips
- If your Internet account settings only reveals barcode and cannot reveal text secret, then unfortunately it cannot be
  used with laitos.
- Do not use any program but OpenSSL to prepare the encrypted secrets file. laitos only recognises the encrypted file
  format specific to OpenSSL.
- The OpenSSL command supplied with Cygwin appears to work, but in fact it cannot encrypt file properly. Therefore do
  not use the OpenSSL command from Cygwin.
- Correct generation of 2FA codes relies heavily on having a correct system clock. Make sure that your laitos server
  system has correct date and time. Consider running the [maintenance daemon](https://github.com/HouzuoGuo/laitos/wiki/Daemon:-system-maintenance)
  that will automatically correct your system clock.