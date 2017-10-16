package launcher

const (
	// Daemon names:
	DNSDName          = "dnsd"
	HTTPDName         = "httpd"
	InsecureHTTPDName = "insecurehttpd"
	MaintenanceName   = "maintenance"
	PlainSocketName   = "plainsocket"
	SMTPDName         = "smtpd"
	SOCKDName         = "sockd"
	TelegramName      = "telegram"
)

// SheddingSequence is the sequence of daemon names to be taken offline one after another in case of program crash.
var SheddingSequence = []string{MaintenanceName, DNSDName, SOCKDName, SMTPDName, HTTPDName, InsecureHTTPDName, TelegramName, PlainSocketName}

/*
Supervisor manages the lifecycle of laitos main program that runs daemons. In case of main program crash, the supervisor
relaunches the main program using reduced number of daemons, thus ensuring maximum availability of healthy daemons.
*/
type Supervisor struct {
	CLIArgs []string // CLIArgs are the thorough list of program arguments to be used to launch main program
}
