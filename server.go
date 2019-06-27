package main

import (
	"github.com/gin-gonic/gin"
	"github.com/leixb/mpdconn"
	"io"
	"log"
	"net/http"
	//"strings"
	"time"
)

type Broker struct {
	stopCh    chan struct{}
	publishCh chan interface{}
	subCh     chan chan interface{}
	unsubCh   chan chan interface{}
}

func NewBroker() *Broker {
	return &Broker{
		stopCh:    make(chan struct{}),
		publishCh: make(chan interface{}, 1),
		subCh:     make(chan chan interface{}, 1),
		unsubCh:   make(chan chan interface{}, 1),
	}
}

func (b *Broker) Start() {
	subs := map[chan interface{}]struct{}{}
	for {
		select {
		case <-b.stopCh:
			return
		case msgCh := <-b.subCh:
			subs[msgCh] = struct{}{}
		case msgCh := <-b.unsubCh:
			delete(subs, msgCh)
		case msg := <-b.publishCh:
			for msgCh := range subs {
				// msgCh is buffered, use non-blocking send to protect the broker:
				select {
				case msgCh <- msg:
				default:
				}
			}
		}
	}
}

func (b *Broker) Stop() {
	close(b.stopCh)
}

func (b *Broker) Subscribe() chan interface{} {
	msgCh := make(chan interface{}, 5)
	b.subCh <- msgCh
	return msgCh
}

func (b *Broker) Unsubscribe(msgCh chan interface{}) {
	b.unsubCh <- msgCh
}

func (b *Broker) Publish(msg interface{}) {
	b.publishCh <- msg
}

func main() {
	r := gin.Default()

	//corsConfig := cors.DefaultConfig()
	//corsConfig.AllowOrigins = []string{"*"}
	//r.Use(cors.New(corsConfig))

	MPDConn, err := mpdconn.NewMPDconn("localhost:6600")
	if err != nil {
		log.Fatal(err)
	}

	data := map[string]interface{}{
		"message": "ping",
	}

	b := NewBroker()
	go b.Start()

	r.GET("/sse", func(c *gin.Context) {

		clientStream := b.Subscribe()
		defer b.Unsubscribe(clientStream)

		c.Stream(func(w io.Writer) bool {
			if msg, ok := <-clientStream; ok {
				c.SSEvent("message", msg)
				return true
			}
			return false
		})
	})

	r.GET("/ajax", func(c *gin.Context) {
		c.JSON(http.StatusOK, data)
	})
	r.GET("/JSONP", func(c *gin.Context) {
		c.JSONP(http.StatusOK, data)
	})
	r.GET("/slowQuery", func(c *gin.Context) {
		time.Sleep(5 * time.Second)
		c.JSON(http.StatusOK, data)
	})

	r.GET("/mpd/:cmd", func(c *gin.Context) {
		data, err := MPDConn.Request(c.Param("cmd"))

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err,
			})
		} else {
			c.JSON(http.StatusOK, data)
		}
	})

	go UpdateAlbum(MPDConn, b)

	r.StaticFile("/", "./web.html")
	r.StaticFile("/cover", "./cover")
	r.StaticFile("/web.js", "./web.js")
	r.StaticFile("/style.css", "./style.css")
	r.Static("/assets/", "./assets")
	r.Run() // listen and serve on 0.0.0.0:8080
}

func format_song(song []string) map[string]string {
	if len(song) < 4 {
		return map[string]string{
			"title":  "",
			"artist": "",
			"album":  "",
			"file":   "",
		}
	}
	return map[string]string{
		"title":  song[0],
		"artist": song[1],
		"album":  song[2],
		"file":   song[3],
	}
}

func UpdateAlbum(MPDConn *mpdconn.MPDconn, b *Broker) error {
	for {
		// Wait for MPD to communicate change
		_, err := MPDConn.Request("idle player")
		if err != nil {
			return err
		}

		// Get current song info
		data, err := MPDConn.Request("currentsong")
		if err != nil {
			return err
		}

		// Download new cover
		err = MPDConn.DownloadCover(data["file"], "cover")
		if err != nil {
			return err
		}

		// Send broadcast to all /sse clients
		b.Publish("player update")
	}
}
