package ui

import "wintoolbox/internal/wintime"

const (
	AppName    = "WinToolbox" // synced from version.json via scripts/sync-version.ps1
	AppVersion = "v1.1"       // synced from version.json via scripts/sync-version.ps1
)

// AppInfo is version metadata for the UI.
type AppInfo struct {
	Name    string `json:"name"`    // WinToolbox
	Version string `json:"version"` // v1.1
}

// Status is a full app snapshot for UI.
type Status struct {
	Overview         OverviewView         `json:"overview"`
	Accounts         []AccountView        `json:"accounts"`
	RdpEnabled       bool                 `json:"rdpEnabled"`
	RdpPort          uint32               `json:"rdpPort"`
	RdpAvailable     bool                 `json:"rdpAvailable"`
	UpdateDisabled   bool                 `json:"updateDisabled"`
	UpdateDetail     string               `json:"updateDetail"`
	FirewallSummary  string               `json:"firewallSummary"`
	FirewallDomain   string               `json:"firewallDomain"`
	FirewallPrivate  string               `json:"firewallPrivate"`
	FirewallPublic   string               `json:"firewallPublic"`
	FirewallAllOn    bool                 `json:"firewallAllOn"`
	FirewallAllOff   bool                 `json:"firewallAllOff"`
	PingBlocked      bool                 `json:"pingBlocked"`
	PingIPv4Blocked  bool                 `json:"pingIPv4Blocked"`
	PingIPv6Blocked  bool                 `json:"pingIPv6Blocked"`
	PingState        string               `json:"pingState"`
	DefenderDisabled bool                 `json:"defenderDisabled"`
	DefenderDetail   string               `json:"defenderDetail"`
	TimeText         string               `json:"timeText"`
	TimeZone         string               `json:"timeZone"`
	TimeZones        []wintime.ZoneOption `json:"timeZones"`
	NTPServer        string               `json:"ntpServer"`
	Warnings         []string             `json:"warnings,omitempty"`
	LockoutDisabled  bool                 `json:"lockoutDisabled"`
	LockoutUnknown   bool                 `json:"lockoutUnknown"`
	LockoutDetail    string               `json:"lockoutDetail"`
	LockoutThreshold int                  `json:"lockoutThreshold"`
	LockoutDuration  int                  `json:"lockoutDuration"`
	LockoutWindow    int                  `json:"lockoutWindow"`
}

// OverviewView is display-ready overview data.
type OverviewView struct {
	Hostname         string   `json:"hostname"`
	OSName           string   `json:"osName"`
	OSBuild          string   `json:"osBuild"`
	Arch             string   `json:"arch"`
	Manufacturer     string   `json:"manufacturer"`
	Model            string   `json:"model"`
	Board            string   `json:"board"`
	BIOS             string   `json:"bios"`
	CPU              string   `json:"cpu"`
	CPUCores         int      `json:"cpuCores"`
	MemoryTotalGB    float64  `json:"memoryTotalGB"`
	MemoryAvailGB    float64  `json:"memoryAvailGB"`
	MemoryModules    string   `json:"memoryModules"`
	Resolution       string   `json:"resolution"`
	IPs              []string `json:"ips"`
	Disks            []string `json:"disks"`
	PhysicalDisks    []string `json:"physicalDisks"`
	GPUs             []string `json:"gpus"`
	Activated        bool     `json:"activated"`
	ActivationStatus string   `json:"activationStatus"`
}

// AccountView is a local account row for UI.
type AccountView struct {
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	EnabledUnknown bool   `json:"enabledUnknown"`
	Admin          bool   `json:"admin"`
	AdminUnknown   bool   `json:"adminUnknown"`
	Current        bool   `json:"current"`
}

// OverviewDetail is the slow slice filled after first paint.
type OverviewDetail struct {
	MemoryModules    string   `json:"memoryModules"`
	PhysicalDisks    []string `json:"physicalDisks"`
	GPUs             []string `json:"gpus"`
	Activated        bool     `json:"activated"`
	ActivationStatus string   `json:"activationStatus"`
}
