package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	broker   = "ssl://test.mosquitto.org:8883" 
	clientID = "go-mqtt-tls-client"
	topic    = "test/topic"
	qos      = 1
)

var message = []byte("Hello from Go MQTT Client over TLS!")

func main() {
	tlsConfig := &tls.Config{ //tls settings
		InsecureSkipVerify: true, // only for testing
	}

	// client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetTLSConfig(tlsConfig)
	opts.SetAutoReconnect(true)

	// create client
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("connecting error: %v", token.Error())
	}

	token := client.Publish(topic, qos, false, message)
	token.Wait()
	if token.Error() != nil {
		log.Fatalf("message sending error: %v", token.Error())
	}
	fmt.Printf("Message '%s' sended on topic '%s' ower TLS\n", message, topic)

	time.Sleep(5 * time.Second)

	client.Disconnect(250)
	fmt.Println("Disconnected from mqtt")
}
