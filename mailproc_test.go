package main

import "testing"

var localMailer = Mailer{
	Recipients:     []string{"root@localhost"},
	MTAAddressPort: "localhost:25",
	MailFrom:       "root@localhost",
}

func TestGetMultipartMailProperties(t *testing.T) {
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
	if subj, contentType, addr := GetMailProperties("foobar"); subj != "" || contentType != "" || addr != "" {
		t.Fatal(subj, contentType, addr)
	}
	if subj, contentType, addr := GetMailProperties(example1); subj != "message from houzuo guo" ||
		contentType != `multipart/alternative; boundary="----------=_1457167900-29001-525"` ||
		addr != "no.reply@example.com" {
		t.Fatal(subj, contentType, addr)
	}
}

func TestGetSinglepartMailProperties(t *testing.T) {
	example := `From bounces+2565887-39aa-root=ccc.ddd@aaa.bbb.com  Sat Dec 24 05:59:29 2016
X-Original-To: root@ccc.ddd
Delivered-To: root@ccc.ddd
DKIM-Signature: v=1; a=rsa-sha1; c=relaxed; d=bbb.com;
        h=from:mime-version:subject:to:content-type; s=s1;
        bh=gfjwgKXXLe/VOMaaaLodbbbABhM=; b=GuP7GxEHKC55B5bRtkVSxFiI9Sqn9
        Uyay9bS+SOmWzy/MY6mbmmmYmwiBHqFfEV/piKByZvLUL6cHz7ZipjCV1U0xlVXq
        X0Mjccc7zzfqYw=
Date: Sat, 24 Dec 2016 04:59:26 +0000
From: "Houzuo Guo (via abc)" <no.reply.abc@def.ghi>
Mime-Version: 1.0
Subject: message from Houzuo Guo
To: root@ccc.ddd
Content-type: multipart/alternative; boundary="----------=_1482555566-20113-459"
X-SG-EID: ZPWmCwPRM4AlS4dNaaaaaa5ynw1Ls9ipvw/8bs0OcA7OhJjGPg3tvaaa2NTHbiYVLzSGZovXQBbvIS
 IJ4vWBvW3zaDlNypLg6X2jeeYEnddd5mKnlSCsezFgiP+s+bNBY4O1fS5Lx1sJbbbk92gdvO4FkTjd
 QFmoC4WbZfffBFzWp3l/kyJUSheee7kxAuOspCEaN/FHgeP0EsGHUD2s+im6Mqccc2UZJG8IovQJt2
 4VL9ikX4agggMtDknmCQdI

Content-Transfer-Encoding: quoted-printable
Content-Type: text/plain; charset=UTF-8

znaaaaaaxdffff

send a reply to Houzuo Guo:
https://aa.ct.sendgrid.net/wf/click?upn=bb-cc-ff-dd-ee-dd-ee-ff-gg-hh-ii-jj-3k-3k

Houzuo Guo sent this message from:
aaa bbb

Do not reply directly to this message.

This message was sent to you using aaa To learn more, visit https://ccc.ct.sendgrid.net/wf/click?upn=dd-ee-ff-gg-hh-3k-3k`
	if subj, contentType, addr := GetMailProperties(example); subj != "message from houzuo guo" ||
		contentType != `multipart/alternative; boundary="----------=_1482555566-20113-459"` ||
		addr != "no.reply.abc@def.ghi" {
		t.Fatal(subj, contentType, addr)
	}
}

func TestGetTextMailProperties(t *testing.T) {
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
	if subj, contentType, addr := GetMailProperties(example2); subj != "hi" ||
		contentType != `text/plain; charset=us-ascii` ||
		addr != "me@example.com" {
		t.Fatal(subj, addr)
	}
}

func TestMailProcess(t *testing.T) {
	// It should not panic
	run := MailProcessor{CommandRunner: CommandRunner{PIN: "abcfoobar", Mailer: localMailer}}
	run.RunCommandFromMail(`From howard@localhost.localdomain  Sat Mar  5 12:17:27 2016
To: howard@localhost.localdomain
Subject: subject does not matter
From: howard@localhost.localdomain (Howard Guo)

abcfoobar  echo "'hello world'" | grep hello > /proc/self/fd/2`)
}

func TestMailRunCmdPlainText(t *testing.T) {
	example := `From howard@localhost.localdomain  Sat Mar  5 12:17:27 2016
X-Original-To: howard@localhost
Delivered-To: howard@localhost.localdomain
Date: Sat, 05 Mar 2016 12:17:27 +0100
To: howard@localhost.localdomain
Subject: subject1
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
From: howard@localhost.localdomain (Howard Guo)

abcfoobar  echo "'hello world'" | grep hello > /proc/self/fd/2

`
	run := MailProcessor{CommandRunner: CommandRunner{PIN: "abcfoobar", TruncateLen: 6, TimeoutSec: 1, Mailer: localMailer}}
	if stmt, out := run.RunCommandFromMailBody("subject1", "text/plain; charset=us-ascii", example); stmt != `echo "'hello world'" | grep hello > /proc/self/fd/2` && out != "'hello" {
		t.Fatal(stmt, out)
	}
	example = `From howard@localhost.localdomain  Sat Mar  5 12:17:27 2016
X-Original-To: howard@localhost
Delivered-To: howard@localhost.localdomain
Date: Sat, 05 Mar 2016 12:17:27 +0100
To: howard@localhost.localdomain
Subject: subject2
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
From: howard@localhost.localdomain (Howard Guo)

abcfoobar  sleep 2

`
	if stmt, out := run.RunCommandFromMailBody("subject2", "text/plain; charset=us-ascii", example); stmt != `sleep 2` && out != "'exit status 143" {
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
	proc := MailProcessor{CommandRunner: CommandRunner{PIN: "abc123", TruncateLen: 200, TimeoutSec: 1, Mailer: localMailer}}
	subject, contentType, replyTo := GetMailProperties(example)
	if subject != "message from houzuo guo" || contentType != `multipart/alternative; boundary="----------=_1457209616-22400-170"` || replyTo != "no.reply@example.com" {
		t.Fatal(subject, contentType, replyTo)
	}
	outputMatch := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbcccccccccccccccccccccccccccccdddddddddddddddddddddddddeeeeeeeeeeeeee"
	stmtMatch := "echo " + outputMatch
	if stmt, out := proc.RunCommandFromMailBody("subject does not matter", contentType, example); stmt != stmtMatch || out != outputMatch {
		t.Fatal(stmt, out)
	}
}
