package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"remote_reality/modules/camera"
	"strings"
	"time"

	"github.com/blackjack/webcam"
)

func main() {
	// Немножко CLI
	device := flag.String("d", "/dev/video0", "video device to use")
	info := flag.Bool("i", false, "check available formats and frame sizes")
	videoFormat := flag.String("f", "", "video format to use, default first supported")
	frameSize := flag.String("s", "", "frame size to use, default largest one")
	addr := flag.String("l", ":8000", "addr to listien")
	fps := flag.Bool("p", false, "print fps info")
	flag.Parse()

	var dev camera.Device
	// Запуск стрима
	if err := camera.BeginStream(dev, device, videoFormat, info, frameSize); err != nil {
		log.Fatal(err)
	}

	var (
		li   chan *bytes.Buffer = make(chan *bytes.Buffer)
		fi   chan []byte        = make(chan []byte)
		back chan struct{}      = make(chan struct{})
	)
	go camera.EncodeToImage(dev, back, fi, li)
	go InitServer(*addr, li)
	port := strings.Split(*addr, ":")[1]
	fmt.Printf("Stream started at %s:%s\n", GetLocalIP(), port)

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
