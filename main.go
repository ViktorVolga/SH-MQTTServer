package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"

	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"sync"
	"time"

	zmq "github.com/go-zeromq/zmq4"
)

const (
	poolSize     = 10
	serverAddr   = "192.168.88.219:8443"
	clientCount  = 5
	messageCount = 3
)

// Генерирует самоподписанный сертификат
func generateSelfSignedCert() (tls.Certificate, error) {
	// Генерируем приватный ключ
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Шаблон сертификата
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // 1 год
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	// Создаём DER-кодированный сертификат
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Кодируем в PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// Создаём tls.Certificate
	return tls.X509KeyPair(certPEM, keyPEM)
}

func runServer() {
	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatal("Не удалось создать сертификат:", err)
	}

	config := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", serverAddr, config)
	if err != nil {
		log.Fatal("Starting listener error:", err)
	}
	defer listener.Close()

	log.Printf("Server running on %s (TLS)", serverAddr)

	connections := make(chan net.Conn, poolSize)

	for i := 0; i < poolSize; i++ {
		go worker(connections)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Ошибка Accept:", err)
			continue
		}
		connections <- conn
	}
}

func worker(connections <-chan net.Conn) {
	for conn := range connections {
		handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		message, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("timeout %s — close", conn.RemoteAddr())
			} else {
				log.Printf("reading error from %s: %v", conn.RemoteAddr(), err)
			}
			break
		}
		fmt.Printf("Server ← [%s]: %s", conn.RemoteAddr(), message)
	}
}

func runClient() {
	zmqEndpoint := "ipc:///temperature"
	ctx := context.Background()
	socket := zmq.NewSub(ctx)
	defer socket.Close()

	if err := socket.Dial(zmqEndpoint); err != nil {
		log.Fatal("ZMQ dial error:", err)
	}

	if err := socket.SetOption(zmq.OptionSubscribe, ""); err != nil {
		log.Fatal("ZMQ subscribe error:", err)
	}

	log.Printf("ZMQ SUB подключён к %s", zmqEndpoint)

	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	var wg sync.WaitGroup

	messageChan := make(chan []byte)

	wg.Add(1)
	go func() {
		defer wg.Done()
		println("before dial")
		dialer := &net.Dialer{
			Timeout: 5 * time.Second, // Таймаут 5 секунд
		}

		for {
			conn, err := tls.DialWithDialer(dialer, "tcp", serverAddr, config)
			if err != nil {
				log.Printf("Connection error: %v", err)
			}
			defer conn.Close()
			if conn != nil && conn.ConnectionState().HandshakeComplete {
				time.Sleep(time.Second)
				for data := range messageChan {
					_, err := conn.Write(data)
					if err != nil {
						log.Printf("sending data error: %v\n", err)
						break
					}
					fmt.Printf("Temperature was send to the cloud\n")
				}
				continue
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			log.Println("started go rutine")
			msg, err := socket.Recv()
			if err != nil {
				log.Println("ZMQ recv error:", err)
				time.Sleep(1 * time.Second)
				continue
			}
			messageChan <- msg.Bytes()
		}
	}()

	wg.Wait()
	//log.Println("all clients end work")

	//select {}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go [server|client]")
		os.Exit(1)
	}

	mode := os.Args[1]

	switch mode {
	case "server":
		runServer()
	case "client":
		runClient()
	default:
		fmt.Println("Usage: server or client")
		os.Exit(1)
	}
}
