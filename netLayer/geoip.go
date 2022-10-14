package netLayer

import (
	_ "embed"
	"log"
	"net"
	"os"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/oschwald/maxminddb-golang"
	"go.uber.org/zap"
)

const MMDB_DownloadLink = "https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country.mmdb"

var (
	the_geoipdb *maxminddb.Reader
	embedGeoip  bool

	GeoipFileName string
)

func HasEmbedGeoip() bool {
	return embedGeoip
}

func loadMaxmindGeoipBytes(bs []byte) {
	db, err := maxminddb.FromBytes(bs)
	if err != nil {
		log.Println("loadMaxmindGeoipBytes err,", err)
		return
	}
	the_geoipdb = db
}

//将一个外部的文件加载为我们默认的 geoip文件;若fn==""，则会自动使用 GeoipFileName 的值
func LoadMaxmindGeoipFile(fn string) {
	if fn == "" {
		fn = GeoipFileName
	}
	if fn == "" { //因为 GeoipFileName 是公有变量，所以可能会被设成""
		return
	}
	bs, e := os.ReadFile(utils.GetFilePath(fn))
	if e != nil {
		log.Printf("LoadMaxmindGeoipFile err, %s\n", e.Error())
		return
	}
	loadMaxmindGeoipBytes(bs)

}

//使用默认的 geoip文件，会调用 GetIP_ISO_byReader
func GetIP_ISO(ip net.IP) string {
	if the_geoipdb == nil {
		return ""
	}
	return GetIP_ISO_byReader(the_geoipdb, ip)
}

//返回 iso 3166 字符串 ，大写，两字节， 见 https://dev.maxmind.com/geoip/legacy/codes?lang=en
func GetIP_ISO_byReader(db *maxminddb.Reader, ip net.IP) string {

	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}

	err := db.Lookup(ip, &record)
	if err != nil {

		if utils.ZapLogger != nil {
			if ce := utils.CanLogErr("GetIP_ISO_byReader db.Lookup err"); ce != nil {
				ce.Write(zap.Error(err))
			}
		}

		return ""
	}
	return record.Country.ISOCode
}
