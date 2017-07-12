package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
	"strconv"
	"errors"
	"github.com/gorilla/mux"
	"html/template"
	"io/ioutil"
	"encoding/base64"
	"encoding/gob"
	"os"
	"bytes"
)

var store Data

//Data ...
type Data struct {
	initialized bool
	mtx         sync.Mutex
	Shorts      *map[string]string
}

func (d *Data) Init() {
	if d.initialized {
		return
	}

	data := make(map[string]string)
	d.Shorts = &data
	d.initialized = true
}

func (d *Data) Get(key string) (url string, err error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	url, ok := (*d.Shorts)[key]
	if !ok {
		return "", errors.New("could not find key")
	}

	return url, nil
}

func (d *Data) Add(url string) (string) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	//attempt generating unused random key
	key := nextRand()
	for {
		_, ok := (*d.Shorts)[key] // check if key is already used
		if !ok {
			break
		} else {
			key = nextRand()
		}
	}

	//add to the map
	(*d.Shorts)[key] = url

	//binary encode and save to disk

	f, err := os.OpenFile("burly.dat", os.O_CREATE | os.O_TRUNC, 0660)
	if err != nil{
		fmt.Printf("Could not open or create data file: %v", err)
	} else {
		b := bytes.Buffer{}
		e := gob.NewEncoder(&b)
		err := e.Encode(*d)
		if err != nil {
			fmt.Printf("failed gob encode: %v", err)
		} else {
			s := base64.StdEncoding.EncodeToString(b.Bytes())
			f.Write([]byte(s))
			f.Close()
		}
	}


	return key
}

func nextRand() string {
	return strconv.Itoa(rand.Int())
}

func main() {
	rand.Seed(time.Now().UnixNano())
	store.Init()
	router := mux.NewRouter()

	router.HandleFunc("/u/{key}", func(rw http.ResponseWriter, req *http.Request) {
		fmt.Println("in /u/{key}")

		key := mux.Vars(req)["key"]
		if key == "" {
			rw.WriteHeader(http.StatusBadRequest)
		} else {
			_, err := strconv.Atoi(key)
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				rw.Write([]byte(err.Error()))
				return
			}

			url, err := store.Get(key)
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				rw.Write([]byte(err.Error()))
				return
			}

			t, err := template.ParseFiles("redirect.html")
			if err != nil{
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			t.Execute(rw, url)
		}
	})

	router.HandleFunc("/new/", func(rw http.ResponseWriter, req *http.Request){
		fmt.Println("in /new/ (POST)")

		bodybytes, err := ioutil.ReadAll(req.Body)
		if err != nil{
			rw.WriteHeader(http.StatusInternalServerError)
		}

		url := string(bodybytes)
		key := store.Add(url)
		rw.Write([]byte(key))

	}).Methods("POST")

	router.HandleFunc("/new/{url}", func(rw http.ResponseWriter, req *http.Request) {
		fmt.Println("in /new/{url}")

		url := mux.Vars(req)["url"]
		key := store.Add(url)
		rw.Write([]byte(key))

		fmt.Printf("Store: \n%v", store.Shorts)
	})

	router.HandleFunc("/list/", func(rw http.ResponseWriter, req *http.Request){
		t, err := template.ParseFiles("list.html")
		if err != nil{
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		var templData struct{
			Host string
			Data map[string]string
		}

		templData.Host = req.Host
		templData.Data = *store.Shorts

		err = t.Execute(rw, templData)
		if err != nil{
			fmt.Printf("Template error: %v", err)
		}
	})

	http.Handle("/", router)

	http.ListenAndServe(":8001", nil)
}
