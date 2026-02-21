package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    Server     `yaml:"server"`
	Transport *Transport `yaml:"transport,omitempty"`
	Routes    []Route    `yaml:"routes"`
}

type Transport struct {
	MaxIdleConns        *int    `yaml:"max_idle_conns,omitempty"`
	MaxIdleConnsPerHost *int    `yaml:"max_idle_conns_per_host,omitempty"`
	IdleConnTimeout     *string `yaml:"idle_conn_timeout,omitempty"`
	Timeout             *string `yaml:"timeout,omitempty"`
	DisableCompression  *bool   `yaml:"disable_compression,omitempty"`
	ForceHTTP2          *bool   `yaml:"force_http2,omitempty"`
}

type Server struct {
	Listen int `yaml:"listen"`
}

type Route struct {
	Path    string   `yaml:"path"`
	Target  *string  `yaml:"target"`
	Targets []string `yaml:"targets"`
}

type compiledRoute struct {
	prefix    string
	upstreams []string
	counter   uint64
}

type Proxy struct {
	Config *Config
	routes []compiledRoute
	Client *http.Client
}

func loadConfig(path string) (*Config, error) {
	yamlFile, err := os.ReadFile(path)
	var config Config
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		return nil, err
	}

	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		return nil, err
	}
	sort.Slice(config.Routes, func(i, j int) bool {
		return len(config.Routes[i].Path) > len(config.Routes[j].Path)
	})
	return &config, nil
}

func buildHTTPClient(transportConfig *Transport) (*http.Client, error) {
	// Defaults
	maxIdleConns := 1000
	maxIdleConnsPerHost := 100
	idleConnTimeout := 90 * time.Second
	reqTimeout := 30 * time.Second
	disableCompression := false
	forceHTTP2 := true

	// Override if available
	if transportConfig != nil {
		if transportConfig.MaxIdleConns != nil {
			maxIdleConns = *transportConfig.MaxIdleConns
		}
		if transportConfig.MaxIdleConnsPerHost != nil {
			maxIdleConnsPerHost = *transportConfig.MaxIdleConnsPerHost
		}
		if transportConfig.IdleConnTimeout != nil {
			d, err := time.ParseDuration(*transportConfig.IdleConnTimeout)
			if err != nil {
				return nil, err
			}
			idleConnTimeout = d
		}
		if transportConfig.Timeout != nil {
			d, err := time.ParseDuration(*transportConfig.Timeout)
			if err != nil {
				return nil, err
			}
			reqTimeout = d
		}
		if transportConfig.DisableCompression != nil {
			disableCompression = *transportConfig.DisableCompression
		}
		if transportConfig.ForceHTTP2 != nil {
			forceHTTP2 = *transportConfig.ForceHTTP2
		}
	}

	transport := &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
		DisableCompression:  disableCompression,
		ForceAttemptHTTP2:   forceHTTP2,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   reqTimeout,
	}

	return client, nil
}

func (r *Route) normalizeTargets() ([]string, error) {
	if r.Target != nil && len(r.Targets) > 0 {
		return nil, fmt.Errorf("route %s: cannot specify both target and targets", r.Path)
	}

	if r.Target != nil {
		return []string{*r.Target}, nil
	}

	if len(r.Targets) > 0 {
		return r.Targets, nil
	}

	return nil, fmt.Errorf("route %s: no upstream targets defined", r.Path)
}

func (p *Proxy) initializeRoutes(config Config) {
	for _, route := range config.Routes {
		upstreams, err := route.normalizeTargets()

		if err != nil {
			log.Fatal(err)
		}

		p.routes = append(p.routes, compiledRoute{
			prefix:    route.Path,
			upstreams: upstreams,
			counter:   0,
		})
	}
}

func (cr *compiledRoute) NextUpstream() string {
	n := atomic.AddUint64(&cr.counter, 1)
	idx := int(n % uint64(len(cr.upstreams)))
	return cr.upstreams[idx]
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	requestPath := r.URL.Path
	var target string

	for i := range p.routes {
		if strings.HasPrefix(requestPath, p.routes[i].prefix) {
			target = p.routes[i].NextUpstream()
			break
		}
	}

	if target == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	targetURL, _ := url.Parse(target)

	targetURL.Path = strings.TrimSuffix(targetURL.Path, "/") + requestPath
	targetURL.RawQuery = r.URL.RawQuery

	fullUrl := targetURL.String()

	forwardedRequest, forwardedRequestError := http.NewRequestWithContext(r.Context(), method, fullUrl, r.Body)

	if forwardedRequestError != nil {
		fmt.Println("Error creating a request")
	}

	fmt.Println("User adress: ", r.RemoteAddr, " Forwarded to: ", fullUrl)

	forwardedRequest.Header = r.Header.Clone()
	forwardedRequest.Host = targetURL.Host

	response, reponseError := p.Client.Do(forwardedRequest)

	if reponseError != nil {
		log.Printf("Error sending a request: %v", reponseError)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	defer response.Body.Close()

	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(response.StatusCode)

	io.Copy(w, response.Body)

}

func main() {
	config, err := loadConfig("./config.yml")

	if err != nil {
		log.Fatal("error occured during loading config file", err)
	}

	fmt.Println(config.Routes)

	proxy := &Proxy{
		Config: config,
	}

	proxy.initializeRoutes(*config)

	httpClient, httpClientErr := buildHTTPClient(proxy.Config.Transport)

	if httpClientErr != nil {
		log.Fatal(httpClientErr)
	}

	proxy.Client = httpClient

	serveErr := http.ListenAndServe(":"+strconv.Itoa(config.Server.Listen), proxy)

	if serveErr != nil {
		log.Fatal("error occured during starting the server", serveErr)
	}
}
