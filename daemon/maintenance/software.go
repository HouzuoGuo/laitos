package maintenance

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HouzuoGuo/laitos/platform"
)

/*
suppressOutputMarkers is a list of strings containing text phrases that indicate system package manager has not modified
the system, for example, when a package to install is not found, or system is already up to date.
*/
var suppressOutputMarkers = []string{
	"no package", "nothing to do", "not found", "0 to upgrade, 0 to newly install",
	"0 upgraded, 0 newly installed", "unable to locate", "already installed", "is the latest",
	"unable to find", "no match",
}

/*
pkgAlreadyInstalledMarkers is a list of strings containing text phrases that indicate package manager has succeeded in
installing/updating a package even though no action is taken, for example, when a package to install is already up to date.
*/
var pkgAlreadyInstalledMarkers = []string{"already the newest", "already installed", "no packages marked for update"}

func (daemon *Daemon) prepareDockerRepositoryForAWSLinux(out *bytes.Buffer) {
	if platform.HostIsWindows() {
		daemon.logPrintStageStep(out, "skipped on windows: prepare docker repository for AWS Linux system")
		return
	}
	daemon.logPrintStageStep(out, "prepare docker repository for AWS Linux system")
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		daemon.logPrintStageStep(out, "failed to read os-release, skip rest of the stage.")
		return
	} else if !strings.Contains(strings.ToLower(string(content)), "amazon") {
		daemon.logPrintStageStep(out, "system is not an Amazon Linux, skip rest of the stage.")
		return
	}
	installOut, err := platform.InvokeProgram(nil, platform.CommonOSCmdTimeoutSec, "/usr/bin/amazon-linux-extras", "install", "-y", "docker")
	if strings.Contains(installOut, "already installed") && err == nil {
		daemon.logPrintStageStep(out, "install docker via extras - ok")
	} else {
		daemon.logPrintStageStep(out, "install docker via extras - %v %s", err, installOut)
	}
}

/*
getSystemPackageManager returns executable path and name of package manager available on this system, as well as
environment variables and command arguments used to invoke them.
*/
func getSystemPackageManager() (pkgManagerPath, pkgManagerName string, pkgManagerEnv, pkgInstallArgs, sysUpgradeArgs []string) {
	if platform.HostIsWindows() {
		pkgManagerPath = filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\WindowsApps\winget.exe`)
		pkgManagerName = "winget"
	} else {
		for _, binPrefix := range []string{"/sbin", "/bin", "/usr/sbin", "/usr/bin", "/usr/sbin/local", "/usr/bin/local"} {
			/*
				Prefer zypper over apt-get because opensuse has a weird "apt-get wrapper" that is not remotely functional.
				Prefer apt over apt-get because some public cloud OS templates can upgrade kernel via apt but not with apt-get.
			*/
			for _, execName := range []string{"dnf", "yum", "zypper", "apt", "apt-get"} {
				pkgManagerPath = filepath.Join(binPrefix, execName)
				if _, err := os.Stat(pkgManagerPath); err == nil {
					pkgManagerName = execName
					break
				}
			}
			if pkgManagerName != "" {
				break
			}
		}
	}
	switch pkgManagerName {
	case "dnf":
		// dnf and yum are simple and easy
		fallthrough
	case "yum":
		pkgInstallArgs = []string{"-y", "--skip-broken", "install"}
		sysUpgradeArgs = []string{"-y", "--skip-broken", "update"}
	case "apt":
		// apt and apt-get are too old to be convenient
		fallthrough
	case "apt-get":
		pkgManagerEnv = []string{"DEBIAN_FRONTEND=noninteractive"}
		pkgInstallArgs = []string{"-q", "-y", "-f", "-m", "-o", "Dpkg::Options::=--force-confold", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-overwrite", "install"}
		sysUpgradeArgs = []string{"-q", "-y", "-f", "-m", "-o", "Dpkg::Options::=--force-confold", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-overwrite", "--with-new-pkgs", "upgrade"}
	case "winget":
		// winget is too young to be convenient.
		// The fuzzy package name match is too eager to pick up odd apps from the store, hence restrict the source to winget for the time being.
		pkgInstallArgs = []string{"install", "--source", "winget", "--scope", "machine", "--silent", "--accept-package-agreements", "--disable-interactivity", "--accept-source-agreements"}
		sysUpgradeArgs = []string{"upgrade", "--all", "--silent", "--accept-source-agreements", "--include-unknown", "--disable-interactivity"}
	case "zypper":
		// zypper cannot English and consistency
		pkgInstallArgs = []string{"--non-interactive", "install", "--recommends", "--auto-agree-with-licenses", "--replacefiles", "--force-resolution"}
		sysUpgradeArgs = []string{"--non-interactive", "update", "--recommends", "--auto-agree-with-licenses", "--skip-interactive", "--replacefiles", "--force-resolution"}
	}
	return
}

/*
InstallSoftware uses system package manager to upgrade system software, and then install a laitos soft dependencies
along with additional software packages according to user configuration.
*/
func (daemon *Daemon) InstallSoftware(out *bytes.Buffer) {
	// Null input suppresses this action, empty input leads to only laitos recommendations to be installed.
	if daemon.InstallPackages == nil {
		return
	}

	// Prepare package manager
	if platform.HostIsWindows() {
		daemon.logPrintStageStep(out, "install windows features")
		shellOut, err := platform.InvokeShell(3600, platform.PowerShellInterpreterPath, `Install-WindowsFeature XPS-Viewer, WoW64-Support, Windows-TIFF-IFilter, PowerShell-ISE, Windows-Defender, TFTP-Client, Telnet-Client, Server-Media-Foundation, GPMC, NET-Framework-45-Core, WebDAV-Redirector`)
		if err != nil {
			daemon.logPrintStageStep(out, "failed to install windows features: %v - %s", err, shellOut)
		}
		daemon.logPrintStageStep(out, "install/upgrade winget")
		shellOut, err = platform.InvokeShell(3600, platform.PowerShellInterpreterPath, `Add-AppxPackage -RegisterByFamilyName -MainPackage Microsoft.DesktopAppInstaller_8wekyb3d8bbwe`)
		if err != nil {
			daemon.logPrintStageStep(out, "failed to install/upgrade winget: %v - %s", err, shellOut)
		}
	} else {
		if os.Getuid() != 0 {
			daemon.logPrintStageStep(out, "skipped because laitos is not running as uid 0 root")
			return
		}
		daemon.prepareDockerRepositoryForAWSLinux(out)
	}

	daemon.logPrintStage(out, "upgrade system software")
	pkgManagerPath, pkgManagerName, pkgManagerEnv, pkgInstallArgs, sysUpgradeArgs := getSystemPackageManager()
	if pkgManagerName == "" {
		daemon.logPrintStageStep(out, "failed to find a compatible package manager")
		return
	}

	// apt-get is too old to be convenient
	if pkgManagerName == "apt-get" || pkgManagerName == "apt" {
		// Five minutes should be enough to grab the latest manifest
		result, err := platform.InvokeProgram(pkgManagerEnv, 5*60, pkgManagerPath, "update")
		// There is no need to suppress this output according to markers
		daemon.logPrintStageStep(out, "update apt manifests: %v - %s", err, strings.TrimSpace(result))
		// Repair interrupted package installation, otherwise no package will upgrade/install in the next steps.
		daemon.logPrintStageStep(out, "repair interrupted package installation if there is any")
		result, err = platform.InvokeProgram(pkgManagerEnv, 2*3600, "dpkg", "--configure", "-a", "--force-confold", "--force-confdef")
		daemon.logPrintStageStep(out, "repaired package installation interruption: %v - %s", err, strings.TrimSpace(result))
	}

	// Upgrade system packages with a time constraint of two hours
	result, err := platform.InvokeProgram(pkgManagerEnv, 2*3600, pkgManagerPath, sysUpgradeArgs...)
	for _, marker := range suppressOutputMarkers {
		// If nothing was done during system update, suppress the rather useless output.
		if strings.Contains(strings.ToLower(result), marker) {
			result = "(nothing to do or upgrade not available)"
			break
		}
	}
	daemon.logPrintStageStep(out, "upgrade system result: (err? %v) %s", err, strings.TrimSpace(result))

	// Install system diagnosis utilities.
	daemon.logPrintStage(out, "install software")
	allPackageNames := []string{
		// For outgoing HTTPS connections made by laitos
		"ca-certificates",

		// For maintenance of software repositories using APT package manager
		"apt-transport-https", "gnupg", "lsb-release", "software-properties-common",

		// For the maintenance of laitos container image.
		"docker", "docker-client", "docker.io", "docker-ce",

		// Essential libraries for command line-based HTTP clients.
		"bzip2", "bzip2-libs", "gnutls", "lib64z1", "libbz2-1", "libbz2-1.0", "nss", "openssl", "zlib", "zlib1g",

		// Soft and hard dependencies of remote virtual machine
		"qemu", "qemu-common", "qemu-img", "qemu-kvm", "qemu-kvm-common", "qemu-kvm-core",
		"qemu-system", "qemu-system-common", "qemu-system-x86", "qemu-system-x86-core",
		"qemu-user", "qemu-utils",

		// Time maintenance utilities
		"ntp", "ntpd", "ntpdate",

		// busybox and toybox are useful for general maintenance, and busybox can synchronise system clock as well.
		"busybox", "toybox",

		// Network diagnosis and system maintenance utilities. On a typical Linux distribution they use ~300MB of disk space altogether.
		"7zip", "apache2-utils", "bash", "bc", "bind-utils", "binutils", "caca-utils", "ca-certificates-mozilla", "cgroup-tools", "curl", "dateutils", "dialog", "diffutils",
		"dnsutils", "dos2unix", "findutils", "finger", "glibc-locale-source", "gnutls-bin", "gnutls-utils", "hostname", "htop", "iftop", "imlib2", "imlib2-filters",
		"imlib2-loaders", "iotop", "iputils", "iputils-ping", "iputils-tracepath", "jsonlint", "language-pack-en", "lftp", "libcaca0", "libcaca0-plugins",
		"libcgroup-tools", "lm-sensors", "locales", "lrzsz", "lsof", "mailutils", "mailx", "minicom", "miscfiles", "moreutils", "mosh", "nc", "ncurses-term", "netcat", "net-snmp",
		"net-snmp-utils", "net-tools", "nfs-common", "nicstat", "nmap", "nmon", "nping", "p7zip", "patchutils", "pciutils", "perf", "procps", "psmisc", "rsync", "screen",
		"sensors", "shadow", "snmp", "socat", "strace", "sudo", "sysinternals", "sysstat", "tcpdump", "tcptraceroute", "telnet", "tmux", "tracepath", "traceroute", "tree",
		"tshark", "unar", "uniutils", "unzip", "usbutils", "util-linux", "util-linux-locales", "util-linux-user", "vim", "wbritish", "wget", "whois", "wiggle", "yamllint", "zip",
	}
	allPackageNames = append(allPackageNames, daemon.InstallPackages...)
	// Collect unique package names for installation
	uniquePackageNames := make(map[string]struct{})
	for _, name := range allPackageNames {
		uniquePackageNames[name] = struct{}{}
	}
	// Install the packages in the alphabetical order
	allPackageNames = make([]string, 0, len(uniquePackageNames))
	for name := range uniquePackageNames {
		allPackageNames = append(allPackageNames, name)
	}
	sort.Strings(allPackageNames)
	/*
		Although most package managers can install more than one packages at a time, the packages are still installed
		one after another, because:
		- apt-get does not ignore non-existent package names, how inconvenient.
		- if zypper runs into unsatisfactory package dependencies, it aborts the whole installation.
		yum is once again the superior solution among all three.
	*/
	for _, name := range allPackageNames {
		// Put software name next to installation parameters
		installCmd := make([]string, len(pkgInstallArgs)+1)
		copy(installCmd, pkgInstallArgs)
		installCmd[len(pkgInstallArgs)] = name
		// Ten minutes should be good enough for each package
		result, err := platform.InvokeProgram(pkgManagerEnv, 10*60, pkgManagerPath, installCmd...)
		// If installation proceeded successfully (i.e. package exists) but no action taken, inform user about it.
		alreadyInstalled := false
		for _, marker := range pkgAlreadyInstalledMarkers {
			if strings.Contains(strings.ToLower(result), marker) {
				result = "already installed/up-to-date"
				alreadyInstalled = true
				err = nil
				break
			}
		}
		// If nothing can be done about the package (i.e. package does not exist), inform the user about it and suppress the error output.
		nothingToDo := false
		if !alreadyInstalled {
			for _, marker := range suppressOutputMarkers {
				if strings.Contains(strings.ToLower(result), marker) {
					result = "nothing to do or not available"
					nothingToDo = true
					err = nil
					break
				}
			}
		}
		if err != nil || alreadyInstalled || nothingToDo {
			daemon.logPrintStageStep(out, "install/upgrade %s: (err? %v) %s", name, err, strings.TrimSpace(result))
		} else {
			daemon.logPrintStageStep(out, "install/upgrade %s: OK", name)
		}
	}
}
