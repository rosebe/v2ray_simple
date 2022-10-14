package netLayer

import (
	"context"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/pires/go-proxyproto"
	"go.uber.org/zap"
)

var (
	//你可以通过向这个map插入 自定义函数的方式 来拓展 vs的 监听功能, 可以监听 其它 net包无法监听的 network
	CustomListenerMap = make(map[string]func(address string) (net.Listener, error))
)

func loopAccept(listener net.Listener, xver int, acceptFunc func(net.Conn)) {
	if xver > 0 {

		if ce := utils.CanLogDebug("Listening PROXY protocol"); ce != nil {
			ce.Write(zap.Int("preferred version", xver))
		}

		listener = &proxyproto.Listener{Listener: listener, Policy: proxyProtocolListenPolicyFunc}
	}

	var tooManyRetryCount time.Duration = 1

	for {
		newc, err := listener.Accept()
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "close") {
				if ce := utils.CanLogDebug("netLayer.loopAccept, listener got closed"); ce != nil {
					ce.Write(zap.Error(err))

				}
				break
			}
			if ce := utils.CanLogWarn("failed to accept connection"); ce != nil {
				ce.Write(zap.Error(err))
			}
			if strings.Contains(errStr, "too many") {
				if ce := utils.CanLogWarn("Too many incoming conns! Will Sleep."); ce != nil {
					ce.Write(zap.String("err", errStr), zap.Int64("tooManyRetryCount", int64(tooManyRetryCount)))

				}
				if tooManyRetryCount > 20 {
					utils.Fatal("Too many incoming conns for 20 times! we will exit program to prevent infinite loop.")
					break
				}
				time.Sleep(time.Millisecond * 500 * tooManyRetryCount)
				tooManyRetryCount++
				continue
			}
			//在 Close这个listener后，会遇到 EOF 错误，所以除了too many的特殊情况外，应该 直接退出才对.
			break
		}
		go acceptFunc(newc)
	}
}

/*
func loopAcceptUDP(uc net.UDPConn, acceptFunc func([]byte, *net.UDPAddr)) {
	for {
		p := utils.GetPacket()
		n, addr, err := uc.ReadFromUDP(p)
		if err != nil {
			if ce := utils.CanLogWarn("loopAcceptUDP failed to accept"); ce != nil {
				ce.Write(zap.Error(err))
			}
			break
		}
		go acceptFunc(p[:n], addr)
	}
}
*/

// ListenAndAccept 试图监听 tcp, udp 和 unix domain socket 这三种传输层协议.
//
// 非阻塞，在自己的goroutine中监听.
func ListenAndAccept(network, addr string, sockopt *Sockopt, xver int, acceptFunc func(net.Conn)) (listener net.Listener, err error) {
	if addr == "" || acceptFunc == nil {
		return nil, utils.ErrNilParameter
	}
	if network == "" {
		network = "tcp"
	}
	p := StrToTransportProtocol(network)

	switch p {
	case TCP:
		var tcplistener *net.TCPListener

		var ta *net.TCPAddr
		ta, err = net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return
		}

		tcplistener, err = net.ListenTCP(network, ta)
		if err != nil {
			return
		}

		if sockopt != nil {
			SetSockOptForListener(tcplistener, sockopt, false, ta.IP.To4() == nil)
		}

		go loopAccept(tcplistener, xver, acceptFunc)

		listener = tcplistener

	case UDP:

		//udp 的透明代理等设置sockopt的情况并不使用本函数监听, 而是使用 ListenUDP_withOpt.

		var ua *net.UDPAddr
		ua, err = net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return
		}

		listener, err = NewUDPListener(ua)
		if err != nil {
			return
		}
		go loopAccept(listener, xver, acceptFunc)

	case UNIX:
		// 参考 https://eli.thegreenplace.net/2019/unix-domain-sockets-in-go/
		//监听 unix domain socket后，就会自动创建 相应文件;
		// 而且程序退出后，该文件不会被删除
		//  而且再次启动后如果遇到了这个文件，就会报错，就像tcp端口已经被监听 的错误一样:
		// “bind: address already in use”
		// 所以必须把原文件删掉
		// 但是问题是，有可能被一些粗心的用户搞出大问题
		// 如果不小心设置成了 '/' 根目录，那我们删的话是不是会直接把所有文件都删掉了？
		// 总之RemoveAll函数千万不能用，Remove函数倒是没什么大事
		if utils.FileExist(addr) {

			if ce := utils.CanLogDebug("unix file exist"); ce != nil {
				ce.Write(zap.String("deleting", addr))
			}
			err = os.Remove(addr)
			if err != nil {
				err = utils.ErrInErr{ErrDesc: "Error when deleting previous unix socket file,", ErrDetail: err, Data: addr}
				return
			}

		}
		fallthrough
	default:

		if len(CustomListenerMap) > 0 {
			if f := CustomListenerMap[network]; f != nil {
				listener, err = f(addr)
				if err != nil {
					return
				}
			}
		}

		if listener == nil {
			listener, err = net.Listen(network, addr)
		}

		if err != nil {
			return
		}

		go loopAccept(listener, xver, acceptFunc)

	}
	return
}

func (a Addr) ListenUDP_withOpt(sockopt *Sockopt) (net.PacketConn, error) {
	var lc net.ListenConfig
	lc.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			SetSockOpt(int(fd), sockopt, true, a.IsIpv6())
		})
	}
	return lc.ListenPacket(context.Background(), "udp", a.String())
}
