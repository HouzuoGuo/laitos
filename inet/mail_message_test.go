package inet

import (
	"errors"
	"reflect"
	"testing"
)

var MultipartMail = []byte(`From bounces+21dd7b-root=houzuo.net@sendgrid.net  Sat Mar  5 19:4d 2016
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
Reply-To: me@example.com
Message-ID: <pwg@ismd1.sendgrid.net>
Content-type: multipart/alternative; boundary="----------=_1457209616-22400-170"
X-SG-EID: QmpGM/Hvmhkk3PuFy9pSYLhAJSXKeY+t4JcfPQksuRn
 E=
Status: RO

This is a multi-part message in MIME format...

------------=_1457209616-22400-170
Content-Transfer-Encoding: quoted-printable
Content-Type: text/plain; charset=UTF-8

View profile: https://www.linkedin.com/comm/profile/view?id=3DAAsAAAAfpNwBt=
yG6EOXBUNrQCHJlntHZ7tt8ooM&authType=3Dname&authToken=3DFPKm&midToken=3DAQER=

------------=_1457209616-22400-170
Content-Type: text/html; charset="UTF-8"
Content-Disposition: inline
Content-Transfer-Encoding: quoted-printable

<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.=
w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd"> <html xmlns=3D"http://www.w3=
.org/1999/xhtml" lang=3D"en" xml:lang=3D"en"> <head> <meta http-equiv=3D"Co=

------------=_1457209616-22400-170--`)

var TextMail = []byte(`From howard@localhost.localdomain  Sat Mar  5 12:17:27 2016
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

`)

func TestReadMessage(t *testing.T) {
	prop, msg, err := ReadMailMessage(TextMail)
	if err != nil || msg == nil {
		t.Fatal(err, msg)
	}
	if !reflect.DeepEqual(prop, BasicMail{
		ContentType:  "text/plain; charset=us-ascii",
		ReplyAddress: "howard@localhost.localdomain",
		FromAddress:  "howard@localhost.localdomain",
		Subject:      "subject1",
	}) {
		t.Fatalf("%+v\n", prop)
	}

	prop, msg, err = ReadMailMessage(MultipartMail)
	if err != nil || msg == nil {
		t.Fatal(err, msg)
	}
	if !reflect.DeepEqual(prop, BasicMail{
		ContentType:  `multipart/alternative; boundary="----------=_1457209616-22400-170"`,
		ReplyAddress: "me@example.com",
		FromAddress:  "no.reply@example.com",
		Subject:      "message from Houzuo Guo",
	}) {
		t.Fatalf("%+v\n", prop)
	}
}

func TestWalkMessage(t *testing.T) {
	// Do nothing and stop walking
	partsWalked := 0
	err := WalkMailMessage(MultipartMail, func(prop BasicMail, body []byte) (next bool, err error) {
		partsWalked++
		return false, nil
	})
	if err != nil || partsWalked != 1 {
		t.Fatal(err, partsWalked)
	}
	// Return error to stop walking
	partsWalked = 0
	err = WalkMailMessage(MultipartMail, func(prop BasicMail, body []byte) (next bool, err error) {
		partsWalked++
		return false, errors.New("hi")
	})
	if err == nil || err.Error() != "hi" || partsWalked != 1 {
		t.Fatal(err, partsWalked)
	}
	// Walk both parts and do nothing
	partsWalked = 0
	err = WalkMailMessage(MultipartMail, func(prop BasicMail, body []byte) (next bool, err error) {
		partsWalked++
		return true, nil
	})
	if err != nil || partsWalked != 2 {
		t.Fatal(err, partsWalked)
	}

	// Walk text message
	matchTextMessage := `abcfoobar  echo "'hello world'" | grep hello > /proc/self/fd/2

`
	partsWalked = 0
	err = WalkMailMessage(TextMail, func(prop BasicMail, body []byte) (next bool, err error) {
		if string(body) != matchTextMessage {
			t.Fatal(string(body))
		}
		partsWalked++
		return true, nil
	})
	if err != nil || partsWalked != 1 {
		t.Fatal(err, partsWalked)
	}

	// Walk multipart message
	matchParts := []string{
		`View profile: https://www.linkedin.com/comm/profile/view?id=AAsAAAAfpNwBtyG6EOXBUNrQCHJlntHZ7tt8ooM&authType=name&authToken=FPKm&midToken=AQER`,
		`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd"> <html xmlns="http://www.w3.org/1999/xhtml" lang="en" xml:lang="en"> <head> <meta http-equiv="Co`,
	}
	matchContentTypes := []string{
		`text/plain; charset=UTF-8`,
		`text/html; charset="UTF-8"`,
	}
	partsWalked = 0
	err = WalkMailMessage(MultipartMail, func(prop BasicMail, body []byte) (next bool, err error) {
		if string(body) != matchParts[partsWalked] {
			t.Fatal(string(body), matchParts[partsWalked])
		}
		if prop.ContentType != matchContentTypes[partsWalked] {
			t.Fatal(prop.ContentType, matchContentTypes[partsWalked])
		}
		partsWalked++
		return true, nil
	})
}
