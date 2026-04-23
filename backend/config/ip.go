package config

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

func updatePublicIP(ctx context.Context, cfg *ALSConfig, wg *sync.WaitGroup) {
	log.Default().Println("Updating IP address from internet...")

	wg.Add(2)
	go func() {
		defer wg.Done()
		addr, err := getPublicIPv4ViaDNS()
		if err == nil {
			cfg.PublicIPv4 = addr
			log.Printf("Public IPv4 address: %s\n", addr)
			return
		}

		addr, err = getPublicIPv4ViaHttp()
		if err == nil {
			cfg.PublicIPv4 = addr
			log.Printf("Public IPv4 address: %s\n", addr)
			return
		}
	}()

	go func() {
		defer wg.Done()
		addr, err := getPublicIPv6ViaDNS()
		if err == nil {
			cfg.PublicIPv6 = addr
			log.Printf("Public IPv6 address: %s\n", addr)
			return
		}
	}()

	wg.Wait()
}

func getPublicIPv4ViaDNS() (string, error) {
	m := new(dns.Msg)
	m.SetQuestion("myip.opendns.com.", dns.TypeA)

	in, err := dns.Exchange(m, "resolver1.opendns.com:53")
	if err != nil {
		return "", err
	}

	if len(in.Answer) < 1 {
		return "", fmt.Errorf("no answer")
	}

	record, ok := in.Answer[0].(*dns.A)
	if !ok {
		return "", fmt.Errorf("not A record")
	}
	return record.A.String(), nil
}

func getPublicIPv6ViaDNS() (string, error) {
	m := new(dns.Msg)
	m.SetQuestion("myip.opendns.com.", dns.TypeAAAA)

	in, err := dns.Exchange(m, "resolver1.opendns.com:53")
	if err != nil {
		return "", err
	}

	if len(in.Answer) < 1 {
		return "", fmt.Errorf("no answer")
	}

	record, ok := in.Answer[0].(*dns.AAAA)
	if !ok {
		return "", fmt.Errorf("not A record")
	}

	return record.AAAA.String(), nil
}

func getPublicIPViaHttp(client *http.Client) (string, error) {
	lists := []string{
		"https://myexternalip.com/raw",
		"https://ifconfig.co/ip",
	}

	for _, url := range lists {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			if cerr := resp.Body.Close(); cerr != nil {
				return "", cerr
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			return "", err
		}

		addr := net.ParseIP(strings.TrimSpace(string(body)))
		if addr != nil {
			return addr.String(), nil
		}
	}

	return "", fmt.Errorf("no answer")
}

func getPublicIPv4ViaHttp() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "tcp4", addr)
			},
		},
	}
	return getPublicIPViaHttp(client)
}
