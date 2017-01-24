package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
)

func main() {
	listen := "127.0.0.1:9158"
	apikey := ""
	keyfile := ""
	format := "deb"

	flag.StringVar(&listen, "listen", listen, "HTTP interface")
	flag.StringVar(&apikey, "key", apikey, "HTTP API Key")
	flag.StringVar(&keyfile, "keyfile", keyfile, "HTTP API Key file")
	flag.StringVar(&format, "format", format, "Package format to serve")
	flag.Parse()

	if apikey == "" && keyfile != "" {
		apikey, err := keyFromFile(keyfile)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
	} else if apikey == "" {
		apikey = randToken()
		log.Printf("APIKey: %v", apikey)
	}

	apiHandler := &API{
		Key:    apikey,
		Format: format,
		Files:  http.FileServer(http.Dir(".")),
	}

	http.Handle("/", apiHandler)
	http.ListenAndServe(listen, nil)
}

func keyFromFile(keyfile string) (string, error) {
	data, err := ioutil.ReadFile(keyfile)
	apikey := string(data)
	if (err != nil && os.IsNotExist(err)) || apikey == "" {
		apikey = randToken()
		err = ioutil.WriteFile(keyfile, []byte(apikey), 0600)
	}
	return apikey, err
}

func randToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

type API struct {
	Key    string
	Format string
	Files  http.Handler
}

func (api *API) checkKey(res http.ResponseWriter, req *http.Request) bool {
	if req.Header.Get("APIKey") == api.Key {
		return true
	}

	res.WriteHeader(http.StatusForbidden)
	fmt.Fprint(res, "Forbidden")
	return false
}

func (api *API) handleUpload(res http.ResponseWriter, req *http.Request) {
	f, err := os.Create(req.URL.Path)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(res, "%v")
		return
	}
	defer f.Close()
	_, err = io.Copy(f, req.Body)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}
}

func (api *API) handleRelease(res http.ResponseWriter, req *http.Request) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(res, "%v")
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()

	cwd, err := os.Getwd()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(res, err.Error())
		return
	}

	from := req.URL.Query().Get("from")

	srcPath := path.Clean("./" + req.URL.Path)
	cmd := exec.Command("fprepo-"+api.Format, path.Base(from))
	cmd.Dir = path.Join(cwd, path.Join(path.Dir(srcPath), path.Base(from)))
	cmd.Stderr = f
	cmd.Stdout = f
	f.Seek(0, 0)

	err = cmd.Run()
	if err != nil {
		res.Header().Set("ExitStatus", err.Error())
		res.WriteHeader(http.StatusInternalServerError)
	}

	d, err := ioutil.TempDir(path.Dir(srcPath), path.Base(srcPath))
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(res, err.Error())
		return
	}
	defer os.RemoveAll(d)

	err = os.Symlink(from, path.Join(d, path.Base(srcPath)))
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(res, err.Error())
		return
	}

	err = os.Rename(path.Join(d, path.Base(srcPath)), srcPath)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(res, err.Error())
		return
	}

	res.WriteHeader(http.StatusOK)

	_, err = io.Copy(res, f)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}
}

func (api *API) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	log.Printf("%v %v", req.Method, req.URL.Path)
	if req.Method == "PUT" {
		if api.checkKey(res, req) {
			if strings.HasSuffix(req.URL.Path, "/") {
				api.handleRelease(res, req)
			} else {
				api.handleUpload(res, req)
			}
		}
	} else {
		if req.URL.Path != "" && req.URL.Path != "/" &&
			(len(req.URL.Path) == 0 || strings.Index(req.URL.Path[1:], "/") == -1) {
			if !api.checkKey(res, req) {
				return
			}
		}
		api.Files.ServeHTTP(res, req)
	}
}
