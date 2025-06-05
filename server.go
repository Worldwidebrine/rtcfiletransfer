package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

//go:embed www/*
var content embed.FS

func main() {
	listener, tcperr := net.Listen("tcp", ":0")
	if tcperr != nil {
		log.Fatal(tcperr)
	}
	url := fmt.Sprintf("http://localhost:%d/", listener.Addr().(*net.TCPAddr).Port)
	fmt.Println()
	fmt.Println(url)
	fmt.Println()
	assets, fserr := fs.Sub(content, "www")
	if fserr != nil {
		log.Fatal(fserr)
	}
	http.Handle("/", http.FileServer(http.FS(assets)))
	var filename *string = nil
	var fp *os.File = nil
	http.HandleFunc("/.offloadapi-init", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("offload api init")
		encoder := json.NewEncoder(w)
		if fp != nil {
			fmt.Println("file already opened")
			err := encoder.Encode(map[string]string{
				"status": "session already opened",
			})
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		decoder := json.NewDecoder(r.Body)
		var data struct {
			FileName string `json:"file_name"`
			FileType string `json:"file_type"`
		}
		err := decoder.Decode(&data)
		if err != nil {
			fmt.Println(err)
			err = encoder.Encode(map[string]string{
				"status": "failed to parse metadata",
				"result": err.Error(),
			})
			if err != nil {
				log.Fatal(err)
			}
			return
		}
		fmt.Println(data.FileType)
		fmt.Println(data.FileName)
		filename = (&(data.FileName))
		fp, err = os.OpenFile("./"+(*filename), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			fmt.Println(err)
			err = encoder.Encode(map[string]string{
				"status": "failed to open file",
				"result": err.Error(),
			})
			if err != nil {
				log.Fatal(err)
			}
			return
		}
		err = encoder.Encode(map[string]string{
			"status": "ok",
		})
		if err != nil {
			log.Fatal(err)
		}
	})
	http.HandleFunc("/.offloadapi", func(w http.ResponseWriter, r *http.Request) {
		encoder := json.NewEncoder(w)
		if fp == nil {
			fmt.Println("file not opened")
			w.WriteHeader(http.StatusInternalServerError)
			err := encoder.Encode(map[string]string{
				"result": "file not opened",
			})
			if err != nil {
				log.Fatal(err)
			}
			return
		}
		buffer, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			{
				err := encoder.Encode(map[string]string{
					"result": "failed to receive chunk",
				})
				if err != nil {
					log.Fatal(err)
				}
			}
			log.Fatal(err)
		}
		_, err = fp.Write(buffer)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			{
				err := encoder.Encode(map[string]string{
					"result": "failed to write chunk",
				})
				if err != nil {
					log.Fatal(err)
				}
			}
			log.Fatal(err)
		}
		err = encoder.Encode(map[string]string{
			"result": "ok",
		})
		if err != nil {
			log.Fatal(err)
		}
	})
	http.HandleFunc("/.offloadapi-close", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("offload api close")
		encoder := json.NewEncoder(w)
		if fp == nil {
			fmt.Println("file not opened")
			err := encoder.Encode(map[string]string{
				"result": "file not opened",
			})
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		file := fp
		fp = nil
		err := file.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Println(err)
			err := encoder.Encode(map[string]string{
				"result": "failed to close file",
			})
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		abspath, err := filepath.Abs(file.Name())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Println(err)
			err := encoder.Encode(map[string]string{
				"result": "failed to get absolute path",
			})
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		err = encoder.Encode(map[string]string{
			"result":    "ok",
			"file_path": abspath,
		})
		if err != nil {
			fmt.Println(err)
		}
	})
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd")
		stdin, err := cmd.StdinPipe()
		if err == nil {
			stdin.Write(fmt.Appendf(nil, "start \"\" \"%s?uselocalapi=true\"", url))
			stdin.Write(fmt.Appendln(nil))
			stdin.Write(fmt.Appendln(nil, "exit"))
			cmd.Run()
		}
	}
	httperr := http.Serve(listener, nil)
	if httperr != nil {
		log.Fatal(httperr)
	}
}
