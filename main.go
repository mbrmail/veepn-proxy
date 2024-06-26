package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
	"runtime"

	xproxy "golang.org/x/net/proxy"
	ve "main/veclient"
)

const (
	API_DOMAIN   = "antpeak.com"
)

var (
	version = "undefined"
)

func perror(msg string) {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, msg)
}

func arg_fail(msg string) {
	perror(msg)
	perror("Usage:")
	flag.PrintDefaults()
	os.Exit(2)
}

type CLIArgs struct {
	country             string
	listCountries       bool
	bindAddress         string
	verbosity           int
	timeout             time.Duration
	showVersion         bool
	proxy               string
	apiAddress          string
	bootstrapDNS        string
	refresh             time.Duration
	refreshRetry        time.Duration
	certChainWorkaround bool
	caFile              string
	serviceInstall      bool
	serviceUninstall    bool
	serviceName         string
}

var args CLIArgs

func init() {
	flag.StringVar(&args.country, "country", "nl", "desired proxy location")
	flag.BoolVar(&args.listCountries, "list-countries", false, "list available countries and exit")
	flag.StringVar(&args.bindAddress, "bind-address", "127.0.0.1:18090", "HTTP proxy listen address")
	flag.IntVar(&args.verbosity, "verbosity", 20, "logging verbosity "+
		"(10 - debug, 20 - info, 30 - warning, 40 - error, 50 - critical)")
	flag.DurationVar(&args.timeout, "timeout", 10*time.Second, "timeout for network operations")
	flag.BoolVar(&args.showVersion, "version", false, "show program version and exit")
	flag.StringVar(&args.proxy, "proxy", "", "sets base proxy to use for all dial-outs. "+
		"Format: <http|https|socks5|socks5h>://[login:password@]host[:port] "+
		"Examples: http://user:password@192.168.1.1:3128, socks5://10.0.0.1:1080")
	flag.StringVar(&args.apiAddress, "api-address", "", fmt.Sprintf("override IP address of %s", API_DOMAIN))
	flag.StringVar(&args.bootstrapDNS, "bootstrap-dns", "",
		"DNS/DoH/DoT/DoQ resolver for initial discovering of SurfEasy API address. "+
			"See https://github.com/ameshkov/dnslookup/ for upstream DNS URL format. "+
			"Examples: https://1.1.1.1/dns-query, quic://dns.adguard.com")
	flag.DurationVar(&args.refresh, "refresh", 4*time.Hour, "login refresh interval")
	flag.DurationVar(&args.refreshRetry, "refresh-retry", 5*time.Second, "login refresh retry interval")
	flag.BoolVar(&args.certChainWorkaround, "certchain-workaround", true,
		"add bundled cross-signed intermediate cert to certchain to make it check out on old systems")
	flag.StringVar(&args.caFile, "cafile", "", "use custom CA certificate bundle file")

    if runtime.GOOS == "windows" {
    }
}

func parse_args() CLIArgs {
	flag.Parse()
	if args.country == "" {
		arg_fail("Country can't be empty string.")
	}
	if args.apiAddress != "" && args.bootstrapDNS != "" {
		arg_fail("api-address and bootstrap-dns options are mutually exclusive")
	}
	return args
}

func proxyFromURLWrapper(u *url.URL, next xproxy.Dialer) (xproxy.Dialer, error) {
	cdialer, ok := next.(ContextDialer)
	if !ok {
		return nil, errors.New("only context dialers are accepted")
	}

	return ProxyDialerFromURL(u, cdialer)
}

func run() int {
	args := parse_args()

	if args.showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	logWriter := NewLogWriter(os.Stderr)
	defer logWriter.Close()

	mainLogger := NewCondLogger(log.New(logWriter, "MAIN    : ",
		log.LstdFlags|log.Lshortfile),
		args.verbosity)

	proxyLogger := NewCondLogger(log.New(logWriter, "PROXY   : ",
		log.LstdFlags|log.Lshortfile),
		args.verbosity)

	mainLogger.Info("client version %s is starting...", version)

	var dialer ContextDialer = &net.Dialer {
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	if args.proxy != "" {
		xproxy.RegisterDialerType("http", proxyFromURLWrapper)
		xproxy.RegisterDialerType("https", proxyFromURLWrapper)
		proxyURL, err := url.Parse(args.proxy)
		if err != nil {
			mainLogger.Critical("Unable to parse base proxy URL: %v", err)
			return 6
		}
		pxDialer, err := xproxy.FromURL(proxyURL, dialer)
		if err != nil {
			mainLogger.Critical("Unable to instantiate base proxy dialer: %v", err)
			return 7
		}
		dialer = pxDialer.(ContextDialer)
	}

	veclientDialer := dialer

	if args.apiAddress != "" || args.bootstrapDNS != "" {
		var apiAddress string
		if args.apiAddress != "" {
			apiAddress = args.apiAddress
			mainLogger.Info("Using fixed API host IP address = %s", apiAddress)
		} else {
			resolver, err := NewResolver(args.bootstrapDNS, args.timeout)
			if err != nil {
				mainLogger.Critical("Unable to instantiate DNS resolver: %v", err)
				return 4
			}

			mainLogger.Info("Discovering API IP address...")
			addrs := resolver.ResolveA(API_DOMAIN)
			if len(addrs) == 0 {
				mainLogger.Critical("Unable to resolve %s with specified bootstrap DNS", API_DOMAIN)
				return 14
			}

			apiAddress = addrs[0]
			mainLogger.Info("Discovered address of API host = %s", apiAddress)
		}
		veclientDialer = NewFixedDialer(apiAddress, dialer)
	}

	// Dialing w/o SNI, receiving self-signed certificate, so skip verification.
	// Either way we'll validate certificate of actual proxy server.

	tlsConfig := &tls.Config{
		ServerName:         "",
		InsecureSkipVerify: true,
	}

	veclient, err := ve.NewVEClient(&http.Transport {
		DialContext: veclientDialer.DialContext,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := veclientDialer.DialContext(ctx, network, addr)
			if err != nil {
				return conn, err
			}
			return tls.Client(conn, tlsConfig), nil
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	})

	if err != nil {
		mainLogger.Critical("Unable to construct VEClient: %v", err)
		return 8
	}

	ctx, cl := context.WithTimeout(context.Background(), args.timeout)
	err = veclient.RegisterDevice(ctx)
	if err != nil {
		mainLogger.Critical("Unable to perform device registration: %v", err)
		return 10
	}
	cl()

	if args.listCountries {
		return printCountries(mainLogger, args.timeout, veclient)
	}

	ctx, cl = context.WithTimeout(context.Background(), args.timeout)
	ips, err := veclient.Discover(ctx, args.country)
	if err != nil {
		mainLogger.Critical("Endpoint discovery failed: %v", err)
		return 12
	}

	if len(ips.Addresses) == 0 {
		mainLogger.Critical("Empty endpoint!")
		return 13
	}

	runTicker(context.Background(), args.refresh, args.refreshRetry, func(ctx context.Context) error {
		mainLogger.Info("Refreshing device endpoint...")

		reqCtx, cl := context.WithTimeout(context.Background(), args.timeout)
		defer cl()
		ips, err := veclient.Discover(reqCtx, args.country)
		if err != nil {
			mainLogger.Critical("Endpoint discovery failed: %v", err)
			return err
		}

		if len(ips.Addresses) == 0 {
			mainLogger.Critical("Empty endpoint!")
			return err
		}

		mainLogger.Info("Device endpoint refreshed, Endpoint: %s", ips.NetAddr())
		return nil
	})

	auth := func() string {
		return basic_auth_header(veclient.GetProxyCredentials())
	}

	var caPool *x509.CertPool
	if args.caFile != "" {
		caPool = x509.NewCertPool()
		certs, err := ioutil.ReadFile(args.caFile)
		if err != nil {
			mainLogger.Error("Can't load CA file: %v", err)
			return 15
		}
		if ok := caPool.AppendCertsFromPEM(certs); !ok {
			mainLogger.Error("Can't load certificates from CA file")
			return 15
		}
	}

	handlerDialer := NewProxyDialer(ips.NetAddr(), ips.Addresses[0], auth, args.certChainWorkaround, caPool, dialer)
	mainLogger.Info("Endpoint: %s", ips.NetAddr())
	mainLogger.Info("Starting proxy server...")
	handler := NewProxyHandler(handlerDialer, proxyLogger)
	mainLogger.Info("Init complete.")
	err = http.ListenAndServe(args.bindAddress, handler)

	mainLogger.Critical("Server terminated with a reason: %v", err)
	mainLogger.Info("Shutting down...")

	return 0
}

func printCountries(logger *CondLogger, timeout time.Duration, veclient *ve.VEClient) int {
	ctx, cl := context.WithTimeout(context.Background(), timeout)
	defer cl()
	list, err := veclient.GeoList(ctx)
	if err != nil {
		logger.Critical("GeoList error: %v", err)
		return 11
	}

	wr := csv.NewWriter(os.Stdout)
	defer wr.Flush()
	wr.Write([]string{"country code", "country name"})
	for _, country := range list {
		if country.ProxyType == 0 {
			wr.Write([]string{country.Region, country.Name})
		}
	}
	return 0
}

func main() {
	CheckRunService()
	os.Exit(run())
}
