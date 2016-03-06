package main

import (
	"errors"
	"log"
	"testing"
)

func TestIsEmailNotificationEnabled(t *testing.T) {
	sh := WebShell{}
	if sh.isEmailNotificationEnabled() {
		t.Fatal()
	}
	sh.MailRecipients = []string{"dummy"}
	if sh.isEmailNotificationEnabled() {
		t.Fatal()
	}
	sh.MailFrom = "abc"
	if sh.isEmailNotificationEnabled() {
		t.Fatal()
	}
	sh.MailAgentAddressPort = "example.com:25"
	if !sh.isEmailNotificationEnabled() {
		t.Fatal()
	}
}

func TestLogStatementAndNotify(t *testing.T) {
	sh := WebShell{
		MailRecipients:       []string{"dummy"},
		MailFrom:             "abc",
		MailAgentAddressPort: "example.com:25"}
	log.Print("========= Observe one log message ========")
	sh.logStatementAndNotify("stmt", "output")
	log.Print("===========================================")
	// the background goroutine should spit another log message, but it won't happen unless we wait.
}

func TestTrimShellOutput(t *testing.T) {
	sh := WebShell{TruncateOutputLen: 0}
	if out := sh.trimShellOutput(nil, "0123456789"); out != "" {
		t.Fatal(out)
	}
	sh = WebShell{TruncateOutputLen: 10}
	if out := sh.trimShellOutput(nil, "0123456789"); out != "0123456789" {
		t.Fatal(out)
	}
	if out := sh.trimShellOutput(nil, "0123456789abc"); out != "0123456789" {
		t.Fatal(out)
	}
	if out := sh.trimShellOutput(errors.New("012345678"), "9"); out != "012345678" {
		t.Fatal(out)
	}
	if out := sh.trimShellOutput(errors.New("01234567"), "8"); out != "01234567 8" {
		t.Fatal(out)
	}
}

func TestRunShellStatement(t *testing.T) {
	sh := WebShell{TruncateOutputLen: 30, ExecutionTimeoutSec: 1, SubHashSlashForPipe: false}
	if out := sh.runShellStatement("echo a | grep a #/thisiscomment"); out != "a" {
		t.Fatal(out)
	}
	if out := sh.runShellStatement("echo a && false # this is comment"); out != "exit status 1 a" {
		t.Fatal(out)
	}
	if out := sh.runShellStatement("echo -e 'a\nb' > /proc/self/fd/1"); out != "a\nb" {
		t.Fatal(out)
	}
	if out := sh.runShellStatement(`echo '"abc"' > /proc/self/fd/2`); out != `"abc"` {
		t.Fatal(out)
	}
	if out := sh.runShellStatement(`echo "'abc'"`); out != `'abc'` {
		t.Fatal(out)
	}
	sh = WebShell{TruncateOutputLen: 16, ExecutionTimeoutSec: 1, SubHashSlashForPipe: false}
	if out := sh.runShellStatement(`sleep 2`); out != "exit status 143" {
		t.Fatal(out)
	}
	if out := sh.runShellStatement(`echo 01234567891234567`); out != "0123456789123456" {
		t.Fatal(out)
	}
	sh = WebShell{TruncateOutputLen: 30, ExecutionTimeoutSec: 1, SubHashSlashForPipe: false}
	if out := sh.runShellStatement("echo a #/ grep a"); out != "a" {
		t.Fatal(out)
	}
	if out := sh.runShellStatement("echo a && false # this is comment"); out != "exit status 1 a" {
		t.Fatal(out)
	}
	if out := sh.runShellStatement("echo -e 'a\nb' > /proc/self/fd/1"); out != "a\nb" {
		t.Fatal(out)
	}
	if out := sh.runShellStatement(`echo '"abc"' > /proc/self/fd/2`); out != `"abc"` {
		t.Fatal(out)
	}
	if out := sh.runShellStatement(`echo "'abc'"`); out != `'abc'` {
		t.Fatal(out)
	}
}

func TestMatchPresetOrPIN(t *testing.T) {
	sh := WebShell{}
	if stmt := sh.matchPresetOrPIN("echo hi"); stmt != "" {
		t.Fatal(stmt)
	}
	sh = WebShell{PresetMessages: map[string]string{"": "echo hi"}}
	if stmt := sh.matchPresetOrPIN("echo hi"); stmt != "" {
		t.Fatal(stmt)
	}
	sh = WebShell{PIN: "abc123"}
	if stmt := sh.matchPresetOrPIN("badpinhello world"); stmt != "" {
		t.Fatal(stmt)
	}
	if stmt := sh.matchPresetOrPIN("abc123hello world"); stmt != "hello world" {
		t.Fatal(stmt)
	}
	if stmt := sh.matchPresetOrPIN("   abc123    hello world   \r\n\t  "); stmt != "hello world" {
		t.Fatal(stmt)
	}
	sh = WebShell{PIN: "nopin", PresetMessages: map[string]string{"abc": "123", "def": "456"}}
	if stmt := sh.matchPresetOrPIN("badbadbad"); stmt != "" {
		t.Fatal(stmt)
	}
	if stmt := sh.matchPresetOrPIN("   abcfoobar  "); stmt != "123" {
		t.Fatal(stmt)
	}
	if stmt := sh.matchPresetOrPIN("   deffoobar  "); stmt != "456" {
		t.Fatal(stmt)
	}
}

func TestFindReplyAddressInMail(t *testing.T) {
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
	if subj, contentType, addr := findSubjectAndReplyAddressInMail("foobar"); subj != "" || contentType != "" || addr != "" {
		t.Fatal(subj, addr)
	}
	if subj, contentType, addr := findSubjectAndReplyAddressInMail(example1); subj != "message from houzuo guo" || contentType != `multipart/alternative; boundary="----------=_1457167900-29001-525"` || addr != "no.reply@example.com" {
		t.Fatal(subj, addr)
	}
	if subj, contentType, addr := findSubjectAndReplyAddressInMail(example2); subj != "hi" || contentType != `text/plain; charset=us-ascii` || addr != "me@example.com" {
		t.Fatal(subj, addr)
	}
}

func TestRunShellStatementInMail(t *testing.T) {
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
	sh := WebShell{PIN: "abcfoobar", TruncateOutputLen: 6, ExecutionTimeoutSec: 1}
	if stmt, out := sh.runShellStatementInEmail("subject does not matter", "text/plain; charset=us-ascii", example); stmt != `echo "'hello world'" | grep hello > /proc/self/fd/2` && out != "'hello" {
		t.Fatal(stmt, out)
	}
}

func TestRunShellStatementInMIMEMail(t *testing.T) {
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
	sh := WebShell{PIN: "abc123", TruncateOutputLen: 200, ExecutionTimeoutSec: 1}
	subject, contentType, replyTo := findSubjectAndReplyAddressInMail(example)
	if subject != "message from houzuo guo" || contentType != `multipart/alternative; boundary="----------=_1457209616-22400-170"` || replyTo != "no.reply@example.com" {
		t.Fatal(subject, contentType, replyTo)
	}
	outputMatch := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbcccccccccccccccccccccccccccccdddddddddddddddddddddddddeeeeeeeeeeeeee"
	stmtMatch := "echo " + outputMatch
	if stmt, out := sh.runShellStatementInEmail("subject does not matter", contentType, example); stmt != stmtMatch || out != outputMatch {
		t.Fatal(stmt, out)
	}
}

func TestMysteriousRequest(t *testing.T) {
	t.Skip()
	sh := WebShell{
		MysteriousURL:   "https://a",
		MysteriousAddr1: "@example.com",
		MysteriousAddr2: "me@example.com",
		MysteriousID1:   "123456",
		MysteriousID2:   "foo-bar",
	}
	sh.doMysteriousHTTPRequest("hi there")
}

func TestProcessingMail(t *testing.T) {
	// It should not panic
	sh := WebShell{PIN: "abcfoobar"}
	sh.processMail(`From howard@localhost.localdomain  Sat Mar  5 12:17:27 2016
To: howard@localhost.localdomain
Subject: subject does not matter
From: howard@localhost.localdomain (Howard Guo)

abcfoobar  echo "'hello world'" | grep hello > /proc/self/fd/2`)
}
