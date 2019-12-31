## Introduction
Find text in plain text files, such as a telephone book or dictionary.

## Configuration
Under JSON object `Features`, construct a JSON object called `TextSearch` that has an inner object called
`FilePaths`. Each key of the inner object is a "shortcut word" that may not include space, the word will be used in
command composition later; value of the shortcut word key is an absolute or relative path to a plain text file.

Here is an example:
<pre>
{
    ...

    "Features": {
        ...

        "TextSearch": {
            "FilePaths": {
                "phone-num": "/howard/telephone-book.txt",
                "en-fi": "/howard/english-finnish-dictionary.txt"
            }
        },

        ...
    },

    ...
}
</pre>

## Usage
Use any capable laitos daemon to invoke the app:

    .g shortcut-word search-text

Where:
- `shortcut-word` is a single word (may contain hyphen) corresponding to an plain text file in configuration.
- `search-text` is case insensitive text to be found among text.

The command response will be the plain text lines from which `search-text` appears.
