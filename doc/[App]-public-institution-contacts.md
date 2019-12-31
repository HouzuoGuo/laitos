## Introduction
Via any of enabled laitos daemons, you may query contact information (telephone number, Email, etc) of various public
institutions.

As of 2017-10-30, the app will only find built-in contact information of several search-and-rescue institutions.

## Configuration
This app is always available for use and does not require configuration.

## Usage
Use any capable laitos daemon to invoke the app:

    .c query-text

Where `query-text` is a search keyword or term; if it is omitted, laitos will find all contacts.

The command response will include the name, telephone number, and email address (if available) of the contacts.

Warning! laitos developer Houzuo (Howard) Guo is not affiliated with any institution among the contacts; the developer
cannot be held responsible for monetary and legal consequences associated with usage of the contact information.
