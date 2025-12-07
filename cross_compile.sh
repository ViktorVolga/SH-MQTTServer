#!/bin/bash

#go get github.com/eclipse/paho.mqtt.golang

export GOARCH=arm
export GOARM=7  # Для ARMv7
export CGO_ENABLED=1  # Включите CGO, если используете системные библиотеки
export CC=arm-linux-gnueabihf-gcc  # Укажите кросс-компилятор для ARM

#go get github.com/go-zeromq/zmq4

go build -o sh-device-server main.go

echo "binary compiled - deliver it to yocto"
