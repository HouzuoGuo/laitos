package misc

import (
	"fmt"
)

var (
	AutoUnlockStats     = NewStats()
	CommandStats        = NewStats()
	DNSDStatsTCP        = NewStats()
	DNSDStatsUDP        = NewStats()
	HTTPDStats          = NewStats()
	HTTPProxyStats      = NewStats()
	TCPOverDNSStats     = NewStats()
	PlainSocketStatsTCP = NewStats()
	PlainSocketStatsUDP = NewStats()
	SerialDevicesStats  = NewStats()
	SimpleIPStatsTCP    = NewStats()
	SimpleIPStatsUDP    = NewStats()
	SMTPDStats          = NewStats()
	SNMPStats           = NewStats()
	SOCKDStatsTCP       = NewStats()
	SOCKDStatsUDP       = NewStats()
	TelegramBotStats    = NewStats()

	// OutstandingMailBytes is the total size of all outstanding mails waiting to be delivered.
	OutstandingMailBytes int64
)

// GetLatestStats returns statistic information from all front-end daemons in a piece of multi-line, formatted text.
func GetLatestStats() string {
	numDecimals := 2
	factor := 1000000000.0
	return fmt.Sprintf(`Auto-unlock events        %s
Commands processed        %s
DNS server TCP|UDP        %s | %s
TCP-over-DNS proxy:       %s
HTTP/S server             %s
Plain text server TCP|UDP %s | %s
Serial port devices       %s
Simple IP servers         %s | %s
SMTP server:              %s
SNMP server:              %s
Sock server TCP|UDP:      %s | %s
Telegram commands:        %s
Mail to deliver:          %d KiloBytes
`,
		AutoUnlockStats.Format(factor, numDecimals),
		CommandStats.Format(factor, numDecimals),
		DNSDStatsTCP.Format(factor, numDecimals), DNSDStatsUDP.Format(factor, numDecimals),
		TCPOverDNSStats.Format(factor, numDecimals),
		HTTPDStats.Format(factor, numDecimals),
		PlainSocketStatsTCP.Format(factor, numDecimals), PlainSocketStatsUDP.Format(factor, numDecimals),
		SerialDevicesStats.Format(factor, numDecimals),
		SimpleIPStatsTCP.Format(factor, numDecimals), SimpleIPStatsUDP.Format(factor, numDecimals),
		SMTPDStats.Format(factor, numDecimals),
		SNMPStats.Format(factor, numDecimals),
		SOCKDStatsTCP.Format(factor, numDecimals), SOCKDStatsUDP.Format(factor, numDecimals),
		TelegramBotStats.Format(factor, numDecimals),
		OutstandingMailBytes/1024)
}
