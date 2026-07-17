// loadtest — tinyredis 서버에 TCP 연결을 N개 열고, 지정한 시간 동안 유지한다.
//
// 목적: "연결을 유지하는 동안" 서버가 얼마나 자원을 쓰는지 밖에서 측정하기 위한 도구다.
// 연결을 열고 바로 닫으면 순간 처리량(throughput)만 보게 되지만,
// C10K 의 본질은 "동시에 열려 있는 연결 수"이므로 연결을 붙잡고 있는 게 핵심이다.
//
// 사용:
//
//	go run ./cmd/loadtest -n 10000 -hold 30s
//	(연결이 열려 있는 30초 동안, 다른 터미널에서 서버 RSS·goroutine·fd 를 측정한다)
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:6379", "서버 주소")
	n := flag.Int("n", 1000, "열고 유지할 연결 수")
	hold := flag.Duration("hold", 30*time.Second, "연결을 유지할 시간 (측정할 시간)")
	ping := flag.Bool("ping", true, "연결마다 한 줄 보내고 에코를 받아 살아있음을 확인")
	flag.Parse()

	// 열린 연결을 슬라이스에 담아둔다. 여기서 참조를 놓지 않아야 GC 도, close 도 안 되고 계속 살아있다.
	conns := make([]net.Conn, 0, *n)
	failed := 0

	start := time.Now()
	for i := 0; i < *n; i++ {
		c, err := net.Dial("tcp", *addr)
		if err != nil {
			failed++
			// 첫 실패 메시지는 그대로 보여준다. 대개 fd 한도(ulimit) 또는 포트 고갈(포트 부족)이다.
			if failed <= 3 {
				fmt.Fprintf(os.Stderr, "  dial #%d 실패: %v\n", i, err)
			}
			continue
		}
		if *ping {
			// "ping\n" 을 보내면 서버가 대문자로 바꿔 "PING\n" 을 돌려준다. 왕복이 되면 연결이 진짜 살아있는 것.
			fmt.Fprintf(c, "ping\n")
			if _, err := bufio.NewReader(c).ReadString('\n'); err != nil {
				failed++
				c.Close()
				continue
			}
		}
		conns = append(conns, c)
		if (i+1)%1000 == 0 {
			fmt.Printf("  %d개 연결 (경과 %s)\n", i+1, time.Since(start).Round(time.Millisecond))
		}
	}

	fmt.Printf("\n열린 연결: %d개 / 목표 %d개, 실패: %d개, 소요: %s\n",
		len(conns), *n, failed, time.Since(start).Round(time.Millisecond))
	fmt.Printf("지금부터 %s 동안 연결을 유지한다. 이 사이에 다른 터미널에서 서버를 측정하라.\n", *hold)
	fmt.Println("  goroutine: curl -s localhost:6060/debug/pprof/goroutine?debug=1 | head -1")
	fmt.Println("  RSS(KB)  : ps -o rss= -p $(pgrep -f tinyredis)")
	fmt.Println("  fd 수    : lsof -p $(pgrep -f tinyredis) | wc -l")

	time.Sleep(*hold)

	for _, c := range conns {
		c.Close()
	}
	fmt.Println("연결 모두 닫음. 종료.")
}
