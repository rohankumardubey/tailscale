// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// Package distro reports which distro we're running on.
package distro

import (
	"bytes"
	"os"
	"runtime"
	"strconv"
	"strings"

	"tailscale.com/types/lazy"
	"tailscale.com/util/lineiter"
)

type Distro string

const (
	Debian    = Distro("debian")
	Arch      = Distro("arch")
	Synology  = Distro("synology")
	OpenWrt   = Distro("openwrt")
	NixOS     = Distro("nixos")
	QNAP      = Distro("qnap")
	Pfsense   = Distro("pfsense")
	OPNsense  = Distro("opnsense")
	TrueNAS   = Distro("truenas")
	Gokrazy   = Distro("gokrazy")
	WDMyCloud = Distro("wdmycloud")
	Unraid    = Distro("unraid")
	Alpine    = Distro("alpine")
	UBNT      = Distro("ubnt") // Ubiquiti Networks
	JetKVM    = Distro("jetkvm")
)

var distro lazy.SyncValue[Distro]
var isWSL lazy.SyncValue[bool]

// Get returns the current distro, or the empty string if unknown.
func Get() Distro {
	return distro.Get(func() Distro {
		switch runtime.GOOS {
		case "linux":
			return linuxDistro()
		case "freebsd":
			return freebsdDistro()
		default:
			return Distro("")
		}
	})
}

// IsWSL reports whether we're running in the Windows Subsystem for Linux.
func IsWSL() bool {
	return runtime.GOOS == "linux" && isWSL.Get(func() bool {
		// We could look for $WSL_INTEROP instead, however that may be missing if
		// the user has started to use systemd in WSL2.
		return have("/proc/sys/fs/binfmt_misc/WSLInterop") || have("/mnt/wsl")
	})
}

func have(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

func haveDir(file string) bool {
	fi, err := os.Stat(file)
	return err == nil && fi.IsDir()
}

func linuxDistro() Distro {
	switch {
	case haveDir("/usr/syno"):
		return Synology
	case have("/usr/local/bin/freenas-debug"):
		// TrueNAS Scale runs on debian
		return TrueNAS
	case have("/usr/bin/ubnt-device-info"):
		// UBNT runs on Debian-based systems. This MUST be checked before Debian.
		//
		// Currently supported product families:
		// - UDM (UniFi Dream Machine, UDM-Pro)
		return UBNT
	case have("/etc/debian_version"):
		return Debian
	case have("/etc/arch-release"):
		return Arch
	case have("/etc/openwrt_version"):
		return OpenWrt
	case have("/run/current-system/sw/bin/nixos-version"):
		return NixOS
	case have("/etc/config/uLinux.conf"):
		return QNAP
	case haveDir("/gokrazy"):
		return Gokrazy
	case have("/usr/local/wdmcserver/bin/wdmc.xml"): // Western Digital MyCloud OS3
		return WDMyCloud
	case have("/usr/sbin/wd_crontab.sh"): // Western Digital MyCloud OS5
		return WDMyCloud
	case have("/etc/unraid-version"):
		return Unraid
	case have("/etc/alpine-release"):
		return Alpine
	case runtime.GOARCH == "arm" && isDeviceModel("JetKVM"):
		return JetKVM
	}
	return ""
}

func isDeviceModel(want string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	v, _ := os.ReadFile("/sys/firmware/devicetree/base/model")
	return want == strings.Trim(string(v), "\x00\r\n\t ")
}

func freebsdDistro() Distro {
	switch {
	case have("/etc/pfSense-rc"):
		return Pfsense
	case have("/usr/local/sbin/opnsense-shell"):
		return OPNsense
	case have("/usr/local/bin/freenas-debug"):
		// TrueNAS Core runs on FreeBSD
		return TrueNAS
	}
	return ""
}

var dsmVersion lazy.SyncValue[int]

// DSMVersion reports the Synology DSM major version.
//
// If not Synology, it reports 0.
func DSMVersion() int {
	if runtime.GOOS != "linux" {
		return 0
	}
	return dsmVersion.Get(func() int {
		if Get() != Synology {
			return 0
		}
		// This is set when running as a package:
		v, _ := strconv.Atoi(os.Getenv("SYNOPKG_DSM_VERSION_MAJOR"))
		if v != 0 {
			return v
		}
		// But when run from the command line, we have to read it from the file:
		for lr := range lineiter.File("/etc/VERSION") {
			line, err := lr.Value()
			if err != nil {
				break // but otherwise ignore
			}
			line = bytes.TrimSpace(line)
			if string(line) == `majorversion="7"` {
				return 7
			}
			if string(line) == `majorversion="6"` {
				return 6
			}
		}
		return 0
	})
}
