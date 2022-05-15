package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

var gConf Config

const IntValueNotSet int = -99999999

type AppConfig struct {
	// required
	AppName  string
	Protocol string
	SrcPort  int
	PeerNode string
	DstPort  int
	DstHost  string
	PeerUser string
	Enabled  int // default:1
	// runtime info
	peerVersion     string
	peerToken       uint64
	peerNatType     int
	hasIPv4         int
	IPv6            string
	hasUPNPorNATPMP int
	peerIP          string
	peerConeNatPort int
	retryNum        int
	retryTime       time.Time
	nextRetryTime   time.Time
	shareBandwidth  int
	errMsg          string
	connectTime     time.Time
	fromToken       uint64
}

// TODO: add loglevel, maxlogfilesize
type Config struct {
	Network    NetworkConfig `json:"network"`
	Apps       []*AppConfig  `json:"apps"`
	LogLevel   int
	daemonMode bool
	mtx        sync.Mutex
}

func (c *Config) switchApp(app AppConfig, enabled int) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for i := 0; i < len(c.Apps); i++ {
		if c.Apps[i].Protocol == app.Protocol && c.Apps[i].SrcPort == app.SrcPort {
			c.Apps[i].Enabled = enabled
			c.Apps[i].retryNum = 0
			c.Apps[i].nextRetryTime = time.Now()
			return
		}
	}
}

func (c *Config) add(app AppConfig, override bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if app.SrcPort == 0 || app.DstPort == 0 {
		gLog.Println(LvERROR, "invalid app ", app)
		return
	}
	if override {
		for i := 0; i < len(c.Apps); i++ {
			if c.Apps[i].Protocol == app.Protocol && c.Apps[i].SrcPort == app.SrcPort {
				c.Apps[i] = &app // override it
				return
			}
		}
	}
	c.Apps = append(c.Apps, &app)
}

func (c *Config) delete(app AppConfig) {
	if app.SrcPort == 0 || app.DstPort == 0 {
		return
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for i := 0; i < len(c.Apps); i++ {
		if c.Apps[i].Protocol == app.Protocol && c.Apps[i].SrcPort == app.SrcPort {
			c.Apps = append(c.Apps[:i], c.Apps[i+1:]...)
			return
		}
	}
}

func (c *Config) save() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	data, _ := json.MarshalIndent(c, "", "  ")
	err := ioutil.WriteFile("config.json", data, 0644)
	if err != nil {
		gLog.Println(LvERROR, "save config.json error:", err)
	}
}

func (c *Config) load() error {
	c.mtx.Lock()
	c.LogLevel = IntValueNotSet
	c.Network.ShareBandwidth = IntValueNotSet
	defer c.mtx.Unlock()
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		// gLog.Println(LevelERROR, "read config.json error:", err)
		return err
	}
	err = json.Unmarshal(data, &c)
	if err != nil {
		gLog.Println(LvERROR, "parse config.json error:", err)
	}
	return err
}

func (c *Config) setToken(token uint64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.Network.Token = token
}
func (c *Config) setUser(user string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.Network.User = user
}
func (c *Config) setNode(node string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.Network.Node = node
}
func (c *Config) setShareBandwidth(bw int) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.Network.ShareBandwidth = bw
}

type NetworkConfig struct {
	// local info
	Token           uint64
	Node            string
	User            string
	localIP         string
	mac             string
	os              string
	publicIP        string
	natType         int
	hasIPv4         int
	IPv6            string
	hasUPNPorNATPMP int
	ShareBandwidth  int
	// server info
	ServerHost string
	ServerPort int
	UDPPort1   int
	UDPPort2   int
}

func parseParams(subCommand string) {
	fset := flag.NewFlagSet(subCommand, flag.ExitOnError)
	serverHost := fset.String("serverhost", "api.openp2p.cn", "server host ")
	// serverHost := flag.String("serverhost", "127.0.0.1", "server host ") // for debug
	token := fset.Uint64("token", 0, "token")
	node := fset.String("node", "", "node name. 8-31 characters. if not set, it will be hostname")
	peerNode := fset.String("peernode", "", "peer node name that you want to connect")
	dstIP := fset.String("dstip", "127.0.0.1", "destination ip ")
	dstPort := fset.Int("dstport", 0, "destination port ")
	srcPort := fset.Int("srcport", 0, "source port ")
	protocol := fset.String("protocol", "tcp", "tcp or udp")
	appName := fset.String("appname", "", "app name")
	shareBandwidth := fset.Int("sharebandwidth", 10, "N mbps share bandwidth limit, private network no limit")
	daemonMode := fset.Bool("d", false, "daemonMode")
	notVerbose := fset.Bool("nv", false, "not log console")
	newconfig := fset.Bool("newconfig", false, "not load existing config.json")
	logLevel := fset.Int("loglevel", 1, "0:debug 1:info 2:warn 3:error")
	if subCommand == "" { // no subcommand
		fset.Parse(os.Args[1:])
	} else {
		fset.Parse(os.Args[2:])
	}

	config := AppConfig{Enabled: 1}
	config.PeerNode = *peerNode
	config.DstHost = *dstIP
	config.DstPort = *dstPort
	config.SrcPort = *srcPort
	config.Protocol = *protocol
	config.AppName = *appName
	if !*newconfig {
		gConf.load() // load old config. otherwise will clear all apps
	}
	gConf.LogLevel = *logLevel
	if config.SrcPort != 0 {
		gConf.add(config, true)
	}
	// gConf.mtx.Lock() // when calling this func it's single-thread no lock
	gConf.daemonMode = *daemonMode
	// spec paramters in commandline will always be used
	fset.Visit(func(f *flag.Flag) {
		if f.Name == "sharebandwidth" {
			gConf.Network.ShareBandwidth = *shareBandwidth
		}
		if f.Name == "node" {
			gConf.Network.Node = *node
		}
		if f.Name == "serverhost" {
			gConf.Network.ServerHost = *serverHost
		}
		if f.Name == "loglevel" {
			gConf.LogLevel = *logLevel
		}
	})

	if gConf.Network.ServerHost == "" {
		gConf.Network.ServerHost = *serverHost
	}
	if *token != 0 {
		gConf.Network.Token = *token
	}
	if *node != "" {
		if len(*node) < 8 {
			gLog.Println(LvERROR, ErrNodeTooShort)
			os.Exit(9)
		}
		gConf.Network.Node = *node
	} else {
		if gConf.Network.Node == "" { // if node name not set. use os.Hostname
			gConf.Network.Node = defaultNodeName()
		}
	}
	if gConf.LogLevel == IntValueNotSet {
		gConf.LogLevel = *logLevel
	}
	if gConf.Network.ShareBandwidth == IntValueNotSet {
		gConf.Network.ShareBandwidth = *shareBandwidth
	}

	gConf.Network.ServerPort = 27183
	gConf.Network.UDPPort1 = 27182
	gConf.Network.UDPPort2 = 27183
	gLog.setLevel(LogLevel(gConf.LogLevel))
	if *notVerbose {
		gLog.setMode(LogFile)
	}
	// gConf.mtx.Unlock()
	gConf.save()
}

func (conf *AppConfig) isSupportTCP(pnConf NetworkConfig) bool {
	if conf.peerVersion == "" || compareVersion(conf.peerVersion, LeastSupportTCPVersion) == LESS {
		return false
	}
	if pnConf.hasIPv4 == 1 || pnConf.hasUPNPorNATPMP == 1 || conf.hasIPv4 == 1 || conf.hasUPNPorNATPMP == 1 || (IsIPv6(pnConf.IPv6) && IsIPv6(conf.IPv6)) {
		return true
	}
	return false
}
