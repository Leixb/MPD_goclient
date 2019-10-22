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
	"syscall"
	"time"
)

type appContext struct {
	updateGroup *bcast.Group
}

func main() {

	group := bcast.NewGroup()
	go group.Broadcast(0)
	app := appContext{group}

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

	MPDConn, err := mpdconn.NewMpdConn(*MPDConnLocation)
	if err != nil {
		log.Fatal(err)
	}

	coverFile, err := ioutil.TempFile("", "cover")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(coverFile.Name())

	go app.updateAlbum(MPDConn, coverFile)

	downloadCover(MPDConn, coverFile)

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
		data, err := MPDConn.Request(c.Param("cmd"))

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

	r.StaticFile("/cover", coverFile.Name())

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
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shuting down...")

	log.Println("Removing temp files...")
	os.Remove(coverFile.Name())

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

func (app *appContext) updateAlbum(MPDConn *mpdconn.MpdConn, file *os.File) error {
	for {
		// Wait for MPD to communicate change
		_, err := MPDConn.Request("idle player")
		if err != nil {
			return err
		}

		err = downloadCover(MPDConn, file)
		if err != nil {
			return err
		}

		// Send broadcast to all /sse clients
		app.updateGroup.Send("player update")
	}
}

func downloadCover(MPDConn *mpdconn.MpdConn, file *os.File) error {

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
