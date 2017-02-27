[![Build Status](https://travis-ci.org/HouzuoGuo/websh.svg?branch=master)](https://travis-ci.org/HouzuoGuo/websh)

Websh
=====
websh is a comprehensive do-everything daemon that delivers many Internet features (not generic Internet Protocol) over alternative communication infrastructures such as PSTN, GSM, and satellite.

The latest features are:
- Run shell commands and retrieve result.
- Run WolframAlpha queries and retrieve result.
- Call another telephone number and speak a piece of text.
- Send SMS to another person.
- Post to Twitter time-line.
- Read the latest updates from Twitter home time-line.
- Post Facebook update.

You must exercise _extreme caution_ when using this software program, inappropriate configuration will severely compromise the security of the host computer. I am not responsible for any damage/potential damage caused to your computers.

Build
=====
Simply check out the source code under your `$GOPATH` directory and run `go build`.

The program only uses Go's standard library, it does not depend on 3rd party libraries.

Configuration
=============
TODO

Copyright
====================
Copyright (c) 2017, Howard Guo <guohouzuo@gmail.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
- Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
- Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
