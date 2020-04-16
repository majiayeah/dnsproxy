package proxy

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/AdguardTeam/golibs/log"
)

type pingResult struct {
	addr        net.IP
	exres       *upstream.ExchangeAllResult
	err         error
	latencyMsec uint
}

// pingDoTCP connects to a remote address via TCP and then send signal to the channel
func (f *FastestAddr) pingDoTCP(addr net.IP, tcpPort uint, exres *upstream.ExchangeAllResult, ch chan *pingResult) {
	res := &pingResult{}
	res.addr = addr
	res.exres = exres

	qName := res.exres.Resp.Question[0].Name
	a := net.JoinHostPort(addr.String(), strconv.Itoa(int(tcpPort)))
	log.Debug("%s: Connecting to %s via TCP", qName, a)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", a, tcpTimeout*time.Millisecond)
	if err != nil {
		res.err = fmt.Errorf("%s: no reply from %s", qName, addr)
		log.Debug("pingDoTCP error: %v", res.err)

		f.cacheAddFailure(res.addr)

		ch <- res
		return
	}
	elapsedMsec := uint(time.Since(start).Milliseconds())
	log.Debug("%s: Elapsed on %s - %d", qName, a, elapsedMsec)

	res.latencyMsec = elapsedMsec
	_ = conn.Close()

	f.cacheAddSuccessful(res.addr, res.latencyMsec)

	ch <- res
}

// Wait for the first successful ping result
func (f *FastestAddr) pingWait(total int, ch chan *pingResult) (fastestAddrResult, error) {
	result := fastestAddrResult{}
	n := 0
	for {
		select {
		case res := <-ch:
			n++

			if res.err != nil {
				break
			}

			proto := "tcp"
			log.Debug("%s: Determined %s address as the fastest (%s, %dms)",
				res.exres.Resp.Question[0].Name, res.addr, proto, res.latencyMsec)

			result.res = res.exres
			result.ip = res.addr
			result.latency = res.latencyMsec
			return result, nil
		}

		if n == total {
			return result, fmt.Errorf("all ping tasks timed out")
		}
	}
}