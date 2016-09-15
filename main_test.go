package main

import (
	"errors"
	"log"
	"testing"
)

func TestIsMailNotificationEnabled(t *testing.T) {
	sh := WebShell{}
	if sh.isMailNotificationEnabled() {
		t.Fatal()
	}
	sh.MailRecipients = []string{"dummy"}
	if sh.isMailNotificationEnabled() {
		t.Fatal()
	}
	sh.MailFrom = "abc"
	if sh.isMailNotificationEnabled() {
		t.Fatal()
	}
	sh.MailAgentAddressPort = "example.com:25"
	if !sh.isMailNotificationEnabled() {
		t.Fatal()
	}
}

func TestLogAndNotify(t *testing.T) {
	sh := WebShell{
		MailRecipients:       []string{"dummy"},
		MailFrom:             "abc",
		MailAgentAddressPort: "example.com:25"}
	log.Print("========= Observe one log message ========")
	sh.logAndNotify("stmt", "output")
	log.Print("===========================================")
	// the background goroutine should spit another log message, but it won't happen unless we wait.
}

func TestLintCommandOutput(t *testing.T) {
	if out := lintCommandOutput(nil, "", 0, false, false); out != "EMPTY OUTPUT" {
		t.Fatal(out)
	}
	if out := lintCommandOutput(nil, "0123456789", 0, false, true); out != "EMPTY OUTPUT" {
		t.Fatal(out)
	}
	if out := lintCommandOutput(nil, "0123456789", 10, false, true); out != "0123456789" {
		t.Fatal(out)
	}
	if out := lintCommandOutput(nil, "0123456789abc", 10, false, true); out != "0123456789" {
		t.Fatal(out)
	}
	if out := lintCommandOutput(nil, "0123456789abc", 10, false, true); out != "0123456789" {
		t.Fatal(out)
	}
	if out := lintCommandOutput(errors.New("012345678"), "9", 10, false, true); out != "012345678" {
		t.Fatal(out)
	}
	if out := lintCommandOutput(errors.New("01234567"), "8", 10, false, true); out != "01234567\n8" {
		t.Fatal(out)
	}
	if out := lintCommandOutput(errors.New(" 0123456789 "), " 0123456789 ", 10, false, false); out != "0123456789\n0123456789" {
		t.Fatal(out)
	}
	if out := lintCommandOutput(errors.New(" 012345 \n 6789 "), " 012345 \n 6789 ", 10, true, false); out != "012345#6789#012345#6789" {
		t.Fatal(out)
	}
	utfSample := `S  (siemens)#1 S | 10 dS  (decisiemens);| 1000 mS  (millisiemens);| 0.001 kS  (kilosiemens);| 1×10^-9 abS  (absiemens);(unit officially deprecated);| 1×10^-9 emus of conductance;(unit officially deprecated);| 8.988×10^11 statS  (statsiemens);(unit officially deprecated);| 8.988×10^11 esus of conductance;(unit offic`
	if out := lintCommandOutput(nil, utfSample, 80, true, true); out != "S  (siemens)#1 S | 10 dS  (decisiemens);| 1000 mS  (millisiemens);| 0.001 kS  (k" {
		t.Fatal(out)
	}
}

func TestCmdRun(t *testing.T) {
	sh := WebShell{SubHashSlashForPipe: false}
	if out := sh.cmdRun("echo a | grep a #/thisiscomment", 1, 30, false, true); out != "a" {
		t.Fatal(out)
	}
	if out := sh.cmdRun("echo a && false # this is comment", 1, 30, false, true); out != "exit status 1\na" {
		t.Fatal(out)
	}
	if out := sh.cmdRun("echo -e 'a\nb' > /proc/self/fd/1", 1, 30, false, true); out != "a\nb" {
		t.Fatal(out)
	}
	if out := sh.cmdRun(`echo '"abc"' > /proc/self/fd/2`, 1, 30, false, true); out != `"abc"` {
		t.Fatal(out)
	}
	if out := sh.cmdRun(`echo "'abc'"`, 1, 30, false, true); out != `'abc'` {
		t.Fatal(out)
	}
	if out := sh.cmdRun(`sleep 2`, 1, 16, false, true); out != "exit status 143" {
		t.Fatal(out)
	}
	if out := sh.cmdRun(`echo 01234567891234567`, 1, 16, false, true); out != "0123456789123456" {
		t.Fatal(out)
	}
	if out := sh.cmdRun("echo a #/ grep a", 1, 30, false, true); out != "a" {
		t.Fatal(out)
	}
	if out := sh.cmdRun("echo a && false # this is comment", 1, 30, false, true); out != "exit status 1\na" {
		t.Fatal(out)
	}
	if out := sh.cmdRun("echo -e 'a\nb' > /proc/self/fd/1", 1, 30, false, true); out != "a\nb" {
		t.Fatal(out)
	}
	if out := sh.cmdRun(`echo '"abc"' > /proc/self/fd/2`, 1, 30, false, true); out != `"abc"` {
		t.Fatal(out)
	}
	if out := sh.cmdRun(`echo "'abc'"`, 1, 30, false, true); out != `'abc'` {
		t.Fatal(out)
	}
}

func TestCmdFind(t *testing.T) {
	sh := WebShell{}
	if stmt := sh.cmdFind("echo hi"); stmt != "" {
		t.Fatal(stmt)
	}
	sh = WebShell{PresetMessages: map[string]string{"": "echo hi"}}
	if stmt := sh.cmdFind("echo hi"); stmt != "" {
		t.Fatal(stmt)
	}
	sh = WebShell{PIN: "abc123"}
	if stmt := sh.cmdFind("badpinhello world"); stmt != "" {
		t.Fatal(stmt)
	}
	if stmt := sh.cmdFind("abc123hello world"); stmt != "hello world" {
		t.Fatal(stmt)
	}
	if stmt := sh.cmdFind("   abc123    hello world   \r\n\t  "); stmt != "hello world" {
		t.Fatal(stmt)
	}
	sh = WebShell{PIN: "nopin", PresetMessages: map[string]string{"abc": "123", "def": "456"}}
	if stmt := sh.cmdFind("badbadbad"); stmt != "" {
		t.Fatal(stmt)
	}
	if stmt := sh.cmdFind("   abcfoobar  "); stmt != "123" {
		t.Fatal(stmt)
	}
	if stmt := sh.cmdFind("   deffoobar  "); stmt != "456" {
		t.Fatal(stmt)
	}
}

func TestMailGetProperties(t *testing.T) {
	example1 := `From bounces+21dd7b-root=houzuo.net@sendgrid.net  Sat Mar  5 19:4d 2016
Delivered-To: guohouzuo@gmail.com
Received: by 7.1.1.7 with SMTP id ev10c4;
        Sat, 5 Mar 2016 00:51:44 -0800 (PST)
X-Received: by 1.10.15.6 with SMTP id j60iof.7.1472;
        Sat, 05 Mar 2016 00:51:44 -0800 (PST)
Return-Path: <bounces+9f-guohouzuo=gmail.com@sendgrid.net>
Received: from o7.outbound-mail.sendgrid.net (77.outbound-mail.sendgrid.net. [1.8.5.1])
        by mx.google.com with ESMTPS id x1i.1.201.03.0.0.1.43
        for <guohouzuo@gmail.com>
        (version=TLS1_2 cipher=ECE-RSA-AES28/128);
        Sat, 05 Ma:44 -0800 (PST)
Received-SPF: pass (google.com: domain of bounces+2-guohouzuo=gmail.com@sendgrid.net designates 7.9.8.7 as permitted sender) client-ip=7.9.8.7;
Authentication-Results: mx.google.com;
       spf=pass (google.com: domain of bounces+2-guohouzuo=gmail.com@sendgrid.net designates 1.9.8.7 as permitted sender) smtp.mailfrom=bouncf-guohouzuo=gmail.com@sendgrid.net;
       dkim=pass header.i=@sendgrid.me
DKIM-Signature: v=1; a=rsa-sha1; c=relaxed; d=sendgrid.me;
	h=from:mime-version:subject:to:content-type:x-feedback-id;
	s=smtpapi; bFO1hB+I=; b=Ze/j/DM
	Jylg=
Received: by filte.sendgrid.net with SMTP id filter0w1.20300.56D9
        2016-03-05 08:500 UTC
Received: from MjyOQ (unknown [40.76.6.133])
	by ismad1.sendgrid.net (SG) with HTTP id pwag
	for <guohouzuo@gmail.com>; Sat, .269 +0000 (UTC)
Date: Sat, 05 Mar 2016 080000
From: "Houzuo Guo" <no.reply@example.com>
Mime-Version: 1.0
Subject: message from Houzuo Guo
To: guohouzuo@gmail.com
Message-ID: <pwg@ismd1.sendgrid.net>
Content-type: multipart/alternative; boundary="----------=_1457167900-29001-525"
This is a multi-part message in MIME format...

------------=_1457900-29001-525
Content-Transfer-Encoding: quoted-printable
Content-Type: text/plain; charset=UTF-8

abc

------------=_1457900-29001-525
Content-Type: text/html; charset="UTF-8"
Content-Disposition: inline
Content-Transfer-Encoding: quoted-printable

def
------------=_1457900-29001-525--
`
	example2 := `From howard@localhost.localdomain  Sat Mar  5 12:12:14 2016
X-Original-To: howard
Delivered-To: howard@localhost.localdomain
Date: Sat, 05 Mar 2016 12:12:14 +0100
To: howard@localhost.localdomain
Reply-To: me@example.com
Subject: hi
User-Agent: Heirloom mailx
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
From: howard@localhost.localdomain (Howard Guo)

hi there
`
	if subj, contentType, addr := mailGetProperties("foobar"); subj != "" || contentType != "" || addr != "" {
		t.Fatal(subj, addr)
	}
	if subj, contentType, addr := mailGetProperties(example1); subj != "message from houzuo guo" || contentType != `multipart/alternative; boundary="----------=_1457167900-29001-525"` || addr != "no.reply@example.com" {
		t.Fatal(subj, addr)
	}
	if subj, contentType, addr := mailGetProperties(example2); subj != "hi" || contentType != `text/plain; charset=us-ascii` || addr != "me@example.com" {
		t.Fatal(subj, addr)
	}
}

func TestMailRunCmdPlainText(t *testing.T) {
	example := `From howard@localhost.localdomain  Sat Mar  5 12:17:27 2016
X-Original-To: howard@localhost
Delivered-To: howard@localhost.localdomain
Date: Sat, 05 Mar 2016 12:17:27 +0100
To: howard@localhost.localdomain
Subject: subject does not matter
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
From: howard@localhost.localdomain (Howard Guo)

abcfoobar  echo "'hello world'" | grep hello > /proc/self/fd/2

`
	sh := WebShell{PIN: "abcfoobar", MailTruncateLen: 6, MailTimeoutSec: 1, WebTruncateLen: 200, WebTimeoutSec: 200}
	if stmt, out := sh.mailRunCmd("subject does not matter", "text/plain; charset=us-ascii", example); stmt != `echo "'hello world'" | grep hello > /proc/self/fd/2` && out != "'hello" {
		t.Fatal(stmt, out)
	}
	example = `From howard@localhost.localdomain  Sat Mar  5 12:17:27 2016
X-Original-To: howard@localhost
Delivered-To: howard@localhost.localdomain
Date: Sat, 05 Mar 2016 12:17:27 +0100
To: howard@localhost.localdomain
Subject: subject does not matter
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
From: howard@localhost.localdomain (Howard Guo)

abcfoobar  sleep 2

`
	sh = WebShell{PIN: "abcfoobar", MailTruncateLen: 6, MailTimeoutSec: 1, WebTruncateLen: 200, WebTimeoutSec: 200}
	if stmt, out := sh.mailRunCmd("subject does not matter", "text/plain; charset=us-ascii", example); stmt != `sleep 2` && out != "'exit status 143" {
		t.Fatal(stmt, out)
	}
}

func TestMailRunCmdMultipart(t *testing.T) {
	example := `From bounces+21dd7b-root=houzuo.net@sendgrid.net  Sat Mar  5 19:4d 2016
Delivered-To: guohouzuo@gmail.com
Received: by 7.1.1.7 with SMTP id ev10c4;
        Sat, 5 Mar 2016 00:51:44 -0800 (PST)
X-Received: by 1.10.15.6 with SMTP id j60iof.7.1472;
        Sat, 05 Mar 2016 00:51:44 -0800 (PST)
Return-Path: <bounces+9f-guohouzuo=gmail.com@sendgrid.net>
Received: from o7.outbound-mail.sendgrid.net (77.outbound-mail.sendgrid.net. [1.8.5.1])
        by mx.google.com with ESMTPS id x1i.1.201.03.0.0.1.43
        for <guohouzuo@gmail.com>
        (version=TLS1_2 cipher=ECE-RSA-AES28/128);
        Sat, 05 Ma:44 -0800 (PST)
Received-SPF: pass (google.com: domain of bounces+2-guohouzuo=gmail.com@sendgrid.net designates 7.9.8.7 as permitted sender) client-ip=7.9.8.7;
Authentication-Results: mx.google.com;
       spf=pass (google.com: domain of bounces+2-guohouzuo=gmail.com@sendgrid.net designates 1.9.8.7 as permitted sender) smtp.mailfrom=bouncf-guohouzuo=gmail.com@sendgrid.net;
       dkim=pass header.i=@sendgrid.me
DKIM-Signature: v=1; a=rsa-sha1; c=relaxed; d=sendgrid.me;
	h=from:mime-version:subject:to:content-type:x-feedback-id;
	s=smtpapi; bFO1hB+I=; b=Ze/j/DM
	Jylg=
Received: by filte.sendgrid.net with SMTP id filter0w1.20300.56D9
        2016-03-05 08:500 UTC
Received: from MjyOQ (unknown [40.76.6.133])
	by ismad1.sendgrid.net (SG) with HTTP id pwag
	for <guohouzuo@gmail.com>; Sat, .269 +0000 (UTC)
Date: Sat, 05 Mar 2016 080000
From: "Houzuo Guo" <no.reply@example.com>
Mime-Version: 1.0
Subject: message from Houzuo Guo
To: guohouzuo@gmail.com
Message-ID: <pwg@ismd1.sendgrid.net>
Content-type: multipart/alternative; boundary="----------=_1457209616-22400-170"
X-SG-EID: QmpGM/Hvmhkk3PuFy9pSYLhAJSXKeY+t4JcfPQksuRn
 E=
Status: RO

This is a multi-part message in MIME format...

------------=_1457209616-22400-170
Content-Transfer-Encoding: quoted-printable
Content-Type: text/plain; charset=UTF-8

abc123echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbb=
bbbbbbbbbbbbbbbcccccccccccccccccccccccccccccdddddddddddddddddddddddddeeeeee=
eeeeeeee=20
=20=20
------------=_1457209616-22400-170
Content-Type: text/html; charset="UTF-8"
Content-Disposition: inline
Content-Transfer-Encoding: quoted-printable

aiwowein

------------=_1457209616-22400-170--`
	sh := WebShell{PIN: "abc123", WebTruncateLen: 200, WebTimeoutSec: 1}
	subject, contentType, replyTo := mailGetProperties(example)
	if subject != "message from houzuo guo" || contentType != `multipart/alternative; boundary="----------=_1457209616-22400-170"` || replyTo != "no.reply@example.com" {
		t.Fatal(subject, contentType, replyTo)
	}
	outputMatch := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbcccccccccccccccccccccccccccccdddddddddddddddddddddddddeeeeeeeeeeeeee"
	stmtMatch := "echo " + outputMatch
	if stmt, out := sh.mailRunCmd("subject does not matter", contentType, example); stmt != stmtMatch || out != outputMatch {
		t.Fatal(stmt, out)
	}
}

func TestMysteriousCallAPI(t *testing.T) {
	t.Skip()
	sh := WebShell{
		MysteriousURL:   "https://a",
		MysteriousAddr1: "@example.com",
		MysteriousAddr2: "me@example.com",
		MysteriousID1:   "123456",
		MysteriousID2:   "foo-bar",
	}
	sh.mysteriousCallAPI("hi there")
}

func TestWACallAPI(t *testing.T) {
	t.Skip()
	sh := WebShell{PIN: "abc", WolframAlphaAppID: "FILLME"}
	output := sh.cmdRun(sh.cmdFind("abc#wweather in nuremberg, bavaria, germany"), 12, 500, true, true)
	t.Log(output)
	if len(output) < 200 {
		t.Fatal("output is too short", len(output))
	}
}

func TestMailProcess(t *testing.T) {
	// It should not panic
	sh := WebShell{PIN: "abcfoobar"}
	sh.mailProcess(`From howard@localhost.localdomain  Sat Mar  5 12:17:27 2016
To: howard@localhost.localdomain
Subject: subject does not matter
From: howard@localhost.localdomain (Howard Guo)

abcfoobar  echo "'hello world'" | grep hello > /proc/self/fd/2`)
}

func TestWAExtractResponse(t *testing.T) {
	input := `<?xml version='1.0' encoding='UTF-8'?>
<queryresult success='true'
    error='false'
    numpods='6'
    datatypes='Weather'
    timedout='Data,Character'
    timedoutpods=''
    timing='7.258'
    parsetiming='0.14'
    parsetimedout='false'
    recalculate='http://www3.wolframalpha.com/api/v2/recalc.jsp?id=MSPa5161cf6a34b84i1870c00006aif8bg164886379&amp;s=34'
    id='MSPa5171cf6a34b84i1870c00002436dcaeaa7734c2'
    host='http://www3.wolframalpha.com'
    server='34'
    related='http://www3.wolframalpha.com/api/v2/relatedQueries.jsp?id=MSPa5181cf6a34b84i1870c00001bg9f0545940f532&amp;s=34'
    version='2.6'
    profile='EnterDoQuery:0.,StartWrap:7.25811'>
 <pod title='Input interpretation'
     scanner='Identity'
     id='Input'
     position='100'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5191cf6a34b84i1870c00002cb1121i5a50518e?MSPStoreType=image/gif&amp;s=34'
       alt='weather | Nuremberg, Germany'
       title='weather | Nuremberg, Germany'
       width='256'
       height='32' />
   <plaintext>weather | Nuremberg, Germany</plaintext>
  </subpod>
 </pod>
 <pod title='Latest recorded weather for Nuremberg, Germany'
     scanner='Data'
     id='InstantaneousWeather:WeatherData'
     position='200'
     error='false'
     numsubpods='1'
     primary='true'>
  <subpod title=''>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5201cf6a34b84i1870c000060i6630e77b0ccd3?MSPStoreType=image/gif&amp;s=34'
       alt='temperature | 9 °C
conditions | clear
relative humidity | 57%  (dew point: 1 °C)
wind speed | 0.5 m/s
(42 minutes ago)'
       title='temperature | 9 °C
conditions | clear
relative humidity | 57%  (dew point: 1 °C)
wind speed | 0.5 m/s
(42 minutes ago)'
       width='304'
       height='153' />
   <plaintext>temperature | 9 °C
conditions | clear
relative humidity | 57%  (dew point: 1 °C)
wind speed | 0.5 m/s
(42 minutes ago)</plaintext>
  </subpod>
  <states count='2'>
   <state name='Show non-metric'
       input='InstantaneousWeather:WeatherData__Show non-metric' />
   <state name='More'
       input='InstantaneousWeather:WeatherData__More' />
  </states>
  <infos count='1'>
   <info>
    <units count='1'>
     <unit short='m/s'
         long='meters per second' />
     <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5211cf6a34b84i1870c00002ddgceh366df6i5a?MSPStoreType=image/gif&amp;s=34'
         width='166'
         height='26' />
    </units>
   </info>
  </infos>
 </pod>
 <pod title='Weather forecast for Nuremberg, Germany'
     scanner='Data'
     id='WeatherForecast:WeatherData'
     position='300'
     error='false'
     numsubpods='2'
     primary='true'>
  <subpod title='Tonight'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5221cf6a34b84i1870c000047a2ae5781h4ef08?MSPStoreType=image/gif&amp;s=34'
       alt='between 3 °C and 7 °C
clear (all night)'
       title='between 3 °C and 7 °C
clear (all night)'
       width='180'
       height='60' />
   <plaintext>between 3 °C and 7 °C
clear (all night)</plaintext>
  </subpod>
  <subpod title='Tomorrow'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5231cf6a34b84i1870c00006963303a6493ce54?MSPStoreType=image/gif&amp;s=34'
       alt='between 5 °C and 14 °C
clear (all day)  |  rain (mid-morning to late afternoon)'
       title='between 5 °C and 14 °C
clear (all day)  |  rain (mid-morning to late afternoon)'
       width='391'
       height='64' />
   <plaintext>between 5 °C and 14 °C
clear (all day)  |  rain (mid-morning to late afternoon)</plaintext>
  </subpod>
  <states count='3'>
   <state name='Show non-metric'
       input='WeatherForecast:WeatherData__Show non-metric' />
   <state name='More days'
       input='WeatherForecast:WeatherData__More days' />
   <state name='More details'
       input='WeatherForecast:WeatherData__More details' />
  </states>
 </pod>
 <pod title='Weather history &amp; forecast'
     scanner='Data'
     id='WeatherCharts:WeatherData'
     position='400'
     error='false'
     numsubpods='4'>
  <subpod title='Temperature'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5241cf6a34b84i1870c00003gi90h59haffaib7?MSPStoreType=image/gif&amp;s=34'
       alt='
  |  |   |
low: 3 °C
Sun, Apr 10, 5:00am, ... | average high:  | 14 °C
average low:  | 6 °C | high: 18 °C
Mon, Apr 11, 5:00pm
 |   |  '
       title='
  |  |   |
low: 3 °C
Sun, Apr 10, 5:00am, ... | average high:  | 14 °C
average low:  | 6 °C | high: 18 °C
Mon, Apr 11, 5:00pm
 |   |  '
       width='519'
       height='190' />
   <plaintext>
  |  |   |
low: 3 °C
Sun, Apr 10, 5:00am, ... | average high:  | 14 °C
average low:  | 6 °C | high: 18 °C
Mon, Apr 11, 5:00pm
 |   |  </plaintext>
  </subpod>
  <subpod title='Cloud cover'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5251cf6a34b84i1870c00001f7ff4cd70hebibf?MSPStoreType=image/gif&amp;s=34'
       alt='
 | clear: 79.8% (4.7 days)   |  overcast: 4.2% (6 hours) '
       title='
 | clear: 79.8% (4.7 days)   |  overcast: 4.2% (6 hours) '
       width='519'
       height='118' />
   <plaintext>
 | clear: 79.8% (4.7 days)   |  overcast: 4.2% (6 hours) </plaintext>
  </subpod>
  <subpod title='Conditions'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5261cf6a34b84i1870c00002da7dbc02ff09i6b?MSPStoreType=image/gif&amp;s=34'
       alt='
 | rain: 13.7% (19.5 hours) '
       title='
 | rain: 13.7% (19.5 hours) '
       width='519'
       height='87' />
   <plaintext>
 | rain: 13.7% (19.5 hours) </plaintext>
  </subpod>
  <subpod title='Precipitation rate'>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5271cf6a34b84i1870c00004b75ahed9afb0ei7?MSPStoreType=image/gif&amp;s=34'
       alt='
  |
maximum: 0.01 mm/h
Sun, Apr 10, 2:00pm
'
       title='
  |
maximum: 0.01 mm/h
Sun, Apr 10, 2:00pm
'
       width='519'
       height='156' />
   <plaintext>
  |
maximum: 0.01 mm/h
Sun, Apr 10, 2:00pm
</plaintext>
  </subpod>
  <states count='3'>
   <statelist count='9'
       value='Current week'
       delimiters=''>
    <state name='Current week'
        input='WeatherCharts:WeatherData__Current week' />
    <state name='Current day'
        input='WeatherCharts:WeatherData__Current day' />
    <state name='Next week'
        input='WeatherCharts:WeatherData__Next week' />
    <state name='Past week'
        input='WeatherCharts:WeatherData__Past week' />
    <state name='Past month'
        input='WeatherCharts:WeatherData__Past month' />
    <state name='Past year'
        input='WeatherCharts:WeatherData__Past year' />
    <state name='Past 5 years'
        input='WeatherCharts:WeatherData__Past 5 years' />
    <state name='Past 10 years'
        input='WeatherCharts:WeatherData__Past 10 years' />
    <state name='All'
        input='WeatherCharts:WeatherData__All' />
   </statelist>
   <state name='Show non-metric'
       input='WeatherCharts:WeatherData__Show non-metric' />
   <state name='More'
       input='WeatherCharts:WeatherData__More' />
  </states>
 </pod>
 <pod title='Historical temperatures for April 8'
     scanner='Data'
     id='HistoricalTemperature:WeatherData'
     position='500'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5281cf6a34b84i1870c00004i6f39ibhd81b098?MSPStoreType=image/gif&amp;s=34'
       alt='
low: -10 °C
2003 | average high:  | 12 °C
average low:  | 1 °C | high: 21 °C
1986
(daily ranges, not corrected for changes in local weather station environment)'
       title='
low: -10 °C
2003 | average high:  | 12 °C
average low:  | 1 °C | high: 21 °C
1986
(daily ranges, not corrected for changes in local weather station environment)'
       width='519'
       height='161' />
   <plaintext>
low: -10 °C
2003 | average high:  | 12 °C
average low:  | 1 °C | high: 21 °C
1986
(daily ranges, not corrected for changes in local weather station environment)</plaintext>
  </subpod>
  <states count='2'>
   <state name='Show table'
       input='HistoricalTemperature:WeatherData__Show table' />
   <state name='Show non-metric'
       input='HistoricalTemperature:WeatherData__Show non-metric' />
  </states>
 </pod>
 <pod title='Weather station information'
     scanner='Data'
     id='WeatherStationInformation:WeatherData'
     position='600'
     error='false'
     numsubpods='1'>
  <subpod title=''>
   <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5291cf6a34b84i1870c00004fhgi340c08c233e?MSPStoreType=image/gif&amp;s=34'
       alt='name | EDDN  (Nürnberg Airport)
relative position | 6 km N  (from center of Nuremberg)
relative elevation | (comparable to center of Nuremberg)
local time | 9:02:24 pm CEST  |  Friday, April 8, 2016
local sunlight | sun is below the horizon
azimuth: 295°  (WNW)  |  altitude: -10°'
       title='name | EDDN  (Nürnberg Airport)
relative position | 6 km N  (from center of Nuremberg)
relative elevation | (comparable to center of Nuremberg)
local time | 9:02:24 pm CEST  |  Friday, April 8, 2016
local sunlight | sun is below the horizon
azimuth: 295°  (WNW)  |  altitude: -10°'
       width='435'
       height='185' />
   <plaintext>name | EDDN  (Nürnberg Airport)
relative position | 6 km N  (from center of Nuremberg)
relative elevation | (comparable to center of Nuremberg)
local time | 9:02:24 pm CEST  |  Friday, April 8, 2016
local sunlight | sun is below the horizon
azimuth: 295°  (WNW)  |  altitude: -10°</plaintext>
  </subpod>
  <states count='2'>
   <state name='Show non-metric'
       input='WeatherStationInformation:WeatherData__Show non-metric' />
   <state name='More'
       input='WeatherStationInformation:WeatherData__More' />
  </states>
  <infos count='2'>
   <info>
    <units count='1'>
     <unit short='km'
         long='kilometers' />
     <img src='http://www3.wolframalpha.com/Calculate/MSP/MSP5301cf6a34b84i1870c00003ec8cc16181cg7id?MSPStoreType=image/gif&amp;s=34'
         width='118'
         height='26' />
    </units>
   </info>
   <info>
    <link url='http://maps.google.com?ie=UTF8&amp;z=12&amp;t=k&amp;ll=49.503%2C11.055&amp;q=49.503%20N%2C%2011.055%20E'
        text='Satellite image' />
   </info>
  </infos>
 </pod>
 <sources count='5'>
  <source url='http://www.wolframalpha.com/sources/AirportDataSourceInformationNotes.html'
      text='Airport data' />
  <source url='http://www.wolframalpha.com/sources/AstronomicalDataSourceInformationNotes.html'
      text='Astronomical data' />
  <source url='http://www.wolframalpha.com/sources/CityDataSourceInformationNotes.html'
      text='City data' />
  <source url='http://www.wolframalpha.com/sources/WeatherDataSourceInformationNotes.html'
      text='Weather data' />
  <source url='http://www.wolframalpha.com/sources/WeatherForecastDataSourceInformationNotes.html'
      text='Weather forecast data' />
 </sources>
</queryresult>`
	sh := WebShell{PIN: "abcfoobar"}
	txtInfo := sh.waExtractResponse([]byte(input))
	if txtInfo != `weather | Nuremberg, Germany;temperature | 9 °C
conditions | clear
relative humidity | 57%  (dew point: 1 °C)
wind speed | 0.5 m/s
(42 minutes ago);between 3 °C and 7 °C
clear (all night);between 5 °C and 14 °C
clear (all day)  |  rain (mid-morning to late afternoon);|  |   |
low: 3 °C
Sun, Apr 10, 5:00am, ... | average high:  | 14 °C
average low:  | 6 °C | high: 18 °C
Mon, Apr 11, 5:00pm
 |   |;| clear: 79.8% (4.7 days)   |  overcast: 4.2% (6 hours);| rain: 13.7% (19.5 hours);|
maximum: 0.01 mm/h
Sun, Apr 10, 2:00pm;low: -10 °C
2003 | average high:  | 12 °C
average low:  | 1 °C | high: 21 °C
1986
(daily ranges, not corrected for changes in local weather station environment);name | EDDN  (Nürnberg Airport)
relative position | 6 km N  (from center of Nuremberg)
relative elevation | (comparable to center of Nuremberg)
local time | 9:02:24 pm CEST  |  Friday, April 8, 2016
local sunlight | sun is below the horizon
azimuth: 295°  (WNW)  |  altitude: -10°;` {
		t.Fatal(txtInfo)
	}
}

func TestVoiceDecodeDTMF(t *testing.T) {
	if s := voiceDecodeDTMF(""); s != "" {
		t.Fatal(s)
	}
	if s := voiceDecodeDTMF("02033004440009999"); s != " ae i  z" {
		t.Fatal(s)
	}
	if s := voiceDecodeDTMF("020330044400099990000"); s != " ae i  z   " {
		t.Fatal(s)
	}
	if s := voiceDecodeDTMF("20*330*4440**9999"); s != "aEiz" {
		t.Fatal(s)
	}
	if s := voiceDecodeDTMF("2010*220*1102220120*3013"); s != "a0B1c2D3" {
		t.Fatal(s)
	}
	goodMsg := "1234567890!@#$%^&*(`~)-_=+[{]}\\|;:'\",<.>/?abcABC"
	if s := voiceDecodeDTMF(`
	11012013014015016017018019010
	111011201130114011501160117011801190
	121012201230124012501260127012801290
	131013201330134013501360137013801390
	14101420143014401450
	202202220
	*202202220*
	`); s != goodMsg {
		t.Fatal(s)
	}
	// Some unusual typing techniques
	if s := voiceDecodeDTMF(`2*2*22*222*`); s != "aAbC" {
		t.Fatal(s)
	}
}
