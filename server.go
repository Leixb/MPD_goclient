package main

import (
	"github.com/gin-gonic/gin"
	"github.com/leixb/mpdconn"
	"io"
	"log"
	"net/http"
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

	MPDConn, err := mpdconn.NewMPDconn("localhost:6600")
	if err != nil {
		log.Fatal(err)
	}

	b := NewBroker()
	go b.Start()
	go UpdateAlbum(MPDConn, b)

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

	r.StaticFile("/", "./web.html")
	r.StaticFile("/cover", "./cover")
	r.StaticFile("/web.js", "./web.js")
	r.StaticFile("/style.css", "./style.css")
	r.Static("/assets/", "./assets")

	r.Run() // listen and serve on 0.0.0.0:8080
}
