package common

import (
	"fmt"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
)

var (
	AutoUnlockStats     = misc.NewStats()
	CommandStats        = misc.NewStats()
	DNSDStatsTCP        = misc.NewStats()
	DNSDStatsUDP        = misc.NewStats()
	HTTPDStats          = misc.NewStats()
	PlainSocketStatsTCP = misc.NewStats()
	PlainSocketStatsUDP = misc.NewStats()
	SerialDevicesStats  = misc.NewStats()
	SimpleIPStatsTCP    = misc.NewStats()
	SimpleIPStatsUDP    = misc.NewStats()
	SMTPDStats          = misc.NewStats()
	SNMPStats           = misc.NewStats()
	SOCKDStatsTCP       = misc.NewStats()
	SOCKDStatsUDP       = misc.NewStats()
	TelegramBotStats    = misc.NewStats()
)

// GetLatestStats returns statistic information from all front-end daemons in a piece of multi-line, formatted text.
func GetLatestStats() string {
	numDecimals := 2
	factor := 1000000000.0
	return fmt.Sprintf(`Auto-unlock events        %s
Commands processed        %s
DNS server TCP|UDP        %s | %s
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
		HTTPDStats.Format(factor, numDecimals),
		PlainSocketStatsTCP.Format(factor, numDecimals), PlainSocketStatsUDP.Format(factor, numDecimals),
		SerialDevicesStats.Format(factor, numDecimals),
		SimpleIPStatsTCP.Format(factor, numDecimals), SimpleIPStatsUDP.Format(factor, numDecimals),
		SMTPDStats.Format(factor, numDecimals),
		SNMPStats.Format(factor, numDecimals),
		SOCKDStatsTCP.Format(factor, numDecimals), SOCKDStatsUDP.Format(factor, numDecimals),
		TelegramBotStats.Format(factor, numDecimals),
		inet.OutstandingMailBytes/1024)
}
