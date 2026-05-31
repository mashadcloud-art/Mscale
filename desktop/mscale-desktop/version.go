package main

// appVersion is set at link time: -ldflags "-X main.appVersion=0.4.27"
var appVersion = "0.4.28"

func GetAppVersion() string {
	if appVersion == "" {
		return "0.0.0"
	}
	return appVersion
}
