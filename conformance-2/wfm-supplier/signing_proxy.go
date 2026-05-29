package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	margocrypto "github.com/margo/sandbox/shared-lib/crypto"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18082", "address for the local proxy")
	targetRaw := flag.String("target", "", "target WFM base URL, e.g. https://symphony.machine:8082/v1alpha2/margo")
	keyPath := flag.String("key", "", "device private key path")
	insecure := flag.Bool("insecure", true, "skip upstream TLS verification")
	flag.Parse()

	if *targetRaw == "" || *keyPath == "" {
		log.Fatal("target and key are required")
	}

	target, err := url.Parse(strings.TrimRight(*targetRaw, "/"))
	if err != nil {
		log.Fatalf("invalid target URL: %v", err)
	}

	signer, err := margocrypto.NewSignerFromFile(*keyPath, "ecdsa", "sha256", "structured")
	if err != nil {
		log.Fatalf("failed to create signer: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	if *insecure {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	proxyBasePath := target.EscapedPath()
	if proxyBasePath == "" {
		proxyBasePath = "/"
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body.Close()

		suffix := r.URL.EscapedPath()
		if strings.HasPrefix(suffix, proxyBasePath) {
			suffix = strings.TrimPrefix(suffix, proxyBasePath)
		}
		if suffix == "" || !strings.HasPrefix(suffix, "/") {
			suffix = "/" + suffix
		}

		upstream := *target
		upstream.Path = strings.TrimRight(target.Path, "/") + suffix
		upstream.RawPath = ""
		upstream.RawQuery = r.URL.RawQuery

		outReq, err := http.NewRequestWithContext(context.Background(), r.Method, upstream.String(), bytes.NewReader(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for name, values := range r.Header {
			lower := strings.ToLower(name)
			if lower == "host" || lower == "content-length" || lower == "signature" || lower == "signature-input" || lower == "content-digest" {
				continue
			}
			for _, value := range values {
				outReq.Header.Add(name, value)
			}
		}
		if len(body) > 0 && outReq.Header.Get("Content-Type") == "" {
			outReq.Header.Set("Content-Type", "application/json")
		}

		if err := signer.SignRequest(context.Background(), outReq); err != nil {
			http.Error(w, fmt.Sprintf("failed to sign request: %v", err), http.StatusInternalServerError)
			return
		}

		resp, err := client.Do(outReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for name, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})

	log.Printf("signing proxy listening on http://%s and forwarding to %s", *listen, target.String())
	if err := http.ListenAndServe(*listen, handler); err != nil {
		log.Fatal(err)
	}
}
