package main

import (
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var timeoutOffset = time.Millisecond * 250 // how long we wait until we abort a `ping` request

func assertRoot() {
	if os.Geteuid() != 0 {
		fmt.Println("This program must be run as root")
		os.Exit(1)
	}
}

func getIpRange(cidr string) ([]net.IP, error) {
	ipRange := []net.IP{}

	// Parse the CIDR notation
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return ipRange, err
	}

	// Get the first IP address in the range (network address)
	startIP := network.IP

	// Calculate the last IP address in the range (broadcast address)
	// The broadcast address is the last IP in the subnet
	endIP := make(net.IP, len(startIP))
	copy(endIP, startIP)

	currentIP := make(net.IP, len(startIP))
	copy(currentIP, startIP)

	// calculate the last IP in the range
	for i := 0; i < len(endIP); i++ {
		endIP[i] |= ^network.Mask[i]
	}

	// Build an array of all ips in the range
	for !currentIP.Equal(endIP) {
		coppiedIP := make(net.IP, len(currentIP))
		copy(coppiedIP, currentIP)
		ipRange = append(ipRange, coppiedIP) // Append the incremented IP
		incIP(currentIP)
	}

	return ipRange, nil
}

func getLocalIP(ifaceName string) (string, error) {
	// Get a list of all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("Error getting interfaces:", err)
		return "", err
	}

	for _, iface := range interfaces {
		// Skip down interfaces and loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		if ifaceName == "" || !strings.Contains(iface.Name, ifaceName) {
			continue
		}

		// Get the addresses associated with the interface
		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println("Error getting addresses:", err)
			continue
		}

		for _, addr := range addrs {
			// Check if the address is an IP address
			if ipNet, ok := addr.(*net.IPNet); ok {
				// Check if the address is IPv4
				if ipNet.IP.To4() != nil {
					return ipNet.IP.String(), nil
				}
			}
		}
	}
	return "", fmt.Errorf("No IPv4 address found for interface %s", ifaceName)
}

func getCIDR(ip string) (string, error) {
	ipArr := strings.Split(ip, ".")
	if len(ipArr) != 4 {
		return "", fmt.Errorf("Invalid IP address: %s", ip)
	}
	return fmt.Sprintf("%s.%s.%s.0/24", ipArr[0], ipArr[1], ipArr[2]), nil
}

func ping(addr string, id int) (int, error) {
	// Resolve the address
	raddr, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		return -1, fmt.Errorf("Failed to resolve address: %s", err)
	}

	// Create a new ICMP message
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return -1, fmt.Errorf("Failed to listen on ICMP: %s", err)
	}
	defer c.Close()

	// Prepare the ICMP message
	seq := 1 // Start with sequence number 1
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte("HELLO-R-U-OK?"),
		},
	}

	// Marshal the message into bytes
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return -1, fmt.Errorf("Failed to marshal ICMP message: %s", err)
	}

	// Send the ICMP message
	_, err = c.WriteTo(msgBytes, raddr)
	if err != nil {
		return -1, fmt.Errorf("Failed to send ICMP message: %s", err)
	}

	// Read the reply
	reply := make([]byte, 1500)
	c.SetDeadline(time.Now().Add(timeoutOffset))
	n, _, err := c.ReadFrom(reply)
	if err != nil {
		if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			// Handle timeout as a ping failure
			return -1, fmt.Errorf("Ping timeout")
		}
		return -1, fmt.Errorf("Failed to read ICMP reply: %s", err)
	}

	// Parse the reply
	rm, err := icmp.ParseMessage(1, reply[:n])
	if err != nil {
		return -1, fmt.Errorf("Failed to parse ICMP reply: %s", err)
	}

	if rm.Type == ipv4.ICMPTypeEchoReply {
		echoReply := rm.Body.(*icmp.Echo)
		// check if the reply is the one we are looking for
		if string(echoReply.Data) == "HELLO-R-U-OK?" {
			return echoReply.ID, nil
		}
	}

	return -1, fmt.Errorf("No valid reply")
}

func incIP(ip net.IP) net.IP {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] > 0 {
			break
		}
	}
	return ip
}

func alreadyInArray(ip net.IP, list *[]net.IP) bool {
	for _, aip := range *list {
		if aip.String() == ip.String() {
			return true
		}
	}
	return false
}

func pingRange(ipRange []net.IP, retries int) []net.IP {
	aliveIps := []net.IP{}
	var mu sync.Mutex                   // To safely append to aliveIps
	limiter := make(chan struct{}, 256) // int here is the number of concurrent pings or 'threads'

	var wg sync.WaitGroup // WaitGroup to wait for all goroutines to finish

	for i := 0; i < retries; i++ {
		for id, ip := range ipRange {

			if alreadyInArray(ip, &aliveIps) {
				continue
			}

			limiter <- struct{}{} // Acquire a token
			wg.Add(1)             // Increment the WaitGroup counter

			go func(ip net.IP) {
				defer wg.Done()              // Decrement the counter when the goroutine completes
				defer func() { <-limiter }() // Release the token

				// fmt.Println("Pinging", ip.String(), "...")
				repliedIdx, _ := ping(ip.String(), id)
				if repliedIdx != -1 {
					// Got a valid reply from one of the ICMP packets...
					mu.Lock() // Lock to safely append to the slice
					defer mu.Unlock()

					if repliedIdx >= len(ipRange)-1 {
						fmt.Println("repliedIdx out of range")
						fmt.Println("Is something else sending ICMP packets to this machine?")
						return
					}

					sendingIp := ipRange[repliedIdx]

					if !alreadyInArray(sendingIp, &aliveIps) {
						aliveIps = append(aliveIps, sendingIp)
					}
				}
			}(ip)
		}
		time.Sleep(50 * time.Millisecond)
	}

	wg.Wait()      // Wait for all goroutines to finish
	close(limiter) // Close the limiter channel
	return aliveIps
}

func ipToInt(ip net.IP) int {
	// This is used for easily sorting the IPs

	// Remove the dots from the IP address
	ipStr := ip.String()
	ipStr = strings.ReplaceAll(ipStr, ".", "")
	// Convert the IP address to an integer
	ipInt := 0
	fmt.Sscanf(ipStr, "%d", &ipInt)
	return ipInt
}

func main() {
	assertRoot()

	ip, err := getLocalIP("en0")
	if err != nil {
		fmt.Println("Error getting local IP:", err)
	}

	cidr, err := getCIDR(ip)
	if err != nil {
		fmt.Println("Error getting CIDR:", err)
	}

	possibleIps, err := getIpRange(cidr)
	if err != nil {
		fmt.Println("Error enumerating possible IP range:", err)
		fmt.Println("This program must be run on a /24 network")
	}

	aliveIPs := pingRange(possibleIps, 5)
	sort.Slice(aliveIPs, func(i, j int) bool {
		left := ipToInt(aliveIPs[i])
		right := ipToInt(aliveIPs[j])
		return left < right
	})

	for _, ip := range aliveIPs {
		fmt.Println(ip, "is alive")
	}
}
