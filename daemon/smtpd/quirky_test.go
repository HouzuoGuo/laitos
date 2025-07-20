package smtpd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetHeader(t *testing.T) {
	example := "Date: Sun, 24 May 2020 08:48:54 +0000 (UTC)\nFrom: Webhooks via IFTTT <action@ifttt.com>\nReply-To: Do not reply <no-reply@ifttt.com>\nMessage-ID: <5eca34f1ec12d_2e8a6b3143706f@ip-172-31-1-54.ec2.internal.mail>\n"
	got := SetHeader(example, "From", "new@sender.com")
	want := "Date: Sun, 24 May 2020 08:48:54 +0000 (UTC)\nFrom: new@sender.com\nReply-To: Do not reply <no-reply@ifttt.com>\nMessage-ID: <5eca34f1ec12d_2e8a6b3143706f@ip-172-31-1-54.ec2.internal.mail>\n"
	require.Equal(t, want, got)

	got = SetHeader(got, "New-Header", "val")
	want = "New-Header: val\nDate: Sun, 24 May 2020 08:48:54 +0000 (UTC)\nFrom: new@sender.com\nReply-To: Do not reply <no-reply@ifttt.com>\nMessage-ID: <5eca34f1ec12d_2e8a6b3143706f@ip-172-31-1-54.ec2.internal.mail>\n"
	require.Equal(t, want, got)
}

func TestGetHeader(t *testing.T) {
	example := "Date: Sun, 24 May 2020 08:48:54 +0000 (UTC)\r\nFrom: Webhooks via IFTTT <action@ifttt.com>\r\nReply-To: Do not reply <no-reply@ifttt.com>\r\nMessage-ID: <5eca34f1ec12d_2e8a6b3143706f@ip-172-31-1-54.ec2.internal.mail>\r\nA: 1\r\n"
	require.Equal(t, "Webhooks via IFTTT <action@ifttt.com>", GetHeader(example, "from"))
	require.Equal(t, "", GetHeader(example, "nonexistent"))
	require.Equal(t, "1", GetHeader(example, "A"))
}
