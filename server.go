package main

import (
	"github.com/Leixb/mpdconn"

	"github.com/akamensky/argparse"
	"github.com/gin-gonic/gin"
	"github.com/zserge/webview"
	"github.com/grafov/bcast"

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

type appContext struct {
	updateGroup *bcast.Group
	stop        chan bool
	MPDConn     *mpdconn.MpdConn
	coverFile   *os.File
}

func main() {

	group := bcast.NewGroup()
	go group.Broadcast(0)
	app := appContext{group, make(chan bool), nil, nil}

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
			c.SSEvent("message", event)
			return true
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

	go webview.Open("MPD client", "http://localhost:8080", 800, 600, true)

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shuting down...")

	app.stop <- true

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

func (app *appContext) updateAlbum() error {
	update := make(chan bool)
	go func() {
		for {
			_, err := app.MPDConn.Request("idle player")
			if err != nil {
				fmt.Println(err)
			}
			update <- true
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
			app.updateGroup.Send("player update")
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
