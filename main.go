package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
	"golang.org/x/time/rate"
)

var (
	DefaultTTL = flag.Duration("default_ttl", time.Hour*24*7, "default ttl for object")
	RateLimit  = flag.Int("rate_limit", 1, "rate limit for api call")
	SizeLimit  = flag.Int64("size_limit", 1024*1024*10, "size limit for object in byte")
	OSSecret   = flag.String("os_secret", "::", "secret key for object storage, format: key:id:session")
	BucketUrl  = flag.String("bucket_url", "", "bucket_url for object storage")
)

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	key, secret, token := "", "", ""
	if os.Getenv("os_secret") != "" {
		*OSSecret = os.Getenv("os_secret")
	}
	if os.Getenv("bucket_url") != "" {
		*BucketUrl = os.Getenv("bucket_url")
	}
	secretList := strings.Split(*OSSecret, ":")
	if len(secretList) >= 1 {
		key = secretList[0]
	}
	if len(secretList) >= 2 {
		secret = secretList[1]
	}
	if len(secretList) >= 3 {
		token = secretList[2]
	}

	u, err := url.Parse(*BucketUrl)
	if err != nil {
		panic("bucket url not valid")
	}
	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:     key,
			SecretKey:    secret,
			SessionToken: token,
		},
	})

	go expireJob(client)

	s := &http.Server{
		Addr:           ":80",
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 10,
	}

	limiter := rate.NewLimiter(rate.Limit(*RateLimit), *RateLimit)
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		_ = limiter.Wait(ctx)

		if request.Method == "POST" {
			log.Printf("[Handle] Method=Post Content-Length=%d", request.ContentLength)
			if *SizeLimit > 0 && request.ContentLength > *SizeLimit {
				writer.WriteHeader(400)
				_, _ = writer.Write([]byte(err.Error()))
				return
			}
			name := fmt.Sprintf("%s/%s", time.Now().Format("20060102"), randString(16))
			resp, err := client.Object.Put(ctx, name, request.Body, &cos.ObjectPutOptions{
				ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{},
			})
			if err != nil {
				writer.WriteHeader(400)
				_, _ = writer.Write([]byte(err.Error()))
				return
			}
			writer.WriteHeader(resp.StatusCode)
			_, _ = writer.Write([]byte(encodeName(name)))
		} else if request.Method == "GET" {
			name, err := decodeName(strings.Trim(request.URL.Path, "/"))
			log.Printf("[Handle] Method=Get Name=%s", name)
			if err != nil {
				writer.WriteHeader(400)
				_, _ = writer.Write([]byte(err.Error()))
				return
			}
			resp, err := client.Object.Get(ctx, name, nil)
			if err != nil {
				writer.WriteHeader(400)
				_, _ = writer.Write([]byte(err.Error()))
				return
			}
			data, _ := ioutil.ReadAll(resp.Body)
			_, _ = writer.Write(data)
		}
	})
	log.Println("starting server...")
	log.Fatal(s.ListenAndServe())
}

func expireJob(client *cos.Client) {
	for range time.Tick(time.Minute * 10) {
		log.Println("do expire....")
		expireTo := time.Now().Add(-1 * *DefaultTTL)

		for day := 1; day <= 30; day++ {
			toDelDay := expireTo.Add(time.Duration(day) * time.Hour * 24 * -1)
			name := toDelDay.Format("20060102") + "/"
			ctx := context.Background()
			resp, err := client.Object.Head(ctx, name, nil)
			if err != nil {
				if resp != nil && resp.StatusCode == 404 {
					continue
				}
				log.Println("head object failed, ", err)
				continue
			}
			if resp.StatusCode != 200 {
				continue
			}
			resp, err = client.Object.Delete(ctx, name, nil)
			if err != nil {
				log.Println("delete object failed, ", err)
				continue
			}
			if resp.StatusCode != 200 {
				log.Println("delete object failed, ", resp.Status)
			} else {
				log.Printf("expire old folder: %s success", name)
			}
		}
		log.Println("do expire done")
	}
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func encodeName(a string) string {
	return a[:8] + a[9:]
}

func decodeName(a string) (string, error) {
	if len(a) != 24 {
		return "", errors.New("invalid name")
	}
	return a[:8] + "/" + a[8:], nil
}
