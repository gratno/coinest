package api

import (
	"bytes"
	"coinest/v1/config"
	"compress/flate"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func IsoTime() string {
	utcTime := time.Now().UTC()
	iso := utcTime.String()
	isoBytes := []byte(iso)
	iso = string(isoBytes[:10]) + "T" + string(isoBytes[11:23]) + "Z"
	return iso
}

func NewRequest(method string, path string, body io.Reader) *http.Request {
	buf := &bytes.Buffer{}
	var s string
	if body != nil {
		io.Copy(buf, body)
		body = ioutil.NopCloser(buf)
		s = buf.String()
		glog.Infof("%s %s %s\n", method, path, s)
	}
	req, _ := http.NewRequest(method, apiDomain+path, body)
	timestamp := IsoTime()
	req.Header.Set("OK-ACCESS-KEY", config.ApiKey)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", config.Passphrase)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", "locale=en_US")
	req.Header.Set("OK-ACCESS-SIGN", accessSign(timestamp, method, path, s))
	return req
}

func accessSign(timestamp string, method string, path string, body string) string {
	if method == "" {
		method = "GET"
	}
	if path == "" {
		path = "/users/self/verify"
	}
	method = strings.ToUpper(method)
	s := fmt.Sprintf("%s%s%s%s", timestamp, method, path, body)
	hash := hmac.New(sha256.New, []byte(config.SecretKey))
	hash.Write([]byte(s))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func decompress(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	var buf bytes.Buffer
	zr := flate.NewReader(nil)
	err := zr.(flate.Resetter).Reset(bytes.NewBuffer(data), nil)
	if err != nil {
		panic(err)
	}

	io.Copy(&buf, zr)
	return buf.Bytes()
}

type FakerPrice struct {
	Price    string
	Weighted int
}

func EqualsFakerPrice(arr1 []FakerPrice, arr2 []FakerPrice) bool {
	if len(arr1) != len(arr2) {
		return false
	}

	for i := range arr1 {
		if arr1[i].Weighted != arr2[i].Weighted && arr1[i].Price != arr2[i].Price {
			return false
		}
	}
	return true
}
