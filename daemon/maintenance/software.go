package maintenance

import (
	"bytes"
	"fmt"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

/*
PrepareDockerRepositorForDebian prepares APT repository for installing debian, because debian does not distribute
docker in their repository for whatever reason. If the system is not a debian the function will do nothing.
*/
func (daemon *Daemon) PrepareDockerRepositoryForDebian(out *bytes.Buffer) {
	if misc.HostIsWindows() {
		daemon.logPrintStage(out, "skipped on windows: prepare docker repository for debian")
		return
	}
	daemon.logPrintStage(out, "prepare docker repository for debian")
	content, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		daemon.logPrintStageStep(out, "failed to read os-release, this is not a critical error.")
		return
	} else if !strings.Contains(strings.ToLower(string(content)), "debian") || strings.Contains(strings.ToLower(string(content)), "ubuntu") {
		daemon.logPrintStageStep(out, "system is not a debian, just FYI.")
		return
	}
	// Install docker's GPG key
	resp, err := inet.DoHTTP(inet.HTTPRequest{}, "https://download.docker.com/linux/debian/gpg")
	if err != nil {
		daemon.logPrintStageStep(out, "failed to download docker GPG key - %v", err)
		return
	}
	gpgKeyFile := "/tmp/laitos-docker-gpg-key"
	if err := ioutil.WriteFile(gpgKeyFile, resp.Body, 0600); err != nil {
		daemon.logPrintStageStep(out, "failed to store docker GPG key - %v", err)
		return
	}
	aptOut, err := misc.InvokeProgram(nil, 10, "apt-key", "add", gpgKeyFile)
	daemon.logPrintStageStep(out, "install docker GPG key - %v %s", err, aptOut)
	// Add docker community edition repository
	lsbOut, err := misc.InvokeProgram(nil, 10, "lsb_release", "-cs")
	daemon.logPrintStageStep(out, "determine release name - %v %s", err, lsbOut)
	if err != nil {
		daemon.logPrintStageStep(out, "failed to determine release name")
		return
	}
	aptOut, err = misc.InvokeProgram(nil, 10, "add-apt-repository", fmt.Sprintf("https://download.docker.com/linux/debian %s stable", strings.TrimSpace(string(lsbOut))))
	daemon.logPrintStageStep(out, "enable docker repository - %v %s", err, aptOut)
}

/*
UpgradeInstallSoftware uses Linux package manager to ensure that all system packages are up to date and installs
optional laitos dependencies as well as diagnosis utilities.
*/
func (daemon *Daemon) UpgradeInstallSoftware(out *bytes.Buffer) {
	if misc.HostIsWindows() {
		daemon.logPrintStage(out, "skipped on windows: upgrade/install software")
		return
	}
	// Find a system package manager
	var pkgManagerPath, pkgManagerName string
	for _, binPrefix := range []string{"/sbin", "/bin", "/usr/sbin", "/usr/bin", "/usr/sbin/local", "/usr/bin/local"} {
		/*
			Prefer zypper over apt-get bacause opensuse has a weird "apt-get wrapper" that is not remotely functional.
			Prefer apt over apt-get because some public cloud OS templates can upgrade kernel via apt but not with apt-get.
		*/
		for _, execName := range []string{"yum", "zypper", "apt", "apt-get"} {
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
	if pkgManagerName == "" {
		daemon.logPrintStage(out, "failed to find package manager")
		return
	}
	daemon.logPrintStage(out, "package maintenance via %s", pkgManagerPath)
	// Determine package manager invocation parameters
	var sysUpgradeArgs, installArgs []string
	switch pkgManagerName {
	case "yum":
		// yum is simple and easy
		sysUpgradeArgs = []string{"-y", "-t", "--skip-broken", "update"}
		installArgs = []string{"-y", "-t", "--skip-broken", "install"}
	case "apt":
		// apt and apt-get are too old to be convenient
		fallthrough
	case "apt-get":
		sysUpgradeArgs = []string{"-q", "-y", "-f", "-m", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", "upgrade"}
		installArgs = []string{"-q", "-y", "-f", "-m", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold", "install"}
	case "zypper":
		// zypper cannot English and integrity
		sysUpgradeArgs = []string{"--non-interactive", "update", "--recommends", "--auto-agree-with-licenses", "--skip-interactive", "--replacefiles", "--force-resolution"}
		installArgs = []string{"--non-interactive", "install", "--recommends", "--auto-agree-with-licenses", "--replacefiles", "--force-resolution"}
	default:
		daemon.logPrintStageStep(out, "failed to find a compatible package manager")
		return
	}
	// apt-get is too old to be convenient, it has to update the manifest first.
	pkgManagerEnv := make([]string, 0, 8)
	if pkgManagerName == "apt-get" {
		pkgManagerEnv = append(pkgManagerEnv, "DEBIAN_FRONTEND=noninteractive")
		// Five minutes should be enough to grab the latest manifest
		result, err := misc.InvokeProgram(pkgManagerEnv, 5*60, pkgManagerPath, "update")
		// There is no need to suppress this output according to markers
		daemon.logPrintStageStep(out, "update apt manifests: %v - %s", err, strings.TrimSpace(result))
	}
	// If package manager output contains any of the strings, the output is reduced into "Nothing to do"
	suppressOutputMarkers := []string{"No packages marked for update", "Nothing to do", "0 upgraded, 0 newly installed", "Unable to locate"}
	// Upgrade system packages with a time constraint of two hours
	result, err := misc.InvokeProgram(pkgManagerEnv, 2*3600, pkgManagerPath, sysUpgradeArgs...)
	for _, marker := range suppressOutputMarkers {
		// If nothing was done during system update, suppress the rather useless output.
		if strings.Contains(result, marker) {
			result = "(nothing to do or upgrade not available)"
			break
		}
	}
	daemon.logPrintStageStep(out, "upgrade system: %v - %s", err, strings.TrimSpace(result))
	/*
		Install additional software packages.
		laitos itself does not rely on any third-party library or program to run, however, it is very useful to install
		several PhantomJS/SlimerJS dependencies, as well as utility applications to help out with system diagnosis.
		Several of the packages are repeated under different names to accommodate the differences in naming convention
		among distributions.
	*/
	pkgs := []string{
		// For outgoing HTTPS connections
		"ca-certificates",

		// Utilities for APT maintenance that also help with installer docker community edition on Debian
		"apt-transport-https", "gnupg", "software-properties-common",
		// Docker for running SlimerJS
		"docker", "docker-client", "docker.io", "docker-ce",

		// Soft and hard dependencies of PhantomJS
		"bzip2", "bzip2-libs", "cjkuni-fonts-common", "cjkuni-ukai-fonts", "cjkuni-uming-fonts", "dbus", "dejavu-fonts-common",
		"dejavu-sans-fonts", "dejavu-serif-fonts", "expat", "firefox", "font-noto", "fontconfig", "fontconfig-config",
		"fontpackages-filesystem", "fonts-arphic-ukai", "fonts-arphic-uming", "fonts-dejavu-core", "fonts-liberation", "freetype",
		"gnutls", "icu", "intlfonts-chinese-big-bitmap-fonts", "intlfonts-chinese-bitmap-fonts", "lib64z1", "libXfont", "libbz2-1",
		"libbz2-1.0", "liberation-fonts-common", "liberation-mono-fonts", "liberation-sans-fonts", "liberation-serif-fonts",
		"liberation2-fonts", "libexpat1", "libfontconfig1", "libfontenc", "libfreetype6", "libicu", "libicu57", "libicu60_2",
		"libpng", "libpng16-16", "nss", "openssl", "ttf-dejavu", "ttf-freefont", "ttf-liberation", "wqy-zenhei", "xorg-x11-font-utils",
		"xorg-x11-fonts-Type1", "zlib", "zlib1g",

		// Time maintenance utilities
		"chrony", "ntp", "ntpd", "ntpdate",
		// Application zip bundle maintenance utilities
		"unzip", "zip",
		// Network diagnosis utilities
		"bind-utils", "curl", "dnsutils", "nc", "net-tools", "netcat", "nmap", "procps", "rsync", "telnet", "tcpdump", "traceroute", "wget", "whois",
		// busybox and toybox are useful for general maintenance, and busybox can synchronise system clock as well.
		"busybox", "toybox",
		// System maintenance utilities
		"lsof", "strace", "sudo", "vim",
	}
	pkgs = append(pkgs, daemon.InstallPackages...)
	/*
		Although all three package managers can install more than one packages at a time, the packages are still
		installed one after another, because:
		- apt-get does not ignore non-existent package names, how inconvenient.
		- if zypper runs into unsatisfactory package dependencies, it aborts the whole installation.
		yum is once again the superior solution among all three.
	*/
	for _, name := range pkgs {
		// Put software name next to installation parameters
		pkgInstallArgs := make([]string, len(installArgs)+1)
		copy(pkgInstallArgs, installArgs)
		pkgInstallArgs[len(installArgs)] = name
		// Ten minutes should be good enough for each package
		result, err := misc.InvokeProgram(pkgManagerEnv, 10*60, pkgManagerPath, pkgInstallArgs...)
		if err != nil {
			for _, marker := range suppressOutputMarkers {
				// If nothing was done about the package, suppress the rather useless output.
				if strings.Contains(result, marker) {
					result = "(nothing to do or package not available)"
					break
				}
			}
			daemon.logPrintStageStep(out, "install/upgrade %s: %v - %s", name, err, strings.TrimSpace(result))
		}
	}
}
