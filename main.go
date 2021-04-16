package main

import (
	"log"
	_ "net/http/pprof"
	"time"

	"github.com/chindeo/rtsp2hls/rtsp"
)

func main() {
	go func() {
		if err := rtsp.GetServer().Start(); err != nil {
			log.Println("start rtsp server error", err)
		}
		log.Println("rtsp server end")
	}()

	url := "rtsp://admin:P@ssw0rd@10.0.0.10:554/Streaming/Channels/102"
	heartbeatInterval := 0
	customPath := "/test"
	idleTimeout := 10
	agent := "EasyDarwinGo/v8.1"

	client, err := rtsp.NewRTSPClient(rtsp.GetServer(), url, int64(heartbeatInterval)*1000, agent, customPath)
	if err != nil {
		log.Printf("new rtsp client: %v", err)
	}

	pusher := rtsp.NewClientPusher(client)

	err = client.Start(time.Duration(idleTimeout) * time.Second)
	if err != nil {
		if len(client.NewURL) > 0 && client.URL != client.NewURL {
			client.URL = client.NewURL
			err = client.Start(time.Duration(idleTimeout) * time.Second)
			if err != nil {
				log.Printf("new rtsp client: %v", err)
			}
		}
	}

	rtsp.GetServer().AddPusher(pusher)

	select {}
}
