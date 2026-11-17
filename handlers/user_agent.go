package handlers

import (
	"regexp"
	"strings"
)

// DeviceInfo holds parsed or provided device metadata
type DeviceInfo struct {
	Browser      *string `json:"browser,omitempty"`
	OS           *string `json:"os,omitempty"`
	Manufacturer *string `json:"manufacturer,omitempty"`
}

// ParseUserAgent extracts basic browser, OS, and manufacturer from User-Agent string.
func ParseUserAgent(ua string) DeviceInfo {
	if strings.TrimSpace(ua) == "" {
		return DeviceInfo{}
	}

	var browser, os, manufacturer string

	// Basic Browser Detection
	lowerUA := strings.ToLower(ua)
	switch {
	case strings.Contains(lowerUA, "edg/"):
		browser = "Edge"
	case strings.Contains(lowerUA, "chrome/") && !strings.Contains(lowerUA, "chromium/"):
		browser = "Chrome"
	case strings.Contains(lowerUA, "safari/") && !strings.Contains(lowerUA, "chrome/"):
		browser = "Safari"
	case strings.Contains(lowerUA, "firefox/"):
		browser = "Firefox"
	case strings.Contains(lowerUA, "opera/") || strings.Contains(lowerUA, "opr/"):
		browser = "Opera"
	default:
		browser = "Other"
	}

	// Basic OS & Manufacturer Detection
	switch {
	case strings.Contains(lowerUA, "iphone") || strings.Contains(lowerUA, "ipad") || strings.Contains(lowerUA, "ipod"):
		os = "iOS"
		manufacturer = "Apple"
	case strings.Contains(lowerUA, "macintosh") || strings.Contains(lowerUA, "mac os x"):
		os = "macOS"
		manufacturer = "Apple"
	case strings.Contains(lowerUA, "android"):
		os = "Android"
		// Try to parse manufacturer if present e.g. "Samsung", "Pixel", etc.
		if match := regexp.MustCompile(`(?i)\b(samsung|pixel|xiaomi|huawei|oneplus|sony|lg|motorola)\b`).FindString(lowerUA); match != "" {
			manufacturer = strings.Title(strings.ToLower(match))
		} else {
			manufacturer = "Generic Android"
		}
	case strings.Contains(lowerUA, "windows"):
		os = "Windows"
		manufacturer = "Microsoft / PC"
	case strings.Contains(lowerUA, "linux"):
		os = "Linux"
		manufacturer = "Generic PC"
	default:
		os = "Unknown"
		manufacturer = "Unknown"
	}

	return DeviceInfo{
		Browser:      &browser,
		OS:           &os,
		Manufacturer: &manufacturer,
	}
}
