package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
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
	noCam := flag.Bool("n", false, "disable camera")
	quality := flag.Int("q", 90, "quality of jpeg (0-100)")
	flag.Parse()

	var (
		imageStream chan *bytes.Buffer = make(chan *bytes.Buffer) // Картинки, что отправятся в стрим
		camStream   chan []byte        = make(chan []byte)        // Сырые данные с камеры
		back        chan struct{}      = make(chan struct{})
	)
	go InitServer(*addr, imageStream)
	port := strings.Split(*addr, ":")[1]
	fmt.Printf("Stream started at %s:%s\n", GetLocalIP(), port)
	if *noCam {
		blank, err := os.Open("./modules/site/blank.png")
		if err != nil {
			log.Fatal("error when opening blank", err)
		}
		defer blank.Close()

		img, _, err := image.Decode(blank)
		if err != nil {
			log.Fatal("error when decoding", err)
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			log.Fatal("error when encoding ", err)
		}
		imageStream <- &buf
		for {
			if len(imageStream) == 0 {
				imageStream <- &buf
			}
		}
	}

	var dev camera.Device
	// Подготовка камеры
	if err := camera.PrepareCamera(&dev, device, videoFormat, info, frameSize); err != nil {
		log.Fatal(err)
	}
	defer dev.Cam.Close()

	go camera.EncodeToImage(dev, back, camStream, imageStream, *quality)

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
			case camStream <- frame:
				<-back
			default:
			}
		}
	}
}
