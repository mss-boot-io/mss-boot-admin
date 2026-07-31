package server

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/11 9:42
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/11 9:42
 */

// Manage server
var Manage = New()

var Mux sync.Mutex

func PrintRunningInfo(address, protocol string) {
	Mux.Lock()
	defer Mux.Unlock()
	var port string
	fmt.Println("        \033[90m╔═════════════════════════════════════════════════════════╗\033[0m")
	fmt.Printf("        \033[90m║\033[0m %s listening at:                                      \033[90m║\033[0m", protocol)
	fmt.Println()
	arr := strings.Split(address, ":")
	if len(arr) <= 1 {
		port = "80"
	} else {
		port = arr[len(arr)-1]
	}
	hosts := []string{"localhost", "127.0.0.1"}
	if strings.Contains(address, "0.0.0.0") || !strings.Contains(address, ".") {
		interfaces, _ := net.Interfaces()
		for i := range interfaces {
			if interfaces[i].Flags&net.FlagUp == 0 {
				continue
			}
			addresses, _ := interfaces[i].Addrs()
			for _, addr := range addresses {
				ipNet, ok := addr.(*net.IPNet)
				if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
					hosts = append(hosts, ipNet.IP.String())
				}
			}
		}
	}
	prefix := "        "
	for i := range hosts {
		if i == 1 {
			prefix = "\033[32mready\033[0m - "
		}
		s := fmt.Sprintf("%s\033[90m║  >\033[0m   %s://%s:%s", prefix, protocol, hosts[i], port)
		fmt.Println(s + strings.Repeat(
			" ",
			51-len(fmt.Sprintf("%s://%s:%s", protocol, hosts[i], port)),
		) + "\033[90m║\033[0m")
		prefix = "        "
	}
	fmt.Println("        \033[90m║\033[0m" +
		"                                                         \033[90m║\033[0m")
	fmt.Print("        \033[90m║\033[0m \033[1;97mNow you can reqeust the above addresses↑\033[0m")
	fmt.Print("                ")
	fmt.Println("\033[90m║\033[0m")
	fmt.Println("        \033[90m╚═════════════════════════════════════════════════════════╝\033[0m")
}
