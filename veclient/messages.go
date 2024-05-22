package veclient

import (
	"fmt"
	"net"
)

const (
	VE_STATUS_OK int64 = 0
)

type VEStatusPair struct {
	Code    int64
	Message string
}

type VERegisterDeviceData struct {
	AccessToken  string `json:"accessToken,omitempty"`
}

type Error struct {
	Name     string `json:"name"`
	Message  string `json:"message"`
}

type VERegisterDeviceResponse struct {
	Data   VERegisterDeviceData `json:"data"`
	Status bool                 `json:"success"`
	Errors []Error              `json:"errors,omitempty"`
}

type VEGeoEntry struct {
    ProxyType   int    `json:"proxyType"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	CountryCode string `json:"countryCode"`
}

type VEGeoListResponse struct {
	Data struct {
		Geos []VEGeoEntry `json:"locations"`
	} `json:"data"`
	Status bool           `json:"success"`
	Errors []Error        `json:"errors,omitempty"`
}

type VEIPEntry struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	Protocol        string `json:"protocol"`
	Port            int    `json:"port"`
	Addresses     []string `json:"addresses"`
}

func (e *VEIPEntry) NetAddr() string {
	return net.JoinHostPort(e.Addresses[0], fmt.Sprintf("%d", e.Port))
}

type VEDiscoverResponse struct {
	Data    VEIPEntry      `json:"data"`
	Status  bool           `json:"success"`
	Errors []Error        `json:"errors,omitempty"`
}
