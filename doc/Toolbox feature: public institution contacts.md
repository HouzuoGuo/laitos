# Toolbox feature: public institution contacts

## Introduction
Via any of enabled laitos daemons, you may query contact information (telephone number, Email, etc) of various public
institutions.

As of 2017-10-30, the toolbox feature will only find contact information of several search-and-rescue institutions.

## Configuration
Configuration is not needed for this feature.

## Usage
Use any capable laitos daemon to run the following toolbox command:

    .c query-text

Where `query-text` is a search keyword or term; if it is omitted, laitos will find all contacts.

The command response will include the name, telephone number, and email address (if available) of the contacts.

Warning! laitos developer Houzuo (Howard) Guo is not affiliated with any institution among the contacts; the developer
cannot be held responsible for monetary and legal consequences associated with usage of the contact information.