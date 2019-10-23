package main

import (
	"github.com/Leixb/mpdconn"

	"github.com/akamensky/argparse"
	"github.com/gin-gonic/gin"
	"github.com/grafov/bcast"

	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"context"
	"os/signal"
	"time"
)

type appContext struct {
	updateGroup *bcast.Group
	stop        chan struct{}
	MPDConn     *mpdconn.MpdConn
	coverFile   *os.File
}

func main() {

	group := bcast.NewGroup()
	go group.Broadcast(0)
	app := appContext{group, make(chan struct{}), nil, nil}

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

	app.MPDConn, err = mpdconn.NewMpdConn(*MPDConnLocation)
	if err != nil {
		log.Fatal(err)
	}

	app.coverFile, err = ioutil.TempFile("", "cover")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(app.coverFile.Name())

	go app.updateAlbum()

	app.downloadCover()

	r.GET("/sse", func(c *gin.Context) {
		recv := app.updateGroup.Join()
		defer recv.Close()

		c.Stream(func(w io.Writer) bool {
			event := recv.Recv()
			if event == "update" {
				c.SSEvent("message", event)
				return true
			}
			return false
		})
	})

	r.GET("/mpd/:cmd", func(c *gin.Context) {
		data, err := app.MPDConn.Request(c.Param("cmd"))

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err,
			})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	r.GET("/assets/*a", func(c *gin.Context) {
		data, err := Asset("static_files/assets" + c.Param("a"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err,
			})
			return
		}
		c.Data(http.StatusOK, "", data)
	})

	r.StaticFile("/cover", app.coverFile.Name())

	r.GET("/", func(c *gin.Context) {
		data, err := Asset("static_files/index.html")
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err,
			})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		data, err := Asset("static_files/favicon.ico")
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err,
			})
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
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Shuting down...")

	app.stop <- struct{}{}       // Stop mpd monitor of changed
	app.updateGroup.Send("quit") // Stop sse connections

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
}

func (app *appContext) updateAlbum() error {
	update := make(chan struct{})
	go func() {
		for {
			_, err := app.MPDConn.Request("idle player")
			if err != nil {
				fmt.Println(err)
			}
			update <- struct{}{}
		}
	}()

	for {
		select {
		case <-update:
			err := app.downloadCover()
			if err != nil {
				fmt.Println(err)
			}
			// Send broadcast to all /sse clients
			app.updateGroup.Send("update")
		case <-app.stop:
			return nil
		}
	}

}

func (app *appContext) downloadCover() error {

	// Get current song info
	data, err := app.MPDConn.Request("currentsong")
	if err != nil {
		return err
	}

	// Download new cover
	err = app.MPDConn.DownloadCover(data["file"], app.coverFile)
	if err != nil {
		return err
	}
	return nil
}
