package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"remote_reality/modules/camera"
	"remote_reality/modules/servo"
	"strings"
	"time"

	"github.com/blackjack/webcam"
	"github.com/stianeikeland/go-rpio/v4"
)

var xServo, yServo servo.Servo
var isRaspberry = true
var dev camera.Device

func main() {
	// Немножко CLI
	device := flag.String("d", "/dev/video0", "video device to use")
	info := flag.Bool("i", false, "check available formats and frame sizes")
	tls := flag.Bool("t", false, "use tls")
	videoFormat := flag.String("f", "", "video format to use, default first supported")
	frameSize := flag.String("s", "", "frame size to use, default largest one")
	addr := flag.String("l", ":8000", "addr to listien")
	fps := flag.Bool("p", false, "print fps info")
	flag.Parse()

	dev.InitCamera(device, videoFormat)
	defer dev.Cam.Close()
	if *info {
		dev.PrintSupported()
		return
	}
	if err := dev.CheckSizeAndRequest(frameSize); err != nil {
		log.Fatal(err)
	}

	// Запуск стрима
	if err := dev.Cam.StartStreaming(); err != nil {
		log.Println(err)
		return
	}

	var (
		li   chan *bytes.Buffer = make(chan *bytes.Buffer)
		fi   chan []byte        = make(chan []byte)
		back chan struct{}      = make(chan struct{})
	)
	go camera.EncodeToImage(dev.Cam, back, fi, li, dev.Width, dev.Height, dev.CurFormat)
	go InitServer(*addr, *tls, li)
	port := strings.Split(*addr, ":")[1]
	fmt.Printf("Stream started at %s:%s\n", GetLocalIP(), port)

	// Запуск сервоприводов
	ServoHeadInit()
	defer rpio.Close()

	timeout := uint32(5) // 5-секундный таймаут
	start := time.Now()
	var fr time.Duration

	for {
		err := dev.Cam.WaitForFrame(timeout)

		switch err.(type) {
		case nil:
		case *webcam.Timeout:
			log.Println(err)
			continue
		default:
			log.Fatal(err)
		}

		frame, err := dev.Cam.ReadFrame()
		if err != nil {
			log.Fatal(err)
		}
		if len(frame) != 0 {

			// Печатаем FPS каждые 10 секунд
			fr++
			if *fps {
				if d := time.Since(start); d > time.Second*10 {
					fmt.Println(float64(fr)/(float64(d)/float64(time.Second)), "fps")
					start = time.Now()
					fr = 0
				}
			}

			select {
			case fi <- frame:
				<-back
			default:
			}
		}
	}
}

func ServoHeadInit() {
	if err := rpio.Open(); err != nil {
		if err.Error() == "open /dev/gpiomem: no such file or directory" {
			isRaspberry = false
			fmt.Println("It is hot Raspberry, Servo disabled")
		} else {
			log.Fatal(err)
		}
	}

	if isRaspberry {
		xServo.Init(18)
		yServo.Init(19)
		fmt.Println("It is Raspberry, Servo ready")
	}
}
