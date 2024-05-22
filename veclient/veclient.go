package veclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"

//	dac "main/digest_auth_client"
)

const (
	DEVICE_ID_BYTES                  = 20
	READ_LIMIT                 int64 = 128 * 1024
)

type VEEndpoints struct {
	RegisterDevice         string
	GeoList                string
	Discover               string
}

var DefaultVEEndpoints = VEEndpoints{
	RegisterDevice:         "https://antpeak.com/api/launch/",
	GeoList:                "https://antpeak.com/api/location/list/",
    Discover:               "https://antpeak.com/api/server/",
}

type VESettings struct {
	AppVersion      string
	Platform        string
	TimeZone        string
	DeviceName      string
	UserAgent       string
	Endpoints       VEEndpoints
}

var DefaultVESettings = VESettings {
	AppVersion:      "2.6.0",
	Platform:        "Chrome",
	TimeZone:        "+0300",
	DeviceName:      "Opera 85",
	UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.192 Safari/537.36 OPR/74.0.3911.232",
	Endpoints:       DefaultVEEndpoints,
}

type VEClient struct {
	httpClient           *http.Client
	Settings             VESettings
	AccessToken          string
	DeviceID             string
	Mux                  sync.Mutex
	rng                  *rand.Rand
	DeviceUsername       string
	DevicePassword       string
}

type StrKV map[string]string

// Instantiates VEEPN client with default settings and given API keys.
// Optional `transport` parameter allows to override HTTP transport used
// for HTTP calls

func NewVEClient(transport http.RoundTripper) (*VEClient, error) {
	if transport == nil {
		transport = http.DefaultTransport
	}

	rng := rand.New(RandomSource)

	device_id, err := randomHexString(rng, DEVICE_ID_BYTES)
	if err != nil {
		return nil, err
	}

	jar, err := NewStdJar()
	if err != nil {
		return nil, err
	}

	res := &VEClient {
		httpClient: &http.Client{
			Jar:       jar,
			// Transport: dac.NewDigestTransport(apiUsername, apiSecret, transport),
		},
		Settings: DefaultVESettings,
		rng:      rng,
		DeviceID: device_id,
	}

	return res, nil
}

func (c *VEClient) ResetCookies() error {
	c.Mux.Lock()
	defer c.Mux.Unlock()
	return c.resetCookies()
}

func (c *VEClient) resetCookies() error {
	return (c.httpClient.Jar.(*StdJar)).Reset()
}

func Var_dump(expression ...interface{} ) {
	fmt.Println(fmt.Sprintf("%#v", expression))
}

func (c *VEClient) RegisterDevice(ctx context.Context) error {
	c.Mux.Lock()
	defer c.Mux.Unlock()

	var regRes VERegisterDeviceResponse

	err := c.rpcCall(ctx, c.Settings.Endpoints.RegisterDevice, StrKV {
		"appVersion": c.Settings.AppVersion,
		"platform": c.Settings.Platform,
		"timeZone": c.Settings.TimeZone,
		"deviceName": c.Settings.DeviceName,
		"udid": c.DeviceID,
	}, &regRes)

	if regRes.Status != true {
		return fmt.Errorf("API responded with error message: msg=\"%s\"",
			regRes.Errors[0].Message)
	}

	if err != nil {
		return err
	}

	c.AccessToken = regRes.Data.AccessToken

	return nil
}

func (c *VEClient) GeoList(ctx context.Context) ([]VEGeoEntry, error) {
	c.Mux.Lock()
	defer c.Mux.Unlock()

	var geoListRes VEGeoListResponse

	err := c.rpcCall(ctx, c.Settings.Endpoints.GeoList, StrKV{}, &geoListRes)

	if geoListRes.Status != true {
		return nil, fmt.Errorf("API responded with error message: msg=\"%s\"",
			geoListRes.Errors[0].Message)
	}

	if err != nil {
		return nil, err
	}

	return geoListRes.Data.Geos, nil
}

func (c *VEClient) Discover(ctx context.Context, requestedGeo string) (*VEIPEntry, error) {
	c.Mux.Lock()
	defer c.Mux.Unlock()

	var discoverRes VEDiscoverResponse

	err := c.rpcCall(ctx, c.Settings.Endpoints.Discover, StrKV{
		"region": requestedGeo,
		"protocol": "https",
		"type": "0",
	}, &discoverRes)

	if discoverRes.Status != true {
		return nil, fmt.Errorf("%s (%s)", discoverRes.Errors[0].Message, requestedGeo)
	}

	if err != nil {
		return nil, err
	}

	c.DeviceUsername = discoverRes.Data.Username
	c.DevicePassword = discoverRes.Data.Password

	return &discoverRes.Data, nil
}

func (c *VEClient) GetProxyCredentials() (string, string) {
	c.Mux.Lock()
	defer c.Mux.Unlock()

	return c.DeviceUsername, c.DevicePassword
}

func (c *VEClient) populateRequest(req *http.Request) {
	if len(c.AccessToken) != 0 {
//		var bearer = "Bearer " + c.AccessToken
		req.Header.Set("Authorization", "Bearer " + c.AccessToken)
	}
}

func (c *VEClient) rpcCall(ctx context.Context, endpoint string, params map[string]string, res interface{}) error {
	input := make(url.Values)
	for k, v := range params {
		input[k] = []string{v}
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		endpoint,
		strings.NewReader(input.Encode()),
	)

	if err != nil {
		return err
	}

	c.populateRequest(req)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(res)
	cleanupBody(resp.Body)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad http status: %s, headers: %#v", resp.Status, resp.Header)
	}

	return nil
}

// Does cleanup of HTTP response in order to make it reusable by keep-alive
// logic of HTTP client
func cleanupBody(body io.ReadCloser) {
	io.Copy(ioutil.Discard, &io.LimitedReader{
		R: body,
		N: READ_LIMIT,
	})
	body.Close()
}
