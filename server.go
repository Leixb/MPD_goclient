package main

import (
	"github.com/akamensky/argparse"
	"github.com/gin-gonic/gin"
	"github.com/leixb/mpdconn"

	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"context"
	"os/signal"
	"syscall"
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

	parser := argparse.NewParser("", "HTML MPD Client")

	port := parser.Int("p", "port", &argparse.Options{
		Help:    "Port where the interface will be served",
		Default: 8080,
	})

	MPDConnLocation := parser.String("m", "mpd-conn", &argparse.Options{
		Help:    "Where is the mpd server located",
		Default: "localhost:6600",
	})

	debug := parser.Flag("d", "debug", &argparse.Options{
		Help: "Run in debug mode",
	})

	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
	}

	if !*debug {
		gin.SetMode("release")
		gin.DefaultWriter = ioutil.Discard
	}

	r := gin.Default()

	MPDConn, err := mpdconn.NewMPDconn(*MPDConnLocation)
	if err != nil {
		log.Fatal(err)
	}

	coverFile, err := ioutil.TempFile("", "cover")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(coverFile.Name())

	b := NewBroker()
	go b.Start()
	go UpdateAlbum(MPDConn, b, coverFile)

	DownloadCover(MPDConn, coverFile)

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

	r.GET("/assets/*a", func(c *gin.Context) {
		data, err := Asset("static_files/assets" + c.Param("a"))
		if err != nil {
			log.Println(err)
			return
		}
		c.Data(http.StatusOK, "", data)
	})

	r.StaticFile("/cover", coverFile.Name())

	r.GET("/", func(c *gin.Context) {
		data, err := Asset("static_files/index.html")
		if err != nil {
			log.Println(err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		data, err := Asset("static_files/favicon.ico")
		if err != nil {
			log.Println(err)
			return
		}
		c.Data(http.StatusOK, "image/x-icon", data)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: r,
	}

	log.Printf("Serving on: localhost:%d", *port)

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shuting down...")

	log.Println("Removing temp files...")
	os.Remove(coverFile.Name())

	log.Println("Closing MPD connection...")
	MPDConn.Close()

	log.Println("Closing server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		log.Println("timeout of 5 seconds.")
	}
	log.Println("Done")
}

func UpdateAlbum(MPDConn *mpdconn.MPDconn, b *Broker, file *os.File) error {
	for {
		// Wait for MPD to communicate change
		_, err := MPDConn.Request("idle player")
		if err != nil {
			return err
		}

		err = DownloadCover(MPDConn, file)
		if err != nil {
			return err
		}

		// Send broadcast to all /sse clients
		b.Publish("player update")
	}
}

func DownloadCover(MPDConn *mpdconn.MPDconn, file *os.File) error {

	// Get current song info
	data, err := MPDConn.Request("currentsong")
	if err != nil {
		return err
	}

	// Download new cover
	err = MPDConn.DownloadCover(data["file"], file)
	if err != nil {
		return err
	}
	return nil
}
