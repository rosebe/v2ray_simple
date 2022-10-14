package proxy

import (
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

//used in real relay progress. See source code of v2ray_simple for details.
type RoutingEnv struct {
	RoutePolicy  *netLayer.RoutePolicy
	MainFallback *httpLayer.ClassicFallback
	DnsMachine   *netLayer.DNSMachine

	ClientsTagMap      map[string]Client //ClientsTagMap 存储 tag 对应的 Client；因为分流时，需要通过某个tag找到Client对象。 若要访问map，请用 Get*, Set*, Del* 方法
	clientsTagMapMutex sync.RWMutex
}

func (re *RoutingEnv) GetClient(tag string) (c Client) {
	re.clientsTagMapMutex.RLock()

	c = re.ClientsTagMap[tag]
	re.clientsTagMapMutex.RUnlock()
	return
}
func (re *RoutingEnv) SetClient(tag string, c Client) {
	re.clientsTagMapMutex.Lock()

	re.ClientsTagMap[tag] = c
	re.clientsTagMapMutex.Unlock()
}
func (re *RoutingEnv) DelClient(tag string) {
	re.clientsTagMapMutex.Lock()

	delete(re.ClientsTagMap, tag)
	re.clientsTagMapMutex.Unlock()
}

func LoadEnvFromStandardConf(standardConf *StandardConf) (routingEnv RoutingEnv) {

	routingEnv.ClientsTagMap = make(map[string]Client)

	if len(standardConf.Fallbacks) != 0 {
		routingEnv.MainFallback = httpLayer.NewClassicFallbackFromConfList(standardConf.Fallbacks)
	}

	if dnsConf := standardConf.DnsConf; dnsConf != nil {
		routingEnv.DnsMachine = netLayer.LoadDnsMachine(dnsConf)
	}

	var hasAppLevelMyCountry bool

	if appConf := standardConf.App; appConf != nil {

		hasAppLevelMyCountry = appConf.MyCountryISO_3166 != ""

		if appConf.UDP_timeout != nil {
			minutes := *appConf.UDP_timeout
			if minutes > 0 {
				netLayer.UDP_timeout = time.Minute * time.Duration(minutes)
			}
		}
	}

	if standardConf.Route != nil || hasAppLevelMyCountry {

		netLayer.LoadMaxmindGeoipFile("")

		rp := netLayer.NewRoutePolicy()
		if hasAppLevelMyCountry {
			rp.AddRouteSet(netLayer.NewRouteSetForMyCountry(standardConf.App.MyCountryISO_3166))

		}

		rp.LoadRulesForRoutePolicy(standardConf.Route)

		routingEnv.RoutePolicy = rp

	}

	return
}
